# Sprint 013: Catalog, Inventory, and Checkout Currency Foundation

## Overview

The admin chrome already has the right navigation shape for Fleet,
Trips, Users, Import, and Inventory, but Inventory is still a
placeholder. This sprint turns that vertical into the operator's
sellable catalog and stock-control workbench: categories, items,
USD pricing, active/inactive lifecycle, per-boat quantities, low-stock
thresholds, and stock adjustments.

The scope deliberately treats "catalog" and "inventory" as separate
concepts. A catalog item is what can be sold or charged to a guest:
beer can, beer bottle, wine glass, wine bottle, massage, laundry item,
Nitrox week, full rental kit, WiFi code, park fee, corkage, gratuity,
or retail merchandise. Inventory is only for items with a stock count
or capacity constraint. Services such as massage, dive instruction,
laundry, fees, gratuity, and WiFi may appear in the catalog without
depleting stock unless an operator explicitly marks them as tracked.

The user request also changes the currency model: catalog prices are
canonical in USD, and guest checkout will need currency conversion.
This sprint should store USD prices in integer cents and introduce the
exchange-rate snapshot model needed for checkout quotes, and build the
backend quote API. It does not build payment processing, receipt
finalization, or the full guest checkout screen.

## Use Cases

1. **Start with a default liveaboard catalog.** A new organization is
   created with suggested categories and SKUs: Bar, Wine, Soft Drinks,
   Dive Services, Gear Rental, Retail, Spa, Laundry, WiFi, Fees,
   Gratuities, and Miscellaneous.
2. **Add sellable item.** Admin adds `Beer - Can`, category `Bar`,
   unit `can`, price `USD 6.00`, stock-tracked `yes`, and a low-stock
   threshold. It appears immediately in the catalog and per-boat stock
   tables.
3. **Represent different units as different SKUs.** Admin creates
   `Wine - Glass` and `Wine - Bottle` separately because their prices,
   inventory units, and checkout behavior differ.
4. **Add non-stocked service.** Admin adds `Massage - 60 minutes`,
   category `Spa`, unit `session`, price `USD 85.00`, stock-tracked
   `no`. It can be sold later without requiring boat stock.
5. **Set per-boat inventory.** Admin opens a boat's Inventory tab and
   sets `Beer - Can` on hand to `144`, reorder level to `24`, and note
   `Loaded before Komodo season`.
6. **Track merchandise.** Admin adds retail SKUs such as `T-Shirt`,
   `Hoodie`, `Mug`, `Rash Guard`, `Reef-Safe Sunscreen`, and `Dry
   Bag`, then tracks stock per boat like any other counted item.
7. **Adjust stock.** Admin records stock adjustments such as
   `restock +48`, `breakage -3`, `cycle count +2`, or `manual
   correction -1`. The current quantity updates and the adjustment is
   auditable.
8. **Reduce inventory from folio activity.** A future Cruise Director
   folio endpoint can add `Beer - Can x2` to a guest's folio and call
   the same inventory movement path with `movement_type =
   folio_charge`, automatically reducing boat stock.
9. **Deactivate item without losing history.** Admin deactivates
   `Nitrox Student Fills`. It disappears from future sale pickers but
   remains visible in catalog management and keeps historical identity
   for future ledger rows.
10. **Quote checkout currency.** Backend can quote a USD cart
   total into EUR, GBP, AUD, IDR, THB, or another supported currency
   using a stored rate snapshot. The quote stores source currency,
   target currency, rate, provider, as-of timestamp, rounded total,
   and expiration.
11. **See low-stock operational risk.** Overview and Inventory show
   boats below reorder threshold for tracked SKUs, with links back to
   the boat stock screen.
12. **Cruise Director visibility later.** Cruise Directors do not
    manage catalog/inventory in this sprint, but the API shape includes
    the stock-decrement contract a folio sale picker will need.

## Proposed Feature Set

### Categories

Default categories seeded from liveaboard research and domain needs:

