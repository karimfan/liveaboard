package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

// Test harness: builds an httptest.Server wired to a StubProvider so the
// HTTP integration tests run fully offline. The webhook handler is
// constructed with a fixed test secret; tests that exercise webhooks
// sign with the same secret.
const testWebhookSecret = "whsec_C2FVsBE8+CIqwLrHLMyD6gtVsh5TfEKJ"

type harness struct {
	server *httptest.Server
	pool   *store.Pool
	stub   *auth.StubProvider
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testdb.Pool(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	stub := auth.NewStubProvider()

	exchanger := &auth.Exchanger{
		Provider:     stub,
		Store:        pool,
		Log:          log,
		SessionTTL:   time.Hour,
		CookieSecure: false,
	}
	session := &auth.SessionMiddleware{Store: pool, Log: log}
	admin := &auth.AdminHandlers{Provider: stub, Store: pool, Log: log}
	wh, err := auth.NewWebhookReceiver(stub, pool, log, testWebhookSecret)
	if err != nil {
		t.Fatalf("NewWebhookReceiver: %v", err)
	}

	srv := &httpapi.Server{
		Org:      org.New(pool),
		Log:      log,
		Exchange: exchanger,
		Session:  session,
		Admin:    admin,
		Webhook:  wh,
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return &harness{server: ts, pool: pool, stub: stub}
}

func doJSON(t *testing.T, c *http.Client, method, url string, body any, cookies ...*http.Cookie) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp, out
}

func bearer(t *testing.T, c *http.Client, url, jwt string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp, out
}

// TestEndToEndSignupExchangeMeOrgLogout drives the full happy path
// against the real chi router: stub-provider signup -> /api/signup-complete
// -> cookie set -> /api/me -> /api/organization -> /api/logout -> 401.
func TestEndToEndSignupExchangeMeOrgLogout(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	// Pretend the user just completed Clerk SignUp.
	pUser := h.stub.NewUser("owner@acme.test", "Acme Owner")
	jwt, _ := h.stub.NewSession(pUser.ID, "", time.Hour)

	// 1. /api/signup-complete: creates the local org + user + sets cookie.
	resp, body := bearer(t, c, h.server.URL+"/api/signup-complete", jwt, map[string]any{
		"organization_name": "Acme Diving",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("signup-complete: %d %v", resp.StatusCode, body)
	}
	var sessionCookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == auth.SessionCookieName {
			sessionCookie = ck
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected lb_session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("session cookie not HttpOnly")
	}

	// 2. /api/me with cookie.
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/me", nil, sessionCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("me: %d %v", resp.StatusCode, body)
	}
	if body["email"] != "owner@acme.test" || body["role"] != "org_admin" {
		t.Fatalf("me body: %v", body)
	}

	// 3. /api/organization with cookie.
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/organization", nil, sessionCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("organization: %d %v", resp.StatusCode, body)
	}
	if body["name"] != "Acme Diving" {
		t.Fatalf("org name: %v", body["name"])
	}

	// 4. /api/logout.
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/logout", nil, sessionCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("logout: %d", resp.StatusCode)
	}

	// 5. /api/me with the now-revoked cookie -> 401.
	resp, _ = doJSON(t, c, "GET", h.server.URL+"/api/me", nil, sessionCookie)
	if resp.StatusCode != 401 {
		t.Fatalf("post-logout me status: %d, want 401", resp.StatusCode)
	}
}

func TestOrganizationRequiresSession(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/organization", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestExchangeWithoutLocalUserIs401(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	pUser := h.stub.NewUser("nobody@x.test", "Nobody")
	jwt, _ := h.stub.NewSession(pUser.ID, "", time.Hour)

	resp, body := bearer(t, c, h.server.URL+"/api/auth/exchange", jwt, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("status: %d body: %v", resp.StatusCode, body)
	}
	if body["error"] != "membership_pending" {
		t.Errorf("error code: %v", body["error"])
	}
}

func TestInviteRequiresOrgAdmin(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	// Bootstrap an admin via signup-complete.
	adminPUser := h.stub.NewUser("admin@x.test", "Admin")
	adminJWT, _ := h.stub.NewSession(adminPUser.ID, "", time.Hour)
	resp, _ := bearer(t, c, h.server.URL+"/api/signup-complete", adminJWT, map[string]any{
		"organization_name": "Org",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("setup admin: %d", resp.StatusCode)
	}
	adminCookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)

	// Admin can invite.
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email": "site@x.test",
		"role":  "site_director",
	}, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("invite admin path: %d %v", resp.StatusCode, body)
	}

	// Now create a non-admin user (site director) and verify they get 403.
	adminUser, err := h.pool.UserByClerkID(context.Background(), adminPUser.ID)
	if err != nil {
		t.Fatalf("UserByClerkID admin: %v", err)
	}
	directorPUser := h.stub.NewUser("dir@x.test", "Director")
	if _, err := h.pool.CreateExternalUser(context.Background(),
		adminUser.OrganizationID, directorPUser.ID, "dir@x.test", "Director", store.RoleSiteDirector); err != nil {
		t.Fatalf("CreateExternalUser director: %v", err)
	}
	dirJWT, _ := h.stub.NewSession(directorPUser.ID, "", time.Hour)
	// Director exchanges to get a cookie.
	resp, _ = bearer(t, c, h.server.URL+"/api/auth/exchange", dirJWT, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("director exchange: %d", resp.StatusCode)
	}
	dirCookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)

	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email": "another@x.test",
		"role":  "site_director",
	}, dirCookie)
	if resp.StatusCode != 403 {
		t.Fatalf("director invite: %d %v want 403", resp.StatusCode, body)
	}
}

func TestDeactivateUserPath(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	adminPUser := h.stub.NewUser("admin@x.test", "Admin")
	adminJWT, _ := h.stub.NewSession(adminPUser.ID, "", time.Hour)
	resp, _ := bearer(t, c, h.server.URL+"/api/signup-complete", adminJWT, map[string]any{
		"organization_name": "Org",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("setup admin: %d", resp.StatusCode)
	}
	adminCookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)

	adminUser, err := h.pool.UserByClerkID(context.Background(), adminPUser.ID)
	if err != nil {
		t.Fatalf("UserByClerkID admin: %v", err)
	}
	// Create a target user in the same org.
	targetPUser := h.stub.NewUser("target@x.test", "Target")
	target, err := h.pool.CreateExternalUser(context.Background(),
		adminUser.OrganizationID, targetPUser.ID, "target@x.test", "Target", store.RoleSiteDirector)
	if err != nil {
		t.Fatalf("CreateExternalUser target: %v", err)
	}

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/users/"+target.ID.String()+"/deactivate", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("deactivate: %d %v", resp.StatusCode, body)
	}

	got, err := h.pool.UserByID(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if got.IsActive {
		t.Errorf("target should be deactivated")
	}
}

func TestSelfDeactivationForbidden(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	adminPUser := h.stub.NewUser("admin@x.test", "Admin")
	adminJWT, _ := h.stub.NewSession(adminPUser.ID, "", time.Hour)
	resp, _ := bearer(t, c, h.server.URL+"/api/signup-complete", adminJWT, map[string]any{
		"organization_name": "Org",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("setup admin: %d", resp.StatusCode)
	}
	adminCookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	admin, err := h.pool.UserByClerkID(context.Background(), adminPUser.ID)
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/users/"+admin.ID.String()+"/deactivate", nil, adminCookie)
	if resp.StatusCode != 403 {
		t.Fatalf("self-deactivate: %d %v want 403", resp.StatusCode, body)
	}
}

func pickCookieFrom(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return nil
}
