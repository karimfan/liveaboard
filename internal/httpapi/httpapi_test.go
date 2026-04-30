package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	p := testdb.Pool(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv := &httpapi.Server{
		Auth: auth.New(p, log),
		Org:  org.New(p),
		Log:  log,
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
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

func TestSignupVerifyLoginDashboardLogoutFlow(t *testing.T) {
	ts := newTestServer(t)
	c := &http.Client{}

	// 1. signup
	resp, body := doJSON(t, c, "POST", ts.URL+"/api/signup", map[string]any{
		"email":             "owner@acme.test",
		"password":          "Password1",
		"full_name":         "Acme Owner",
		"organization_name": "Acme Diving",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("signup status: %d body=%v", resp.StatusCode, body)
	}
	token, _ := body["verification_token"].(string)
	if token == "" {
		t.Fatal("missing verification_token")
	}

	// 2. verify-email
	resp, body = doJSON(t, c, "POST", ts.URL+"/api/verify-email", map[string]any{"token": token})
	if resp.StatusCode != 200 {
		t.Fatalf("verify status: %d body=%v", resp.StatusCode, body)
	}

	// 3. login
	resp, body = doJSON(t, c, "POST", ts.URL+"/api/login", map[string]any{
		"email":    "owner@acme.test",
		"password": "Password1",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("login status: %d body=%v", resp.StatusCode, body)
	}
	var sessionCookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "lb_session" {
			sessionCookie = ck
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("login did not set lb_session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("session cookie not HttpOnly")
	}

	// 4. /api/organization with cookie
	resp, body = doJSON(t, c, "GET", ts.URL+"/api/organization", nil, sessionCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("organization status: %d body=%v", resp.StatusCode, body)
	}
	if body["name"] != "Acme Diving" {
		t.Fatalf("org name = %v", body["name"])
	}
	stats, ok := body["stats"].(map[string]any)
	if !ok {
		t.Fatalf("missing stats: %v", body)
	}
	for _, key := range []string{"boats", "active_trips", "total_guests"} {
		if v, _ := stats[key].(float64); v != 0 {
			t.Errorf("stats.%s = %v, want 0", key, v)
		}
	}

	// 5. /api/me with cookie
	resp, body = doJSON(t, c, "GET", ts.URL+"/api/me", nil, sessionCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("me status: %d body=%v", resp.StatusCode, body)
	}
	if body["email"] != "owner@acme.test" || body["role"] != "org_admin" {
		t.Fatalf("me body = %v", body)
	}

	// 6. logout
	resp, _ = doJSON(t, c, "POST", ts.URL+"/api/logout", nil, sessionCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("logout status: %d", resp.StatusCode)
	}

	// 7. /api/organization with old cookie -> 401
	resp, _ = doJSON(t, c, "GET", ts.URL+"/api/organization", nil, sessionCookie)
	if resp.StatusCode != 401 {
		t.Fatalf("post-logout status: %d, want 401", resp.StatusCode)
	}
}

func TestLoginInvalidCredentialsIsGeneric(t *testing.T) {
	ts := newTestServer(t)
	c := &http.Client{}
	resp, body := doJSON(t, c, "POST", ts.URL+"/api/login", map[string]any{
		"email":    "nobody@nowhere.test",
		"password": "Password1",
	})
	if resp.StatusCode != 401 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "invalid email or password") {
		t.Fatalf("message: %q", msg)
	}
}

func TestOrganizationRequiresSession(t *testing.T) {
	ts := newTestServer(t)
	c := &http.Client{}
	resp, _ := doJSON(t, c, "GET", ts.URL+"/api/organization", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestSignupDuplicateEmailReturns200(t *testing.T) {
	// Non-enumerating: a duplicate signup must not reveal the email is taken.
	// Implementation choice: respond 200 ok-shape, send no verification token
	// (since none was actually issued).
	ts := newTestServer(t)
	c := &http.Client{}
	body := map[string]any{
		"email":             "owner@acme.test",
		"password":          "Password1",
		"full_name":         "Acme Owner",
		"organization_name": "Acme Diving",
	}
	if resp, _ := doJSON(t, c, "POST", ts.URL+"/api/signup", body); resp.StatusCode != 200 {
		t.Fatalf("first status: %d", resp.StatusCode)
	}
	resp, out := doJSON(t, c, "POST", ts.URL+"/api/signup", body)
	if resp.StatusCode != 200 {
		t.Fatalf("dup status: %d body=%v", resp.StatusCode, out)
	}
	if _, ok := out["verification_token"]; ok {
		t.Fatalf("duplicate signup leaked verification_token: %v", out)
	}
}
