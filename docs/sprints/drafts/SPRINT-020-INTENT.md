# Sprint 020 Intent: Pricing Overrides and Currency Defaults

## Seed

kplan implement pricing configuration, ignoring tax for now.

Current state:

- The platform has base USD catalog pricing.
- Organization payment settings support card fees and settlement
  currencies.
- FX conversion exists for checkout quotes when exchange rates are
  present.

Missing for this sprint:

- Per-boat and per-trip price overrides.
- Consistent USD-to-accepted-currency conversion support for orgs.
- Default accepted currencies should be USD and EUR for all orgs.

Explicitly out of scope after product interview:

- Tax rules.
- Service charge rules.
- Package pricing as a bundled product concept.
- Discounts.

## Context

The product now has the operational pieces that make price configuration
valuable: catalog items, boat inventory, trip lifecycle, live folio lines,
offline checkout, and folio emails. Sprint 019 made the folio a live
ledger and snapshots catalog prices when lines are written. Sprint 020
should extend that snapshot point so the system records the effective
price for the guest, not just the base catalog price.

The accounting model should remain intentionally simple. Catalog prices,
price overrides, and folio totals are all represented in canonical USD
first. Checkout may settle in any organization-supported currency when a
valid FX rate exists. Payment is still processed offline.

Tax and service charge rules are deferred together. Packages and
discounts are also deferred; pricing remains per catalog item.

## Recent Sprint Context

- Sprint 015 added guest folios, offline checkout, payment settings, card
  fees, settlement currency snapshots, and folio email.
- Sprint 018 added trip lifecycle and readiness warnings.
- Sprint 019 made folios live during active trips, moved counted stock
  decrement to line write, and added the mobile-friendly consumption
  ledger shape.

## Relevant Codebase Areas

- `internal/store/catalog.go` owns catalog items and canonical
  `price_usd_cents`.
- `internal/store/guest_folios.go` resolves the current catalog item at
  line-add time and snapshots item name, price, stock mode, and totals.
- `internal/store/payment_settings.go` owns organization settlement
  currencies, card fee basis points, readiness, and default settings.
- `internal/store/fx.go` validates currencies and converts USD cents to
  target minor units from stored exchange rates.
- `internal/httpapi/guest_folio_handlers.go` and catalog/payment handlers
  are the API integration points.
- `web/src/admin/pages/GuestFolio.tsx` and the Sprint 019 trip ledger
  surface are where effective prices and settlement currency warnings
  will surface.
- `web/src/admin/pages/OrganizationPayments.tsx` already exposes
  supported currencies and FX readiness.
- `docs/product/organization-admin-user-stories.md` currently treats
  per-boat/per-trip overrides as a deferred follow-up and should be
  updated when this sprint lands.

## Constraints

- Keep USD as the canonical pricing currency in storage.
- Keep payment processing offline; do not add processors or POS
  confirmation tracking.
- Preserve historical folio lines exactly as charged. Changing a base
  price or override must not mutate existing folio lines.
- Keep organization scoping strict.
- Keep assigned-trip Cruise Director scoping for trip/folio operations.
- Avoid deleting guests or historical consumption data.
- Do not build tax calculations in Sprint 020.
- Avoid making checkout depend on live network FX lookup. Use stored FX
  rates and readiness warnings, consistent with existing code.
- Keep packages and discounts out of Sprint 020.

## Success Criteria

- New and existing organizations default to accepted currencies USD and
  EUR.
- USD remains always supported. EUR is default-enabled but removable by
  an admin.
- Admins can configure per-boat and per-trip USD price overrides for
  catalog items.
- Effective catalog item price resolution is deterministic, tested, and
  snapshots onto folio lines:
  - trip override wins over boat override;
  - boat override wins over base catalog item price;
  - base catalog item price is used when no override applies.
- Checkout in supported non-USD currencies uses stored FX rates, card fee
  settings, and clear readiness warnings when a rate is missing or
  expired.
- Store and HTTP tests cover currency defaults, price precedence, folio
  snapshots, and authorization.

## Open Questions

Resolved by product interview:

1. Service charge is deferred with tax rules.
2. Package pricing is deferred; pricing remains per catalog item.
3. Discounts are out of scope.
4. EUR is default-enabled but removable by admins.
