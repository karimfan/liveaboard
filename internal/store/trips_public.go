package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PublicTrip is the explicitly-enumerated read shape for the
// /api/public/charters/:slug endpoints. Sprint 026 deliberately
// uses a separate projection from the rich `Trip` struct so the
// public catalog cannot accidentally leak operator-only fields
// (departure_port internal_notes, etc.) — a public-tier read
// goes through this function or it doesn't go out.
type PublicTrip struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	BoatID             uuid.UUID
	BoatName           string
	StartDate          time.Time
	EndDate            time.Time
	Itinerary          string
	PublicTitle        *string
	PublicSummary      *string
	DepositAmountCents *int64
	DepositCurrency    *string
	BerthHoldDays      int
	PublishedAt        time.Time
}

const publicTripColumns = `
	t.id, t.organization_id, t.boat_id, b.name,
	t.start_date, t.end_date, t.itinerary,
	t.public_title, t.public_summary,
	t.deposit_amount_cents, t.deposit_currency, t.berth_hold_days,
	t.published_at
`

func scanPublicTrip(row interface{ Scan(dest ...any) error }, t *PublicTrip) error {
	return row.Scan(
		&t.ID, &t.OrganizationID, &t.BoatID, &t.BoatName,
		&t.StartDate, &t.EndDate, &t.Itinerary,
		&t.PublicTitle, &t.PublicSummary,
		&t.DepositAmountCents, &t.DepositCurrency, &t.BerthHoldDays,
		&t.PublishedAt,
	)
}

// PublicOrgProfile is the shape /api/public/charters/:slug
// returns for the org itself — name + slug only, no PII.
type PublicOrgProfile struct {
	ID         uuid.UUID
	Name       string
	PublicSlug string
}

// GetPublicOrgBySlug resolves a case-insensitive slug to the
// org. Returns ErrNotFound for missing/unpublished orgs.
func (p *Pool) GetPublicOrgBySlug(ctx context.Context, slug string) (*PublicOrgProfile, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, errors.New("public_org: slug required")
	}
	out := &PublicOrgProfile{}
	err := p.QueryRow(ctx, `
		SELECT id, name, public_slug
		FROM organizations
		WHERE public_slug IS NOT NULL AND lower(public_slug) = lower($1)
	`, slug).Scan(&out.ID, &out.Name, &out.PublicSlug)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListPublishedUpcomingTripsForOrg returns trips that are
// published (published_at IS NOT NULL), planned (not active/
// completed/cancelled), and whose end date is in the future.
// Used by the public catalog handler.
func (p *Pool) ListPublishedUpcomingTripsForOrg(ctx context.Context, orgID uuid.UUID, now time.Time) ([]*PublicTrip, error) {
	rows, err := p.Query(ctx, `
		SELECT `+publicTripColumns+`
		FROM trips t
		JOIN boats b ON b.id = t.boat_id
		WHERE t.organization_id = $1
		  AND t.published_at IS NOT NULL
		  AND t.status = 'planned'
		  AND t.end_date >= $2
		ORDER BY t.start_date ASC
	`, orgID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PublicTrip
	for rows.Next() {
		t := &PublicTrip{}
		if err := scanPublicTrip(rows, t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetPublishedTrip returns one trip in the public projection
// shape iff it's published and belongs to the named org.
func (p *Pool) GetPublishedTrip(ctx context.Context, orgID, tripID uuid.UUID) (*PublicTrip, error) {
	out := &PublicTrip{}
	err := scanPublicTrip(p.QueryRow(ctx, `
		SELECT `+publicTripColumns+`
		FROM trips t
		JOIN boats b ON b.id = t.boat_id
		WHERE t.organization_id = $1 AND t.id = $2
		  AND t.published_at IS NOT NULL
	`, orgID, tripID), out)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetOrganizationPublicSlug assigns or updates the public_slug.
// Unique case-insensitively across all orgs; returns ErrInvalidInput
// if the slug is taken or malformed.
func (p *Pool) SetOrganizationPublicSlug(ctx context.Context, orgID uuid.UUID, slug *string) error {
	if slug != nil {
		s := strings.TrimSpace(*slug)
		if s == "" {
			slug = nil
		} else {
			slug = &s
		}
	}
	tag, err := p.Exec(ctx, `
		UPDATE organizations SET public_slug = $1, updated_at = now()
		WHERE id = $2
	`, slug, orgID)
	if err != nil {
		// A unique-violation on the slug index means the slug is taken.
		if strings.Contains(err.Error(), "organizations_public_slug_idx") {
			return ErrInvalidInput
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PublishTrip marks a trip as visible in the public catalog with
// optional public-title/summary + deposit terms. Idempotent —
// calling again updates the published-at timestamp.
type PublishTripInput struct {
	PublicTitle        *string
	PublicSummary      *string
	DepositAmountCents *int64
	DepositCurrency    *string
	BerthHoldDays      *int
}

func (p *Pool) PublishTrip(ctx context.Context, orgID, tripID uuid.UUID, in PublishTripInput, now time.Time) error {
	if in.BerthHoldDays != nil && (*in.BerthHoldDays < 1 || *in.BerthHoldDays > 30) {
		return errors.New("publish_trip: berth_hold_days must be 1..30")
	}
	if in.DepositCurrency != nil {
		cur, err := NormalizeCurrency(*in.DepositCurrency)
		if err != nil {
			return err
		}
		in.DepositCurrency = &cur
	}
	tag, err := p.Exec(ctx, `
		UPDATE trips
		SET published_at         = $1,
		    public_title         = COALESCE($2, public_title),
		    public_summary       = COALESCE($3, public_summary),
		    deposit_amount_cents = COALESCE($4, deposit_amount_cents),
		    deposit_currency     = COALESCE($5, deposit_currency),
		    berth_hold_days      = COALESCE($6, berth_hold_days),
		    updated_at           = now()
		WHERE organization_id = $7 AND id = $8
	`,
		now, in.PublicTitle, in.PublicSummary,
		in.DepositAmountCents, in.DepositCurrency, in.BerthHoldDays,
		orgID, tripID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Pool) UnpublishTrip(ctx context.Context, orgID, tripID uuid.UUID) error {
	tag, err := p.Exec(ctx, `
		UPDATE trips SET published_at = NULL, updated_at = now()
		WHERE organization_id = $1 AND id = $2
	`, orgID, tripID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
