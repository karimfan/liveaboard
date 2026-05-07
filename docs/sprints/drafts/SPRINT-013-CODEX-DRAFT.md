# Sprint 013: Catalog, Inventory, and Checkout Currency Foundation

## Overview

This sprint turns the Inventory placeholder into the foundation for
onboard sales: an org-owned catalog, per-boat stock tracking,
auditable inventory movements, and checkout currency quoting. Catalog
prices are canonical in USD cents. `organizations.currency` remains
the operator's display/default checkout preference, not the source of
catalog price truth.

The scope covers both stock-tracked goods and non-stocked services.
Beer cans, wine bottles, t-shirts, hoodies, mugs, sunscreen, and dry
bags can deplete per-boat inventory. Massage, laundry, Nitrox
packages, WiFi, fees, gratuities, and custom services can live in the
same catalog without requiring stock rows. Inventory mutations must
support manual admin adjustments now and future automatic decrements
when Cruise Directors add stock-tracked items to guest folios.

Checkout payment, receipts, taxes, and folio posting are out of
scope. The quote API is in scope: it must quote a USD cart or amount
into a target currency using a persisted exchange-rate snapshot and
deterministic rounding.

## Use Cases

1. **Seed a new org catalog**: New organization signup creates a
   default liveaboard catalog with bar, soft drinks, dive services,
   gear rental, merchandise, spa, laundry, WiFi, fees, gratuities, and
   miscellaneous categories.
2. **Manage catalog items**: Org Admin creates, edits, deactivates,
   reactivates, and archives sellable items without losing historical
   identity.
3. **Separate unit-specific SKUs**: Beer can, beer bottle, wine glass,
   and wine bottle are separate items because price and depletion
   units differ.
4. **Track merchandise**: T-shirts, hoodies, mugs, rash guards,
   sunscreen, logbooks, and dry bags are stock-tracked retail SKUs per
   boat.
5. **Add services and fees**: Massage, laundry, Nitrox week, marine
   park fee, airport transfer, corkage, WiFi pass, and gratuity can be
   sold later without stock depletion unless explicitly tracked.
6. **Manage boat stock**: Org Admin sets on-hand quantity, reorder
   level, par level, and notes for stock-tracked items on each boat.
7. **Adjust inventory manually**: Admin records restock, breakage,
   spoilage, cycle count, correction, and internal-use adjustments
   with actor, note, and before/after quantities.
8. **Prepare for folio decrements**: Future Cruise Director folio
   posting can call the same movement service with `movement_type =
   folio_charge`.
9. **Quote checkout currency**: Backend returns a persisted quote
   converting USD totals to the guest checkout currency with rate
   metadata, expiration, and rounded minor-unit amount.
10. **Surface stock risk**: Admin inventory views show low-stock and
    out-of-stock status for every boat.

## Architecture

Core modeling rules:

- Catalog prices are stored as USD minor units:
  `price_usd_cents bigint NOT NULL CHECK (price_usd_cents >= 0)`.
- Inventory quantities are integers. Fractional pours are modeled as
  separate SKUs, not decimal stock.
- Every tenant-owned table includes `organization_id`.
- Boat inventory rows must prove that the boat and catalog item belong
  to the same org.
- Stock movements are append-only. Current quantity is updated
  transactionally from the movement path.
- Quote responses persist the rate snapshot used so checkout math is
  reproducible.

## Implementation Plan

### Phase 1: Schema, Defaults, and Store Layer (~35%)

**Files:**
- `internal/store/migrations/0011_catalog_inventory_fx.sql`
- `internal/store/catalog.go`
- `internal/store/inventory.go`
- `internal/store/fx.go`
- `internal/store/checkout_quotes.go`
- `internal/store/*_test.go`

**Tasks:**
- [ ] Add catalog, inventory, stock movement, exchange-rate, and
      checkout quote tables.
- [ ] Add check constraints for `charge_type`, `stock_mode`,
      `movement_type`, non-negative prices, non-negative stock, and
      valid rate denominators.
- [ ] Add partial unique indexes for active/non-archived category and
      item names per org.
- [ ] Add composite integrity checks or explicit store validation so a
      boat inventory row cannot connect an org's item to another org's
      boat.
