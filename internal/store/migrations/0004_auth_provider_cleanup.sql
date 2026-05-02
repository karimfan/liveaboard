-- +goose Up
-- +goose StatementBegin

-- Sprint 005 cutover cleanup. By the time this migration runs, every
-- /api/* path goes through Clerk -> exchange -> lb_session, and every
-- users row created since migration 0002 has clerk_user_id populated.
--
-- This migration is destructive on purpose: we are pre-customer, dev
-- data is throwaway, and any users created on the legacy Sprint 003
-- path (without clerk_user_id) would be orphaned by the cutover anyway.
-- We delete those rows defensively before tightening the constraints.

DELETE FROM users WHERE clerk_user_id IS NULL;
DELETE FROM organizations WHERE clerk_org_id IS NULL;

ALTER TABLE users
    ALTER COLUMN clerk_user_id SET NOT NULL;

ALTER TABLE organizations
    ALTER COLUMN clerk_org_id SET NOT NULL;

ALTER TABLE users
    DROP COLUMN password_hash;

ALTER TABLE users
    DROP COLUMN email_verified_at;

DROP TABLE sessions;
DROP TABLE email_verifications;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE TABLE email_verifications (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_verifications_user_id_idx ON email_verifications(user_id);

CREATE TABLE sessions (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   bytea       NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

ALTER TABLE users
    ADD COLUMN email_verified_at timestamptz NULL;

-- password_hash cannot be reliably recovered. Re-add as nullable so the
-- column exists; values are gone forever.
ALTER TABLE users
    ADD COLUMN password_hash bytea NULL;

ALTER TABLE organizations
    ALTER COLUMN clerk_org_id DROP NOT NULL;

ALTER TABLE users
    ALTER COLUMN clerk_user_id DROP NOT NULL;

-- +goose StatementEnd
