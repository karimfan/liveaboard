# Sprint 020 Codex Draft — Claude Critique

Reviewed against `SPRINT-020-INTENT.md`, `docs/sprints/README.md`, the
sprint template, recent SPRINT-019 conventions, and the actual code in
`internal/store`, `internal/store/migrations/`, and `internal/httpapi/`.

The draft is on the right track. It captures intent faithfully, picks a
reasonable storage shape, and adds a `price_source` snapshot that the
intent does not explicitly demand but that pays for itself in audit and
UX. The main weaknesses are: it diverges from the sprint document
template in `docs/sprints/README.md`, it leaves the line-snapshot data
model under-specified at the boundaries that matter (pricing-stamp on
crew-tip vs catalog lines, behavior when the override is archived
mid-line-write, etc.), it duplicates and partially conflicts with the
existing `EnsurePaymentSettings` / `UpdatePaymentSettings` codepaths,
and its DoD line about "forward-only archive/nullable" migrations is
inconsistent with the project's existing reversible `+goose Up/Down`
convention.

Below: strengths to preserve, then concerns by category.

---

## Strengths Worth Preserving

1. **Effective price resolver returns `(cents, source)`.** Returning the
   resolution source from `EffectiveCatalogItemPrice` is the right
   shape. UI surfaces, audit metadata, and tests all need the source,
   not just the number.
2. **Snapshot `price_source` and `price_override_id` on the line.** This
   matches how the codebase already snapshots `item_name`,
   `unit_price_usd_cents`, and `stock_mode` at insert time and preserves
   "historical safety" exactly the way Sprint 019 preserved it for
   stock posting. `ON DELETE SET NULL` on `price_override_id` is the
   right call.
3. **One override row per scope (single-non-null FK with a CHECK).**
   Modeling boat and trip overrides in the same table with a partial
   unique index on `archived_at IS NULL` reuses the catalog/inventory
   archive-instead-of-delete pattern already in use.
4. **Resolution order is explicit and matches intent**: trip > boat >
   base.
5. **Defaults, USD invariance, and EUR removability** are stated
   correctly.
6. **No close-time repricing** is called out, which is the correct
   reading of Sprint 019's "snapshot at line write" rule.
7. **No external FX call.** The draft re-affirms the existing
   stored-rates posture.

---

## Major Concerns

### 1. Document does not follow the project's sprint template

`docs/sprints/README.md` requires sections **Architecture**,
**Implementation Plan** (phased with percentages and `[ ]` task
checkboxes), **API Endpoints**, **Files Summary** (table), **Definition
of Done** (checkboxes), **Security Considerations**, **Dependencies**,
and **References**. The draft is missing:

- A phased Implementation Plan with checkbox tasks. Sprint 019 used
  four phases at 40/20/25/15. Sprint 020 should follow that shape.
- A **Files Summary** table.
- A **Security Considerations** section. (Pricing config is admin-only,
  org-scoped; the section is short but mandatory.)
- A **Dependencies** section. (Sprint 015 payment settings, Sprint 019
  folio-line snapshot semantics.)
- A **References** section (sprint docs and product backlog).
- The **Definition of Done** is prose-shaped, not the required
  checkbox list.

Final merge should restructure the doc to match Sprint 019's layout.

### 2. Migration style does not match the codebase

Every migration in `internal/store/migrations/` is `+goose Up /
+goose Down` with reversible DDL (see `0018_realtime_consumption_ledger.sql`,
which writes a non-trivial `Down`). The draft's DoD says "Migrations
are reversible by forward-only archive/nullable strategy" — that phrase
is contradictory, and the rest of the project does not follow a
forward-only model. The final sprint should:

- Specify a real `+goose Up` block: create
  `catalog_price_overrides`, add `price_source` and
  `price_override_id` to `guest_folio_lines`, and add a `CHECK`
  constraint for `price_source IN ('base','boat_override','trip_override','tip')`.
- Specify a `+goose Down` block that drops the new columns, drops the
  table, and is safe to run on a DB that already has overrides.
