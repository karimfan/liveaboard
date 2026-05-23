package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

// reportFixture is a compact, multi-org seed for report tests. Each
// org has one boat and a small set of trips with different statuses,
// guests, folios, and folio lines (including voided lines and a
// crew tip) so the reports surface can be exercised across the full
// matrix in one place.
type reportFixture struct {
	OrgA, OrgB                       *store.Organization
	BoatA                            *store.Boat
	BoatB                            *store.Boat
	TripPlanned, TripActive, TripDone *store.Trip
	TripB                            *store.Trip
	GuestUserA                       uuid.UUID // guest_users.id
	GuestUserA2                      uuid.UUID
	GuestUserB                       uuid.UUID
	TripGuestA1                      uuid.UUID // owned by GuestUserA on TripActive
	TripGuestA2                      uuid.UUID // owned by GuestUserA2 on TripActive
	TripGuestB                       uuid.UUID // owned by GuestUserB on TripB
	FolioA1Closed                    uuid.UUID
	FolioA2Open                      uuid.UUID
	FolioB                           uuid.UUID
	AdminA                           *store.User
}

func seedReportFixture(t *testing.T, p *store.Pool) *reportFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)

	orgA, adminA := testdb.SeedOrgWithAdmin(t, p, "Acme Diving", "owner-a@x.test", "Owner A")
	orgB, _ := testdb.SeedOrgWithAdmin(t, p, "Reef Co", "owner-b@x.test", "Owner B")

	boatA, err := p.UpsertBoat(ctx, orgA.ID, "liveaboard.com", store.BoatScrape{
		Slug: "gaia-a", Name: "Gaia A", URL: "https://x/gaia-a",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	boatB, err := p.UpsertBoat(ctx, orgB.ID, "liveaboard.com", store.BoatScrape{
		Slug: "gaia-b", Name: "Gaia B", URL: "https://x/gaia-b",
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	// Three org-A trips, one org-B trip — all inside the default window.
	tripPlanned := insertTripDirect(t, p, orgA.ID, boatA.ID, "2026-06-01", "2026-06-07", "Komodo P", "planned")
	tripActive := insertTripDirect(t, p, orgA.ID, boatA.ID, "2026-05-20", "2026-05-27", "Komodo A", "active")
	tripDone := insertTripDirect(t, p, orgA.ID, boatA.ID, "2026-05-01", "2026-05-08", "Komodo D", "completed")
	tripB := insertTripDirect(t, p, orgB.ID, boatB.ID, "2026-05-20", "2026-05-27", "Reef B", "active")

	guestUserA := insertGuestUser(t, p, "a1@x.test")
	guestUserA2 := insertGuestUser(t, p, "a2@x.test")
	guestUserB := insertGuestUser(t, p, "b1@x.test")

	tripGuestA1 := insertTripGuest(t, p, orgA.ID, tripActive.ID, guestUserA, "Aya")
	tripGuestA2 := insertTripGuest(t, p, orgA.ID, tripActive.ID, guestUserA2, "Bob")
	tripGuestB := insertTripGuest(t, p, orgB.ID, tripB.ID, guestUserB, "Cy")

	// Folio for trip-guest A1: closed, $100 in catalog items, $10 tip, $5 voided correction, EUR settlement.
	folioA1 := insertFolio(t, p, orgA.ID, tripActive.ID, tripGuestA1, adminA.ID, "closed", folioMoney{
		Subtotal: 11000, CardFee: 330, Total: 11330,
		SettleCcy: ptrStr("EUR"), SettleMinor: ptr64(10800), CcyExp: ptrInt(2),
		PaymentMethod: ptrStr("card"), ClosedAt: ptrTime(now),
	})
	insertFolioLine(t, p, orgA.ID, folioA1, "catalog_item", "Beer", 4, 2500, 10000, "")
	insertFolioLine(t, p, orgA.ID, folioA1, "crew_tip", "Crew tip", 1, 1000, 1000, "")
	insertFolioLine(t, p, orgA.ID, folioA1, "catalog_item", "Souvenir", 1, 500, 500, "voided")

	// Folio for trip-guest A2: open, $40 charges.
	folioA2 := insertFolio(t, p, orgA.ID, tripActive.ID, tripGuestA2, adminA.ID, "open", folioMoney{
		Subtotal: 4000, CardFee: 0, Total: 4000,
	})
	insertFolioLine(t, p, orgA.ID, folioA2, "catalog_item", "Beer", 2, 2000, 4000, "")

	// Org B folio — must never show up in org A reports.
	folioB := insertFolio(t, p, orgB.ID, tripB.ID, tripGuestB, adminA.ID, "open", folioMoney{
		Subtotal: 99999, CardFee: 0, Total: 99999,
	})
	insertFolioLine(t, p, orgB.ID, folioB, "catalog_item", "Beer", 1, 99999, 99999, "")

	return &reportFixture{
		OrgA: orgA, OrgB: orgB,
		BoatA: boatA, BoatB: boatB,
		TripPlanned: tripPlanned, TripActive: tripActive, TripDone: tripDone,
		TripB:        tripB,
		GuestUserA:   guestUserA, GuestUserA2: guestUserA2, GuestUserB: guestUserB,
		TripGuestA1: tripGuestA1, TripGuestA2: tripGuestA2, TripGuestB: tripGuestB,
		FolioA1Closed: folioA1, FolioA2Open: folioA2, FolioB: folioB,
		AdminA: adminA,
	}
}

// --- direct SQL inserts (avoid hauling in whole writer flows for fixtures) ---

func insertTripDirect(t *testing.T, p *store.Pool, orgID, boatID uuid.UUID, start, end, itinerary, status string) *store.Trip {
	t.Helper()
	startT, _ := time.Parse("2006-01-02", start)
	endT, _ := time.Parse("2006-01-02", end)
	var id uuid.UUID
	err := p.QueryRow(context.Background(), `
		INSERT INTO trips (
			organization_id, boat_id, start_date, end_date, itinerary,
			source_provider, source_trip_key, source_url, source_last_synced_at, status
		) VALUES ($1, $2, $3, $4, $5, 'liveaboard.com', $6, 'https://x/' || $6, now(), $7)
		RETURNING id
	`, orgID, boatID, startT, endT, itinerary, itinerary+"-"+start, status).Scan(&id)
	if err != nil {
		t.Fatalf("insertTripDirect: %v", err)
	}
	got, err := p.TripByID(context.Background(), orgID, id)
	if err != nil {
		t.Fatalf("TripByID: %v", err)
	}
	return got
}

func insertGuestUser(t *testing.T, p *store.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := p.QueryRow(context.Background(), `
		INSERT INTO guest_users (email, password_hash, email_verified_at)
		VALUES ($1, $2, now()) RETURNING id
	`, email, []byte("x")).Scan(&id); err != nil {
		t.Fatalf("insertGuestUser: %v", err)
	}
	return id
}

func insertTripGuest(t *testing.T, p *store.Pool, orgID, tripID, guestUserID uuid.UUID, fullName string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := p.QueryRow(context.Background(), `
		INSERT INTO trip_guests (organization_id, trip_id, guest_user_id, full_name, email)
		VALUES ($1, $2, $3, $4, lower($4) || '@e2e.test') RETURNING id
	`, orgID, tripID, guestUserID, fullName).Scan(&id); err != nil {
		t.Fatalf("insertTripGuest: %v", err)
	}
	return id
}

type folioMoney struct {
	Subtotal, CardFee, Total int64
	SettleCcy                *string
	SettleMinor              *int64
	CcyExp                   *int
	PaymentMethod            *string
	ClosedAt                 *time.Time
}

func insertFolio(t *testing.T, p *store.Pool, orgID, tripID, tripGuestID, openedBy uuid.UUID, status string, m folioMoney) uuid.UUID {
	t.Helper()
	var closedBy *uuid.UUID
	if status == "closed" {
		closedBy = &openedBy
	}
	var id uuid.UUID
	err := p.QueryRow(context.Background(), `
		INSERT INTO guest_folios (
			organization_id, trip_id, trip_guest_id, status, opened_by_user_id,
			subtotal_usd_cents, card_fee_usd_cents, total_usd_cents,
			settlement_currency, settlement_total_minor, currency_exponent,
			payment_method, closed_at, closed_by_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`, orgID, tripID, tripGuestID, status, openedBy,
		m.Subtotal, m.CardFee, m.Total,
		m.SettleCcy, m.SettleMinor, m.CcyExp,
		m.PaymentMethod, m.ClosedAt, closedBy).Scan(&id)
	if err != nil {
		t.Fatalf("insertFolio: %v", err)
	}
	return id
}

func insertFolioLine(t *testing.T, p *store.Pool, orgID, folioID uuid.UUID, lineType, name string, qty int, unit, total int64, marker string) {
	t.Helper()
	ctx := context.Background()
	var tg uuid.UUID
	if err := p.QueryRow(ctx, `SELECT trip_guest_id FROM guest_folios WHERE id = $1`, folioID).Scan(&tg); err != nil {
		t.Fatalf("lookup trip_guest_id: %v", err)
	}
	// catalog_item lines need catalog_item_id NOT NULL per the line-type
	// check constraint. Create (or look up) an org-scoped catalog item by
	// name and reuse it.
	var catalogItemID *uuid.UUID
	if lineType == "catalog_item" {
		id := ensureCatalogItem(t, p, orgID, name, unit)
		catalogItemID = &id
	}
	voided := marker == "voided"
	_, err := p.Exec(ctx, `
		INSERT INTO guest_folio_lines (
			organization_id, folio_id, trip_guest_id, catalog_item_id, line_type,
			item_name, quantity, unit_price_usd_cents, line_total_usd_cents, stock_mode,
			voided_at, voided_by_user_id, void_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'none',
		          CASE WHEN $10::bool THEN now() ELSE NULL END,
		          CASE WHEN $10::bool THEN (SELECT id FROM users WHERE organization_id = $1 LIMIT 1) ELSE NULL END,
		          CASE WHEN $10::bool THEN 'test-void' ELSE NULL END)
	`, orgID, folioID, tg, catalogItemID, lineType, name, qty, unit, total, voided)
	if err != nil {
		t.Fatalf("insertFolioLine: %v", err)
	}
}

// ensureCatalogItem creates a catalog category + item for the org if
// one with the given name doesn't exist, and returns the item id.
func ensureCatalogItem(t *testing.T, p *store.Pool, orgID uuid.UUID, name string, unit int64) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	err := p.QueryRow(ctx, `SELECT id FROM catalog_items WHERE organization_id = $1 AND name = $2`, orgID, name).Scan(&id)
	if err == nil {
		return id
	}
	// Need a category first.
	var catID uuid.UUID
	if err := p.QueryRow(ctx, `
		INSERT INTO catalog_categories (organization_id, name)
		VALUES ($1, 'Test')
		ON CONFLICT (organization_id, lower(name)) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, orgID).Scan(&catID); err != nil {
		// Fallback: insert without conflict handling if the index doesn't
		// match exactly (older schema). Try a SELECT.
		_ = p.QueryRow(ctx, `SELECT id FROM catalog_categories WHERE organization_id = $1 LIMIT 1`, orgID).Scan(&catID)
		if catID == uuid.Nil {
			t.Fatalf("ensure category: %v", err)
		}
	}
	if err := p.QueryRow(ctx, `
		INSERT INTO catalog_items (
			organization_id, category_id, name, unit, charge_type, stock_mode,
			price_usd_cents, is_active
		) VALUES ($1, $2, $3, 'each', 'sale', 'none', $4, true) RETURNING id
	`, orgID, catID, name, unit).Scan(&id); err != nil {
		t.Fatalf("ensure catalog item: %v", err)
	}
	return id
}

func ptrStr(s string) *string  { return &s }
func ptr64(n int64) *int64     { return &n }
func ptrInt(n int) *int        { return &n }
func ptrTime(t time.Time) *time.Time {
	return &t
}

// --- tests ---

func TestSetupCompletenessReflectsOrgState(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()

	// Fresh org has only the admin → directors=0, boats=0, trips=0,
	// currency unset → pct 0.
	org, _ := testdb.SeedOrgWithAdmin(t, p, "Acme", "owner@x.test", "Owner")
	sc, err := p.SetupCompleteness(ctx, org.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sc.Percent != 0 {
		t.Errorf("fresh org Percent=%d want 0", sc.Percent)
	}
	if len(sc.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(sc.Steps))
	}
	for _, s := range sc.Steps {
		if s.Done {
			t.Errorf("step %q unexpectedly done in fresh org", s.Key)
		}
	}

	// Add a boat → boats step done.
	_, err = p.UpsertBoat(ctx, org.ID, "liveaboard.com", store.BoatScrape{
		Slug: "x", Name: "X", URL: "https://x",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	sc, _ = p.SetupCompleteness(ctx, org.ID)
	if sc.Percent != 25 {
		t.Errorf("after-boat Percent=%d want 25", sc.Percent)
	}
}

func TestAdminReportsScopesByOrg(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	f := seedReportFixture(t, p)
	w := store.DefaultReportWindow(time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC))

	rA, err := p.AdminReports(ctx, f.OrgA.ID, w)
	if err != nil {
		t.Fatal(err)
	}
	if rA.TripStatusCounts.Planned != 1 || rA.TripStatusCounts.Active != 1 || rA.TripStatusCounts.Completed != 1 {
		t.Errorf("orgA status counts = %+v want planned=1, active=1, completed=1", rA.TripStatusCounts)
	}
	if len(rA.TripOperational) != 3 {
		t.Errorf("orgA operational rows = %d want 3", len(rA.TripOperational))
	}
	for _, row := range rA.TripOperational {
		if row.TripID == f.TripB.ID {
			t.Errorf("orgA leaked orgB trip %v", row.TripID)
		}
	}

	rB, err := p.AdminReports(ctx, f.OrgB.ID, w)
	if err != nil {
		t.Fatal(err)
	}
	if rB.TripStatusCounts.Active != 1 || rB.TripStatusCounts.Planned != 0 {
		t.Errorf("orgB status counts = %+v want active=1", rB.TripStatusCounts)
	}
	for _, row := range rB.TripRevenue {
		if row.TripID != f.TripB.ID {
			t.Errorf("orgB report leaked trip %v (not in org)", row.TripID)
		}
	}
}

func TestTripRevenueExcludesVoidedAndSplitsCrewTips(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	f := seedReportFixture(t, p)
	w := store.DefaultReportWindow(time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC))

	r, err := p.AdminReports(ctx, f.OrgA.ID, w)
	if err != nil {
		t.Fatal(err)
	}
	var active *store.TripRevenueRow
	for i := range r.TripRevenue {
		if r.TripRevenue[i].TripID == f.TripActive.ID {
			active = &r.TripRevenue[i]
			break
		}
	}
	if active == nil {
		t.Fatal("active trip not in revenue rows")
	}

	// Charges = subtotal across folios (open + closed). Closed folio has
	// subtotal 11000 (Beer 10000 + crew_tip 1000), open folio has 4000.
	if active.ChargesUSDCents != 11000+4000 {
		t.Errorf("ChargesUSDCents=%d want %d", active.ChargesUSDCents, 11000+4000)
	}
	if active.SettledUSDCents != 11330 { // closed folio total
		t.Errorf("SettledUSDCents=%d want 11330", active.SettledUSDCents)
	}
	if active.OutstandingUSDCents != 4000 {
		t.Errorf("OutstandingUSDCents=%d want 4000", active.OutstandingUSDCents)
	}
	if active.CrewTipUSDCents != 1000 {
		t.Errorf("CrewTipUSDCents=%d want 1000", active.CrewTipUSDCents)
	}
	if active.OpenFolioCount != 1 || active.ClosedFolioCount != 1 {
		t.Errorf("folio counts open=%d closed=%d want 1/1", active.OpenFolioCount, active.ClosedFolioCount)
	}
	if active.VoidedLineCount != 1 {
		t.Errorf("VoidedLineCount=%d want 1", active.VoidedLineCount)
	}
	if active.VoidedUSDCents != 500 {
		t.Errorf("VoidedUSDCents=%d want 500", active.VoidedUSDCents)
	}
	if len(active.SettlementByCurrency) != 1 || active.SettlementByCurrency[0].Currency != "EUR" {
		t.Errorf("SettlementByCurrency=%+v want one EUR row", active.SettlementByCurrency)
	}
	if active.SettlementByCurrency[0].TotalMinor != 10800 {
		t.Errorf("EUR TotalMinor=%d want 10800", active.SettlementByCurrency[0].TotalMinor)
	}
}

func TestTripDashboardScopesByOrg(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	f := seedReportFixture(t, p)

	d, err := p.TripDashboard(ctx, f.OrgA.ID, f.TripActive.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Occupancy.GuestCount != 2 {
		t.Errorf("GuestCount=%d want 2", d.Occupancy.GuestCount)
	}
	if d.FolioTotals.OpenCount != 1 || d.FolioTotals.ClosedCount != 1 {
		t.Errorf("folio counts open=%d closed=%d", d.FolioTotals.OpenCount, d.FolioTotals.ClosedCount)
	}
	// Top items: only catalog_item, non-voided. Beer 4@2500 + Beer 2@2000 = qty 6, $140.
	if len(d.TopItems) == 0 {
		t.Fatal("expected at least one top item row")
	}
	// First row should be Beer (highest USD).
	if d.TopItems[0].ItemName != "Beer" {
		t.Errorf("top item ItemName=%q want Beer", d.TopItems[0].ItemName)
	}
	if d.TopItems[0].USDCents != 14000 {
		t.Errorf("top item USDCents=%d want 14000", d.TopItems[0].USDCents)
	}

	// Cross-org call returns ErrNotFound for org B's trip.
	if _, err := p.TripDashboard(ctx, f.OrgA.ID, f.TripB.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("cross-org TripDashboard err=%v want ErrNotFound", err)
	}
}

func TestGuestTabRequiresOwnership(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	f := seedReportFixture(t, p)

	// Owner sees their tab.
	tab, err := p.GuestTab(ctx, f.GuestUserA, f.TripGuestA1)
	if err != nil {
		t.Fatalf("owner GuestTab: %v", err)
	}
	if !tab.HasFolio {
		t.Errorf("expected HasFolio=true")
	}
	if tab.Status != "closed" {
		t.Errorf("Status=%q want closed", tab.Status)
	}
	if tab.TotalUSD != 11330 {
		t.Errorf("TotalUSD=%d want 11330", tab.TotalUSD)
	}
	// Voided line excluded.
	for _, ln := range tab.Lines {
		if ln.ItemName == "Souvenir" {
			t.Errorf("voided line leaked into guest tab: %+v", ln)
		}
	}
	if tab.Settlement == nil || tab.Settlement.Currency != "EUR" {
		t.Errorf("Settlement=%+v want EUR", tab.Settlement)
	}

	// Different guest user calling the same trip_guest_id → ErrNotFound.
	if _, err := p.GuestTab(ctx, f.GuestUserA2, f.TripGuestA1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("foreign guest GuestTab err=%v want ErrNotFound", err)
	}

	// Cross-org guest trying to read another org's trip_guest → ErrNotFound.
	if _, err := p.GuestTab(ctx, f.GuestUserB, f.TripGuestA1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("cross-org guest tab err=%v want ErrNotFound", err)
	}
}

func TestGuestTabEmptyFolio(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	f := seedReportFixture(t, p)

	// Create a new trip_guest for guest A on the planned trip with no folio yet.
	tg := insertTripGuest(t, p, f.OrgA.ID, f.TripPlanned.ID, f.GuestUserA, "Aya-future")
	tab, err := p.GuestTab(ctx, f.GuestUserA, tg)
	if err != nil {
		t.Fatal(err)
	}
	if tab.HasFolio {
		t.Errorf("expected HasFolio=false when no folio exists")
	}
	if len(tab.Lines) != 0 {
		t.Errorf("expected empty lines, got %d", len(tab.Lines))
	}
	if tab.Settlement != nil {
		t.Errorf("expected no settlement for open/none folio")
	}
}

func TestReportWindowValidate(t *testing.T) {
	bad := store.ReportWindow{
		From: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := bad.Validate(); !errors.Is(err, store.ErrReportWindowTooWide) {
		t.Errorf("wide window err=%v want ErrReportWindowTooWide", err)
	}
	good := store.DefaultReportWindow(time.Now().UTC())
	if err := good.Validate(); err != nil {
		t.Errorf("default window failed: %v", err)
	}
	rev := store.ReportWindow{
		From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := rev.Validate(); err == nil {
		t.Errorf("reversed window should fail")
	}
}
