package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

type GuestSessionMiddleware struct {
	Store *store.Pool
	Log   *slog.Logger
	Now   func() time.Time
}

type guestCtxKey struct{}
type guestCookieHashCtxKey struct{}

var GuestUserKey = guestCtxKey{}
var GuestCookieHashKey = guestCookieHashCtxKey{}

func (m *GuestSessionMiddleware) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now().UTC()
}

func (m *GuestSessionMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(GuestSessionCookieName)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "guest session required")
			return
		}
		hash := HashToken(c.Value)
		sess, err := m.Store.GuestSessionByTokenHash(r.Context(), hash, m.now())
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "guest session required")
			return
		}
		if err != nil {
			m.Log.Error("guest session middleware: lookup", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		guest, err := m.Store.GuestUserByID(r.Context(), sess.GuestUserID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "guest session required")
			return
		}
		if err != nil {
			m.Log.Error("guest session middleware: user lookup", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if !guest.IsActive {
			writeError(w, http.StatusUnauthorized, "deactivated", "guest account is deactivated")
			return
		}
		ctx := context.WithValue(r.Context(), GuestUserKey, guest)
		ctx = context.WithValue(ctx, GuestCookieHashKey, hash)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GuestFromContext(ctx context.Context) *store.GuestUser {
	u, _ := ctx.Value(GuestUserKey).(*store.GuestUser)
	return u
}

func GuestCookieHashFromContext(ctx context.Context) []byte {
	h, _ := ctx.Value(GuestCookieHashKey).([]byte)
	return h
}