- For currency defaults: do **not** add a separate normalization
  migration. The existing `EnsurePaymentSettings` /
  `UpdatePaymentSettings` codepath is where USD-set logic lives today
  (see `internal/store/payment_settings.go:69` and `:121`). Either:
  (a) extend the SQL `DEFAULT` to `ARRAY['USD','EUR']` for net-new
  rows and add a one-shot `UPDATE … WHERE NOT 'EUR' = ANY(supported_currencies)`
  in the migration, or (b) backfill on Ensure. Decide explicitly. The
  draft says both ("backfilled to include EUR once" and lets normalize
  add EUR), which risks double-write or drift.

### 3. The `price_source` enum is incomplete for `crew_tip` and tip-line snapshots

`guest_folio_lines` carries both `catalog_item` and `crew_tip` line
types (see `internal/store/guest_folios.go:22`). If the new column is
`NOT NULL DEFAULT 'base'`, then crew-tip lines will be tagged `base`,
which is misleading. Pick one:

- Allow `price_source` to be `NULL` for non-catalog lines, OR
- Add a `'tip'` value to the allowed set, OR
- Make the column `nullable` and only set it for `line_type =
  'catalog_item'`.

The draft does not address this; the final merge should.

### 4. `EUR removable but USD always included` conflicts with current normalization

`normalizePaymentSettings` in `internal/store/payment_settings.go:121`
*forces* `USD` into every supported set. Good. But there is no symmetric
"add EUR if missing" rule. If the migration backfills EUR but a
subsequent `UpdatePaymentSettings` call from the UI omits EUR, EUR
disappears (which is the documented behavior — admins can remove it).
However, the *defaults flow* must not silently re-add EUR on the next
read. The draft says "default-enabled but removable"; the final sprint
should:

- Add EUR only at *organization creation* (Ensure path) and during the
  one-shot migration backfill.
- **Not** add EUR back in `paymentRateReadiness` or
  `normalizePaymentSettings`.
- Add a test that asserts: admin removes EUR → settings persist
  without EUR → re-reading does not re-add EUR.

This invariant is implied but not explicit; without a test it will
regress.

### 5. The override resolver is not yet specified at edge cases

`EffectiveCatalogItemPrice(ctx, orgID, tripID, itemID)` is named, but
the draft does not specify:

- What happens when `tripID` is `uuid.Nil` (e.g. a catalog list page
  for a boat with no trip). If the resolver is called only inside
  `AddGuestFolioLine`, this may be fine, but the draft also wires
  effective-price metadata into catalog responses for trip context —
  needs a separate `EffectiveCatalogItemPriceForBoat(ctx, orgID,
  boatID, itemID)` overload, or a single function that accepts an
  optional trip and resolves the boat from it.
- Whether the resolver runs inside the line-add transaction (it
  must — otherwise an admin can race an `archive` against a line
  insert and the line writes a price after the override row is
  archived). The draft says it should be called in the same place as
  `AddGuestFolioLine`'s catalog lookup, but does not state the locking
  contract.
- The Sprint 019 canonical lock order
  (`trips → trip_guests → guest_folios → guest_folio_lines →
  boat_inventory_items`) needs an explicit insertion point for
  `catalog_price_overrides`. Suggested: after `boat_inventory_items`
  with `FOR SHARE` (the resolver only reads), or inside a single
  `SELECT … FOR SHARE` snapshot earlier. State this explicitly.
- What happens if a line is added with `client_request_id` and the
  retry happens after the override changes. The Sprint 019 idempotency
  contract says "duplicate retry returns the same logical result,
  including the original line id" — that means the retry must return
  the original snapshot, *not* re-resolve. The draft does not address
  this and the test list does not exercise it.

### 6. Cruise Director write-path implications are unclear

Sprint 019 lets *assigned* Cruise Directors add lines via
`/api/admin/trips/{id}/ledger/lines` and the per-guest folio endpoints.
The draft says only Admin can mutate **pricing config** — that is
correct — but it does not state whether the **read** of effective
prices flows through the existing CD-authorized ledger GET, or whether
it requires a separate handler. It should be explicit:

