# Sprint 015 Codex Draft — Claude Critique

Critique of `SPRINT-015-CODEX-DRAFT.md`. This document does not rewrite the
draft; it identifies what to keep, what to fix, and what to reject before the
final sprint document is written.

---

## Strengths Worth Preserving

- **Migration number is correct.** `0013_guest_folios.sql` is the next
  available slot (current high is `0012_guest_registration.sql`).
- **FX function names are accurate.** `LatestExchangeRate`,
  `ConvertUSDCentsToMinor`, and `NormalizeCurrency` all exist with signatures
  compatible with how the draft uses them.
- **`MovementFolioCharge` is correctly referenced.** It exists in
  `internal/store/inventory.go` as a defined constant.
- **Basis points for card fee is sound.** Storing `card_fee_basis_points`
  avoids percentage float drift; the basis-point formula is correct in
  principle.
- **Partial unique index for one open folio per guest.** The draft correctly
  notes PostgreSQL cannot express a WHERE clause on an inline constraint and
  calls for a separate `CREATE UNIQUE INDEX`.
- **`authorizeManifestAccess` reuse.** The function exists in
  `internal/httpapi/guest_manifest_handlers.go` with the right signature and
  the correct Admin/Director branch. Reusing it for folio endpoints is the
  right call.
- **Email template extension pattern.** Adding a new `Kind` constant and three
  `.tmpl` files matches how every prior email kind was added.
- **Immutable closed folios.** Deferring void/refund is correct for Sprint 015.
- **Risk table.** The risk table is thoughtful and realistic. Scope creep into
  payment processing is a real threat and the explicit exclusions are correct.
- **`testdb.go` truncation mention.** The draft correctly identifies this file
  as needing to be updated.

---

## Major Concerns

### 1. `AdjustStock` is not atomic inside the folio close — partial decrements are possible

`AdjustStock` (signature: `func (p *Pool) AdjustStock(...) (*StockMovement, *BoatInventoryItem, error)`)
runs its own internal transaction. The draft says to call it once per
stock-tracked line during close. If the first item decrements and the second
fails (e.g., insufficient stock), the first decrement has already committed.
The folio would be left in a broken state: partially decremented inventory,
folio still open.

The fix is to perform all stock mutations inside a single explicit database
transaction that also writes the folio `status = 'closed'`. This likely means
a dedicated `CloseGuestFolio` function in the store that opens a `BEGIN`,
calls lower-level stock-insertion SQL directly (not through `AdjustStock`),
and commits or rolls back atomically. Alternatively, expose an
`AdjustStockTx` variant that accepts a caller-owned `*sql.Tx`.

This is the highest-priority architectural gap. Without it, any multi-item
folio with a counted item can produce inconsistent ledger state.

### 2. `apply_card_fee: true` in the close request is a business-rule bypass risk

The close request body includes `"apply_card_fee": true`. This means a
Cruise Director can pass `false` and waive the configured card surcharge
without any administrative record of the waiver. The fee should be determined
server-side from `payment_method` and the org's `card_fee_basis_points`; the
client should not be able to suppress it.

If operators need a waiver path, it should be explicitly gated: either Org
Admin only, or require a separate `waive_reason` field that is persisted and
auditable. As written, this is a silent revenue leak vector.

### 3. GET `/folio` that also creates a folio violates HTTP semantics

The draft describes `GET /api/admin/trips/{id}/guests/{guest_id}/folio` as
"Get or create open folio." GET must be safe and idempotent; it must not
create side effects. A repeated GET by a network proxy or browser prefetch
could create spurious open folios.

Preferred approach: `GET` returns 404 if no folio exists. A separate
`POST /api/admin/trips/{id}/guests/{guest_id}/folio` explicitly opens one.
The frontend clicks "Open Checkout" which hits the POST, then navigates to
the folio page which uses GET.

### 4. `checkout_quotes` relationship to `guest_folios` is unaddressed

Sprint 013 introduced `checkout_quotes` and `checkout_quote_lines` as
"persisted USD-to-target quote creation and line price snapshots." Sprint 015
introduces `guest_folios` and `guest_folio_lines` for what appears to be an
overlapping purpose. The draft never explains how these coexist:

