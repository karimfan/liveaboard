-- +goose Up
-- +goose StatementBegin

-- Sprint 019: folio lines become the real-time consumption ledger.
-- Counted stock can go negative operationally; the UI warns instead of
-- blocking line entry.

ALTER TABLE boat_inventory_items
    DROP CONSTRAINT IF EXISTS boat_inventory_items_quantity_on_hand_check;

ALTER TABLE stock_movements
    DROP CONSTRAINT IF EXISTS stock_movements_quantity_before_check,
    DROP CONSTRAINT IF EXISTS stock_movements_quantity_after_check;

ALTER TABLE guest_folio_lines
    ADD COLUMN trip_guest_id uuid NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
    ADD COLUMN stock_posted_at timestamptz NULL,
    ADD COLUMN voided_at timestamptz NULL,
    ADD COLUMN voided_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN void_reason text NULL,
    ADD COLUMN client_request_id text NULL,
    ADD CONSTRAINT guest_folio_lines_client_request_len_check
        CHECK (client_request_id IS NULL OR char_length(client_request_id) <= 64);

UPDATE guest_folio_lines l
SET trip_guest_id = f.trip_guest_id
FROM guest_folios f
WHERE f.id = l.folio_id AND l.trip_guest_id IS NULL;

ALTER TABLE guest_folio_lines
    ALTER COLUMN trip_guest_id SET NOT NULL;

DROP INDEX IF EXISTS guest_folio_lines_one_tip_idx;

CREATE UNIQUE INDEX guest_folio_lines_one_tip_idx
    ON guest_folio_lines(folio_id)
    WHERE line_type = 'crew_tip' AND voided_at IS NULL;

CREATE UNIQUE INDEX guest_folio_lines_trip_guest_request_idx
    ON guest_folio_lines(trip_guest_id, client_request_id)
    WHERE client_request_id IS NOT NULL;

CREATE INDEX guest_folio_lines_active_folio_idx
    ON guest_folio_lines(folio_id, sort_order, created_at)
    WHERE voided_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS guest_folio_lines_active_folio_idx;
DROP INDEX IF EXISTS guest_folio_lines_trip_guest_request_idx;
DROP INDEX IF EXISTS guest_folio_lines_one_tip_idx;

CREATE UNIQUE INDEX guest_folio_lines_one_tip_idx
    ON guest_folio_lines(folio_id)
    WHERE line_type = 'crew_tip';

ALTER TABLE guest_folio_lines
    DROP CONSTRAINT IF EXISTS guest_folio_lines_client_request_len_check,
    DROP COLUMN IF EXISTS client_request_id,
    DROP COLUMN IF EXISTS void_reason,
    DROP COLUMN IF EXISTS voided_by_user_id,
    DROP COLUMN IF EXISTS voided_at,
    DROP COLUMN IF EXISTS stock_posted_at,
    DROP COLUMN IF EXISTS trip_guest_id;

ALTER TABLE boat_inventory_items
    ADD CONSTRAINT boat_inventory_items_quantity_on_hand_check CHECK (quantity_on_hand >= 0);

ALTER TABLE stock_movements
    ADD CONSTRAINT stock_movements_quantity_before_check CHECK (quantity_before >= 0),
    ADD CONSTRAINT stock_movements_quantity_after_check CHECK (quantity_after >= 0);

-- +goose StatementEnd
