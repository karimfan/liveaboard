package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// auth_handlers.go: Sprint 009 HTTP surface for the custom-auth flows.
// Backend handlers map service-layer outcomes to HTTP. The service is
// responsible for non-enumeration; handlers preserve that by returning
// 200 on the same shape regardless of whether the operation matched a
// real account.

// --- signup / verification ---

type signupReq struct {
	Email            string `json:"email"`
	Password         string `json:"password"`
	FullName         string `json:"full_name"`
	OrganizationName string `json:"organization_name"`
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	_, err := s.Auth.Signup(r.Context(), auth.SignupInput{
		Email:            req.Email,
		Password:         req.Password,
		FullName:         req.FullName,
		OrganizationName: req.OrganizationName,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Always 200 — duplicate emails are silently swallowed by the service.
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Check your inbox to verify your email.",
	})
}

type verifyEmailReq struct {
	Token string `json:"token"`
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if err := s.Auth.VerifyEmail(r.Context(), req.Token); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type resendVerificationReq struct {
	Email string `json:"email"`
}

func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	var req resendVerificationReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if err := s.Auth.ResendVerification(r.Context(), req.Email); err != nil {
		s.Log.Error("resend verification", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- login / logout ---

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := s.Auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		var lockout *auth.LockoutError
		switch {
		case errors.As(err, &lockout):
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":           "rate_limited",
				"message":         "Too many failed attempts.",
				"retry_after_sec": int(lockout.RetryAfter.Seconds()),
			})
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password.")
		case errors.Is(err, auth.ErrVerificationRequired):
			writeError(w, http.StatusForbidden, "verification_required", "Please verify your email before signing in.")
		default:
			s.Log.Error("login", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	auth.SetSessionCookie(w, res.Token, res.ExpiresAt, s.CookieSecure)
	writeJSON(w, http.StatusOK, authUserView(res.User))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookieName); err == nil {
		_ = s.Auth.Logout(r.Context(), c.Value)
	}
	auth.ClearSessionCookie(w, s.CookieSecure)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- forgot / reset / change password ---

type forgotPasswordReq struct {
	Email string `json:"email"`
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if err := s.Auth.ForgotPassword(r.Context(), req.Email); err != nil {
		s.Log.Error("forgot password", "err", err)
		// The service swallows not-found internally; this is a real error.
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "If that email exists, a reset link is on its way.",
	})
}

type resetPasswordReq struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	res, err := s.Auth.ResetPassword(r.Context(), req.Token, req.NewPassword)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	auth.SetSessionCookie(w, res.Token, res.ExpiresAt, s.CookieSecure)
	writeJSON(w, http.StatusOK, authUserView(res.User))
}

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	keep := auth.CookieHashFromContext(r.Context())
	var req changePasswordReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if err := s.Auth.ChangePassword(r.Context(), u.ID, req.CurrentPassword, req.NewPassword, keep); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- change email ---

type requestEmailChangeReq struct {
	NewEmail        string `json:"new_email"`
	CurrentPassword string `json:"current_password"`
}

func (s *Server) handleRequestEmailChange(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req requestEmailChangeReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if err := s.Auth.RequestEmailChange(r.Context(), u.ID, req.NewEmail, req.CurrentPassword); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Confirmation email sent to the new address.",
	})
}

type confirmEmailChangeReq struct {
	Token string `json:"token"`
}

func (s *Server) handleConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	keep := auth.CookieHashFromContext(r.Context())
	var req confirmEmailChangeReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	u, err := s.Auth.ConfirmEmailChange(r.Context(), req.Token, keep)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authUserView(u))
}

func (s *Server) handlePendingEmailChange(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	pending, err := s.Auth.PendingEmailChange(r.Context(), u.ID)
	if err != nil {
		s.Log.Error("pending email change", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if pending == nil {
		writeJSON(w, http.StatusOK, map[string]any{"pending": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pending": map[string]any{
			"new_email":  pending.NewEmail,
			"expires_at": pending.ExpiresAt,
		},
	})
}

func (s *Server) handleCancelEmailChange(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	if err := s.Auth.CancelEmailChange(r.Context(), u.ID); err != nil {
		s.Log.Error("cancel email change", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- helpers ---

// decodeJSON enforces unknown-field rejection so tests catch typos.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// authUserView is the auth-flow shape: includes organization_id and an
// email_verified flag the SPA uses to decide post-signup routing. The
// admin-chrome surface has its own narrower userView (see admin.go).
func authUserView(u *store.User) map[string]any {
	verified := false
	if u.EmailVerifiedAt != nil {
		verified = true
	}
	return map[string]any{
		"id":              u.ID,
		"email":           u.Email,
		"full_name":       u.FullName,
		"role":            u.Role,
		"organization_id": u.OrganizationID,
		"email_verified":  verified,
	}
}

// writeServiceError maps auth-package sentinels to HTTP statuses. Any
// error not recognised here logs at ERROR and surfaces as 500.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials.")
	case errors.Is(err, auth.ErrTokenInvalid):
		writeError(w, http.StatusBadRequest, "token_invalid", "This link is invalid or has expired.")
	case errors.Is(err, auth.ErrVerificationRequired):
		writeError(w, http.StatusForbidden, "verification_required", "Please verify your email first.")
	case errors.Is(err, store.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "That email is already in use.")
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", trimSentinel(err.Error()))
	case strings.HasPrefix(err.Error(), "auth: password"):
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

// trimSentinel turns "auth: invalid input: full_name" into "full_name"
// for cleaner client-side messages.
func trimSentinel(s string) string {
	for _, p := range []string{"auth: invalid input: ", "auth: "} {
		if strings.HasPrefix(s, p) {
			return strings.TrimPrefix(s, p)
		}
	}
	return s
}
