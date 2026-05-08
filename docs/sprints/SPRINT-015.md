# Sprint 015: Guest Folio Checkout and Payment Settings

## Overview

Sprint 015 builds the first end-of-trip guest checkout flow. A Cruise
Director opens a guest's folio from the trip manifest, reviews and
corrects the items the guest purchased or consumed, optionally adds a
crew-tip line if the guest asks for one, shows the guest the final
itemization, records that the guest paid offline, closes the folio, and
emails the folio to the guest.

This sprint does not process payments. The actual payment happens
outside Liveaboard through a POS machine, cash handling, or another
offline method. Liveaboard records only the closed/paid folio state,
the selected method, and immutable money snapshots needed for operator
records. Org Admins configure supported checkout currencies and card
transaction fees under Organization > Payments; Cruise Directors cannot
override those settings.

## Use Cases

1. **Configure payment settings.** Org Admin opens Organization >
   Payments, enables supported settlement currencies, chooses the
   default settlement currency, enables payment methods, and sets the
   credit-card transaction fee percentage.
2. **Open guest checkout.** Cruise Director opens an assigned trip
   manifest, selects a guest, and opens Checkout. Org Admin can do the
   same for any organization trip.
3. **Review and correct purchases.** Staff adds catalog items and
   quantities to the guest folio and can correct quantity errors before
   close.
4. **Add optional crew tip.** If the guest asks to add a tip, staff adds
   one crew-tip line. There is no guest-facing tip-entry screen in
   Sprint 015.
5. **Show final itemization.** Staff shows the guest line items,
   subtotal, automatic card fee when paying by card, settlement
   currency, FX metadata when applicable, and total due.
6. **Close as paid.** Staff selects payment method (`card`, `cash`, or
   `other`) and settlement currency, then closes the folio after the
   offline payment is handled. Liveaboard does not store POS
   confirmation or transaction reference data.
7. **Email folio.** Closing emails the itemized folio to the guest
   email on `trip_guests`. Email failure does not undo a closed folio;
   staff can resend.
8. **View closed folio.** Org Admin and assigned Cruise Director can
   view the closed folio. Closed folios are immutable in Sprint 015.

## Architecture

### Core Rules

- Checkout is one end-of-trip event per guest/trip. The database
  enforces one folio per `trip_guest`.
- Catalog prices remain canonical in USD cents.
- Folio lines snapshot item names, unit prices, stock mode, quantities,
  and line totals so later catalog edits do not alter history.
- Supported settlement currencies are organization-specific. USD is
  always supported.
- FX conversion uses Sprint 013 integer math and exchange-rate
  snapshots. Closed folios snapshot provider, numerator, denominator,
  rate timestamp, settlement currency, currency exponent, and totals.
- Card transaction fee is stored as org-level basis points and applied
  automatically when payment method is `card`. Directors cannot waive
  it.
- The client never supplies trusted totals or fee decisions. Close
  recomputes everything server-side.
- No online payment processing, POS integration, card numbers, CVV,
  payment tokens, or payment confirmation references are stored.
- Stock-tracked catalog lines decrement the trip boat inventory through
  `folio_charge` stock movements.
- Folio close is atomic: line totals, card fee, FX snapshot, stock
  movements, and `status = closed` commit or roll back together.
- Closed folios are immutable. Voids/refunds are deferred.
- Sprint 013 `checkout_quotes` remains a generic quote snapshot/dev
  helper. Guest folios are the authoritative closed settlement record
  and do not depend on `checkout_quotes`.

### Payment Settings

Create a dedicated org-owned settings table:

