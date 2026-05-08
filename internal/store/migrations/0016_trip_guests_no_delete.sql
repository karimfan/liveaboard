-- +goose Up
-- +goose StatementBegin

CREATE OR REPLACE FUNCTION reject_trip_guests_delete()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'trip_guests are historical records and must be revoked, not deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trip_guests_no_delete
    BEFORE DELETE ON trip_guests
    FOR EACH ROW EXECUTE FUNCTION reject_trip_guests_delete();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trip_guests_no_delete ON trip_guests;
DROP FUNCTION IF EXISTS reject_trip_guests_delete();
-- +goose StatementEnd
