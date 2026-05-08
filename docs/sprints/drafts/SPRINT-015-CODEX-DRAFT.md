# Sprint 015 Draft: Guest Folio Checkout and Payment Settings

## Overview

Sprint 015 turns the catalog, inventory, FX, and trip-guest foundations
into the first real onboard checkout flow. A Cruise Director can open a
guest's trip folio, add catalog purchases, add a crew-tip line, show the
guest the itemized total, record an offline settlement, close the folio,
and email the guest a receipt-style folio.

This sprint does not process payments online. The real payment happens
outside Liveaboard through a POS terminal, cash handling, bank transfer,
or another offline workflow. Liveaboard records the settlement facts and
snapshots the money math so revenue summaries are reproducible later.
Org Admins configure which settlement currencies are allowed and which
payment methods/fees apply; Cruise Directors execute checkout only for
trips assigned to them.

## Use Cases

1. **Configure payment settings.** Org Admin opens Organization >
   Payments, chooses supported settlement currencies, default
   settlement currency, enabled offline payment methods, and credit-card
   fee percentage.
2. **Start a guest folio.** Cruise Director opens a trip manifest, picks
   a guest, and opens Checkout. The page shows the guest identity, trip
   context, current open folio lines, and totals.
3. **Add catalog purchases.** Cruise Director adds catalog items and
   quantities to the guest folio. Stock-tracked items are checked against
   the trip boat's inventory before close.
4. **Add crew tip.** Cruise Director can add or edit a crew-tip line
   after the guest chooses the amount. The tip is represented as a
   special folio line, not as a mutable catalog item.
5. **Show guest itemization.** Cruise Director can present the line
   items, subtotal, card fee when applicable, settlement currency, FX
   rate metadata, and total due before closing.
6. **Record offline settlement.** Cruise Director selects payment
   method and settlement currency, records optional POS/reference notes,
   and closes the folio after the offline POS/cash workflow succeeds.
7. **Email closed folio.** Closing sends an itemized folio email to the
   guest email on `trip_guests`.
8. **Review closed folio.** Org Admin and assigned Cruise Director can
   view closed folios for a trip/guest; closed folios are immutable
   except for a future void/refund workflow.
9. **Prevent leakage and double charge.** Cruise Directors cannot access
   unassigned trip folios. A guest cannot see or mutate checkout via the
   admin API. Closing the same open folio twice is rejected.

## Architecture

### Core Rules

- Catalog prices remain canonical in USD cents.
- Folio line prices are snapshotted at add time or close time so later
  catalog edits do not alter historical folios.
- Supported settlement currencies are organization-specific. USD is
  always supported; additional currencies must be enabled by Org Admin.
- Exchange conversion uses `exchange_rates` and integer math from
  Sprint 013. Closed folios snapshot rate provider, numerator,
  denominator, rate timestamp, and currency exponent.
- Credit-card fee percentage is an organization payment setting. It is
  snapshotted on closed folios and applied only when selected payment
  method is `card`.
- No online payment processing, card tokens, PANs, CVVs, payment
  gateway sessions, or PCI-sensitive data is stored.
- Folio close is transactional: validate access, lock the folio, compute
  totals, create stock movements, mark closed, and enqueue/send email.
- Closed folios are immutable in Sprint 015. Voids/refunds are deferred.
- Stock-tracked catalog items decrement the trip boat inventory through
  `AdjustStock(... MovementFolioCharge ...)`.
- Tip is a special `line_type = 'crew_tip'` row so it is not confused
  with catalog revenue or stock.
- One open folio per `trip_guest` in Sprint 015. Multiple folios per
  guest/trip are deferred.

### Payment Settings

Add a dedicated settings table rather than expanding `organizations`
with more payment columns:

