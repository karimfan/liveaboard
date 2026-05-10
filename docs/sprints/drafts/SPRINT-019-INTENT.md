# Sprint 019 Intent: Real-Time Consumption Ledger

## Seed

kplan Real-time consumption ledger second -- the daily work of the
cruise director and the biggest UX gap. The data model already supports
it: `guest_folios` is openable and `guest_folio_lines` already snapshot
prices at write time. What's missing is the operational shape:

- Folio opens at trip start, or on first line-add.
- Mobile-friendly add-line UI for one-handed bar work.
- Concurrency safety on line writes for multiple staff at meals.
- Stock decrement on each add, not deferred to close, so inventory
  reflects the trip in progress.

## Context

The product now has the needed foundations for live consumption entry:
catalog, per-boat inventory, one folio per guest/trip, offline checkout,
payment settings, audit, guest profiles, cabin assignments, and explicit
trip lifecycle. Sprint 015 implemented the checkout-oriented folio
experience, but it treats stock decrement as a close-time side effect.
That model works for end-of-trip settlement but fails onboard operations
because inventory remains stale during the trip.

Sprint 019 should turn the existing folio into the operational ledger
Cruise Directors use during the trip. Checkout remains the payment and
folio-email step, not the moment where consumption becomes real.

## Recent Sprint Context

- Sprint 015 added one end-of-trip folio per guest/trip, offline payment
  closure, folio email, payment settings, and stock decrement on close.
- Sprint 016 added mandatory cabin/berth assignment for trip guests.
- Sprint 017 added audit events and guest documents.
- Sprint 018 added explicit trip lifecycle. Active trips are the natural
  scope for live consumption; completed and cancelled trips are guarded.

## Relevant Codebase Areas

- `internal/store/guest_folios.go` owns folio open, line add/update/delete,
  close, and current close-time stock decrement.
- `internal/store/inventory.go` owns counted stock rows and stock
  movement semantics. It already has `folio_charge` and `folio_void`.
- `internal/store/trip_lifecycle.go` starts/completes trips and already
  reads folio status for readiness.
- `internal/httpapi/guest_folio_handlers.go` exposes per-guest folio
  endpoints and records staff audit events.
- `internal/httpapi/httpapi.go` registers folio and lifecycle routes.
- `web/src/admin/pages/GuestFolio.tsx` is the current checkout view. It
  is serviceable for settlement but not a fast mobile consumption UI.
- `web/src/admin/pages/TripManifest.tsx` is the likely navigation point
  for the trip-level ledger.
- `web/src/admin/api.ts` contains the admin client types and folio calls.
- `DESIGN.md` requires dense, utilitarian working surfaces with stable
  dimensions and mobile-safe controls.

## Constraints

- Must keep one folio per guest/trip.
- Must not process payments online or store POS confirmation data.
- Must preserve historical folio lines and stock movements.
- Must avoid double-decrementing inventory for counted items when folios
  are later closed.
- Must support corrections after a line was posted to inventory.
- Must keep strict organization scoping and assigned-trip Director
  scoping.
- Must keep completed and cancelled trips read-only for line mutation.
- Must not introduce offline/sync scope.
- Must follow the existing Go store/handler style and React admin chrome.

## Success Criteria

- Starting a trip opens folios for active guests, with lazy open on first
  line-add as a fallback for late-added guests.
- Cruise Directors can record consumption from a mobile-friendly trip
  ledger surface without drilling into each guest checkout page.
- Counted catalog items decrement boat inventory inside the same
  transaction as the folio line write.
- Quantity edits and deletions on open folios adjust or reverse inventory
  with matching stock movements.
- Closing a folio no longer double-decrements stock; it only settles
  payment, snapshots fees/FX, marks paid, and emails the folio.
- Concurrent line writes serialize correctly on the affected folio and
  inventory rows.
- Duplicate mobile submits are idempotent or otherwise safely rejected.
- Audit events remain safe and useful for folio and inventory activity.

## Open Questions

Resolved by product interview:

1. No batch add in Sprint 019. Build single-guest quick add only.
2. Allow negative counted stock with a warning because some inventory is
   operationally infinite or loosely tracked. Non-stocked/infinite items
   should continue to use `stock_mode = none`.
3. Keep live consumption entry scoped to Org Admin and assigned Cruise
   Director. Do not add a new onboard staff/bar role.
4. Hide voided/corrected lines from the folio view while preserving
   audit and stock movement history underneath.
