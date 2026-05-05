package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Trip is one upcoming departure for a boat. It is tenant-scoped via
// organization_id (denormalized from boats so cross-tenant queries
// are a single index scan).
type Trip struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	BoatID         uuid.UUID

	StartDate time.Time
	EndDate   time.Time
	Itinerary string

	DeparturePort *string
	ReturnPort    *string

	PriceText        *string
	AvailabilityText *string

	// NumGuests is the expected guest count for the trip (Sprint 012).
	// Spreadsheet imports are the authoritative source for this field;
	// the liveaboard.com scraper never touches it.
	NumGuests *int

	SourceProvider     string
	SourceTripKey      string
	SourceURL          string
	SourceLastSyncedAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TripScrape is the input shape produced by the parser for one trip row.
type TripScrape struct {
	StartDate        time.Time
	EndDate          time.Time
	Itinerary        string
	DeparturePort    string
	ReturnPort       string
	PriceText        string
	AvailabilityText string
	SourceURL        string
	SourceTripKey    string

	// NumGuests is honored by ReplaceSpreadsheetTrips and ignored by
	// ReplaceFutureScrapedTrips. Pass nil to leave the column as-is on
	// update / NULL on insert.
	NumGuests *int
}

// ReplaceFutureScrapedTripsResult reports the outcome of a scrape
// reconciliation pass.
type ReplaceFutureScrapedTripsResult struct {
	Inserts      int
	Updates      int
	StaleDeletes int
}

const tripColumns = `id, organization_id, boat_id,
	start_date, end_date, itinerary,
	departure_port, return_port,
	price_text, availability_text,
	num_guests,
	source_provider, source_trip_key, source_url, source_last_synced_at,
	created_at, updated_at`

// prefixedTripColumns returns tripColumns with each name prefixed by
// `<alias>.`. Used by joined queries (TripsForUser,
// TripsNeedingAttention) so the SELECT list disambiguates against
// the join target.
func prefixedTripColumns(alias string) string {
	cols := []string{
		"id", "organization_id", "boat_id",
		"start_date", "end_date", "itinerary",
		"departure_port", "return_port",
		"price_text", "availability_text",
		"num_guests",
		"source_provider", "source_trip_key", "source_url", "source_last_synced_at",
		"created_at", "updated_at",
	}
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += alias + "." + c
	}
	return out
}

