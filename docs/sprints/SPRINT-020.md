# Sprint 020: Pricing Overrides and Currency Defaults

## Overview

Sprint 020 adds per-item pricing configuration between the catalog and
guest folios. Catalog items remain canonical USD, but a folio line now
snapshots the effective price for the trip context: trip override first,
boat override second, base catalog price last.

This sprint also changes payment settings so every organization starts
with USD and EUR accepted. USD remains always supported; EUR is
default-enabled but removable by an Org Admin. Tax, service charge,
packages, and discounts are deferred.

## Use Cases

1. **Boat-level pricing**: An Org Admin sets a USD override for one
   catalog item on one boat.
2. **Trip-level pricing**: An Org Admin sets a USD override for one
   catalog item on one trip.
3. **Operational line entry**: A Director adds a catalog item from the
   live ledger and the line snapshots the effective price and source.
4. **Historical safety**: Admin edits or archives an override after a
   line is written; existing folio totals do not change.
5. **Currency defaults**: A new org starts with USD/EUR accepted, then
   an Admin can remove EUR if the org does not accept it.

## Architecture

### Price Model

- Base catalog prices stay in `catalog_items.price_usd_cents`.
- Overrides live in `catalog_price_overrides`.
- Exactly one of `boat_id` or `trip_id` is set per override.
- Active overrides are unique per `(organization, catalog item, boat)`
  and `(organization, catalog item, trip)`.
- Archived overrides remain for history and line snapshot references.

Effective price precedence:

1. Active trip override.
2. Active boat override for the trip's boat.
3. Base catalog item price.

### Folio Snapshots

`guest_folio_lines` now stores:

- `price_source`: `base`, `boat_override`, `trip_override`, or `tip`.
- `price_override_id`: nullable pointer to the override used.

Line totals remain authoritative after insert. Checkout does not reprice.

### Currency Defaults

The migration backfills existing settings with EUR once and changes the
database default to `['USD','EUR']`. `EnsurePaymentSettings` creates new
settings with USD/EUR but does not re-add EUR after an Admin removes it.

## Implementation Plan

### Phase 1: Schema and Store (~35%)

**Files:**
- `internal/store/migrations/0019_price_overrides_currency_defaults.sql`
- `internal/store/catalog_pricing.go`
- `internal/store/catalog.go`
- `internal/store/guest_folios.go`
- `internal/store/payment_settings.go`

**Tasks:**
- [x] Add reversible migration for price overrides, folio price source,
  and currency defaults.
- [x] Add override CRUD/archive store methods.
- [x] Add effective price resolver.
- [x] Make live folio line-add snapshot effective price/source.
- [x] Preserve EUR removability after default setup.

### Phase 2: API (~20%)

**Files:**
- `internal/httpapi/catalog_pricing_handlers.go`
- `internal/httpapi/catalog_handlers.go`
- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/httpapi.go`

**Tasks:**
- [x] Add Admin-only override list/upsert/archive endpoints.
- [x] Include effective price metadata on trip ledger catalog items.
- [x] Include folio line price source metadata in folio responses.
- [x] Audit override upsert/archive events.

### Phase 3: Frontend (~25%)

**Files:**
- `web/src/admin/pages/OrganizationPricing.tsx`
- `web/src/admin/pages/OrganizationPayments.tsx`
- `web/src/admin/pages/TripConsumptionLedger.tsx`
- `web/src/admin/api.ts`
- `web/src/admin/Shell.tsx`
- `web/src/main.tsx`
- `web/src/styles/app.css`

**Tasks:**
- [x] Add Organization → Pricing page.
- [x] Let Admins create boat/trip item overrides and archive them.
- [x] Show effective prices in the live trip ledger.
- [x] Keep EUR enabled by default but removable in Payments.

### Phase 4: Tests and Docs (~20%)

**Files:**
- `internal/httpapi/guest_folio_test.go`
- `internal/testdb/testdb.go`
- `docs/product/organization-admin-user-stories.md`

**Tasks:**
- [x] Test effective price precedence and folio snapshot stability.
- [x] Test USD/EUR defaults and EUR removability.
- [x] Update product docs for pricing override scope.
- [x] Run Go tests, vet, and frontend build.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/pricing/overrides` | GET | List active pricing overrides |
| `/api/admin/pricing/boat-overrides` | PUT | Upsert one boat/item override |
| `/api/admin/pricing/trip-overrides` | PUT | Upsert one trip/item override |
| `/api/admin/pricing/overrides/{id}` | DELETE | Archive an override |
| `/api/admin/trips/{id}/ledger` | GET | Return catalog items with effective trip prices |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0019_price_overrides_currency_defaults.sql` | Create | Price override schema and currency defaults |
| `internal/store/catalog_pricing.go` | Create | Override CRUD and effective price resolver |
| `internal/store/guest_folios.go` | Modify | Snapshot effective prices on line add |
| `internal/store/payment_settings.go` | Modify | Default new settings to USD/EUR while keeping EUR removable |
| `internal/httpapi/catalog_pricing_handlers.go` | Create | Admin pricing override API |
| `web/src/admin/pages/OrganizationPricing.tsx` | Create | Admin pricing configuration UI |
| `web/src/admin/pages/TripConsumptionLedger.tsx` | Modify | Display effective trip prices |
| `docs/product/organization-admin-user-stories.md` | Modify | Move item price overrides into active scope |

## Definition of Done

- [x] New and existing org settings default to USD and EUR once.
- [x] EUR can be removed by an Admin and is not re-added on read.
- [x] Admin can configure boat and trip item price overrides.
- [x] Trip override wins over boat override; boat override wins over base.
- [x] Folio lines snapshot effective price, source, and override id.
- [x] Existing folio lines are not repriced after override changes.
- [x] Directors see effective prices in the live ledger but cannot mutate pricing config.
- [x] Tax, service charge, package pricing, and discounts remain out of scope.
- [x] `go test ./...` passes.
- [x] `go vet ./...` passes.
- [x] `npm run build` passes.

## Security Considerations

- Pricing configuration endpoints are mounted inside the Org Admin route
  group.
- All store methods require `organization_id` and verify target boat,
  trip, and catalog item belong to the org.
- Assigned Cruise Directors can read/use effective prices only through
  existing trip ledger and folio authorization.
- Historical folio lines preserve price snapshots even if an override is
  archived later.

## Dependencies

- Sprint 015: payment settings, folios, checkout settlement snapshots.
- Sprint 019: live consumption ledger and line-write stock posting.
- Existing stored FX rate model; no external FX provider is added.

## References

- `docs/sprints/SPRINT-015.md`
- `docs/sprints/SPRINT-019.md`
- `docs/product/organization-admin-user-stories.md`
