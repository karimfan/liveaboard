// Package httpapi wires the auth + organization services into HTTP handlers.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/store"
)

// Server bundles the dependencies the chi router needs. After Sprint 005
// the Auth surface is split: Exchanger handles the Clerk-backed auth
// flows, AdminHandlers handles invitations and deactivation, Webhook
// handles Clerk -> us syncing, and SessionMiddleware authenticates every
// other API call via the lb_session cookie.
type Server struct {
	Org      *org.Service
	Log      *slog.Logger
	Exchange *auth.Exchanger
	Session  *auth.SessionMiddleware
	Admin    *auth.AdminHandlers
	Webhook  *auth.WebhookReceiver
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogRequestLogger(s.Log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Route("/api", func(r chi.Router) {
		// Public auth orchestration. The SPA calls these once after a
		// successful Clerk SignUp / SignIn to mint the lb_session
		// cookie; from then on every other endpoint is cookie-auth.
		r.Post("/signup-complete", s.Exchange.HandleSignupComplete)
		r.Post("/auth/exchange", s.Exchange.HandleExchange)
		r.Post("/logout", s.Exchange.HandleLogout)

		// Webhook: signature-verified, never cookie-auth.
		r.Post("/webhooks/clerk", s.Webhook.Handle)

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(s.Session.Wrap)

			r.Get("/me", s.handleMe)
			r.Get("/organization", s.handleOrganization)

			// Org-admin-only routes.
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireOrgAdmin)
				r.Post("/invitations", s.Admin.HandleInvite)
				r.Post("/invitations/{id}/resend", s.handleResendInvite)
				r.Post("/users/{id}/deactivate", s.handleDeactivateUser)
			})
		})
	})

	// Static SPA + fallback. Anything not under /api falls through here.
	r.Handle("/*", SPAHandler())

	return r
}

// --- handlers ---

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"id":              u.ID,
		"email":           u.Email,
		"full_name":       u.FullName,
		"role":            u.Role,
		"organization_id": u.OrganizationID,
	})
}

func (s *Server) handleOrganization(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	view, err := s.Org.Dashboard(r.Context(), u.OrganizationID)
	if err != nil {
		s.Log.Error("dashboard failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleResendInvite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.Admin.HandleResendInvite(w, r, id)
}

func (s *Server) handleDeactivateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.Admin.HandleDeactivateUser(w, r, id)
}

// --- helpers ---

// userFromContext is a thin re-export of auth.UserFromContext for any
// callers within this package that don't want to import internal/auth.
func userFromContext(ctx interface{ Value(any) any }) *store.User {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(auth.SessionUserKey).(*store.User)
	return v
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
