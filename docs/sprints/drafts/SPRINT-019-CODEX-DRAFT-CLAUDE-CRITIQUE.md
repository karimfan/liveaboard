# Sprint 019 Claude Critique of Codex Draft

## Actionable Concerns

1. **Negative stock requires schema changes.** The existing inventory
   schema checks `quantity_on_hand >= 0` and movement before/after
   quantities `>= 0`. Sprint 019 must remove those checks and remove the
   store branch that rejects negative quantity after adjustment.

2. **Movement signs must be explicit.** Quantity increase should write
   `folio_charge` with negative delta. Quantity decrease or void should
   write `folio_void` with positive delta. The sprint doc should include
   this table so implementation and review use the same convention.

3. **Do not add `stock_movement_id` to folio lines.** A line can accrue
   several movements over time: initial charge, quantity edits, and void.
   `stock_movements.source_type/source_id` should remain the historical
   source of truth. Use `stock_posted_at` plus movement rows, not a single
   movement foreign key.

4. **Idempotency should be scoped to `trip_guest_id`.** A unique
   `(folio_id, client_request_id)` does not cover retries that race before
   lazy-open creates the folio. Use `(trip_guest_id, client_request_id)`
   on folio lines, or an equivalent transactional lock strategy.

5. **Crew-tip uniqueness must account for voiding.** The current unique
   tip index should be recreated as active-only, where
   `line_type = 'crew_tip' AND voided_at IS NULL`.

6. **Lifecycle scope must be explicit.** Live consumption writes should
   target active trips only. Existing checkout close should remain allowed
   for active trips; completed/cancelled trips stay guarded.

7. **Start readiness should not warn for folios that start will create.**
   The current readiness logic warns on missing folios. Sprint 019 should
   either remove missing folio from start warnings or recompute after
   pre-open.

8. **Lazy-created inventory rows must be intentional.** If an item has no
   boat inventory row, line-add can create one at zero and then take it
   negative. That matches the chosen product rule, but the doc should
   state it.

9. **Ledger GET response should be bounded.** Cap recent lines and scope
   inventory to the trip boat. Avoid returning unbounded folio history.

10. **Lock order must be canonical.** Use the same transaction lock order
    across add/update/delete/close: trip, trip guest, folio, line if
    applicable, inventory row.

## Test Additions

- Concurrent lazy-open retries with the same `client_request_id` create
  one folio and one line.
- Retried POST returns the same line id.
- Voiding a crew tip then adding a new one succeeds.
- Negative stock succeeds, persists below-zero inventory, and returns a
  warning.
- `StartTrip` folio pre-open is idempotent.
- Completed/cancelled trips reject POST, PATCH, and DELETE line mutation.
- Concurrent update/delete of the same line has one winner and no phantom
  stock movement.
- Assigned Cruise Director scoping blocks unrelated trips.
- Cross-org idempotency key collisions are allowed.
- Delete reversal uses the current posted quantity, not the original
  line quantity.
- Reversal uses the line's snapshotted stock mode, not the current
  catalog item stock mode.
