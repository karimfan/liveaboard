package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	PriceSourceBase         = "base"
	PriceSourceBoatOverride = "boat_override"
	PriceSourceTripOverride = "trip_override"
	PriceSourceTip          = "tip"
)

type CatalogPriceOverride struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	CatalogItemID   uuid.UUID
	BoatID          *uuid.UUID
	TripID          *uuid.UUID
	PriceUSDCents   int64
	Notes           string
	CreatedByUserID *uuid.UUID
	UpdatedByUserID *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
	ItemName        string
	BoatName        *string
	TripLabel       *string
}

type PriceOverrideInput struct {
	CatalogItemID uuid.UUID
	BoatID        uuid.UUID
	TripID        uuid.UUID
	PriceUSDCents int64
	Notes         string
	ActorUserID   uuid.UUID
}

type EffectiveCatalogPrice struct {
	PriceUSDCents int64
	PriceSource   string
	OverrideID    *uuid.UUID
}

const catalogPriceOverrideColumns = `o.id, o.organization_id, o.catalog_item_id, o.boat_id, o.trip_id,
	o.price_usd_cents, o.notes, o.created_by_user_id, o.updated_by_user_id,
	o.created_at, o.updated_at, o.archived_at,
	i.name,
	b.display_name,
	CASE WHEN t.id IS NULL THEN NULL ELSE t.itinerary || ' - ' || t.start_date::text END`

func scanCatalogPriceOverride(row interface{ Scan(dest ...any) error }, o *CatalogPriceOverride) error {
	return row.Scan(&o.ID, &o.OrganizationID, &o.CatalogItemID, &o.BoatID, &o.TripID,
		&o.PriceUSDCents, &o.Notes, &o.CreatedByUserID, &o.UpdatedByUserID,
		&o.CreatedAt, &o.UpdatedAt, &o.ArchivedAt, &o.ItemName, &o.BoatName, &o.TripLabel)
}

