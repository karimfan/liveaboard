# Sprint 019 Merge Notes

## Inputs

- Product seed: real-time consumption ledger for the Cruise Director's
  daily onboard workflow.
- Codex draft: trip-level quick-add UI, line-write stock posting,
  idempotency, trip-start folio open, checkout remains settlement.
- Product interview:
  - no batch add in Sprint 019;
  - allow negative counted stock with a warning;
  - roles remain Org Admin and assigned Cruise Director;
  - hide voided/corrected lines from the folio while preserving audit
    and stock movement history.
- Claude critique: schema constraints currently forbid negative stock,
  single `stock_movement_id` is the wrong shape, idempotency must cover
  lazy-open races, crew-tip uniqueness must be void-aware, and tests need
  concurrency/idempotency coverage.

## Merged Decisions

- Sprint 019 builds single-guest quick add only.
- Consumption line writes are active-trip operations.
- Counted inventory may go negative; the UI/API surfaces warnings rather
  than blocking the line.
- Inventory rows may be created at zero for a trip boat when first
  charged, then taken negative.
- `stock_movements` remains the source of stock history. Folio lines get
  `stock_posted_at`, void metadata, and `client_request_id`, but no
  single movement foreign key.
- Idempotency is keyed by `trip_guest_id` and `client_request_id`.
- Voided lines are hidden from folio totals/views; audit and stock
  movements retain the correction history.
- Checkout close no longer decrements stock for already posted lines.

## Final Sprint Doc

Created `docs/sprints/SPRINT-019.md` and synced the tracker as a planned
sprint.
