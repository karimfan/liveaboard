# Codex Critique of Claude Draft: Sprint 022

## Strengths to Preserve

- The draft correctly centers the sprint on three persona surfaces:
  Admin reports, Director per-trip analytics, and Guest "My tab".
- It makes the right top-level storage call: Postgres only for MVP scale,
  no external warehouse, no ETL service, and a thin reporting abstraction
  for future externalization.
- It clearly separates read-only reporting from existing mutation paths.
  The Director dashboard should deep-link to Manifest/Ledger instead of
  becoming a second write surface.
- It treats ledger rows and Sprint 020 price snapshots as authoritative.
  That is the right basis for reproducible revenue.
- It includes strong security themes: role-gated endpoints, explicit
  org-scoped SQL, cross-org tests, and guest isolation tests.

## Major Concerns

### 1. Materialized Views and Refresh Job Are Premature

The intent explicitly asks the drafts to choose between plain views,
materialized views, or denormalized fact tables. Claude's draft chooses
materialized revenue plus a server refresh goroutine, startup refresh, manual
refresh endpoint, and `REFRESH MATERIALIZED VIEW CONCURRENTLY`.

That is probably too much for Sprint 022. The expected MVP scale in the intent
is small, and existing report windows can be bounded. A 5-minute refresh job
also introduces stale report semantics, operational state, startup behavior,
and failure handling before we have evidence direct indexed queries are too
slow.

Recommended change:

- Start with direct parameterized SQL queries in `internal/store/reports.go`.
- Add only targeted indexes after checking existing indexes.
- Document materialized views as the first escalation path in the ADR.
- Drop `POST /api/admin/reports/refresh`, the ticker, `cmd/server/main.go`
  refresh work, and refresh-specific tests from this sprint unless a measured
  query proves they are needed.

### 2. ADR Number Is Wrong

The draft proposes `docs/decisions/0001-analytics-storage.md`, but the repo
already has:

- `docs/decisions/0001-auth-provider.md`
- `docs/decisions/0002-revert-clerk.md`

Recommended change:

- Use `docs/decisions/0003-reporting-storage.md`.

### 3. New `internal/store/reports` Subpackage Does Not Fit the Current Store Shape

The project currently keeps store methods in package `store` as files under
`internal/store/*.go` (`guest_folios.go`, `trips.go`, `inventory.go`, etc.).
Adding `internal/store/reports` means either creating a subpackage that cannot
directly access package-private helpers or changing dependency wiring more than
this sprint needs.

Recommended change:

- Use `internal/store/reports.go` in package `store`.
- Define report DTOs and methods there, implemented by `*store.Pool`.
- If a formal interface is useful, keep it small and consume it at the handler
  boundary later; do not restructure store packages now.

### 4. Guest Session Assumption Is Incorrect

The draft says the guest endpoint should use "the session's `trip_guest_id`
directly" and mount `/api/guest/tab` with no URL parameter. Current guest
sessions are scoped to `guest_user_id`, not `trip_guest_id`:

- `auth.GuestSessionMiddleware` loads a `store.GuestUser`.
- `store.GuestSession` stores `GuestUserID`.
- Existing guest routes take `trip_guest_id` in the URL and then verify access.

A guest user can plausibly be linked to multiple `trip_guests` over time, so
the tab endpoint needs a `trip_guest_id` parameter and a server-side ownership
join.

Recommended change:

- Mount `GET /api/guest/trip-registrations/{trip_guest_id}/tab`.
- In the store query, join `trip_guests` on `guest_user_id = current guest user
  id` and `id = trip_guest_id`.
- Return a non-leaking 404/403 for another guest's `trip_guest_id`.

### 5. Frontend Paths Reference a Nonexistent Guest App Structure

The draft lists:

- `web/src/guest/pages/MyTab.tsx`
- `web/src/guest/Shell.tsx`

The current frontend has `web/src/pages/*` for public/guest pages and
`web/src/admin/*` for staff/admin chrome. There is no `web/src/guest` shell
today. Creating one may be a valid future move, but it is extra architecture
for a first read-only guest tab.

Recommended change:

- Use `web/src/pages/GuestTab.tsx`.
- Register `/guest/trips/:tripGuestId/tab` in `web/src/main.tsx`.
- Link from the existing guest registration/trip page first.
- Defer a guest shell until there are multiple guest self-service pages that
  need shared navigation.

### 6. SQL Views Are Not Clearly Better Than Store Queries Here

