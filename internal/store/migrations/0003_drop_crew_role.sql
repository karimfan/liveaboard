-- +goose Up
-- +goose StatementBegin

-- Crew is no longer a persona in the product. The MVP only needs Org Admins
-- and Site Directors; consumption recording is a Site Director responsibility
-- now. See docs/product/personas.md for the updated persona model.
--
-- We are pre-customer; no production data exists. We defensively delete any
-- dev rows that happen to carry role='crew' so the constraint swap is clean.

DELETE FROM users WHERE role = 'crew';

ALTER TABLE users
    DROP CONSTRAINT users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('org_admin','site_director'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Restore the crew option in the role check. Note: Down cannot recover any
-- crew users that were deleted by Up; that data is gone forever.
ALTER TABLE users
    DROP CONSTRAINT users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('org_admin','site_director','crew'));

-- +goose StatementEnd
