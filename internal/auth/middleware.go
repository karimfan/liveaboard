package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

// SessionMiddleware authenticates requests via the lb_session cookie.
// It looks up the matching app_sessions row, resolves the local users
// row, and attaches it to the request context. Inactive users get 401.
//
// Phase 6 will mount this on the chi router in place of Sprint 003's
// requireSession; until then it is exercised by unit tests only.
type SessionMiddleware struct {
	Store *store.Pool
	Log   *slog.Logger
	Now   func() time.Time
}

// SessionUserContextKey is the context key under which authenticated
// users are stored. The httpapi package re-exports the same key type so
// handlers can read the user without depending on this package.
type sessionCtxKey struct{}

// SessionUserKey is the typed context key for the authenticated user.
var SessionUserKey = sessionCtxKey{}

func (m *SessionMiddleware) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Wrap returns a chi-compatible middleware: cookie missing/invalid -> 401.
func (m *SessionMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookieName)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		sess, err := m.Store.AppSessionByTokenHash(r.Context(), HashCookieToken(c.Value), m.now())
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		if err != nil {
			m.Log.Error("session middleware: app session lookup", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		user, err := m.Store.UserByID(r.Context(), sess.UserID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		if err != nil {
			m.Log.Error("session middleware: user lookup", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if !user.IsActive {
			writeError(w, http.StatusUnauthorized, "deactivated", "account is deactivated")
			return
		}
		ctx := context.WithValue(r.Context(), SessionUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromContext returns the authenticated user attached by Wrap.
// Returns nil if the request did not pass through the middleware.
func UserFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(SessionUserKey).(*store.User)
	return u
}
