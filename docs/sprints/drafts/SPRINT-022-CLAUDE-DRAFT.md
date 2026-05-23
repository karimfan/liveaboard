# Sprint 022: Analytical Reports — Foundation + Per-Persona Surfaces

## Overview

Sprint 022 stands up the first analytical reporting surface in the
product. It does two things at once: it picks the storage approach
(Postgres-only, with a thin `Reports` interface that leaves a clean
seam to externalize later) and it ships one concrete report per
persona — Admin, Cruise Director, Guest — so the new architecture
is exercised end-to-end on real ledger data.

The lever the sprint pulls is **build reports as read-only views
and queries over the existing OLTP tables**, gated through a
dedicated `internal/store/reports` package that handlers and
frontends consume through stable typed structs. No new
denormalized fact tables, no ETL job, no read replica, no managed
warehouse. The folio ledger from Sprint 019 and the snapshotted
price source from Sprint 020 are already the right shape for this:
every revenue number reports needs is already reproducible from
`guest_folio_lines` and friends.

The sprint deliberately avoids both extremes. It does not pretend
analytics is "just a SELECT" (the persona scoping, multi-currency
totals, void semantics, and director/trip authorization rules need
real care). It also does not over-engineer for a scale we don't
have — the storage decision and the one ADR are explicit so a
future sprint can move OLAP to a read replica, materialized refresh
schedule, or external warehouse without rewriting handlers or UI.

## Use Cases

1. **Admin operational dashboard.** An Org Admin opens
   `/admin/reports` and sees: setup completeness (US-7.1),
   operational trip status (US-7.2), and per-trip revenue
   summaries (US-7.3) — all three Must/Should reports from the
   existing backlog, in one place.
2. **Director trip dashboard.** An assigned Cruise Director opens
   one of their trips and sees: occupancy %, registration
   readiness (submitted vs pending), document readiness (uploaded
   vs missing), live folio totals across all guests, top-consumed
   catalog items, low-stock alerts. Read-only summary; mutations
   stay on the existing manifest/ledger screens.
3. **Guest tab.** A guest signed in to their trip-registration
   session opens "My tab" and sees their own folio for the active
   or most-recent trip: itemized lines, subtotal, settlement total
   if closed, payment method if recorded. No other trip, no other
   guest visible.
4. **Director: low-stock alert.** During an active trip, a director
   sees stock that has fallen below the configured minimum, with a
   link to the inventory adjustment screen.
5. **Admin: reproducible revenue.** Admin downloads the per-trip
   revenue numbers from `/admin/reports` and the same numbers can
   be recomputed from `guest_folio_lines` by hand — reports are a
   view, not a separate write path.
6. **Future-proofing.** A Sprint 0NN swaps the `Reports`
   implementation to a read replica without touching handlers,
   frontend, or test fixtures.

## Architecture

### Storage Decision

Two new keys are added to **none of the config files** — there is
nothing to configure yet. The choice is:

- **Postgres remains the only datastore.** Both OLTP and OLAP queries
  hit the same primary at MVP scale.
- **Reports are read-only Postgres views**, not new write paths. A
  small number of cheap reports use plain views. A small number of
  expensive ones use **materialized views** with a manual refresh
  endpoint plus a 5-minute auto-refresh ticker in the server
  process.
- **No new aggregate tables** maintained by application code. If a
  materialized view stops being fast enough, the next sprint can
  add an incrementally maintained fact table — but not yet.
- **All report queries route through `internal/store/reports`.**
  Handlers and the UI never write SQL inline. This is the seam
  that lets us swap to a read replica or warehouse later.

The accompanying ADR (`docs/decisions/0001-analytics-storage.md`)
documents this with an explicit "revisit when …" trigger:
> Revisit when (a) any single materialized view takes > 30s to
> refresh, (b) admin dashboard p95 query time exceeds 2s, or (c)
> we onboard the first customer with > 1M ledger lines.

### Data Flow

