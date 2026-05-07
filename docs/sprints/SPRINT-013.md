# Sprint 013: Catalog, Inventory, and Checkout Currency Foundation

## Overview

This sprint turns the current Inventory placeholder into the foundation
for onboard sales: an org-owned catalog, per-boat stock tracking,
auditable inventory movements, and checkout currency quoting. Catalog
prices are canonical in USD cents. `organizations.currency` remains
the operator's display/default checkout preference, not the source of
catalog price truth.

The scope covers both stock-tracked goods and non-stocked services.
Beer cans, beer bottles, wine glasses, wine bottles, t-shirts,
hoodies, mugs, sunscreen, logbooks, and dry bags can deplete per-boat
inventory. Massage, laundry, Nitrox packages, WiFi, fees, gratuities,
and custom services can live in the same catalog without stock rows.
Inventory mutations support manual admin adjustments now and reserve a
shared movement path for automatic decrements when Cruise Directors
later add stock-tracked items to guest folios.

Checkout payment, receipts, taxes, guest checkout UI, and folio posting
are out of scope. The quote API is in scope: it quotes a USD amount or
item lines into a target currency using a persisted exchange-rate
snapshot and deterministic rounding.

## Use Cases

1. **Start with a default catalog.** New organization signup seeds a
   liveaboard catalog with Bar, Soft Drinks, Dive Services, Gear
   Rental, Retail, Spa, Laundry, Connectivity, Fees, Gratuities, and
   Miscellaneous categories.
2. **Manage catalog items.** Org Admin creates, edits, deactivates,
   reactivates, and archives sellable items without losing historical
   identity.
3. **Represent units as SKUs.** `Beer - Can`, `Beer - Bottle`,
   `House Wine - Glass`, and `House Wine - Bottle` are separate items
   because their prices and depletion units differ.
4. **Track merchandise.** T-shirts, hoodies, mugs, rash guards,
   reef-safe sunscreen, logbooks, and dry bags are stock-tracked retail
   SKUs per boat.
5. **Add services and fees.** Massage, laundry, Nitrox week, marine
   park fee, airport transfer, corkage, WiFi pass, and gratuity can be
   sold later without stock depletion unless explicitly tracked.
6. **Manage boat stock.** Org Admin sets on-hand quantity, reorder
   level, par level, and notes for stock-tracked items on each boat.
7. **Adjust inventory manually.** Admin records restock, breakage,
   spoilage, cycle count, correction, and internal-use adjustments with
   actor, note, and before/after quantities.
8. **Prepare for folio decrements.** Future Cruise Director folio
   posting calls the same movement service with `movement_type =
   folio_charge`; Sprint 013 reserves the contract and tests the shared
   service path.
9. **Quote checkout currency.** Backend returns a persisted quote
   converting USD totals or item lines to the guest checkout currency
   with target minor units, currency exponent, rate metadata,
   expiration, and line price snapshots when lines are provided.
10. **Surface stock risk.** Admin inventory views show low-stock and
    out-of-stock status for every boat.

## Architecture

### Core Rules

- Catalog prices are stored as USD minor units:
  `price_usd_cents bigint NOT NULL CHECK (price_usd_cents >= 0)`.
- `organizations.currency` is retained as display/default checkout
  preference only.
- Inventory quantities are integers. Fractional pours are separate
  SKUs, not decimal stock.
- `stock_mode` values in this sprint are `none` and `counted`.
  Capacity/reservation semantics are deferred.
- Every tenant-owned table includes `organization_id`.
- Stock movement is append-only; current quantity updates in the same
  transaction.
- Negative stock is rejected.
- Checkout quotes persist the rate snapshot and line price snapshots
  used so checkout math is reproducible.

### Schema