| Category | Examples | Notes |
|---|---|---|
| Bar | Beer can, beer bottle, wine glass, wine bottle, spirits, cocktail, corkage beer can, corkage wine bottle | Alcohol units are explicit SKUs. |
| Soft Drinks | Soda can, tonic, sparkling water, fresh juice, electrolyte drink | Often paid extra while water/tea/coffee are included. |
| Wine | House wine glass, house wine bottle, premium wine bottle, sparkling bottle | Can be separate from Bar if operators want tighter reporting. |
| Dive Services | Nitrox fill, Nitrox day, Nitrox week, Nitrox course, extra dive, private guide | Some operators include Nitrox; others charge per fill/day/week. |
| Gear Rental | Full kit, BCD, regulator, computer, mask/fins/snorkel, wetsuit, torch, SMB, reef hook, 15L tank, rescue GPS | Rental may be per trip, per day, or per night. |
| Retail | T-shirt, hoodie, mug, rash guard, reef-safe sunscreen, mask strap, logbook, dry bag, camera accessory | Stock-tracked merchandise. |
| Spa | Massage 30 min, massage 60 min, massage 90 min, spa treatment | Non-stocked service. |
| Laundry | Laundry item, laundry bag, pressing item | Can be unit-priced by item or bag. |
| Connectivity | WiFi week code, WiFi day pass, satellite phone minute | Stock optional if codes are finite. |
| Fees | Marine park fee, harbor fee, fuel surcharge, environment tax, visa, transfer, land tour | May be pass-through and required at checkout. |
| Gratuities | Crew gratuity suggested, guide gratuity | Optional or required depending operator. |
| Miscellaneous | Custom charge, damage/lost gear fee | Admin-defined fallback. |

### Catalog Item Fields

- `name`: required, unique per org among non-archived items.
- `category_id`: required.
- `description`: optional admin-facing description.
- `unit`: enum/text with suggested values: `each`, `can`, `bottle`,
  `glass`, `fill`, `day`, `week`, `trip`, `session`, `item`, `bag`,
  `minute`, `person`, `custom`.
- `price_usd_cents`: required non-negative integer. Zero is allowed
  for included-but-tracked items such as complimentary water or free
  Nitrox when the operator wants stock visibility.
- `charge_type`: `sale`, `rental`, `service`, `fee`, `gratuity`,
  `deposit`, `damage`, `included`.
- `stock_mode`: `none`, `counted`, `capacity`, `unlimited`.
- `is_taxable`: boolean placeholder for future tax/reporting.
- `is_required_fee`: boolean for items that should be added to every
  guest/trip checkout later.
- `is_active`: boolean; inactive items hidden from future sale flows.
- `archived_at`: nullable soft-delete marker.
- `created_at`, `updated_at`.

### Inventory Fields

- `boat_inventory_items` rows exist only for stock-tracked/capacity
  items on a boat.
- `quantity_on_hand`: integer for counted stock.
- `quantity_reserved`: integer reserved by future ledger/cart flows.
  Defaults to 0 in this sprint.
- `reorder_level`: nullable integer threshold.
- `par_level`: nullable integer target load quantity.
- `last_counted_at`: nullable timestamp.
- `notes`: optional per-boat notes.
- `stock_movements`: append-only adjustment log with movement type,
  delta, before/after quantities, actor, optional source type/id,
  note, and timestamp.

### Currency Conversion Foundation

- Store catalog prices only in USD cents for now.
- Add `exchange_rates` snapshots with provider, base currency,
  quote currency, rate decimal string, as-of timestamp, fetched_at,
  and expires_at.
- Add a small `internal/fx` service that can:
  - return identity conversion for USD -> USD,
  - load the latest non-expired rate from DB,
  - optionally fetch rates from a provider later,
  - convert integer USD cents to target minor units with deterministic
    rounding.
- Add a `checkout_quotes` table for quote API responses. It stores
  source amount, source currency, target amount, target currency, rate
  snapshot, provider/as-of metadata, expiration, and optional
  requested_by user.
- Do not use floating point for money. Use integer minor units plus
  decimal rate strings parsed with fixed-precision decimal math. Since
  this repo avoids heavy dependencies, implement a small rational
  decimal helper or store rates as numerator/denominator integers.

## Architecture

### Data Model

```
organizations
  id
  currency                 -- existing display/default preference

catalog_categories
  id
  organization_id
  name
  sort_order
  is_system
  archived_at
  created_at
  updated_at

catalog_items
  id
  organization_id
  category_id
  name
  description
  unit
  charge_type
  stock_mode
  price_usd_cents
  is_taxable
  is_required_fee
  is_active
  archived_at
  created_at
  updated_at

boat_inventory_items
  id
  organization_id
  boat_id
  catalog_item_id
  quantity_on_hand
  quantity_reserved
  reorder_level
  par_level
  last_counted_at
  notes
  created_at
  updated_at

stock_movements
  id
  organization_id
  boat_id
  catalog_item_id
  actor_user_id
  movement_type           -- initial_count | restock | correction | breakage | spoilage | manual_use
                          -- folio_charge is reserved for Cruise Director folio rows
  delta_quantity
  quantity_before
  quantity_after
  source_type
  source_id
  note
  created_at

exchange_rates
  id
  provider
  base_currency           -- USD
  quote_currency
  rate_numerator
  rate_denominator
  as_of
  fetched_at
  expires_at

checkout_quotes
  id
  organization_id
  requested_by
  source_currency         -- USD
  target_currency
  source_amount_cents
  target_amount_minor
  rate_numerator
  rate_denominator
  rate_provider
  rate_as_of
  expires_at
  created_at
```

