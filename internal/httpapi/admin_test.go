package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
)

// signInAsAdmin bootstraps an org via the new signup -> verify -> login
// flow and returns the admin's session cookie + the org/user it created.
func signInAsAdmin(t *testing.T, h *harness) (*http.Cookie, *store.Organization, *store.User) {
	t.Helper()
	c := &http.Client{}
	cookie := signupAndVerify(t, h, c, "Acme Diving", "admin@x.test", "Admin", "Sup3rStrong!")
	user, err := h.pool.UserByEmail(context.Background(), "admin@x.test")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	org, err := h.pool.OrganizationByID(context.Background(), user.OrganizationID)
	if err != nil {
		t.Fatalf("OrganizationByID: %v", err)
	}
	return cookie, org, user
}

// bootstrapDirector creates a Site Director user in the same org by
// shortcutting the invitation flow at the store layer (we already test
// the full HTTP invitation flow elsewhere), marks them verified, and
// logs them in.
func bootstrapDirector(t *testing.T, h *harness, orgID uuid.UUID) (*http.Cookie, *store.User) {
	t.Helper()
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte("Sup3rStrong!"), 4)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	user, err := h.pool.CreateInvitedUser(ctx, orgID, "dir@x.test", "Director", store.RoleSiteDirector, hash)
	if err != nil {
		t.Fatalf("CreateInvitedUser: %v", err)
	}
	if err := h.pool.MarkEmailVerified(ctx, user.ID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkEmailVerified: %v", err)
	}
	got, err := h.pool.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	c := &http.Client{}
	resp, body := doJSON(t, c, "POST", h.server.URL+"/api/auth/login", map[string]any{
		"email":    "dir@x.test",
		"password": "Sup3rStrong!",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("director login: %d %v", resp.StatusCode, body)
	}
	cookie := pickCookieFrom(resp.Cookies(), auth.SessionCookieName)
	if cookie == nil {
		t.Fatalf("no lb_session cookie for director")
	}
	return cookie, got
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
