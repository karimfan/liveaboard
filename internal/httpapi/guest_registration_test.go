package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
)

func TestGuestRegistrationHappyPathAndDraftReturn(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, _ := signInAsAdmin(t, h)
	tripID := seedManifestTrip(t, h, org.ID, 1)

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Ada Guest",
		"email":     "ada.guest@example.test",
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	tripGuestID, _ := uuid.Parse(body["id"].(string))

	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Ada Guest",
		"email":     "ada.guest@example.test",
	}, adminCookie)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate add: %d want 409", resp.StatusCode)
	}

	token := tokenFromLink(t, h.mail.LinkFor("ada.guest@example.test", "guest/invitations"))
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/guest/invitations/"+token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("lookup invite: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/guest/invitations/"+token+"/accept", map[string]any{
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("accept invite: %d %v", resp.StatusCode, body)
	}
	guestCookie := pickCookieFrom(resp.Cookies(), auth.GuestSessionCookieName)
	if guestCookie == nil {
		t.Fatalf("accept did not set guest cookie")
	}

	resp, _ = doJSON(t, c, "GET", h.server.URL+"/api/me", nil, guestCookie)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("guest cookie reached staff /api/me: %d want 401", resp.StatusCode)
	}

	regURL := h.server.URL + "/api/guest/trip-registrations/" + tripGuestID.String()
	resp, body = doJSON(t, c, "PATCH", regURL, map[string]any{
		"identity": map[string]any{"legal_name": "Ada Guest"},
	}, guestCookie)
	if resp.StatusCode != http.StatusOK || body["status"] != "draft" {
		t.Fatalf("save draft: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSON(t, c, "GET", regURL, nil, guestCookie)
	if resp.StatusCode != http.StatusOK || body["status"] != "draft" {
		t.Fatalf("return to draft: %d %v", resp.StatusCode, body)
	}

	resp, _ = doJSON(t, c, "POST", regURL+"/submit", map[string]any{
		"identity": map[string]any{"legal_name": "Ada Guest"},
	}, guestCookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid submit: %d want 400", resp.StatusCode)
	}

	resp, body = doJSON(t, c, "POST", regURL+"/submit", validGuestRegistrationPayload(), guestCookie)
	if resp.StatusCode != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("submit: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/manifest", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manifest: %d %v", resp.StatusCode, body)
	}
	summary := body["summary"].(map[string]any)
	if summary["guest_count"] != float64(1) || summary["submitted_count"] != float64(1) || summary["has_warning"] != false {
		t.Fatalf("summary: %v", summary)
	}

	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+tripGuestID.String()+"/registration", nil, adminCookie)
	if resp.StatusCode != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("staff registration detail: %d %v", resp.StatusCode, body)
	}
}

func TestGuestManifestDirectorScopingAndExpectedWarning(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, admin := signInAsAdmin(t, h)
	dirCookie, director := bootstrapDirector(t, h, org.ID)
	tripID := seedManifestTrip(t, h, org.ID, 1)

	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/manifest", nil, dirCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unassigned director manifest: %d want 403", resp.StatusCode)
	}
	if _, err := h.pool.AssignCruiseDirector(context.Background(), tripID, director.ID, &admin.ID); err != nil {
		t.Fatalf("AssignCruiseDirector: %v", err)
	}

	for _, guest := range []string{"one@example.test", "two@example.test"} {
		resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
			"full_name": "Trip Guest",
			"email":     guest,
		}, dirCookie)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("director add guest %s: %d %v", guest, resp.StatusCode, body)
		}
	}

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/manifest", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manifest: %d %v", resp.StatusCode, body)
	}
	summary := body["summary"].(map[string]any)
	if summary["guest_count"] != float64(2) || summary["expected_count"] != float64(1) || summary["has_warning"] != true {
		t.Fatalf("expected-count warning summary: %v", summary)
	}
}

func seedManifestTrip(t *testing.T, h *harness, orgID uuid.UUID, expectedGuests int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var boatID uuid.UUID
	if err := h.pool.QueryRow(ctx, `
		INSERT INTO boats (
			organization_id, display_name, source_slug, source_name, source_url, source_last_synced_at
		) VALUES ($1, 'Gaia Love', $2, 'Gaia Love', 'https://example.test/boat', $3)
		RETURNING id
	`, orgID, uuid.NewString(), time.Now().UTC()).Scan(&boatID); err != nil {
		t.Fatalf("insert boat: %v", err)
	}
	var tripID uuid.UUID
	if err := h.pool.QueryRow(ctx, `
		INSERT INTO trips (
			organization_id, boat_id, start_date, end_date, itinerary,
			departure_port, return_port, price_text, availability_text,
			num_guests, source_trip_key, source_url, source_last_synced_at
		) VALUES (
			$1, $2, '2026-08-01', '2026-08-08', 'Generic Coral Route',
			'Port A', 'Port A', '$5,000', 'AVAILABLE',
			$3, $4, 'https://example.test/trip', $5
		)
		RETURNING id
	`, orgID, boatID, expectedGuests, uuid.NewString(), time.Now().UTC()).Scan(&tripID); err != nil {
		t.Fatalf("insert trip: %v", err)
	}
	return tripID
}

func validGuestRegistrationPayload() map[string]any {
	return map[string]any{
		"identity": map[string]any{
			"legal_name":    "Ada Guest",
			"date_of_birth": "1988-01-02",
			"nationality":   "US",
		},
		"emergency_contact": map[string]any{
			"name":  "Grace Contact",
			"phone": "+1 555 0100",
		},
		"dive_profile": map[string]any{
			"certification_agency": "PADI",
			"certification_level":  "Advanced Open Water",
			"logged_dives":         42,
		},
		"dietary": map[string]any{
			"no_dietary_or_allergy_notes": true,
		},
	}
}
