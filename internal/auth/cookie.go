package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

// SessionCookieName is the SPA-facing session cookie. It is the same name
// Sprint 003 used; the cutover replaces the data behind it (sessions row
// -> app_sessions row backed by a Clerk session) without changing the
// frontend contract.
const SessionCookieName = "lb_session"

// MintAppSession generates a new opaque cookie token, persists an
// app_sessions row, and returns (rawToken, sessionRow). Callers set the
// cookie via SetSessionCookie.
func MintAppSession(
	ctx context.Context,
	pool *store.Pool,
	user *store.User,
	clerkUserID, clerkSessionID string,
	now time.Time,
	ttl time.Duration,
) (rawToken string, sess *store.AppSession, err error) {
	rawToken, tokenHash, err := newCookieToken()
	if err != nil {
		return "", nil, err
	}
	sess, err = pool.CreateAppSession(ctx, user.ID, tokenHash, clerkUserID, clerkSessionID, now.Add(ttl))
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

// HashCookieToken returns sha256(token) — the value persisted as
// app_sessions.token_hash. The plaintext token is never stored.
func HashCookieToken(rawToken string) []byte {
	sum := sha256.Sum256([]byte(rawToken))
	return sum[:]
}

func newCookieToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = hex.EncodeToString(b)
	return raw, HashCookieToken(raw), nil
}
