# Sprint 022: Analytical Reports Across Personas + Postgres-Only Storage

## Overview

Sprint 022 stands up the first analytical reporting surface in the
product and commits to a storage strategy. The two decisions are
deliberately bundled because they constrain each other: at MVP
scale, the reports are small enough that Postgres can be both the
OLTP store and the analytical read path; and the persona-shaped
read API is what gives us a clean seam to externalize later if we
ever need to.

The lever is **plain, indexed Go queries over the existing ledger,
manifest, and inventory tables**, gated through a single
`internal/store/reports.go` surface. No materialized views. No
refresh job. No new fact tables. No external warehouse. No ETL.
The folio ledger from Sprint 019 — with snapshotted effective
prices from Sprint 020 — is already in exactly the right shape:
every revenue number a report needs is reproducible from
`guest_folio_lines` with a `WHERE voided_at IS NULL` filter.

The sprint ships one concrete surface per persona:

- **Admin**: `/admin/reports` with setup completeness (US-7.1),
  operational trip status (US-7.2), and per-trip revenue (US-7.3).
- **Cruise Director**: `/admin/trips/{id}/dashboard` with
  occupancy, registration/document readiness, folio totals, top
  catalog items, and low stock.
- **Guest**: `/guest/trips/:tripGuestId/tab` with the guest's own
  itemized folio and trip context.

Cross-trip analytics (US-7.4) remains deferred. The ADR records
the storage decision with explicit thresholds for revisiting it,
so a future sprint can lift this layer to materialized views, a
read replica, or an external warehouse without touching handlers
or UI.

## Use Cases

1. **Admin operational dashboard.** An Org Admin opens
   `/admin/reports`, picks a date window (defaults to `-30/+180`
   days), and sees: setup blockers with deep links, trip status
   counts + per-trip operational rows, and per-trip revenue
   (charges, settled, outstanding, voided corrections, crew tips,
   settlement currency breakdown). Numbers are reproducible from
   the underlying ledger.
2. **Cruise Director trip dashboard.** A Director opens an
   assigned trip and sees: occupancy %, manifest readiness,
   document readiness, folio totals across all guests, top-10
   consumed catalog items by USD, and low/zero/negative stock for
   the trip boat. Deep links into the existing manifest, ledger,
   and inventory screens for any mutation.
3. **Guest "My Tab".** A signed-in Guest opens the tab for one of
   their trips and sees the trip header, itemized non-voided folio
   lines, subtotal, card fee, total USD, and (if the folio is
   closed) settlement currency total. No other trip, no other
   guest visible.
4. **Reproducible revenue.** An auditor (or a curious Admin) can
   hand-roll a `SUM(line_total_usd_cents) WHERE voided_at IS NULL`
   over `guest_folio_lines` for one trip and get exactly the
   number the report shows.
5. **Storage decision documented.** The team has one written ADR
   stating Postgres is the MVP analytical store, with explicit
   revisit triggers (size + latency thresholds).

## Architecture

### Storage Decision (ADR 0003)

`docs/decisions/0003-reporting-storage.md` records:

- **Postgres for OLTP and OLAP at MVP scale.** Reports are plain
  Go queries through `internal/store/reports.go`.
- **No new datastore, no ETL, no managed warehouse, no read
  replica today.** Local-only dev posture; cloud comes later.
- **No materialized views, no denormalized fact tables, no
  refresh job.** Plain indexed queries first.
- **Escalation path** (in order):
  1. Materialized views + a refresh helper, when one or more
     reports cross the p95 budget.
  2. Read replica for reporting traffic, when contention with
     ledger writes becomes measurable.
  3. External OLAP store (BigQuery / ClickHouse / Snowflake),
     when cross-trip analytics or multi-org analytics demand it.
- **Revisit triggers** (any one is enough):
  - Any single report p95 > 500ms locally, or > 1s in deployed
    env after straightforward indexing.
  - Any organization crosses 10 boats, 1k trips, or 50k folio
    lines.
  - The team commits to features requiring trend analytics
    beyond US-7.3.
  - Report queries start contending with ledger writes.

### Data Flow