```sql
catalog_categories (
  id uuid primary key,
  organization_id uuid not null references organizations(id) on delete cascade,
  template_key text null,
  name text not null,
  sort_order int not null default 0,
  is_default_seed boolean not null default false,
  archived_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)

catalog_items (
  id uuid primary key,
  organization_id uuid not null references organizations(id) on delete cascade,
  category_id uuid not null references catalog_categories(id),
  template_key text null,
  name text not null,
  description text null,
  unit text not null,
  charge_type text not null,
  stock_mode text not null check (stock_mode in ('none','counted')),
  price_usd_cents bigint not null check (price_usd_cents >= 0),
  is_taxable boolean not null default false,
  is_required_fee boolean not null default false,
  is_active boolean not null default true,
  archived_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)

boat_inventory_items (
  id uuid primary key,
  organization_id uuid not null references organizations(id) on delete cascade,
  boat_id uuid not null references boats(id) on delete cascade,
  catalog_item_id uuid not null references catalog_items(id),
  quantity_on_hand int not null default 0 check (quantity_on_hand >= 0),
  quantity_reserved int not null default 0 check (quantity_reserved >= 0),
  reorder_level int null check (reorder_level is null or reorder_level >= 0),
  par_level int null check (par_level is null or par_level >= 0),
  last_counted_at timestamptz null,
  notes text null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (organization_id, boat_id, catalog_item_id)
)

stock_movements (
  id uuid primary key,
  organization_id uuid not null references organizations(id) on delete cascade,
  boat_id uuid not null references boats(id) on delete cascade,
  catalog_item_id uuid not null references catalog_items(id),
  actor_user_id uuid null references users(id) on delete set null,
  movement_type text not null,
  delta_quantity int not null,
  quantity_before int not null check (quantity_before >= 0),
  quantity_after int not null check (quantity_after >= 0),
  source_type text null,
  source_id uuid null,
  note text null,
  created_at timestamptz not null default now()
)

exchange_rates (
  id uuid primary key,
  provider text not null,
  base_currency char(3) not null,
  quote_currency char(3) not null,
  rate_numerator bigint not null check (rate_numerator > 0),
  rate_denominator bigint not null check (rate_denominator > 0),
  as_of timestamptz not null,
  fetched_at timestamptz not null default now(),
  expires_at timestamptz not null
)

checkout_quotes (
  id uuid primary key,
  organization_id uuid not null references organizations(id) on delete cascade,
  requested_by uuid null references users(id) on delete set null,
  source_currency char(3) not null default 'USD',
  target_currency char(3) not null,
  source_amount_cents bigint not null check (source_amount_cents >= 0),
  target_amount_minor bigint not null check (target_amount_minor >= 0),
  currency_exponent int not null check (currency_exponent >= 0),
  rate_provider text not null,
  rate_numerator bigint not null,
  rate_denominator bigint not null,
  rate_as_of timestamptz not null,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
)

checkout_quote_lines (
  id uuid primary key,
  quote_id uuid not null references checkout_quotes(id) on delete cascade,
  catalog_item_id uuid null references catalog_items(id),
  item_name text not null,
  quantity int not null check (quantity > 0),
  unit_price_usd_cents bigint not null check (unit_price_usd_cents >= 0),
  line_total_usd_cents bigint not null check (line_total_usd_cents >= 0),
  sort_order int not null default 0
)
```

Migration/store work must add partial unique indexes for active,
non-archived category and item names per org; template-key uniqueness
per org for default repair; latest-rate lookup indexes; and either
composite DB constraints or explicit transactional store checks that
prove category, item, and boat rows belong to the caller's org.

### Default Catalog

Default categories/items are copied into each new organization during
signup. They are not global mutable rows. A repair endpoint can apply
missing defaults later, keyed by stable `template_key`, without
overwriting operator-edited prices or reactivating intentionally
deactivated items.

Suggested defaults:

| Category | Item | Unit | USD | Stock |
|---|---|---:|---:|---|
| Bar | Beer - Can | can | 6.00 | counted |
| Bar | Beer - Bottle | bottle | 7.00 | counted |
| Bar | House Wine - Glass | glass | 8.00 | none |
| Bar | House Wine - Bottle | bottle | 38.00 | counted |
| Bar | Premium Wine - Bottle | bottle | 65.00 | counted |
| Bar | Cocktail | each | 12.00 | none |
| Bar | Corkage - Wine Bottle | bottle | 20.00 | none |
| Soft Drinks | Soft Drink - Can | can | 3.00 | counted |
| Soft Drinks | Fresh Juice | glass | 5.00 | none |
| Dive Services | Nitrox - Fill | fill | 10.00 | none |
| Dive Services | Nitrox - Day | day | 20.00 | none |
| Dive Services | Nitrox - Week | week | 150.00 | none |
| Dive Services | Extra Dive | each | 45.00 | none |
| Gear Rental | Full Rental Kit | trip | 175.00 | none |
| Gear Rental | BCD Rental | trip | 60.00 | counted |
| Gear Rental | Regulator Rental | trip | 60.00 | counted |
| Gear Rental | Dive Computer Rental | trip | 60.00 | counted |
| Gear Rental | Wetsuit Rental | trip | 50.00 | counted |
| Gear Rental | Night Light Rental | night | 8.00 | counted |
| Retail | T-Shirt | each | 25.00 | counted |
| Retail | Hoodie | each | 55.00 | counted |
| Retail | Mug | each | 18.00 | counted |
| Retail | Rash Guard | each | 45.00 | counted |
| Retail | Reef-Safe Sunscreen | each | 18.00 | counted |
| Retail | Dry Bag | each | 30.00 | counted |
| Spa | Massage - 30 Minutes | session | 45.00 | none |
| Spa | Massage - 60 Minutes | session | 85.00 | none |
| Laundry | Laundry - Item | item | 5.00 | none |
| Laundry | Laundry - Bag | bag | 25.00 | none |
| Connectivity | WiFi - Week Code | week | 30.00 | none |
| Fees | Marine Park Fee | person | 150.00 | none |
| Fees | Fuel Surcharge | person | 150.00 | none |
| Fees | Airport Transfer | person | 25.00 | none |
| Gratuities | Crew Gratuity | person | 150.00 | none |
| Miscellaneous | Custom Charge | each | 0.00 | none |

