-- +goose Up
-- +goose StatementBegin

-- Sprint 026: open the funnel, guest portal, and operations spine.
-- Three feature anchors share one migration. Every operational table
-- carries organization_id; multi-tenant isolation is enforced at the
-- store layer. No external payment processor is introduced — deposits
-- and refunds are operator-confirmed offline records that mirror the
-- Sprint 015 folio-payment idiom. See docs/sprints/SPRINT-026.md.

-- ============================================================
-- Anchor 1: Public funnel — catalog publication + leads/quotes/
-- offline payments.
-- ============================================================

ALTER TABLE organizations
    ADD COLUMN public_slug text NULL;

CREATE UNIQUE INDEX organizations_public_slug_idx
    ON organizations(lower(public_slug))
    WHERE public_slug IS NOT NULL;

ALTER TABLE trips
    ADD COLUMN published_at         timestamptz NULL,
    ADD COLUMN public_title         text        NULL,
    ADD COLUMN public_summary       text        NULL,
    ADD COLUMN deposit_amount_cents bigint      NULL
        CHECK (deposit_amount_cents IS NULL OR deposit_amount_cents >= 0),
    ADD COLUMN deposit_currency     char(3)     NULL,
    ADD COLUMN berth_hold_days      integer     NOT NULL DEFAULT 7
        CHECK (berth_hold_days BETWEEN 1 AND 30);

