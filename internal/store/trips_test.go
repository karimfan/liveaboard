package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

// fingerprint mirrors the parser's source_trip_key contract so the repo
// tests don't depend on the parser package.
func fingerprint(slug, startDate, endDate, itinerary, departurePort string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s",
		strings.ToLower(strings.TrimSpace(slug)),
		startDate, endDate,
		strings.ToLower(strings.TrimSpace(itinerary)),
		strings.ToLower(strings.TrimSpace(departurePort)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func bootstrapBoat(t *testing.T, p *store.Pool) *store.Boat {
	t.Helper()
	org := bootstrapOrg(t, p)
	now := time.Now().UTC()
	b, err := p.UpsertBoat(context.Background(), org.ID, "liveaboard.com", store.BoatScrape{
		Slug: "gaia-love", Name: "Gaia Love", URL: "https://x/gaia-love",
	}, now)
	if err != nil {
		t.Fatalf("UpsertBoat: %v", err)
	}
	return b
}

func tripFor(slug, start, end, it, dep string) store.TripScrape {
	startT, _ := time.Parse("2006-01-02", start)
	endT, _ := time.Parse("2006-01-02", end)
	return store.TripScrape{
		StartDate:        startT,
		EndDate:          endT,
		Itinerary:        it,
		DeparturePort:    dep,
		ReturnPort:       dep,
		PriceText:        "$6,400",
		AvailabilityText: "AVAILABLE",
		SourceURL:        "https://x/gaia-love?m=" + start[5:7] + "/" + start[:4],
		SourceTripKey:    fingerprint(slug, start, end, it, dep),
	}
}

func TestReplaceFutureScrapedTripsInsertsAll(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	b := bootstrapBoat(t, p)
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	now := today

	scrapes := []store.TripScrape{
		tripFor("gaia-love", "2026-06-01", "2026-06-08", "Komodo", "Bali"),
		tripFor("gaia-love", "2026-06-10", "2026-06-17", "Komodo", "Bali"),
	}

	res, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com", scrapes, now, today)
	if err != nil {
		t.Fatalf("ReplaceFutureScrapedTrips: %v", err)
	}
	if res.Inserts != 2 || res.Updates != 0 || res.StaleDeletes != 0 {
		t.Errorf("counts: %+v want {2 0 0}", res)
	}

	rows, err := p.TripsForBoat(ctx, b.ID)
	if err != nil {
		t.Fatalf("TripsForBoat: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("rows = %d want 2", len(rows))
	}
	for _, r := range rows {
		if r.OrganizationID != b.OrganizationID {
			t.Errorf("organization_id not denormalized: got %v", r.OrganizationID)
		}
	}
}

func TestReplaceFutureScrapedTripsIdempotentRerun(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	b := bootstrapBoat(t, p)
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	scrapes := []store.TripScrape{
		tripFor("gaia-love", "2026-06-01", "2026-06-08", "Komodo", "Bali"),
	}

	first := today
	if _, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com", scrapes, first, today); err != nil {
		t.Fatalf("first run: %v", err)
	}
	second := today.Add(time.Hour)
	res, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com", scrapes, second, today)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if res.Inserts != 0 || res.Updates != 1 || res.StaleDeletes != 0 {
		t.Errorf("rerun counts: %+v want {0 1 0}", res)
	}

	rows, _ := p.TripsForBoat(ctx, b.ID)
	if len(rows) != 1 {
		t.Errorf("len = %d want 1", len(rows))
	}
	if !rows[0].SourceLastSyncedAt.After(first) {
		t.Errorf("synced_at did not advance: %v", rows[0].SourceLastSyncedAt)
	}
}

func TestReplaceFutureScrapedTripsAppliesPriceUpdates(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	b := bootstrapBoat(t, p)
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	first := tripFor("gaia-love", "2026-06-01", "2026-06-08", "Komodo", "Bali")
	if _, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com",
		[]store.TripScrape{first}, today, today); err != nil {
		t.Fatalf("first: %v", err)
	}

	updated := first
	updated.PriceText = "$5,900"
	updated.AvailabilityText = "FULL"
	if _, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com",
		[]store.TripScrape{updated}, today.Add(time.Hour), today); err != nil {
		t.Fatalf("update: %v", err)
	}

	rows, _ := p.TripsForBoat(ctx, b.ID)
	if len(rows) != 1 {
		t.Fatalf("len = %d", len(rows))
	}
	if rows[0].PriceText == nil || *rows[0].PriceText != "$5,900" {
		t.Errorf("PriceText not updated: %v", rows[0].PriceText)
	}
	if rows[0].AvailabilityText == nil || *rows[0].AvailabilityText != "FULL" {
		t.Errorf("AvailabilityText not updated: %v", rows[0].AvailabilityText)
	}
}

func TestReplaceFutureScrapedTripsDeletesStaleTrips(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	b := bootstrapBoat(t, p)
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	twoTrips := []store.TripScrape{
		tripFor("gaia-love", "2026-06-01", "2026-06-08", "Komodo", "Bali"),
		tripFor("gaia-love", "2026-07-01", "2026-07-08", "Komodo", "Bali"),
	}
	if _, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com",
		twoTrips, today, today); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Source-side, the July trip is canceled. New scrape only carries June.
	oneTrip := twoTrips[:1]
	res, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com",
		oneTrip, today.Add(time.Hour), today)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if res.StaleDeletes != 1 {
		t.Errorf("StaleDeletes = %d want 1", res.StaleDeletes)
	}
	if res.Inserts != 0 || res.Updates != 1 {
		t.Errorf("counts: %+v", res)
	}

	rows, _ := p.TripsForBoat(ctx, b.ID)
	if len(rows) != 1 {
		t.Errorf("len = %d want 1", len(rows))
	}
}

func TestReplaceFutureScrapedTripsDoesNotTouchPastTrips(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	b := bootstrapBoat(t, p)

	today := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	pastStart := today.AddDate(0, -2, 0).Format("2006-01-02")
	pastEnd := today.AddDate(0, -2, 7).Format("2006-01-02")

	past := tripFor("gaia-love", pastStart, pastEnd, "Komodo", "Bali")
	// Seed the past trip via an insert that bypasses the "stale-delete"
	// behavior. Pass `today=pastStart-day` on this seeding call so
	// nothing gets deleted, and use the same row in the upsert path.
	pastDay, _ := time.Parse("2006-01-02", pastStart)
	if _, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com",
		[]store.TripScrape{past}, pastDay, pastDay); err != nil {
		t.Fatalf("seed past trip: %v", err)
	}

	// Now do a "real" scrape with today=today and an empty future scrape.
	// The past trip must remain because it's strictly before today.
	res, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com",
		nil, today, today)
	if err != nil {
		t.Fatalf("empty rerun: %v", err)
	}
	if res.StaleDeletes != 0 {
		t.Errorf("past trip got deleted; StaleDeletes = %d want 0", res.StaleDeletes)
	}

	rows, _ := p.TripsForBoat(ctx, b.ID)
	if len(rows) != 1 {
		t.Errorf("len = %d want 1 (past trip preserved)", len(rows))
	}
}

func TestReplaceFutureScrapedTripsRejectsBadDateRange(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	b := bootstrapBoat(t, p)
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	bad := tripFor("gaia-love", "2026-06-08", "2026-06-01", "Komodo", "Bali")
	if _, err := p.ReplaceFutureScrapedTrips(ctx, b.OrganizationID, b.ID, "liveaboard.com",
		[]store.TripScrape{bad}, today, today); err == nil {
		t.Fatalf("expected error on end<start")
	}
}
