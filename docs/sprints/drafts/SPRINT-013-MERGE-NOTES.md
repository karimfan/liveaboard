# Sprint 013 Merge Notes

## Claude Draft Strengths

- Clear separation between catalog items and per-boat inventory.
- Broad, domain-specific default catalog across bar, soft drinks, dive
  services, gear rental, retail, spa, laundry, connectivity, fees, and
  gratuities.
- Captured the interview decisions: quote API in scope, USD canonical
  pricing, default catalog for new orgs, merchandise as inventory, and
  folio-driven future decrements.
- Good frontend workbench shape for `/admin/inventory` and boat-level
  inventory.

## Codex Draft Strengths

- Sharper schema constraints and tenant-isolation concerns.
- Better stock movement requirements: row locking, before/after
  quantities, negative-stock rejection, and concurrency tests.
- Better FX/quote precision: target currency exponent, persisted quote
  snapshots, and expired-rate behavior.
- Corrected RBAC: quote endpoint must be authenticated for future
  Cruise Directors, while catalog/inventory mutations remain admin-only.

## Valid Critiques Accepted

- Use composite ownership validation for boat/item/category stock
  operations.
- Reject negative stock in Sprint 013.
- Defer `capacity` stock mode; ship `none` and `counted`.
- Add stable template keys for default categories/items; do not
  overwrite operator edits when applying defaults.
- Store quote line snapshots when item lines are quoted.
- Include currency exponent in checkout quote responses and persistence.
- Make signup seeding the primary default-catalog path.
- Keep manual FX rate management admin-only and auditable.

## Critiques Rejected

- Removing tax/required-fee placeholders entirely. The final plan keeps
  them as stored metadata only, with all calculation behavior deferred.
- Deferring item-line quote support. Since the user asked for checkout
  quote API, line-based quoting is included in minimal form with price
  snapshots.

## Interview Refinements Applied

- Checkout quote API is in scope.
- Inventory supports manual admin adjustments and a reserved
  `folio_charge` movement path for Cruise Director guest folios.
- Merchandise includes t-shirts, hoodies, mugs, rash guards, sunscreen,
  logbooks, dry bags, and similar onboard shop items.
- New orgs start with default catalog data.
- USD is the canonical catalog price currency; org currency is display
  or default checkout preference.

## Final Decisions

- Sprint 013 ships catalog, inventory, stock movement, FX rate, and
  checkout quote foundations.
- Payment capture, receipt generation, tax calculation, guest checkout
  UI, and folio posting are out of scope.
- `stock_mode` values for this sprint are `none` and `counted`.
- Quote endpoint accepts either a source USD amount or item lines. Item
  lines snapshot item name, quantity, and USD cents used.
- Negative stock is rejected.
- Default catalog seed uses stable template keys and is invoked during
  signup.
