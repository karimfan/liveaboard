# Sprint 015 Intent: Guest Folio Checkout and Payment Settings

## Seed

kplan we next need to work on the guest checkout flow. this is a flow
that is done by the director. the director will need to show the guest
what they purchased. the guest can add a line item for crew tips as
well. and the flow should support payments in different currencies. the
currencies supported are org specific, so we will need to add that as
well under the org settings (the admin is the only role that can set
those). the org settings shoudl also have other payment related
settings like credit card transaction fees in %. Perhaps we add a sub
menu under org for payments. Note that we will not actually process the
payment. payment processing will be done offline via a POS machine. We
will need to close the guest transaction and email them a folio too.

## Context

The app now has the pieces needed for a first real checkout workflow:
trip guests and registration from Sprint 014, catalog/inventory/FX quote
foundations from Sprint 013, and imported trip expected counts from
Sprint 012. Sprint 015 should connect those foundations into a
Cruise-Director-operated guest folio close flow.

This sprint should not introduce online card processing, payment
gateway integration, tax modeling, cabin assignment, or a broad guest
portal. Payment happens offline through a POS machine. Liveaboard should
record the settlement details, apply the configured payment/currency
math, close the folio, decrement stock for stock-tracked items, and
email the guest a folio.

## Recent Sprint Context

- Sprint 012 added native trip import and `trips.num_guests` as an
  expected/imported guest count. It is not capacity.
- Sprint 013 added org-owned catalog items with canonical USD-cent
  prices, per-boat inventory, append-only stock movements, manual FX
  rates, and persisted checkout quote snapshots. It reserved
  `folio_charge` and `folio_void` stock movement types but deliberately
  deferred payment, receipts, guest checkout UI, and folio posting.
- Sprint 014 added `trip_guests`, separate guest accounts/sessions,
  guest registration draft/submit, staff manifest status/detail views,
  and assigned-director trip scoping.

## Relevant Codebase Areas

- `internal/store/migrations/0011_catalog_inventory_fx.sql` - catalog,
  inventory, `exchange_rates`, `checkout_quotes`, and quote-line schema.
- `internal/store/checkout_quotes.go` - persisted USD-to-target quote
  creation and line price snapshots.
- `internal/store/fx.go` - supported currency list, currency exponent,
  rate lookup, and integer conversion.
- `internal/store/inventory.go` - stock movement service with
  `folio_charge`/`folio_void`.
- `internal/store/trip_guests.go` and `internal/httpapi/guest_manifest_handlers.go`
  - trip guest manifest rows and Org Admin/assigned Cruise Director
  access rules.
- `internal/email/templates.go` and `internal/email/templates/*` - email
  template pattern for verification, staff invite, guest invite, and
  password reset.
- `internal/httpapi/fx_handlers.go` - admin FX endpoints and
  authenticated quote endpoint.
- `internal/httpapi/admin.go` and `web/src/admin/pages/Organization.tsx`
  - org profile/settings surface. Today only name/default currency is
  editable; payment settings should live under Organization, likely as a
  child route/sub-menu.
- `web/src/admin/pages/TripManifest.tsx` and `web/src/admin/pages/Trips.tsx`
  - current manifest entry points where a director can find guests.
- `web/src/admin/Shell.tsx` and `web/src/main.tsx` - admin nav and route
  registration.
- `web/src/admin/api.ts` and `web/src/lib/api.ts` - typed API wrappers.

## Constraints

- Must follow project conventions in `CLAUDE.md`.
- Must integrate with existing architecture and sprint conventions in
  `docs/sprints/README.md`.
- Must preserve strict tenant isolation with explicit `organization_id`
  scoping.
- Must preserve persona boundaries from `docs/product/personas.md`:
  Org Admin configures payment settings; Cruise Director performs
  guest checkout only for assigned trips; guests do not enter the admin
  chrome.
- Catalog prices remain canonical in USD cents.
- FX conversion must use persisted integer snapshots; no float math for
  money.
- Supported checkout currencies are organization-specific, not a global
  open list.
- Credit-card transaction fee percentage is an org setting. The plan
  should decide how to snapshot it on closed folios.
- Payment processing is explicitly offline through a POS machine. No
  card tokens, payment gateway, web checkout, or PCI-sensitive storage.
- Guest tip is allowed as a checkout line. The plan should decide
  whether it is represented as a special folio line type, a default
  catalog gratuity item, or another mechanism.
- Closing a folio must be durable and auditable. Closed folios should
  not be silently mutable.
- Stock-tracked catalog lines should decrement boat inventory through
  the existing stock movement service.
- Emailing the folio should reuse local email template patterns and
  avoid logging sensitive payment references.
- UI should remain dense and operational per `DESIGN.md`.

## Success Criteria

- Org Admin can configure payment settings, including supported
  checkout currencies and credit-card fee percentage, from an
  Organization > Payments surface.
- Cruise Director can open an assigned trip guest checkout, add catalog
  line items, add/edit a crew-tip line, show a clear folio summary to
  the guest, select offline payment method/currency, and close the
  transaction.
- Closing records immutable snapshots: catalog item names/prices,
  quantities, tip amount, selected payment method, settlement currency,
  exchange-rate metadata, configured fee percentage/amount, total due,
  and actor.
- Closing stock-tracked lines decrements the trip boat inventory with
  auditable `folio_charge` movements.
- Closed folio can be viewed by staff and is emailed to the guest.
- Org Admin and assigned Cruise Director authorization is correct;
  Cruise Directors cannot access unassigned trips or mutate org payment
  settings.
- Backend tests cover money math, settings validation, role/tenant
  isolation, folio lifecycle, duplicate close prevention, inventory
  movement, and email send behavior.
- Frontend build and Go tests pass.

## Open Questions

- Should Sprint 015 include only one closed folio per `trip_guest`, or
  should it support multiple closed transactions per guest/trip?
- Should a guest be allowed to add the crew-tip line directly on a
  guest-facing screen, or should the Cruise Director enter it after the
  guest tells them the amount?
- Which offline payment methods should be supported initially:
  credit card, cash, bank transfer, room/account transfer, or custom?
- Should the configured credit-card fee apply only when payment method
  is credit card, and should it be optional per transaction?
- Should FX rates remain managed in Inventory > FX Rates, or should
  Organization > Payments also surface the org-supported currencies and
  rate readiness?
- What minimum folio email contents are required for operators: line
  details only, payment settlement details, legal footer, or CSV/PDF
  attachment later?
