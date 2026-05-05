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
	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/httpapi"
	"github.com/karimfan/liveaboard/internal/imports"
	"github.com/karimfan/liveaboard/internal/org"
	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

// Harness for the Sprint 009 router. Replaces the Clerk-stub harness with
// a real auth.Service backed by a MockSender so we can read tokens out of
// captured emails without a live SMTP relay.

type harness struct {
	server *httptest.Server
	pool   *store.Pool
	auth   *auth.Service
	mail   *email.MockSender
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testdb.Pool(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	mock := &email.MockSender{}

	svc := auth.New(pool, mock, log, "http://localhost:5173", "Liveaboard <noreply@x.test>")
	svc.BcryptCost = 4
	svc.SessionDuration = time.Hour

	session := &auth.SessionMiddleware{Store: pool, Log: log}

	// Sprint 012 — wire a Runner with no real network. Tests don't
	// exercise the goroutine path; this just keeps Kick() from
	// dereferencing a nil Runner.
	runner := &imports.Runner{Store: pool, Log: log, Months: 1}
	srv := &httpapi.Server{
		Org:          org.New(pool),
		Log:          log,
		Auth:         svc,
		Session:      session,
		AdminAPI:     &httpapi.AdminHandlers{Store: pool},
		ImportRunner: runner,
		CookieSecure: false,
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return &harness{server: ts, pool: pool, auth: svc, mail: mock}
}

// doJSON sends a JSON request, returns the response + decoded body.
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

func pickCookieFrom(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// signupAndVerify drives the full bootstrap-an-org-admin flow against the
// HTTP surface and returns a logged-in session cookie.
func signupAndVerify(t *testing.T, h *harness, c *http.Client, orgName, email, fullName, password string) *http.Cookie {
	t.Helper()
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/auth/signup", map[string]any{
		"email":             email,
		"password":          password,
		"full_name":         fullName,
		"organization_name": orgName,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("signup: %d %v", resp.StatusCode, body)
	}
	// Pull the verification token out of the user's row in the DB. The
	// service emails it but we only know the hash; reusing the service's
	// internal helper would be cleaner, but for tests we extract from the
	// MockSender capture.
	link := h.mail.LinkFor(email, "verify-email")
	if link == "" {
		t.Fatalf("no verification link captured: %+v", h.mail.Messages)
	}
	tok := tokenFromLink(t, link)
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/auth/verify-email", map[string]any{
		"token": tok,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("verify-email: %d %v", resp.StatusCode, body)
	}
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/auth/login", map[string]any{
		"email":    email,
		"password": password,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("login: %d %v", resp.StatusCode, body)
	}
	ck := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	if ck == nil {
		t.Fatalf("login did not set %s cookie", auth.SessionCookieName)
	}
	return ck
}