```
            ┌──────────────────────────────────┐
            │ Source-of-truth OLTP tables       │
            │   guest_folios / guest_folio_lines│
            │   stock_movements / boat_inventory│
            │   trips / trip_guests / boats     │
            │   catalog_* / exchange_rates      │
            │   audit_events / trip_cabin_*     │
            └──────────────────┬────────────────┘
                               │ read only
                               ▼
            ┌──────────────────────────────────┐
            │ internal/store/reports.go         │
            │   AdminReports(orgID, window)     │
            │   TripDashboard(orgID, tripID,    │
            │       actor)                      │
            │   GuestTab(guestUserID,           │
            │       tripGuestID)                │
            │ — all queries scope by org_id     │
            │   (or guest_user_id) in SQL       │
            └──────────────────┬────────────────┘
                               │
                               ▼
            ┌──────────────────────────────────┐
            │ httpapi handlers (thin)           │
            │   /api/admin/reports              │
            │   /api/admin/trips/{id}/dashboard │
            │   /api/guest/trip-registrations/  │
            │       {trip_guest_id}/tab         │
            └──────────────────────────────────┘
```

### Reports Surface

One file in package `store`: `internal/store/reports.go`.

```go
type AdminReports struct {
    Window           ReportWindow
    SetupBlockers    []SetupBlocker
    TripStatusCounts TripStatusCounts          // planned/active/completed/cancelled
    TripOperational  []TripOperationalRow      // per-trip readiness in window
    TripRevenue      []TripRevenueRow          // per-trip revenue in window
}

type TripDashboard struct {
    TripHeader        TripHeader
    Occupancy         Occupancy                // assigned berths / capacity
    RegistrationReady RegistrationReadiness    // submitted vs pending
    DocumentReady     DocumentReadiness        // uploaded vs missing
    FolioTotals       FolioTotals              // across all guests
    TopItems          []TripTopItemRow         // limit 10, by USD
    LowStock          []LowStockRow            // reorder_level floor or <= 0
}

type GuestTab struct {
    TripHeader   TripHeader
    FolioStatus  string                  // "open"/"closed"/"none"
    Lines        []GuestTabLine          // non-voided only
    SubtotalUSD  int64
    CardFeeUSD   int64
    TotalUSD     int64
    Settlement   *GuestTabSettlement     // present only when closed
}

// Methods on *store.Pool implement the queries:
func (p *Pool) AdminReports(ctx context.Context, orgID uuid.UUID, w ReportWindow) (*AdminReports, error)
func (p *Pool) TripDashboard(ctx context.Context, orgID, tripID uuid.UUID) (*TripDashboard, error)
func (p *Pool) GuestTab(ctx context.Context, guestUserID, tripGuestID uuid.UUID) (*GuestTab, error)
```

`HandleOverview`'s existing setup-completeness logic moves into
`reports.go` as the single source of truth, then `HandleOverview`
calls into the reports surface. This keeps the existing Overview
screen working and unifies "what's misconfigured" in one place.

### Revenue Semantics

- **Canonical USD revenue** = `SUM(line_total_usd_cents)` over
  `guest_folio_lines` where `voided_at IS NULL`. The Sprint 020
  snapshot columns (`price_source`, `price_override_id`) make this
  reproducible regardless of later catalog/pricing edits.
- **Crew tips** are line type `crew_tip`; reported separately, not
  rolled into "item revenue".
- **Voided corrections** are reported as
  `voided_line_count` + `voided_usd_cents`. They are *not*
  subtracted from revenue (because they were never added). They
  exist so ops can see corrective activity.
- **Settlement currency totals** come from closed folios only
  (`guest_folios.settlement_currency`, `settlement_total_minor`).
  They are **grouped by currency**, never summed across mixed
  currencies. A trip with mixed-currency settlements gets one row
  per currency.
- **Card fees** come from `guest_folios.card_fee_usd_cents`,
  rolled up across closed folios.

### Low-Stock Semantics

Per `boat_inventory_items` for the trip's boat:

- A row is "low" if `quantity_on_hand <= COALESCE(reorder_level, 0)`.
- Negative `quantity_on_hand` (allowed since Sprint 019) is
  always surfaced regardless of `reorder_level`.
- The dashboard shows item name, category, current quantity,
  `reorder_level`, and `par_level` (for context).

### Director Authorization

Reused from the Sprint 019 ledger endpoints — Org Admin can access
any org trip; a Cruise Director can access only trips where they
appear in `trip_cruise_directors`. No new middleware.

### Guest Authorization

The guest session middleware (`auth.GuestSessionMiddleware`)
already loads the `guest_user_id`. The handler:

1. Reads `trip_guest_id` from the URL.
2. Calls `pool.GuestTab(ctx, session.GuestUserID, tripGuestID)`.
3. The store query joins `trip_guests` on
   `id = tripGuestID AND guest_user_id = session.GuestUserID`. A
   non-match returns `sql.ErrNoRows` → handler returns the same
   opaque 404 existing guest routes return for unknown IDs.

