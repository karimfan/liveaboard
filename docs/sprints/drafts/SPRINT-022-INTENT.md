# Sprint 022 Intent: Analytical Reporting Across Personas + Storage Strategy

## Seed

> We need to think about analytical reports for three main personas:
> guest, director and admin. We also need to think about where we
> will store analytical data. We can store it in postgres or ETL it
> to somewhere else e.g. BigQuery? I don't think this service will be
> super sized in terms of scale, so perhaps Postgres for OLTP and
> OLAP?

## Context

- The product already has a reporting backlog in
  `docs/product/organization-admin-user-stories.md`. US-7.1 (setup
  completeness, **Must**) and US-7.2 (operational trip status,
  **Must**) are MVP-scoped. US-7.3 (per-trip revenue summary) is
  **Should**. US-7.4 (cross-trip analytics) is **Could / deferred**.
- The personas doc names three personas but only two have a current
  reporting story: Org Admin owns oversight; Cruise Director owns
  one trip's operational view; Guest's future scope mentions "tab,
  dive schedule, trip details" — guest analytics = a self-service
  tab view, not cross-trip insight.
- Real consumption data flows daily through `guest_folio_lines`
  (Sprint 019), with snapshotted effective prices and sources
  (Sprint 020). Trip lifecycle is explicit (`planned`/`active`/
  `completed`/`cancelled`, Sprint 018). Audit events (Sprint 017)
  exist but are operational accountability, not analytics.
- Tech stack is committed to Postgres with strict multi-tenant
  isolation, Go stdlib + minimal deps backend, and local-only dev
  for now. Cloud deployment is future.

## Recent Sprint Context

- **Sprint 019 — Real-Time Consumption Ledger**: turned folios into
  the live ledger. This is the primary source of revenue and
  consumption signal.
- **Sprint 020 — Pricing Overrides and Currency Defaults**: folio
  lines now snapshot `price_source` and `price_override_id` so
  historical revenue is reproducible from the ledger even after
  pricing config changes.
- **Sprint 021 — Filesystem Email Transport**: infra sprint enabling
  end-to-end testing without SMTP. Unrelated to analytics but useful
  for testing report email delivery if we add that.

## Relevant Codebase Areas

Tables that feed reports (all already exist):

- Identity & scope: `organizations`, `users`, `boats`, `trips`
  (with `status`, `started_at`, `completed_at`,
  `removed_from_source_at`), `trip_cruise_directors`, `trip_guests`,
  `guest_users`.
- Ledger & money: `guest_folios`, `guest_folio_lines` (with
  `price_source`, `price_override_id`, `voided_at`, `stock_posted_at`,
  `line_type` including `crew_tip`), `organization_payment_settings`,
  `exchange_rates`.
- Inventory: `boat_inventory_items`, `stock_movements`.
- Catalog: `catalog_categories`, `catalog_items`,
  `catalog_price_overrides`.
- Audit (already in place): `audit_events`.

Likely new code locations:

- `internal/store/migrations/0020_*.sql` — schema (views and/or
  materialized views; possibly a small `report_runs` table for
  scheduled refreshes).
- `internal/store/reports.go` — Go-side report queries (one file,
  one struct per report).
- `internal/httpapi/report_handlers.go` — Admin and Director report
  endpoints; Guest tab endpoint.
- `web/src/admin/pages/Reports*.tsx` — Admin reporting screens.
- `web/src/admin/pages/TripDashboard.tsx` (or extend the existing
  trip manifest screen) — Director per-trip dashboard.
- `web/src/guest/pages/MyTab.tsx` — Guest self-service tab.

## Constraints

- **No new infrastructure dependency.** Postgres only; no BigQuery,
  Snowflake, ClickHouse, or external ETL service. The user's lean
  is explicit and matches CLAUDE.md's "local-only dev for now"
  posture.
- **Multi-tenant isolation is non-negotiable.** Every report query,
  view, and materialized view must scope by `organization_id`. No
  cross-org leakage even at the SQL layer.