CREATE TABLE leads (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id         uuid        NULL     REFERENCES trips(id)         ON DELETE SET NULL,
    email           citext      NOT NULL,
    name            text        NOT NULL CHECK (length(trim(name)) > 0),
    party_size      integer     NOT NULL CHECK (party_size BETWEEN 1 AND 50),
    date_window     text        NULL,
    notes           text        NULL,
    source_ip       text        NULL,
    status          text        NOT NULL DEFAULT 'new'
        CHECK (status IN ('new','contacted','quoted','won','lost','archived')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX leads_org_status_idx ON leads(organization_id, status, created_at DESC);

CREATE TABLE booking_quotes (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    lead_id             uuid        NULL     REFERENCES leads(id)         ON DELETE SET NULL,
    trip_id             uuid        NOT NULL REFERENCES trips(id)         ON DELETE CASCADE,
    token_hash          bytea       NOT NULL UNIQUE,
    guest_name          text        NOT NULL CHECK (length(trim(guest_name)) > 0),
    guest_email         citext      NOT NULL,
    party_size          integer     NOT NULL CHECK (party_size > 0),
    quoted_total_cents  bigint      NOT NULL CHECK (quoted_total_cents >= 0),
    deposit_due_cents   bigint      NOT NULL CHECK (deposit_due_cents >= 0),
    currency            char(3)     NOT NULL,
    status              text        NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','sent','accepted','deposit_pending','held','cancelled','expired')),
    accepted_at         timestamptz NULL,
    hold_expires_at     timestamptz NULL,
    cancelled_reason    text        NULL,
    created_by_user_id  uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX booking_quotes_org_trip_idx ON booking_quotes(organization_id, trip_id, status);
CREATE INDEX booking_quotes_org_status_idx ON booking_quotes(organization_id, status, created_at DESC);

CREATE TABLE offline_payments (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    quote_id              uuid        NULL     REFERENCES booking_quotes(id) ON DELETE SET NULL,
    trip_guest_id         uuid        NULL     REFERENCES trip_guests(id)    ON DELETE SET NULL,
    direction             text        NOT NULL CHECK (direction IN ('deposit','refund')),
    amount_cents          bigint      NOT NULL CHECK (amount_cents > 0),
    currency              char(3)     NOT NULL,
    method                text        NOT NULL
        CHECK (method IN ('bank_transfer','cash','wise','card_external','other')),
    received_on           date        NULL,
    reference             text        NULL,
    notes                 text        NULL,
    recorded_by_user_id   uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    recorded_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX offline_payments_org_quote_idx ON offline_payments(organization_id, quote_id, recorded_at DESC);

-- ============================================================
-- Anchor 2: Guest portal — briefing documents, day plans,
-- pre-trip requests, and certification metadata.
-- ============================================================

CREATE TABLE trip_briefing_documents (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id               uuid        NOT NULL REFERENCES trips(id)         ON DELETE CASCADE,
    title                 text        NOT NULL CHECK (length(trim(title)) > 0),
    file_path             text        NOT NULL,
    content_type          text        NOT NULL,
    uploaded_by_user_id   uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX trip_briefing_documents_org_trip_idx ON trip_briefing_documents(organization_id, trip_id);

CREATE TABLE trip_day_plans (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id             uuid        NOT NULL REFERENCES trips(id)         ON DELETE CASCADE,
    day_date            date        NOT NULL,
    title               text        NOT NULL,
    schedule_json       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    dive_plan_json      jsonb       NOT NULL DEFAULT '[]'::jsonb,
    updated_by_user_id  uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trip_id, day_date)
);
CREATE INDEX trip_day_plans_org_trip_idx ON trip_day_plans(organization_id, trip_id, day_date);

CREATE TABLE guest_portal_requests (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_guest_id   uuid        NOT NULL REFERENCES trip_guests(id)   ON DELETE CASCADE,
    request_type    text        NOT NULL CHECK (request_type IN ('equipment','dietary')),
    payload         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trip_guest_id, request_type)
);

-- Guest certification metadata layered on top of Sprint 015's
-- guest_documents storage. The doc reference is nullable so an
-- operator can pre-fill cert metadata before the guest uploads.
CREATE TABLE guest_certifications (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_guest_id            uuid        NOT NULL REFERENCES trip_guests(id)   ON DELETE CASCADE,
    guest_document_id        uuid        NULL     REFERENCES guest_documents(id) ON DELETE SET NULL,
    certification_type       text        NOT NULL,
    issuer                   text        NULL,
    certification_number     text        NULL,
    expires_on               date        NULL,
    verified_at              timestamptz NULL,
    verified_by_user_id      uuid        NULL     REFERENCES users(id) ON DELETE SET NULL,
    verification_notes       text        NULL,
    created_at               timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX guest_certifications_org_guest_idx ON guest_certifications(organization_id, trip_guest_id);

-- ============================================================
-- Anchor 3: Crew + equipment + readiness — supply-side schema
-- driving the start-transition readiness gate.
-- ============================================================

CREATE TABLE crew_members (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         uuid        NULL     REFERENCES users(id)         ON DELETE SET NULL,
    name            text        NOT NULL CHECK (length(trim(name)) > 0),
    email           citext      NULL,
    role            text        NOT NULL
        CHECK (role IN ('captain','dive_master','chef','deckhand','engineer','cruise_director','other')),
    status          text        NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','inactive')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX crew_members_org_role_idx ON crew_members(organization_id, role, status);

CREATE TABLE crew_certifications (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    crew_member_id        uuid        NOT NULL REFERENCES crew_members(id)  ON DELETE CASCADE,
    certification_type    text        NOT NULL,
    issuer                text        NULL,
    certification_number  text        NULL,
    expires_on            date        NULL,
    required_for_roles    text[]      NOT NULL DEFAULT '{}',
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX crew_certifications_org_expiry_idx ON crew_certifications(organization_id, expires_on);
CREATE INDEX crew_certifications_member_idx     ON crew_certifications(crew_member_id);

CREATE TABLE equipment_assets (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    boat_id             uuid        NULL     REFERENCES boats(id)         ON DELETE SET NULL,
    asset_type          text        NOT NULL
        CHECK (asset_type IN ('bcd','regulator','tank','dive_computer','o2_kit','other')),
    label               text        NOT NULL CHECK (length(trim(label)) > 0),
    serial_number       text        NULL,
    status              text        NOT NULL DEFAULT 'in_service'
        CHECK (status IN ('in_service','out_of_service','retired')),
    service_due_on      date        NULL,
    required_for_dive   boolean     NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX equipment_assets_org_status_idx ON equipment_assets(organization_id, status, service_due_on);
CREATE INDEX equipment_assets_boat_idx       ON equipment_assets(boat_id);

CREATE TABLE equipment_service_events (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    equipment_asset_id   uuid        NOT NULL REFERENCES equipment_assets(id) ON DELETE CASCADE,
    event_type           text        NOT NULL
        CHECK (event_type IN ('annual_service','vip_inspection','repair','retirement')),
    event_on             date        NOT NULL,
    notes                text        NULL,
    recorded_by_user_id  uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX equipment_service_events_asset_idx ON equipment_service_events(equipment_asset_id, event_on DESC);

CREATE TABLE trip_crew_assignments (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id         uuid        NOT NULL REFERENCES trips(id)         ON DELETE CASCADE,
    crew_member_id  uuid        NOT NULL REFERENCES crew_members(id)  ON DELETE RESTRICT,
    role            text        NOT NULL,
    assigned_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trip_id, crew_member_id)
);
CREATE INDEX trip_crew_assignments_trip_idx ON trip_crew_assignments(trip_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS trip_crew_assignments;
DROP TABLE IF EXISTS equipment_service_events;
DROP TABLE IF EXISTS equipment_assets;
DROP TABLE IF EXISTS crew_certifications;
DROP TABLE IF EXISTS crew_members;
DROP TABLE IF EXISTS guest_certifications;
DROP TABLE IF EXISTS guest_portal_requests;
DROP TABLE IF EXISTS trip_day_plans;
DROP TABLE IF EXISTS trip_briefing_documents;
DROP TABLE IF EXISTS offline_payments;
DROP TABLE IF EXISTS booking_quotes;
DROP TABLE IF EXISTS leads;

ALTER TABLE trips
    DROP COLUMN IF EXISTS berth_hold_days,
    DROP COLUMN IF EXISTS deposit_currency,
    DROP COLUMN IF EXISTS deposit_amount_cents,
    DROP COLUMN IF EXISTS public_summary,
    DROP COLUMN IF EXISTS public_title,
    DROP COLUMN IF EXISTS published_at;

DROP INDEX IF EXISTS organizations_public_slug_idx;
ALTER TABLE organizations DROP COLUMN IF EXISTS public_slug;

-- +goose StatementEnd
