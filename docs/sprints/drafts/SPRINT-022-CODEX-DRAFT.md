# Sprint 022 Codex Draft: Persona Reporting and Postgres Analytics Strategy

## Overview

Sprint 022 turns the placeholder reporting surface into a small, real
analytics layer for the three current personas: Org Admin, Cruise Director,
and Guest. The sprint is intentionally not a warehouse sprint. MVP scale,
local-only development, and strict tenant isolation all point to Postgres as
both the OLTP store and the reporting read store for now.

The main architectural outcome is a thin `store.Reports` query surface over
the existing ledger, trip, manifest, inventory, and setup tables. Handlers and
UI code consume report-shaped DTOs, not ad hoc SQL from multiple places. That
keeps today's implementation simple while leaving a clean future path to a
read replica, materialized views, or an external warehouse if org size or query
latency later justifies it.

## Use Cases

1. **Org Admin setup oversight.** An Admin opens Reports and sees the same
   setup completeness signal that Overview already uses, with concrete
   misconfiguration rows linked to the screens where they can be fixed.
2. **Org Admin operational status.** An Admin sees trips by status for a
   bounded window, plus per-trip occupancy, assigned director status, manifest
   readiness, and lifecycle state.
3. **Org Admin per-trip revenue.** An Admin sees total charges, closed/settled
   totals, open/outstanding totals, voided correction count, and currency notes
   per trip, reproducible from `guest_folio_lines` and `guest_folios`.
4. **Cruise Director assigned-trip dashboard.** A Director opens an assigned
   trip and sees operational analytics for that one trip: occupancy, folio
   totals, top consumed items, low/negative stock, registration readiness, and
   document readiness.
5. **Guest self-service tab.** A signed-in Guest opens their own current or
   completed trip tab and sees only their own itemized folio and running total.
6. **Storage decision.** The team has one documented decision that Postgres is
   the MVP analytical store, with explicit thresholds for revisiting the choice.

## Architecture

### Storage Strategy

Create `docs/decisions/0003-reporting-storage.md` with the decision:

- Use Postgres for OLTP and MVP analytical reporting.
- Use direct, bounded read queries first; no BigQuery, Snowflake, ClickHouse,
  external ETL, or new service in this sprint.
- Do not add denormalized fact tables yet.
- Do not add materialized views yet unless implementation proves a specific
  report cannot meet the sprint's local performance budget with indexed SQL.
- Add query-facing indexes only where they support known filters or joins.
- Keep report reads behind a store-level reporting interface so handlers and UI
  are insulated from a future read replica, materialized view, or warehouse.

Revisit the decision when any of these become true:

- one organization has more than 10 boats, 1,000 trips, or 50,000 folio lines;
- Admin report p95 exceeds 500 ms locally or 1,000 ms in the future deployed
  environment after straightforward indexing;
- reports require cross-season trend analytics beyond US-7.3;
- report execution starts contending with live ledger writes.

### Source of Truth

Reports read from existing authoritative tables:

- setup and scope: `organizations`, `boats`, `trips`, `users`,
  `trip_cruise_directors`;
- readiness: `trip_guests`, guest registrations, guest documents, cabin
  assignments;
- revenue and consumption: `guest_folios`, `guest_folio_lines`;
- stock: `boat_inventory_items`, `stock_movements`;
- catalog context: `catalog_items`, `catalog_categories`,
  `catalog_price_overrides`.

Revenue rules:

- exclude `guest_folio_lines.voided_at IS NOT NULL` from charge and settlement
  totals;
- expose voided line count and voided USD total as correction metadata, not as
  revenue;
- use `line_total_usd_cents` as canonical report revenue;
- report settlement currency totals only for closed folios where settlement
  snapshots exist;
- do not re-run pricing override logic for reports because line snapshots are
  authoritative.

### Report Query Surface

Add a focused reporting file in the store package:

```go
type Reports interface {
    AdminReports(ctx context.Context, orgID uuid.UUID, window ReportWindow) (*AdminReports, error)
    TripOperationsReport(ctx context.Context, orgID, tripID, actorUserID uuid.UUID) (*TripOperationsReport, error)
    GuestTab(ctx context.Context, guestUserID, tripGuestID uuid.UUID) (*GuestTabReport, error)
}
```

Because `store.Pool` already owns the query methods, the first implementation
can be concrete methods on `*Pool` rather than a new dependency injection graph.
The important boundary is that handlers call report methods from one file and
return report DTOs. They should not duplicate aggregation SQL in handlers.

