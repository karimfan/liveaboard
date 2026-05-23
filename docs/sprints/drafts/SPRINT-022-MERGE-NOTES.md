# Sprint 022 Merge Notes

## Claude Draft Strengths

- Three-persona framing (Admin, Director, Guest) with clear authz
  axes.
- Postgres-only storage decision with a `Reports` interface as the
  externalization seam.
- Folio ledger + Sprint 020 price snapshots as the authoritative
  revenue source (no re-running pricing logic in reports).
- Director dashboard as a read-only summary that deep-links into
  operational screens — not a competing write surface.
- Risks table called out cross-org leakage and guest-tab cross-
  trip leakage explicitly.

## Codex Draft Strengths

- **Refuses to materialize prematurely.** Plain indexed Go queries
  first; materialized views as the *first escalation path* in the
  ADR, not the MVP. Drops the 5-minute refresh ticker, startup
  refresh, manual refresh endpoint, and `REFRESH … CONCURRENTLY`
  plumbing.
- **Correct ADR number** (`0003-reporting-storage.md`); Claude
  proposed `0001` which already exists as `0001-auth-provider.md`.
- **Correct guest session model**: guest sessions key off
  `guest_user_id`, not `trip_guest_id`. The endpoint takes
  `trip_guest_id` in the URL and verifies ownership via a join.
- **Correct frontend paths**: `web/src/pages/GuestTab.tsx`. No new
  `web/src/guest/` shell.
- **Uses real ledger column names** (`line_total_usd_cents`,
  `unit_price_usd_cents`) rather than the placeholder `price_total_*`
  Claude used.
- **Names real inventory columns** (`reorder_level`, `par_level`)
  for low-stock semantics.
- **Notes existing `AdminHandlers.HandleOverview`** that already
  computes setup completeness — either move that logic into the
  reports surface or accept short-term duplication.
- **Bounded windows** with default + server-side max so a single
  report request can't scan all history.
- **Flat `internal/store/reports.go`** (package `store`), not a
  subpackage — matches the existing flat layout
  (`guest_folios.go`, `trips.go`, etc.).

## Valid Critiques Accepted

1. **Drop materialized views + refresh job + manual refresh
   endpoint.** Plain Go queries with targeted indexes for MVP. The
   ADR records materialized views as escalation #1.
2. **ADR is `0003-reporting-storage.md`** (not `0001-*`).
3. **`internal/store/reports.go` in package `store`** (no
   subpackage).
4. **Guest endpoint is `GET /api/guest/trip-registrations/{trip_guest_id}/tab`**
   — server-side ownership join against the session's
   `guest_user_id`. Returns the same opaque 404 pattern existing
   guest routes use for unknown/foreign IDs.
5. **`web/src/pages/GuestTab.tsx`**, no new guest shell.
6. **Prefer Go store queries over SQL views.** Add a view only if
   the same aggregation is reused across surfaces or materially
   reduces duplication. Migration `0020_reporting_indexes.sql` is
   indexes-only.
7. **Real column names everywhere**: `line_total_usd_cents`,
   `unit_price_usd_cents`, `subtotal_usd_cents`, `card_fee_usd_cents`,
   `total_usd_cents`, `voided_at`, `line_type`, `price_source`,
   `price_override_id`. Low-stock uses `reorder_level` (and
   surfaces zero/negative regardless).
8. **Settlement totals**: USD canonical headline; settlement
   currency totals exposed *grouped by currency*, never summed
   across mixed currencies. Only meaningful for closed folios.
9. **Voided lines**: excluded from revenue totals; exposed as
   `voided_line_count` + `voided_usd_cents` correction metadata so
   ops can see corrections.
10. **Window bounds**: default `from = -30d`, `to = +180d`;
    server-side max window 1 year. Reject larger windows with 400.
11. **Move `HandleOverview` setup logic into `reports.go`** as the
    single source of truth, then have `HandleOverview` call into
    the reports surface. (Acceptable alternative if the refactor
    grows: short-term duplication, explicitly documented.)
12. **Refresh endpoint security claim was unsupported** — moot
    because we drop the refresh endpoint with #1.

## Critiques Rejected (with reasoning)