Tenant scoping is duplicated onto every operational table so every
query can include `organization_id = $ctx_org` without relying on
joins for isolation.

### API Surface

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/admin/catalog/categories` | Org Admin | List categories with item counts. |
| POST | `/api/admin/catalog/categories` | Org Admin | Create category. |
| PATCH | `/api/admin/catalog/categories/{id}` | Org Admin | Rename/reorder/archive category. |
| GET | `/api/admin/catalog/items` | Org Admin | List catalog items with filters. |
| POST | `/api/admin/catalog/items` | Org Admin | Create catalog item. |
| PATCH | `/api/admin/catalog/items/{id}` | Org Admin | Edit item fields and active state. |
| POST | `/api/admin/catalog/defaults` | Org Admin | Copy default liveaboard template into org. |
| GET | `/api/admin/inventory/boats` | Org Admin | Matrix summary: boats x low-stock counts. |
| GET | `/api/admin/boats/{id}/inventory` | Org Admin | Per-boat stock table. |
| PUT | `/api/admin/boats/{id}/inventory/{item_id}` | Org Admin | Set stock config/quantity. |
| POST | `/api/admin/boats/{id}/inventory/{item_id}/adjustments` | Org Admin | Append stock movement and update quantity. |
| GET | `/api/admin/fx/rates` | Org Admin | List latest rates available for checkout preview. |
| POST | `/api/admin/fx/rates` | Org Admin | Manually upsert a USD conversion rate for local testing. |
| POST | `/api/checkout/quote` | Authenticated | Quote a USD amount or item list into a target checkout currency. |

The manual FX rate endpoint is intentionally admin-only and dev-friendly.
A provider-backed fetcher can be added once payment/checkout scope is
clear. The quote endpoint is authenticated rather than admin-only so a
future Cruise Director checkout flow can call it; it still scopes all
data to the caller's organization.

### Frontend Layout

`/admin/inventory` becomes an org-level workbench. Because new orgs get
the default catalog at signup, the empty state is only for legacy/dev
orgs whose catalog has been deliberately cleared:

```
Inventory
Items, prices, and stock across the fleet.

[ Items ] [ Categories ] [ Boat Stock ] [ FX Rates ]

Items tab
  Search [_______] Category [All v] Status [Active v] Stock [All v]
  + Add item        Reset/apply missing defaults

  CATEGORY      ITEM                 UNIT     PRICE USD  STOCK   STATUS
  Bar           Beer - Can           can      $6.00      Counted Active
  Bar           Wine - Bottle        bottle   $42.00     Counted Active
  Spa           Massage - 60 min     session  $85.00     None    Active
  Laundry       Laundry - Item       item     $5.00      None    Active
```

Boat detail Inventory tab becomes the operational stock screen for a
single vessel:

```
Gaia Love / Inventory

Search [_______] Category [All v] Low stock only [ ]

