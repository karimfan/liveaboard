# Sprint 019: Real-Time Consumption Ledger

## Overview

Sprint 019 turns guest folios from an end-of-trip checkout worksheet into
the live consumption ledger used during an active trip. Cruise Directors
need to record onboard purchases as they happen, often on a phone while
working a bar or meal service. The existing data model is close: one
folio per trip guest, folio lines snapshot prices, and stock movements
can reference operational sources. The missing behavior is the live
ledger workflow.

After this sprint, active trip guests have open folios, single-guest
quick-add writes a line immediately, counted boat inventory updates in
the same transaction, and checkout only settles offline payment, applies
fees/FX, closes the folio, and emails the guest.

## Use Cases

1. **Open trip ledgers at trip start.** When a trip becomes `active`,
   missing folios are opened for all non-revoked trip guests.
2. **Lazy-open a late guest.** If a guest is added during an active trip,
   the first consumption line opens their folio automatically.
3. **Record a one-handed sale.** Director selects one guest, taps one
   catalog item, confirms quantity, and records the line from a
   mobile-friendly trip ledger screen.
4. **Warn on low or negative stock.** Counted stock may go below zero,
   but the API and UI warn staff when a charge creates or worsens
   negative inventory.
5. **Correct a mistake.** Staff can update quantity or remove a line
   from an open folio; stock receives the compensating movement.
6. **Handle simultaneous staff entry.** Concurrent authorized users can
   add lines without duplicate mobile submits or corrupt inventory
   arithmetic.
7. **Settle at checkout.** Director reviews the folio, optionally adds a
   crew tip, records offline payment method and currency, closes as paid,
   and emails the itemized folio.

## Architecture

### Core Rules

- One folio exists per `trip_guest`.
- Folios open when a trip starts, with lazy-open on first active-trip
  line-add as a fallback.
- Live consumption writes target one guest at a time. Batch add is out
  of scope for Sprint 019.
- Line writes are allowed for Org Admins and assigned Cruise Directors.
- Consumption line writes are active-trip operations.
- Completed and cancelled trips remain read-only for folio line
  mutation.
- Crew tips remain part of checkout unless already supported by the
  existing per-guest folio page. They do not affect stock.
- Catalog line item name, price, and stock mode continue to snapshot at
  creation time.
- Counted stock posts during add/update/delete, not during checkout
  close.
- Counted stock may go negative. Negative stock is an operational
  warning, not a blocker.
- Voided/corrected lines are hidden from folio totals and guest-facing
  views. Audit and stock movements preserve history.
- Checkout close no longer decrements stock for already posted lines.

### Schema

Create migration `0018_realtime_consumption_ledger.sql`.

Allow negative inventory:

```sql
ALTER TABLE boat_inventory_items
  DROP CONSTRAINT IF EXISTS boat_inventory_items_quantity_on_hand_check;

ALTER TABLE stock_movements
  DROP CONSTRAINT IF EXISTS stock_movements_quantity_before_check,
  DROP CONSTRAINT IF EXISTS stock_movements_quantity_after_check;
```

Add posting, void, and idempotency fields:

```sql
ALTER TABLE guest_folio_lines
  ADD COLUMN trip_guest_id uuid NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
  ADD COLUMN stock_posted_at timestamptz NULL,
  ADD COLUMN voided_at timestamptz NULL,
  ADD COLUMN voided_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN void_reason text NULL,
  ADD COLUMN client_request_id text NULL,
  ADD CONSTRAINT guest_folio_lines_client_request_len_check
    CHECK (client_request_id IS NULL OR char_length(client_request_id) <= 64);
```

Backfill `trip_guest_id` from `guest_folios`, then make it not null:

```sql
UPDATE guest_folio_lines l
SET trip_guest_id = f.trip_guest_id
FROM guest_folios f
WHERE f.id = l.folio_id AND l.trip_guest_id IS NULL;

ALTER TABLE guest_folio_lines
  ALTER COLUMN trip_guest_id SET NOT NULL;
```

Indexes:

```sql
CREATE UNIQUE INDEX guest_folio_lines_trip_guest_request_idx
  ON guest_folio_lines(trip_guest_id, client_request_id)
  WHERE client_request_id IS NOT NULL;

CREATE INDEX guest_folio_lines_active_folio_idx
  ON guest_folio_lines(folio_id, sort_order, created_at)
  WHERE voided_at IS NULL;
```

Recreate the crew-tip uniqueness index so voided tips do not block a new
active tip:

```sql
DROP INDEX IF EXISTS guest_folio_lines_one_tip_idx;

CREATE UNIQUE INDEX guest_folio_lines_one_tip_idx
  ON guest_folio_lines(folio_id)
  WHERE line_type = 'crew_tip' AND voided_at IS NULL;
```

Do not add a `stock_movement_id` column to folio lines. A single line can
have multiple stock movements after quantity edits or voids.
`stock_movements.source_type/source_id` remains the historical stock
source.

### Stock Movement Semantics

All stock movement arithmetic uses the line's snapshotted `stock_mode`.
Catalog changes after a line is written do not affect whether correction
movements are posted.