- [ ] Implement default catalog seeding as a store transaction used by
      signup and an idempotent admin repair endpoint.
- [ ] Implement stock adjustment with `SELECT ... FOR UPDATE`,
      before/after quantities, append-only movement row, and current
      quantity update in one transaction.
- [ ] Reserve `folio_charge` and `folio_void` movement types even if
      folio tables are future work.
- [ ] Implement FX conversion without floats, including USD identity
      conversion and currency minor-unit exponent handling.
- [ ] Persist checkout quotes with expiration and rate snapshot.

### Phase 2: Backend API (~25%)

**Files:**
- `internal/httpapi/catalog_handlers.go`
- `internal/httpapi/inventory_handlers.go`
- `internal/httpapi/fx_handlers.go`
- `internal/httpapi/checkout_quote_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/*_test.go`

**Tasks:**
- [ ] Mount catalog and inventory mutations under authenticated Org
      Admin routes.
- [ ] Keep `/api/checkout/quote` authenticated but not admin-only so
      future Cruise Director checkout can use it.
- [ ] Validate UUID ownership for category, item, boat, and stock rows.
- [ ] Reject inventory rows for `stock_mode = none`.
- [ ] Reject movements that make `quantity_on_hand < 0`.
- [ ] Return low-stock and out-of-stock flags from inventory list
      responses.
- [ ] Make default catalog repair idempotent without overwriting
      operator-edited prices.
- [ ] Add quote request shape for either `source_amount_cents` or item
      lines; item-line quotes must snapshot current USD price in the
      quote calculation.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/catalog/categories` | GET | List categories. |
| `/api/admin/catalog/categories` | POST | Create category. |
| `/api/admin/catalog/categories/{id}` | PATCH | Rename, reorder, reactivate, or archive category. |
| `/api/admin/catalog/items` | GET | List catalog items. |
| `/api/admin/catalog/items` | POST | Create catalog item. |
| `/api/admin/catalog/items/{id}` | PATCH | Edit item, price, active state, or archive marker. |
| `/api/admin/catalog/defaults/apply` | POST | Apply missing defaults idempotently. |
| `/api/admin/inventory/boats` | GET | Fleet stock summary. |
| `/api/admin/boats/{boat_id}/inventory` | GET | Per-boat inventory table. |
| `/api/admin/boats/{boat_id}/inventory/{item_id}` | PUT | Set stock config. |
| `/api/admin/boats/{boat_id}/inventory/{item_id}/adjustments` | POST | Append manual stock movement. |
| `/api/admin/fx/rates` | GET | List latest stored USD conversion rates. |
| `/api/admin/fx/rates` | POST | Manually upsert rate. |
| `/api/checkout/quote` | POST | Persist and return checkout currency quote. |

## Definition of Done

- [ ] New organizations receive the default liveaboard catalog at
      signup.
- [ ] Admin can re-apply missing defaults idempotently without
      overwriting edited items.
- [ ] Admin can create, edit, deactivate, reactivate, and archive
      catalog items and categories.
- [ ] Catalog prices are stored only as USD cents.
- [ ] Admin can configure and adjust per-boat stock for stock-tracked
      items.
- [ ] Stock adjustments are transactional, auditable, and reject
      cross-tenant references.
- [ ] Movement service includes a reusable future `folio_charge` path.
- [ ] FX rates are stored as snapshots and conversion avoids float
      math.
- [ ] `/api/checkout/quote` persists and returns quote amount, target
      currency, exponent, rate metadata, and expiration.
- [ ] Backend tests cover store, API, tenant isolation, validation,
      rounding, and stock movement behavior.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Catalog scope grows into folios/payments | High | High | Ship quote API only; defer payment, receipt, tax, and ledger posting. |
| Default seed overwrites operator edits | Medium | High | Seed by stable template key/name only for missing rows. |
| Cross-tenant FK mistakes leak data | Medium | High | Require org-scoped queries and composite ownership validation. |
| Stock races produce incorrect counts | Medium | High | Use transaction + row lock for movements. |
| FX rounding differs by currency | Medium | High | Store currency exponent and table-test zero/three-decimal currencies. |