### Admin Reports

`AdminReports` should include:

- setup completeness steps, ideally by reusing or moving the existing Overview
  setup calculation into shared store/report logic;
- setup issue rows for the US-7.1 items that are currently measurable:
  currency missing, no boats, no active Cruise Directors, planned trips without
  directors, planned trips with empty manifests inside the configured window,
  catalog items without a category, and low/negative stock rows;
- trip status counts for `planned`, `active`, `completed`, `cancelled` inside
  the requested window;
- per-trip operational rows with boat name, dates, status, occupancy percent,
  guest count, expected count, director count, submitted registration count,
  document count, and attention flags;
- per-trip revenue rows with charges, settled, outstanding, open folio count,
  closed folio count, voided line count, and crew tip total.

The default window should be practical rather than global:

- from 30 days before today through 180 days after today for operational status;
- include completed trips in that window for revenue;
- allow query params to override `from` and `to` with server-side bounds.

### Cruise Director Trip Report

`TripOperationsReport` should be available to Org Admins and assigned Cruise
Directors. It is trip-scoped and should not expose cross-trip or org-wide
metrics.

Include:

- trip header: boat, itinerary, dates, status, capacity/expected guests;
- occupancy and manifest readiness;
- registration submitted/pending counts;
- document uploaded/missing counts;
- folio totals across guests: charges, open/outstanding, closed/settled;
- top consumed catalog items by quantity and USD total, limited to 10;
- low stock and negative stock rows for the trip boat;
- direct links already used by the admin shell: manifest, ledger, cabins, and
  guest folios.

This should be a new trip dashboard route, for example
`/admin/trips/{id}/dashboard`, rather than overloading the live consumption
ledger. The ledger remains a write-heavy operational tool; the dashboard is a
read-only summary.

### Guest Tab

Guest analytics for Sprint 022 means "my tab", not analytics in the Admin
sense.

Add a guest-session endpoint that returns the signed-in guest's own folio for a
single `trip_guest_id`. The store query must prove ownership by joining the
guest session's `guest_user_id` to `trip_guests.guest_user_id`; it must not
trust a `trip_guest_id` alone.

The UI should live in the guest surface, for example:

- `/guest/trips/:tripGuestId/tab`
- `web/src/pages/GuestTab.tsx`

The tab should show:

- trip and boat summary;
- folio status;
- itemized non-voided lines;
- subtotal, card fee if closed, total USD;
- settlement currency total if closed;
- empty state when no folio exists yet or no lines have been added.

Past completed trips are allowed if the guest owns that `trip_guest_id`.
Cross-trip guest history is out of scope.

### SQL and Indexing

Add migration `0020_reporting_indexes.sql` only for indexes and optional
database comments. Avoid SQL views in this sprint unless they materially reduce
duplication; Go query methods are easier to test and keep parameterized by
actor scope.

Candidate indexes:

```sql
CREATE INDEX guest_folio_lines_org_trip_active_idx
  ON guest_folio_lines (organization_id, trip_guest_id, created_at)
  WHERE voided_at IS NULL;

CREATE INDEX guest_folios_org_trip_status_idx
  ON guest_folios (organization_id, trip_id, status);

CREATE INDEX trips_org_status_dates_idx
  ON trips (organization_id, status, start_date, end_date);

CREATE INDEX trip_guests_org_trip_active_idx
  ON trip_guests (organization_id, trip_id)
  WHERE revoked_at IS NULL;
```

Validate each candidate against existing indexes before adding it; do not add
duplicates.

## Implementation Plan

### Phase 1: ADR, Report Types, and Query Foundation (~25%)

**Files:**
- `docs/decisions/0003-reporting-storage.md`
- `internal/store/migrations/0020_reporting_indexes.sql`
- `internal/store/reports.go`
- `internal/store/reports_test.go`
- `internal/store/store.go`

**Tasks:**
- [ ] Document the Postgres-for-OLTP-and-OLAP MVP decision and revisit
  thresholds.
- [ ] Add report DTOs and query methods in `internal/store/reports.go`.
- [ ] Reuse existing setup, trip, manifest, folio, and inventory logic where it
  keeps behavior consistent.
- [ ] Add only non-duplicate indexes that support the bounded report queries.
- [ ] Test revenue calculations exclude voided lines, include crew tips, and
  split open/outstanding from closed/settled folios.
- [ ] Test every report query scopes by `organization_id`.

