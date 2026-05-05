-- +goose Up
-- +goose StatementBegin

-- Sprint 012: native trip import.
--
-- Adds:
--   - trips.num_guests (operator/spreadsheet-owned)
--   - import_jobs (queued/running/succeeded/failed records for both
--                  liveaboard.com scrapes and spreadsheet commits)
--   - import_previews (server-persisted spreadsheet previews referenced
--                      by preview_id at commit time; 1h expiry)

ALTER TABLE trips
    ADD COLUMN num_guests integer NULL;

CREATE TABLE import_jobs (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    started_by      uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    source          text        NOT NULL CHECK (source IN ('liveaboard_com', 'spreadsheet')),
    source_input    text        NOT NULL,

    status          text        NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),

    boats_inserted  integer     NULL,
    boats_updated   integer     NULL,
    trips_inserted  integer     NULL,
    trips_updated   integer     NULL,
    trips_deleted   integer     NULL,
    error_message   text        NULL,

    started_at      timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz NULL
);

CREATE INDEX import_jobs_org_idx     ON import_jobs(organization_id, started_at DESC);
CREATE INDEX import_jobs_status_idx  ON import_jobs(status) WHERE status IN ('queued', 'running');

CREATE TABLE import_previews (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    started_by      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename        text        NOT NULL,
    payload         jsonb       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL DEFAULT (now() + interval '1 hour')
);

CREATE INDEX import_previews_org_idx     ON import_previews(organization_id, created_at DESC);
CREATE INDEX import_previews_expires_idx ON import_previews(expires_at);

-- +goose StatementEnd