- `GET /api/admin/trips/{id}/ledger` should return effective prices
  per catalog item for that trip (CD or Admin allowed).
- The new `/api/admin/pricing/...` endpoints are Admin-only and
  org-scoped.

### 7. URL design diverges from existing conventions

The existing admin tree (see `internal/httpapi/httpapi.go:110`) groups
resources by parent (`/trips/{id}/...`, `/boats/{id}/inventory/{id}`,
`/catalog/items`, etc.). The draft proposes a flat
`/api/admin/pricing/boat-overrides` and `/api/admin/pricing/trip-overrides`.
That is reasonable, but two issues:

- A `PUT` that creates-or-updates a single override needs a stable
  identifier. Either accept `(boat_id, catalog_item_id)` in the body
  and do server-side upsert, or use a path
  `/api/admin/pricing/boats/{boat_id}/catalog-items/{item_id}`. The
  body-keyed form is harder to test and audit-log — the
  parent-scoped path is more consistent with `PUT
  /boats/{id}/inventory/{item_id}` already in the routes.
- `DELETE /api/admin/pricing/overrides/{id}` is fine, but the draft
  says "archive" while the verb is `DELETE`. Match Sprint 017's
  pattern: handler archives, returns the archived row's metadata in
  the response, audit logs `pricing.override_archived`.

### 8. Boat-pricing-without-trip lookup is missing from the resolver story

The intent calls out "boat override wins over base catalog item price"
even before a trip exists in scope (e.g. when an admin edits the
catalog and wants to see the effective price on a specific boat). The
draft surfaces this only inside the per-trip ledger context. The final
sprint should either:

- Explicitly defer "boat-only effective price preview" out of scope
  (with a one-line note), or
- Add a tiny resolver helper for boat-context preview that returns
  `(base, override?, source)` without requiring a trip.

### 9. Frontend section is too vague to lock in scope

Compared to Sprint 019's frontend section (which named exact files,
described specific UI elements, and listed mobile-first constraints),
the Codex draft says "add a pricing area under the admin
organization/catalog surface" without naming files or routes. The
final sprint should specify:

- File path (e.g. `web/src/admin/pages/PricingOverrides.tsx`).
- Where it links from (Catalog page tab? Sidebar nav?). `Catalog.tsx`
  does not appear to exist in `web/src/admin/pages/` — the catalog UI
  is currently inside `Organization.tsx` or another surface; verify.
- Concrete table columns: catalog item, scope (boat name or trip
  itinerary+date), USD price, base USD price, delta, last edited,
  archive button.
- Whether the trip-override editor lives on `TripManifest.tsx`,
  `Trips.tsx`, or its own page. Recommend a tab on the existing
  `TripManifest.tsx` for trip overrides (so Admins manage trip
  pricing where they already manage the trip) and a section on the
  boat detail page (`BoatDetail.tsx`) for boat overrides.
- Whether `OrganizationPayments.tsx` needs any new copy explaining
  EUR-default ("EUR is enabled by default; remove it if you do not
  accept Euros.").
- The `web/src/admin/api.ts` and `web/src/main.tsx` route additions
  needed (Sprint 019 listed them; Sprint 020 should too).

### 10. Test list is missing several cases the intent demands

Add to the test plan:

- Override **archived after** a line is written → existing line totals
  unchanged, line still references the archived override id.
- Override **edited** (price change) after a line is written → existing
  line totals unchanged.
- Catalog item **deactivated** while overrides exist → confirm
  override resolver still returns the right source (or 404s
  consistently with current `IsActive || ArchivedAt != nil` checks at
  `internal/store/guest_folios.go:358`).
- Trip override `boat_id IS NULL` and trip override `boat_id NOT NULL`
  resolution when the trip's boat changes. (Trip's boat_id changes
  are uncommon but possible at planning time.)
- Idempotent retry of `AddGuestFolioLine` after the override changes
  returns the **original** snapshot, not a re-resolved price.
- Org isolation: org A's overrides cannot affect org B's resolution
  even if a (catalog_item_id, boat_id) row collides somehow (the
  org-scoped FK should make this structurally impossible — assert it).
