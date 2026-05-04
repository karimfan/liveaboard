-- +goose Up
-- +goose StatementBegin

-- Sprint 009. Reverses Sprint 005's Clerk migration, restores Sprint
-- 003's custom-auth schema with a bcrypt password column + opaque
-- cookie sessions, and adds tables for invitation, password reset,
-- email-change, and login-attempt tracking.
--
-- Ordering matters: rows DELETED before constraints tightened.
-- Otherwise any Clerk-era dev DB would fail mid-statement when
-- users.password_hash flips to NOT NULL.

-- 1. Wipe legacy rows. Pre-customer; clean slate.
DELETE FROM trips;
DELETE FROM boats;
DELETE FROM users;
DELETE FROM organizations;

-- 2. Drop Clerk integration tables.
DROP TABLE IF EXISTS app_sessions;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS auth_sync_cursors;

-- 3. Drop Clerk linkage columns.
ALTER TABLE users         DROP COLUMN IF EXISTS clerk_user_id;
ALTER TABLE organizations DROP COLUMN IF EXISTS clerk_org_id;

-- 4. Restore Sprint-003 columns dropped by migration 0004.
ALTER TABLE users
    ADD COLUMN password_hash bytea NOT NULL,
    ADD COLUMN email_verified_at timestamptz NULL;

-- 5. Restore Sprint-003 sessions / email_verifications tables.
CREATE TABLE sessions (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   bytea       NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz NULL
);
CREATE INDEX sessions_user_id_idx     ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx  ON sessions(expires_at);

CREATE TABLE email_verifications (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_verifications_user_id_idx ON email_verifications(user_id);

-- 6. Invitations.
CREATE TABLE invitations (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id    uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email              citext      NOT NULL,
    role               text        NOT NULL CHECK (role IN ('site_director')),
    invited_by_user_id uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash         bytea       NOT NULL UNIQUE,
    expires_at         timestamptz NOT NULL,
    accepted_at        timestamptz NULL,
    accepted_user_id   uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    revoked_at         timestamptz NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invitations_org_idx     ON invitations(organization_id);
CREATE INDEX invitations_email_idx   ON invitations(email);
CREATE INDEX invitations_expires_idx ON invitations(expires_at);
-- Only one PENDING invitation per (org, email).
CREATE UNIQUE INDEX invitations_pending_unique_idx
    ON invitations(organization_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- 7. Password reset tokens.
CREATE TABLE password_reset_tokens (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens(user_id);

-- 8. Email-change requests (two-phase: new email pending until confirmation).
CREATE TABLE email_change_requests (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    new_email   citext      NOT NULL UNIQUE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX email_change_requests_pending_unique_idx
    ON email_change_requests(user_id)
    WHERE consumed_at IS NULL;

-- 9. Login attempts (per-email cooldown).
CREATE TABLE login_attempts (
    email          citext      PRIMARY KEY,
    failed_count   integer     NOT NULL DEFAULT 0,
    last_failed_at timestamptz NOT NULL DEFAULT now(),
    locked_until   timestamptz NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS email_change_requests;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS email_verifications;
DROP TABLE IF EXISTS sessions;

ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified_at,
    DROP COLUMN IF EXISTS password_hash;

-- Note: this Down does not recreate Clerk linkage columns or tables.
-- Migration 0007 is one-way once applied. Restoring Clerk would mean
-- branching back to clerk-archive, not running this Down.

-- +goose StatementEnd
