-- +goose Up
-- +goose StatementBegin

-- Sprint 023: per-org dismissal flag for the onboarding wizard.
-- Auto-show is also gated by derived onboarding completeness
-- (currency + boats + usable cabin layouts + at least one director),
-- so no backfill is necessary — already-set-up orgs return
-- onboarding_complete=true regardless of this timestamp.
ALTER TABLE organizations
    ADD COLUMN onboarding_dismissed_at timestamptz NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE organizations
    DROP COLUMN IF EXISTS onboarding_dismissed_at;

-- +goose StatementEnd