ITEM             ON HAND  RESERVED  REORDER  PAR   STATUS       ACTION
Beer - Can       144      0         24       192   OK           Adjust
Wine - Bottle    8        0         12       24    Low stock    Adjust
T-shirt          0        0         4        12    Out          Adjust
Massage - 60     -        -         -        -     Service      -
```

### Default Liveaboard Template

Seeded defaults should be copied into every new org during signup, not
inserted globally at migration time. That keeps operator catalogs
editable and prevents global data from acquiring tenant-specific
assumptions. The admin "reset/apply missing defaults" command exists
for legacy/dev orgs and for operators who deleted a starter SKU and
later want it back.

Suggested default items:

| Category | Item | Unit | USD |
|---|---|---:|---:|
| Bar | Beer - Can | can | 6.00 |
| Bar | Beer - Bottle | bottle | 7.00 |
| Bar | House Wine - Glass | glass | 8.00 |
| Bar | House Wine - Bottle | bottle | 38.00 |
| Bar | Premium Wine - Bottle | bottle | 65.00 |
| Bar | Cocktail | each | 12.00 |
| Bar | Corkage - Wine Bottle | bottle | 20.00 |
| Bar | Corkage - Beer Can | can | 2.00 |
| Soft Drinks | Soft Drink - Can | can | 3.00 |
| Soft Drinks | Fresh Juice | glass | 5.00 |
| Dive Services | Nitrox - Fill | fill | 10.00 |
| Dive Services | Nitrox - Day | day | 20.00 |
| Dive Services | Nitrox - Week | week | 150.00 |
| Dive Services | Nitrox Course | each | 150.00 |
| Dive Services | Extra Dive | each | 45.00 |
| Dive Services | Private Dive Guide | day | 125.00 |
| Gear Rental | Full Rental Kit | trip | 175.00 |
| Gear Rental | BCD Rental | trip | 60.00 |
| Gear Rental | Regulator Rental | trip | 60.00 |
| Gear Rental | Dive Computer Rental | trip | 60.00 |
| Gear Rental | Mask/Fins/Snorkel Rental | trip | 50.00 |
| Gear Rental | Wetsuit Rental | trip | 50.00 |
| Gear Rental | Night Light Rental | night | 8.00 |
| Gear Rental | 15L / Large Tank Rental | trip | 75.00 |
| Retail | T-Shirt | each | 25.00 |
| Retail | Hoodie | each | 55.00 |
| Retail | Mug | each | 18.00 |
| Retail | Rash Guard | each | 45.00 |
| Retail | Reef-Safe Sunscreen | each | 18.00 |
| Spa | Massage - 30 Minutes | session | 45.00 |
| Spa | Massage - 60 Minutes | session | 85.00 |
| Laundry | Laundry - Item | item | 5.00 |
| Laundry | Laundry - Bag | bag | 25.00 |
| Connectivity | WiFi - Week Code | week | 30.00 |
| Fees | Marine Park Fee | person | 150.00 |
| Fees | Fuel Surcharge | person | 150.00 |
| Fees | Airport Transfer | person | 25.00 |
| Gratuities | Crew Gratuity | person | 150.00 |
| Miscellaneous | Custom Charge | each | 0.00 |

Prices are defaults for bootstrapping only. Operators can edit them
immediately; historical price preservation matters once ledger rows
exist.

## Implementation Plan

### Phase 1: Schema and Store Layer (~35%)

**Files:**
- `internal/store/migrations/0011_catalog_inventory_fx.sql` - Create catalog, inventory, movement, and FX tables.
- `internal/store/catalog.go` - Category/item CRUD.
- `internal/store/inventory.go` - Per-boat stock and movement helpers.
- `internal/store/fx.go` - Rate storage and conversion helpers.
- `internal/store/checkout_quotes.go` - Quote persistence helper.
- `internal/store/catalog_test.go` - Catalog uniqueness/lifecycle tests.
- `internal/store/inventory_test.go` - Stock adjustment and tenant isolation tests.
- `internal/store/fx_test.go` - Deterministic conversion/rounding tests.

**Tasks:**
- [ ] Add migration with org-scoped tables and indexes.
- [ ] Enforce case-insensitive category and item uniqueness per org
      among non-archived records.
- [ ] Store all USD prices as integer cents with `CHECK
      (price_usd_cents >= 0)`.
- [ ] Enforce valid `charge_type`, `stock_mode`, and stock movement
      enums with DB checks.
- [ ] Ensure inventory rows are unique by `(organization_id, boat_id,
      catalog_item_id)`.
- [ ] Add transactional stock adjustment helper that writes
      `stock_movements` and updates `quantity_on_hand` atomically.
- [ ] Add a shared stock movement entrypoint that can be reused by
      future Cruise Director folio charges with `movement_type =
      folio_charge`.
- [ ] Add fixed-precision FX conversion helper with no float math.
- [ ] Add checkout quote persistence with expiration.
- [ ] Cover cross-tenant access and soft archive behavior in tests.

### Phase 2: Admin API (~25%)

**Files:**
- `internal/httpapi/catalog_handlers.go` - Catalog/category/default-template endpoints.
- `internal/httpapi/inventory_handlers.go` - Fleet/boat inventory endpoints.
- `internal/httpapi/fx_handlers.go` - Manual FX rate endpoints.
- `internal/httpapi/checkout_quote_handlers.go` - Authenticated quote API.
- `internal/httpapi/httpapi.go` - Mount endpoints under authenticated Org Admin routes.
- `internal/httpapi/catalog_handlers_test.go` - API/RBAC/validation tests.
- `internal/httpapi/inventory_handlers_test.go` - Stock adjustment API tests.
- `internal/httpapi/fx_handlers_test.go` - Rate validation/conversion tests.

**Tasks:**
- [ ] Mount all mutating endpoints behind `auth.RequireOrgAdmin`.
- [ ] Return 403 for Cruise Director direct API access.
- [ ] Validate money, units, stock modes, and category ownership.
- [ ] Make item deactivation/reactivation a PATCH, not delete.
- [ ] Make category archival fail if active items still reference it.
- [ ] Add default-template endpoint that is idempotent by item name.
- [ ] Return low-stock/out-of-stock flags from inventory list endpoints.
- [ ] Add JSON shapes to `web/src/admin/api.ts`.
- [ ] Add quote endpoint tests for USD identity, converted currency,
      missing rate, expired rate, and tenant isolation.

### Phase 3: Frontend Workbench (~30%)

**Files:**
- `web/src/admin/pages/Inventory.tsx` - Replace placeholder with tabs and item/category/boat-stock/FX views.
- `web/src/admin/pages/BoatTabs.tsx` - Replace boat inventory placeholder with per-boat stock table.
- `web/src/admin/api.ts` - Add catalog, inventory, and FX types/wrappers.
- `web/src/styles/app.css` - Add dense table, editor, status badge, and adjustment modal styles.

**Tasks:**
- [ ] Build `/admin/inventory` tabs for Items, Categories, Boat Stock,
      and FX Rates.
- [ ] Add item create/edit modal with unit, charge type, stock mode,
      active state, and USD price fields.
- [ ] Add category create/rename/archive UI.
- [ ] Add "Reset/apply missing defaults" command for legacy/dev orgs,
      while signup seeds defaults automatically.
- [ ] Add fleet inventory summary with low-stock count by boat.
- [ ] Add boat inventory table with stock adjustment modal.
- [ ] Keep the UI dense and operational per `DESIGN.md`; no marketing
      cards beyond repeated item rows/modals.
- [ ] Ensure all text fits at narrow widths and tables remain usable.

### Phase 4: Product Docs and Follow-up Hooks (~10%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Update catalog/inventory stories and USD/conversion decision.
- `docs/product/personas.md` - Clarify that Org Admin owns catalog and stock; Cruise Director consumes it later.
- `docs/CONFIG.md` - Document future FX provider env keys only if added.
- `docs/sprints/SPRINT-013.md` - Final sprint doc after merge.

**Tasks:**
- [ ] Update the backlog decision from org-currency pricing to USD
      catalog base + checkout conversion.
- [ ] Capture which catalog features are in this sprint vs. future
      ledger/checkout.
- [ ] Note that payment processing, receipt generation, taxes, and
      ledger posting are out of scope.
- [ ] Add follow-up notes for stock decrements from guest consumption.
- [ ] Document the `folio_charge` stock movement contract for the
      Cruise Director ledger sprint.

## API Response Sketches

### Catalog Item

```json
{
  "id": "uuid",
  "category_id": "uuid",
  "category_name": "Bar",
  "name": "Beer - Can",
  "description": null,
  "unit": "can",
  "charge_type": "sale",
  "stock_mode": "counted",
  "price_usd_cents": 600,
  "price_usd": "6.00",
  "is_taxable": true,
  "is_required_fee": false,
  "is_active": true,
  "archived_at": null
}
```

### Boat Inventory Row

```json
{
  "boat_id": "uuid",
  "catalog_item_id": "uuid",
  "item_name": "Beer - Can",
  "category_name": "Bar",
  "unit": "can",
  "stock_mode": "counted",
  "quantity_on_hand": 144,
  "quantity_reserved": 0,
  "reorder_level": 24,
  "par_level": 192,
  "status": "ok",
  "last_counted_at": "2026-05-07T18:00:00Z"
}
```

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0011_catalog_inventory_fx.sql` | Create | Catalog, inventory, stock movements, FX rates schema. |
| `internal/store/catalog.go` | Create | Store helpers for categories/items/default template. |
| `internal/store/inventory.go` | Create | Store helpers for boat stock and movements. |
| `internal/store/fx.go` | Create | Store helpers for USD conversion rates. |
| `internal/store/checkout_quotes.go` | Create | Store helpers for persisted checkout quote snapshots. |
| `internal/httpapi/catalog_handlers.go` | Create | Catalog/category/default-template API. |
| `internal/httpapi/inventory_handlers.go` | Create | Per-boat and fleet inventory API. |
| `internal/httpapi/fx_handlers.go` | Create | Manual FX rate API for checkout foundation. |
| `internal/httpapi/checkout_quote_handlers.go` | Create | Authenticated checkout currency quote API. |
| `internal/httpapi/httpapi.go` | Modify | Mount new routes behind Org Admin RBAC. |
| `web/src/admin/pages/Inventory.tsx` | Modify | Org-level inventory workbench. |
| `web/src/admin/pages/BoatTabs.tsx` | Modify | Boat-level stock table and adjustments. |
| `web/src/admin/api.ts` | Modify | Typed API wrappers. |
| `web/src/styles/app.css` | Modify | Inventory workbench styling. |
| `docs/product/organization-admin-user-stories.md` | Modify | Update catalog/inventory/currency decisions. |
| `docs/product/personas.md` | Modify | Clarify ownership boundaries. |