| Operation | Movement type | Delta |
|---|---:|---:|
| Add counted line, quantity `q` | `folio_charge` | `-q` |
| Increase counted line by `d` | `folio_charge` | `-d` |
| Decrease counted line by `d` | `folio_void` | `+d` |
| Void counted line with active quantity `q` | `folio_void` | `+q` |

If no `boat_inventory_items` row exists for the trip boat and counted
catalog item, the line-write transaction creates one at zero and applies
the movement. This may take quantity negative and should return warning
metadata.

### Concurrency and Idempotency

Use a canonical transaction lock order across add, update, delete, and
close:

1. `trips`
2. `trip_guests`
3. `guest_folios`
4. `guest_folio_lines` when editing or voiding a line
5. `boat_inventory_items`

Line add accepts `client_request_id`. Mobile clients generate one value
per submit. The unique key is `(trip_guest_id, client_request_id)` so
duplicate retries during lazy-open produce one folio and one line. A
duplicate retry returns the same logical result, including the original
line id, rather than creating a fresh line.

### Store Layer

Modify `internal/store/guest_folios.go`:

- Add `TripGuestID`, posting metadata, void metadata, and
  `ClientRequestID` to folio line models and inputs.
- Refactor line reads to ignore `voided_at IS NOT NULL`.
- Make add-line lazy-open a folio when needed for active trips.
- Post counted stock inside the add-line transaction and set
  `stock_posted_at`.
- Allow stock quantities below zero and return warning metadata.
- Update counted line quantity by posting only the stock delta.
- Replace hard delete with hidden void semantics.
- Keep checkout close as settlement and email; do not double-decrement
  posted lines.
- Preserve a close-time compatibility fallback for open legacy counted
  lines with `stock_posted_at IS NULL`.

Modify `internal/store/trip_lifecycle.go`:

- During `StartTrip`, open missing folios for non-revoked guests in the
  same transaction as the status transition and audit event.
- Stop warning at start about missing folios that start will auto-create.
- Keep completion warnings focused on open folios that are not closed.

Modify `internal/store/inventory.go`:

- Remove negative quantity rejection from stock adjustment helpers used
  by folio writes.
- Keep manual inventory adjustment validation appropriate for admin
  stock operations, but ensure shared helpers can support negative folio
  outcomes.

### HTTP API

Keep existing per-guest folio endpoints and route them through the same
store logic as the live ledger.

Add trip-level ledger endpoints:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/ledger` | GET | Org Admin or assigned CD | Return bounded ledger data for the active trip. |
| `/api/admin/trips/{id}/ledger/lines` | POST | Org Admin or assigned CD | Add one catalog item line to one guest folio. |

Single add payload:

```json
{
  "trip_guest_id": "uuid",
  "catalog_item_id": "uuid",
  "quantity": 1,
  "client_request_id": "uuid-or-ulid"
}
```

The ledger GET response should be bounded:

- active non-revoked guests for the trip;
- folio status and current totals per guest;
- catalog items grouped by category;
- inventory rows scoped to the trip boat;
- recent lines capped globally, for example last 50 active lines.

Response warning example:

```json
{
  "folio": {},
  "line": {},
  "warnings": [
    {
      "code": "negative_stock",
      "message": "Stock for Beer is now -2.",
      "catalog_item_id": "uuid",
      "quantity_on_hand": -2
    }
  ]
}
```

Authorization:

- Org Admin can use the ledger for any org trip.
- Cruise Director must be assigned through the current trip director
  assignment table, not a legacy single-director column.

Errors:

- inactive/planned trip for live add: `409 trip_not_active`
- completed/cancelled trip mutation: existing lifecycle mutation error
- closed folio: `409 folio_closed`
- duplicate idempotency key: `200` with original line result
- negative stock: success with warning metadata

### Frontend

Add `web/src/admin/pages/TripConsumptionLedger.tsx`, linked from the
active trip manifest.

The page should be mobile-first and operational:

- large guest and item targets;
- stable grid dimensions so controls do not shift;
- guest search/filter;
- catalog category chips or tabs;
- recent/frequent item area;
- quantity stepper defaulting to `1`;
- sticky bottom submit/action area on narrow screens;
- visible low/out/negative stock state;
- success and warning feedback that keeps the Director in the same flow.

Do not turn the checkout page into the live ledger. `GuestFolio.tsx`
remains the slower review, crew tip, payment, close, and resend-email
surface.

### Audit

Record safe staff audit metadata for:

- `guest.folio_opened`;
- `guest.folio_line_added`;
- `guest.folio_line_updated`;
- `guest.folio_line_deleted` or `guest.folio_line_voided`;
- `guest.folio_closed`;
- inventory movements caused by `folio_charge` and `folio_void`.

Avoid raw guest PII, full folio payloads, or full registration data in
metadata.

## Implementation Plan

### Phase 1: Schema and Store Safety (~40%)

**Files:**

- `internal/store/migrations/0018_realtime_consumption_ledger.sql`
- `internal/store/guest_folios.go`
- `internal/store/inventory.go`
- `internal/store/trip_lifecycle.go`
- `internal/store/guest_folios_test.go`
- `internal/store/trip_lifecycle_test.go`

**Tasks:**

- [x] Add migration for negative stock, line idempotency, posting, void
      metadata, and active crew-tip uniqueness.
- [x] Refactor add/update/delete line logic into transactions with the
      canonical lock order.
- [x] Add lazy-open on active line-add.
- [x] Open missing folios during trip start.
- [x] Move counted stock posting to line writes.
- [x] Make checkout close skip posted lines and retain legacy fallback.
- [x] Return warning metadata for negative stock outcomes.

### Phase 2: Ledger API (~20%)

**Files:**

- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/guest_folio_test.go`