```
            ┌──────────────────────────────────┐
            │ guest_folios / guest_folio_lines │  (write path: ledger)
            │ stock_movements                   │
            │ trips / trip_guests / boats       │
            │ catalog_* / exchange_rates        │
            │ audit_events                      │
            └──────────────────┬────────────────┘
                               │ read only
                               ▼
            ┌──────────────────────────────────┐
            │ Postgres views + materialized    │
            │ views, namespaced report_*       │
            └──────────────────┬────────────────┘
                               │
                               ▼
            ┌──────────────────────────────────┐
            │ internal/store/reports           │
            │   - SetupCompleteness(...)       │
            │   - TripStatus(...)              │
            │   - TripRevenue(...)             │
            │   - TripDashboard(...)           │
            │   - GuestTab(...)                │
            └──────────────────┬────────────────┘
                               │
                               ▼
            ┌──────────────────────────────────┐
            │ httpapi handlers                  │
            │   /api/admin/reports/*            │
            │   /api/admin/trips/{id}/dashboard │
            │   /api/guest/tab                  │
            └──────────────────────────────────┘
```

### Reports

Naming convention for SQL objects: `report_<scope>_<thing>` (e.g.
`report_admin_trip_revenue`). Every view and materialized view
includes `organization_id` as the first column so a per-row WHERE
clause is always available.

| Persona | Report | SQL object | Refresh |
|---|---|---|---|
| Admin | Setup completeness | view `report_admin_setup_completeness` | live |
| Admin | Operational trip status | view `report_admin_trip_status` | live |
| Admin | Per-trip revenue | materialized `report_admin_trip_revenue` | 5 min + manual |
| Director | Trip dashboard | view `report_director_trip_dashboard` | live |
| Director | Top items per trip | view `report_director_trip_top_items` | live |
| Director | Low-stock per boat | view `report_director_low_stock` | live |
| Guest | My tab | view `report_guest_tab` (filtered by `trip_guest_id`) | live |

Materialized only where the aggregation across all org trips makes
the live query expensive (per-trip revenue summed over folio
lines). Everything else stays as a plain view — straightforward,
no refresh logic to maintain.

### `internal/store/reports` Package

One file per persona to keep the surface small and reviewable:
`admin.go`, `director.go`, `guest.go`, plus a `reports.go` that
defines the `Reports` interface and the typed return shapes.

The interface is implemented today by `*store.Pool` (same DB
connection as the rest of the app). The seam exists so a future
sprint can add a second implementation (`ReadReplicaPool` or
`WarehouseClient`) without churn elsewhere.

```go
// internal/store/reports/reports.go
package reports

type Reports interface {
    // Admin
    SetupCompleteness(ctx context.Context, orgID uuid.UUID) (SetupCompleteness, error)
    TripStatusSummary(ctx context.Context, orgID uuid.UUID, window TimeWindow) (TripStatusSummary, error)
    TripRevenueSummary(ctx context.Context, orgID uuid.UUID, window TimeWindow) ([]TripRevenueRow, error)
    RefreshTripRevenueMaterialized(ctx context.Context) error

    // Director (per-trip; the call must verify assignment upstream)
    TripDashboard(ctx context.Context, orgID, tripID uuid.UUID) (TripDashboard, error)
    LowStock(ctx context.Context, orgID, boatID uuid.UUID, threshold int) ([]LowStockRow, error)

    // Guest (per-guest-trip; the guest session bounds tripGuestID)
    GuestTab(ctx context.Context, tripGuestID uuid.UUID) (GuestTab, error)
}
```

### Authorization

Reports inherit existing auth rules — no new middleware:

- Admin endpoints behind `RequireOrgAdmin` (already in use for
  `/admin/*`).
- Director endpoints behind the trip-cruise-director-assignment
  check (already in use on the ledger endpoints).
- Guest endpoint behind the existing guest-session middleware,
  using the session's `trip_guest_id` directly.

Defense-in-depth: every report query in `reports/*.go` takes the
caller's `organization_id` (or, for guest, the session-bound
`trip_guest_id`) and includes it as a WHERE clause. The view
itself does not enforce isolation — the query does. A regression
test for each handler asserts cross-org isolation explicitly.

### Refresh Strategy

The single materialized view (`report_admin_trip_revenue`):

- Refreshed every **5 minutes** by a goroutine in
  `cmd/server/main.go` started after migrations apply.
- Refreshed **on demand** via
  `POST /api/admin/reports/refresh` (Admin-only).
- Refreshed **once at server startup** so a freshly migrated
  database has data immediately.

