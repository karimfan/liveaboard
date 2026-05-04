package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// signInAsAdmin bootstraps an org via /api/signup-complete and returns
// the admin's session cookie + the org/user it created.
func signInAsAdmin(t *testing.T, h *harness) (*http.Cookie, *store.Organization, *store.User) {
	t.Helper()
	c := &http.Client{}
	pUser := h.stub.NewUser("admin@x.test", "Admin")
	jwt, _ := h.stub.NewSession(pUser.ID, "", time.Hour)
	resp, body := bearer(t, c, h.server.URL+"/api/signup-complete", jwt, map[string]any{
		"organization_name": "Acme Diving",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("signup-complete: %d %v", resp.StatusCode, body)
	}
	cookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	if cookie == nil {
		t.Fatalf("no lb_session cookie")
	}
	user, err := h.pool.UserByClerkID(context.Background(), pUser.ID)
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	org, err := h.pool.OrganizationByID(context.Background(), user.OrganizationID)
	if err != nil {
		t.Fatalf("OrganizationByID: %v", err)
	}
	return cookie, org, user
}

// bootstrapDirector creates a Site Director user in the same org and
// returns their session cookie + user row.
func bootstrapDirector(t *testing.T, h *harness, orgID uuid.UUID) (*http.Cookie, *store.User) {
	t.Helper()
	pUser := h.stub.NewUser("dir@x.test", "Site Director")
	user, err := h.pool.CreateExternalUser(context.Background(),
		orgID, pUser.ID, "dir@x.test", "Site Director", store.RoleSiteDirector)
	if err != nil {
		t.Fatalf("CreateExternalUser: %v", err)
	}
	jwt, _ := h.stub.NewSession(pUser.ID, "", time.Hour)
	c := &http.Client{}
	resp, body := bearer(t, c, h.server.URL+"/api/auth/exchange", jwt, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("director exchange: %d %v", resp.StatusCode, body)
	}
	cookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	if cookie == nil {
		t.Fatalf("no lb_session cookie for director")
	}
	return cookie, user
}

func TestAdminOverviewRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	_, org, _ := signInAsAdmin(t, h)
	dirCookie, _ := bootstrapDirector(t, h, org.ID)

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/overview", nil, dirCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("overview as director: %d %v want 403", resp.StatusCode, body)
	}
}

func TestAdminOverviewReturnsCounts(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/overview", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("overview: %d %v", resp.StatusCode, body)
	}
	counts, ok := body["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts missing: %v", body)
	}
	if _, ok := counts["boats"]; !ok {
		t.Errorf("counts.boats missing")
	}
	if _, ok := counts["trips"]; !ok {
		t.Errorf("counts.trips missing")
	}
	setup, ok := body["setup"].(map[string]any)
	if !ok || setup["pct"] == nil {
		t.Errorf("setup.pct missing: %v", body)
	}
}

func TestAdminListBoatsReturnsEmptyForNewOrg(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/boats", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("boats: %d %v", resp.StatusCode, body)
	}
	if body["boats"] == nil {
		t.Errorf("boats key missing")
	}
}

func TestAdminListTripsAdminGetsAll(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/trips", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trips: %d %v", resp.StatusCode, body)
	}
	if body["scope"] != "all" {
		t.Errorf("scope = %v want all", body["scope"])
	}
}

func TestAdminListTripsDirectorGetsAssigned(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	_, org, _ := signInAsAdmin(t, h)
	dirCookie, _ := bootstrapDirector(t, h, org.ID)
	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/trips", nil, dirCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trips as director: %d %v", resp.StatusCode, body)
	}
	if body["scope"] != "assigned_to_me" {
		t.Errorf("scope = %v want assigned_to_me", body["scope"])
	}
}

func TestAdminListUsersRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	_, org, _ := signInAsAdmin(t, h)
	dirCookie, _ := bootstrapDirector(t, h, org.ID)

	resp, _ := doJSON(t, c, "GET", h.server.URL+"/api/admin/users", nil, dirCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("users as director: %d want 403", resp.StatusCode)
	}
}

func TestAdminListUsersReturnsAllOrgUsers(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, org, _ := signInAsAdmin(t, h)
	bootstrapDirector(t, h, org.ID)

	resp, body := doJSON(t, c, "GET", h.server.URL+"/api/admin/users", nil, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("users: %d %v", resp.StatusCode, body)
	}
	rows, _ := body["users"].([]any)
	if len(rows) != 2 {
		t.Errorf("len users = %d want 2", len(rows))
	}
}

func TestAdminPatchOrganizationUpdatesNameAndCurrency(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	cookie, _, _ := signInAsAdmin(t, h)
	resp, body := doJSON(t, c, "PATCH", h.server.URL+"/api/organization", map[string]any{
		"name":     "Renamed Diving",
		"currency": "EUR",
	}, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: %d %v", resp.StatusCode, body)
	}
	if body["name"] != "Renamed Diving" {
		t.Errorf("name = %v", body["name"])
	}
	if body["currency"] != "EUR" {
		t.Errorf("currency = %v", body["currency"])
	}
}

func TestAdminPatchOrganizationDirectorIs403(t *testing.T) {
	h := newHarness(t)
	c := &http.Client{}
	_, org, _ := signInAsAdmin(t, h)
	dirCookie, _ := bootstrapDirector(t, h, org.ID)
	resp, _ := doJSON(t, c, "PATCH", h.server.URL+"/api/organization", map[string]any{
		"name": "Bad",
	}, dirCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("patch as director: %d want 403", resp.StatusCode)
	}
}
