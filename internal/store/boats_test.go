package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func bootstrapOrg(t *testing.T, p *store.Pool) *store.Organization {
	t.Helper()
	org, _, err := p.CreateExternalOrgAndAdmin(context.Background(),
		"Acme Diving", "org_clerk_boats", "user_clerk_boats", "owner@x.test", "Owner")
	if err != nil {
		t.Fatalf("CreateExternalOrgAndAdmin: %v", err)
	}
	return org
}

func TestUpsertBoatInsertsAndReturnsRow(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org := bootstrapOrg(t, p)
	now := time.Now().UTC()

	b, err := p.UpsertBoat(ctx, org.ID, "liveaboard.com", store.BoatScrape{
		Slug:       "gaia-love",
		Name:       "Gaia Love",
		URL:        "https://www.liveaboard.com/diving/indonesia/gaia-love",
		ImageURL:   "https://img.liveaboard.com/picture_library/boat/5695/gaia-main.jpg",
		ExternalID: "5695",
	}, now)
	if err != nil {
		t.Fatalf("UpsertBoat: %v", err)
	}

	if b.OrganizationID != org.ID {
		t.Errorf("OrganizationID = %v want %v", b.OrganizationID, org.ID)
	}
	if b.DisplayName != "Gaia Love" {
		t.Errorf("DisplayName = %q want %q", b.DisplayName, "Gaia Love")
	}
	if b.SourceName != "Gaia Love" || b.SourceSlug != "gaia-love" {
		t.Errorf("source fields drift: %+v", b)
	}
	if b.SourceImageURL == nil || *b.SourceImageURL == "" {
		t.Errorf("source_image_url not set")
	}
	if b.SourceLastSyncedAt.IsZero() {
		t.Errorf("source_last_synced_at not set")
	}
}

func TestUpsertBoatPreservesOperatorEditedDisplayName(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org := bootstrapOrg(t, p)
	first := time.Now().UTC().Truncate(time.Second)

	b, err := p.UpsertBoat(ctx, org.ID, "liveaboard.com", store.BoatScrape{
		Slug: "gaia-love", Name: "Gaia Love",
		URL: "https://www.liveaboard.com/diving/indonesia/gaia-love",
	}, first)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Operator renames the boat in our app.
	if _, err := p.Exec(ctx, `UPDATE boats SET display_name = $1 WHERE id = $2`,
		"Acme Diver Express", b.ID); err != nil {
		t.Fatalf("operator rename: %v", err)
	}

	// Source-side rename happens too.
	second := first.Add(time.Hour)
	b2, err := p.UpsertBoat(ctx, org.ID, "liveaboard.com", store.BoatScrape{
		Slug: "gaia-love", Name: "Gaia Love (rebrand)",
		URL: "https://www.liveaboard.com/diving/indonesia/gaia-love",
	}, second)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if b2.DisplayName != "Acme Diver Express" {
		t.Errorf("display_name was clobbered: got %q want %q", b2.DisplayName, "Acme Diver Express")
	}
	if b2.SourceName != "Gaia Love (rebrand)" {
		t.Errorf("source_name not updated: %q", b2.SourceName)
	}
	if !b2.SourceLastSyncedAt.After(first) {
		t.Errorf("source_last_synced_at did not advance: %v", b2.SourceLastSyncedAt)
	}
	if b2.ID != b.ID {
		t.Errorf("upsert created a new row instead of updating: %v != %v", b2.ID, b.ID)
	}
}

func TestUpsertBoatTenantIsolation(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	orgA := bootstrapOrg(t, p)

	var orgBID uuid.UUID
	if err := p.QueryRow(ctx,
		`INSERT INTO organizations (name, clerk_org_id) VALUES ($1, $2) RETURNING id`,
		"Beta Diving", "org_clerk_b").Scan(&orgBID); err != nil {
		t.Fatalf("create org B: %v", err)
	}

	now := time.Now().UTC()
	if _, err := p.UpsertBoat(ctx, orgA.ID, "liveaboard.com", store.BoatScrape{
		Slug: "gaia-love", Name: "Gaia Love",
		URL: "https://x/gaia-love",
	}, now); err != nil {
		t.Fatalf("orgA upsert: %v", err)
	}
	if _, err := p.UpsertBoat(ctx, orgBID, "liveaboard.com", store.BoatScrape{
		Slug: "gaia-love", Name: "Gaia Love (B copy)",
		URL: "https://x/gaia-love",
	}, now); err != nil {
		t.Fatalf("orgB upsert: %v", err)
	}

	a, err := p.BoatBySourceSlug(ctx, orgA.ID, "liveaboard.com", "gaia-love")
	if err != nil {
		t.Fatalf("orgA lookup: %v", err)
	}
	b, err := p.BoatBySourceSlug(ctx, orgBID, "liveaboard.com", "gaia-love")
	if err != nil {
		t.Fatalf("orgB lookup: %v", err)
	}
	if a.ID == b.ID {
		t.Errorf("expected distinct boat rows per tenant; got same id")
	}
	if a.SourceName == b.SourceName {
		t.Errorf("source_name should reflect each tenant's scrape independently")
	}
}

func TestBoatsForOrg(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org := bootstrapOrg(t, p)
	now := time.Now().UTC()

	for _, name := range []string{"Beta", "Alpha"} {
		if _, err := p.UpsertBoat(ctx, org.ID, "liveaboard.com", store.BoatScrape{
			Slug: name + "-slug", Name: name, URL: "https://x/" + name,
		}, now); err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
	}

	got, err := p.BoatsForOrg(ctx, org.ID)
	if err != nil {
		t.Fatalf("BoatsForOrg: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d want 2", len(got))
	}
	if got[0].DisplayName != "Alpha" || got[1].DisplayName != "Beta" {
		t.Errorf("not ordered by display_name: %s, %s", got[0].DisplayName, got[1].DisplayName)
	}
}