- `EnsurePaymentSettings` for a brand-new org returns
  `['EUR','USD']` (sorted), not just `['USD']`.
- Removing EUR via `UpdatePaymentSettings` persists and survives
  re-read.

### 11. The "EUR readiness" warning model is under-specified

The draft says "missing-rate warnings" but does not say where they
surface or how they are returned. The existing
`paymentRateReadiness` already returns
`[]PaymentCurrencyRateStatus{Currency, Ready, Rate}` from
`PaymentSettings()`. The frontend already reads it. The new sprint
should clarify that **no new** warning surface is being added —
EUR readiness uses the existing field. Any new wording belongs in the
frontend layer.

---

## Missing Implementation Details

- **Audit events.** Sprint 017 + 019 standardized audit metadata. The
  draft mentions audit only obliquely. Add explicit events:
  `pricing.boat_override_created`, `pricing.boat_override_updated`,
  `pricing.trip_override_created`, `pricing.trip_override_updated`,
  `pricing.override_archived`. Metadata: catalog_item_id, scope id,
  before/after price cents. Not the actor name or PII.
- **Default-currency invariant for orgs that already have a non-USD
  default.** If `organizations.currency` is `'EUR'`, `EnsurePaymentSettings`
  already initializes `default_currency = 'EUR'` and
  `supported_currencies = ['EUR']`. The migration must not drop EUR
  back to USD; it must add USD if missing **and** EUR if missing,
  preserving whatever the default was. State this.
- **Concurrency on override upsert.** Two admins editing the same
  (org, item, boat) override simultaneously — the partial unique index
  prevents two active rows, but the upsert path needs to tolerate the
  race. Spell out the SQL: `INSERT … ON CONFLICT (org, item, boat)
  WHERE archived_at IS NULL DO UPDATE SET price_usd_cents = …,
  updated_by_user_id = …, updated_at = now()`. Note: PostgreSQL only
  supports `ON CONFLICT` with full unique indexes, not partial — so
  the upsert may need to be a `SELECT … FOR UPDATE` + `UPDATE` /
  `INSERT` inside a transaction. Verify before committing to the
  partial-index approach, or accept a full unique index `(org, item,
  boat, archived_at)` with `archived_at` becoming NOT NULL DEFAULT
  '0001-01-01' for the active case (uglier but PG-supported).
- **Files Summary** must list (Create/Modify) entries for the
  migration, the new `internal/store/pricing_overrides.go` (new file?
  or extend `catalog.go`?), modified `guest_folios.go`, modified
  `payment_settings.go`, modified `httpapi.go`, new
  `pricing_handlers.go`, modified `catalog_handlers.go` (to surface
  effective price), and the frontend list above.
- **Endpoint that returns "catalog items for trip" with effective
  price.** The draft hints at this in the response example but does
  not name an endpoint. Decide: extend `GET /api/admin/catalog/items`
  to accept an optional `trip_id=` (or `boat_id=`) query and include
  effective fields, or add `GET /api/admin/trips/{id}/catalog`. The
  ledger GET `(/api/admin/trips/{id}/ledger)` already returns catalog
  rows scoped to the trip boat — it is the cheapest place to add the
  effective price metadata.

---

## Suggested Changes (Concrete)

1. **Restructure** the doc to match `docs/sprints/README.md`'s template,
   modeled on Sprint 019: phased Implementation Plan with checkboxes,
   Files Summary table, Security Considerations, Dependencies,
   References.
2. **Migration**: write `0019_pricing_overrides.sql` with `+goose Up`
   and `+goose Down` blocks; use the same statement style as
   `0018_realtime_consumption_ledger.sql`. Include EUR backfill in the
   same migration with an `UPDATE … WHERE NOT ('EUR' = ANY(supported_currencies))`.
3. **Verify** that PostgreSQL `ON CONFLICT` works with the partial
   unique index. If not, switch to transactional select-then-update.
4. **Define** `price_source` allowed values explicitly:
   `('base','boat_override','trip_override')`. Make the column nullable
   for crew-tip lines (or add `'tip'` and require it for crew-tip
   lines — pick one and state it).