### Stock Movement Contract

Movement types:

- `initial_count`
- `restock`
- `correction`
- `breakage`
- `spoilage`
- `internal_use`
- `folio_charge`
- `folio_void`

Manual admin adjustments use `source_type = 'manual_adjustment'`.
Future folio rows use `source_type = 'folio_line'` or
`'voided_folio_line'`. The service computes before/after quantities
inside a transaction with a row lock and rejects negative results.

### Checkout Quote Contract

`POST /api/checkout/quote` accepts either:

```json
{
  "target_currency": "EUR",
  "source_amount_cents": 12345
}
```

or:

```json
{
  "target_currency": "EUR",
  "lines": [
    { "catalog_item_id": "uuid", "quantity": 2 },
    { "catalog_item_id": "uuid", "quantity": 1 }
  ]
}
```

Line quotes snapshot item id, item name, quantity, unit USD cents, and
line total USD cents. The quote response includes `quote_id`,
`source_amount_cents`, `target_amount_minor`, `target_currency`,
`currency_exponent`, `rate_provider`, `rate_as_of`, `expires_at`, and
line snapshots when applicable. USD to USD is identity conversion.
Other currencies use the latest non-expired USD rate snapshot. Missing
or expired rates return a clear validation error.

## Implementation Plan

### Phase 1: Schema, Defaults, and Store Layer (~35%)

**Files:**
- `internal/store/migrations/0011_catalog_inventory_fx.sql` - Create
  catalog, inventory, movement, FX, and quote tables.
- `internal/store/catalog.go` - Category/item CRUD and default seeding.
- `internal/store/inventory.go` - Per-boat stock and movement service.
- `internal/store/fx.go` - Rate storage and integer conversion helpers.
- `internal/store/checkout_quotes.go` - Persisted quote creation.
- `internal/store/catalog_test.go` - Catalog lifecycle/default tests.
- `internal/store/inventory_test.go` - Adjustment/concurrency/isolation
  tests.
- `internal/store/fx_test.go` - Rounding and exponent tests.
- `internal/store/checkout_quotes_test.go` - Quote persistence tests.

**Tasks:**
- [ ] Add `0011_catalog_inventory_fx.sql` with constraints and indexes.
- [ ] Enforce category/item uniqueness among non-archived rows.
- [ ] Add stable `template_key` handling for default seed/repair.
- [ ] Seed the default catalog in the same transaction as org signup.
- [ ] Implement idempotent default repair that adds missing defaults
      and does not overwrite operator edits.
- [ ] Implement stock configuration and transactional movement with
      `SELECT ... FOR UPDATE`.
- [ ] Reject negative stock.
- [ ] Validate category, item, and boat ownership in store helpers.
- [ ] Implement FX conversion using integer/rational math only.
- [ ] Persist quote and quote-line snapshots.

### Phase 2: Backend API (~25%)

**Files:**
- `internal/httpapi/catalog_handlers.go` - Catalog/category/default
  endpoints.
- `internal/httpapi/inventory_handlers.go` - Fleet and boat stock
  endpoints.
- `internal/httpapi/fx_handlers.go` - Manual FX-rate endpoints.
- `internal/httpapi/checkout_quote_handlers.go` - Authenticated quote
  API.
- `internal/httpapi/httpapi.go` - Mount routes.
- `internal/httpapi/*_test.go` - API/RBAC/validation tests.

**Tasks:**
- [ ] Mount catalog/inventory/fx mutation endpoints behind
      `auth.RequireOrgAdmin`.
- [ ] Mount `/api/checkout/quote` behind session auth, not admin-only.
- [ ] Return 403 for Cruise Directors on admin catalog/inventory/fx
      mutation endpoints.