The refresh uses `REFRESH MATERIALIZED VIEW CONCURRENTLY` so the
refresh does not block reads. This requires a unique index on the
materialized view, which we add at create time.

### What Does NOT Change

- `email.Sender` and Sprint 021's filesystem transport.
- Any existing handler under `/api/admin/*` other than the new
  `/reports` and `/trips/{id}/dashboard` endpoints.
- Mutation paths through the ledger, inventory, or audit.
- Audit semantics — reports are read-only and do not write
  `audit_events`.
- Frontend chrome (Shell, sidebar, etc.) except for adding three
  new routes.

## Implementation Plan

### Phase 1: Storage Foundation + ADR (~15%)

**Files:**
- `docs/decisions/0001-analytics-storage.md` — new ADR
- `internal/store/migrations/0020_reports_views.sql` — views and
  materialized view + their unique index
- `internal/store/migrations/0020_reports_views_test.sql` (optional;
  smoke fixture)

**Tasks:**
- [ ] Write ADR: Postgres-only for OLTP+OLAP at MVP; revisit
      conditions stated; seam through `internal/store/reports`.
- [ ] Migration creates the seven `report_*` SQL objects above.
      Plain views for the six live ones; materialized view +
      unique index for `report_admin_trip_revenue`.
- [ ] Each view includes `organization_id` as the leading column.
- [ ] Migration is reversible (`+goose Down` drops the view set).
- [ ] Spot-check the migration on a copy of the dev DB.

### Phase 2: Reports Package (~30%)

**Files:**
- `internal/store/reports/reports.go` — interface, shared types,
  `TimeWindow`, `MoneyUSD` helpers (or reuse existing money types)
- `internal/store/reports/admin.go` — `SetupCompleteness`,
  `TripStatusSummary`, `TripRevenueSummary`,
  `RefreshTripRevenueMaterialized`
- `internal/store/reports/director.go` — `TripDashboard`,
  `LowStock`
- `internal/store/reports/guest.go` — `GuestTab`
- `internal/store/reports/*_test.go` — fixture-driven tests for
  each report

**Tasks:**
- [ ] Define the typed structs that handlers/UI consume. Match the
      existing money representation (USD cents) used in
      `guest_folio_lines`.
- [ ] Implement each report query. WHERE clauses are explicit
      and `organization_id`-scoped.
- [ ] Materialized-view refresh helper that calls
      `REFRESH MATERIALIZED VIEW CONCURRENTLY report_admin_trip_revenue`.
- [ ] Tests over `testdb` for each report: happy path, empty
      org returns zero rows, voided folio lines excluded from
      totals, two orgs cannot see each other's data.

### Phase 3: HTTP Handlers (~15%)

**Files:**
- `internal/httpapi/report_handlers.go` — Admin and Director
  endpoints
- `internal/httpapi/guest_tab_handlers.go` — Guest endpoint
- `internal/httpapi/httpapi.go` — route mounts
- `internal/httpapi/report_handlers_test.go` — auth and isolation
  tests

**Tasks:**
- [ ] Mount Admin endpoints under `/api/admin/reports/*` (gated
      by `RequireOrgAdmin`).
- [ ] Mount Director endpoint under
      `/api/admin/trips/{id}/dashboard` (existing assignment check
      reused).
- [ ] Mount Guest endpoint under `/api/guest/tab` (guest session
      middleware).
- [ ] Tests: every endpoint refuses cross-org and unauth callers
      with the same status codes existing endpoints use.

### Phase 4: Refresh Job + Manual Refresh Endpoint (~10%)

**Files:**
- `cmd/server/main.go` — start the refresh goroutine after
  `store.Migrate`
- `internal/store/reports/refresh.go` — small loop with context
  cancellation
- `internal/httpapi/report_handlers.go` — `POST
  /api/admin/reports/refresh`

**Tasks:**
- [ ] 5-minute ticker that calls
      `RefreshTripRevenueMaterialized` and logs the duration.
- [ ] One refresh at startup before listen.
- [ ] Admin-only manual refresh endpoint that calls the same
      helper synchronously.
- [ ] Goroutine respects the existing graceful-shutdown context.

### Phase 5: Frontend (~25%)

**Files:**
- `web/src/admin/pages/Reports.tsx` — admin dashboard combining
  the three admin reports
