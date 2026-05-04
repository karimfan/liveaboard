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
// It looks up the matching sessions row, resolves the user, and
// attaches the user (plus the cookie token hash) to the request
// context. Inactive users get 401.
type SessionMiddleware struct {
	Store *store.Pool
	Log   *slog.Logger
	Now   func() time.Time
}

type sessionCtxKey struct{}
type cookieHashCtxKey struct{}

// SessionUserKey holds the *store.User on authenticated requests.
var SessionUserKey = sessionCtxKey{}

// SessionCookieHashKey holds the sha256(rawToken) so handlers that
// need to "invalidate other sessions" can preserve the caller's.
var SessionCookieHashKey = cookieHashCtxKey{}

func (m *SessionMiddleware) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now().UTC()
}

// Wrap returns a chi-compatible middleware: cookie missing/invalid -> 401.
func (m *SessionMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookieName)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		hash := HashToken(c.Value)
		sess, err := m.Store.SessionByTokenHash(r.Context(), hash, m.now())
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		if err != nil {
			m.Log.Error("session middleware: session lookup", "err", err)
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
		ctx = context.WithValue(ctx, SessionCookieHashKey, hash)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromContext returns the authenticated user attached by Wrap.
// Returns nil if the request did not pass through the middleware.
func UserFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(SessionUserKey).(*store.User)
	return u
}

// CookieHashFromContext returns the sha256 of the lb_session cookie
// for the current request, so handlers that "invalidate other sessions"
// (change-password, change-email confirm) can preserve the caller's.
func CookieHashFromContext(ctx context.Context) []byte {
	h, _ := ctx.Value(SessionCookieHashKey).([]byte)
	return h
}

// RequireOrgAdmin is a chi-compatible middleware that returns 403 unless
// the authenticated user has role org_admin. It assumes SessionMiddleware
// already attached the user to the context; if it didn't, the request is
// treated as unauthenticated.
func RequireOrgAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "session required")
			return
		}
		if u.Role != store.RoleOrgAdmin {
			writeError(w, http.StatusForbidden, "forbidden", "org admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
