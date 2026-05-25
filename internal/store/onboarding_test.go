package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

// TestOnboardingStateBrandNewOrg confirms that a fresh org returns
// all four wizard steps not-done and onboarding_complete = false.
func TestOnboardingStateBrandNewOrg(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org, _ := testdb.SeedOrgWithAdmin(t, p, "Acme", "owner@x.test", "Owner")

	state, err := p.OnboardingState(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.OnboardingComplete {
		t.Error("expected new org to be incomplete")
	}
	if state.DismissedAt != nil {
		t.Error("expected no dismissed timestamp on a new org")
	}
	wantKeys := []string{"currency", "boats", "layouts", "directors"}
	if len(state.Steps) != len(wantKeys) {
		t.Fatalf("expected %d steps, got %d", len(wantKeys), len(state.Steps))
	}
	for i, k := range wantKeys {
		if state.Steps[i].Key != k {
			t.Errorf("step %d key = %q, want %q", i, state.Steps[i].Key, k)
		}
		if state.Steps[i].Done {
			t.Errorf("step %q should be not-done for a new org", k)
		}
	}
	if len(state.BoatsWithoutLayouts) != 0 {
		t.Errorf("expected no boats-without-layouts for a new org, got %d", len(state.BoatsWithoutLayouts))
	}
}

// TestOnboardingStateLayoutsDefinition asserts that a boat with
// active cabins but zero active berths is reported as missing a
// layout (the merge-notes definition: ≥1 active cabin AND ≥1 active
// berth, both required).
func TestOnboardingStateLayoutsDefinition(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org, _ := testdb.SeedOrgWithAdmin(t, p, "Acme", "owner@x.test", "Owner")

	boat, err := p.UpsertBoat(ctx, org.ID, "liveaboard.com", store.BoatScrape{
		Slug: "sirius", Name: "Sirius", URL: "https://x/sirius",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// Boat exists, no cabins → unconfigured.
	state, err := p.OnboardingState(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.BoatsWithoutLayouts) != 1 {
		t.Fatalf("expected boat to need layout: %+v", state.BoatsWithoutLayouts)
	}

	// Insert a cabin with no berths → still unconfigured (no usable
	// sleeping spots).
	var cabinID string
	if err := p.QueryRow(ctx, `
		INSERT INTO boat_cabins (organization_id, boat_id, label, sort_order, is_active)
		VALUES ($1, $2, 'A1', 0, true) RETURNING id
	`, org.ID, boat.ID).Scan(&cabinID); err != nil {
		t.Fatal(err)
	}
	state, _ = p.OnboardingState(ctx, org.ID)
	if len(state.BoatsWithoutLayouts) != 1 {
		t.Errorf("cabin without berths should still count as unconfigured: %+v", state.BoatsWithoutLayouts)
	}

	// Insert an inactive berth → still unconfigured.
	if _, err := p.Exec(ctx, `
		INSERT INTO boat_cabin_berths (organization_id, boat_id, cabin_id, berth_label, display_label, is_active)
		VALUES ($1, $2, $3, 'lower', 'A1 lower', false)
	`, org.ID, boat.ID, cabinID); err != nil {
		t.Fatal(err)
	}
	state, _ = p.OnboardingState(ctx, org.ID)
	if len(state.BoatsWithoutLayouts) != 1 {
		t.Errorf("inactive berth should still count as unconfigured")
	}

	// Activate the berth → boat becomes configured.
	if _, err := p.Exec(ctx, `
		UPDATE boat_cabin_berths SET is_active = true WHERE boat_id = $1
	`, boat.ID); err != nil {
		t.Fatal(err)
	}
	state, _ = p.OnboardingState(ctx, org.ID)
	if len(state.BoatsWithoutLayouts) != 0 {
		t.Errorf("boat with active cabin + active berth should be configured: %+v", state.BoatsWithoutLayouts)
	}
}

// TestOnboardingStateCrossTenantIsolation asserts that one org's
// onboarding state never includes another org's boats.
func TestOnboardingStateCrossTenantIsolation(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	orgA, _ := testdb.SeedOrgWithAdmin(t, p, "A", "a@x.test", "A")
	orgB, _ := testdb.SeedOrgWithAdmin(t, p, "B", "b@x.test", "B")

	// Both orgs get an unconfigured boat (zero active berths).
	if _, err := p.UpsertBoat(ctx, orgA.ID, "liveaboard.com", store.BoatScrape{
		Slug: "boatA", Name: "boatA", URL: "https://x/a",
	}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpsertBoat(ctx, orgB.ID, "liveaboard.com", store.BoatScrape{
		Slug: "boatB", Name: "boatB", URL: "https://x/b",
	}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	stateA, err := p.OnboardingState(ctx, orgA.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range stateA.BoatsWithoutLayouts {
		if b.BoatName == "boatB" {
			t.Fatalf("org A leaked boat from org B: %+v", b)
		}
	}
}

// TestDismissOrganizationOnboarding asserts the timestamp is written
// idempotently and reflected on subsequent reads.
func TestDismissOrganizationOnboarding(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org, _ := testdb.SeedOrgWithAdmin(t, p, "Acme", "owner@x.test", "Owner")

	// Initially nil.
	state, _ := p.OnboardingState(ctx, org.ID)
	if state.DismissedAt != nil {
		t.Fatalf("expected nil dismissed_at initially, got %v", *state.DismissedAt)
	}

	t0 := time.Now().UTC()
	got, err := p.DismissOrganizationOnboarding(ctx, org.ID, t0)
	if err != nil {
		t.Fatal(err)
	}
	if got.OnboardingDismissedAt == nil {
		t.Fatal("dismissed_at should be set")
	}

	// Idempotent: a second call doesn't change the timestamp.
	t1 := t0.Add(time.Hour)
	got2, err := p.DismissOrganizationOnboarding(ctx, org.ID, t1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OnboardingDismissedAt.Equal(*got2.OnboardingDismissedAt) {
		t.Errorf("second dismiss should preserve original timestamp; got %v then %v",
			*got.OnboardingDismissedAt, *got2.OnboardingDismissedAt)
	}

	// And the read path reflects it.
	state, _ = p.OnboardingState(ctx, org.ID)
	if state.DismissedAt == nil {
		t.Fatal("OnboardingState should report dismissed_at after dismiss")
	}
}