```sql
organization_payment_settings (
  organization_id uuid primary key references organizations(id) on delete cascade,
  default_currency char(3) not null default 'USD',
  supported_currencies text[] not null default ARRAY['USD'],
  enabled_payment_methods text[] not null default ARRAY['card','cash'],
  card_fee_basis_points integer not null default 0 check (card_fee_basis_points >= 0 and card_fee_basis_points <= 2000),
  folio_email_from_name text null,
  folio_email_footer text null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

Supported payment methods for Sprint 015:

- `card`
- `cash`
- `bank_transfer`
- `other`

`organizations.currency` remains a legacy/display default from earlier
sprints. Sprint 015 should keep reading/writing it on Organization >
Profile, but checkout uses `organization_payment_settings`.

### Folio Schema

```sql
guest_folios (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  guest_user_id uuid null references guest_users(id) on delete set null,
  status text not null check (status in ('open','closed')),
  opened_by_user_id uuid null references users(id) on delete set null,
  closed_by_user_id uuid null references users(id) on delete set null,
  closed_at timestamptz null,
  subtotal_usd_cents bigint not null default 0 check (subtotal_usd_cents >= 0),
  card_fee_usd_cents bigint not null default 0 check (card_fee_usd_cents >= 0),
  total_usd_cents bigint not null default 0 check (total_usd_cents >= 0),
  settlement_currency char(3) null,
  settlement_total_minor bigint null check (settlement_total_minor is null or settlement_total_minor >= 0),
  currency_exponent integer null,
  rate_provider text null,
  rate_numerator bigint null,
  rate_denominator bigint null,
  rate_as_of timestamptz null,
  payment_method text null,
  card_fee_basis_points integer not null default 0,
  offline_reference text null,
  email_send_status text not null default 'not_sent'
    check (email_send_status in ('not_sent','sent','failed')),
  email_last_sent_at timestamptz null,
  email_last_error text null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (trip_guest_id) where status = 'open'
)
```

PostgreSQL does not support partial unique constraints inline; implement
as a partial unique index:

```sql
CREATE UNIQUE INDEX guest_folios_one_open_per_trip_guest_idx
  ON guest_folios(trip_guest_id)
  WHERE status = 'open';