- `web/src/admin/pages/TripDashboard.tsx` — director per-trip
  analytics
- `web/src/admin/pages/TripManifest.tsx` — modify: add a link to
  TripDashboard for directors
- `web/src/admin/Shell.tsx` — modify: add "Reports" nav entry
  (Admin only)
- `web/src/admin/api.ts` — modify: typed report fetchers
- `web/src/guest/pages/MyTab.tsx` — guest tab page
- `web/src/guest/Shell.tsx` (or equivalent) — modify: add "My
  Tab" link
- `web/src/main.tsx` — modify: register the three new routes
- `web/src/styles/app.css` — modify: report card styling

**Tasks:**
- [ ] Admin `Reports.tsx` renders the three cards from
      `/api/admin/reports/{setup,trip-status,trip-revenue}`.
- [ ] Manual refresh button calls the refresh endpoint.
- [ ] `TripDashboard.tsx` renders occupancy, readiness, top
      items, low stock; deep-links into the operational screens.
- [ ] `MyTab.tsx` renders the guest's own folio summary; defaults
      to the active or most-recent trip.
- [ ] Authorization-driven nav: Admin sees Reports, Director sees
      Trip Dashboard, Guest sees My Tab.

### Phase 6: Docs and Verification (~5%)

**Files:**
- `docs/product/organization-admin-user-stories.md` — modify:
  link US-7.1/7.2/7.3 to Sprint 022; mark US-7.4 still deferred.
- `docs/product/personas.md` — modify: guest "tab" moves from
  Future into current scope (read-only).
- `docs/CONFIG.md` — modify only if a new key is added (likely
  not — refresh cadence stays hard-coded for now).

**Tasks:**
- [ ] Update product docs to reflect what Sprint 022 ships.
- [ ] `go test ./...`, `go vet ./...`, `npm run build` pass.

