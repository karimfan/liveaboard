# Sprint 018 Merge Notes

## Codex Draft Strengths

- Correctly identified `planned`, `active`, `completed`, and
  `cancelled` as persisted lifecycle states.
- Kept readiness aggregation in the store/service layer instead of
  duplicating checks in handlers.
- Connected lifecycle to existing guest registration, document,
  cabin, folio, and audit surfaces.
- Called out mutation guards for completed/cancelled trips.
- Identified stale import hard-delete behavior as a lifecycle risk.

## Claude Code Critique Strengths

- Caught direct conflicts with the interview answers: Admin emergency
  override, documents as warnings, open folios as warnings, and soft
  removal for stale imported trips.
- Correctly rejected the speculative `trip_lifecycle_exceptions` table.
- Identified missing backend files: `cruise_director.go` and
  `cruise_director_assign.go`.
- Added concrete tests for concurrent transitions, reverse
  transitions, override audit metadata, and stale-import soft removal.
- Flagged the need to update import upserts so removed trips can
  reappear if the source lists them again.

## Valid Critiques Accepted

- Org Admins can start and complete trips only as emergency override,
  with required reason and audit metadata.
- Documents never block start in Sprint 018; they are warnings.
- Open folios never block completion in Sprint 018; they are warnings.
- Stale imported trips are retained in the backend and hidden from
  default operational trip lists through `removed_from_source_at`.
- Replace hard-delete reconciliation in both liveaboard.com and
  spreadsheet imports with soft removal.
- Add trip deletion protection when historical records exist.
- Add lifecycle behavior for Cruise Director assignment changes.
- Tighten cancellation and acknowledgement reason length.

## Critiques Rejected or Modified

- The critique mentioned `CHANGELOG.md`, but this repository currently
  has no root changelog file. The final Sprint 018 DoD does not require
  one.
- `ON DELETE SET NULL` for transition actor columns is retained to match
  recent Sprint 017 document/archive columns, while audit events remain
  the durable accountability record.
- A trip-level no-delete safeguard is scoped to rows with operational
  history rather than banning all trip deletes, because empty imported
  placeholders may still be cleaned up safely.

## Interview Refinements Applied

- Missing guest documents warn only; they do not block trip start.
- Open guest folios warn only; they do not block trip completion.
- Org Admin emergency override is allowed for start and complete.
- Imported trips removed from liveaboard.com/spreadsheet sources should
  disappear from the operational trip ledger by default while remaining
  retained for analytics, search, and history.

## Final Decisions

- Sprint 018 introduces explicit trip lifecycle plus operational
  readiness, not analytics/reporting.
- Use `acknowledged_warnings` plus bounded reason text instead of a
  structured exception table.
- Preserve backend trip and guest history. Revoke/soft-remove from
  workflows; do not hard-delete records with history.
- Date buckets are secondary schedule context; persisted lifecycle
  status drives permissions and UI state.
