-- +goose Up
-- +goose StatementBegin

CREATE TABLE guest_users (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email              citext      NOT NULL UNIQUE,
    password_hash      bytea       NOT NULL,
    email_verified_at  timestamptz NOT NULL,
    is_active          boolean     NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE guest_sessions (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    guest_user_id   uuid        NOT NULL REFERENCES guest_users(id) ON DELETE CASCADE,
    token_hash      bytea       NOT NULL UNIQUE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    revoked_at      timestamptz NULL
);
CREATE INDEX guest_sessions_guest_user_id_idx ON guest_sessions(guest_user_id);
CREATE INDEX guest_sessions_expires_at_idx ON guest_sessions(expires_at);

CREATE TABLE trip_guests (
    id                         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id            uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id                    uuid        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    guest_user_id              uuid        NULL REFERENCES guest_users(id) ON DELETE SET NULL,
    invited_by_user_id         uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    full_name                  text        NOT NULL CHECK (length(trim(full_name)) > 0),
    email                      citext      NOT NULL,
    invite_send_status         text        NOT NULL DEFAULT 'not_sent' CHECK (invite_send_status IN ('not_sent','sent','failed')),
    invite_last_sent_at        timestamptz NULL,
    invite_last_error          text        NULL,
    account_created_at         timestamptz NULL,
    registration_submitted_at  timestamptz NULL,
    revoked_at                 timestamptz NULL,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, trip_id, email)
);
CREATE INDEX trip_guests_org_trip_idx ON trip_guests(organization_id, trip_id);
CREATE INDEX trip_guests_guest_user_idx ON trip_guests(guest_user_id);

CREATE TABLE guest_trip_invitations (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id          uuid        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    trip_guest_id    uuid        NOT NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
    email            citext      NOT NULL,
    token_hash       bytea       NOT NULL UNIQUE,
    expires_at       timestamptz NOT NULL,
    accepted_at      timestamptz NULL,
    revoked_at       timestamptz NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX guest_trip_invitations_trip_guest_pending_idx
    ON guest_trip_invitations(trip_guest_id)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE INDEX guest_trip_invitations_expires_idx ON guest_trip_invitations(expires_at);

CREATE TABLE guest_trip_registrations (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id         uuid        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    trip_guest_id   uuid        NOT NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
    guest_user_id   uuid        NOT NULL REFERENCES guest_users(id) ON DELETE CASCADE,
    status          text        NOT NULL CHECK (status IN ('draft','submitted')),
    payload         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    submitted_at    timestamptz NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trip_guest_id)
);
CREATE INDEX guest_trip_registrations_org_trip_idx ON guest_trip_registrations(organization_id, trip_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS guest_trip_registrations;
DROP TABLE IF EXISTS guest_trip_invitations;
DROP TABLE IF EXISTS trip_guests;
DROP TABLE IF EXISTS guest_sessions;
DROP TABLE IF EXISTS guest_users;

-- +goose StatementEnd
