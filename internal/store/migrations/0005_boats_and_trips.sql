-- +goose Up
-- +goose StatementBegin

-- Boats. The schema separates "scraper-owned" columns (rewritten on
-- every successful scrape) from "operator-owned" columns (defaulted on
-- insert, never overwritten by re-scrape). This lets future hand-edits
-- coexist with the importer without a "scraper clobbered my edit" class
-- of bug.

CREATE TABLE boats (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Operator-owned: defaults to source_name on insert, NEVER overwritten by re-scrape.
    display_name             text        NOT NULL,

    -- Scraper-owned: rewritten on every successful scrape.
    source_provider          text        NOT NULL DEFAULT 'liveaboard.com',
    source_slug              text        NOT NULL,
    source_name              text        NOT NULL,
    source_url               text        NOT NULL,
    source_image_url         text        NULL,
    source_external_id       text        NULL,
    source_last_synced_at    timestamptz NOT NULL,

    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),

    UNIQUE (organization_id, source_provider, source_slug)
);
CREATE INDEX boats_organization_id_idx ON boats(organization_id);

-- Trips. Uniqueness is keyed on source_trip_key (a deterministic
-- fingerprint computed in the parser) rather than itinerary text, which
-- is source-controlled marketing copy. organization_id is denormalized
-- here so cross-tenant queries are a single index scan.

CREATE TABLE trips (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    boat_id                  uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,

    start_date               date        NOT NULL,
    end_date                 date        NOT NULL,
    itinerary                text        NOT NULL,
    departure_port           text        NULL,
    return_port              text        NULL,

    -- Raw text from the source. Structured pricing is a follow-up sprint
    -- (once the product has a pricing model).
    price_text               text        NULL,
    availability_text        text        NULL,

    -- Scraper identity / provenance.
    source_provider          text        NOT NULL DEFAULT 'liveaboard.com',
    source_trip_key          text        NOT NULL,
    source_url               text        NOT NULL,
    source_last_synced_at    timestamptz NOT NULL,

    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),

    UNIQUE (boat_id, source_provider, source_trip_key),
    CHECK (end_date >= start_date)
);
CREATE INDEX trips_boat_id_idx          ON trips(boat_id);
CREATE INDEX trips_organization_id_idx  ON trips(organization_id);
CREATE INDEX trips_start_date_idx       ON trips(start_date);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS trips;
DROP TABLE IF EXISTS boats;

-- +goose StatementEnd