- **Authoritative ledger stays the source of truth.** Reports read
  from `guest_folio_lines` and its snapshotted price columns; they
  do not duplicate pricing logic. Historical totals must be
  reproducible from raw ledger rows.
- **No new aggregation tables of "denormalized facts" yet.** If
  performance demands it later, a follow-up sprint adds them. For
  MVP, reports are views (or materialized views with a refresh job)
  over the existing tables.
- **No real-time streaming.** Reports refresh on a schedule (or on
  read) — async, not push.
- **Persona scoping is mandatory.** Admin sees org-wide. Cruise
  Director sees only assigned trips. Guest sees only their own
  folio.
- **Architecture must leave a clean seam to externalize later.**
  Report queries live behind a `Reports` interface in the store;
  swapping the backend to a separate read-only replica or external
  warehouse should not require handler or UI changes.

## Success Criteria

- An Org Admin can open a reports section and see at least:
  (a) the existing US-7.1 setup completeness dashboard
  (b) the existing US-7.2 operational trip status view, and
  (c) US-7.3 per-trip revenue summary (Should).
- A Cruise Director can open one assigned trip and see operational
  analytics for that trip: occupancy %, current folio totals,
  top-consumed catalog items, low-stock items, manifest readiness
  (registrations submitted vs pending, documents uploaded vs
  missing).
- A Guest, signed in to their trip-registration session, can see
  "My tab": their own folio so far for the active or completed
  trip, itemized.
- Reports are reproducible: querying the same window twice yields
  identical numbers as long as no new ledger lines were posted in
  between.
- There is a single documented decision on the storage strategy
  (`docs/decisions/`-style ADR or inside the sprint doc) saying:
  "Postgres for both OLTP and OLAP at MVP scale, behind a thin
  Reports interface; revisit when N or query latency cross a
  documented threshold."
- Definition of Done includes `go test`, `go vet`, frontend build,
  and a tested guarantee that the reports cannot return rows from
  another organization.

## Open Questions

The drafts should answer (or take a clear position on) each of
these — the interview will resolve any disagreement.

1. **Storage strategy.** Plain Postgres views, materialized views
   with a refresh job, or a small dedicated set of denormalized
   fact tables maintained incrementally?
2. **OLAP-vs-OLTP isolation today.** Do we add a read-replica or
   read-only DB user for reports, or accept that reports hit the
   primary at MVP scale?
3. **Refresh cadence.** If materialized views: refresh every N
   minutes, refresh on demand from the UI, or refresh after each
   folio-close? Mix?
4. **Scope of this sprint.** All three personas, or admin-only with
   director + guest deferred to 023 / 024? The reporting backlog is
   admin-heavy.
5. **Cross-trip analytics (US-7.4).** Out of scope per the
   personas doc (deferred). Reaffirm or pull in?
6. **Director analytics scope.** Is it a new "Trip Dashboard" page,
   or does it extend the existing TripManifest / TripConsumption-
   Ledger screens? The line between "operational live view" (which
   already exists) and "analytical dashboard" is blurry here.
7. **Guest tab UX.** Read-only itemized folio for the currently
   active trip only, or also past trips? Show running total in
   guest's preferred currency or USD-only with a footer note?
8. **Currency reporting.** All revenue in USD (canonical price)
   only, or also in the settlement currency captured at close?
   Folios store both; reports should be explicit.
9. **Voided lines.** Exclude entirely from revenue totals (matches
   folio view), or expose a void/correction breakdown for finance?
10. **Authz for reports.** Inherit existing role gates (Org Admin /
    assigned Cruise Director / guest session)? Anything new needed?
11. **Performance budget.** What is the acceptable p95 query time
    for the admin dashboard at the org scale we expect (e.g., 5
    boats × 50 trips/year × 20 guests/trip)?
12. **External seam.** Add a `Reports` interface in `internal/store`
    now even though there's only one implementation, so a future
    BigQuery/ClickHouse backend is a swap, not a rewrite?
