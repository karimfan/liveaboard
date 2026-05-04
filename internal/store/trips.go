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

	// SiteDirectorUserID is nullable: an unassigned trip is the default
	// state. Sprint 008's Overview "trips needing attention" depends on
	// detecting NULL here.
	SiteDirectorUserID *uuid.UUID

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
	site_director_user_id,
	source_provider, source_trip_key, source_url, source_last_synced_at,
	created_at, updated_at`

func scanTrip(row interface {
	Scan(dest ...any) error
}, t *Trip) error {
	return row.Scan(
		&t.ID, &t.OrganizationID, &t.BoatID,
		&t.StartDate, &t.EndDate, &t.Itinerary,
		&t.DeparturePort, &t.ReturnPort,
		&t.PriceText, &t.AvailabilityText,
		&t.SiteDirectorUserID,
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

// TripsForUser returns trips assigned to a specific user (Site Director),
// scoped to that user's organization. Used for the SD-scoped /api/admin/trips
// response.
func (p *Pool) TripsForUser(ctx context.Context, orgID, userID uuid.UUID) ([]*Trip, error) {
	rows, err := p.Query(ctx, `
		SELECT `+tripColumns+`
		FROM trips
		WHERE organization_id = $1 AND site_director_user_id = $2
		ORDER BY start_date
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

// AssignSiteDirector sets (or clears) the trip's assigned Site Director.
// Pass uuid.Nil to clear the assignment. Tenant scoping: rejects updates
// that would cross organizations.
func (p *Pool) AssignSiteDirector(ctx context.Context, orgID, tripID uuid.UUID, directorUserID *uuid.UUID) error {
	tag, err := p.Exec(ctx, `
		UPDATE trips
		SET site_director_user_id = $3, updated_at = now()
		WHERE id = $1 AND organization_id = $2
	`, tripID, orgID, directorUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TripsNeedingAttention returns planned trips for an org with either
// no site director assigned or a manifest fill ratio below 50%.
// Manifest fill is not yet a real column; for now we treat
// "needs attention" as "no director assigned and start_date in the
// next 90 days".
func (p *Pool) TripsNeedingAttention(ctx context.Context, orgID uuid.UUID, today time.Time) ([]*Trip, error) {
	rows, err := p.Query(ctx, `
		SELECT `+tripColumns+`
		FROM trips
		WHERE organization_id = $1
		  AND start_date BETWEEN $2 AND $2 + INTERVAL '90 days'
		  AND site_director_user_id IS NULL
		ORDER BY start_date
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
