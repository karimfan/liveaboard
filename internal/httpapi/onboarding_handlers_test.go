package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
)

func parseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return id
}

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

// TestAssignCruiseDirectorRequiresBoatLayout exercises the Sprint 023
// gate: the assign-director endpoint refuses with
// "boat_layout_required" + boat_id when the trip's boat has no
// usable cabin layout.
func TestAssignCruiseDirectorRequiresBoatLayout(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie := signupAndVerify(t, h, c, "Acme", "owner@x.test", "Owner", "Sup3rStrong!")

	// Get the admin's org id by hitting /api/me.
	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/me", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("me: %d %v", resp.StatusCode, body)
	}
	orgIDStr, _ := body["organization_id"].(string)
	if orgIDStr == "" {
		t.Fatalf("no organization_id in /api/me: %v", body)
	}

	ctx := context.Background()
	// Insert a boat directly (no cabin layout) and a trip on it.
	boat, err := h.pool.UpsertBoat(ctx, parseUUID(t, orgIDStr), "liveaboard.com",
		store.BoatScrape{Slug: "no-layout", Name: "No Layout", URL: "https://x/no-layout"},
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var tripID string
	if err := h.pool.QueryRow(ctx, `
		INSERT INTO trips (organization_id, boat_id, start_date, end_date, itinerary,
		                   source_provider, source_trip_key, source_url, source_last_synced_at, status)
		VALUES ($1, $2, current_date + 7, current_date + 14, 'Test',
		        'liveaboard.com', 'k1', 'https://x/t1', now(), 'planned')
		RETURNING id
	`, orgIDStr, boat.ID).Scan(&tripID); err != nil {
		t.Fatal(err)
	}

	// Invite a director (real user we can assign).
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/invitations", map[string]any{
		"email":     "dir@x.test",
		"full_name": "Dir Dee",
		"role":      "cruise_director",
	}, adminCookie)
	if resp.StatusCode != 201 {
		t.Fatalf("invite: %d %v", resp.StatusCode, body)
	}
	tok := tokenFromLink(t, h.mail.LinkFor("dir@x.test", "/invitations/"))
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/invitations/accept", map[string]any{
		"token":    tok,
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("accept: %d", resp.StatusCode)
	}
	// Pull the director's user id.
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/users", nil, adminCookie)
	if resp.StatusCode != 200 {
		t.Fatalf("users list: %d", resp.StatusCode)
	}
	users, _ := body["users"].([]any)
	var dirID string
	for _, u := range users {
		row, _ := u.(map[string]any)
		if row["email"] == "dir@x.test" {
			dirID, _ = row["id"].(string)
		}
	}
	if dirID == "" {
		t.Fatal("director id not found")
	}

	// Try to assign — should 409 with boat_layout_required + boat_id.
	resp, body = doJSON(t, c, "POST",
		h.server.URL+"/api/admin/trips/"+tripID+"/cruise-directors",
		map[string]any{"user_id": dirID}, adminCookie)
	if resp.StatusCode != 409 {
		t.Fatalf("assign without layout=%d %v want 409", resp.StatusCode, body)
	}
	if body["error"] != "boat_layout_required" {
		t.Errorf("error code=%v want boat_layout_required", body["error"])
	}
	if body["boat_id"] != boat.ID.String() {
		t.Errorf("boat_id=%v want %v", body["boat_id"], boat.ID)
	}
	if body["boat_name"] != "No Layout" {
		t.Errorf("boat_name=%v want 'No Layout'", body["boat_name"])
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
