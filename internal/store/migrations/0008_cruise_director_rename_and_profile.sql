-- +goose Up
-- +goose StatementBegin

-- Sprint 010: Site Director -> Cruise Director rename, plus richer
-- invitation metadata (full_name + phone) captured at invite-time.
--
-- Order matters: drop both role checks before rewriting role values,
-- revoke pre-existing pending invitations (their full_name would be
-- blank under the new contract), then recreate constraints, rename
-- the trip column + index, and finally add the new columns.

-- 1. Drop both role checks so we can rewrite values + recreate
--    with the new allowed-set.
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE invitations
    DROP CONSTRAINT IF EXISTS invitations_role_check;

-- 2. Rewrite role strings on existing rows.
UPDATE users
    SET role = 'cruise_director'
    WHERE role = 'site_director';
UPDATE invitations
    SET role = 'cruise_director'
    WHERE role = 'site_director';

-- 3. Revoke every pre-Sprint-010 pending invitation. The new contract
--    requires invitations.full_name; any older pending row has none.
--    Pre-customer; cleaner to revoke and let admins re-invite with
--    full metadata than to carry blank-name fallbacks forever.
UPDATE invitations
    SET revoked_at = now(), updated_at = now()
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- 4. Recreate role checks with the new value set.
ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('org_admin', 'cruise_director'));
ALTER TABLE invitations
    ADD CONSTRAINT invitations_role_check
    CHECK (role IN ('cruise_director'));

-- 5. Rename trip assignment column + supporting index.
ALTER TABLE trips
    RENAME COLUMN site_director_user_id TO cruise_director_user_id;
ALTER INDEX trips_site_director_user_id_idx
    RENAME TO trips_cruise_director_user_id_idx;

-- 6. Add the new metadata columns.
ALTER TABLE users
    ADD COLUMN phone text NULL;

-- invitations.full_name is required for new rows. We add it with a
-- temporary default of '' so the ADD COLUMN succeeds even if any
-- non-revoked rows survive (they shouldn't given step 3, but the
-- default is defense-in-depth). Then drop the default so future
-- inserts must specify a real name.
ALTER TABLE invitations
    ADD COLUMN full_name text NOT NULL DEFAULT '',
    ADD COLUMN phone     text NULL;

ALTER TABLE invitations
    ALTER COLUMN full_name DROP DEFAULT;

-- +goose StatementEnd