5. **Add a CD-allowed read** of effective prices via the existing
   `/api/admin/trips/{id}/ledger` GET. The new override mutation
   endpoints stay Admin-only.
6. **Drop or revise** the DoD line "Migrations are reversible by
   forward-only archive/nullable strategy" — it is internally
   contradictory and not how this repo manages migrations.
7. **Move "EUR is removable"** from a prose claim into an explicit
   normalization invariant: `normalizePaymentSettings` keeps USD only;
   `EnsurePaymentSettings` adds EUR only on first creation; migration
   backfills EUR once. Add the test that proves removability sticks.
8. **Specify** the route(s) for catalog effective-price exposure (extend
   ledger GET; do not add a new handler).
9. **Name files**, link points, and table columns for the frontend
   pricing surface.
10. **Expand** test cases per §10 above.

---

## Risks the Final Merge Should Address

- **Data integrity race**: an override archived between resolver-read
  and line-insert. Mitigation: resolver runs in the same tx as the
  insert, with `FOR SHARE` on the override row.
- **Migration-time double-add of EUR**: backfill runs, then `Ensure`
  re-runs and does nothing because `ON CONFLICT DO NOTHING` — fine.
  But document that the migration is idempotent if re-applied.
- **Removability-vs-default tension**: EUR appears default; admin
  removes it; a future-Claude "fixes" `Ensure` to always include EUR
  and silently re-adds it. Mitigation: comment in
  `payment_settings.go` explicitly stating "EUR is added only on first
  creation; removability is intentional" + the test.
- **`organizations.currency` already EUR**: this org should still get
  USD added as supported (currently does), and EUR should remain. Make
  sure the migration's WHERE clause does not strip the EUR that is
  already there.
- **Ledger response payload size**: adding effective-price fields per
  catalog row to the ledger GET is cheap (one extra select per item).
  Validate the bounded-recent-lines cap in Sprint 019 still holds.
- **Audit volume**: pricing overrides change rarely; logging every
  upsert is fine. Do not log the override id in line-write audit events
  unless it is already standard there — keep it minimal.

---

## Parts of the Codex Draft to Reject or Simplify

1. **Reject the "forward-only archive/nullable strategy" DoD line**.
   Replace with the standard `+goose Up/Down` reversible migration
   convention used everywhere else in the repo.
2. **Simplify the Frontend section** by deferring the precise UI
   placement to one of two places: tab in `BoatDetail.tsx` for boat
   overrides, tab in `TripManifest.tsx` for trip overrides — and not a
   separate top-level page. This avoids a third nav surface for an
   admin-rare config flow and matches the existing pattern where
   inventory lives under boats.
3. **Simplify endpoint surface** by collapsing
   `/api/admin/pricing/boat-overrides` and
   `/api/admin/pricing/trip-overrides` into a single
   `PUT /api/admin/pricing/overrides` that accepts a body with either
   `boat_id` or `trip_id` and the server's CHECK validates exclusivity.
   It mirrors the underlying table shape and reduces handler count from
   three to two (PUT + DELETE; GET stays). Either choice is defensible
   — pick one explicitly.
4. **Drop the implicit "checkout repricing" anxiety** language
   (`CloseGuestFolio should continue to calculate settlement totals
   from snapshotted folio line totals…`). That is already true after
   Sprint 019; restating it adds noise. Replace with a short
   "Sprint 019 close-time semantics unchanged" line.
5. **Reject** the implicit assumption that the resolver only ever runs
   in trip context. Add an explicit trip-optional or boat-only variant
   if the catalog UI needs it; otherwise state plainly that
   non-trip-context price preview is out of scope and the catalog page
   shows base only.

---

## Bottom Line

The draft is a credible sprint plan that captures the intent. To merge
cleanly with project conventions it needs: the standard sprint-doc
shape, a real reversible `+goose` migration, an explicit and tested
EUR-removability invariant, an explicit transactional contract for the
effective-price resolver, and a concrete frontend surface. The
`price_source` snapshot idea is a good extension — keep it, but
specify its handling for crew-tip lines and idempotent retries.