## Definition of Done

- [ ] Org Admin can create, list, edit, deactivate, reactivate catalog
      items.
- [ ] Org Admin can create, rename, reorder, and archive empty
      categories.
- [ ] Default liveaboard catalog template can be applied idempotently.
- [ ] New organizations receive the default liveaboard catalog at
      signup.
- [ ] Catalog item prices are stored as USD cents and displayed as USD.
- [ ] Org Admin can set per-boat stock configuration and quantities for
      stock-tracked items.
- [ ] Stock adjustments are transactional and auditable.
- [ ] Inventory surfaces low-stock/out-of-stock state.
- [ ] FX rate snapshots can be stored and used to convert USD cents to
      another currency without float math.
- [ ] `/api/checkout/quote` returns a persisted quote with expiration,
      target minor units, rate metadata, and deterministic rounding.
- [ ] Stock movement service reserves a `folio_charge` path for future
      Cruise Director guest folio entries.
- [ ] Cruise Directors cannot access admin catalog/inventory mutation
      endpoints.
- [ ] Backend tests cover schema/store/API behavior, including tenant
      isolation.
- [ ] Frontend build passes.
- [ ] `go test ./...` passes in a local environment with Postgres and
      network bind permissions.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Scope balloons into ledger/payment | High | High | Include quote API only; defer guest tabs, payment capture, receipts, and taxes. |