This is defense-in-depth: even if the URL is tampered with, the
query refuses to return rows the session does not own.

### Cross-Tenant Isolation

Every query in `reports.go` takes an explicit `organization_id`
(or `guest_user_id`) and includes it in `WHERE` clauses.
Two-organization fixtures in `reports_test.go` assert that org A
cannot see org B's data via any report path.

### Bounded Windows

Admin reports accept optional `from` and `to` query params:

- Default `from = today - 30d`, `to = today + 180d`.
- Server caps the window to a maximum of 1 year. Larger windows
  return 400 with a clear error code.
- Dates parsed as ISO-8601 in the org's effective timezone (UTC
  for MVP).

### What Does NOT Change

- `email.Sender` and Sprint 021's filesystem transport.
- The ledger, inventory, audit, or pricing mutation paths.
- The existing guest-session middleware contract.
- The existing Sprint 008 admin overview surface, except that its
  setup-completeness query now lives in `reports.go`.
- DESIGN.md visual conventions; dashboards use dense tables and
  card layout, no chart libraries, no decorative analytics hero.

## Implementation Plan

### Phase 1: ADR + Reporting Indexes + Reports Surface (~35%)

**Files:**
- `docs/decisions/0003-reporting-storage.md` — new ADR
- `internal/store/migrations/0020_reporting_indexes.sql` —
  targeted indexes only
- `internal/store/reports.go` — DTOs + query methods on `*Pool`
- `internal/store/reports_test.go` — DB-backed tests including
  cross-tenant isolation

**Tasks:**
- [x] Write the ADR covering the Postgres-only decision, the
      escalation path, and the revisit triggers.
- [x] Audit existing migration indexes before adding new ones. Add
      only non-duplicate, query-justified indexes (candidate set
      from Codex draft, validated against current schema):
      `guest_folio_lines (organization_id, trip_guest_id, created_at) WHERE voided_at IS NULL`,
      `guest_folios (organization_id, trip_id, status)` if absent,
      `trips (organization_id, status, start_date, end_date)` if
      absent.
- [x] Implement `AdminReports`: setup blockers (moved from
      `HandleOverview`), trip status counts, trip operational
      rows, trip revenue rows.
- [x] Implement `TripDashboard`: header, occupancy, readiness,
      folio totals, top-10 items, low stock.
- [x] Implement `GuestTab` with the guest-user ownership join.
- [x] Tests: revenue excludes voided, separates crew tips,
      reports voided counts as correction metadata, settlement
      totals grouped per currency, low-stock floor, two-org
      isolation, director-not-assigned returns no data, guest
      cannot read another guest's `trip_guest_id`.
- [x] Update `HandleOverview` to call into the unified setup
      logic.

### Phase 2: HTTP Handlers + Authz Tests (~20%)

**Files:**
- `internal/httpapi/report_handlers.go` — new
- `internal/httpapi/report_handlers_test.go` — new
- `internal/httpapi/httpapi.go` — route mounts

**Tasks:**
- [x] Mount `GET /api/admin/reports` (RequireOrgAdmin); parse and
      bound the `from`/`to` window, reject > 1 year with 400.
- [x] Mount `GET /api/admin/trips/{id}/dashboard` (Admin or
      assigned CD).
- [x] Mount `GET /api/guest/trip-registrations/{trip_guest_id}/tab`
      (guest session middleware).
- [x] Tests: every endpoint refuses cross-org callers; Director
      cannot call org-wide endpoints; Director cannot access
      unassigned trips; Guest cannot read another guest's
      `trip_guest_id`; window > 1 year returns 400.

### Phase 3: Admin Reports UI (~15%)

**Files:**
- `web/src/admin/pages/Reports.tsx` — modify (replace placeholder
  if present, otherwise create)
- `web/src/admin/api.ts` — modify
- `web/src/main.tsx` — modify if route entry needs updating
- `web/src/admin/Shell.tsx` — modify (Reports nav, Admin only)
- `web/src/styles/app.css` — modify (report table styles)

**Tasks:**
- [x] Render setup completeness rows with deep links to fix
      screens.
- [x] Render trip status counts and operational table for the
      selected window.
- [x] Render per-trip revenue table with USD canonical headlines
      and per-currency settlement footnotes when present.
- [x] Date-window controls with conservative defaults.
- [x] DESIGN.md compliance: dense tables, muted surfaces, no
      decorative charts or hero analytics.