```sql
organization_payment_settings (
  organization_id uuid primary key references organizations(id) on delete cascade,
  default_currency char(3) not null default 'USD',
  supported_currencies text[] not null default ARRAY['USD'],
  enabled_payment_methods text[] not null default ARRAY['card','cash','other'],
  card_fee_basis_points integer not null default 0
    check (card_fee_basis_points >= 0 and card_fee_basis_points <= 2000),
  folio_email_footer text null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

Supported payment methods for Sprint 015:

- `card`
- `cash`
- `other`

`organizations.currency` remains the older display/default currency
field. Payment settings initialize `default_currency` from
`organizations.currency` when present, otherwise `USD`; after creation,
checkout reads from `organization_payment_settings`.

The payment settings response should include rate readiness for enabled
non-USD currencies using the current `exchange_rates` table so admins
can see whether checkout will be able to close in each currency.

### Folio Schema

```sql
guest_folios (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  guest_user_id uuid null references guest_users(id) on delete set null,
  status text not null check (status in ('open','closed')),
  opened_by_user_id uuid not null references users(id) on delete restrict,
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
  payment_method text null check (payment_method is null or payment_method in ('card','cash','other')),
  card_fee_basis_points integer not null default 0,
  email_send_status text not null default 'not_sent'
    check (email_send_status in ('not_sent','sent','failed')),
  email_last_sent_at timestamptz null,
  email_last_error text null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

Indexes:

```sql
CREATE UNIQUE INDEX guest_folios_one_per_trip_guest_idx
  ON guest_folios(trip_guest_id);

CREATE INDEX guest_folios_org_trip_idx
  ON guest_folios(organization_id, trip_id);
```

```sql
guest_folio_lines (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  folio_id uuid not null references guest_folios(id) on delete cascade,
  catalog_item_id uuid null references catalog_items(id) on delete set null,
  line_type text not null check (line_type in ('catalog_item','crew_tip')),
  item_name text not null,
  quantity integer not null check (quantity > 0),
  unit_price_usd_cents bigint not null check (unit_price_usd_cents >= 0),
  line_total_usd_cents bigint not null check (line_total_usd_cents >= 0),
  stock_mode text not null default 'none' check (stock_mode in ('none','counted')),
  sort_order integer not null default 0,
  created_by_user_id uuid null references users(id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (line_type <> 'crew_tip' or quantity = 1),
  check (line_type <> 'catalog_item' or catalog_item_id is not null)
)
```

Indexes:

```sql
CREATE UNIQUE INDEX guest_folio_lines_one_tip_idx
  ON guest_folio_lines(folio_id)
  WHERE line_type = 'crew_tip';

CREATE INDEX guest_folio_lines_folio_idx
  ON guest_folio_lines(folio_id, sort_order);
```

### Money Math

Store card fee as basis points. If `payment_method = 'card'`, compute
the fee in USD cents with round-half-up integer math:

```go
fee = (subtotalUSDCents*cardFeeBasisPoints + 5000) / 10000
```

For `cash` and `other`, fee is zero. Then:

```go
totalUSDCents = subtotalUSDCents + fee
```

Convert `totalUSDCents` to the selected settlement currency through
`LatestExchangeRate("USD", currency, now)` and
`ConvertUSDCentsToMinor`. USD settlement uses the existing identity
rate behavior. Missing or expired rates for enabled non-USD currencies
produce a clear validation error at close.

### Atomic Close and Stock

`CloseGuestFolio` must own one database transaction:

1. Validate staff access and load `trip_guests` by
   `(organization_id, trip_id, trip_guest_id)`.
2. Lock the folio row `FOR UPDATE`; reject if already closed.
3. Load and lock the trip row to get `boat_id`.
4. Load payment settings and validate selected payment method/currency.
5. Load folio lines and recompute subtotal.
6. Compute card fee, total USD, FX snapshot, and settlement total.
7. For each counted catalog line, lock the corresponding
   `boat_inventory_items` row and insert a `stock_movements` row with:
   - `movement_type = 'folio_charge'`
   - `delta_quantity = -quantity`
   - `source_type = 'guest_folio_line'`
   - `source_id = guest_folio_lines.id`
8. Reject negative stock before any commit.
9. Mark the folio closed with all snapshots.
10. Commit.
11. Send folio email after commit. If email fails, keep the folio
    closed and record `email_send_status = 'failed'`.

This must not call the existing `AdjustStock` helper in a loop, because
that helper owns its own transaction and would allow partial stock
decrements.

### Email

Add `KindGuestFolioClosed` templates:

- `internal/email/templates/guest_folio_closed.subject.tmpl`
- `internal/email/templates/guest_folio_closed.txt.tmpl`
- `internal/email/templates/guest_folio_closed.html.tmpl`

Email includes:

- Organization name
- Boat, trip itinerary, and dates
- Guest name
- Itemized lines
- Subtotal USD
- Card fee when non-zero
- Settlement total and currency
- Payment method
- Optional org footer text

No PDF attachment in Sprint 015.

### Authorization

- Org Admin can configure payment settings and operate/view checkout for
  any organization trip.
- Cruise Director can operate/view checkout only for assigned trips.
- Cruise Director cannot configure payment settings or waive card fees.
- Guest sessions do not authorize checkout endpoints.
- Store helpers must scope every folio read/mutation by
  `organization_id`, `trip_id`, and `trip_guest_id`; route-level
  manifest authorization is not enough by itself.

## Implementation Plan

### Phase 1: Schema and Store Layer (~40%)

**Files:**

- `internal/store/migrations/0013_guest_folios.sql`
- `internal/store/payment_settings.go`
- `internal/store/guest_folios.go`
- `internal/store/guest_folios_test.go`
- `internal/testdb/testdb.go`

**Tasks:**

- [ ] Add payment settings, folio, and folio-line tables.
- [ ] Add one-folio-per-`trip_guest` and one-tip-line indexes.
- [ ] Add default payment settings creation/repair helper.
- [ ] Validate currencies with `NormalizeCurrency`; validate close-time
      rate availability separately.
- [ ] Validate default currency is enabled.
- [ ] Validate enabled methods are `card`, `cash`, and/or `other`.
- [ ] Add explicit org/trip/trip_guest ownership checks.
- [ ] Add open folio creation through explicit POST semantics.
- [ ] Add catalog line and crew-tip line CRUD for open folios.
- [ ] Add `CloseGuestFolio` transaction with stock movement SQL inside
      the same transaction.
- [ ] Persist closed snapshots and prevent mutation after close.

### Phase 2: HTTP API and Email (~25%)

**Files:**

- `internal/httpapi/payment_settings_handlers.go`
- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/email/templates.go`
- `internal/email/templates/guest_folio_closed.subject.tmpl`
- `internal/email/templates/guest_folio_closed.txt.tmpl`
- `internal/email/templates/guest_folio_closed.html.tmpl`
- `internal/httpapi/guest_folio_test.go`

**API Endpoints:**

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/organization/payment-settings` | GET | Org Admin | Read payment settings with rate readiness. |
| `/api/admin/organization/payment-settings` | PATCH | Org Admin | Update currencies, methods, card fee, footer. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio` | GET | Org Admin or assigned Cruise Director | Read existing folio, 404 if absent. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio` | POST | Org Admin or assigned Cruise Director | Open the one guest/trip folio. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines` | POST | Org Admin or assigned Cruise Director | Add catalog or crew-tip line. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines/{line_id}` | PATCH | Org Admin or assigned Cruise Director | Edit open line quantity/amount. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/lines/{line_id}` | DELETE | Org Admin or assigned Cruise Director | Remove open line. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/close` | POST | Org Admin or assigned Cruise Director | Close as paid and send email. |
| `/api/admin/trips/{id}/guests/{guest_id}/folio/resend-email` | POST | Org Admin or assigned Cruise Director | Resend closed folio email. |

Close request:

```json
{
  "payment_method": "card",
  "settlement_currency": "EUR"
}
```

**Tasks:**

- [ ] Mount payment settings under RequireOrgAdmin.
- [ ] Mount folio endpoints under authenticated `/api/admin`.
- [ ] Reuse `authorizeManifestAccess`, then verify `trip_guest_id`
      belongs to the same org/trip in store helpers.
- [ ] Add errors for unsupported currency, currency not enabled,
      missing FX rate, invalid payment method, no folio lines, duplicate
      folio, duplicate close, closed-folio mutation, and insufficient
      stock.
- [ ] Send folio email after successful close and persist sent/failed
      status.
- [ ] Add resend endpoint and tests.

### Phase 3: Organization Payments UI (~15%)

**Files:**

- `web/src/admin/pages/OrganizationPayments.tsx`
- `web/src/admin/pages/Organization.tsx`
- `web/src/admin/Shell.tsx`
- `web/src/main.tsx`
- `web/src/admin/api.ts`
- `web/src/styles/app.css`

**Tasks:**

- [ ] Add Organization > Payments child route visible only to Org
      Admin.
- [ ] Keep Organization > Profile focused on name/display currency.
- [ ] Build supported-currency controls.
- [ ] Add default settlement currency select.
- [ ] Add payment method toggles for card, cash, and other.
- [ ] Add card transaction fee percentage input, stored as basis
      points.
- [ ] Show rate readiness for enabled non-USD currencies from the
      payment settings response.

### Phase 4: Director Checkout UI (~15%)

**Files:**

- `web/src/admin/pages/GuestFolio.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [ ] Add Checkout action to manifest guest rows.
- [ ] Build folio page with guest/trip context, line table, add catalog
      item controls, optional crew-tip controls, and totals.
- [ ] Allow line quantity edits before close.
- [ ] Add close panel with payment method and settlement currency.
- [ ] Show automatic card-fee row only when card payment applies.
- [ ] Show closed folio state, email status, and resend action.
- [ ] Keep the UI dense and operational per `DESIGN.md`.

### Phase 5: Product Docs and Verification (~5%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`

**Tasks:**

- [ ] Clarify Org Admin owns payment settings.
- [ ] Clarify Cruise Director owns assigned-trip checkout.
- [ ] Add product stories for payment settings and guest checkout.
- [ ] Run `npm run build`.
- [ ] Run `go vet ./...`.
- [ ] Run `go test ./...`.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0013_guest_folios.sql` | Create | Payment settings, folios, folio lines. |
| `internal/store/payment_settings.go` | Create | Org payment settings CRUD/defaults. |
| `internal/store/guest_folios.go` | Create | Folio line CRUD and atomic close transaction. |
| `internal/store/guest_folios_test.go` | Create | Store lifecycle, money, stock, isolation tests. |
| `internal/httpapi/payment_settings_handlers.go` | Create | Org Admin payment settings API. |
| `internal/httpapi/guest_folio_handlers.go` | Create | Staff checkout API. |
| `internal/httpapi/guest_folio_test.go` | Create | HTTP authorization, close, email tests. |
| `internal/httpapi/httpapi.go` | Modify | Mount settings and folio routes. |
| `internal/email/templates.go` | Modify | Add closed-folio email kind. |
| `internal/email/templates/guest_folio_closed.*.tmpl` | Create | Closed folio email templates. |
| `internal/testdb/testdb.go` | Modify | Truncate new tables. |
| `web/src/admin/api.ts` | Modify | Payment settings and folio wrappers. |
| `web/src/admin/Shell.tsx` | Modify | Organization > Payments child nav. |
| `web/src/main.tsx` | Modify | Payments and folio routes. |
| `web/src/admin/pages/OrganizationPayments.tsx` | Create | Payment settings UI. |
| `web/src/admin/pages/GuestFolio.tsx` | Create | Director checkout UI. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Checkout entry point. |
| `web/src/styles/app.css` | Modify | Settings and checkout styles. |
| `docs/product/personas.md` | Modify | Payment/checkout ownership. |
| `docs/product/organization-admin-user-stories.md` | Modify | Payment/checkout stories. |

## Definition of Done

- [ ] Org Admin can configure payment settings from Organization >
      Payments.
- [ ] Payment settings include supported currencies, default settlement
      currency, enabled methods, card fee basis points, and email
      footer.
- [ ] Initial payment methods are card, cash, and other.
- [ ] Supported currencies are org-specific and close-time FX
      availability is validated.
- [ ] Cruise Director can checkout only guests on assigned trips.
- [ ] Org Admin can operate checkout for any org trip.
- [ ] Guest sessions cannot access checkout APIs.
- [ ] Exactly one folio can exist per guest/trip.
- [ ] Staff can add catalog lines and one optional crew-tip line.
- [ ] Staff can edit line quantities/amounts before close.
- [ ] Server snapshots line names, prices, quantities, stock mode, and
      totals.
- [ ] Close recomputes all totals server-side.
- [ ] Card fee is automatic for card payments and cannot be waived by
      Cruise Directors.
- [ ] Close snapshots payment method, settlement currency, card fee,
      FX rate metadata, total USD, settlement total, actor, and
      timestamp.
- [ ] Closing counted stock lines creates `folio_charge` movements in
      the same transaction.
- [ ] Partial stock failure rolls back the entire close and moves no
      stock.
- [ ] Duplicate close is rejected and does not double-decrement stock.
- [ ] Closed folios are immutable.
- [ ] Closed folio email is sent after close; failure is visible and
      retryable.
- [ ] Backend tests cover settings validation, role authorization,
      tenant isolation, org/trip/guest scoping, money rounding, folio
      lifecycle, stock rollback, duplicate close, email failure/resend,
      and guest-session rejection.
- [ ] Product docs reflect payment settings and director checkout
      boundaries.
- [ ] `npm run build` passes.
- [ ] `go vet ./...` passes.
- [ ] `go test ./...` passes.

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Scope grows into payment processing | High | High | Store only offline paid/closed state; no POS integration or payment credentials. |
| Money math differs by path | Medium | High | Server-side integer math only; snapshot all settings/rates on close. |
| Partial inventory decrement corrupts stock | Medium | High | One `CloseGuestFolio` transaction owns all stock and folio writes. |
| Director closes wrong guest | Medium | Medium | Strong UI context plus store-level org/trip/guest scoping. |
| Card fee is accidentally waived | Medium | Medium | No client fee toggle; automatic server computation from org settings. |
| Email failure blocks checkout | Medium | Medium | Email is post-commit and retryable; closed paid state is preserved. |
| Currency is enabled without rate | Medium | Medium | Settings shows rate readiness; close validates a current rate. |
| Closed folio edits compromise audit trail | Medium | High | Closed folios are immutable; void/refund deferred. |

## Security Considerations

- Do not store card numbers, CVV, payment tokens, gateway IDs, POS
  confirmation numbers, or POS credentials.
- Do not log folio email bodies, payment method details beyond method,
  or guest registration payloads.
- Enforce Org Admin-only payment settings in the API.
- Enforce assigned-trip Cruise Director access through
  `trip_cruise_directors`.
- Verify `trip_guest_id` belongs to the URL trip and organization in
  store helpers.
- Reject guest sessions on checkout endpoints.
- Keep closed folio snapshots immutable for auditability.
- Avoid sending sensitive registration data in folio emails.

## Dependencies

- Sprint 013 catalog, inventory, stock movement, FX, and quote
  foundations.
- Sprint 014 trip guest manifest and assigned-director authorization
  patterns.
- Existing email sender/template infrastructure.
- Local PostgreSQL test database.

## Follow-Ups

- Void/refund workflow.
- Printable/downloadable folio PDF.
- Guest-facing review/signature screen.
- Payment method reporting and revenue summaries.
- POS integration, if ever needed.
- More payment methods, if operators ask for them.

## References

- `docs/sprints/drafts/SPRINT-015-INTENT.md`
- `docs/sprints/drafts/SPRINT-015-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-015-CODEX-DRAFT-CLAUDE-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-015-MERGE-NOTES.md`
- `docs/sprints/SPRINT-013.md`
- `docs/sprints/SPRINT-014.md`
