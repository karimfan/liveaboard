# Sprint 013 Intent: Catalog, Inventory, and Checkout Currency Foundation

## Seed

kplan: finish catalog and inventory functionality. propose all features for these verticals. the inventory will include things like beverages: beer by bottle/can, wine by glass/bottle, massage. laundry and any other items you can find from liveaboards and your own research. pricing should be in USD and we will have to support currency conversion on guest checkout.

## Context

- The app is a Go + PostgreSQL backend with a React/Vite admin SPA for liveaboard operators. Current implemented surfaces include auth, organization profile/currency, fleet/trips, user management, trip import, and Cruise Director assignment.
- The Organization Admin backlog already defines catalog as admin-owned and inventory as per-boat stock tracking, but catalog/inventory are still placeholders in the UI.
- `web/src/admin/pages/Inventory.tsx` is currently an empty state for org-level items, categories, prices, and per-boat stock. `web/src/admin/pages/BoatTabs.tsx` has a Boat Inventory placeholder.
- The current org model has a nullable `organizations.currency`; the user is now asking for catalog pricing in USD with guest checkout conversion, which is a change from the earlier "org default currency owns prices" backlog assumption.
- Recent code contains comments and commits labeled Sprint 013 for 1:N Cruise Director assignment, but sprint docs only go through Sprint 012. This plan uses `SPRINT-013.md` because the sprint ledger is doc-based.

## Recent Sprint Context

- **Sprint 010** renamed Site Director to Cruise Director, enriched invitations with name/phone, and added a real Cruise Director landing page plus profile editing.
- **Sprint 011** completed session chrome with profile/sign-out affordances and refreshed the visual design with a sea-gradient page background while keeping admin surfaces utilitarian.
- **Sprint 012** moved trip import into the product with liveaboard.com import jobs and CSV/XLSX spreadsheet preview/commit. It added `trips.num_guests` and continued the pattern of source-owned vs operator-owned data.

## Relevant Codebase Areas

- `internal/store/migrations/` - add catalog, price, stock, stock movement, and FX tables.
- `internal/store/` - add catalog/category/item/boat stock/price/FX query helpers and tests.
- `internal/httpapi/` - mount org-admin-only catalog/inventory APIs and expose read-only active catalog to Cruise Directors later.
- `web/src/admin/pages/Inventory.tsx` - replace org-level placeholder with catalog and stock workbench.
- `web/src/admin/pages/BoatTabs.tsx` - replace per-boat inventory placeholder with focused stock table for one boat.
- `web/src/admin/api.ts` - add typed wrappers for catalog, inventory, and exchange-rate endpoints.
- `docs/product/organization-admin-user-stories.md` - update catalog/inventory story scope and currency decision.

## Research Notes

- Liveaboard onboard charge examples include dive gear rental, Nitrox, dive courses/instruction, retail sales, soft drinks, juices, alcoholic beverages, spa treatments, WiFi, park/harbor/fuel fees, corkage, transfers, land tours, and gratuities. Examples: Explorer Ventures onboard charges, Aggressor equipment rental/Nitrox pricing, Emperor boat pages, and public liveaboard listings.
- Concrete item examples from research: Nitrox per fill/day/week, full gear set, regulator, BCD, dive computer, wetsuit, torch/night light, larger tank, beer, wine glass/bottle, spirits/cocktails, soft drinks, fresh juice, WiFi week code, laundry per item, massage/spa treatment, park/harbor fees, fuel surcharge, gratuity, corkage fee.
- Currency conversion should not use ECB public reference rates as direct transaction rates without care; ECB says their reference rates are informational. For checkout, store quoted conversion snapshots and provider metadata so historical receipts are reproducible.

## Constraints

- Must follow project conventions in `CLAUDE.md`, `DESIGN.md`, and the sprint template.
- Must integrate with existing org-scoped RBAC: Org Admin manages catalog/inventory; Cruise Director will eventually sell/consume items during trips.
- Must preserve future ledger integrity: catalog items and prices should be archived/versioned, not destructively rewritten in ways that break historical tabs.
- Prices should be stored in integer minor units, not floats.
- USD should become the canonical catalog base currency for this sprint. Guest checkout conversion must be modeled, but payment processing itself can remain out of scope.
- Inventory must support both countable goods and non-stocked services. Beer by can/bottle and wine by glass/bottle are separate SKUs because their units and stock depletion differ.

## Success Criteria

- Org Admin can create, view, edit, deactivate, and reactivate catalog categories and items.
- Seed defaults cover realistic liveaboard verticals: bar, soft drinks, wine, spirits, dive services, equipment rental, retail, spa, laundry, WiFi, fees, gratuities, and miscellaneous services.
- Each catalog item has a unit, tax/fee/service type, USD price, stock tracking mode, and active status.
- New organizations start with a default liveaboard catalog rather than an empty catalog.
- Org Admin can set per-boat stock quantities, reorder/minimum levels, and stock adjustment notes for countable SKUs.
- Per-boat inventory views show low-stock and out-of-stock state.
- The API can produce checkout quotes in guest-selected currency with stored exchange-rate snapshots and rounding.
- The inventory model supports future Cruise Director folio entry: adding a stock-tracked item to a guest folio automatically reduces inventory through the same stock movement mechanism.
- Existing tests/build continue to pass in an unrestricted local environment.

## Open Questions

- Should categories support tax/gratuity defaults now, or should tax/service-charge rules wait for the ledger/checkout sprint?

## Interview Decisions

- Build the checkout quote API in this sprint.
- Inventory must support both manual admin adjustments and automatic future decrements when a Cruise Director adds stock-tracked catalog items to a guest folio.
- Merchandise is in scope as stock-tracked retail inventory, including t-shirts, hoodies, mugs, rash guards, sunscreen, logbooks, dry bags, and similar onboard shop items.
- New organizations should start with a default liveaboard catalog.
- USD is the canonical catalog price currency; `organizations.currency` remains a display/default checkout preference.
