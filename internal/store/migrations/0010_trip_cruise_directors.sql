-- +goose Up
-- +goose StatementBegin

-- Sprint 013: 1:N cruise-director assignment.
--
-- Sprint 008 modeled trip leadership as a single nullable FK on
-- trips.cruise_director_user_id. In practice an operator may need
-- multiple Cruise Directors on a single trip (training, larger
-- vessels, language coverage, hand-offs mid-trip). Move to a join
-- table; preserve every existing assignment.

CREATE TABLE trip_cruise_directors (
    trip_id     uuid        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    assigned_by uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (trip_id, user_id)
);

CREATE INDEX trip_cruise_directors_user_idx ON trip_cruise_directors(user_id);

-- Migrate existing 1:1 assignments into the join table.
INSERT INTO trip_cruise_directors (trip_id, user_id)
SELECT id, cruise_director_user_id
  FROM trips
 WHERE cruise_director_user_id IS NOT NULL;

-- Drop the old single-FK column + its index. Sprint 010 named the
-- index trips_cruise_director_user_id_idx after renaming it from
-- trips_site_director_user_id_idx in migration 0008.
DROP INDEX IF EXISTS trips_cruise_director_user_id_idx;
ALTER TABLE trips DROP COLUMN cruise_director_user_id;

-- +goose StatementEnd