## API Endpoints

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/reports/setup-completeness` | GET | Org Admin | Setup blockers (US-7.1). |
| `/api/admin/reports/trip-status` | GET | Org Admin | Trip lifecycle counts + occupancy (US-7.2). |
| `/api/admin/reports/trip-revenue` | GET | Org Admin | Per-trip revenue rollup (US-7.3). |
| `/api/admin/reports/refresh` | POST | Org Admin | Force refresh the materialized view. |
| `/api/admin/trips/{id}/dashboard` | GET | Org Admin or assigned CD | Per-trip operational analytics. |
| `/api/guest/tab` | GET | Guest session | Guest's own folio summary. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `docs/decisions/0001-analytics-storage.md` | Create | ADR. |
| `internal/store/migrations/0020_reports_views.sql` | Create | Views + materialized view. |
| `internal/store/reports/reports.go` | Create | Interface + shared types. |
| `internal/store/reports/admin.go` | Create | Admin report queries. |
| `internal/store/reports/director.go` | Create | Director report queries. |
| `internal/store/reports/guest.go` | Create | Guest report queries. |
| `internal/store/reports/refresh.go` | Create | Materialized view refresher. |
| `internal/store/reports/*_test.go` | Create | DB-backed report tests. |
| `internal/httpapi/report_handlers.go` | Create | Admin + Director HTTP. |
| `internal/httpapi/guest_tab_handlers.go` | Create | Guest HTTP. |
| `internal/httpapi/report_handlers_test.go` | Create | Auth + isolation tests. |
| `internal/httpapi/httpapi.go` | Modify | Route mounts. |
| `cmd/server/main.go` | Modify | Start refresh goroutine. |
| `web/src/admin/pages/Reports.tsx` | Create | Admin reports page. |
| `web/src/admin/pages/TripDashboard.tsx` | Create | Director dashboard. |
| `web/src/guest/pages/MyTab.tsx` | Create | Guest tab. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Link to dashboard. |
| `web/src/admin/Shell.tsx` | Modify | Reports nav entry. |
| `web/src/admin/api.ts` | Modify | Typed fetchers. |
| `web/src/main.tsx` | Modify | Routes. |
| `web/src/styles/app.css` | Modify | Card styles. |
| `docs/product/organization-admin-user-stories.md` | Modify | Mark stories done/deferred. |
| `docs/product/personas.md` | Modify | Promote guest tab. |

## Definition of Done

- [ ] ADR explains the Postgres-only OLTP+OLAP choice with explicit
      revisit triggers.
- [ ] All seven `report_*` SQL objects created; materialized view
      has a unique index so `REFRESH … CONCURRENTLY` works.
- [ ] Every report query in `internal/store/reports` scopes by
      `organization_id` (or session-bound `trip_guest_id` for
      guest tab).
- [ ] Tests prove a second org's data is never returned by any
      report.
- [ ] Materialized view refreshes at startup, every 5 minutes, and
      on demand via the admin endpoint.
- [ ] Per-trip revenue numbers match what a hand-rolled SUM over
      `guest_folio_lines` (non-voided) returns for the same trip.
- [ ] Voided lines and pre-line-add stock corrections are excluded
      from revenue but visible in audit (no change to existing
      audit behavior).
- [ ] Director endpoint refuses unassigned directors with the same
      403 the ledger uses.
- [ ] Guest endpoint refuses any `trip_guest_id` other than the
      one bound to the guest session.
- [ ] Admin Reports page renders all three cards without
      visible loading flicker on a tiny dev dataset.
- [ ] `go test ./...`, `go vet ./...`, `npm run build` pass.
- [ ] Product docs updated; persona scope changes reflected.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Materialized refresh blocks reads. | Low | Medium | Use `REFRESH … CONCURRENTLY`; requires unique index, added on create. |
| Report query missing `organization_id` allows cross-org leakage. | Medium | High | Every query goes through `reports.*` package; isolation test asserts second org sees nothing. |
| Reports drift from ledger semantics (e.g., recompute price). | Medium | High | Reports read snapshotted `price_total_*` columns; no pricing logic in views. |
| 5-min cadence stale for admin "live" feel. | Medium | Low | Manual refresh endpoint + a "last refreshed" timestamp on the page. |
| `report_*` SQL objects collide with future migrations. | Low | Low | Namespace prefix + ADR documents the convention. |
| Director dashboard becomes a parallel "live ops" screen and confuses staff. | Medium | Medium | Read-only summary only; deep links into the operational screens for any mutation. |
| Guest tab leaks across trips. | Medium | High | Guest session is single-trip-scoped; report query keys off the session's bound `trip_guest_id`, never accepts an ID from the URL or body. |

## Security Considerations

- Every report endpoint inherits an existing role gate. Reports do
  not introduce a new authz axis.
- Defense-in-depth: each SQL query includes the caller's
  `organization_id` even if the view does not.
- Guest tab is bound to the session's `trip_guest_id` — no URL
  parameters that could be tampered with.
- Reports never expose audit fields beyond what the corresponding
  operational screen would (no raw token URLs, no PII payloads).
- Refresh endpoint is Admin-only and rate-limited via the existing
  middleware that protects other admin write endpoints. The
  underlying `REFRESH … CONCURRENTLY` is idempotent.
- No new logging surface introduced; refresh duration logged at
  INFO without secret-bearing fields.

## Dependencies

- Sprint 017: `audit_events` (used by setup completeness if we
  count outstanding warnings).
- Sprint 018: trip lifecycle states drive `trip_status` rollups.
- Sprint 019: live consumption ledger is the revenue source.
- Sprint 020: snapshotted effective prices make historical revenue
  reproducible.
- No new external Go dependencies.

## Open Questions

- Should the manual-refresh endpoint also trigger background
  refresh of any future materialized views, or stay scoped to the
  one we ship now? Default to one-now; revisit when we add more.
- Should "My Tab" expose past trips, or only the active/most-recent
  one? Default: most-recent for simplicity; revisit if guests ask
  for history.
- Should the admin revenue card show all currencies or USD-only?
  Default: USD totals with a footer-note showing settlement
  currency breakdown if non-USD lines exist; the ledger already
  carries both.
- Cross-trip analytics (US-7.4) remains deferred. Confirm during
  the interview.

## References

- `docs/product/personas.md` — persona boundaries.
- `docs/product/organization-admin-user-stories.md` — US-7.1, 7.2,
  7.3, 7.4 backlog.
- `docs/sprints/SPRINT-017.md` — audit foundation.
- `docs/sprints/SPRINT-019.md` — live consumption ledger.
- `docs/sprints/SPRINT-020.md` — effective price snapshots.
