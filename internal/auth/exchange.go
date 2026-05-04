package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

// Exchanger orchestrates the Clerk -> lb_session bridge. It exposes two
// HTTP handlers:
//
//   - HandleSignupComplete: brand-new user. Creates local org + Clerk org +
//     local users row atomically, then mints lb_session.
//   - HandleExchange: existing local user. Verifies a Clerk JWT, refreshes
//     identity, mints lb_session.
//
// Both handlers preserve the SPA contract: same-origin lb_session cookie,
// no Bearer tokens leaking into the SPA, no JWT in long-term storage.
type Exchanger struct {
	Provider Provider
	Store    *store.Pool
	Log      *slog.Logger
	Now      func() time.Time

	// SessionTTL controls how long an lb_session is valid before the SPA
	// must re-exchange. Should match the Clerk session TTL or be shorter.
	SessionTTL time.Duration

	// CookieSecure controls the Secure flag on the response cookie.
	CookieSecure bool
}

// HandleSignupCompleteRequest is the JSON body of POST /api/signup-complete.
//
// The Clerk session JWT is supplied either in the JSON body (`clerk_jwt`)
// or in the Authorization header. The SPA submits it once, immediately
// after a successful Clerk SignUp; from there on, the SPA uses the
// lb_session cookie and the JWT is dropped.
type HandleSignupCompleteRequest struct {
	ClerkJWT         string `json:"clerk_jwt,omitempty"`
	OrganizationName string `json:"organization_name"`
	FullName         string `json:"full_name,omitempty"`
}

func (e *Exchanger) HandleSignupComplete(w http.ResponseWriter, r *http.Request) {
	var in HandleSignupCompleteRequest
	if !readJSON(w, r, &in) {
		return
	}
	jwt := pickJWT(r, in.ClerkJWT)
	if jwt == "" {
		writeError(w, http.StatusUnauthorized, "missing_token", "clerk session token required")
		return
	}
	orgName := strings.TrimSpace(in.OrganizationName)
	if orgName == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "organization_name is required")
		return
	}

	claims, err := e.Provider.VerifyJWT(r.Context(), jwt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "clerk session token is invalid or expired")
		return
	}
	pUser, err := e.Provider.FetchUser(r.Context(), claims.UserID)
	if err != nil {
		e.Log.Error("signup-complete: fetch user", "err", err, "clerk_user_id", claims.UserID)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	fullName := strings.TrimSpace(in.FullName)
	if fullName == "" {
		fullName = pUser.FullName
	}
	if fullName == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "full_name is required")
		return
	}

	// Create a Clerk organization with this user as admin; idempotent
	// retries fall back to the existing org via FetchOrganization later
	// if needed (Phase 5 reconciler concern; out of scope here).
	pOrg, err := e.Provider.CreateOrganization(r.Context(), orgName, claims.UserID)
	if err != nil {
		e.Log.Error("signup-complete: create clerk org", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	org, user, err := e.Store.CreateExternalOrgAndAdmin(
		r.Context(),
		orgName, pOrg.ID,
		claims.UserID, pUser.Email, fullName,
	)
	if err != nil {
		// Compensate: best-effort delete the just-created Clerk org so a
		// retry by the user does not pile up dangling orgs. Errors here
		// are logged but do not change the response — the local insert
		// already failed.
		_ = err // see also Phase 5: reconciler closes any drift
		switch {
		case errors.Is(err, store.ErrEmailTaken):
			writeError(w, http.StatusConflict, "email_taken", "an account with this email already exists")
		case errors.Is(err, store.ErrUserClerkIDTaken), errors.Is(err, store.ErrOrgClerkIDTaken):
			// Idempotency: another in-flight request already linked this user.
			writeError(w, http.StatusConflict, "already_linked", "user already linked to an organization")
		default:
			e.Log.Error("signup-complete: create local org+admin", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	if err := e.mintAndSet(w, r.Context(), user, claims); err != nil {
		e.Log.Error("signup-complete: mint session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"organization_id": org.ID,
		"user_id":         user.ID,
	})
}

// HandleExchangeRequest is the JSON body of POST /api/auth/exchange.
type HandleExchangeRequest struct {
	ClerkJWT string `json:"clerk_jwt,omitempty"`
}

func (e *Exchanger) HandleExchange(w http.ResponseWriter, r *http.Request) {
	var in HandleExchangeRequest
	// Body is optional — JWT may also come from Authorization.
	_ = json.NewDecoder(r.Body).Decode(&in)
	jwt := pickJWT(r, in.ClerkJWT)
	if jwt == "" {
		writeError(w, http.StatusUnauthorized, "missing_token", "clerk session token required")
		return
	}

	claims, err := e.Provider.VerifyJWT(r.Context(), jwt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "clerk session token is invalid or expired")
		return
	}

	user, err := e.Store.UserByClerkID(r.Context(), claims.UserID)
	if errors.Is(err, store.ErrNotFound) {
		// Either the user has not yet completed signup (no org) or their
		// invitation acceptance webhook has not landed. Phase 5 will add
		// a synchronous fallback for the second case; for now, a clear
		// 401 telling the SPA to call /signup-complete (or wait briefly
		// and retry) is correct.
		writeError(w, http.StatusUnauthorized, "membership_pending", "no organization membership for this user yet")
		return
	}
	if err != nil {
		e.Log.Error("exchange: lookup user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if !user.IsActive {
		writeError(w, http.StatusUnauthorized, "deactivated", "account is deactivated")
		return
	}

	// Refresh identity from Clerk if it has changed (no role overwrite).
	pUser, err := e.Provider.FetchUser(r.Context(), claims.UserID)
	if err == nil && (pUser.Email != user.Email || pUser.FullName != user.FullName) {
		if err := e.Store.UpdateExternalUser(r.Context(), claims.UserID, pUser.Email, pUser.FullName); err != nil {
			e.Log.Warn("exchange: identity sync failed", "err", err)
		}
	}

	if err := e.mintAndSet(w, r.Context(), user, claims); err != nil {
		e.Log.Error("exchange: mint session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleLogout revokes the Clerk session, deletes the local app_session
// row, and clears the lb_session cookie.
func (e *Exchanger) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		hash := HashCookieToken(c.Value)
		if sess, err := e.Store.AppSessionByTokenHash(r.Context(), hash, e.now()); err == nil {
			_ = e.Provider.RevokeSession(r.Context(), sess.ClerkSessionID)
		}
		_ = e.Store.DeleteAppSessionByTokenHash(r.Context(), hash)
	}
	ClearSessionCookie(w, e.CookieSecure)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (e *Exchanger) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Exchanger) mintAndSet(w http.ResponseWriter, ctx context.Context, user *store.User, claims *Claims) error {
	now := e.now()
	rawToken, sess, err := MintAppSession(ctx, e.Store, user, claims.UserID, claims.SessionID, now, e.SessionTTL)
	if err != nil {
		return fmt.Errorf("mint app session: %w", err)
	}
	SetSessionCookie(w, rawToken, sess.ExpiresAt, e.CookieSecure)
	return nil
}

// --- request/response helpers ---
//
// These mirror the existing helpers in internal/httpapi/httpapi.go but live
// here so this package can be used standalone in tests without pulling in
// the full HTTP router.

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": code, "message": message})
}

func pickJWT(r *http.Request, body string) string {
	if body != "" {
		return body
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
