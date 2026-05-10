-- +goose Up
-- +goose StatementBegin

-- Sprint 020: per-item boat/trip price overrides and USD/EUR default
-- settlement currencies. Pricing remains canonical USD; tax, service
-- charge, packages, and discounts stay out of scope.

UPDATE organization_payment_settings
SET supported_currencies = (
        SELECT array_agg(DISTINCT currency ORDER BY currency)
        FROM unnest(supported_currencies || ARRAY['USD','EUR']::text[]) AS u(currency)
    ),
    updated_at = now()
WHERE NOT supported_currencies @> ARRAY['EUR']::text[]
   OR NOT supported_currencies @> ARRAY['USD']::text[];

ALTER TABLE organization_payment_settings
    ALTER COLUMN supported_currencies SET DEFAULT ARRAY['USD','EUR']::text[];

CREATE TABLE catalog_price_overrides (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    catalog_item_id     uuid        NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    boat_id             uuid        NULL REFERENCES boats(id) ON DELETE CASCADE,
    trip_id             uuid        NULL REFERENCES trips(id) ON DELETE CASCADE,
    price_usd_cents     bigint      NOT NULL CHECK (price_usd_cents >= 0),
    notes               text        NOT NULL DEFAULT '',
    created_by_user_id  uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id  uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    archived_at         timestamptz NULL,
    CHECK ((boat_id IS NOT NULL)::int + (trip_id IS NOT NULL)::int = 1)
);

CREATE UNIQUE INDEX catalog_price_overrides_active_boat_idx
    ON catalog_price_overrides(organization_id, catalog_item_id, boat_id)
    WHERE boat_id IS NOT NULL AND archived_at IS NULL;

CREATE UNIQUE INDEX catalog_price_overrides_active_trip_idx
    ON catalog_price_overrides(organization_id, catalog_item_id, trip_id)
    WHERE trip_id IS NOT NULL AND archived_at IS NULL;

CREATE INDEX catalog_price_overrides_org_boat_idx
    ON catalog_price_overrides(organization_id, boat_id)
    WHERE boat_id IS NOT NULL;

CREATE INDEX catalog_price_overrides_org_trip_idx
    ON catalog_price_overrides(organization_id, trip_id)
    WHERE trip_id IS NOT NULL;

ALTER TABLE guest_folio_lines
    ADD COLUMN price_source text NOT NULL DEFAULT 'base',
    ADD COLUMN price_override_id uuid NULL REFERENCES catalog_price_overrides(id) ON DELETE SET NULL,
    ADD CONSTRAINT guest_folio_lines_price_source_check
        CHECK (price_source IN ('base','boat_override','trip_override','tip'));

UPDATE guest_folio_lines
SET price_source = 'tip'
WHERE line_type = 'crew_tip';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE guest_folio_lines
    DROP CONSTRAINT IF EXISTS guest_folio_lines_price_source_check,
    DROP COLUMN IF EXISTS price_override_id,
    DROP COLUMN IF EXISTS price_source;

DROP INDEX IF EXISTS catalog_price_overrides_org_trip_idx;
DROP INDEX IF EXISTS catalog_price_overrides_org_boat_idx;
DROP INDEX IF EXISTS catalog_price_overrides_active_trip_idx;
DROP INDEX IF EXISTS catalog_price_overrides_active_boat_idx;
DROP TABLE IF EXISTS catalog_price_overrides;

ALTER TABLE organization_payment_settings
    ALTER COLUMN supported_currencies SET DEFAULT ARRAY['USD']::text[];

-- +goose StatementEnd