- Are checkout quotes superseded and unused once folios exist?
- Does a folio start from a quote, or are they parallel paths?
- Will the Inventory > FX Rates / quote flow remain a distinct entry point?

This must be resolved before implementation. If checkout quotes are now dead
code, say so. If they serve a different workflow, document the boundary.
Leaving two overlapping snapshot schemas without explanation creates
long-term maintenance confusion.

### 5. `boat_id` is missing from the close transaction context

`AdjustStock` requires `(orgID, boatID, itemID uuid.UUID, ...)`. A
`guest_folio` stores `organization_id`, `trip_id`, and `trip_guest_id`, but
not `boat_id`. The folio close must look up `trips.boat_id` to perform the
stock adjustment.

The draft never calls out this lookup step. The implementation plan must
explicitly load the trip (or denormalize `boat_id` on `guest_folios`) inside
the close transaction before calling into the inventory layer.

---

## Schema Issues

### 6. Inline partial unique constraint in CREATE TABLE contradicts the text

The `guest_folios` schema block includes:
```sql
unique (trip_guest_id) where status = 'open'
```
...inside the CREATE TABLE body. The very next paragraph says PostgreSQL does
not support this inline and to use a separate `CREATE UNIQUE INDEX`. The
inline clause is a syntax error in PostgreSQL and should be removed from the
schema DDL shown; only the separate index should appear.

The same issue appears on `guest_folio_lines` for the crew-tip uniqueness:
```sql
unique (folio_id, line_type) where line_type = 'crew_tip'
```
Remove from the CREATE TABLE and implement as an explicit index.

### 7. `guest_folio_lines` denormalized `trip_id` and `trip_guest_id` are unjustified

The lines table carries `trip_id` and `trip_guest_id` which are both
derivable via `folio_id → guest_folios`. Denormalization is sometimes correct
for query performance but the draft never justifies it. These columns add two
FK constraint writes on every line insert and create a potential for
inconsistency if a migration ever needs to move a folio.

