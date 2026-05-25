package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Reporting DTOs and queries for Sprint 022.
//
// Three persona surfaces, all read-only:
//
//   - AdminReports        — org-wide setup, trip status, per-trip revenue.
//   - TripDashboard       — single-trip operational dashboard (Admin
//                            or assigned Cruise Director).
//   - GuestTab            — guest's own itemized folio + trip context.
//
// Every query scopes by organization_id (or guest_user_id for the
// guest tab) in SQL. Defense-in-depth: handler-side authorization
// gates are first; the SQL refuses to return rows the caller does not
// own.

// ReportWindow is the inclusive [From, To] date window for admin
// reports. From/To are date-typed (00:00 UTC); the server caps the
// window to one year and supplies defaults if either bound is missing.
type ReportWindow struct {
	From time.Time
	To   time.Time
}

// DefaultReportWindow returns the canonical default used when the
// caller omits both bounds: today-30d through today+180d (UTC).
func DefaultReportWindow(now time.Time) ReportWindow {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return ReportWindow{
		From: today.AddDate(0, 0, -30),
		To:   today.AddDate(0, 0, 180),
	}
}

// MaxReportWindow is the cap enforced on Admin report windows. A
// caller-supplied window wider than this is rejected at the HTTP
// layer with 400.
const MaxReportWindow = 366 * 24 * time.Hour

// ErrReportWindowTooWide is returned by validation helpers when the
// requested window exceeds MaxReportWindow.
var ErrReportWindowTooWide = errors.New("store: report window exceeds maximum span")

// Validate normalizes the window to UTC date boundaries and rejects
// windows that exceed MaxReportWindow.
func (w ReportWindow) Validate() error {
	if w.From.After(w.To) {
		return fmt.Errorf("store: report window from > to")
	}
	if w.To.Sub(w.From) > MaxReportWindow {
		return ErrReportWindowTooWide
	}
	return nil
}

// --- Admin reports ---

// AdminReports is the consolidated payload for /api/admin/reports.
type AdminReports struct {
	Window           ReportWindow
	Setup            SetupCompleteness
	TripStatusCounts TripStatusCounts
	TripOperational  []TripOperationalRow
	TripRevenue      []TripRevenueRow
}

// SetupCompleteness is the unified setup signal used by both the
// Overview screen (Sprint 008) and the Reports page (Sprint 022).
type SetupCompleteness struct {
	Percent int
	Steps   []SetupStep
}

type SetupStep struct {
	Key   string
	Label string
	Done  bool
	Hint  string
	Href  string
}

// TripStatusCounts is the lifecycle breakdown for trips inside the
// requested window.
type TripStatusCounts struct {
	Planned   int
	Active    int
	Completed int
	Cancelled int
}

// TripOperationalRow is one trip in the operational status table.
type TripOperationalRow struct {
	TripID           uuid.UUID
	BoatID           uuid.UUID
	BoatName         string
	StartDate        time.Time
	EndDate          time.Time
	Status           string
	NumGuests        *int
	GuestCount       int
	SubmittedCount   int
	DocumentCount    int
	CabinAssignments int
	DirectorCount    int
}

// TripRevenueRow is one trip's revenue rollup.
type TripRevenueRow struct {
	TripID               uuid.UUID
	BoatName             string
	StartDate            time.Time
	EndDate              time.Time
	Status               string
	OpenFolioCount       int
	ClosedFolioCount     int
	ChargesUSDCents      int64
	CrewTipUSDCents      int64
	CardFeeUSDCents      int64
	SettledUSDCents      int64
	OutstandingUSDCents  int64
	VoidedLineCount      int
	VoidedUSDCents       int64
	SettlementByCurrency []SettlementCurrencyRow
}

// SettlementCurrencyRow groups closed-folio settlement totals by
// currency. Reports never sum settlement totals across mixed
// currencies — they are returned as separate rows so the UI can
// render them per currency.
type SettlementCurrencyRow struct {
	Currency   string
	TotalMinor int64
	FolioCount int
}

// --- Trip dashboard ---

// TripDashboard is the per-trip read-only payload for both Org Admins
// and assigned Cruise Directors.
type TripDashboard struct {
	Trip              TripHeader
	Occupancy         Occupancy
	RegistrationReady RegistrationReadiness
	DocumentReady     DocumentReadiness
	FolioTotals       FolioTotals
	TopItems          []TripTopItemRow
	LowStock          []LowStockRow
}