### Phase 4: Cruise Director Trip Dashboard UI (~15%)

**Files:**
- `web/src/admin/pages/TripDashboard.tsx` — new
- `web/src/admin/pages/Trips.tsx` — modify (link to dashboard)
- `web/src/admin/pages/TripManifest.tsx` — modify (link in
  toolbar)
- `web/src/admin/api.ts` — modify
- `web/src/main.tsx` — modify (route)
- `web/src/styles/app.css` — modify

**Tasks:**
- [x] Read-only dashboard route for a single trip.
- [x] Cards/tables for occupancy, readiness, folio totals,
      top-10 items, low stock.
- [x] Deep links into existing operational screens for any
      mutation; no write affordances on the dashboard itself.
- [x] Visible to Org Admin for any org trip; visible to a Cruise
      Director only for assigned trips (relies on handler
      authz; UI hides the link for unassigned directors).

### Phase 5: Guest Tab UI (~10%)

**Files:**
- `web/src/pages/GuestTab.tsx` — new
- `web/src/pages/GuestRegistration.tsx` — modify (link to tab
  once the guest is authenticated)
- `web/src/lib/api.ts` — modify
- `web/src/main.tsx` — modify (route
  `/guest/trips/:tripGuestId/tab`)
- `web/src/styles/app.css` — modify

**Tasks:**
- [x] Tab page in the existing guest surface (`web/src/pages/`,
      no new shell).
- [x] Show trip header, itemized non-voided lines, subtotal, card
      fee + total if closed, settlement currency total if closed.
- [x] Empty state when no folio yet.
- [x] Link in from the existing guest registration page after
      the guest authenticates.
- [x] Never expose admin chrome or other guests/trips.

### Phase 6: Docs and Verification (~5%)

**Files:**
- `docs/product/organization-admin-user-stories.md` — modify
  (mark US-7.1/7.2/7.3 done in Sprint 022; confirm US-7.4 deferred)
- `docs/product/personas.md` — modify (move guest tab from
  "Future" into current scope)

**Tasks:**
- [x] Update product docs.
- [x] Run `go test ./...`, `go vet ./...`, `npm run build`.