func (p *Pool) ListCatalogPriceOverrides(ctx context.Context, orgID uuid.UUID, includeArchived bool) ([]*CatalogPriceOverride, error) {
	archivedClause := "AND o.archived_at IS NULL"
	if includeArchived {
		archivedClause = ""
	}
	rows, err := p.Query(ctx, `
		SELECT `+catalogPriceOverrideColumns+`
		FROM catalog_price_overrides o
		JOIN catalog_items i ON i.id = o.catalog_item_id AND i.organization_id = o.organization_id
		LEFT JOIN boats b ON b.id = o.boat_id AND b.organization_id = o.organization_id
		LEFT JOIN trips t ON t.id = o.trip_id AND t.organization_id = o.organization_id
		WHERE o.organization_id = $1 `+archivedClause+`
		ORDER BY o.archived_at NULLS FIRST, lower(i.name), lower(COALESCE(b.display_name, t.itinerary, ''))
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*CatalogPriceOverride{}
	for rows.Next() {
		o := &CatalogPriceOverride{}
		if err := scanCatalogPriceOverride(rows, o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (p *Pool) UpsertBoatPriceOverride(ctx context.Context, orgID uuid.UUID, in PriceOverrideInput) (*CatalogPriceOverride, error) {
	if in.BoatID == uuid.Nil {
		return nil, errors.New("boat_id required")
	}
	if err := validatePriceOverrideInput(in); err != nil {
		return nil, err
	}
	if err := p.ensureCatalogItemInOrg(ctx, orgID, in.CatalogItemID); err != nil {
		return nil, err
	}
	if _, err := p.BoatByID(ctx, orgID, in.BoatID); err != nil {
		return nil, err
	}
	o := &CatalogPriceOverride{}
	err := scanCatalogPriceOverride(p.QueryRow(ctx, `
		WITH upserted AS (
			INSERT INTO catalog_price_overrides (
				organization_id, catalog_item_id, boat_id, price_usd_cents, notes,
				created_by_user_id, updated_by_user_id
			)
			VALUES ($1,$2,$3,$4,$5,$6,$6)
			ON CONFLICT (organization_id, catalog_item_id, boat_id)
			WHERE boat_id IS NOT NULL AND archived_at IS NULL
			DO UPDATE SET price_usd_cents = EXCLUDED.price_usd_cents,
				notes = EXCLUDED.notes,
				updated_by_user_id = EXCLUDED.updated_by_user_id,
				updated_at = now()
			RETURNING *
		)
		SELECT `+catalogPriceOverrideColumns+`
		FROM upserted o
		JOIN catalog_items i ON i.id = o.catalog_item_id AND i.organization_id = o.organization_id
		LEFT JOIN boats b ON b.id = o.boat_id AND b.organization_id = o.organization_id
		LEFT JOIN trips t ON t.id = o.trip_id AND t.organization_id = o.organization_id
	`, orgID, in.CatalogItemID, in.BoatID, in.PriceUSDCents, strings.TrimSpace(in.Notes), in.ActorUserID), o)
	return o, err
}

func (p *Pool) UpsertTripPriceOverride(ctx context.Context, orgID uuid.UUID, in PriceOverrideInput) (*CatalogPriceOverride, error) {
	if in.TripID == uuid.Nil {
		return nil, errors.New("trip_id required")
	}
	if err := validatePriceOverrideInput(in); err != nil {
		return nil, err
	}
	if err := p.ensureCatalogItemInOrg(ctx, orgID, in.CatalogItemID); err != nil {
		return nil, err
	}
	if _, err := p.TripByID(ctx, orgID, in.TripID); err != nil {
		return nil, err
	}
	o := &CatalogPriceOverride{}
	err := scanCatalogPriceOverride(p.QueryRow(ctx, `
		WITH upserted AS (
			INSERT INTO catalog_price_overrides (
				organization_id, catalog_item_id, trip_id, price_usd_cents, notes,
				created_by_user_id, updated_by_user_id
			)
			VALUES ($1,$2,$3,$4,$5,$6,$6)
			ON CONFLICT (organization_id, catalog_item_id, trip_id)
			WHERE trip_id IS NOT NULL AND archived_at IS NULL
			DO UPDATE SET price_usd_cents = EXCLUDED.price_usd_cents,
				notes = EXCLUDED.notes,
				updated_by_user_id = EXCLUDED.updated_by_user_id,
				updated_at = now()
			RETURNING *
		)
		SELECT `+catalogPriceOverrideColumns+`
		FROM upserted o
		JOIN catalog_items i ON i.id = o.catalog_item_id AND i.organization_id = o.organization_id
		LEFT JOIN boats b ON b.id = o.boat_id AND b.organization_id = o.organization_id
		LEFT JOIN trips t ON t.id = o.trip_id AND t.organization_id = o.organization_id
	`, orgID, in.CatalogItemID, in.TripID, in.PriceUSDCents, strings.TrimSpace(in.Notes), in.ActorUserID), o)
	return o, err
}

func (p *Pool) ArchiveCatalogPriceOverride(ctx context.Context, orgID, overrideID, actorID uuid.UUID) (*CatalogPriceOverride, error) {
	o := &CatalogPriceOverride{}
	err := scanCatalogPriceOverride(p.QueryRow(ctx, `
		WITH archived AS (
			UPDATE catalog_price_overrides
			SET archived_at = COALESCE(archived_at, now()),
				updated_by_user_id = $3,
				updated_at = now()
			WHERE organization_id = $1 AND id = $2
			RETURNING *
		)
		SELECT `+catalogPriceOverrideColumns+`
		FROM archived o
		JOIN catalog_items i ON i.id = o.catalog_item_id AND i.organization_id = o.organization_id
		LEFT JOIN boats b ON b.id = o.boat_id AND b.organization_id = o.organization_id
		LEFT JOIN trips t ON t.id = o.trip_id AND t.organization_id = o.organization_id
	`, orgID, overrideID, actorID), o)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return o, err
}

func (p *Pool) EffectiveCatalogItemsForTrip(ctx context.Context, orgID, tripID uuid.UUID) ([]*CatalogItem, error) {
	trip, err := p.TripByID(ctx, orgID, tripID)
	if err != nil {
		return nil, err
	}
	items, err := p.CatalogItems(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		price, err := effectiveCatalogItemPrice(ctx, p, orgID, trip.ID, trip.BoatID, item.ID)
		if err != nil {
			return nil, err
		}
		item.EffectivePriceUSDCents = &price.PriceUSDCents
		item.PriceSource = price.PriceSource
		item.PriceOverrideID = price.OverrideID
	}
	return items, nil
}

func validatePriceOverrideInput(in PriceOverrideInput) error {
	if in.CatalogItemID == uuid.Nil {
		return errors.New("catalog_item_id required")
	}
	if in.PriceUSDCents < 0 {
		return errors.New("price_usd_cents must be non-negative")
	}
	if in.BoatID != uuid.Nil && in.TripID != uuid.Nil {
		return errors.New("override must target one boat or one trip")
	}
	return nil
}

func (p *Pool) ensureCatalogItemInOrg(ctx context.Context, orgID, itemID uuid.UUID) error {
	var exists bool
	if err := p.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM catalog_items
			WHERE organization_id = $1 AND id = $2 AND archived_at IS NULL
		)
	`, orgID, itemID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func effectiveCatalogItemPrice(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, orgID, tripID, boatID, itemID uuid.UUID) (*EffectiveCatalogPrice, error) {
	price := &EffectiveCatalogPrice{}
	err := q.QueryRow(ctx, `
		WITH base AS (
			SELECT price_usd_cents
			FROM catalog_items
			WHERE organization_id = $1 AND id = $4
		),
		trip_override AS (
			SELECT id, price_usd_cents
			FROM catalog_price_overrides
			WHERE organization_id = $1 AND trip_id = $2 AND catalog_item_id = $4 AND archived_at IS NULL
			LIMIT 1
		),
		boat_override AS (
			SELECT id, price_usd_cents
			FROM catalog_price_overrides
			WHERE organization_id = $1 AND boat_id = $3 AND catalog_item_id = $4 AND archived_at IS NULL
			LIMIT 1
		)
		SELECT
			COALESCE(trip_override.price_usd_cents, boat_override.price_usd_cents, base.price_usd_cents),
			CASE
				WHEN trip_override.id IS NOT NULL THEN 'trip_override'
				WHEN boat_override.id IS NOT NULL THEN 'boat_override'
				ELSE 'base'
			END,
			COALESCE(trip_override.id, boat_override.id)
		FROM base
		LEFT JOIN trip_override ON true
		LEFT JOIN boat_override ON true
	`, orgID, tripID, boatID, itemID).Scan(&price.PriceUSDCents, &price.PriceSource, &price.OverrideID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return price, err
}
