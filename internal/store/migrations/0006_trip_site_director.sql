-- +goose Up
-- +goose StatementBegin

-- Add the Site Director assignment column to trips. Nullable because
-- an unassigned trip is the default state; the Sprint 008 Overview
-- card "trips needing attention" depends on the NULL state.

ALTER TABLE trips
    ADD COLUMN site_director_user_id uuid NULL
    REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX trips_site_director_user_id_idx
    ON trips(site_director_user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS trips_site_director_user_id_idx;
ALTER TABLE trips DROP COLUMN IF EXISTS site_director_user_id;

-- +goose StatementEnd
