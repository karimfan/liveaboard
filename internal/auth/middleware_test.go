package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func newMiddleware(t *testing.T, pool *store.Pool) *auth.SessionMiddleware {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return &auth.SessionMiddleware{Store: pool, Log: log}
}

func protectedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			http.Error(w, "no user", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(u.Email))
	})
}

func bootstrapUserAndCookie(t *testing.T, pool *store.Pool) (*store.User, string) {
	t.Helper()
	ctx := context.Background()
	_, user, err := pool.CreateExternalOrgAndAdmin(ctx, "Acme", "org_clerk_x", "user_clerk_x", "u@x.test", "U")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	tokenBytes := []byte("middleware-token-fixture")
	hashArr := sha256.Sum256(tokenBytes)
	tokenHash := hashArr[:]
	tokenStr := hex.EncodeToString(tokenBytes)
	_, err = pool.CreateAppSession(ctx, user.ID, auth.HashCookieToken(tokenStr), "user_clerk_x", "sess_clerk_x", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create app session: %v", err)
	}
	_ = tokenHash
	return user, tokenStr
}

func TestMiddlewareNoCookieIs401(t *testing.T) {
	pool := testdb.Pool(t)
	mw := newMiddleware(t, pool)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	mw.Wrap(protectedHandler()).ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}
}

func TestMiddlewareUnknownTokenIs401(t *testing.T) {
	pool := testdb.Pool(t)
	mw := newMiddleware(t, pool)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "completely-bogus"})
	rec := httptest.NewRecorder()
	mw.Wrap(protectedHandler()).ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Result().StatusCode)
	}
}

func TestMiddlewareValidCookieAttachesUser(t *testing.T) {
	pool := testdb.Pool(t)
	mw := newMiddleware(t, pool)
	user, token := bootstrapUserAndCookie(t, pool)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	mw.Wrap(protectedHandler()).ServeHTTP(rec, req)

	resp := rec.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != user.Email {
		t.Errorf("body %q want %q", string(body), user.Email)
	}
}

func TestMiddlewareDeactivatedUserIs401(t *testing.T) {
	pool := testdb.Pool(t)
	mw := newMiddleware(t, pool)
	user, token := bootstrapUserAndCookie(t, pool)
	if err := pool.DeactivateUser(context.Background(), user.ID); err != nil {
		t.Fatalf("DeactivateUser: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	mw.Wrap(protectedHandler()).ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", rec.Result().StatusCode)
	}
}

func TestMiddlewareExpiredSessionIs401(t *testing.T) {
	pool := testdb.Pool(t)
	mw := newMiddleware(t, pool)
	ctx := context.Background()
	_, user, err := pool.CreateExternalOrgAndAdmin(ctx, "Acme", "org_y", "user_y", "y@x.test", "Y")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	rawToken := "expired-token-fixture"
	if _, err := pool.CreateAppSession(ctx, user.ID, auth.HashCookieToken(rawToken), "user_y", "sess_y", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("CreateAppSession: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: rawToken})
	rec := httptest.NewRecorder()
	mw.Wrap(protectedHandler()).ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", rec.Result().StatusCode)
	}
}
