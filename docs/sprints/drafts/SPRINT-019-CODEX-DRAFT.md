# Sprint 019 Codex Draft: Real-Time Consumption Ledger

## Overview

Sprint 019 changes guest folios from an end-of-trip checkout worksheet
into the live consumption ledger used during an active trip. The existing
schema already has the core primitives: one openable `guest_folios` row
per trip guest, `guest_folio_lines` that snapshot price/name/stock mode,
and inventory movements that can reference a folio line. The missing
piece is operational behavior.

The target state is simple: when a guest buys a beer, rental, service, or
other catalog item, the Director records it immediately, the folio line is
created immediately, and counted boat stock decrements immediately. The
checkout screen then becomes settlement: review lines, add optional crew
tip, collect offline payment, close the folio, and email the itemized
folio.

## Use Cases

1. **Open trip ledgers at trip start.** When a trip moves to `active`,
   every active trip guest gets an open folio. Late-added guests get a
   folio automatically on first line-add if one does not exist.
2. **Record a bar sale quickly.** Cruise Director opens the trip ledger
   on a phone, selects a guest, taps a common item, confirms quantity,
   and sees the line recorded without navigating to checkout.
3. **Correct a mistake.** Staff can edit quantity or remove a line from
   an open folio, and inventory is adjusted back through a matching stock
   movement.
4. **Warn on negative stock.** Counted catalog items may take boat
   inventory negative, but the UI clearly warns staff when an item is low,
   out, or driven below zero.
5. **Handle simultaneous staff entry.** Multiple authorized staff can add
   lines at meals without corrupting folio sort order, totals, or boat
   inventory.
6. **Settle at checkout.** Director reviews the guest folio, adds an
   optional card crew-tip if requested, chooses offline payment method and
   settlement currency, closes as paid, and sends the folio email.

## Architecture

### Core Rules

- One folio exists per `trip_guest`, unchanged from Sprint 015.
- Folios should be open while the trip is active.
- `StartTrip` opens missing folios for all non-revoked guests after the
  lifecycle transition succeeds in the same transaction.
- First line-add lazy-opens a folio when needed, so active late-added
  guests and partially migrated data still work.
- Consumption line writes are allowed only for `active` trips.
- Checkout close is allowed for `active` trips and remains blocked on
  `completed` and `cancelled` trips by the existing lifecycle guard.
- Crew tips remain checkout-only by default. They do not affect stock.
- Batch add is out of scope for Sprint 019; all quick adds target one
  guest at a time.
- Catalog line price, item name, and stock mode continue to snapshot from
  the catalog at line creation.
- Counted stock posts at line creation, not at close.
- Updating or deleting a posted counted line posts a compensating stock
  movement.
- Counted stock may go negative. Negative stock is allowed and surfaced
  as an operational warning, not a mutation blocker.
- Closing a folio does not decrement stock for lines already posted.
- All ledger mutations are transactionally coupled with stock movement
  and audit writes where applicable.

### Inventory Posting Model

Add explicit posting state to folio lines so old and new behavior can
coexist safely.

```sql
ALTER TABLE guest_folio_lines
  ADD COLUMN stock_movement_id uuid NULL REFERENCES stock_movements(id) ON DELETE SET NULL,
  ADD COLUMN stock_posted_at timestamptz NULL,
  ADD COLUMN voided_at timestamptz NULL,
  ADD COLUMN voided_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN void_reason text NULL,
  ADD COLUMN client_request_id text NULL;

CREATE UNIQUE INDEX guest_folio_lines_folio_client_request_idx
  ON guest_folio_lines(folio_id, client_request_id)
  WHERE client_request_id IS NOT NULL;

CREATE INDEX guest_folio_lines_active_folio_idx
  ON guest_folio_lines(folio_id, sort_order, created_at)
  WHERE voided_at IS NULL;
```

Rules:

- New counted catalog lines create a `folio_charge` stock movement in the
  same transaction and set `stock_movement_id` plus `stock_posted_at`.
- Non-counted catalog lines and crew tips leave those columns null.
- Open-folio quantity increase posts an additional negative
  `folio_charge` movement for the delta, even if that drives quantity
  below zero.