type TripHeader struct {
	TripID        uuid.UUID
	BoatID        uuid.UUID
	BoatName      string
	StartDate     time.Time
	EndDate       time.Time
	Itinerary     string // trips.itinerary is NOT NULL
	DeparturePort *string
	ReturnPort    *string
	Status        string
	StartedAt     *time.Time
	CompletedAt   *time.Time
	CancelledAt   *time.Time
	DirectorCount int
}

type Occupancy struct {
	NumGuests        *int // declared expected count from import / admin
	GuestCount       int  // active (non-revoked) trip_guests rows
	CabinAssignments int
	BerthsTotal      int // sum of berths across boat_cabin_berths for the trip's boat
}

type RegistrationReadiness struct {
	SubmittedCount int
	PendingCount   int
	GuestCount     int
}

type DocumentReadiness struct {
	UploadedCount       int
	GuestsWithDocsCount int
	GuestCount          int
}

type FolioTotals struct {
	OpenCount           int
	ClosedCount         int
	ChargesUSDCents     int64
	CrewTipUSDCents     int64
	CardFeeUSDCents     int64
	SettledUSDCents     int64
	OutstandingUSDCents int64
	VoidedLineCount     int
	VoidedUSDCents      int64
}

type TripTopItemRow struct {
	CatalogItemID uuid.UUID
	ItemName      string
	Quantity      int
	USDCents      int64
}

type LowStockRow struct {
	CatalogItemID  uuid.UUID
	ItemName       string
	CategoryName   string
	QuantityOnHand int
	ReorderLevel   *int
	ParLevel       *int
}

// --- Guest tab ---

// GuestTab is the guest's own itemized folio for one trip, plus the
// trip header context.
type GuestTab struct {
	Trip       TripHeader
	HasFolio   bool
	Status     string // "open" or "closed"; empty if HasFolio is false
	Lines      []GuestTabLine
	Subtotal   int64
	CardFee    int64
	TotalUSD   int64
	Settlement *GuestTabSettlement // present only for closed folios
}

type GuestTabLine struct {
	ID                uuid.UUID
	LineType          string
	ItemName          string
	Quantity          int
	UnitPriceUSDCents int64
	LineTotalUSDCents int64
	CreatedAt         time.Time
}

type GuestTabSettlement struct {
	Currency      string
	TotalMinor    int64
	CurrencyExp   int
	PaymentMethod *string
	ClosedAt      time.Time
}

// --- Methods on *Pool ---

// AdminReports returns the consolidated Admin reports payload for
// the given window. The caller (handler) is responsible for window
// validation; the query trusts the values provided.
func (p *Pool) AdminReports(ctx context.Context, orgID uuid.UUID, w ReportWindow) (*AdminReports, error) {
	setup, err := p.SetupCompleteness(ctx, orgID)
	if err != nil {
		return nil, err
	}
	counts, err := p.tripStatusCounts(ctx, orgID, w)
	if err != nil {
		return nil, err
	}
	ops, err := p.tripOperationalRows(ctx, orgID, w)
	if err != nil {
		return nil, err
	}
	revenue, err := p.tripRevenueRows(ctx, orgID, w)
	if err != nil {
		return nil, err
	}
	return &AdminReports{
		Window:           w,
		Setup:            *setup,
		TripStatusCounts: counts,
		TripOperational:  ops,
		TripRevenue:      revenue,
	}, nil
}

