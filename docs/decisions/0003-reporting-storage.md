# ADR-0003: Reporting Storage

Status: Accepted (Sprint 022)
Date: 2026-05-23

## Context

Sprint 022 introduces the first analytical reporting surface for the
three product personas (Org Admin, Cruise Director, Guest). The team
needs a documented answer to "where does analytical data live?"
before building screens, so a future scale-up does not surprise us.

Constraints in play:

- The product is intentionally small at MVP. A single Org Admin owns
  a single organization with a handful of boats and a few dozen
  trips per year.
- The backend is committed to Postgres with strict tenant
  isolation. Cloud deployment is still future work.
- The folio ledger (Sprint 019) and snapshotted pricing (Sprint 020)
  already make revenue numbers reproducible from raw OLTP rows. No
  derived warehouse is required to compute them.
- Local-only dev posture per CLAUDE.md. Adding a managed warehouse
  or ETL service would add operational state we are not ready to
  carry.

## Decision

**Postgres is the only analytical store at MVP scale.** Reports
are plain, indexed Go queries that live in
`internal/store/reports.go`. Handlers consume typed DTOs from that
package and never write aggregate SQL inline.

Specifically:

- No new datastore (BigQuery / Snowflake / ClickHouse / DuckDB).
- No ETL or sync job.
- No materialized views, denormalized fact tables, or refresh
  ticker.
- No read replica.
- No new indexes were added in Sprint 022. Existing indexes from
  migrations 0011 / 0013 / 0017 / 0018 cover the report query
  shapes. The decision tree for future indexes lives in the
  escalation path below.

The seam that keeps externalization cheap is the reports surface in
package `store`: every report has a typed method on `*store.Pool`
and a typed return shape. A future implementation can return the
same DTOs from a read replica, materialized view, or external
warehouse without changing handlers or UI.

## Escalation Path

When the revisit triggers below fire, work through the steps in
order. Each step is its own sprint scope:

1. **Add a materialized view** for the offending report (start with
   per-trip revenue), with a small `REFRESH MATERIALIZED VIEW
   CONCURRENTLY` helper and either a startup refresh + ticker or a
   manual refresh endpoint. Requires a unique index on the
   materialized view.
2. **Add a Postgres read replica** for reporting traffic, behind a
   second `*store.Pool` wired into the reports surface only. OLTP
   continues to hit the primary.
3. **Add an external OLAP store** (e.g. BigQuery, ClickHouse) with
   a small ingestion job, behind a new implementation of the
   reports surface. This is also the moment cross-trip / cross-org
   analytics becomes reasonable to build.

## Revisit Triggers

Revisit when any one of these becomes true:

- A single report's p95 query latency exceeds 500ms locally, or 1s
  in the deployed environment, after exhausting straightforward
  per-query index additions.
- Any organization crosses 10 boats, 1k trips, or 50k folio lines.
- The team commits to features that require cross-trip or
  cross-season trend analytics beyond US-7.3 (per-trip revenue).
- Report query execution starts contending with live ledger writes
  (visible in lock waits or `pg_stat_activity`).
- Multi-org analytics or vendor data sharing enters scope.

## Consequences

Positive:

- Zero new operational state. The deployed binary still owns
  exactly one external dependency (Postgres).
- Revenue numbers are reproducible from raw OLTP rows. There is no
  "stale warehouse vs. live ledger" confusion.
- The reports surface is a clean handoff point if/when we
  externalize.

Negative:

- Every report query competes with OLTP writes on the same
  Postgres instance. At MVP scale this is fine; at scale it is the
  thing that fires the first revisit trigger.
- Cross-trip / cross-org analytics will eventually need
  denormalized aggregates. We pay that cost when it becomes real,
  not now.

## References

- `docs/sprints/SPRINT-022.md` — the implementing sprint.
- `docs/sprints/drafts/SPRINT-022-MERGE-NOTES.md` — Claude/Codex
  synthesis that led to this decision.
- `internal/store/reports.go` — the reports surface.
- ADR-0001 / ADR-0002 — auth provider history (precedent for
  decision-then-revert in this repo).
