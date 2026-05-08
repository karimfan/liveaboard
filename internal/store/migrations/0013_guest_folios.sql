-- +goose Up
-- +goose StatementBegin

-- Sprint 015: organization payment settings and one end-of-trip guest
-- folio per trip guest. Payment processing remains offline; these rows
-- are immutable settlement records once closed.

CREATE TABLE organization_payment_settings (
    organization_id           uuid        PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    default_currency          char(3)     NOT NULL DEFAULT 'USD',
    supported_currencies      text[]      NOT NULL DEFAULT ARRAY['USD']::text[],
    enabled_payment_methods   text[]      NOT NULL DEFAULT ARRAY['card','cash','other']::text[],
    card_fee_basis_points     integer     NOT NULL DEFAULT 0 CHECK (card_fee_basis_points >= 0 AND card_fee_basis_points <= 2000),
    folio_email_footer        text        NULL,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE guest_folios (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id                  uuid        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    trip_guest_id            uuid        NOT NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
    guest_user_id            uuid        NULL REFERENCES guest_users(id) ON DELETE SET NULL,
    status                   text        NOT NULL CHECK (status IN ('open','closed')),
    opened_by_user_id        uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    closed_by_user_id        uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    closed_at                timestamptz NULL,
    subtotal_usd_cents       bigint      NOT NULL DEFAULT 0 CHECK (subtotal_usd_cents >= 0),
    card_fee_usd_cents       bigint      NOT NULL DEFAULT 0 CHECK (card_fee_usd_cents >= 0),
    total_usd_cents          bigint      NOT NULL DEFAULT 0 CHECK (total_usd_cents >= 0),
    settlement_currency      char(3)     NULL,
    settlement_total_minor   bigint      NULL CHECK (settlement_total_minor IS NULL OR settlement_total_minor >= 0),
    currency_exponent        integer     NULL,
    rate_provider            text        NULL,
    rate_numerator           bigint      NULL,
    rate_denominator         bigint      NULL,
    rate_as_of               timestamptz NULL,
    payment_method           text        NULL CHECK (payment_method IS NULL OR payment_method IN ('card','cash','other')),
    card_fee_basis_points    integer     NOT NULL DEFAULT 0,
    email_send_status        text        NOT NULL DEFAULT 'not_sent' CHECK (email_send_status IN ('not_sent','sent','failed')),
    email_last_sent_at       timestamptz NULL,
    email_last_error         text        NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX guest_folios_one_per_trip_guest_idx ON guest_folios(trip_guest_id);
CREATE INDEX guest_folios_org_trip_idx ON guest_folios(organization_id, trip_id);

CREATE TABLE guest_folio_lines (
    id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id        uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    folio_id               uuid        NOT NULL REFERENCES guest_folios(id) ON DELETE CASCADE,
    catalog_item_id         uuid        NULL REFERENCES catalog_items(id) ON DELETE SET NULL,
    line_type              text        NOT NULL CHECK (line_type IN ('catalog_item','crew_tip')),
    item_name              text        NOT NULL CHECK (length(trim(item_name)) > 0),
    quantity               integer     NOT NULL CHECK (quantity > 0),
    unit_price_usd_cents   bigint      NOT NULL CHECK (unit_price_usd_cents >= 0),
    line_total_usd_cents   bigint      NOT NULL CHECK (line_total_usd_cents >= 0),
    stock_mode             text        NOT NULL DEFAULT 'none' CHECK (stock_mode IN ('none','counted')),
    sort_order             integer     NOT NULL DEFAULT 0,
    created_by_user_id     uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    CHECK (line_type <> 'crew_tip' OR quantity = 1),
    CHECK (line_type <> 'catalog_item' OR catalog_item_id IS NOT NULL)
);

CREATE UNIQUE INDEX guest_folio_lines_one_tip_idx
    ON guest_folio_lines(folio_id)
    WHERE line_type = 'crew_tip';
CREATE INDEX guest_folio_lines_folio_idx ON guest_folio_lines(folio_id, sort_order);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS guest_folio_lines;
DROP TABLE IF EXISTS guest_folios;
DROP TABLE IF EXISTS organization_payment_settings;

-- +goose StatementEnd