- Open-folio quantity decrease posts a positive `folio_void` movement for
  the delta.
- Removing a posted line sets `voided_at` and posts a positive
  `folio_void` movement for the active quantity.
- Guest-facing and checkout totals ignore voided lines.
- Closed historical folios created before Sprint 019 may have null
  `stock_posted_at`; close logic should not re-run on already closed
  folios. For open legacy folios, close can post only unposted counted
  lines as a compatibility fallback, then mark them posted.

### Concurrency

All add/update/delete operations should run in a database transaction.
The transaction should lock:

- the active trip row enough to verify status;
- the target `trip_guest` row enough to verify it is not revoked;
- the target `guest_folios` row with `FOR UPDATE`, creating it if needed;
- the target `guest_folio_lines` row with `FOR UPDATE` for edits/deletes;
- the relevant `boat_inventory_items` row with `FOR UPDATE` before stock
  arithmetic.

`sort_order` should be assigned inside the folio lock, or replaced with
a timestamp-first display ordering if sort order races are not worth the
complexity. The draft recommends keeping `sort_order` but deriving it
while the folio row is locked.

`client_request_id` should be accepted on add-line requests. Mobile UI
generates one id per submit. If the same request is retried, the API
returns the existing resulting folio state instead of adding a duplicate
line.

### Store Layer

Modify `internal/store/guest_folios.go`:

- Add `ClientRequestID` to `AddFolioLineInput`.
- Add `VoidReason` to delete input if the API collects one.
- Expand `GuestFolioLine` with stock posting and void metadata.
- Replace close-time stock decrement with line-write stock posting.
- Keep a close-time fallback for unposted counted lines on open legacy
  folios only.
- Filter active line reads by `voided_at IS NULL`, and add an internal
  option if staff needs to see voided history later.
- Add helper methods:
  - `OpenGuestFoliosForTripTx(ctx, tx, orgID, tripID, actorID)`
  - `folioForMutationTx(...)`
  - `insertFolioLineTx(...)`
  - `postFolioStockMovementTx(...)`
  - `reverseFolioStockMovementTx(...)`

Modify `internal/store/trip_lifecycle.go`:

- During `StartTrip`, open missing folios for active guests in the same
  transaction after readiness passes and before audit commit.
- Update readiness messaging so missing folios before start are no longer
  surprising once trip start pre-opens them.

Modify `internal/store/inventory.go` only if a shared transaction helper
is cleaner than duplicating stock arithmetic. Keep public `AdjustStock`
for manual admin adjustments.

### HTTP API

Keep existing per-guest folio endpoints for checkout and detail.

