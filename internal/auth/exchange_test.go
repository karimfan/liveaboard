package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func newExchanger(t *testing.T, pool *store.Pool, p *auth.StubProvider) *auth.Exchanger {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	return &auth.Exchanger{
		Provider:     p,
		Store:        pool,
		Log:          log,
		SessionTTL:   time.Hour,
		CookieSecure: false,
	}
}

func postJSON(handler http.HandlerFunc, body any, jwt string) *http.Response {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", "application/json")
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec.Result()
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode body: %v: %s", err, b)
	}
	return m
}

func TestSignupCompleteHappyPath(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	pUser := stub.NewUser("owner@acme.test", "Acme Owner")
	token, _ := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)
	resp := postJSON(x.HandleSignupComplete, map[string]string{
		"organization_name": "Acme Diving",
		"full_name":         "Acme Owner",
	}, token)

	if resp.StatusCode != http.StatusOK {
		body := decodeBody(t, resp)
		t.Fatalf("status %d: %v", resp.StatusCode, body)
	}
	body := decodeBody(t, resp)
	if body["ok"] != true {
		t.Errorf("ok: %v", body["ok"])
	}

	// lb_session cookie set.
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected lb_session cookie")
	}

	// Local rows exist with linkage.
	user, err := pool.UserByClerkID(context.Background(), pUser.ID)
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	if user.Role != store.RoleOrgAdmin {
		t.Errorf("role = %q", user.Role)
	}
	// app_sessions row created.
	sess, err := pool.AppSessionByTokenHash(context.Background(), auth.HashCookieToken(sessionCookie.Value), time.Now())
	if err != nil {
		t.Fatalf("AppSessionByTokenHash: %v", err)
	}
	if sess.UserID != user.ID {
		t.Errorf("session.UserID = %v want %v", sess.UserID, user.ID)
	}
}

func TestSignupCompleteRejectsMissingOrganizationName(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	pUser := stub.NewUser("a@x.test", "Alice")
	token, _ := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)
	resp := postJSON(x.HandleSignupComplete, map[string]string{
		"organization_name": "",
	}, token)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["error"] != "invalid_input" {
		t.Errorf("error code = %v", body["error"])
	}
}

func TestSignupCompleteRejectsInvalidToken(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	x := newExchanger(t, pool, stub)

	resp := postJSON(x.HandleSignupComplete, map[string]string{
		"organization_name": "Acme",
	}, "not-a-real-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestSignupCompleteIdempotentOnDoubleCall(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	pUser := stub.NewUser("a@x.test", "Alice")
	token, _ := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)

	resp := postJSON(x.HandleSignupComplete, map[string]string{"organization_name": "Acme"}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first: %d", resp.StatusCode)
	}

	// Second call: same Clerk user, should now conflict.
	resp = postJSON(x.HandleSignupComplete, map[string]string{"organization_name": "Acme 2"}, token)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second: %d want 409", resp.StatusCode)
	}
}

func TestExchangeHappyPath(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	pUser := stub.NewUser("a@x.test", "Alice")
	token, _ := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)

	// Bootstrap via signup-complete first.
	if resp := postJSON(x.HandleSignupComplete, map[string]string{"organization_name": "Acme"}, token); resp.StatusCode != http.StatusOK {
		t.Fatalf("setup: %d", resp.StatusCode)
	}

	// New Clerk session for the same user (e.g. logging in fresh in another browser).
	token2, _ := stub.NewSession(pUser.ID, "", time.Hour)
	resp := postJSON(x.HandleExchange, map[string]string{}, token2)
	if resp.StatusCode != http.StatusOK {
		body := decodeBody(t, resp)
		t.Fatalf("exchange: %d %v", resp.StatusCode, body)
	}

	cookies := resp.Cookies()
	if !hasCookie(cookies, auth.SessionCookieName) {
		t.Errorf("expected lb_session cookie")
	}
}

func TestExchangeMembershipPendingFor401(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	// User exists in Clerk but never called /signup-complete.
	pUser := stub.NewUser("a@x.test", "Alice")
	token, _ := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)
	resp := postJSON(x.HandleExchange, map[string]string{}, token)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["error"] != "membership_pending" {
		t.Errorf("error = %v", body["error"])
	}
}

func TestExchangeRefreshesEmailAndFullName(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	pUser := stub.NewUser("old@x.test", "Old Name")
	token, _ := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)
	if resp := postJSON(x.HandleSignupComplete, map[string]string{"organization_name": "Acme"}, token); resp.StatusCode != http.StatusOK {
		t.Fatalf("setup: %d", resp.StatusCode)
	}

	// Mutate Clerk-side identity.
	pUser.Email = "new@x.test"
	pUser.FullName = "New Name"
	stub.OverrideUser(pUser)

	if resp := postJSON(x.HandleExchange, map[string]string{}, token); resp.StatusCode != http.StatusOK {
		t.Fatalf("exchange: %d", resp.StatusCode)
	}

	got, err := pool.UserByClerkID(context.Background(), pUser.ID)
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	if got.Email != "new@x.test" || got.FullName != "New Name" {
		t.Errorf("identity not refreshed: %+v", got)
	}
}

func TestExchangeRejectsDeactivatedUser(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	pUser := stub.NewUser("a@x.test", "Alice")
	token, _ := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)
	if resp := postJSON(x.HandleSignupComplete, map[string]string{"organization_name": "Acme"}, token); resp.StatusCode != http.StatusOK {
		t.Fatalf("setup: %d", resp.StatusCode)
	}

	user, err := pool.UserByClerkID(context.Background(), pUser.ID)
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	if err := pool.DeactivateUser(context.Background(), user.ID); err != nil {
		t.Fatalf("DeactivateUser: %v", err)
	}

	resp := postJSON(x.HandleExchange, map[string]string{}, token)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d want 401", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["error"] != "deactivated" {
		t.Errorf("error = %v", body["error"])
	}
}

func TestLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	pool := testdb.Pool(t)
	stub := auth.NewStubProvider()
	pUser := stub.NewUser("a@x.test", "Alice")
	token, clerkSessID := stub.NewSession(pUser.ID, "", time.Hour)

	x := newExchanger(t, pool, stub)
	resp := postJSON(x.HandleSignupComplete, map[string]string{"organization_name": "Acme"}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup: %d", resp.StatusCode)
	}
	cookies := resp.Cookies()
	cookie := pickCookie(cookies, auth.SessionCookieName)

	// Logout request includes the cookie.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	x.HandleLogout(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("logout status %d", rec.Result().StatusCode)
	}

	// app_session is gone.
	if _, err := pool.AppSessionByTokenHash(context.Background(), auth.HashCookieToken(cookie.Value), time.Now()); err == nil {
		t.Errorf("app_session should be deleted")
	}
	// Clerk session is revoked (subsequent verify fails).
	if _, err := stub.VerifyJWT(context.Background(), token); err == nil {
		t.Errorf("Clerk session should be revoked")
	}
	// Cookie has been cleared.
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.MaxAge >= 0 {
			t.Errorf("expected cookie to be cleared, got %+v", c)
		}
	}
	_ = clerkSessID
}

func hasCookie(cs []*http.Cookie, name string) bool {
	for _, c := range cs {
		if c.Name == name && c.Value != "" {
			return true
		}
	}
	return false
}

func pickCookie(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return nil
}
