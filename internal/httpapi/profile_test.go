package httpapi_test

import (
	"net/http"
	"testing"
)

func TestUpdateProfileRequiresSession(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	resp, _ := doJSON(t, c, "PATCH", h.server.URL+"/api/account/profile", map[string]any{
		"full_name": "X", "phone": "+1",
	})
	if resp.StatusCode != 401 {
		t.Fatalf("status: %d want 401", resp.StatusCode)
	}
}

func TestUpdateProfileChangesNameAndPhone(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	resp, body := doJSON(t, c, "PATCH", h.server.URL+"/api/account/profile", map[string]any{
		"full_name": "Renamed Admin",
		"phone":     "+1 555 0000",
	}, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("patch: %d %v", resp.StatusCode, body)
	}
	if body["full_name"] != "Renamed Admin" {
		t.Errorf("full_name = %v", body["full_name"])
	}
	if body["phone"] != "+1 555 0000" {
		t.Errorf("phone = %v", body["phone"])
	}

	// /api/me reflects the change.
	resp, body = doJSON(t, c, "GET", h.server.URL+"/api/me", nil, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("me: %d", resp.StatusCode)
	}
	if body["full_name"] != "Renamed Admin" {
		t.Errorf("me.full_name = %v", body["full_name"])
	}
}

func TestUpdateProfileRejectsBlankName(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, body := doJSON(t, c, "PATCH", h.server.URL+"/api/account/profile", map[string]any{
		"full_name": "   ",
		"phone":     "+1",
	}, cookie)
	if resp.StatusCode != 400 {
		t.Fatalf("status: %d %v want 400", resp.StatusCode, body)
	}
}

func TestUpdateProfileRejectsUnknownFields(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, body := doJSON(t, c, "PATCH", h.server.URL+"/api/account/profile", map[string]any{
		"full_name": "Sneaky",
		"role":      "org_admin",
	}, cookie)
	if resp.StatusCode != 400 {
		t.Fatalf("status: %d %v want 400 (unknown field)", resp.StatusCode, body)
	}
}

func TestUpdateProfileClearsPhone(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	// First set a phone.
	resp, _ := doJSON(t, c, "PATCH", h.server.URL+"/api/account/profile", map[string]any{
		"full_name": "Admin",
		"phone":     "+1 555 0000",
	}, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("set phone: %d", resp.StatusCode)
	}

	// Then clear it.
	resp, body := doJSON(t, c, "PATCH", h.server.URL+"/api/account/profile", map[string]any{
		"full_name": "Admin",
		"phone":     nil,
	}, cookie)
	if resp.StatusCode != 200 {
		t.Fatalf("clear phone: %d %v", resp.StatusCode, body)
	}
	if body["phone"] != nil {
		t.Errorf("phone = %v want nil", body["phone"])
	}
}