### Phase 2: HTTP API and Authorization (~20%)

**Files:**
- `internal/httpapi/report_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/report_handlers_test.go`
- `internal/httpapi/guest_folio_handlers.go` (only if response helpers are
  reused)

**Tasks:**
- [ ] Add `GET /api/admin/reports` for Org Admin report data.
- [ ] Add `GET /api/admin/trips/{id}/dashboard` for Org Admin or assigned
  Cruise Director trip analytics.
- [ ] Add `GET /api/guest/trip-registrations/{trip_guest_id}/tab` for the
  signed-in guest's own tab.
- [ ] Parse and bound optional `from` / `to` query params for Admin reports.
- [ ] Reuse existing manifest assignment authorization for Director trip
  access.
- [ ] Return `403` for Director attempts to call org-wide Admin reports.
- [ ] Return `404` or `403` without leaking existence when a Guest requests
  another guest's tab.

### Phase 3: Admin Reports UI (~20%)

**Files:**
- `web/src/admin/pages/Reports.tsx`
- `web/src/admin/api.ts`
- `web/src/styles/app.css`

**Tasks:**
- [ ] Replace Reports placeholder cards with live report data.
- [ ] Show setup completeness and fixable setup issue rows.
- [ ] Show status counts and a dense operational trip table.
- [ ] Show per-trip revenue table with USD canonical totals and settlement
  notes.
- [ ] Add date-window controls with conservative defaults.
- [ ] Keep cross-trip trend analytics visibly out of scope rather than adding
  fake charts.
- [ ] Follow `DESIGN.md`: dense tables, muted working surfaces, no decorative
  analytics hero.

### Phase 4: Cruise Director Trip Dashboard (~15%)

**Files:**
- `web/src/admin/pages/TripDashboard.tsx`
- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/admin/pages/Trips.tsx`
- `web/src/styles/app.css`

**Tasks:**
- [ ] Add a read-only dashboard route for a single trip.
- [ ] Link to it from trip lists and existing trip surfaces.
- [ ] Show occupancy, readiness, folio totals, top consumed items, and low-stock
  rows.
- [ ] Preserve the ledger page as the place where Directors record
  consumption.
- [ ] Ensure Directors only see assigned trips and Admins can see any org trip.

### Phase 5: Guest Tab UI (~10%)

**Files:**
- `web/src/pages/GuestTab.tsx`
- `web/src/lib/api.ts`
- `web/src/main.tsx`
- `web/src/pages/GuestRegistration.tsx`
- `web/src/styles/app.css`

**Tasks:**
- [ ] Add a guest-session tab route.
- [ ] Add an entry point from the guest registration/trip page once a guest is
  authenticated.
- [ ] Show itemized folio lines and totals with clear empty states.
- [ ] Show USD canonical totals and settlement totals only when available.
- [ ] Avoid exposing org admin chrome or other guest/trip data.

### Phase 6: Verification and Documentation (~10%)

**Files:**
- `internal/store/reports_test.go`
- `internal/httpapi/report_handlers_test.go`
- `web/src/admin/pages/Reports.tsx`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/drafts/SPRINT-022-CODEX-DRAFT.md`

**Tasks:**
- [ ] Add cross-tenant tests using two organizations with overlapping trips,
  guests, and folio lines.
- [ ] Add Director authz tests for assigned and unassigned trips.
- [ ] Add Guest authz tests for own tab and another guest's tab.
- [ ] Add at least one frontend build-time type check path for each new report
  response shape.
- [ ] Update product docs only if the sprint clarifies guest tab or reporting
  scope.
- [ ] Run `go test ./...`, `go vet ./...`, and `npm run build`.

