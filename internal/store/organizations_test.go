package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func TestCreateOrgAndAdmin(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()

	org, user := testdb.SeedOrgWithAdmin(t, p, "Acme Diving", "owner@acme.test", "Owner")
	if org.Name != "Acme Diving" {
		t.Errorf("org name: %q", org.Name)
	}
	if user.OrganizationID != org.ID {
		t.Errorf("user.OrganizationID = %v, want %v", user.OrganizationID, org.ID)
	}
	if user.Role != store.RoleOrgAdmin {
		t.Errorf("user.Role = %q, want %q", user.Role, store.RoleOrgAdmin)
	}
	if user.EmailVerifiedAt == nil {
		t.Errorf("expected verified-at set after seed")
	}
	if !user.IsActive {
		t.Errorf("expected admin to be active")
	}
	if got, err := p.UserByEmail(ctx, "owner@acme.test"); err != nil || got.ID != user.ID {
		t.Errorf("UserByEmail roundtrip: got %v err %v", got, err)
	}
}

func TestCreateOrgAndAdminEmailConflict(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	testdb.SeedOrgWithAdmin(t, p, "A", "x@x.test", "Alice")

	hash := []byte("doesnt-matter")
	_, _, err := p.CreateOrgAndAdmin(ctx, "B", "x@x.test", "Bob", hash)
	if !errors.Is(err, store.ErrEmailTaken) {
		t.Fatalf("err = %v want ErrEmailTaken", err)
	}
}

func TestCreateInvitedUser(t *testing.T) {
	p := testdb.Pool(t)
	org, _ := testdb.SeedOrgWithAdmin(t, p, "A", "a@x.test", "Alice")
	site := testdb.SeedSiteDirector(t, p, org.ID, "site@x.test", "Site Director")
	if site.Role != store.RoleSiteDirector {
		t.Errorf("Role = %q", site.Role)
	}
	if site.OrganizationID != org.ID {
		t.Errorf("OrganizationID drift: %v", site.OrganizationID)
	}
	if site.EmailVerifiedAt == nil {
		t.Errorf("seeded directors should be verified")
	}
}

func TestDeactivateUser(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	_, user := testdb.SeedOrgWithAdmin(t, p, "A", "a@x.test", "Alice")
	if err := p.DeactivateUser(ctx, user.ID); err != nil {
		t.Fatalf("DeactivateUser: %v", err)
	}
	got, err := p.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if got.IsActive {
		t.Errorf("user still active after DeactivateUser")
	}
}

func TestOrganizationByNameCaseInsensitive(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	want, _ := testdb.SeedOrgWithAdmin(t, p, "Acme Diving", "byname@x.test", "U")

	for _, q := range []string{"Acme Diving", "acme diving", "ACME DIVING"} {
		got, err := p.OrganizationByName(ctx, q)
		if err != nil {
			t.Errorf("query %q: %v", q, err)
			continue
		}
		if got.ID != want.ID {
			t.Errorf("query %q: got id %v want %v", q, got.ID, want.ID)
		}
	}

	if _, err := p.OrganizationByName(ctx, "no-such-org"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing: %v want ErrNotFound", err)
	}
}

func TestOrganizationByNameAmbiguous(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	testdb.SeedOrgWithAdmin(t, p, "Acme", "a1@x.test", "A1")
	// Direct INSERT a second org with the same name in a different case so
	// the case-insensitive matcher returns multiple rows. The unique
	// constraint on `organizations.name` is exact-case, so this is allowed.
	if _, err := p.Exec(ctx,
		`INSERT INTO organizations (name) VALUES ($1)`, "acme"); err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	if _, err := p.OrganizationByName(ctx, "Acme"); !errors.Is(err, store.ErrOrgAmbiguous) {
		t.Fatalf("err = %v want ErrOrgAmbiguous", err)
	}
}

func TestUpdateOrganizationProfile(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org, _ := testdb.SeedOrgWithAdmin(t, p, "Old", "a@x.test", "Alice")

	cur := "EUR"
	got, err := p.UpdateOrganizationProfile(ctx, org.ID, "New", &cur)
	if err != nil {
		t.Fatalf("UpdateOrganizationProfile: %v", err)
	}
	if got.Name != "New" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Currency == nil || *got.Currency != "EUR" {
		t.Errorf("Currency = %v want EUR", got.Currency)
	}
}