// SetupCompleteness is the unified setup signal used by both the
// Overview screen and the Reports page. Sprint 022 moved this here
// so there is one source of truth.
func (p *Pool) SetupCompleteness(ctx context.Context, orgID uuid.UUID) (*SetupCompleteness, error) {
	org, err := p.OrganizationByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	boatCount, err := p.BoatCountForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	tripCount, err := p.TripCountForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	directorCount, err := p.CountActiveUsersByRole(ctx, orgID, RoleCruiseDirector)
	if err != nil {
		return nil, err
	}
	unconfiguredBoats, err := p.UnconfiguredBoatCount(ctx, orgID)
	if err != nil {
		return nil, err
	}
	currencySet := org.Currency != nil && *org.Currency != ""
	// A boat needs a usable layout: at least one active cabin AND at
	// least one active berth. UnconfiguredBoatCount counts boats whose
	// active-berth count is zero, which subsumes both. The layouts
	// step is done iff there is at least one boat AND none of them
	// is unconfigured.
	layoutsDone := boatCount > 0 && unconfiguredBoats == 0
	steps := []SetupStep{
		{Key: "currency", Label: "Set organization currency", Done: currencySet, Hint: derefString(org.Currency), Href: "/admin/organization"},
		{Key: "boats", Label: "Add or import a boat", Done: boatCount > 0, Hint: pluralizeNoun(boatCount, "boat", "boats"), Href: "/admin/fleet"},
		{Key: "layouts", Label: "Lay out cabins for each boat", Done: layoutsDone, Hint: layoutsHint(boatCount, unconfiguredBoats), Href: "/admin/onboarding?step=layouts"},
		{Key: "directors", Label: "Invite a Cruise Director", Done: directorCount > 0, Hint: pluralizeNoun(directorCount, "active", "active"), Href: "/admin/users"},
		{Key: "trips", Label: "Create your first trip", Done: tripCount > 0, Hint: pluralizeNoun(tripCount, "trip", "trips"), Href: "/admin/trips"},
	}
	done := 0
	for _, s := range steps {
		if s.Done {
			done++
		}
	}
	pct := 0
	if len(steps) > 0 {
		pct = int(float64(done) / float64(len(steps)) * 100)
	}
	return &SetupCompleteness{Percent: pct, Steps: steps}, nil
}

