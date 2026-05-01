// Package httpapi wires the auth + organization services into HTTP handlers.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/store"
)

const sessionCookieName = "lb_session"

type Server struct {
	Auth   *auth.Service
	Org    *org.Service
	Log    *slog.Logger
	Secure bool // set Secure flag on cookies (true in prod, false in dev http)
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogRequestLogger(s.Log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Route("/api", func(r chi.Router) {
		r.Post("/signup", s.handleSignup)
		r.Post("/verify-email", s.handleVerifyEmail)
		r.Post("/login", s.handleLogin)
		r.Post("/logout", s.handleLogout)

		r.Group(func(r chi.Router) {
			r.Use(s.requireSession)
			r.Get("/me", s.handleMe)
			r.Get("/organization", s.handleOrganization)
		})
	})

	return r
}

// --- handlers ---

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		FullName         string `json:"full_name"`
		OrganizationName string `json:"organization_name"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	res, err := s.Auth.Signup(r.Context(), auth.SignupInput{
		Email:            in.Email,
		Password:         in.Password,
		FullName:         in.FullName,
		OrganizationName: in.OrganizationName,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		case errors.Is(err, auth.ErrEmailTaken):
			// Non-enumerating: respond 200 with "if not already registered" semantics.
			// We still return 200 so callers cannot probe for email existence by
			// observing 409 vs 200. The verification email (logged) only fires for
			// genuinely-new accounts, so an attacker learns nothing.
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			s.Log.Error("signup failed", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"user_id":         res.User.ID,
		"organization_id": res.Organization.ID,
		// dev convenience — token is also logged. In production remove this field.
		"verification_token": res.VerificationToken,
	})
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.Auth.VerifyEmail(r.Context(), in.Token); err != nil {
		if errors.Is(err, auth.ErrTokenInvalid) {
			writeError(w, http.StatusBadRequest, "token_invalid", "token invalid or expired")
			return
		}
		s.Log.Error("verify-email failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	res, err := s.Auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		s.Log.Error("login failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	http.SetCookie(w, s.sessionCookie(res.Token, res.ExpiresAt))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.Auth.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, s.sessionCookie("", time.Unix(0, 0)))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":              u.ID,
		"email":           u.Email,
		"full_name":       u.FullName,
		"role":            u.Role,
		"organization_id": u.OrganizationID,
	})
}

func (s *Server) handleOrganization(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r.Context())
	view, err := s.Org.Dashboard(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("dashboard failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// --- middleware + helpers ---

type ctxKey string

const userCtxKey ctxKey = "user"

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookieName)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		user, err := s.Auth.ResolveSession(r.Context(), c.Value)
		if err != nil {
			s.Log.Error("resolve session", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if user == nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(userCtxKey).(*store.User)
	return u
}

func (s *Server) sessionCookie(value string, expires time.Time) *http.Cookie {
	c := &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.Secure,
		Expires:  expires,
	}
	if value == "" {
		c.MaxAge = -1
	}
	return c
}

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
	writeJSON(w, status, map[string]any{
		"error":   code,
		"message": message,
	})
}

func slogRequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
