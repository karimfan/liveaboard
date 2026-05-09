-- +goose Up
-- +goose StatementBegin

ALTER TABLE trips
    ADD COLUMN status text NOT NULL DEFAULT 'planned'
        CHECK (status IN ('planned', 'active', 'completed', 'cancelled')),
    ADD COLUMN started_at timestamptz NULL,
    ADD COLUMN started_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN completed_at timestamptz NULL,
    ADD COLUMN completed_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN cancelled_at timestamptz NULL,
    ADD COLUMN cancelled_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN cancellation_reason text NULL
        CHECK (cancellation_reason IS NULL OR char_length(cancellation_reason) <= 500),
    ADD COLUMN removed_from_source_at timestamptz NULL;

CREATE INDEX trips_org_status_start_idx
    ON trips(organization_id, status, start_date)
    WHERE removed_from_source_at IS NULL;

CREATE INDEX trips_org_removed_source_idx
    ON trips(organization_id, removed_from_source_at, start_date)
    WHERE removed_from_source_at IS NOT NULL;

CREATE OR REPLACE FUNCTION reject_trip_delete_with_history()
RETURNS trigger AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM trip_guests WHERE trip_id = OLD.id)
        OR EXISTS (SELECT 1 FROM trip_cabin_assignments WHERE trip_id = OLD.id)
        OR EXISTS (SELECT 1 FROM guest_documents WHERE trip_id = OLD.id)
        OR EXISTS (SELECT 1 FROM guest_folios WHERE trip_id = OLD.id)
        OR EXISTS (SELECT 1 FROM audit_events WHERE trip_id = OLD.id)
    THEN
        RAISE EXCEPTION 'trips with operational history must be retained';
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trips_no_delete_with_history
    BEFORE DELETE ON trips
    FOR EACH ROW EXECUTE FUNCTION reject_trip_delete_with_history();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trips_no_delete_with_history ON trips;
DROP FUNCTION IF EXISTS reject_trip_delete_with_history();
DROP INDEX IF EXISTS trips_org_removed_source_idx;
DROP INDEX IF EXISTS trips_org_status_start_idx;
ALTER TABLE trips
    DROP COLUMN IF EXISTS removed_from_source_at,
    DROP COLUMN IF EXISTS cancellation_reason,
    DROP COLUMN IF EXISTS cancelled_by_user_id,
    DROP COLUMN IF EXISTS cancelled_at,
    DROP COLUMN IF EXISTS completed_by_user_id,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS started_by_user_id,
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS status;
-- +goose StatementEnd
