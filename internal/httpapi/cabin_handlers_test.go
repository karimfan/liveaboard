package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCabinLayoutAssignmentAndRevokeFlow(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	adminCookie, org, admin := signInAsAdmin(t, h)
	dirCookie, director := bootstrapDirector(t, h, org.ID)
	tripID := seedManifestTrip(t, h, org.ID, 1)
	boatID := boatIDForTrip(t, h, tripID)

	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/admin/boats/"+boatID.String()+"/cabins", nil, dirCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unassigned director boat layout: %d want 403", resp.StatusCode)
	}
	if _, err := h.pool.AssignCruiseDirector(context.Background(), tripID, director.ID, &admin.ID); err != nil {
		t.Fatalf("AssignCruiseDirector: %v", err)
	}

	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/admin/boats/"+boatID.String()+"/cabins/preview", map[string]any{
		"source": "paste",
		"paste":  "10,A,B\n11,A",
	}, dirCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview: %d %v", resp.StatusCode, body)
	}

	resp, body = doJSON(t, c, "PUT", h.server.URL+"/api/admin/boats/"+boatID.String()+"/cabins", map[string]any{
		"source": "csv",
		"csv":    "cabin_label,berth_label,deck,sort_order,notes\n10,A,Main,10,\n10,B,Main,11,\n11,A,Main,20,\n",
	}, dirCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replace layout: %d %v", resp.StatusCode, body)
	}
	if body["active_berth_count"] != float64(3) {
		t.Fatalf("layout count: %v", body)
	}

	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "No Cabin",
		"email":     "no-cabin@example.test",
	}, adminCookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("add guest without berth: %d %v want 400", resp.StatusCode, body)
	}

	berthID := nextBerthForTrip(t, h, tripID)
	resp, body = doJSON(t, c, "POST", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests", map[string]any{
		"full_name": "Cabin Guest",
		"email":     "cabin@example.test",
		"berth_id":  berthID.String(),
	}, dirCookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add guest with berth: %d %v", resp.StatusCode, body)
	}
	guestID, _ := uuid.Parse(body["id"].(string))
	if body["cabin_assignment"] == nil {
		t.Fatalf("missing cabin assignment in manifest row: %v", body)
	}

	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/admin/trips/"+tripID.String()+"/cabins", nil, dirCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cabin board: %d %v", resp.StatusCode, body)
	}
	cabins := body["cabins"].([]any)
	if len(cabins) == 0 {
		t.Fatalf("empty cabin board: %v", body)
	}

	resp, body = doJSON(t, c, "DELETE", h.server.URL+"/api/admin/trips/"+tripID.String()+"/guests/"+guestID.String()+"/invite", nil, dirCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke: %d %v", resp.StatusCode, body)
	}
	var active int
	if err := h.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM trip_cabin_assignments
		WHERE trip_guest_id = $1 AND unassigned_at IS NULL
	`, guestID).Scan(&active); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if active != 0 {
		t.Fatalf("active assignments after revoke = %d want 0", active)
	}
}