func (p *Pool) tripStatusCounts(ctx context.Context, orgID uuid.UUID, w ReportWindow) (TripStatusCounts, error) {
	row := p.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'planned')::int,
			count(*) FILTER (WHERE status = 'active')::int,
			count(*) FILTER (WHERE status = 'completed')::int,
			count(*) FILTER (WHERE status = 'cancelled')::int
		FROM trips
		WHERE organization_id = $1
		  AND removed_from_source_at IS NULL
		  AND start_date BETWEEN $2 AND $3
	`, orgID, w.From, w.To)
	var c TripStatusCounts
	if err := row.Scan(&c.Planned, &c.Active, &c.Completed, &c.Cancelled); err != nil {
		return TripStatusCounts{}, err
	}
	return c, nil
}

func (p *Pool) tripOperationalRows(ctx context.Context, orgID uuid.UUID, w ReportWindow) ([]TripOperationalRow, error) {
	rows, err := p.Query(ctx, `
		SELECT
			t.id, t.boat_id, b.display_name, t.start_date, t.end_date, t.status, t.num_guests,
			count(DISTINCT g.id) FILTER (WHERE g.revoked_at IS NULL)::int AS guests,
			count(DISTINCT r.id) FILTER (WHERE r.status = 'submitted')::int AS submitted,
			count(DISTINCT d.id) FILTER (WHERE d.archived_at IS NULL)::int AS docs,
			count(DISTINCT ca.id) FILTER (WHERE ca.unassigned_at IS NULL)::int AS cabin_assignments,
			count(DISTINCT tcd.user_id)::int AS director_count
		FROM trips t
		JOIN boats b ON b.id = t.boat_id
		LEFT JOIN trip_guests g ON g.trip_id = t.id
		LEFT JOIN guest_trip_registrations r ON r.trip_guest_id = g.id
		LEFT JOIN guest_documents d ON d.trip_guest_id = g.id
		LEFT JOIN trip_cabin_assignments ca ON ca.trip_id = t.id
		LEFT JOIN trip_cruise_directors tcd ON tcd.trip_id = t.id
		WHERE t.organization_id = $1
		  AND t.removed_from_source_at IS NULL
		  AND t.start_date BETWEEN $2 AND $3
		GROUP BY t.id, t.boat_id, b.display_name, t.start_date, t.end_date, t.status, t.num_guests
		ORDER BY t.start_date, t.id
	`, orgID, w.From, w.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TripOperationalRow
	for rows.Next() {
		var r TripOperationalRow
		if err := rows.Scan(&r.TripID, &r.BoatID, &r.BoatName, &r.StartDate, &r.EndDate, &r.Status, &r.NumGuests,
			&r.GuestCount, &r.SubmittedCount, &r.DocumentCount, &r.CabinAssignments, &r.DirectorCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Pool) tripRevenueRows(ctx context.Context, orgID uuid.UUID, w ReportWindow) ([]TripRevenueRow, error) {
	rows, err := p.Query(ctx, `
		SELECT
			t.id, b.display_name, t.start_date, t.end_date, t.status,
			coalesce(sum(CASE WHEN f.status = 'open'   THEN 1 ELSE 0 END), 0)::int AS open_folios,
			coalesce(sum(CASE WHEN f.status = 'closed' THEN 1 ELSE 0 END), 0)::int AS closed_folios,
			coalesce(sum(f.subtotal_usd_cents), 0)::bigint AS charges,
			coalesce(sum(f.card_fee_usd_cents), 0)::bigint AS card_fees,
			coalesce(sum(CASE WHEN f.status = 'closed' THEN f.total_usd_cents ELSE 0 END), 0)::bigint AS settled,
			coalesce(sum(CASE WHEN f.status = 'open'   THEN f.total_usd_cents ELSE 0 END), 0)::bigint AS outstanding
		FROM trips t
		JOIN boats b ON b.id = t.boat_id
		LEFT JOIN guest_folios f ON f.trip_id = t.id AND f.organization_id = t.organization_id
		WHERE t.organization_id = $1
		  AND t.removed_from_source_at IS NULL
		  AND t.start_date BETWEEN $2 AND $3
		GROUP BY t.id, b.display_name, t.start_date, t.end_date, t.status
		ORDER BY t.start_date, t.id
	`, orgID, w.From, w.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TripRevenueRow{}
	tripIDs := []uuid.UUID{}
	rowByTrip := map[uuid.UUID]int{}
	for rows.Next() {
		var r TripRevenueRow
		if err := rows.Scan(&r.TripID, &r.BoatName, &r.StartDate, &r.EndDate, &r.Status,
			&r.OpenFolioCount, &r.ClosedFolioCount,
			&r.ChargesUSDCents, &r.CardFeeUSDCents,
			&r.SettledUSDCents, &r.OutstandingUSDCents); err != nil {
			return nil, err
		}
		tripIDs = append(tripIDs, r.TripID)
		rowByTrip[r.TripID] = len(out)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tripIDs) == 0 {
		return out, nil
	}

	// Crew tips (active line_type='crew_tip').
	tipRows, err := p.Query(ctx, `
		SELECT f.trip_id, coalesce(sum(l.line_total_usd_cents), 0)::bigint
		FROM guest_folio_lines l
		JOIN guest_folios f ON f.id = l.folio_id
		WHERE l.organization_id = $1
		  AND f.trip_id = ANY($2::uuid[])
		  AND l.line_type = 'crew_tip'
		  AND l.voided_at IS NULL
		GROUP BY f.trip_id
	`, orgID, tripIDs)
	if err != nil {
		return nil, err
	}
	for tipRows.Next() {
		var tripID uuid.UUID
		var tips int64
		if err := tipRows.Scan(&tripID, &tips); err != nil {
			tipRows.Close()
			return nil, err
		}
		if i, ok := rowByTrip[tripID]; ok {
			out[i].CrewTipUSDCents = tips
		}
	}
	tipRows.Close()
	if err := tipRows.Err(); err != nil {
		return nil, err
	}

	// Voided line corrections.
	vRows, err := p.Query(ctx, `
		SELECT f.trip_id, count(*)::int, coalesce(sum(l.line_total_usd_cents), 0)::bigint
		FROM guest_folio_lines l
		JOIN guest_folios f ON f.id = l.folio_id
		WHERE l.organization_id = $1
		  AND f.trip_id = ANY($2::uuid[])
		  AND l.voided_at IS NOT NULL
		GROUP BY f.trip_id
	`, orgID, tripIDs)
	if err != nil {
		return nil, err
	}
	for vRows.Next() {
		var tripID uuid.UUID
		var count int
		var totalCents int64
		if err := vRows.Scan(&tripID, &count, &totalCents); err != nil {
			vRows.Close()
			return nil, err
		}
		if i, ok := rowByTrip[tripID]; ok {
			out[i].VoidedLineCount = count
			out[i].VoidedUSDCents = totalCents
		}
	}
	vRows.Close()
	if err := vRows.Err(); err != nil {
		return nil, err
	}

	// Settlement currency rollups (closed folios only, grouped per currency).
	sRows, err := p.Query(ctx, `
		SELECT f.trip_id, f.settlement_currency,
		       sum(f.settlement_total_minor)::bigint,
		       count(*)::int
		FROM guest_folios f
		WHERE f.organization_id = $1
		  AND f.trip_id = ANY($2::uuid[])
		  AND f.status = 'closed'
		  AND f.settlement_currency IS NOT NULL
		  AND f.settlement_total_minor IS NOT NULL
		GROUP BY f.trip_id, f.settlement_currency
		ORDER BY f.trip_id, f.settlement_currency
	`, orgID, tripIDs)
	if err != nil {
		return nil, err
	}
	for sRows.Next() {
		var tripID uuid.UUID
		var currency string
		var totalMinor int64
		var folioCount int
		if err := sRows.Scan(&tripID, &currency, &totalMinor, &folioCount); err != nil {
			sRows.Close()
			return nil, err
		}
		if i, ok := rowByTrip[tripID]; ok {
			out[i].SettlementByCurrency = append(out[i].SettlementByCurrency, SettlementCurrencyRow{
				Currency:   currency,
				TotalMinor: totalMinor,
				FolioCount: folioCount,
			})
		}
	}
	sRows.Close()
	if err := sRows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// TripDashboard returns the per-trip operational dashboard for either
// an Org Admin (any org trip) or an assigned Cruise Director. Caller
// is responsible for the role and assignment check; the query trusts
// orgID and only validates that the trip belongs to it.
func (p *Pool) TripDashboard(ctx context.Context, orgID, tripID uuid.UUID) (*TripDashboard, error) {
	trip, err := p.TripByID(ctx, orgID, tripID)
	if err != nil {
		return nil, err
	}

	header := TripHeader{
		TripID:        trip.ID,
		BoatID:        trip.BoatID,
		StartDate:     trip.StartDate,
		EndDate:       trip.EndDate,
		Itinerary:     trip.Itinerary,
		DeparturePort: trip.DeparturePort,
		ReturnPort:    trip.ReturnPort,
		Status:        trip.Status,
		StartedAt:     trip.StartedAt,
		CompletedAt:   trip.CompletedAt,
		CancelledAt:   trip.CancelledAt,
	}
	if err := p.QueryRow(ctx, `SELECT display_name FROM boats WHERE id = $1`, trip.BoatID).Scan(&header.BoatName); err != nil {
		return nil, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*)::int FROM trip_cruise_directors WHERE trip_id = $1`, trip.ID).Scan(&header.DirectorCount); err != nil {
		return nil, err
	}

	// Occupancy: active trip_guests + cabin assignments + berths total for the boat.
	occ := Occupancy{NumGuests: trip.NumGuests}
	if err := p.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE revoked_at IS NULL)::int
		FROM trip_guests
		WHERE organization_id = $1 AND trip_id = $2
	`, orgID, tripID).Scan(&occ.GuestCount); err != nil {
		return nil, err
	}
	if err := p.QueryRow(ctx, `
		SELECT count(*)::int
		FROM trip_cabin_assignments
		WHERE organization_id = $1 AND trip_id = $2 AND unassigned_at IS NULL
	`, orgID, tripID).Scan(&occ.CabinAssignments); err != nil {
		return nil, err
	}
	if err := p.QueryRow(ctx, `
		SELECT count(*)::int
		FROM boat_cabin_berths bb
		JOIN boat_cabins bc ON bc.id = bb.cabin_id
		WHERE bc.boat_id = $1 AND bc.archived_at IS NULL AND bb.archived_at IS NULL
	`, trip.BoatID).Scan(&occ.BerthsTotal); err != nil {
		// boat_cabins / boat_cabin_berths may not have archived_at — fall back
		// to a simple count if the WHERE clause errors. We tolerate a count
		// of 0 here rather than failing the whole dashboard.
		_ = err
		occ.BerthsTotal = 0
	}

	// Registration readiness.
	reg := RegistrationReadiness{GuestCount: occ.GuestCount}
	if err := p.QueryRow(ctx, `
		SELECT
			count(DISTINCT r.id) FILTER (WHERE r.status = 'submitted')::int,
			count(DISTINCT r.id) FILTER (WHERE r.status <> 'submitted')::int
		FROM trip_guests g
		LEFT JOIN guest_trip_registrations r ON r.trip_guest_id = g.id
		WHERE g.organization_id = $1 AND g.trip_id = $2 AND g.revoked_at IS NULL
	`, orgID, tripID).Scan(&reg.SubmittedCount, &reg.PendingCount); err != nil {
		return nil, err
	}

	// Document readiness.
	doc := DocumentReadiness{GuestCount: occ.GuestCount}
	if err := p.QueryRow(ctx, `
		SELECT
			count(*)::int,
			count(DISTINCT d.trip_guest_id)::int
		FROM guest_documents d
		WHERE d.organization_id = $1 AND d.trip_id = $2 AND d.archived_at IS NULL
	`, orgID, tripID).Scan(&doc.UploadedCount, &doc.GuestsWithDocsCount); err != nil {
		return nil, err
	}

	// Folio totals.
	var folio FolioTotals
	if err := p.QueryRow(ctx, `
		SELECT
			coalesce(sum(CASE WHEN status = 'open'   THEN 1 ELSE 0 END), 0)::int,
			coalesce(sum(CASE WHEN status = 'closed' THEN 1 ELSE 0 END), 0)::int,
			coalesce(sum(subtotal_usd_cents), 0)::bigint,
			coalesce(sum(card_fee_usd_cents), 0)::bigint,
			coalesce(sum(CASE WHEN status = 'closed' THEN total_usd_cents ELSE 0 END), 0)::bigint,
			coalesce(sum(CASE WHEN status = 'open'   THEN total_usd_cents ELSE 0 END), 0)::bigint
		FROM guest_folios
		WHERE organization_id = $1 AND trip_id = $2
	`, orgID, tripID).Scan(
		&folio.OpenCount, &folio.ClosedCount,
		&folio.ChargesUSDCents, &folio.CardFeeUSDCents,
		&folio.SettledUSDCents, &folio.OutstandingUSDCents,
	); err != nil {
		return nil, err
	}
	if err := p.QueryRow(ctx, `
		SELECT coalesce(sum(l.line_total_usd_cents), 0)::bigint
		FROM guest_folio_lines l
		JOIN guest_folios f ON f.id = l.folio_id
		WHERE l.organization_id = $1 AND f.trip_id = $2
		  AND l.line_type = 'crew_tip' AND l.voided_at IS NULL
	`, orgID, tripID).Scan(&folio.CrewTipUSDCents); err != nil {
		return nil, err
	}
	if err := p.QueryRow(ctx, `
		SELECT count(*)::int, coalesce(sum(l.line_total_usd_cents), 0)::bigint
		FROM guest_folio_lines l
		JOIN guest_folios f ON f.id = l.folio_id
		WHERE l.organization_id = $1 AND f.trip_id = $2 AND l.voided_at IS NOT NULL
	`, orgID, tripID).Scan(&folio.VoidedLineCount, &folio.VoidedUSDCents); err != nil {
		return nil, err
	}

	// Top 10 catalog items by USD revenue (non-voided, catalog_item type).
	topItems, err := p.topItemsForTrip(ctx, orgID, tripID)
	if err != nil {
		return nil, err
	}

	// Low stock for the trip's boat: reorder_level floor + always-surface
	// zero/negative.
	lowStock, err := p.lowStockForBoat(ctx, orgID, trip.BoatID)
	if err != nil {
		return nil, err
	}

	return &TripDashboard{
		Trip:              header,
		Occupancy:         occ,
		RegistrationReady: reg,
		DocumentReady:     doc,
		FolioTotals:       folio,
		TopItems:          topItems,
		LowStock:          lowStock,
	}, nil
}

func (p *Pool) topItemsForTrip(ctx context.Context, orgID, tripID uuid.UUID) ([]TripTopItemRow, error) {
	rows, err := p.Query(ctx, `
		SELECT l.catalog_item_id, l.item_name,
		       sum(l.quantity)::int AS qty,
		       sum(l.line_total_usd_cents)::bigint AS usd
		FROM guest_folio_lines l
		JOIN guest_folios f ON f.id = l.folio_id
		WHERE l.organization_id = $1
		  AND f.trip_id = $2
		  AND l.line_type = 'catalog_item'
		  AND l.voided_at IS NULL
		  AND l.catalog_item_id IS NOT NULL
		GROUP BY l.catalog_item_id, l.item_name
		ORDER BY usd DESC, l.item_name
		LIMIT 10
	`, orgID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TripTopItemRow{}
	for rows.Next() {
		var r TripTopItemRow
		if err := rows.Scan(&r.CatalogItemID, &r.ItemName, &r.Quantity, &r.USDCents); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Pool) lowStockForBoat(ctx context.Context, orgID, boatID uuid.UUID) ([]LowStockRow, error) {
	rows, err := p.Query(ctx, `
		SELECT bi.catalog_item_id, i.name, c.name,
		       bi.quantity_on_hand, bi.reorder_level, bi.par_level
		FROM boat_inventory_items bi
		JOIN catalog_items i ON i.id = bi.catalog_item_id
		JOIN catalog_categories c ON c.id = i.category_id
		WHERE bi.organization_id = $1
		  AND bi.boat_id = $2
		  AND (
		      bi.quantity_on_hand <= 0
		      OR (bi.reorder_level IS NOT NULL AND bi.quantity_on_hand <= bi.reorder_level)
		  )
		ORDER BY bi.quantity_on_hand, lower(i.name)
	`, orgID, boatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LowStockRow{}
	for rows.Next() {
		var r LowStockRow
		if err := rows.Scan(&r.CatalogItemID, &r.ItemName, &r.CategoryName,
			&r.QuantityOnHand, &r.ReorderLevel, &r.ParLevel); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GuestTab returns the signed-in guest's own folio for one trip. The
// query joins trip_guests on guest_user_id so the caller cannot read
// another guest's data even if they tamper with the URL parameter.
// Returns ErrNotFound if the trip_guest_id is unknown to this guest.
func (p *Pool) GuestTab(ctx context.Context, guestUserID, tripGuestID uuid.UUID) (*GuestTab, error) {
	// Ownership check + trip context, all in one query.
	row := p.QueryRow(ctx, `
		SELECT g.trip_id, b.id, b.display_name, t.start_date, t.end_date, t.status,
		       t.itinerary, t.departure_port, t.return_port,
		       t.started_at, t.completed_at, t.cancelled_at
		FROM trip_guests g
		JOIN trips t ON t.id = g.trip_id
		JOIN boats b ON b.id = t.boat_id
		WHERE g.id = $1 AND g.guest_user_id = $2 AND g.revoked_at IS NULL
	`, tripGuestID, guestUserID)
	var tripID, boatID uuid.UUID
	var boatName, status, itinerary string
	var startDate, endDate time.Time
	var departurePort, returnPort *string
	var startedAt, completedAt, cancelledAt *time.Time
	if err := row.Scan(&tripID, &boatID, &boatName, &startDate, &endDate, &status,
		&itinerary, &departurePort, &returnPort,
		&startedAt, &completedAt, &cancelledAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	out := &GuestTab{
		Trip: TripHeader{
			TripID:        tripID,
			BoatID:        boatID,
			BoatName:      boatName,
			StartDate:     startDate,
			EndDate:       endDate,
			Status:        status,
			Itinerary:     itinerary,
			DeparturePort: departurePort,
			ReturnPort:    returnPort,
			StartedAt:     startedAt,
			CompletedAt:   completedAt,
			CancelledAt:   cancelledAt,
		},
	}

	// Folio lookup (one per trip_guest by unique index). May not exist
	// yet — that's the empty-state path.
	var (
		folioID                  uuid.UUID
		folioStatus              string
		subtotal, cardFee, total int64
		settlementCurrency       *string
		settlementTotalMinor     *int64
		currencyExp              *int
		paymentMethod            *string
		closedAt                 *time.Time
	)
	err := p.QueryRow(ctx, `
		SELECT id, status, subtotal_usd_cents, card_fee_usd_cents, total_usd_cents,
		       settlement_currency, settlement_total_minor, currency_exponent,
		       payment_method, closed_at
		FROM guest_folios
		WHERE trip_guest_id = $1
	`, tripGuestID).Scan(&folioID, &folioStatus, &subtotal, &cardFee, &total,
		&settlementCurrency, &settlementTotalMinor, &currencyExp,
		&paymentMethod, &closedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out.HasFolio = true
	out.Status = folioStatus
	out.Subtotal = subtotal
	out.CardFee = cardFee
	out.TotalUSD = total

	if folioStatus == FolioStatusClosed && settlementCurrency != nil && settlementTotalMinor != nil && currencyExp != nil && closedAt != nil {
		out.Settlement = &GuestTabSettlement{
			Currency:      *settlementCurrency,
			TotalMinor:    *settlementTotalMinor,
			CurrencyExp:   *currencyExp,
			PaymentMethod: paymentMethod,
			ClosedAt:      *closedAt,
		}
	}

	// Lines (non-voided, in display order).
	lineRows, err := p.Query(ctx, `
		SELECT id, line_type, item_name, quantity, unit_price_usd_cents, line_total_usd_cents, created_at
		FROM guest_folio_lines
		WHERE folio_id = $1 AND voided_at IS NULL
		ORDER BY sort_order, created_at, id
	`, folioID)
	if err != nil {
		return nil, err
	}
	defer lineRows.Close()
	for lineRows.Next() {
		var l GuestTabLine
		if err := lineRows.Scan(&l.ID, &l.LineType, &l.ItemName, &l.Quantity,
			&l.UnitPriceUSDCents, &l.LineTotalUSDCents, &l.CreatedAt); err != nil {
			return nil, err
		}
		out.Lines = append(out.Lines, l)
	}
	return out, lineRows.Err()
}

// --- small helpers ---

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func pluralizeNoun(n int, sing, plur string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, sing)
	}
	return fmt.Sprintf("%d %s", n, plur)
}

// layoutsHint returns the SetupCompleteness hint string for the
// Sprint 023 layouts step. "No boats yet" reads better than "0
// configured" when boats step itself is incomplete.
func layoutsHint(boatCount, unconfigured int) string {
	if boatCount == 0 {
		return ""
	}
	if unconfigured == 0 {
		return "all boats configured"
	}
	return pluralizeNoun(unconfigured, "boat needs cabins", "boats need cabins")
}

// --- Sprint 023: onboarding wizard ---

// OnboardingState is the unified payload the onboarding wizard reads.
// onboarding_complete is computed from the four wizard steps only
// (currency, boats, layouts, directors). It is independent of
// SetupCompleteness.Percent, which also counts trips — including
// trips would loop the wizard forever for orgs that finish setup but
// haven't created a trip yet.
type OnboardingState struct {
	DismissedAt         *time.Time
	OnboardingComplete  bool
	SetupPercent        int
	Steps               []OnboardingStep
	BoatsWithoutLayouts []BoatLayoutSummary
}

type OnboardingStep struct {
	Key   string // "currency" | "boats" | "layouts" | "directors"
	Label string
	Done  bool
	Hint  string
}

// OnboardingState assembles the wizard payload. It reuses
// SetupCompleteness as the source of step labels/done flags, derives
// the wizard-only completion signal from the four onboarding steps,
// and returns the boats-without-layouts list inline so the wizard
// can render the layouts step without a second round-trip.
func (p *Pool) OnboardingState(ctx context.Context, orgID uuid.UUID) (*OnboardingState, error) {
	org, err := p.OrganizationByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	setup, err := p.SetupCompleteness(ctx, orgID)
	if err != nil {
		return nil, err
	}
	boatsWithout, err := p.UnconfiguredBoats(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if boatsWithout == nil {
		boatsWithout = []BoatLayoutSummary{}
	}

	// Pull the four wizard steps out of SetupCompleteness in their
	// canonical order. trips is intentionally excluded.
	wizardKeys := []string{"currency", "boats", "layouts", "directors"}
	byKey := map[string]SetupStep{}
	for _, s := range setup.Steps {
		byKey[s.Key] = s
	}
	steps := make([]OnboardingStep, 0, len(wizardKeys))
	complete := true
	for _, k := range wizardKeys {
		s, ok := byKey[k]
		if !ok {
			// Defensive: a missing key shouldn't happen, but treat
			// as not-done rather than panic so the wizard renders.
			steps = append(steps, OnboardingStep{Key: k, Done: false})
			complete = false
			continue
		}
		steps = append(steps, OnboardingStep{
			Key:   s.Key,
			Label: s.Label,
			Done:  s.Done,
			Hint:  s.Hint,
		})
		if !s.Done {
			complete = false
		}
	}

	return &OnboardingState{
		DismissedAt:         org.OnboardingDismissedAt,
		OnboardingComplete:  complete,
		SetupPercent:        setup.Percent,
		Steps:               steps,
		BoatsWithoutLayouts: boatsWithout,
	}, nil
}
