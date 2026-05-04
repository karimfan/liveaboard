package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Boat is a tenant-scoped row representing one operator boat.
//
// The struct mirrors the boats table: DisplayName is operator-owned
// (initialized to SourceName on insert; never overwritten by a re-scrape),
// while every Source* field is rewritten on every successful scrape.
type Boat struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID

	DisplayName string

	SourceProvider     string
	SourceSlug         string
	SourceName         string
	SourceURL          string
	SourceImageURL     *string
	SourceExternalID   *string
	SourceLastSyncedAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// BoatScrape carries the values produced by a single scrape pass. It is
// the input to UpsertBoat.
type BoatScrape struct {
	Slug       string
	Name       string
	URL        string
	ImageURL   string
	ExternalID string
}

const boatColumns = `id, organization_id, display_name,
	source_provider, source_slug, source_name, source_url,
	source_image_url, source_external_id, source_last_synced_at,
	created_at, updated_at`

func scanBoat(row interface {
	Scan(dest ...any) error
}, b *Boat) error {
	return row.Scan(
		&b.ID, &b.OrganizationID, &b.DisplayName,
		&b.SourceProvider, &b.SourceSlug, &b.SourceName, &b.SourceURL,
		&b.SourceImageURL, &b.SourceExternalID, &b.SourceLastSyncedAt,
		&b.CreatedAt, &b.UpdatedAt,
	)
}

// UpsertBoat inserts a boat for the given org or updates the
// scraper-owned fields if a row already exists for
// (organization_id, source_provider, source_slug).
//
// On INSERT, display_name is initialized to scrape.Name. On UPDATE,
// display_name is NEVER touched — the operator owns it.
func (p *Pool) UpsertBoat(
	ctx context.Context,
	orgID uuid.UUID,
	sourceProvider string,
	scrape BoatScrape,
	syncedAt time.Time,
) (*Boat, error) {
	if sourceProvider == "" {
		return nil, errors.New("store.UpsertBoat: source_provider required")
	}
	if scrape.Slug == "" || scrape.Name == "" || scrape.URL == "" {
		return nil, errors.New("store.UpsertBoat: slug, name, and url required")
	}

	imageURL := nullableString(scrape.ImageURL)
	externalID := nullableString(scrape.ExternalID)

	boat := &Boat{}
	err := scanBoat(p.QueryRow(ctx, `
		INSERT INTO boats (
			organization_id, display_name,
			source_provider, source_slug, source_name, source_url,
			source_image_url, source_external_id, source_last_synced_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (organization_id, source_provider, source_slug) DO UPDATE SET
			source_name           = EXCLUDED.source_name,
			source_url            = EXCLUDED.source_url,
			source_image_url      = EXCLUDED.source_image_url,
			source_external_id    = EXCLUDED.source_external_id,
			source_last_synced_at = EXCLUDED.source_last_synced_at,
			updated_at            = now()
		RETURNING `+boatColumns,
		orgID, scrape.Name,
		sourceProvider, scrape.Slug, scrape.Name, scrape.URL,
		imageURL, externalID, syncedAt,
	), boat)
	if err != nil {
		return nil, err
	}
	return boat, nil
}

// BoatBySourceSlug looks up a boat by its (org, provider, slug) tuple.
func (p *Pool) BoatBySourceSlug(
	ctx context.Context,
	orgID uuid.UUID,
	sourceProvider, slug string,
) (*Boat, error) {
	boat := &Boat{}
	err := scanBoat(p.QueryRow(ctx, `
		SELECT `+boatColumns+`
		FROM boats
		WHERE organization_id = $1 AND source_provider = $2 AND source_slug = $3
	`, orgID, sourceProvider, slug), boat)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return boat, nil
}

// BoatsForOrg returns every boat owned by the given org, ordered by
// display_name. Used by the dashboard once it starts surfacing fleet
// data.
func (p *Pool) BoatsForOrg(ctx context.Context, orgID uuid.UUID) ([]*Boat, error) {
	rows, err := p.Query(ctx, `
		SELECT `+boatColumns+`
		FROM boats
		WHERE organization_id = $1
		ORDER BY display_name
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Boat
	for rows.Next() {
		b := &Boat{}
		if err := scanBoat(rows, b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// nullableString turns "" into a NULL stored in the database.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