**Tasks:**

- [x] Add bounded trip ledger read endpoint.
- [x] Add single-guest ledger line-add endpoint.
- [x] Thread `client_request_id` through per-guest and ledger line-add
      endpoints.
- [x] Route existing per-guest line add/update/delete through the same
      stock-posting semantics.
- [x] Map trip status, closed folio, idempotency, and warning responses
      explicitly.

### Phase 3: Mobile Ledger UI (~25%)

**Files:**

- `web/src/admin/pages/TripConsumptionLedger.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/pages/GuestFolio.tsx`
- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [x] Add ledger route and manifest entry point.
- [x] Build mobile-first single-guest quick-add workflow.
- [x] Show catalog categories, stock state, quantity controls, and recent
      line feedback.
- [x] Generate one `client_request_id` per submit.
- [x] Display negative stock warnings without blocking the flow.
- [x] Keep checkout focused on review, tip, payment, close, and email.

### Phase 4: Verification and Docs (~15%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`

**Tasks:**

- [x] Update product docs to make live consumption in-scope.
- [x] Add tests for negative stock, idempotency, lazy-open, trip-start
      folio open, corrections, voids, lifecycle guards, and role scoping.
- [x] Run backend tests, vet, and frontend build.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/trips/{id}/ledger` | GET | Trip-level live consumption data. |
| `/api/admin/trips/{id}/ledger/lines` | POST | Add one catalog item to one guest folio. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines` | POST | Existing per-guest add, now stock-posting and idempotent. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines/{line_id}` | PATCH | Correct quantity with stock delta. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines/{line_id}` | DELETE | Hide/void line with stock reversal. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0018_realtime_consumption_ledger.sql` | Create | Schema for negative stock, line posting, voiding, and idempotency. |
| `internal/store/guest_folios.go` | Modify | Live line posting, lazy-open, voids, checkout no double decrement. |
| `internal/store/inventory.go` | Modify | Support negative stock outcomes for folio writes. |
| `internal/store/trip_lifecycle.go` | Modify | Open folios at trip start and adjust readiness. |
| `internal/httpapi/guest_folio_handlers.go` | Modify | Ledger endpoints and response/error mapping. |
| `internal/httpapi/httpapi.go` | Modify | Route ledger endpoints. |
| `web/src/admin/pages/TripConsumptionLedger.tsx` | Create | Mobile-first quick-add ledger UI. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Link active trips to ledger. |
| `web/src/admin/pages/GuestFolio.tsx` | Modify | Keep checkout aligned with new line semantics. |
| `web/src/admin/api.ts` | Modify | Ledger types and calls. |
| `web/src/main.tsx` | Modify | Ledger route. |
| `web/src/styles/app.css` | Modify | Mobile ledger layout and controls. |
| `docs/product/personas.md` | Modify | Confirm live consumption ownership. |
| `docs/product/organization-admin-user-stories.md` | Modify | Move live ledger from out-of-scope into current backlog. |

## Definition of Done

- [x] Trip start opens missing folios for non-revoked guests.
- [x] First line-add on an active trip lazy-opens the folio when needed.
- [x] Single-guest ledger add writes the folio line and stock movement in
      one transaction.
- [x] Negative stock is allowed and returned/displayed as a warning.
- [x] Quantity corrections post correct stock deltas.
- [x] Voiding hides the line from folio views and posts stock reversal.
- [x] Checkout close does not double-decrement stock.
- [x] Duplicate `client_request_id` returns the original line result.
- [x] Concurrent lazy-open retries create one folio and one line.
- [x] Completed/cancelled trips reject POST, PATCH, and DELETE line
      mutation.
- [x] Org Admin and assigned Cruise Director access works; unrelated
      Directors are blocked.
- [x] Frontend ledger layout is usable on mobile widths with no text or
      control overlap.
- [x] `go test ./...`, `go vet ./...`, and frontend build pass.

## Security Considerations

- All ledger reads and writes are scoped by organization.
- Cruise Director access requires assigned-trip authorization.
- Guest sessions cannot access staff ledger endpoints.
- Idempotency keys are scoped per trip guest and capped to 64
  characters.
- Audit metadata must not include raw registration payloads or large PII.

## Dependencies

- Sprint 015: guest folios, checkout, payment settings, and stock
  movement foundation.
- Sprint 017: audit foundation.
- Sprint 018: trip lifecycle and active/completed/cancelled guards.

## References

- `docs/sprints/SPRINT-015.md`
- `docs/sprints/SPRINT-017.md`
- `docs/sprints/SPRINT-018.md`
- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