```

```sql
guest_folio_lines (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  folio_id uuid not null references guest_folios(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  catalog_item_id uuid null references catalog_items(id) on delete set null,
  line_type text not null check (line_type in ('catalog_item','crew_tip','custom')),
  item_name text not null,
  quantity integer not null check (quantity > 0),
  unit_price_usd_cents bigint not null check (unit_price_usd_cents >= 0),
  line_total_usd_cents bigint not null check (line_total_usd_cents >= 0),
  stock_mode text not null default 'none',
  sort_order integer not null default 0,
  created_by_user_id uuid null references users(id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (folio_id, line_type) where line_type = 'crew_tip'
)
```

Implement crew-tip uniqueness as a partial unique index.

### Money Math

- Store card fee as basis points to avoid decimal percentage drift.
- Compute card fee in USD cents with round-half-up:
  `fee = round(subtotal_usd_cents * card_fee_basis_points / 10000)`.
- `total_usd_cents = subtotal_usd_cents + fee`.
- Convert `total_usd_cents` to selected settlement currency using the
  existing `LatestExchangeRate` and `ConvertUSDCentsToMinor`.
- USD settlement uses identity conversion.
- Recompute totals inside the close transaction, not from client input.
- Response shapes should include both USD and settlement amounts plus
  currency exponent for display formatting.

### Stock Integration

On close, for each `catalog_item` line whose snapshot `stock_mode =
'counted'`, call `AdjustStock` with:

- `MovementType = MovementFolioCharge`
- `DeltaQuantity = -quantity`
- `SourceType = 'guest_folio_line'`
- `SourceID = line.id`
- `ActorUserID = closed_by_user_id`

If any stock adjustment would go negative, close fails and the folio
remains open. This keeps Sprint 013's negative-stock rule intact.

### Email

Add `KindGuestFolioClosed` templates:

- `internal/email/templates/guest_folio_closed.subject.tmpl`
- `internal/email/templates/guest_folio_closed.txt.tmpl`
- `internal/email/templates/guest_folio_closed.html.tmpl`

Email contents:

- Operator/org name
- Boat, trip itinerary, dates
- Guest name
- Itemized lines
- Subtotal USD
- Card fee when non-zero
- Settlement total and currency
- Payment method
- Offline reference when present
- Footer text from payment settings when configured

No PDF attachment in Sprint 015. A printable/downloadable folio can be a
future browser feature.

### Authorization

- Org Admin can configure payment settings and view folios for all org
  trips.
- Cruise Director can create/update/close folios only for assigned
  trips.
- Cruise Director cannot configure payment settings.
- Guest accounts/sessions do not authorize checkout endpoints in Sprint
  015.
- Store helpers always scope by `organization_id`, `trip_id`, and
  `trip_guest_id`.

### API Endpoints

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/organization/payment-settings` | GET | Org Admin | Read payment settings. |
| `/api/admin/organization/payment-settings` | PATCH | Org Admin | Update supported currencies, default currency, methods, card fee, email footer. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio` | GET | Org Admin or assigned Cruise Director | Get or create open folio plus closed summary. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines` | POST | Org Admin or assigned Cruise Director | Add catalog/custom/tip line to open folio. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines/{line_id}` | PATCH | Org Admin or assigned Cruise Director | Edit quantity/amount on open folio line. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines/{line_id}` | DELETE | Org Admin or assigned Cruise Director | Remove open folio line. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/close` | POST | Org Admin or assigned Cruise Director | Close with payment method/currency/reference and send email. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/resend-email` | POST | Org Admin or assigned Cruise Director | Resend closed folio email. |

Close request:

```json
{
  "payment_method": "card",
  "settlement_currency": "EUR",
  "apply_card_fee": true,
  "offline_reference": "POS batch 123"
}
```

Server ignores client totals and recomputes from persisted lines and
settings.

## Implementation Plan

### Phase 1: Schema and Store Layer (~35%)

**Files:**

- `internal/store/migrations/0013_guest_folios.sql` - payment settings,
  folios, folio lines, indexes.
- `internal/store/payment_settings.go` - get/update/default payment
  settings with currency/method validation.
- `internal/store/guest_folios.go` - open folio, line CRUD, close,
  resend email state helpers, list/view helpers.
- `internal/store/guest_folios_test.go` - money math, lifecycle,
  isolation, stock movement, close idempotency.
- `internal/testdb/testdb.go` - truncate new tables in dependency
  order.

**Tasks:**

- [ ] Add migration with payment settings and folio tables.
- [ ] Add default payment settings creation/repair helper.
- [ ] Validate supported currencies through `NormalizeCurrency`.
- [ ] Validate default currency is inside supported currencies.
- [ ] Validate enabled methods against the fixed Sprint 015 method set.
- [ ] Add folio line snapshots for catalog items, tips, and custom
      lines.
- [ ] Add transactional close that recomputes totals and snapshots
      settings/rates.
- [ ] Integrate inventory decrement for counted items.

### Phase 2: Authenticated HTTP API and Email (~25%)

**Files:**

- `internal/httpapi/payment_settings_handlers.go` - admin settings
  endpoints.
- `internal/httpapi/guest_folio_handlers.go` - folio endpoints and view
  shapes.
- `internal/httpapi/httpapi.go` - mount routes under existing admin
  groups.
- `internal/email/templates.go` - add folio email kind.
- `internal/email/templates/guest_folio_closed.*.tmpl` - email
  templates.
- `internal/httpapi/guest_folio_test.go` - HTTP authorization, close,
  email, and validation tests.

**Tasks:**

- [ ] Mount payment settings under RequireOrgAdmin.
- [ ] Mount folio endpoints under authenticated `/api/admin`.
- [ ] Reuse manifest authorization for Org Admin/assigned Cruise
      Director access.
- [ ] Add clear error mapping for unsupported currency, missing FX
      rate, invalid payment method, no folio lines, duplicate close, and
      insufficient stock.
- [ ] Send folio email after successful close and persist send status.
- [ ] Add resend endpoint for failed/missed emails.

### Phase 3: Admin Frontend Settings (~15%)

**Files:**

- `web/src/admin/pages/OrganizationPayments.tsx` - Organization >
  Payments settings page.
- `web/src/admin/pages/Organization.tsx` - keep profile focused on
  name/default display currency.
- `web/src/admin/Shell.tsx` - add Payments child under Organization.
- `web/src/main.tsx` - add route.
- `web/src/admin/api.ts` - payment settings types/wrappers.
- `web/src/styles/app.css` - compact settings form styles as needed.

**Tasks:**

- [ ] Add Organization > Payments child nav visible only to Org Admin.
- [ ] Build supported-currency controls using checkboxes/toggles from
      the supported currency list.
- [ ] Add default settlement currency select.
- [ ] Add enabled payment method toggles.
- [ ] Add credit-card fee percentage input, storing basis points.
- [ ] Show rate readiness hints for enabled non-USD currencies when
      available.

### Phase 4: Director Checkout Frontend (~20%)

**Files:**

- `web/src/admin/pages/GuestFolio.tsx` - guest checkout flow.
- `web/src/admin/pages/TripManifest.tsx` - add Checkout action/link.
- `web/src/admin/api.ts` - folio types/wrappers.
- `web/src/main.tsx` - route for guest folio.
- `web/src/styles/app.css` - folio layout, item table, totals panel,
  close modal.

**Tasks:**

- [ ] Add Checkout action to manifest guest rows.
- [ ] Build folio page with guest/trip context, open line table, add
      catalog item controls, crew-tip controls, and totals.
- [ ] Add payment close panel with method/currency/reference.
- [ ] Show card-fee row only when card payment applies.
- [ ] Show closed folio state and resend email action.
- [ ] Keep the UI dense and usable on tablet/desktop; avoid marketing
      layout.

### Phase 5: Product Docs and Verification (~5%)

**Files:**

- `docs/product/personas.md` - clarify payment-settings vs checkout
  ownership.
- `docs/product/organization-admin-user-stories.md` - add payment
  settings and guest checkout stories.
- `docs/sprints/SPRINT-015.md` - final merged sprint plan.

**Tasks:**

- [ ] Update persona boundaries.
- [ ] Update product backlog.
- [ ] Run `npm run build`.
- [ ] Run `go vet ./...`.
- [ ] Run `go test ./...`.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0013_guest_folios.sql` | Create | Payment settings, folios, folio lines. |
| `internal/store/payment_settings.go` | Create | Org payment settings CRUD/defaults. |
| `internal/store/guest_folios.go` | Create | Folio lifecycle, line CRUD, close transaction. |
| `internal/store/guest_folios_test.go` | Create | Store lifecycle, money, stock, isolation tests. |
| `internal/httpapi/payment_settings_handlers.go` | Create | Org Admin payment settings API. |
| `internal/httpapi/guest_folio_handlers.go` | Create | Staff folio checkout API. |
| `internal/httpapi/guest_folio_test.go` | Create | HTTP authorization and close-flow tests. |
| `internal/httpapi/httpapi.go` | Modify | Mount settings and folio routes. |
| `internal/email/templates.go` | Modify | Add folio email kind. |
| `internal/email/templates/guest_folio_closed.*.tmpl` | Create | Closed folio email templates. |
| `internal/testdb/testdb.go` | Modify | Truncate folio tables. |
| `web/src/admin/api.ts` | Modify | Payment settings and folio API wrappers. |
| `web/src/admin/Shell.tsx` | Modify | Add Organization > Payments child nav. |
| `web/src/main.tsx` | Modify | Register Payments and Folio routes. |
| `web/src/admin/pages/OrganizationPayments.tsx` | Create | Payment settings UI. |
| `web/src/admin/pages/GuestFolio.tsx` | Create | Director checkout UI. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Add Checkout action. |
| `web/src/styles/app.css` | Modify | Settings and checkout styles. |
| `docs/product/personas.md` | Modify | Payment settings/checkout boundaries. |
| `docs/product/organization-admin-user-stories.md` | Modify | Payment and checkout stories. |

## Definition of Done

- [ ] Org Admin can read and update payment settings from Organization >
      Payments.
- [ ] Payment settings include supported currencies, default settlement
      currency, enabled offline methods, card fee basis points, and
      folio email footer.
- [ ] Supported currencies are org-specific and validated against
      supported ISO codes.
- [ ] Cruise Director can open checkout only for assigned trips.
- [ ] Org Admin can view/operate checkout for any org trip.
- [ ] Guest/admin auth separation remains intact; guest sessions cannot
      call checkout APIs.
- [ ] Staff can add catalog item lines to an open guest folio.
- [ ] Staff can add/edit one crew-tip line.
- [ ] Server snapshots catalog names, unit prices, stock mode, and line
      totals.
- [ ] Close recomputes totals server-side and rejects client-supplied
      totals.
- [ ] Close snapshots payment method, settlement currency, card fee
      basis points/amount, FX rate metadata, total USD, and total
      settlement minor units.
- [ ] Card fee applies only to card payment when selected/enabled.
- [ ] Missing/expired FX rate for selected non-USD currency produces a
      clear validation error.
- [ ] Closing a folio with counted stock lines creates `folio_charge`
      stock movements and rejects negative inventory.
- [ ] Duplicate close is rejected.
- [ ] Closed folios are immutable in Sprint 015.
- [ ] Closed folio email is sent and send status is visible/retryable.
- [ ] Backend tests cover settings validation, role authorization,
      tenant isolation, money rounding, folio lifecycle, stock movement,
      duplicate close, and email status.
- [ ] Product docs reflect payment settings and director checkout
      boundaries.
- [ ] `npm run build` passes.
- [ ] `go vet ./...` passes.
- [ ] `go test ./...` passes.

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Scope grows into payment processing | High | High | Explicitly store offline settlement only; no gateways, card data, or POS integrations. |
| Money math becomes inconsistent | Medium | High | Use integer cents/minor units, basis points, and snapshot all rates/settings on close. |
| Stock decrement races | Medium | High | Reuse `AdjustStock` row locking inside the close transaction or a tightly coordinated transaction. |
| Director closes wrong guest | Medium | Medium | Strong trip/guest context in UI and route/store scoping by org/trip/trip_guest. |
| Closed folio edits corrupt audit trail | Medium | High | Make closed folios immutable; defer void/refund workflow. |
| Email failure loses receipt | Medium | Medium | Persist send status/error and provide resend endpoint. |
| Supported currencies diverge from FX rates | Medium | Medium | Settings validates currency codes; checkout validates available current rate at close. |
| Tip modeling becomes accounting-specific too early | Medium | Medium | Use special line type with clear snapshot fields; deeper accounting categories deferred. |

## Security Considerations

- Do not store card numbers, CVV, magnetic stripe data, payment tokens,
  gateway IDs, or POS device credentials.
- Treat offline reference as operator-entered text; cap length and do
  not log it.
- Scope every folio query/mutation by `organization_id`, `trip_id`, and
  `trip_guest_id`.
- Enforce Org Admin-only payment settings endpoints in API, not just
  UI.
- Enforce assigned Cruise Director access through
  `trip_cruise_directors`.
- Do not expose folio APIs to guest sessions.
- Do not log folio email bodies, payment references, or raw guest data.
- Email contains operational receipt data; avoid sensitive registration
  details.
- Preserve immutable closed folio snapshots for auditability.

## Dependencies

- Sprint 013 catalog, inventory, stock movement, FX, and checkout quote
  foundations.
- Sprint 014 `trip_guests` manifest and assigned-director access
  patterns.
- Existing local email sender/templates.
- Local PostgreSQL test database.

## Open Questions

- Should Org Admins also be able to operate checkout, or only view and
  resend closed folio emails? This draft allows operation for any org
  trip for support/admin override.
- Should card fee be mandatory for card payments or configurable per
  close? This draft supports `apply_card_fee` so a director can waive it
  when the operator decides.
- Should custom non-catalog lines ship in Sprint 015? This draft allows
  a `custom` line type but the implementation could omit frontend
  controls if scope needs tightening.
- Should folio email send happen inside the close request or via an
  outbox table? Existing code sends emails synchronously; this draft
  follows that pattern and persists failure state.
- Should FX rates move from Inventory > FX Rates to Organization >
  Payments? This draft keeps rate entry where it exists and adds rate
  readiness hints under Payments.