// tokenFromLink extracts ?token=... or trailing /<token> from a URL.
func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	if i := indexOf(link, "token="); i >= 0 {
		return link[i+len("token="):]
	}
	// /invitations/<token>/accept
	if i := indexOf(link, "/invitations/"); i >= 0 {
		rest := link[i+len("/invitations/"):]
		if j := indexOf(rest, "/"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	t.Fatalf("could not extract token from %q", link)
	return ""
}

func indexOf(s, sub string) int {
	return bytesIndex([]byte(s), []byte(sub))
}

// bytesIndex avoids pulling in strings just for this.
func bytesIndex(haystack, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
outer:
	for i := 0; i+len(needle) <= len(haystack); i++ {
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

// --- tests ---

// TestEndToEndSignupVerifyLoginMeOrgLogout drives the canonical happy path.
func TestEndToEndSignupVerifyLoginMeOrgLogout(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	cookie := signupAndVerify(t, h, c, "Acme Diving", "owner@acme.test", "Acme Owner", "Sup3rStrong!")

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/me", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("me: %d %v", resp.StatusCode, body)
	}
	if body["email"] != "owner@acme.test" || body["role"] != string(store.RoleOrgAdmin) {
		t.Fatalf("me body: %v", body)
	}
	if body["email_verified"] != true {
		t.Errorf("expected email_verified=true, got %v", body["email_verified"])
	}

	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/organization", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("organization: %d %v", resp.StatusCode, body)
	}
	if body["name"] != "Acme Diving" {
		t.Fatalf("org name: %v", body["name"])
	}

	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/auth/logout", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	resp, _ = doJSON(t, c, "GET", h.server.URL+"/api/me", nil, cookie)
	if resp.StatusCode != 401 {
		t.Fatalf("post-logout me: %d want 401", resp.StatusCode)
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

func TestLoginUnverifiedReturnsVerificationRequired(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	// Signup without verifying.
	resp, _ := doJSON(t, c, "POST", h.server.URL+"/api/auth/signup", map[string]any{
		"email":             "owner@x.test",
		"password":          "Sup3rStrong!",
		"full_name":         "Owner",
		"organization_name": "Org",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("signup: %d", resp.StatusCode)
	}
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/auth/login", map[string]any{
		"email": "owner@x.test", "password": "Sup3rStrong!",
	})
	if resp.StatusCode != 403 {
		t.Fatalf("login unverified: %d body %v want 403", resp.StatusCode, body)
	}
	if body["error"] != "verification_required" {
		t.Errorf("error code: %v", body["error"])
	}
}

func TestInviteRequiresOrgAdmin(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	// Bootstrap admin.
	adminCookie := signupAndVerify(t, h, c, "Acme", "admin@x.test", "Admin", "Sup3rStrong!")

	// Admin can invite.
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email":     "site@x.test",
		"full_name": "Maya Sanchez",
		"role":      "cruise_director",
	}, adminCookie)
	if resp.StatusCode != 201 {
		t.Fatalf("admin invite: %d %v", resp.StatusCode, body)
	}

	// Pull the invitation accept link, accept it, get a director cookie.
	link := h.mail.LinkFor("site@x.test", "/invitations/")
	if link == "" {
		t.Fatalf("no invitation link captured: %+v", h.mail.Messages)
	}
	tok := tokenFromLink(t, link)
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/invitations/accept", map[string]any{
		"token":    tok,
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("accept: %d %v", resp.StatusCode, body)
	}
	dirCookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	if dirCookie == nil {
		t.Fatalf("accept didn't set cookie")
	}

	// Director CANNOT invite.
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email":     "another@x.test",
		"full_name": "Other",
		"role":      "cruise_director",
	}, dirCookie)
	if resp.StatusCode != 403 {
		t.Fatalf("director invite: %d %v want 403", resp.StatusCode, body)
	}
}

func TestForgotPasswordSilentlyAcceptsUnknownEmail(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/auth/forgot-password", map[string]any{
		"email": "nobody@x.test",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("forgot-password unknown: %d %v want 200", resp.StatusCode, body)
	}
}

func TestResetPasswordRotatesAndLogsIn(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	signupAndVerify(t, h, c, "Acme", "owner@x.test", "Owner", "Sup3rStrong!")

	// Trigger forgot password.
	resp, _ := doJSON(t, c, "POST", h.server.URL+"/api/auth/forgot-password", map[string]any{
		"email": "owner@x.test",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("forgot: %d", resp.StatusCode)
	}
	link := h.mail.LinkFor("owner@x.test", "reset-password")
	if link == "" {
		t.Fatalf("no reset link: %+v", h.mail.Messages)
	}
	tok := tokenFromLink(t, link)

	// Reset to a new password — should set a fresh cookie.
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/auth/reset-password", map[string]any{
		"token":        tok,
		"new_password": "Different1!",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("reset: %d %v", resp.StatusCode, body)
	}
	freshCookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	if freshCookie == nil {
		t.Fatalf("reset didn't set cookie")
	}

	// New cookie works against /api/me.
	resp, _ = doJSON(t, c, "GET", h.server.URL+"/api/me", nil, freshCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("post-reset me: %d", resp.StatusCode)
	}

	// Old password no longer works.
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/auth/login", map[string]any{
		"email": "owner@x.test", "password": "Sup3rStrong!",
	})
	if resp.StatusCode != 401 {
		t.Fatalf("old-password login: %d want 401", resp.StatusCode)
	}
}

// silence unused-import warning when context isn't referenced in an
// occasional refactor. (Keeps the import line stable.)
var _ = context.Background