func scanTrip(row interface {
	Scan(dest ...any) error
}, t *Trip) error {
	return row.Scan(
		&t.ID, &t.OrganizationID, &t.BoatID,
		&t.StartDate, &t.EndDate, &t.Itinerary,
		&t.DeparturePort, &t.ReturnPort,
		&t.PriceText, &t.AvailabilityText,
		&t.NumGuests,
		&t.SourceProvider, &t.SourceTripKey, &t.SourceURL, &t.SourceLastSyncedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
}

// ReplaceFutureScrapedTrips upserts every scraped trip for a boat in a
// single transaction and then deletes any trip for that boat from the
// same source provider whose start_date is on or after `today` and
// whose source_trip_key was NOT in the just-touched set. That makes the
// scrape authoritative for the imported window.
//
// Returns counts of inserts, updates, and stale deletes.
func (p *Pool) ReplaceFutureScrapedTrips(
	ctx context.Context,
	orgID, boatID uuid.UUID,
	sourceProvider string,
	scrapes []TripScrape,
	syncedAt time.Time,
	today time.Time,
) (*ReplaceFutureScrapedTripsResult, error) {
	if sourceProvider == "" {
		return nil, errors.New("store.ReplaceFutureScrapedTrips: source_provider required")
	}

	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	res := &ReplaceFutureScrapedTripsResult{}
	touched := make([]string, 0, len(scrapes))

	for i := range scrapes {
		s := scrapes[i]
		if s.SourceTripKey == "" {
			return nil, fmt.Errorf("trip %d has empty source_trip_key", i)
		}
		if s.EndDate.Before(s.StartDate) {
			return nil, fmt.Errorf("trip %s has end_date before start_date", s.SourceTripKey)
		}

		var inserted bool
		err := tx.QueryRow(ctx, `
			INSERT INTO trips (
				organization_id, boat_id,
				start_date, end_date, itinerary,
				departure_port, return_port,
				price_text, availability_text,
				source_provider, source_trip_key, source_url, source_last_synced_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (boat_id, source_provider, source_trip_key) DO UPDATE SET
				start_date            = EXCLUDED.start_date,
				end_date              = EXCLUDED.end_date,
				itinerary             = EXCLUDED.itinerary,
				departure_port        = EXCLUDED.departure_port,
				return_port           = EXCLUDED.return_port,
				price_text            = EXCLUDED.price_text,
				availability_text     = EXCLUDED.availability_text,
				source_url            = EXCLUDED.source_url,
				source_last_synced_at = EXCLUDED.source_last_synced_at,
				updated_at            = now()
			RETURNING (xmax = 0) AS inserted
		`,
			orgID, boatID,
			s.StartDate, s.EndDate, s.Itinerary,
			nullableString(s.DeparturePort), nullableString(s.ReturnPort),
			nullableString(s.PriceText), nullableString(s.AvailabilityText),
			sourceProvider, s.SourceTripKey, s.SourceURL, syncedAt,
		).Scan(&inserted)
		if err != nil {
			return nil, fmt.Errorf("upsert trip %s: %w", s.SourceTripKey, err)
		}
		if inserted {
			res.Inserts++
		} else {
			res.Updates++
		}
		touched = append(touched, s.SourceTripKey)
	}

	// Stale-trip reconciliation: future trips for this boat from the
	// same source that we did NOT touch this run are deleted. The empty
	// touched-list case still works (`<> ALL($3::text[])` against an
	// empty array is true for every row, so all future rows go).
	tag, err := tx.Exec(ctx, `
		DELETE FROM trips
		WHERE boat_id = $1
		  AND source_provider = $2
		  AND start_date >= $3
		  AND source_trip_key <> ALL($4::text[])
	`, boatID, sourceProvider, today, touched)
	if err != nil {
		return nil, fmt.Errorf("delete stale trips: %w", err)
	}
	res.StaleDeletes = int(tag.RowsAffected())

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

// ReplaceSpreadsheetTrips upserts spreadsheet-sourced trips grouped by
// boat in a single transaction, with per-boat stale-delete: for each
// (org, boat, source_provider='spreadsheet') triple, the file is the
// authoritative schedule for THAT boat's future spreadsheet trips.
// Boats not present in the upload are untouched.
//
// Differs from ReplaceFutureScrapedTrips in two ways:
//
//  1. INSERT and UPDATE both write num_guests from the input. The
//     spreadsheet is the authoritative source for this field; a
//     corrected upload can fix the count.
//  2. Operates on multiple boats per call (the upload mixes vessels);
//     ReplaceFutureScrapedTrips is per-boat.
//
// scrapesByBoat keys are boat UUIDs already resolved from the
// vessel-name mapping in the wizard.
func (p *Pool) ReplaceSpreadsheetTrips(
	ctx context.Context,
	orgID uuid.UUID,
	scrapesByBoat map[uuid.UUID][]TripScrape,
	syncedAt time.Time,
	today time.Time,
) (*ReplaceFutureScrapedTripsResult, error) {
	const sourceProvider = "spreadsheet"

	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	res := &ReplaceFutureScrapedTripsResult{}

	for boatID, scrapes := range scrapesByBoat {
		touched := make([]string, 0, len(scrapes))

		for i := range scrapes {
			s := scrapes[i]
			if s.SourceTripKey == "" {
				return nil, fmt.Errorf("trip %d for boat %s has empty source_trip_key", i, boatID)
			}
			if s.EndDate.Before(s.StartDate) {
				return nil, fmt.Errorf("trip %s has end_date before start_date", s.SourceTripKey)
			}

			var inserted bool
			err := tx.QueryRow(ctx, `
				INSERT INTO trips (
					organization_id, boat_id,
					start_date, end_date, itinerary,
					departure_port, return_port,
					price_text, availability_text,
					num_guests,
					source_provider, source_trip_key, source_url, source_last_synced_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
				ON CONFLICT (boat_id, source_provider, source_trip_key) DO UPDATE SET
					start_date            = EXCLUDED.start_date,
					end_date              = EXCLUDED.end_date,
					itinerary             = EXCLUDED.itinerary,
					departure_port        = EXCLUDED.departure_port,
					return_port           = EXCLUDED.return_port,
					price_text            = EXCLUDED.price_text,
					availability_text     = EXCLUDED.availability_text,
					num_guests            = EXCLUDED.num_guests,
					source_url            = EXCLUDED.source_url,
					source_last_synced_at = EXCLUDED.source_last_synced_at,
					updated_at            = now()
				RETURNING (xmax = 0) AS inserted
			`,
				orgID, boatID,
				s.StartDate, s.EndDate, s.Itinerary,
				nullableString(s.DeparturePort), nullableString(s.ReturnPort),
				nullableString(s.PriceText), nullableString(s.AvailabilityText),
				s.NumGuests,
				sourceProvider, s.SourceTripKey, s.SourceURL, syncedAt,
			).Scan(&inserted)
			if err != nil {
				return nil, fmt.Errorf("upsert trip %s: %w", s.SourceTripKey, err)
			}
			if inserted {
				res.Inserts++
			} else {
				res.Updates++
			}
			touched = append(touched, s.SourceTripKey)
		}

		// Per-boat stale-delete: future spreadsheet trips for this
		// boat that aren't in the upload are removed.
		tag, err := tx.Exec(ctx, `
			DELETE FROM trips
			WHERE boat_id = $1
			  AND source_provider = $2
			  AND start_date >= $3
			  AND source_trip_key <> ALL($4::text[])
		`, boatID, sourceProvider, today, touched)
		if err != nil {
			return nil, fmt.Errorf("delete stale trips for boat %s: %w", boatID, err)
		}
		res.StaleDeletes += int(tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

// TripsForBoat returns every trip for a boat ordered by start_date.
// Used by the dashboard once it surfaces a boat's schedule.
func (p *Pool) TripsForBoat(ctx context.Context, boatID uuid.UUID) ([]*Trip, error) {
	rows, err := p.Query(ctx, `
		SELECT `+tripColumns+`
		FROM trips
		WHERE boat_id = $1
		ORDER BY start_date
	`, boatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Trip
	for rows.Next() {
		t := &Trip{}
		if err := scanTrip(rows, t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TripsByOrgInRange returns trips for an org with start_date in
// [from, to). Used by future reporting endpoints.
func (p *Pool) TripsByOrgInRange(
	ctx context.Context,
	orgID uuid.UUID,
	from, to time.Time,
) ([]*Trip, error) {
	rows, err := p.Query(ctx, `
		SELECT `+tripColumns+`
		FROM trips
		WHERE organization_id = $1
		  AND start_date >= $2 AND start_date < $3
		ORDER BY start_date
	`, orgID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Trip
	for rows.Next() {
		t := &Trip{}
		if err := scanTrip(rows, t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TripsForUser returns trips a Cruise Director is assigned to, scoped
// to the org. Sprint 013 — joins through trip_cruise_directors so the
// 1:N model works.
func (p *Pool) TripsForUser(ctx context.Context, orgID, userID uuid.UUID) ([]*Trip, error) {
	rows, err := p.Query(ctx, `
		SELECT `+prefixedTripColumns("t")+`
		FROM trips t
		JOIN trip_cruise_directors tcd ON tcd.trip_id = t.id
		WHERE t.organization_id = $1 AND tcd.user_id = $2
		ORDER BY t.start_date
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Trip
	for rows.Next() {
		t := &Trip{}
		if err := scanTrip(rows, t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AssignCruiseDirector adds a (trip, user) pair to the join table.
// Idempotent — INSERT ... ON CONFLICT DO NOTHING — and returns
// (added=true) only on a fresh assignment so the handler can
// suppress duplicate emails on retry. assignedBy is the org admin
// who triggered the change; nil if unknown.
//
// Cross-org checks are caller responsibility: callers must verify
// both the trip and the user belong to orgID before calling.
func (p *Pool) AssignCruiseDirector(ctx context.Context, tripID, userID uuid.UUID, assignedBy *uuid.UUID) (added bool, err error) {
	tag, err := p.Exec(ctx, `
		INSERT INTO trip_cruise_directors (trip_id, user_id, assigned_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (trip_id, user_id) DO NOTHING
	`, tripID, userID, assignedBy)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// UnassignCruiseDirector removes a (trip, user) pair. Returns
// (removed=true) only when an actual row was deleted, so the
// handler can skip the email if nothing changed.
func (p *Pool) UnassignCruiseDirector(ctx context.Context, tripID, userID uuid.UUID) (removed bool, err error) {
	tag, err := p.Exec(ctx, `
		DELETE FROM trip_cruise_directors
		WHERE trip_id = $1 AND user_id = $2
	`, tripID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// CruiseDirectorIDsForTrip returns the user IDs assigned to a trip,
// in stable assignment-order. Used to build the trip JSON response.
func (p *Pool) CruiseDirectorIDsForTrip(ctx context.Context, tripID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := p.Query(ctx, `
		SELECT user_id FROM trip_cruise_directors
		WHERE trip_id = $1
		ORDER BY assigned_at, user_id
	`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// CruiseDirectorAssignmentsByTrip is a bulk lookup for the trip
// list views: returns a map[trip_id] -> []user_id for every trip
// in `tripIDs`. Saves N+1 round-trips when rendering a long list.
// Empty tripIDs returns an empty map without a query.
func (p *Pool) CruiseDirectorAssignmentsByTrip(ctx context.Context, tripIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := map[uuid.UUID][]uuid.UUID{}
	if len(tripIDs) == 0 {
		return out, nil
	}
	rows, err := p.Query(ctx, `
		SELECT trip_id, user_id FROM trip_cruise_directors
		WHERE trip_id = ANY($1::uuid[])
		ORDER BY assigned_at, user_id
	`, tripIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tid, uid uuid.UUID
		if err := rows.Scan(&tid, &uid); err != nil {
			return nil, err
		}
		out[tid] = append(out[tid], uid)
	}
	return out, rows.Err()
}

// TripsNeedingAttention returns planned trips for an org with either
// no cruise director assigned or a manifest fill ratio below 50%.
// Manifest fill is not yet a real column; "needs attention" is
// "zero directors assigned and start_date in the next 90 days".
// Sprint 013 reframed director presence as a NOT EXISTS check
// against the join table.
func (p *Pool) TripsNeedingAttention(ctx context.Context, orgID uuid.UUID, today time.Time) ([]*Trip, error) {
	rows, err := p.Query(ctx, `
		SELECT `+prefixedTripColumns("t")+`
		FROM trips t
		WHERE t.organization_id = $1
		  AND t.start_date BETWEEN $2 AND $2 + INTERVAL '90 days'
		  AND NOT EXISTS (
		      SELECT 1 FROM trip_cruise_directors tcd
		      WHERE tcd.trip_id = t.id
		  )
		ORDER BY t.start_date
		LIMIT 10
	`, orgID, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Trip
	for rows.Next() {
		t := &Trip{}
		if err := scanTrip(rows, t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TripCountForOrg returns the count of trips for an org. Used by the
// Overview's setup completeness card.
func (p *Pool) TripCountForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM trips WHERE organization_id = $1`, orgID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// TripByID returns a single trip scoped to its org.
func (p *Pool) TripByID(ctx context.Context, orgID, tripID uuid.UUID) (*Trip, error) {
	t := &Trip{}
	err := scanTrip(p.QueryRow(ctx, `
		SELECT `+tripColumns+` FROM trips WHERE id = $1 AND organization_id = $2
	`, tripID, orgID), t)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}