| Price edits break future historical ledger | Medium | High | Store ledger price snapshots later; this sprint avoids hard delete and keeps item identity stable. |
| USD base conflicts with existing org currency field | Medium | Medium | Treat org currency as display/default checkout preference; catalog base is USD. Update docs explicitly. |
| Inventory model too rigid for services | Medium | Medium | `stock_mode = none` for services; only counted/capacity items require stock rows. |
| FX rounding errors | Medium | High | Use integer minor units plus rational rates; add table-driven tests. |
| Default catalog feels too opinionated | Medium | Low | Make defaults opt-in and editable; keep prices as starter values. |

## Security Considerations

- Every catalog, inventory, movement, and FX query is scoped by
  `organization_id`.
- All mutation endpoints require `org_admin`; Cruise Director direct
  calls return 403.
- No destructive deletion of catalog items with future ledger
  references; use inactive/archive semantics.
- Stock movement rows are append-only audit records.
- Do not expose FX provider API keys to the frontend if provider-backed
  rates are added later.
- Validate currency codes against a small allow-list before storing FX
  rates.

## Dependencies

- Existing auth/session/RBAC stack.
- Existing boats and organizations schema.
- PostgreSQL local test database for store/API tests.
- Optional later dependency: a production FX provider. This sprint can
  ship with manually-entered rates plus schema/service boundaries.

## References

- Explorer Ventures current onboard charges: https://www.explorerventures.com/current-onboard-charges/
- Aggressor equipment rental and Nitrox pricing: https://www.aggressor.com/pages/equipment-rental
- Emperor Divers boat pages: https://www.emperordivers.com/liveaboard-boat/emperor-voyager/
- Horizon III liveaboard optional extras: https://www.liveaboards.com/en-us/maldives/horizon-iii-liveaboard
- ECB reference-rate caveat: https://www.ecb.europa.eu/stats/policy_and_exchange_rates/euro_reference_exchange_rates/html/index.en.html