None outright. Codex's critique is mostly factual corrections and
"don't over-engineer", which align with the project's design
philosophy. The only nuance is on SQL views vs Go queries — Codex
says "Go queries by default", which I accept, but I'm leaving the
door open in the ADR to add views if duplication appears (Codex
acknowledges this).

## Interview Refinements Applied

| Interview answer | Final-doc impact |
|---|---|
| Postgres-only behind a Reports interface | Accepted — keep the seam at the `internal/store` boundary, but as a small Go surface, not a subpackage. |
| All three personas, **full report set** | Keep all three personas. "Full report set" = every report mentioned in the personas + admin-user-stories docs that has data today: setup completeness, trip status, trip revenue (Admin); occupancy + readiness + folio totals + top items + low stock (Director); itemized folio + trip context (Guest). Cross-trip analytics (US-7.4) stays deferred. |
| New `/admin/trips/{id}/dashboard` page | Accepted. |
| USD canonical + settlement breakdown footnote | Accepted, with Codex's refinement that settlement totals are grouped by currency, never summed across mixed currencies. |

## Final Decisions

- **One ADR**: `docs/decisions/0003-reporting-storage.md`. Records:
  Postgres-only for OLTP+OLAP at MVP scale; reports are plain Go
  queries through `internal/store/reports.go`; materialized views
  are escalation #1, read replica is #2, external warehouse is
  #3. Revisit triggers: any single report's p95 > 500ms locally
  or > 1s in deployed env after indexing, OR any organization
  crosses 10 boats / 1k trips / 50k folio lines.
- **One migration**: `0020_reporting_indexes.sql` — indexes only.
  No views. No materialized views. No new tables.
- **One reports file in package `store`**:
  `internal/store/reports.go` (+ `reports_test.go`). DTOs and
  query methods. `HandleOverview`'s setup-completeness logic moves
  here as the single source of truth.
- **Three HTTP endpoints**:
  - `GET /api/admin/reports?from=…&to=…` — Admin (org-wide).
  - `GET /api/admin/trips/{id}/dashboard` — Admin or assigned
    Cruise Director (single trip).
  - `GET /api/guest/trip-registrations/{trip_guest_id}/tab` —
    guest session, with server-side ownership join.
- **Three frontend pages**:
  - `web/src/admin/pages/Reports.tsx` (modify the existing
    placeholder if any; otherwise create).
  - `web/src/admin/pages/TripDashboard.tsx` (new).
  - `web/src/pages/GuestTab.tsx` (new, no guest shell).
- **Revenue semantics**:
  - USD canonical: SUM(`line_total_usd_cents`) WHERE
    `voided_at IS NULL`.
  - Settlement totals: per-currency rollup over closed folios
    only (`settlement_currency`, `settlement_total_minor`),
    never summed across currencies.
  - Voided: separate `voided_line_count`, `voided_usd_cents`
    correction metadata; not part of revenue totals.
- **Director dashboard**: occupancy, registration readiness
  (submitted vs pending), document readiness (uploaded vs missing),
  folio totals (open/outstanding/closed), top-10 catalog items by
  USD, low stock (`reorder_level` floor + always-surface for
  zero/negative).
- **Guest tab**: trip header (boat, itinerary, dates, status),
  itemized non-voided lines, subtotal, card fee + total if closed,
  settlement total + currency if closed. Empty state when no folio
  lines yet.
- **Authz**:
  - Admin endpoints behind `RequireOrgAdmin`.
  - Director endpoint reuses the trip-assignment check from the
    ledger handler.
  - Guest endpoint: middleware loads guest session →
    `guest_user_id`; query joins `trip_guests` on
    `id = {trip_guest_id} AND guest_user_id = session.guest_user_id`.
- **Cross-trip analytics (US-7.4)**: remains deferred.
- **No materialized refresh ticker, no refresh endpoint, no
  background job.** Plain queries with indexes only.

## Phasing for the Final Doc

1. Phase 1 — ADR + reporting indexes migration + reports surface
   (DTOs, queries, tests) (~35%)
2. Phase 2 — HTTP handlers + authz tests (~20%)
3. Phase 3 — Admin Reports UI (~15%)
4. Phase 4 — Director Trip Dashboard UI (~15%)
5. Phase 5 — Guest Tab UI (~10%)
6. Phase 6 — Docs + verification (~5%)
