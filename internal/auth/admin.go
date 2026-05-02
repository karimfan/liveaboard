package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
)

// AdminHandlers backs the org-admin operations: invite, resend, revoke
// invitation, and deactivate user. These endpoints are mounted behind
// requireSession + requireOrgAdmin in the router.
type AdminHandlers struct {
	Provider Provider
	Store    *store.Pool
	Log      *slog.Logger
}

// HandleInvite is POST /api/invitations.
//
// Body: { "email": "...", "role": "site_director" }.
// The acting user must be an org_admin; the invitation goes to that
// admin's organization.
func (a *AdminHandlers) HandleInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !looksLikeEmailAddr(email) {
		writeError(w, http.StatusBadRequest, "invalid_input", "email is required")
		return
	}
	role := normalizeRole(in.Role, RoleSiteDirector)
	if role != RoleOrgAdmin && role != RoleSiteDirector {
		writeError(w, http.StatusBadRequest, "invalid_input", "role must be org_admin or site_director")
		return
	}

	admin := UserFromContext(r.Context())
	clerkOrgID, err := a.lookupClerkOrgID(r.Context(), admin)
	if err != nil {
		a.Log.Error("invite: lookup org", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	inv, err := a.Provider.InviteToOrganization(r.Context(), clerkOrgID, email, role)
	if err != nil {
		switch {
		case errors.Is(err, ErrProviderConflict):
			writeError(w, http.StatusConflict, "already_invited", "an invitation for this email is already pending")
		default:
			a.Log.Error("invite: provider call", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"invitation": invitationView(inv),
	})
}

// HandleResendInvite is POST /api/invitations/{id}/resend. The url path
// router (chi) extracts the id and calls this with chi.URLParam.
func (a *AdminHandlers) HandleResendInvite(w http.ResponseWriter, r *http.Request, inviteID string) {
	if inviteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "invitation id required")
		return
	}
	admin := UserFromContext(r.Context())
	clerkOrgID, err := a.lookupClerkOrgID(r.Context(), admin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	if err := a.Provider.ResendInvitation(r.Context(), clerkOrgID, inviteID); err != nil {
		switch {
		case errors.Is(err, ErrProviderNotFound):
			writeError(w, http.StatusNotFound, "not_found", "invitation not found")
		case errors.Is(err, ErrProviderConflict):
			writeError(w, http.StatusConflict, "not_pending", "invitation is no longer pending")
		default:
			a.Log.Error("resend invite: provider call", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// HandleDeactivateUser is POST /api/users/{id}/deactivate. Removes the
// Clerk membership, sets users.is_active=false, deletes app_sessions for
// the target user. Refuses to deactivate the calling admin (no
// self-lock-out).
func (a *AdminHandlers) HandleDeactivateUser(w http.ResponseWriter, r *http.Request, targetUserID string) {
	id, err := uuid.Parse(targetUserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "user id must be a uuid")
		return
	}
	admin := UserFromContext(r.Context())
	if admin.ID == id {
		writeError(w, http.StatusForbidden, "self_deactivation", "cannot deactivate yourself")
		return
	}

	target, err := a.Store.UserByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		a.Log.Error("deactivate: lookup user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if target.OrganizationID != admin.OrganizationID {
		// Cross-tenant access. Don't enumerate.
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	clerkOrgID, err := a.lookupClerkOrgID(r.Context(), admin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if target.ClerkUserID != "" {
		if err := a.Provider.RemoveMembership(r.Context(), clerkOrgID, target.ClerkUserID); err != nil {
			// Provider not_found means the membership was already gone —
			// proceed with local deactivation.
			if !errors.Is(err, ErrProviderNotFound) {
				a.Log.Warn("deactivate: provider RemoveMembership", "err", err)
			}
		}
	}

	if err := a.Store.DeactivateUser(r.Context(), target.ID); err != nil {
		a.Log.Error("deactivate: store", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if err := a.Store.DeleteAppSessionsForUser(r.Context(), target.ID); err != nil {
		a.Log.Warn("deactivate: revoke app sessions", "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// RequireOrgAdmin is a middleware factory that 403s any request whose
// authenticated user does not have role=org_admin. Mount AFTER
// SessionMiddleware on routes that should be admin-only.
func RequireOrgAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		if u.Role != RoleOrgAdmin {
			writeError(w, http.StatusForbidden, "forbidden", "org_admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- helpers ---

func (a *AdminHandlers) lookupClerkOrgID(ctx context.Context, admin *store.User) (string, error) {
	// We look up the org row to get clerk_org_id. The local row was
	// created via /api/signup-complete which set clerk_org_id, so this
	// should always succeed.
	type orgPeek struct {
		ClerkOrgID *string
	}
	var peek orgPeek
	err := a.Store.QueryRow(ctx, `SELECT clerk_org_id FROM organizations WHERE id = $1`, admin.OrganizationID).Scan(&peek.ClerkOrgID)
	if err != nil {
		return "", err
	}
	if peek.ClerkOrgID == nil || *peek.ClerkOrgID == "" {
		return "", errors.New("organization is not linked to clerk")
	}
	return *peek.ClerkOrgID, nil
}

func looksLikeEmailAddr(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.ContainsAny(s, " \t\n") {
		return false
	}
	return strings.Contains(s[at+1:], ".")
}

func normalizeRole(in, fallback string) string {
	r := strings.ToLower(strings.TrimSpace(in))
	if r == "" {
		return fallback
	}
	return r
}

func invitationView(inv *ProviderInvitation) map[string]any {
	if inv == nil {
		return nil
	}
	return map[string]any{
		"id":     inv.ID,
		"email":  inv.Email,
		"role":   inv.Role,
		"status": inv.Status,
	}
}