Either justify the denormalization (e.g., "required for row-level security
policies") or remove the columns from the schema. The RLS scoping in the store
can be enforced by joining through `guest_folios`.

### 8. `quantity` constraint is wrong for crew-tip lines

The schema requires `quantity integer not null check (quantity > 0)` for all
line types. Crew tips have no meaningful quantity — the tip is an amount. If
quantity must be 1 for tip lines, add a check constraint or application
validation that enforces `quantity = 1 WHERE line_type = 'crew_tip'`.
Without this, a tip line with quantity 3 and unit price X would produce 3X
rather than the intended X.

### 9. `organizations.currency` vs `organization_payment_settings.default_currency` sync is undefined

The draft says `organizations.currency` is a "legacy/display default" and
checkout uses `organization_payment_settings.default_currency`. Nothing
specifies whether these should be kept in sync, which takes precedence in
display contexts, or what happens when a new org is created (does a default
`organization_payment_settings` row get created immediately?).

This needs an explicit policy: either deprecate `organizations.currency`
entirely (requires a migration and UI update), or specify that payment
settings `default_currency` is initialized from `organizations.currency` at
row-creation time and they evolve independently thereafter. Leaving both
writeable and unsynchronized will produce UI inconsistency.

---

## Authorization / Security Gaps

### 10. `trip_guest_id` is not validated as belonging to the trip in folio authorization

`authorizeManifestAccess` checks `organization_id` and `trip_id` access, but
the folio endpoints also take a `{guest_id}` path segment. The implementation
must verify that the `trip_guest` row with that ID belongs to both the
`trip_id` in the URL and the caller's `organization_id`. Without this check, a
Cruise Director assigned to trip A could construct a URL with guest_id from
trip B and access that folio.

The draft's authorization section says "Store helpers always scope by
`organization_id`, `trip_id`, and `trip_guest_id`" but does not make this
an explicit task or test case. It must be an explicit store-level guard, not
just assumed from URL routing.

### 11. Org Admin "operating" checkout is not resolved, but is in the DoD

The "Open Questions" section asks whether Org Admins should operate checkout
or only view. The DoD includes "Org Admin can view/operate checkout for any
org trip" — so the question is already answered in the DoD but marked open
in the questions section. This inconsistency will cause confusion during
implementation. Pick one and remove the open question.

### 12. `offline_reference` length cap is mentioned but not specified

The security section says "cap length" for offline reference text. The
schema has no `CHECK` constraint or length limit. Add an explicit
`VARCHAR(500)` or a `CHECK (char_length(offline_reference) <= 500)` in the
migration. Without it, "cap length" remains aspirational.

---

## Testing Gaps

### 13. Stock partial-failure close path is not a named test case

The DoD says "Closing a folio with counted stock lines creates `folio_charge`
stock movements and rejects negative inventory." But the test tasks don't
include a case for: "folio has two counted items; first succeeds, second
would go negative — folio remains open, no stock moved." This is the exact
partial-failure scenario from Major Concern #1 and must be an explicit test.

### 14. Resend email flow is not tested

The draft lists a `resend-email` endpoint and the DoD says email is
"retryable," but the test tasks (Phase 2) don't include a test for:
`send_status = 'failed'` → resend endpoint → email service called again →
`send_status = 'sent'`. Without this, the resend endpoint ships untested.

### 15. Duplicate close test is listed but not specified

The DoD says "Duplicate close is rejected" but the test should be explicit
about the expected HTTP status and the folio state after the rejected second
close attempt (i.e., folio remains closed, no stock double-decremented). Add
this to the Phase 2 test task list.

### 16. Email failure at close time: blocking vs non-blocking is unspecified

The draft says to "Send folio email after successful close and persist send
status." It does not say whether an email failure rolls back the close
transaction or leaves the folio closed with `send_status = 'failed'`. This
is a behavioral decision that affects test setup and operator UX. Specify it.
(Recommendation: non-blocking — folio closes successfully, email failure is
logged and retryable, so the POS interaction is not held hostage to an SMTP
hiccup.)

---

## Missing Implementation Details

### 17. Round-half-up formula must be specified in Go terms

The draft says `fee = round(subtotal_usd_cents * card_fee_basis_points / 10000)`
with "round-half-up." Go integer division truncates. The correct integer
implementation for round-half-up is:
```go
fee = (subtotal * bps + 5000) / 10000
```
The draft should include this exact formula rather than the pseudo-mathematical
description, because a naive Go port of the formula would produce truncation
instead of rounding.

### 18. `NormalizeCurrency` scope: format validation vs allowed-list validation

The Explore survey confirmed `NormalizeCurrency` uppercases and validates. The
draft says to "Validate supported currencies through `NormalizeCurrency`" for
payment settings. But `NormalizeCurrency` validates ISO-3 format, not whether
the app has FX support for the currency. The store should also validate that
any enabled non-USD currency has a corresponding exchange rate configured, or
at least surface a clear warning. The draft should distinguish between
"is a valid ISO code" (NormalizeCurrency) and "can we actually quote in this
currency at close time" (rate availability check).

### 19. FX rate readiness hint in payment settings requires an implicit API dependency

Phase 3 tasks include "Show rate readiness hints for enabled non-USD
currencies when available." This requires the `OrganizationPayments` page to
call an API that returns current exchange rate status. The files list does not
include a rate-status endpoint or an extension to the payment settings GET
response that includes rate metadata. Either add the endpoint to the API table
and files list, or remove the hint from Phase 3 scope.

### 20. `opened_by_user_id` population is unspecified

`guest_folios.opened_by_user_id` is nullable but the schema implies it should
be set when the folio is created. Under the "get or create" GET pattern (or
a corrected POST-to-open pattern), which user becomes the opener? This should
be specified: the authenticated staff user who triggers folio creation.

---

## Over-scoping Concerns

### 21. `custom` line type in schema without committed frontend scope

The schema includes `line_type = 'custom'` as a valid value. The draft then
hedges: "the implementation could omit frontend controls if scope needs
tightening." Adding a database-level value for a feature with no committed
frontend adds permanent schema weight. Either commit to custom lines (add
frontend tasks) or remove `'custom'` from the `CHECK` constraint and
`line_type` enum. Dead schema values create confusion for future sprints.

### 22. `folio_email_from_name` in payment settings is likely not Sprint 015 scope

The payment settings table includes `folio_email_from_name text null`. The
existing email sender probably has a fixed from-name configured at the
application level. Overriding it per-org introduces complexity (template
rendering, sender display logic) that is not addressed anywhere in the plan.
`folio_email_footer` is useful for legal text and should stay. The from-name
override should be deferred unless there is a concrete operator request for
it in Sprint 015.

### 23. Resend email endpoint may be premature for Sprint 015

The resend endpoint is a useful recovery path, but it adds an endpoint, a
test, and frontend handling for a `send_status` badge and a button. For a
first-pass checkout flow, an operator who misses an email can ask the
director to re-send through a future mechanism. If the close is non-blocking
on email (see Gap #16), the resend can be a Phase 2 item in a later sprint.
Consider removing it from Sprint 015 DoD and noting it as a follow-on.

---

## Suggested Changes Summary

| # | Area | Action |
|---|------|--------|
| 1 | Stock atomicity | Add `CloseGuestFolio` transactional store function; do not use `AdjustStock` in sequence |
| 2 | Card fee toggle | Remove `apply_card_fee` from close request; compute server-side from `payment_method` |
| 3 | GET creates folio | Split into `GET` (read-only, 404 if none) + `POST` (open) |
| 4 | checkout_quotes | Declare relationship explicitly; confirm quotes are superseded or document parallel use |
| 5 | boat_id | Explicitly load `trips.boat_id` in close transaction context; document this in plan |
| 6 | Partial unique syntax | Remove inline WHERE clauses from CREATE TABLE DDL |
| 7 | Line denormalization | Justify or remove `trip_id`/`trip_guest_id` from `guest_folio_lines` |
| 8 | Tip quantity | Add `CHECK (line_type != 'crew_tip' OR quantity = 1)` constraint |
| 9 | Currency sync | Document `organizations.currency` deprecation or sync policy |
| 10 | trip_guest auth | Add explicit store-level org+trip+guest triple-scoping check |
| 11 | Org Admin DoD | Resolve open question; remove from open questions once decided |
| 12 | Reference length | Add explicit length constraint in migration |
| 13 | Partial stock failure | Add explicit test case for multi-item partial failure |
| 14 | Resend test | Add resend flow test case |
| 15 | Email blocking | Specify and test non-blocking email behavior |
| 16 | Go rounding formula | Use `(subtotal * bps + 5000) / 10000` in plan |
| 17 | Currency validation | Distinguish format validation from rate-availability validation |
| 18 | Rate hint | Either add rate-status endpoint to files list or drop from Phase 3 |
| 19 | opened_by | Specify who becomes `opened_by_user_id` in the open flow |
| 20 | Custom line type | Commit with frontend or remove from schema |
| 21 | from_name | Remove `folio_email_from_name` from Sprint 015 scope |
| 22 | Resend endpoint | Consider deferring to a follow-on sprint |

---

## Parts of the Codex Draft That Should Be Rejected or Simplified

1. **The `apply_card_fee` request field.** Remove entirely. Fee computation
   is server-side and not client-overridable without an explicit Org Admin
   waiver path.

2. **GET-to-create semantics.** The "get or create" GET endpoint violates
   HTTP safety. Replace with a proper POST to open a folio.

3. **`folio_email_from_name` in payment settings.** Drop from Sprint 015.
   The existing sender has a fixed from-name and overriding it per-org is
   not justified by the intent.

4. **`custom` line type in schema without frontend.** Either implement with
   frontend or drop from the schema. Do not add dead schema values.

5. **Inline partial unique constraint syntax in DDL blocks.** Remove the
   WHERE clauses from the CREATE TABLE examples; keep only the explicit
   CREATE UNIQUE INDEX forms.

---

## Risks the Final Merge Must Address

- **Atomicity of folio close with stock decrements.** If not addressed, the
  first multi-item checkout with inventory tracking will produce a corrupted
  ledger state with no clean recovery path.
- **`boat_id` lookup in close path.** If the implementation blindly follows
  the draft without this lookup, `AdjustStock` calls will fail or use a zero
  UUID for `boatID`, silently adjusting the wrong inventory.
- **`checkout_quotes` architectural ambiguity.** If not resolved, future
  sprints will encounter two overlapping snapshotting systems and no clear
  owner for each.
- **`organizations.currency` / `payment_settings.default_currency` drift.**
  If not resolved, the admin UI will show inconsistent default currency
  values depending on which settings page the admin last visited.
