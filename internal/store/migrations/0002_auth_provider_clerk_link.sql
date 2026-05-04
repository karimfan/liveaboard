-- +goose Up
-- +goose StatementBegin

-- 1. Provider linkage. NULLABLE during cutover; promoted to NOT NULL in 0003
--    after Sprint 005's Phase 6 manual smoke validates end-to-end.
ALTER TABLE users
    ADD COLUMN clerk_user_id text UNIQUE;

ALTER TABLE organizations
    ADD COLUMN clerk_org_id text UNIQUE;

-- 2. Make legacy local-credential columns NULLABLE so newly-Clerk-backed
--    users can be inserted without a password hash. Migration 0003 drops
--    these columns entirely once the cutover is complete.
ALTER TABLE users
    ALTER COLUMN password_hash DROP NOT NULL;

-- 3. app_sessions is the cookie <-> provider-session bridge. The SPA
--    continues to authenticate via the lb_session cookie; this table maps
--    cookie token sha256 -> local user + Clerk user/session ids.
CREATE TABLE app_sessions (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash        bytea       NOT NULL UNIQUE,
    clerk_user_id     text        NOT NULL,
    clerk_session_id  text        NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    last_seen_at      timestamptz NOT NULL DEFAULT now(),
    expires_at        timestamptz NOT NULL
);
CREATE INDEX app_sessions_user_id_idx          ON app_sessions(user_id);
CREATE INDEX app_sessions_expires_at_idx       ON app_sessions(expires_at);
CREATE INDEX app_sessions_clerk_session_id_idx ON app_sessions(clerk_session_id);

-- 4. webhook_events provides idempotency for Clerk webhook delivery.
--    Keyed by the svix-id header so replays no-op.
CREATE TABLE webhook_events (
    id          text        PRIMARY KEY,
    received_at timestamptz NOT NULL DEFAULT now()
);

-- 5. auth_sync_cursors reserves space for a future reconciler that
--    periodically diffs Clerk -> local users. The reconciler itself is
--    out of scope for Sprint 005; the table exists so adding it later is
--    a code change only.
CREATE TABLE auth_sync_cursors (
    name       text        PRIMARY KEY,
    cursor     text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS auth_sync_cursors;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS app_sessions;

ALTER TABLE users
    ALTER COLUMN password_hash SET NOT NULL;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS clerk_org_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS clerk_user_id;

-- +goose StatementEnd
