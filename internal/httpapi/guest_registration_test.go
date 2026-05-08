package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	tripGuestID, _ := uuid.Parse(body["id"].(string))

	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Ada Guest",
		"email":     "ada.guest@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("staff registration detail: %d %v", resp.StatusCode, body)
	}
	reg, ok := body["registration"].(map[string]any)
	if !ok || reg["status"] != "submitted" {
		t.Fatalf("staff registration detail: %v", body)
	}
}

func TestGuestRegistrationPostSubmitEdits(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, _ := signInAsAdmin(t, h)
	tripID := seedManifestTrip(t, h, org.ID, 2)

	// Add guest and accept invite to mint a guest session.
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Bea Diver",
		"email":     "bea.diver@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	tripGuestID, _ := uuid.Parse(body["id"].(string))

	token := tokenFromLink(t, h.mail.LinkFor("bea.diver@example.test", "guest/invitations"))
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/guest/invitations/"+token+"/accept", map[string]any{
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("accept invite: %d", resp.StatusCode)
	}
	guestCookie := pickCookieFrom(resp.Cookies(), auth.GuestSessionCookieName)

	regURL := h.server.URL + "/api/guest/trip-registrations/" + tripGuestID.String()

	// First submission.
	payload := validGuestRegistrationPayload()
	resp, body = doJSON(t, c, "POST", regURL+"/submit", payload, guestCookie)
	if resp.StatusCode != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("submit: %d %v", resp.StatusCode, body)
	}
	firstSubmittedAt, _ := body["submitted_at"].(string)
	if firstSubmittedAt == "" {
		t.Fatalf("expected submitted_at on first submit, got %v", body)
	}

	// PATCH (draft) is rejected once submitted.
	resp, body = doJSON(t, c, "PATCH", regURL, payload, guestCookie)
	if resp.StatusCode != http.StatusConflict || body["error"] != "already_submitted" {
		t.Fatalf("post-submit PATCH: %d %v want 409 already_submitted", resp.StatusCode, body)
	}

	// Edit a field and re-submit. Spend at least a millisecond so any
	// timestamp drift would be observable.
	time.Sleep(10 * time.Millisecond)
	identity := payload["identity"].(map[string]any)
	identity["preferred_name"] = "Bee"
	resp, body = doJSON(t, c, "POST", regURL+"/submit", payload, guestCookie)
	if resp.StatusCode != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("re-submit: %d %v", resp.StatusCode, body)
	}
	if got, _ := body["submitted_at"].(string); got != firstSubmittedAt {
		t.Fatalf("submitted_at drifted across re-submit: first %s, now %s", firstSubmittedAt, got)
	}

	// Re-submit with an invalidated payload still re-validates.
	identity["legal_name"] = ""
	resp, body = doJSON(t, c, "POST", regURL+"/submit", payload, guestCookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("re-submit invalid: %d %v want 400", resp.StatusCode, body)
	}

	// Manifest still reports submitted_at from the first submission.
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/manifest", nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manifest: %d %v", resp.StatusCode, body)
	}
	guests := body["guests"].([]any)
	if len(guests) != 1 {
		t.Fatalf("manifest guests: %v", guests)
	}
	if got, _ := guests[0].(map[string]any)["registration_submitted_at"].(string); got != firstSubmittedAt {
		t.Fatalf("manifest registration_submitted_at drifted: first %s, now %s", firstSubmittedAt, got)
	}
}

func TestStaffRegistrationDetailExposesDraftAndTripGuest(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, _ := signInAsAdmin(t, h)
	tripID := seedManifestTrip(t, h, org.ID, 1)

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Carl Diver",
		"email":     "carl.diver@example.test",
		"berth_id":  nextBerthForTrip(t, h, tripID).String(),
	}, adminCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest: %d %v", resp.StatusCode, body)
	}
	tripGuestID, _ := uuid.Parse(body["id"].(string))
	staffURL := h.server.URL + "/api/admin/trips/" + tripID.String() + "/guests/" + tripGuestID.String() + "/registration"

	// Before any guest activity: registration is null but trip_guest is present.
	resp, body = doJSON(t, c, "GET", staffURL, nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("staff registration (no guest activity): %d %v", resp.StatusCode, body)
	}
	if body["registration"] != nil {
		t.Fatalf("expected nil registration, got %v", body["registration"])
	}
	tg, ok := body["trip_guest"].(map[string]any)
	if !ok || tg["full_name"] != "Carl Diver" {
		t.Fatalf("staff trip_guest block: %v", body["trip_guest"])
	}

	// Guest accepts and saves a draft.
	token := tokenFromLink(t, h.mail.LinkFor("carl.diver@example.test", "guest/invitations"))
	resp, _ = doJSON(t, c, "POST", h.server.URL+"/api/guest/invitations/"+token+"/accept", map[string]any{
		"password": "Sup3rStrong!",
	})
	guestCookie := pickCookieFrom(resp.Cookies(), auth.GuestSessionCookieName)
	regURL := h.server.URL + "/api/guest/trip-registrations/" + tripGuestID.String()
	resp, _ = doJSON(t, c, "PATCH", regURL, map[string]any{
		"identity": map[string]any{"legal_name": "Carl Diver"},
	}, guestCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save draft: %d", resp.StatusCode)
	}

	// Staff sees the draft.
	resp, body = doJSON(t, c, "GET", staffURL, nil, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("staff registration (draft): %d %v", resp.StatusCode, body)
	}
	reg, ok := body["registration"].(map[string]any)
	if !ok || reg["status"] != "draft" {
		t.Fatalf("expected draft registration, got %v", body["registration"])
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
			"berth_id":  nextBerthForTrip(t, h, tripID).String(),
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
	seedBoatCabins(t, h, orgID, boatID, 8)
	return tripID
}

func seedBoatCabins(t *testing.T, h *harness, orgID, boatID uuid.UUID, count int) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= count; i++ {
		var cabinID uuid.UUID
		if err := h.pool.QueryRow(ctx, `
			INSERT INTO boat_cabins (organization_id, boat_id, label, sort_order)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`, orgID, boatID, strconv.Itoa(i), i*10).Scan(&cabinID); err != nil {
			t.Fatalf("insert cabin: %v", err)
		}
		for j, berth := range []string{"A", "B"} {
			if _, err := h.pool.Exec(ctx, `
				INSERT INTO boat_cabin_berths (organization_id, boat_id, cabin_id, berth_label, display_label, sort_order)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, orgID, boatID, cabinID, berth, fmt.Sprintf("%d%s", i, berth), i*10+j); err != nil {
				t.Fatalf("insert berth: %v", err)
			}
		}
	}
}

func nextBerthForTrip(t *testing.T, h *harness, tripID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := h.pool.QueryRow(context.Background(), `
		SELECT b.id
		FROM trips t
		JOIN boat_cabin_berths b ON b.boat_id = t.boat_id
		WHERE t.id = $1 AND b.is_active
		  AND NOT EXISTS (
		    SELECT 1 FROM trip_cabin_assignments a
		    WHERE a.trip_id = t.id AND a.berth_id = b.id AND a.unassigned_at IS NULL
		  )
		ORDER BY b.sort_order, b.display_label
		LIMIT 1
	`, tripID).Scan(&id); err != nil {
		t.Fatalf("nextBerthForTrip: %v", err)
	}
	return id
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