The draft proposes seven `report_*` SQL objects. Views can be useful, but they
also push business naming, refresh choices, and versioning into migrations
before query shapes settle. Since every report still needs caller-specific
filters, role checks, windows, limits, and response shaping, Go store queries
may be simpler and more testable for Sprint 022.

Recommended change:

- Prefer Go store queries for the first implementation.
- Add SQL views only when the same aggregation is reused in multiple query
  surfaces or when a view materially reduces duplication.
- If any SQL view is added, require `organization_id` as a leading column and
  still filter by caller scope in the outer query.

### 7. Refresh Endpoint Security Claim Is Unsupported

The draft says the refresh endpoint is "rate-limited via the existing
middleware that protects other admin write endpoints." The current router does
not show a general admin write rate-limit middleware. Auth flows have throttles,
but admin endpoints mostly rely on session/role middleware.

Recommended change:

- Remove the refresh endpoint with the materialized-view scope.
- If a refresh endpoint remains, specify explicit throttling or concurrency
  protection rather than relying on a nonexistent general middleware.

## Missing or Under-Specified Details

- **Revenue columns:** The draft says reports read `price_total_*` columns, but
  the actual line model uses `line_total_usd_cents`, `unit_price_usd_cents`,
  `price_source`, and `price_override_id`.
- **Settlement totals:** The draft should distinguish canonical USD revenue
  from closed-folio settlement fields (`settlement_currency`,
  `settlement_total_minor`, FX snapshot fields). Settlement totals should not
  be summed across mixed currencies without grouping by currency.
- **Voided line handling:** It states voided lines are excluded, but the final
  plan should also decide whether to expose voided count/amount as correction
  metadata.
- **Low-stock semantics:** Inventory has `reorder_level` and `par_level`, not a
  generic configured minimum. Low-stock reports should use `reorder_level` when
  present and always surface zero/negative stock.
- **Window bounds:** Admin reports need default and maximum windows. A global
  all-history report risks making the first implementation slower than needed.
- **Setup completeness reuse:** Existing `AdminHandlers.HandleOverview`
  computes setup steps directly. The sprint should either move that calculation
  into store/report code or accept short-term duplication explicitly.
- **Trip dashboard navigation:** The draft says add Reports nav and Director
  dashboard nav, but the current shell already has admin reports routing. The
  concrete work is route/link placement from `Trips`, `TripManifest`, and maybe
  `TripConsumptionLedger`.

## Suggested Scope Changes

- Keep all three personas, but reduce infrastructure:
  Admin reports + Director trip dashboard + Guest tab are enough.
- Drop materialized view refresh, manual refresh endpoint, and background job.
- Use one consolidated Admin endpoint (`GET /api/admin/reports`) or three
  read-only endpoints. Either is fine, but a single endpoint is likely simpler
  for the current Reports page.
- Use `internal/store/reports.go`, not a new subpackage.
- Use `docs/decisions/0003-reporting-storage.md`.
- Use guest-user ownership checks for a parameterized Guest tab endpoint.
- Add indexes only after validating existing migration indexes.

## Risks the Final Sprint Should Address

- **Overbuilding analytics infrastructure:** materialized views and refresh jobs
  may consume the sprint without improving the first user-facing reports.
- **Guest data leakage:** the final plan must correct the session model and
  prove guest-user-to-trip-guest ownership in tests.
- **Cross-org leakage:** every report query needs two-org fixtures, not just
  handler role tests.
- **Stale revenue semantics:** if materialized views stay in scope, the UI must
  show `last_refreshed_at`; otherwise Admins may compare stale reports to live
  ledger pages.
- **Mixed currency confusion:** report USD as canonical and group settlement
  totals by currency, or omit aggregate settlement totals.

## Parts to Reject or Simplify

- Reject `docs/decisions/0001-analytics-storage.md`; use ADR 0003.
- Reject `web/src/guest/*` unless the final sprint intentionally creates a
  guest app shell.
- Reject `/api/guest/tab` as currently described; it does not match the guest
  session model.
- Simplify `internal/store/reports/*` subpackage to `internal/store/reports.go`.
- Simplify `report_*` SQL objects to direct store queries unless a concrete
  query needs a view.
- Reject the 5-minute materialized refresh loop and manual refresh endpoint for
  the MVP version unless performance testing justifies them.

## Bottom Line

Claude's draft is directionally strong on persona boundaries and the Postgres
storage decision, but it should be merged with a much leaner implementation
shape. The final sprint should ship real reports first, keep the warehouse seam
at the store boundary, and defer materialization/refresh machinery until direct
Postgres queries actually fail the documented performance budget.
