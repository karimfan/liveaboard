package httpapi_test

import (
	"net/http"
	"testing"
)

// /api/admin/cruise-director-overview is mounted inside the
// authenticated /api/admin group but enforces a cruise_director role
// check inside the handler. These tests cover the four states the
// handler can reach: 401 unauthenticated, 403 as admin, 200 as a
// director with no trips, 200 as a director with trips.

func TestCruiseDirectorOverviewUnauthenticated(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/admin/cruise-director-overview", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("unauth: %d want 401", resp.StatusCode)
	}
}

func TestCruiseDirectorOverviewAdminIs403(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/cruise-director-overview", nil, cookie)
	if resp.StatusCode != 403 {
		t.Fatalf("admin: %d %v want 403", resp.StatusCode, body)
	}
}

func TestCruiseDirectorOverviewEmpty(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	_, org, _ := signInAsAdmin(t, h)
	dirCookie, _ := bootstrapDirector(t, h, org.ID)

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/cruise-director-overview", nil, dirCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("director: %d %v want 200", resp.StatusCode, body)
	}
	profile, ok := body["profile"].(map[string]any)
	if !ok {
		t.Fatalf("profile missing: %v", body)
	}
	if profile["role"] != "cruise_director" {
		t.Errorf("role = %v want cruise_director", profile["role"])
	}
	if profile["organization_name"] != org.Name {
		t.Errorf("organization_name = %v want %q", profile["organization_name"], org.Name)
	}
	stats, ok := body["stats"].(map[string]any)
	if !ok {
		t.Fatalf("stats missing: %v", body)
	}
	for _, k := range []string{"upcoming", "active", "past"} {
		if v, ok := stats[k]; !ok || v.(float64) != 0 {
			t.Errorf("stats.%s = %v want 0", k, v)
		}
	}
	trips, ok := body["trips"].([]any)
	if !ok {
		t.Fatalf("trips missing: %v", body)
	}
	if len(trips) != 0 {
		t.Errorf("len(trips) = %d want 0", len(trips))
	}
}