- [ ] Validate item fields, stock modes, movement types, money, and
      supported currency codes.
- [ ] Reject inventory rows for `stock_mode = none`.
- [ ] Return low-stock/out-of-stock status in inventory responses.
- [ ] Add quote tests for USD identity, converted currency, item-line
      snapshots, missing rate, expired rate, zero-decimal currency, and
      tenant isolation.

### Phase 3: Frontend Workbench (~30%)

**Files:**
- `web/src/admin/pages/Inventory.tsx` - Replace placeholder with
  org-level catalog/inventory workbench.
- `web/src/admin/pages/BoatTabs.tsx` - Replace boat inventory
  placeholder with per-boat stock table.
- `web/src/admin/api.ts` - Add catalog, inventory, FX, and quote types.
- `web/src/styles/app.css` - Add dense table, tab, form, badge, and
  modal styles.

**Tasks:**
- [ ] Build `/admin/inventory` tabs for Items, Categories, Boat Stock,
      and FX Rates.
- [ ] Add item create/edit modal with unit, charge type, stock mode,
      active state, and USD price fields.
- [ ] Add category create/rename/archive UI.
- [ ] Add "Apply missing defaults" command for legacy/dev orgs.
- [ ] Add fleet inventory summary with low-stock count by boat.
- [ ] Add boat inventory table with adjustment modal.
- [ ] Add admin FX-rate screen sufficient to support local quote API
      testing.
- [ ] Keep quote UI to admin/dev preview only; no guest checkout
      surface in this sprint.
- [ ] Run `npm run build` and manually inspect narrow table behavior.

### Phase 4: Product Docs and Follow-up Hooks (~10%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Update
  catalog/inventory/currency decisions.
- `docs/product/personas.md` - Clarify Admin ownership and future
  Cruise Director consumption.
- `docs/CONFIG.md` - Document future FX provider config only if added.

**Tasks:**
- [ ] Update backlog decision from org-currency pricing to USD catalog
      base plus checkout conversion.
- [ ] Document `folio_charge` and `folio_void` as the bridge to the
      Cruise Director ledger sprint.
- [ ] Note that payment, receipts, taxes, guest checkout UI, and folio
      posting are out of scope.
- [ ] Capture provider-backed FX fetching as a follow-up.

## API Endpoints

