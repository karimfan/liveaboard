package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func TestCreateExternalOrgAndAdmin(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()

	org, user, err := p.CreateExternalOrgAndAdmin(ctx,
		"Acme Diving", "org_clerk_1",
		"user_clerk_1", "owner@acme.test", "Owner",
	)
	if err != nil {
		t.Fatalf("CreateExternalOrgAndAdmin: %v", err)
	}
	if org.Name != "Acme Diving" {
		t.Errorf("org name: %q", org.Name)
	}
	if user.OrganizationID != org.ID {
		t.Errorf("user.OrganizationID = %v, want %v", user.OrganizationID, org.ID)
	}
	if user.Role != store.RoleOrgAdmin {
		t.Errorf("user.Role = %q, want %q", user.Role, store.RoleOrgAdmin)
	}
	if user.ClerkUserID != "user_clerk_1" {
		t.Errorf("user.ClerkUserID = %q", user.ClerkUserID)
	}
}

func TestCreateExternalOrgAndAdminEmailConflict(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	if _, _, err := p.CreateExternalOrgAndAdmin(ctx, "A", "org_a", "user_a", "x@x.test", "Alice"); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, _, err := p.CreateExternalOrgAndAdmin(ctx, "B", "org_b", "user_b", "x@x.test", "Bob")
	if !errors.Is(err, store.ErrEmailTaken) {
		t.Fatalf("err = %v want ErrEmailTaken", err)
	}
}

func TestCreateExternalOrgAndAdminClerkIDConflict(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	if _, _, err := p.CreateExternalOrgAndAdmin(ctx, "A", "org_a", "user_a", "a@x.test", "Alice"); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Same clerk_org_id, different email -> ErrOrgClerkIDTaken
	_, _, err := p.CreateExternalOrgAndAdmin(ctx, "B", "org_a", "user_b", "b@x.test", "Bob")
	if !errors.Is(err, store.ErrOrgClerkIDTaken) {
		t.Fatalf("err = %v want ErrOrgClerkIDTaken", err)
	}
}

func TestUserByClerkID(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	_, want, err := p.CreateExternalOrgAndAdmin(ctx, "A", "org_a", "user_a", "a@x.test", "Alice")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := p.UserByClerkID(ctx, "user_a")
	if err != nil {
		t.Fatalf("UserByClerkID: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("got id %v want %v", got.ID, want.ID)
	}

	if _, err := p.UserByClerkID(ctx, "user_missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing: %v want ErrNotFound", err)
	}
}

func TestCreateExternalUser(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org, _, err := p.CreateExternalOrgAndAdmin(ctx, "A", "org_a", "user_a", "a@x.test", "Alice")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	site, err := p.CreateExternalUser(ctx, org.ID, "user_b", "site@x.test", "Site Director", store.RoleSiteDirector)
	if err != nil {
		t.Fatalf("CreateExternalUser: %v", err)
	}
	if site.Role != store.RoleSiteDirector {
		t.Errorf("Role = %q", site.Role)
	}
	if site.ClerkUserID != "user_b" {
		t.Errorf("ClerkUserID = %q", site.ClerkUserID)
	}
}

func TestUpdateExternalUserSyncsIdentity(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	_, want, err := p.CreateExternalOrgAndAdmin(ctx, "A", "org_a", "user_a", "old@x.test", "Old Name")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := p.UpdateExternalUser(ctx, "user_a", "new@x.test", "New Name"); err != nil {
		t.Fatalf("UpdateExternalUser: %v", err)
	}

	got, err := p.UserByID(ctx, want.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	if got.Email != "new@x.test" || got.FullName != "New Name" {
		t.Errorf("got %+v", got)
	}
	// Role unchanged.
	if got.Role != store.RoleOrgAdmin {
		t.Errorf("Role drift: %q", got.Role)
	}
}

func TestDeactivateUser(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	_, user, err := p.CreateExternalOrgAndAdmin(ctx, "A", "org_a", "user_a", "a@x.test", "Alice")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
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

func TestOrganizationByClerkID(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	want, _, err := p.CreateExternalOrgAndAdmin(ctx, "A", "org_a", "user_a", "a@x.test", "Alice")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := p.OrganizationByClerkID(ctx, "org_a")
	if err != nil {
		t.Fatalf("OrganizationByClerkID: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("id mismatch")
	}

	if _, err := p.OrganizationByClerkID(ctx, "org_missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing: %v want ErrNotFound", err)
	}
}

func TestOrganizationByNameCaseInsensitive(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	want, _, err := p.CreateExternalOrgAndAdmin(ctx, "Acme Diving", "org_byname", "user_byname", "byname@x.test", "U")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

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
	if _, _, err := p.CreateExternalOrgAndAdmin(ctx, "Acme", "org_a1", "user_a1", "a1@x.test", "A"); err != nil {
		t.Fatalf("setup A: %v", err)
	}
	// Insert a second "Acme" via direct INSERT (case-different but
	// matched case-insensitively).
	if _, err := p.Exec(ctx,
		`INSERT INTO organizations (name, clerk_org_id) VALUES ($1, $2)`,
		"acme", "org_a2"); err != nil {
		t.Fatalf("setup A2: %v", err)
	}

	_, err := p.OrganizationByName(ctx, "Acme")
	if !errors.Is(err, store.ErrOrgAmbiguous) {
		t.Fatalf("err = %v want ErrOrgAmbiguous", err)
	}
}

func TestUpdateOrganizationName(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	if _, _, err := p.CreateExternalOrgAndAdmin(ctx, "Old", "org_a", "user_a", "a@x.test", "Alice"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := p.UpdateOrganizationName(ctx, "org_a", "New", time.Now()); err != nil {
		t.Fatalf("UpdateOrganizationName: %v", err)
	}
	got, err := p.OrganizationByClerkID(ctx, "org_a")
	if err != nil {
		t.Fatalf("OrganizationByClerkID: %v", err)
	}
	if got.Name != "New" {
		t.Errorf("Name = %q", got.Name)
	}
}
