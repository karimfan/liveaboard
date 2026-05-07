package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

// SessionCookieName is the SPA-facing session cookie. Same name we
// have used since Sprint 003; the data behind it now lives in the
// `sessions` table (Sprint 009 cutover).
const SessionCookieName = "lb_session"
const GuestSessionCookieName = "lb_guest_session"

// MintSession generates a new opaque cookie token, persists a session
// row, and returns the raw token + session row. Callers set the cookie
// via SetSessionCookie.
func MintSession(
	ctx context.Context,
	pool *store.Pool,
	user *store.User,
	now time.Time,
	ttl time.Duration,
) (rawToken string, sess *store.Session, err error) {
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return "", nil, err
	}
	sess, err = pool.CreateSession(ctx, user.ID, tokenHash, now.Add(ttl))
	if err != nil {
		return "", nil, err
	}
	return rawToken, sess, nil
}

// SetSessionCookie writes the lb_session cookie to the response.
func SetSessionCookie(w http.ResponseWriter, rawToken string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  expires,
	})
}

// ClearSessionCookie writes a deletion cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func MintGuestSession(
	ctx context.Context,
	pool *store.Pool,
	guest *store.GuestUser,
	now time.Time,
	ttl time.Duration,
) (rawToken string, sess *store.GuestSession, err error) {
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return "", nil, err
	}
	sess, err = pool.CreateGuestSession(ctx, guest.ID, tokenHash, now.Add(ttl))
	if err != nil {
		return "", nil, err
	}
	return rawToken, sess, nil
}

func SetGuestSessionCookie(w http.ResponseWriter, rawToken string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     GuestSessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  expires,
	})
}

func ClearGuestSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     GuestSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
