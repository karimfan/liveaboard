package httpapi_test

import (
	"net/http"
	"testing"
)

// These tests live at the HTTP layer to prove the per-endpoint
// authorization contract. Cross-tenant data isolation is tested in
// internal/store/reports_test.go against the store directly; here we
// confirm the handlers refuse unauth, wrong-role, and assignment-less
// callers with the right status codes.

func TestAdminReportsRequiresAuth(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/admin/reports", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}

func TestAdminReportsRequiresOrgAdmin(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	// Bootstrap admin and a director, log in as director.
	adminCookie := signupAndVerify(t, h, c, "Acme", "admin@x.test", "Admin", "Sup3rStrong!")
	// Admin invites a director.
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email":     "dir@x.test",
		"full_name": "Director Dee",
		"role":      "cruise_director",
	}, adminCookie)
	if resp.StatusCode != 201 {
		t.Fatalf("invite director: %d %v", resp.StatusCode, body)
	}
	link := h.mail.LinkFor("dir@x.test", "/invitations/")
	tok := tokenFromLink(t, link)
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/invitations/accept", map[string]any{
		"token":    tok,
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("accept director: %d %v", resp.StatusCode, body)
	}
	dirCookie := pickCookieFrom(resp.Cookies(), "lb_session")
	if dirCookie == nil {
		t.Fatal("no director cookie")
	}

	// Director cannot read /api/admin/reports.
	resp, _ = doJSON(t, c, "GET", h.server.URL+"/api/admin/reports", nil, dirCookie)
	if resp.StatusCode != 403 {
		t.Errorf("director hit admin reports: %d want 403", resp.StatusCode)
	}

	// Admin can.
	resp, _ = doJSON(t, c, "GET", h.server.URL+"/api/admin/reports", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Errorf("admin reports: %d want 200", resp.StatusCode)
	}
}

func TestAdminReportsRejectsWideWindow(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie := signupAndVerify(t, h, c, "Acme", "admin@x.test", "Admin", "Sup3rStrong!")

	// 5-year window — definitely past the 1-year cap.
	url := h.server.URL + "/api/admin/reports?from=2020-01-01&to=2025-12-31"
	resp, body := doJSON(t, c, "GET", url, nil, cookie)
	if resp.StatusCode != 400 {
		t.Fatalf("status=%d want 400 body=%v", resp.StatusCode, body)
	}
	if body["error"] != "window_too_wide" {
		t.Errorf("error code=%v want window_too_wide", body["error"])
	}
}

func TestAdminReportsRejectsBadDate(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie := signupAndVerify(t, h, c, "Acme", "admin@x.test", "Admin", "Sup3rStrong!")

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/reports?from=not-a-date", nil, cookie)
	if resp.StatusCode != 400 {
		t.Fatalf("status=%d want 400 body=%v", resp.StatusCode, body)
	}
}

func TestGuestTabRequiresGuestSession(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}

	// No cookie at all.
	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/guest/trip-registrations/00000000-0000-0000-0000-000000000000/tab", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("no-session status=%d want 401", resp.StatusCode)
	}
}