## API Endpoints

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/reports?from=YYYY-MM-DD&to=YYYY-MM-DD` | GET | Org Admin | Org-wide setup, status, and per-trip revenue reports |
| `/api/admin/trips/{id}/dashboard` | GET | Org Admin or assigned Cruise Director | Single-trip operational analytics |
| `/api/guest/trip-registrations/{trip_guest_id}/tab` | GET | Guest session owner | Guest's own itemized tab |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `docs/decisions/0003-reporting-storage.md` | Create | Record Postgres MVP analytics strategy and revisit thresholds |
| `internal/store/migrations/0020_reporting_indexes.sql` | Create | Add non-duplicate indexes for report queries |
| `internal/store/reports.go` | Create | Central report DTOs and SQL query methods |
| `internal/store/reports_test.go` | Create | Revenue, scoping, and report aggregation tests |
| `internal/httpapi/report_handlers.go` | Create | Admin, Director, and Guest report endpoints |
| `internal/httpapi/report_handlers_test.go` | Create | Authz and cross-tenant HTTP tests |
| `internal/httpapi/httpapi.go` | Modify | Mount report routes in the correct auth groups |
| `web/src/admin/pages/Reports.tsx` | Modify | Replace placeholder with live Admin reporting |
| `web/src/admin/pages/TripDashboard.tsx` | Create | Assigned-trip operational dashboard |
| `web/src/pages/GuestTab.tsx` | Create | Guest self-service tab |
| `web/src/admin/api.ts` | Modify | Add typed Admin/Director report API calls |
| `web/src/lib/api.ts` | Modify | Add typed Guest tab API call |
| `web/src/main.tsx` | Modify | Add new Admin trip dashboard and Guest tab routes |
| `web/src/styles/app.css` | Modify | Add dense report/table styles as needed |
| `docs/product/organization-admin-user-stories.md` | Modify | Clarify any final scope decisions if needed |

## Definition of Done

- [ ] ADR states: Postgres is the MVP OLTP and OLAP store; no external ETL or
  warehouse; revisit at documented size/latency thresholds.
- [ ] Org Admin Reports page shows setup completeness, operational trip status,
  and per-trip revenue from real data.
- [ ] Cruise Director can see a read-only operational dashboard only for
  assigned trips.
- [ ] Guest can see only their own itemized tab for an active or completed trip.
- [ ] Revenue reports are reproducible from `guest_folio_lines` snapshots and
  do not re-run current pricing logic.
- [ ] Voided lines are excluded from revenue totals and exposed only as
  correction metadata.
- [ ] Canonical report revenue is USD; settlement currency totals are shown only
  when folios are closed with settlement snapshots.
- [ ] Cross-trip trend analytics remains deferred.
- [ ] Tests prove org reports cannot return another organization's rows.
- [ ] Tests prove Directors cannot access unassigned trip reports.
- [ ] Tests prove Guests cannot access another guest's tab.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `npm run build` passes.

## Security Considerations

- Every report query accepts `organization_id` or `guest_user_id` explicitly and
  includes that scope in SQL.
- Admin org-wide reports stay inside the existing Org Admin route group.
- Director trip reports reuse assigned-trip authorization; Director access must
  never become "all org trips".
- Guest tab ownership is proven server-side by guest session user id, not by a
  client-provided trip guest id alone.
- No report response includes raw tokens, document storage paths, audit metadata
  blobs, payment processor data, or another guest's personal information.
- Report endpoints should use bounded windows and limits so a single request
  cannot scan all retained history indefinitely.

## Dependencies

- Sprint 017: audit and document foundations for readiness context.
- Sprint 018: trip lifecycle statuses and retained historical trips.
- Sprint 019: live consumption ledger and void semantics.
- Sprint 020: price snapshots and settlement currency metadata.
- Sprint 021: optional filesystem email transport, not required for this sprint
  unless report delivery is added later.

## Risks and Mitigations

- **Over-scoping all personas.** Keep Guest scope to "my tab" and Director
  scope to one assigned-trip dashboard. Do not add cross-trip Guest history or
  finance exports.
- **Ad hoc SQL drift.** Put report queries in `internal/store/reports.go` and
  keep handlers thin.
- **Tenant leakage.** Require explicit cross-org fixtures in store and HTTP
  tests before considering the sprint complete.
- **Revenue ambiguity.** Use USD ledger snapshots as canonical and label
  settlement totals separately.
- **Performance surprises.** Bound report windows, add only measured indexes,
  and document thresholds before introducing materialized views or ETL.
- **UI becoming decorative.** Use dense tables and summary counters consistent
  with `DESIGN.md`; avoid fake charts for deferred cross-trip analytics.

## Open Questions

1. Should the Admin report default window be `-30/+180` days, or should it match
   the existing Overview's 90-day attention horizon?
2. Should low stock use any future minimum stock threshold, or only surface
   zero/negative stock until min-stock configuration exists?
3. Should Guest tab be linked from the registration page immediately, or should
   there be a separate guest trip home page first?
4. Should Director dashboard be the default click target for trip rows, with
   Manifest/Ledger as secondary actions?
5. Is 500 ms local p95 the right revisit threshold for report queries at MVP
   scale?