Add trip-level ledger endpoints:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/ledger` | GET | Admin or assigned CD | Return active guests, open folio summaries, catalog items, inventory state, and recent lines. |
| `/api/admin/trips/{id}/ledger/lines` | POST | Admin or assigned CD | Add one catalog item line to one guest folio. |

Single add payload:

```json
{
  "trip_guest_id": "uuid",
  "catalog_item_id": "uuid",
  "quantity": 1,
  "client_request_id": "uuid-or-ulid"
}
```

Existing per-guest `POST /folio/lines` should call the same store path
so checkout corrections and quick ledger writes have identical inventory
semantics. Existing `PATCH` and `DELETE` line endpoints must also adjust
posted stock.

Error mapping:

- negative stock: success response with warning metadata, not an error
- duplicate idempotency key: `200` with current folio or ledger state
- inactive/non-active trip: `409 trip_not_active`
- closed folio: `409 folio_closed`
- completed/cancelled trip: existing lifecycle mutation error

### Frontend

Add a trip-level ledger page, for example
`web/src/admin/pages/TripConsumptionLedger.tsx`, linked from the active
trip manifest lifecycle banner and guest list.

Design principles:

- Mobile-first working surface, no decorative card-heavy layout.
- Large guest and item hit targets with stable dimensions.
- Sticky bottom confirmation/action bar on narrow screens.
- Search/filter for guests and catalog items.
- Category chips or tabs for catalog categories.
- Recent/frequent item strip for bar work.
- Quantity stepper with one-tap `+1` default.
- Clear stock status for counted items: in stock, low, out.
- If an add drives stock below zero, keep the line and show a warning.
- Immediate success/error feedback without moving the operator away from
  the current guest/item context.
- Avoid placing the live ledger inside the checkout page; checkout is a
  slower review and settlement flow.

`GuestFolio.tsx` remains the checkout view but should use the refactored
line mutation APIs. It should make clear that stock has already been
posted for active consumption lines and that close is payment settlement.

### Audit

Record or preserve:

- `guest.folio_opened` when a folio is opened at trip start or lazy open.
- `guest.folio_line_added` with safe metadata:
  `{line_type, catalog_item_id, item_name, quantity, stock_posted}`.
- `guest.folio_line_updated` with quantity delta for catalog lines.
- `guest.folio_line_deleted` or a new `guest.folio_line_voided` when a
  line is removed after stock was posted.
- `inventory.adjusted` or existing stock movement records for
  `folio_charge` and `folio_void`.

Avoid raw guest PII and full folio payloads in audit metadata.

## Implementation Plan

### Phase 1: Schema and Store Safety

Files:

- `internal/store/migrations/0018_realtime_consumption_ledger.sql`
- `internal/store/guest_folios.go`
- `internal/store/guest_folios_test.go`
- `internal/store/trip_lifecycle.go`
- `internal/store/trip_lifecycle_test.go`

Work:

- Add stock posting, void, and idempotency columns.
- Update line scanning and active-line filtering.
- Move counted stock decrement into add-line transactions.
- Add stock reversal for update/delete.
- Make close skip already posted lines and retain legacy fallback for
  open unposted counted lines.
- Open missing folios during trip start.

### Phase 2: Ledger API

Files:

- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/guest_folio_test.go`

Work:

- Add trip-level ledger read and add endpoints.
- Thread `client_request_id` through add-line requests.
- Reuse the same store method from per-guest and trip-level endpoints.
- Map stock, lifecycle, and idempotency errors explicitly.
- Record safe audit events for auto-open and line writes.

### Phase 3: Mobile Ledger UI

Files:

- `web/src/admin/pages/TripConsumptionLedger.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/pages/GuestFolio.tsx`
- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/styles/app.css`

Work:

- Add route and navigation from the trip manifest.
- Build the mobile-friendly add-line workflow.
- Show guests, catalog categories/items, stock status, quantity controls,
  and recent lines.
- Generate a per-submit `client_request_id`.
- Keep checkout available for review, tip, payment, close, and email.

### Phase 4: Verification and Docs

Files:

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- final `docs/sprints/SPRINT-019.md`

Tests:

- Folios open at trip start.
- Lazy open on first active-trip add.
- Counted item add decrements stock once.
- Close does not double-decrement posted lines.
- Quantity increase/decrease posts correct stock deltas.
- Delete/void reverses posted stock.
- Negative stock succeeds and returns/display warning metadata.
- Duplicate `client_request_id` does not duplicate a line.
- Concurrent add-line requests preserve stock integrity.
- Completed/cancelled trips reject ledger mutation.
- Director can mutate only assigned trips.
- Frontend build passes.

## Definition of Done

- Sprint 019 final doc is completed and marked `completed` only after
  implementation.
- Backend tests cover stock posting, reversal, idempotency, lazy open,
  trip-start open, and lifecycle guards.
- Existing checkout behavior still closes a folio, records offline
  payment details, applies card fees, snapshots FX, sends email, and
  blocks closed folio mutations.
- Trip ledger UI works on mobile widths with no overlapping controls or
  text overflow.
- `go test ./...`, `go vet ./...`, and frontend build pass before commit.

## Open Questions

Resolved by product interview:

1. No batch add in Sprint 019; build single-guest quick add only.
2. Allow negative counted stock with a prominent warning. Operationally
   infinite items should remain `stock_mode = none`.
3. Keep roles limited to Org Admin and assigned Cruise Director.
4. Hide voided/corrected lines from the folio view while preserving
   audit and stock movement history underneath.
