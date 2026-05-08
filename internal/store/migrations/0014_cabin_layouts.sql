-- +goose Up
-- +goose StatementBegin

CREATE TABLE boat_cabins (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    boat_id         uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,
    label           text        NOT NULL CHECK (length(trim(label)) > 0),
    deck            text        NULL,
    sort_order      integer     NOT NULL DEFAULT 0,
    notes           text        NULL,
    is_active       boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX boat_cabins_org_boat_idx
    ON boat_cabins(organization_id, boat_id, sort_order);

CREATE UNIQUE INDEX boat_cabins_org_boat_label_active_idx
    ON boat_cabins(organization_id, boat_id, label)
    WHERE is_active;

CREATE TABLE boat_cabin_berths (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    boat_id         uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,
    cabin_id        uuid        NOT NULL REFERENCES boat_cabins(id) ON DELETE CASCADE,
    berth_label     text        NOT NULL CHECK (length(trim(berth_label)) > 0),
    display_label   text        NOT NULL CHECK (length(trim(display_label)) > 0),
    sort_order      integer     NOT NULL DEFAULT 0,
    notes           text        NULL,
    is_active       boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX boat_cabin_berths_org_boat_idx
    ON boat_cabin_berths(organization_id, boat_id, sort_order);

CREATE UNIQUE INDEX boat_cabin_berths_cabin_label_active_idx
    ON boat_cabin_berths(organization_id, cabin_id, berth_label)
    WHERE is_active;

CREATE UNIQUE INDEX boat_cabin_berths_display_label_active_idx
    ON boat_cabin_berths(organization_id, boat_id, display_label)
    WHERE is_active;

CREATE TABLE trip_cabin_assignments (
    id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id        uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id                uuid        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    trip_guest_id          uuid        NOT NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
    boat_id                uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,
    berth_id               uuid        NOT NULL REFERENCES boat_cabin_berths(id) ON DELETE RESTRICT,
    cabin_label_snapshot   text        NOT NULL,
    berth_label_snapshot   text        NOT NULL,
    display_label_snapshot text        NOT NULL,
    assigned_by_user_id    uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    assigned_at            timestamptz NOT NULL DEFAULT now(),
    unassigned_by_user_id  uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    unassigned_at          timestamptz NULL,
    notes                  text        NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX trip_cabin_assignments_one_active_guest_idx
    ON trip_cabin_assignments(trip_guest_id)
    WHERE unassigned_at IS NULL;

CREATE UNIQUE INDEX trip_cabin_assignments_one_active_berth_per_trip_idx
    ON trip_cabin_assignments(trip_id, berth_id)
    WHERE unassigned_at IS NULL;

CREATE INDEX trip_cabin_assignments_org_trip_idx
    ON trip_cabin_assignments(organization_id, trip_id);

CREATE INDEX trip_cabin_assignments_berth_active_idx
    ON trip_cabin_assignments(berth_id)
    WHERE unassigned_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS trip_cabin_assignments;
DROP TABLE IF EXISTS boat_cabin_berths;
DROP TABLE IF EXISTS boat_cabins;

-- +goose StatementEnd
