# Sprint 020 Codex Draft: Pricing Overrides and Currency Defaults

## Overview

Sprint 020 adds the missing per-item pricing configuration layer between
the catalog and guest folios. The catalog remains the canonical source of
base USD prices, but line-add now resolves an effective price from the
trip context before snapshotting it to the folio. That gives operators
boat-specific and trip-specific pricing without rewriting historical
folios.

Tax, service charges, packages, and discounts are out of scope for this
sprint. Pricing remains per catalog item.

## Use Cases

1. **Default currency readiness.** Every organization starts with USD and
   EUR as accepted currencies. USD always works. EUR checkout is allowed
   when the organization has a valid stored USD-to-EUR exchange rate.
2. **Boat-level override.** Admin sets a higher beer price or rental
   price for a specific boat. All trips on that boat use that price unless
   the trip has its own override.
3. **Trip-level override.** Admin sets a special price for a catalog item
   on one trip. That price wins over boat and base catalog pricing for
   new folio lines on that trip.
4. **Historical safety.** Updating any price configuration changes only
   future folio lines. Existing line snapshots and closed folios remain
   unchanged.

## Data Model

### Currency Defaults

Update `organization_payment_settings` defaults and normalization:

- New orgs get `supported_currencies = ['USD', 'EUR']`.
- Existing settings are backfilled to include EUR once.
- `default_currency` remains the organization currency when valid, else
  USD.
- USD remains always included.
- EUR is default-enabled but removable by admins.

The existing `exchange_rates` table and `ConvertUSDCentsToMinor` flow
remain the conversion mechanism. Sprint 020 should not call an external
FX API.

### Price Overrides

Add a scoped override table:

```sql
CREATE TABLE catalog_price_overrides (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  catalog_item_id uuid NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
  boat_id uuid NULL REFERENCES boats(id) ON DELETE CASCADE,
  trip_id uuid NULL REFERENCES trips(id) ON DELETE CASCADE,
  price_usd_cents integer NOT NULL CHECK (price_usd_cents >= 0),
  notes text NOT NULL DEFAULT '',
  created_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  updated_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz NULL,
  CHECK ((boat_id IS NOT NULL)::int + (trip_id IS NOT NULL)::int = 1)
);
```

Indexes:

- unique active `(organization_id, catalog_item_id, boat_id)` where
  `boat_id IS NOT NULL AND archived_at IS NULL`
- unique active `(organization_id, catalog_item_id, trip_id)` where
  `trip_id IS NOT NULL AND archived_at IS NULL`
- lookup indexes on `(organization_id, trip_id)` and
  `(organization_id, boat_id)`

Effective price resolution:

1. active trip override for `(org, trip, item)`
2. active boat override for `(org, trip.boat_id, item)`
3. base `catalog_items.price_usd_cents`

The resolver should return both the cents value and the source so UI/API
responses can explain why a price is what it is.

### Folio Line Price Source

Add a lightweight price source snapshot to folio lines:

- `price_source text NOT NULL DEFAULT 'base'`
- `price_override_id uuid NULL REFERENCES catalog_price_overrides(id) ON DELETE SET NULL`

Allowed `price_source` values should be `base`, `boat_override`, and
`trip_override`. The `price_override_id` gives staff and future audit
work a durable pointer to the active rule used when the line was written,
while preserving line totals if the override later changes or archives.

## Store Layer

Add pricing helpers near catalog/folio code:

- `ListPriceOverrides(ctx, orgID, filter)`
- `UpsertBoatPriceOverride(ctx, orgID, boatID, itemID, priceCents, notes, actorID)`
- `UpsertTripPriceOverride(ctx, orgID, tripID, itemID, priceCents, notes, actorID)`
- `ArchivePriceOverride(ctx, orgID, overrideID, actorID)`
- `EffectiveCatalogItemPrice(ctx, orgID, tripID, itemID)`

`AddGuestFolioLine` should call the effective price resolver instead of
copying `catalog_items.price_usd_cents` directly. It should preserve the
Sprint 019 transaction and idempotency behavior.

`CloseGuestFolio` should continue to calculate settlement totals from
snapshotted folio line totals, card fee settings, and stored FX rates.
No close-time repricing should occur.

## HTTP API

Admin-only pricing configuration:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/pricing/overrides` | GET | Admin | List boat/trip overrides |
| `/api/admin/pricing/boat-overrides` | PUT | Admin | Upsert one boat/item override |
| `/api/admin/pricing/trip-overrides` | PUT | Admin | Upsert one trip/item override |
| `/api/admin/pricing/overrides/{id}` | DELETE | Admin | Archive an override |

Responses that include catalog items for a trip should include effective
price metadata:

```json
{
  "price_usd_cents": 1200,
  "effective_price_usd_cents": 1500,
  "price_source": "trip_override",
  "price_scope_label": "Komodo May 2026"
}
```

## Frontend

### Organization Payments

Update `OrganizationPayments.tsx` so new/read settings show USD and EUR
as default accepted currencies. Keep the existing readiness model:
non-USD currencies show whether a usable exchange rate exists.

Leave EUR enabled by default but removable. USD remains fixed.

### Pricing Configuration

Add a pricing area under the admin organization/catalog surface:

- catalog base prices remain on catalog items;
- boat override table filters by boat and catalog item;
- trip override table filters by upcoming/active trip and catalog item;
- clear effective price preview when viewing a trip/boat context.

Design should match existing dense admin surfaces: compact tables,
filters, inline edit panels or modals, and no marketing-style cards.

### Ledger and Checkout

Update the live ledger/catalog picker so Directors see effective prices
for the current trip.

Update checkout/folio view:

- display line type labels only where helpful;
- show effective price/source metadata only where operationally useful;
- keep settlement currency behavior unchanged except for USD/EUR default
  support and missing-rate warnings.

## Tests

Store tests:

- payment settings ensure/normalize/backfill includes USD and EUR;
- EUR readiness uses stored FX rate state;
- effective price precedence: trip > boat > base;
- folio line snapshots effective price and source;
- changing an override after a line is written does not change the line;
- org and assigned-CD authorization paths remain enforced.

HTTP tests:

- Admin can create/update/archive overrides.
- Director cannot mutate organization-level pricing configuration.
- Missing/expired FX rate gives readiness warning rather than silent
  incorrect conversion.

Frontend checks:

- `npm run build`
- currency defaults render correctly;
- pricing config pages handle empty state, override state, and archived
  state;
- ledger and checkout totals display effective prices correctly on
  desktop and mobile widths.

## Definition of Done

- Migrations are reversible by forward-only archive/nullable strategy and
  preserve existing data.
- Existing organizations have USD and EUR in supported currencies after
  migration or normalization.
- EUR is removable by admins after default setup.
- Price override APIs and UI are admin-only.
- Effective price is used for all future catalog folio lines.
- Non-USD checkout continues to use stored FX rates and snapshots the
  rate used.
- Product docs are updated to move price overrides from deferred/future
  into implemented scope while leaving tax, service charge, packages, and
  discounts deferred.
- `go test ./...` and frontend build pass.
