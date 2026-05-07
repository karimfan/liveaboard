-- +goose Up
-- +goose StatementBegin

-- Sprint 013: Catalog, inventory, stock movements, FX snapshots, and
-- checkout quote persistence.

CREATE TABLE catalog_categories (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    template_key    text        NULL,
    name            text        NOT NULL CHECK (length(trim(name)) > 0),
    sort_order      integer     NOT NULL DEFAULT 0,
    is_default_seed boolean     NOT NULL DEFAULT false,
    archived_at     timestamptz NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX catalog_categories_org_idx ON catalog_categories(organization_id, sort_order, lower(name));
CREATE UNIQUE INDEX catalog_categories_org_name_active_idx
    ON catalog_categories(organization_id, lower(name))
    WHERE archived_at IS NULL;
CREATE UNIQUE INDEX catalog_categories_org_template_idx
    ON catalog_categories(organization_id, template_key)
    WHERE template_key IS NOT NULL;

CREATE TABLE catalog_items (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    category_id       uuid        NOT NULL REFERENCES catalog_categories(id) ON DELETE RESTRICT,
    template_key      text        NULL,
    name              text        NOT NULL CHECK (length(trim(name)) > 0),
    description       text        NULL,
    unit              text        NOT NULL CHECK (length(trim(unit)) > 0),
    charge_type       text        NOT NULL CHECK (charge_type IN ('sale','rental','service','fee','gratuity','deposit','damage','included')),
    stock_mode        text        NOT NULL CHECK (stock_mode IN ('none','counted')),
    price_usd_cents   bigint      NOT NULL CHECK (price_usd_cents >= 0),
    is_taxable        boolean     NOT NULL DEFAULT false,
    is_required_fee   boolean     NOT NULL DEFAULT false,
    is_active         boolean     NOT NULL DEFAULT true,
    archived_at       timestamptz NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX catalog_items_org_idx ON catalog_items(organization_id, category_id, lower(name));
CREATE INDEX catalog_items_active_idx ON catalog_items(organization_id, is_active) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX catalog_items_org_name_active_idx
    ON catalog_items(organization_id, lower(name))
    WHERE archived_at IS NULL;
CREATE UNIQUE INDEX catalog_items_org_template_idx
    ON catalog_items(organization_id, template_key)
    WHERE template_key IS NOT NULL;

CREATE TABLE boat_inventory_items (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    boat_id           uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,
    catalog_item_id   uuid        NOT NULL REFERENCES catalog_items(id) ON DELETE RESTRICT,
    quantity_on_hand  integer     NOT NULL DEFAULT 0 CHECK (quantity_on_hand >= 0),
    quantity_reserved integer     NOT NULL DEFAULT 0 CHECK (quantity_reserved >= 0),
    reorder_level     integer     NULL CHECK (reorder_level IS NULL OR reorder_level >= 0),
    par_level         integer     NULL CHECK (par_level IS NULL OR par_level >= 0),
    last_counted_at   timestamptz NULL,
    notes             text        NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, boat_id, catalog_item_id)
);

CREATE INDEX boat_inventory_items_boat_idx ON boat_inventory_items(organization_id, boat_id);
CREATE INDEX boat_inventory_items_item_idx ON boat_inventory_items(organization_id, catalog_item_id);

CREATE TABLE stock_movements (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    boat_id           uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,
    catalog_item_id   uuid        NOT NULL REFERENCES catalog_items(id) ON DELETE RESTRICT,
    actor_user_id     uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    movement_type     text        NOT NULL CHECK (movement_type IN ('initial_count','restock','correction','breakage','spoilage','internal_use','folio_charge','folio_void')),
    delta_quantity    integer     NOT NULL,
    quantity_before   integer     NOT NULL CHECK (quantity_before >= 0),
    quantity_after    integer     NOT NULL CHECK (quantity_after >= 0),
    source_type       text        NULL,
    source_id         uuid        NULL,
    note              text        NULL,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX stock_movements_boat_item_idx ON stock_movements(organization_id, boat_id, catalog_item_id, created_at DESC);

CREATE TABLE exchange_rates (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    provider          text        NOT NULL CHECK (length(trim(provider)) > 0),
    base_currency     char(3)     NOT NULL,
    quote_currency    char(3)     NOT NULL,
    rate_numerator    bigint      NOT NULL CHECK (rate_numerator > 0),
    rate_denominator  bigint      NOT NULL CHECK (rate_denominator > 0),
    as_of             timestamptz NOT NULL,
    fetched_at        timestamptz NOT NULL DEFAULT now(),
    expires_at        timestamptz NOT NULL
);

CREATE INDEX exchange_rates_latest_idx
    ON exchange_rates(base_currency, quote_currency, expires_at DESC, as_of DESC);

CREATE TABLE checkout_quotes (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id        uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    requested_by           uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    source_currency        char(3)     NOT NULL DEFAULT 'USD',
    target_currency        char(3)     NOT NULL,
    source_amount_cents    bigint      NOT NULL CHECK (source_amount_cents >= 0),
    target_amount_minor    bigint      NOT NULL CHECK (target_amount_minor >= 0),
    currency_exponent      integer     NOT NULL CHECK (currency_exponent >= 0),
    rate_provider          text        NOT NULL,
    rate_numerator         bigint      NOT NULL CHECK (rate_numerator > 0),
    rate_denominator       bigint      NOT NULL CHECK (rate_denominator > 0),
    rate_as_of             timestamptz NOT NULL,
    expires_at             timestamptz NOT NULL,
    created_at             timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX checkout_quotes_org_idx ON checkout_quotes(organization_id, created_at DESC);

CREATE TABLE checkout_quote_lines (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id              uuid        NOT NULL REFERENCES checkout_quotes(id) ON DELETE CASCADE,
    catalog_item_id        uuid        NULL REFERENCES catalog_items(id) ON DELETE SET NULL,
    item_name             text        NOT NULL,
    quantity              integer     NOT NULL CHECK (quantity > 0),
    unit_price_usd_cents  bigint      NOT NULL CHECK (unit_price_usd_cents >= 0),
    line_total_usd_cents  bigint      NOT NULL CHECK (line_total_usd_cents >= 0),
    sort_order            integer     NOT NULL DEFAULT 0
);

CREATE INDEX checkout_quote_lines_quote_idx ON checkout_quote_lines(quote_id, sort_order);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS checkout_quote_lines;
DROP TABLE IF EXISTS checkout_quotes;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS stock_movements;
DROP TABLE IF EXISTS boat_inventory_items;
DROP TABLE IF EXISTS catalog_items;
DROP TABLE IF EXISTS catalog_categories;

-- +goose StatementEnd