| Endpoint | Method | Auth | Purpose |
|---|---|---|---|
| `/api/admin/catalog/categories` | GET | Org Admin | List categories. |
| `/api/admin/catalog/categories` | POST | Org Admin | Create category. |
| `/api/admin/catalog/categories/{id}` | PATCH | Org Admin | Rename, reorder, reactivate, or archive category. |
| `/api/admin/catalog/items` | GET | Org Admin | List catalog items. |
| `/api/admin/catalog/items` | POST | Org Admin | Create catalog item. |
| `/api/admin/catalog/items/{id}` | PATCH | Org Admin | Edit item, price, active state, or archive marker. |
| `/api/admin/catalog/defaults/apply` | POST | Org Admin | Apply missing default categories/items. |
| `/api/admin/inventory/boats` | GET | Org Admin | Fleet stock summary. |
| `/api/admin/boats/{boat_id}/inventory` | GET | Org Admin | Per-boat inventory table. |
| `/api/admin/boats/{boat_id}/inventory/{item_id}` | PUT | Org Admin | Set stock config. |
| `/api/admin/boats/{boat_id}/inventory/{item_id}/adjustments` | POST | Org Admin | Append manual stock movement. |
| `/api/admin/fx/rates` | GET | Org Admin | List latest stored USD rates. |
| `/api/admin/fx/rates` | POST | Org Admin | Manually upsert a USD rate. |
| `/api/checkout/quote` | POST | Session | Persist and return checkout currency quote. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0011_catalog_inventory_fx.sql` | Create | Catalog, inventory, movements, FX, quote schema. |
| `internal/store/catalog.go` | Create | Catalog CRUD and default seeding. |
| `internal/store/inventory.go` | Create | Stock config and movement service. |
| `internal/store/fx.go` | Create | Rate storage and conversion math. |
| `internal/store/checkout_quotes.go` | Create | Checkout quote persistence. |
| `internal/httpapi/catalog_handlers.go` | Create | Admin catalog API. |
| `internal/httpapi/inventory_handlers.go` | Create | Admin inventory API. |
| `internal/httpapi/fx_handlers.go` | Create | Manual FX API. |
| `internal/httpapi/checkout_quote_handlers.go` | Create | Checkout quote API. |
| `internal/httpapi/httpapi.go` | Modify | Mount new routes. |
| `internal/auth/auth.go` | Modify | Seed default catalog after org creation. |
| `web/src/admin/api.ts` | Modify | Add typed catalog, inventory, FX, quote wrappers. |
| `web/src/admin/pages/Inventory.tsx` | Modify | Replace placeholder with workbench. |
| `web/src/admin/pages/BoatTabs.tsx` | Modify | Replace inventory placeholder with boat stock table. |
| `web/src/styles/app.css` | Modify | Dense tables, forms, status badges, modal styling. |
| `docs/product/organization-admin-user-stories.md` | Modify | Update catalog/inventory/currency decisions. |
| `docs/product/personas.md` | Modify | Clarify Admin ownership and future Cruise Director consumption. |

## Definition of Done

- [ ] New organizations receive the default liveaboard catalog at
      signup.
- [ ] Admin can apply missing defaults idempotently without overwriting
      edited items.
- [ ] Admin can create, edit, deactivate, reactivate, and archive
      catalog items.
- [ ] Admin can create, rename, reorder, reactivate, and archive
      categories.
- [ ] Catalog prices are stored only as USD cents.
- [ ] Merchandise defaults include t-shirts, hoodies, mugs, rash
      guards, sunscreen, logbooks, and dry bags.
- [ ] Admin can configure and adjust per-boat stock for counted items.
- [ ] Stock adjustments are transactional, auditable, and reject
      cross-tenant references.
- [ ] Stock adjustments reject negative stock.
- [ ] Inventory endpoints show low-stock and out-of-stock status.
- [ ] Movement service includes reusable `folio_charge` and
      `folio_void` paths for the future guest folio workflow.
- [ ] FX rates are stored as snapshots and conversion avoids float
      math.
- [ ] `/api/checkout/quote` persists and returns quote amount, target
      currency, exponent, rate metadata, expiration, and line snapshots
      when provided.
- [ ] Cruise Directors cannot mutate admin catalog, inventory, or FX
      endpoints.
- [ ] Backend tests cover store, API, tenant isolation, validation,
      rounding, default seeding, and stock movement behavior.
- [ ] `npm run build` passes.
- [ ] `go test ./...` passes with local PostgreSQL and network bind
      permissions.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Scope grows into folios/payments | High | High | Ship quote API only; defer payment, receipt, tax, guest checkout UI, and ledger posting. |
| Default seed overwrites operator edits | Medium | High | Seed by stable template keys; repair endpoint adds missing defaults only. |
| Cross-tenant FK mistakes leak data | Medium | High | Org-scoped store helpers, ownership validation, and tenant isolation tests. |
| Stock races produce incorrect counts | Medium | High | Use transaction + row lock for every movement. |
| Negative stock semantics are unclear | Medium | Medium | Reject negative stock in this sprint; revisit override roles later. |
| FX rounding differs by currency | Medium | High | Store currency exponent and test zero-, two-, and three-decimal currencies. |
| Price edits break quote reproducibility | Medium | High | Persist source total, rate snapshot, and item-line price snapshots. |
| Default catalog is too opinionated | Medium | Low | Defaults are editable; repair endpoint only adds missing defaults. |

## Security Considerations

- Every catalog, inventory, movement, FX, and quote query is scoped by
  authenticated `organization_id`.
- Admin catalog/inventory/FX mutations require `org_admin`.
- Checkout quote is authenticated and org-scoped; it is not admin-only
  so future Cruise Director checkout can use it.
- No hard deletion for catalog rows that may later be referenced by
  ledger/folio data.
- Stock movements are append-only audit records with actor attribution.
- Manual FX-rate endpoint is admin-only and validates supported
  currencies.
- Future FX provider secrets must stay server-side only.

## Dependencies

- Existing auth/session/RBAC stack.
- Existing organizations, users, boats, and trips schema.
- Local PostgreSQL test database.
- Future dependency: production FX provider. Sprint 013 can ship with
  manual rates and provider-neutral snapshots.

## References

- Explorer Ventures current onboard charges: https://www.explorerventures.com/current-onboard-charges/
- Aggressor equipment rental and Nitrox pricing: https://www.aggressor.com/pages/equipment-rental
- Emperor Divers boat pages: https://www.emperordivers.com/liveaboard-boat/emperor-voyager/
- Horizon III optional extras: https://www.liveaboards.com/en-us/maldives/horizon-iii-liveaboard
- ECB reference-rate caveat: https://www.ecb.europa.eu/stats/policy_and_exchange_rates/euro_reference_exchange_rates/html/index.en.html