## API Endpoints

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/reports?from=…&to=…` | GET | Org Admin | Org-wide setup, trip status, per-trip revenue. |
| `/api/admin/trips/{id}/dashboard` | GET | Org Admin or assigned CD | Single-trip operational analytics. |
| `/api/guest/trip-registrations/{trip_guest_id}/tab` | GET | Guest session owner | Guest's own itemized tab. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `docs/decisions/0003-reporting-storage.md` | Create | ADR — Postgres-only at MVP, escalation path, revisit triggers. |
| `internal/store/migrations/0020_reporting_indexes.sql` | Create | Targeted, non-duplicate indexes for report queries. |
| `internal/store/reports.go` | Create | DTOs + queries; also hosts unified setup-completeness. |
| `internal/store/reports_test.go` | Create | DB-backed tests incl. cross-tenant isolation. |
| `internal/httpapi/admin.go` | Modify | `HandleOverview` calls into the unified setup logic. |
| `internal/httpapi/report_handlers.go` | Create | Admin + Director + Guest HTTP. |
| `internal/httpapi/report_handlers_test.go` | Create | Authz + isolation tests. |
| `internal/httpapi/httpapi.go` | Modify | Route mounts. |
| `web/src/admin/pages/Reports.tsx` | Modify | Live Admin reports. |
| `web/src/admin/pages/TripDashboard.tsx` | Create | Director dashboard. |
| `web/src/pages/GuestTab.tsx` | Create | Guest tab. |
| `web/src/admin/pages/Trips.tsx` | Modify | Link to TripDashboard. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Link to TripDashboard. |
| `web/src/admin/Shell.tsx` | Modify | Reports nav (Admin only). |
| `web/src/admin/api.ts` | Modify | Typed report fetchers. |
| `web/src/lib/api.ts` | Modify | Typed guest tab fetcher. |
| `web/src/pages/GuestRegistration.tsx` | Modify | Link to tab post-auth. |
| `web/src/main.tsx` | Modify | New routes. |
| `web/src/styles/app.css` | Modify | Report/table styles. |
| `docs/product/organization-admin-user-stories.md` | Modify | Mark stories done/deferred. |
| `docs/product/personas.md` | Modify | Promote guest tab. |

## Definition of Done

- [x] ADR 0003 records the Postgres-only decision, escalation
      path, and revisit triggers.
- [x] One migration adds only targeted, non-duplicate indexes.
      No views, no materialized views, no new tables.
- [x] `internal/store/reports.go` exposes `AdminReports`,
      `TripDashboard`, `GuestTab`. Every query scopes by
      `organization_id` (or `guest_user_id`) in SQL.
- [x] `HandleOverview`'s setup-completeness logic is sourced from
      `reports.go`; no duplicated calculation.
- [x] Admin Reports page renders setup completeness, trip status,
      and per-trip revenue from live data.
- [x] Cruise Director can open a read-only dashboard for an
      assigned trip and sees occupancy, readiness, folio totals,
      top items, and low stock.
- [x] Guest can open `/guest/trips/:tripGuestId/tab` for a
      `trip_guest_id` bound to their session, and only that one.
- [x] Revenue numbers are reproducible from
      `SUM(line_total_usd_cents) WHERE voided_at IS NULL` on
      `guest_folio_lines`.
- [x] Voided lines are excluded from revenue and exposed only as
      correction metadata (count + USD).
- [x] Settlement totals are grouped by currency and shown only
      for closed folios. They are never summed across mixed
      currencies.
- [x] Crew tips are reported separately from item revenue.
- [x] Low-stock surfaces items at or below `reorder_level`, plus
      always surfaces zero/negative stock.
- [x] Window > 1 year returns 400.
- [x] Cross-tenant tests prove org A cannot see org B's data via
      any report path.
- [x] Director-not-assigned and guest-not-owner authz tests pass.
- [x] Cross-trip analytics (US-7.4) remains deferred and is
      called out as such in the product docs.
- [x] `go test ./...` passes.
- [x] `go vet ./...` passes.
- [x] `npm run build` passes.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Report query missing `organization_id` allows cross-org leakage. | Medium | High | All report queries flow through `reports.go`; two-org fixtures assert isolation. |
| Reports drift from ledger semantics (e.g., re-running price logic). | Medium | High | Reports read snapshotted `line_total_usd_cents` only; no pricing recomputation. |
| Guest endpoint trusts URL parameter without ownership. | Medium | High | Guest query *requires* the session's `guest_user_id` in the WHERE clause; non-matches return opaque 404. |
| Director dashboard becomes a parallel write surface. | Medium | Medium | Read-only; deep links into manifest/ledger for any mutation. |
| Settlement totals get summed across currencies. | Medium | Medium | API returns settlement as a grouped list, never a single number; UI renders per-currency rows. |
| Query latency at small scale anyway. | Low | Low | Plain indexed queries with bounded windows; ADR documents the escalation path if this changes. |
| Setup-completeness duplication if `HandleOverview` refactor is bigger than expected. | Low | Low | Acceptable fallback: short-term duplication, flagged in code, with a follow-up task. |

## Security Considerations

- Every report endpoint inherits an existing role gate. No new
  authz middleware.
- Defense-in-depth: every SQL query includes the caller's
  `organization_id` (or `guest_user_id`).
- Guest tab is bound to the session's `guest_user_id` via a
  server-side join; the URL parameter is not trusted on its own.
- Reports never expose audit metadata blobs, raw tokens, document
  storage paths, payment-processor data, or another guest's PII.
- Bounded windows (max 1 year) prevent a single request from
  scanning unbounded history.
- No new logging surface introduced.

## Dependencies

- Sprint 017 — `audit_events` and document foundations for
  readiness context.
- Sprint 018 — trip lifecycle states drive trip status rollups.
- Sprint 019 — live consumption ledger is the revenue source;
  void semantics are honored.
- Sprint 020 — snapshotted effective prices make historical
  revenue reproducible.
- Sprint 021 — not required; the filesystem email transport is
  unrelated to this sprint (useful only if we later add report
  email delivery).
- No new external Go dependencies.

## References

- `docs/product/personas.md` — persona boundaries.
- `docs/product/organization-admin-user-stories.md` — US-7.1,
  7.2, 7.3, 7.4 backlog.
- `docs/decisions/0003-reporting-storage.md` — this sprint's ADR.
- `docs/sprints/SPRINT-017.md` — audit foundation.
- `docs/sprints/SPRINT-018.md` — trip lifecycle.
- `docs/sprints/SPRINT-019.md` — live consumption ledger.
- `docs/sprints/SPRINT-020.md` — effective price snapshots.
- `docs/sprints/drafts/SPRINT-022-INTENT.md`
- `docs/sprints/drafts/SPRINT-022-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-022-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-022-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-022-MERGE-NOTES.md`
