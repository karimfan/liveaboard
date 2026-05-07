# Sprint 013 Claude Draft Critique

Claude's draft is directionally strong and covers the interview
decisions, but it needs sharper implementation boundaries in several
areas.

## Accepted Critiques

- The draft duplicates `organization_id` on tenant tables, but should
  specify composite ownership constraints or explicit store-layer
  ownership validation so `boat_inventory_items` cannot link another
  org's boat to this org's item.
- The inventory schema needs a clear negative-stock policy. Sprint 013
  should reject movements that make `quantity_on_hand < 0`.
- Stock movement concurrency should be explicit: use a transaction,
  row lock, before/after quantities computed inside the transaction,
  and tests for competing adjustments.
- `stock_mode = capacity` is underdefined. Capacity should be
  deferred; ship `none` and `counted` first.
- Movement `source_type`/`source_id` should document expected values
  such as `manual_adjustment`, `folio_line`, `voided_folio_line`, and
  `import`.
- Default catalog seeding should happen at signup. The admin endpoint
  is only an idempotent repair/apply-missing-defaults command.
- Default seeding should not key only by item name. Use stable
  template keys, or at minimum category + item matching, and never
  overwrite operator-edited prices.
- The default taxonomy should avoid duplicating Wine under both Bar
  and Wine unless the final plan explicitly chooses one.
- Money conversion needs currency exponent metadata. Quotes should
  store `target_amount_minor` and `currency_exponent`.
- Quote request/response shape needs line snapshots if item-line
  quoting is supported.
- Manual FX rates need validation, auditability, and admin-only access.
- `exchange_rates` needs latest-rate lookup indexes and behavior for
  overlapping rates.
- The quote endpoint should be authenticated and available to future
  Cruise Directors; admin-only restrictions apply to catalog/inventory
  mutations.
- Frontend scope is large. The final sprint should emphasize backend
  correctness first and keep quote UI as an admin/dev preview rather
  than a guest checkout surface.
- Tests should cover signup default seeding, idempotent default repair,
  cross-org rejection, negative-stock rejection, concurrent stock
  adjustments, expired FX rates, zero-decimal rounding, and role
  boundaries.

## Rejected Or Deferred Critiques

- The draft's `is_taxable` and `is_required_fee` fields can stay as
  placeholders because they avoid another schema migration before
  ledger/checkout, but no tax or required-fee calculation rules should
  ship in this sprint.
- Item-line quote support is useful, but the final plan should keep it
  minimal: snapshot line item id, name, quantity, and USD cents used.
  Full cart lifecycle remains deferred.

