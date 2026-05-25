package httpapi_test

import (
	"net/http"
	"testing"
)

func TestOnboardingRequiresAuth(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/admin/onboarding", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("GET no-session=%d want 401", resp.StatusCode)
	}
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/admin/onboarding/dismiss", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("POST no-session=%d want 401", resp.StatusCode)
	}
}

func TestOnboardingRequiresOrgAdmin(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	adminCookie := signupAndVerify(t, h, c, "Acme", "admin@x.test", "Admin", "Sup3rStrong!")

	// Invite + log in as a director.
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email":     "dir@x.test",
		"full_name": "Director Dee",
		"role":      "cruise_director",
	}, adminCookie)
	if resp.StatusCode != 201 {
		t.Fatalf("invite director: %d %v", resp.StatusCode, body)
	}
	tok := tokenFromLink(t, h.mail.LinkFor("dir@x.test", "/invitations/"))
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/invitations/accept", map[string]any{
		"token":    tok,
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("accept director: %d", resp.StatusCode)
	}
	dirCookie := pickCookieFrom(resp.Cookies(), "lb_session")
	if dirCookie == nil {
		t.Fatal("no director cookie")
	}

	// Director cannot read or dismiss onboarding.
	resp, _ = doJSON(t, c, "GET", h.server.URL+"/api/admin/onboarding", nil, dirCookie)
	if resp.StatusCode != 403 {
		t.Errorf("director GET onboarding=%d want 403", resp.StatusCode)
	}
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/admin/onboarding/dismiss", nil, dirCookie)
	if resp.StatusCode != 403 {
		t.Errorf("director POST dismiss=%d want 403", resp.StatusCode)
	}

	// Admin can. Default response for a brand-new org: not complete,
	// not dismissed, four wizard steps all not-done.
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/onboarding", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("admin GET=%d %v", resp.StatusCode, body)
	}
	if body["onboarding_complete"] != false {
		t.Errorf("onboarding_complete=%v want false", body["onboarding_complete"])
	}
	if body["dismissed_at"] != nil {
		t.Errorf("dismissed_at=%v want nil", body["dismissed_at"])
	}
	steps, _ := body["steps"].([]any)
	if len(steps) != 4 {
		t.Errorf("steps len=%d want 4", len(steps))
	}
}

func TestOnboardingDismissIsIdempotent(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie := signupAndVerify(t, h, c, "Acme", "admin@x.test", "Admin", "Sup3rStrong!")

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/onboarding/dismiss", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("first dismiss=%d %v", resp.StatusCode, body)
	}
	first := body["dismissed_at"]
	if first == nil {
		t.Fatal("expected dismissed_at after first call")
	}

	// Second call — same timestamp, no error.
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/onboarding/dismiss", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("second dismiss=%d %v", resp.StatusCode, body)
	}
	if body["dismissed_at"] != first {
		t.Errorf("dismissed_at changed on repeat call: %v -> %v", first, body["dismissed_at"])
	}
}
