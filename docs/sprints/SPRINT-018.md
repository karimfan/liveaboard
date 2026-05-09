# Sprint 018: Trip Lifecycle and Readiness Gates

## Overview

Sprint 018 makes trip lifecycle explicit. Trips currently have dates and
some screens infer upcoming, active, or past from those dates. That is
not enough for operations: a trip needs to be intentionally started,
completed, cancelled, or removed from the operational trip ledger while
preserving backend history.

This sprint connects the foundations from Sprints 014-017: guest
registration, mandatory cabin/berth assignment, guest documents, folio
checkout, and audit. Lifecycle is an operational workflow, not a
reporting sprint.

## Use Cases

1. **See trip status.** Staff sees `planned`, `active`, `completed`,
   `cancelled`, and source-removed trip states without relying on date
   inference.
2. **Review readiness before start.** Staff sees blockers and warnings
   before starting a trip.
3. **Start a trip.** Assigned Cruise Director starts a planned trip.
   Org Admin can start any org trip through emergency override with a
   required reason.
4. **Complete a trip.** Assigned Cruise Director completes an active
   trip. Org Admin can complete through emergency override with a
   required reason.
5. **Acknowledge warnings.** Missing documents and open folios warn, but
   do not block lifecycle transitions when acknowledged.
6. **Cancel a planned trip.** Org Admin cancels a planned trip with a
   reason while preserving all historical records.
7. **Soft-remove stale imported trips.** Trips that disappear from
   liveaboard.com or spreadsheet import sources disappear from default
   operational trip lists but remain retained in the backend for search,
   analytics, and history.
8. **Audit lifecycle decisions.** Start, complete, cancel, override, and
   warning acknowledgement decisions are audit visible.

## Architecture

### Core Rules

- `trips.status` is the operational source of truth.
- Valid statuses are `planned`, `active`, `completed`, and `cancelled`.
- Existing trips default to `planned`.
- Date-derived buckets can still be shown as schedule context, but they
  do not drive lifecycle permissions.
- Assigned Cruise Directors can start assigned `planned` trips and
  complete assigned `active` trips.
- Org Admins can start or complete any org trip only as emergency
  override with a required reason.
- Org Admins can cancel `planned` trips with a required reason.
- Completed and cancelled trips are read-only for guest, document,
  cabin, registration, folio, and director-assignment mutations unless a
  handler explicitly documents a historical read/resend exception.
- Guests are revoked from workflows, never hard-deleted.
- Trips with operational history are retained, never hard-deleted.
- Lifecycle transitions and audit events are committed in the same
  transaction.

### Status and Source Removal

Add lifecycle columns to `trips`:

```sql
status text not null default 'planned'
  check (status in ('planned','active','completed','cancelled')),
started_at timestamptz null,
started_by_user_id uuid null references users(id) on delete set null,
completed_at timestamptz null,
completed_by_user_id uuid null references users(id) on delete set null,
cancelled_at timestamptz null,
cancelled_by_user_id uuid null references users(id) on delete set null,
cancellation_reason text null check (
  cancellation_reason is null or char_length(cancellation_reason) <= 500
),
removed_from_source_at timestamptz null
```

Indexes:

```sql
CREATE INDEX trips_org_status_start_idx
  ON trips(organization_id, status, start_date)
  WHERE removed_from_source_at IS NULL;

CREATE INDEX trips_org_removed_source_idx
  ON trips(organization_id, removed_from_source_at, start_date)
  WHERE removed_from_source_at IS NOT NULL;
```

Import behavior:

- `ReplaceFutureScrapedTrips` and `ReplaceSpreadsheetTrips` no longer
  hard-delete stale future trips by default.
- A row absent from the latest source reconciliation gets
  `removed_from_source_at = now()` and is hidden from the default trip
  list.
- If the same source trip key appears again, the import upsert clears
  `removed_from_source_at`.
- Empty placeholder trips with no operational history may still be
  hard-deleted only if the store can prove there are no dependent guest,
  audit, document, cabin, or folio rows.

### Readiness

Create `TripLifecycleReadiness` in the store layer. It should return:

- trip status and transition metadata
- `can_start` and `can_complete`
- hard blockers
- warnings
- active guest count
- per-guest readiness rows:
  - revoked state
  - registration status
  - document count/category summary
  - berth assignment state
  - folio status

Start blockers:

- trip is not `planned`
- caller lacks assigned Director permission and is not Org Admin
  emergency override
- active guest lacks berth assignment
- active guest has not submitted registration

Start warnings:

- active guest has no active document
- no active guests
- boat has no active cabin layout
- Org Admin emergency override was used

Complete blockers:

- trip is not `active`
- caller lacks assigned Director permission and is not Org Admin
  emergency override

Complete warnings:

- one or more active guests have open or missing folios
- active guest has no active document
- Org Admin emergency override was used

Warnings are acknowledged with:

```json
{
  "acknowledged_warnings": ["missing_documents", "open_folios"],
  "reason": "Guest documents were checked onboard."
}
```

Reason text is required for Org Admin override and for any transition
that proceeds with warnings. Reason length is capped at 500 characters.

### Audit Actions

Add lifecycle audit events:

| Action | Entity | Metadata |
|---|---|---|
| `trip.started` | `trip` | `{previous_status, new_status, warning_codes, acknowledged_warnings, override_used, override_role, reason}` |
| `trip.completed` | `trip` | `{previous_status, new_status, open_folio_count, warning_codes, acknowledged_warnings, override_used, override_role, reason}` |
| `trip.cancelled` | `trip` | `{previous_status, new_status, reason}` |
| `trip.removed_from_source` | `trip` | `{source_provider}` |
| `trip.restored_from_source` | `trip` | `{source_provider}` |

No raw registration payloads, document storage keys, local paths, or
large PII blobs go into audit metadata.

### Store Layer

Create `internal/store/trip_lifecycle.go`:

- `TripLifecycleStatus` constants
- `TripLifecycleReadiness`
- `TripReadinessIssue`
- `TripReadinessGuest`
- `TripLifecycleTransitionInput`
- `TripReadiness(ctx, orgID, tripID, now)`
- `StartTrip(ctx, orgID, tripID, actorID, input, now)`
- `CompleteTrip(ctx, orgID, tripID, actorID, input, now)`
- `CancelTrip(ctx, orgID, tripID, actorID, reason, now)`
- `TripHasOperationalHistory(ctx, orgID, tripID)`

Modify `internal/store/trips.go`:

- Add lifecycle fields to `Trip`, `tripColumns`,
  `prefixedTripColumns`, and `scanTrip`.
- Preserve lifecycle columns during imports.
- Clear `removed_from_source_at` when an imported source trip reappears.
- Soft-remove stale source trips and return a renamed/expanded import
  result count such as `TripsRemovedFromSource`.
- Exclude `removed_from_source_at IS NOT NULL` from default operational
  trip list methods unless an include flag/filter asks for them.
- Update `TripsNeedingAttention` and `TripCountForOrg` semantics:
  setup completeness counts trips not removed from source; attention
  excludes completed, cancelled, and removed-from-source trips.

Add a delete safeguard in the migration:

- Reject `DELETE FROM trips` when a trip has operational history:
  `trip_guests`, `guest_documents`, `guest_folios`,
  `trip_cabin_assignments`, or `audit_events`.
- Empty source placeholder trips without history may still be cleaned up
  by import reconciliation if needed.

### HTTP API

Add staff routes:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/lifecycle` | GET | Admin or assigned CD | Status, transition metadata, readiness. |
| `/api/admin/trips/{id}/start` | POST | Assigned CD or Admin override | Start planned trip. |
| `/api/admin/trips/{id}/complete` | POST | Assigned CD or Admin override | Complete active trip. |
| `/api/admin/trips/{id}/cancel` | POST | Org Admin | Cancel planned trip. |

Start/complete request:

```json
{
  "acknowledged_warnings": ["missing_documents"],
  "reason": "Documents checked onboard."
}
```

Cancel request:

```json
{
  "reason": "Operator cancelled departure."
}
```

Permission helper:

- Normal start/complete: `role = cruise_director` and assigned to trip.
- Emergency start/complete: `role = org_admin`, reason required,
  `override_used = true` in audit metadata.
- Cancel: `role = org_admin`, reason required.

Mutation guards:

- Block guest invite add/resend/revoke on completed/cancelled trips.
- Block cabin assignment changes on completed/cancelled trips.
- Block guest document upload/archive on completed/cancelled trips.
- Block guest registration save/submit/document upload on
  completed/cancelled trips.
- Block folio open/line mutation/close on completed/cancelled trips.
- Block Cruise Director assignment changes on completed/cancelled
  trips. Assignment changes remain allowed on planned/active trips.

### Frontend

Trip list:

- Show lifecycle status chips.
- Add status filter including "Removed from source".
- Default filter hides `removed_from_source_at IS NOT NULL`.
- Keep source-removed trips available through explicit filter/search.

Trip manifest:

- Add lifecycle banner with current status, transition timestamps, and
  actions.
- Show readiness blockers and warnings in a dense checklist.
- Start button for assigned Directors on planned trips.
- Start/complete emergency override controls for Org Admins, with
  required reason.
- Complete button for assigned Directors on active trips.
- Cancel button for Org Admins on planned trips.
- Disabled/read-only operational controls on completed/cancelled trips.

Cruise Director overview:

- Replace date-derived upcoming/active/past counts with persisted
  lifecycle counts: planned, active, completed.
- Keep dates visible as schedule context.

Guest registration:

- If trip is cancelled or completed, show a minimal closed/cancelled
  state and do not render editable registration or document upload.

## Implementation Plan

### Phase 1: Schema, Store, Import Semantics (~35%)

**Files:**

- `internal/store/migrations/0017_trip_lifecycle.sql`
- `internal/store/trips.go`
- `internal/store/trip_lifecycle.go`
- `internal/store/trip_lifecycle_test.go`
- `internal/store/trips_test.go`
- `internal/testdb/testdb.go`

**Tasks:**

- [ ] Add lifecycle and source-removal columns to `trips`.
- [ ] Add lifecycle indexes.
- [ ] Add trip delete safeguard for rows with operational history.
- [ ] Update `Trip` scanning and trip list methods.
- [ ] Implement readiness aggregation.
- [ ] Implement transactional start, complete, and cancel mutations with
      audit events.
- [ ] Replace stale import hard deletes with soft removal for both
      liveaboard.com and spreadsheet imports.
- [ ] Clear `removed_from_source_at` when a source trip reappears.
- [ ] Add tests for transition validity, concurrent double-start,
      reverse transition rejection, soft-remove import behavior, and
      no hard delete with history.

### Phase 2: HTTP API and Guards (~25%)

**Files:**

- `internal/httpapi/trip_lifecycle_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/admin.go`
- `internal/httpapi/cruise_director.go`
- `internal/httpapi/cruise_director_assign.go`
- `internal/httpapi/guest_manifest_handlers.go`
- `internal/httpapi/guest_registration_handlers.go`
- `internal/httpapi/document_handlers.go`
- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/cabin_handlers.go`
- `internal/httpapi/trip_lifecycle_handlers_test.go`

**Tasks:**

- [ ] Add lifecycle read/start/complete/cancel routes.
- [ ] Enforce assigned Director and Admin override permissions.
- [ ] Require bounded reasons for Admin override and warning
      acknowledgement.
- [ ] Add completed/cancelled mutation guards.
- [ ] Update Admin trip list and CD overview payloads to use persisted
      lifecycle status.
- [ ] Add handler tests for assigned Director, unassigned Director, Org
      Admin override, missing override reason, warning acknowledgement,
      cancelled guest registration denial, and completed/cancelled
      mutation denial.

### Phase 3: Frontend Lifecycle UX (~25%)

**Files:**

- `web/src/admin/api.ts`
- `web/src/lib/api.ts`
- `web/src/admin/pages/Trips.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/pages/Overview.tsx`
- `web/src/pages/GuestRegistration.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [ ] Add lifecycle API types and methods.
- [ ] Add status chips and filters to trip list.
- [ ] Add manifest lifecycle banner, readiness checklist, and
      transition controls.
- [ ] Add warning acknowledgement and Admin override reason flows.
- [ ] Update CD overview tiles to lifecycle counts.
- [ ] Add guest closed/cancelled state.
- [ ] Keep visual treatment dense and operational per `DESIGN.md`.

### Phase 4: Product Docs and Verification (~15%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-018.md`
- `docs/sprints/tracker.tsv`

**Tasks:**

- [ ] Update lifecycle persona boundaries for Admin override.
- [ ] Document source-removed trip retention.
- [ ] Update user stories for start, complete, cancel, and stale import
      behavior.
- [ ] Run `go run docs/sprints/tracker.go sync`.
- [ ] Run `gofmt` on Go changes.
- [ ] Run `npm --prefix web run build`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/trips/{id}/lifecycle` | GET | Lifecycle status and readiness. |
| `/api/admin/trips/{id}/start` | POST | Start planned trip. |
| `/api/admin/trips/{id}/complete` | POST | Complete active trip. |
| `/api/admin/trips/{id}/cancel` | POST | Cancel planned trip. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0017_trip_lifecycle.sql` | Create | Persist lifecycle and source-removal state. |
| `internal/store/trips.go` | Modify | Add fields, filters, and soft-remove import reconciliation. |
| `internal/store/trip_lifecycle.go` | Create | Readiness and transitions. |
| `internal/httpapi/trip_lifecycle_handlers.go` | Create | Lifecycle API. |
| `internal/httpapi/cruise_director.go` | Modify | Persisted lifecycle counts. |
| `internal/httpapi/cruise_director_assign.go` | Modify | Assignment guards by status. |
| `web/src/admin/pages/Trips.tsx` | Modify | Status filters and chips. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Lifecycle controls and readiness. |
| `web/src/admin/pages/Overview.tsx` | Modify | Lifecycle-aware counts. |
| `web/src/pages/GuestRegistration.tsx` | Modify | Closed/cancelled state. |

## Definition of Done

- [ ] Trips have persisted lifecycle status and transition metadata.
- [ ] Existing trips default safely to `planned`.
- [ ] Import upserts preserve lifecycle state and clear source-removal
      when a trip reappears.
- [ ] Stale imported trips are retained in the backend and hidden from
      default operational trip lists.
- [ ] Assigned Directors can start planned assigned trips.
- [ ] Assigned Directors can complete active assigned trips.
- [ ] Org Admins can start/complete any org trip with required
      emergency override reason.
- [ ] Org Admins can cancel planned trips with required reason.
- [ ] Override starts/completes are audit-flagged with actor role and
      reason.
- [ ] Missing documents warn but do not block start.
- [ ] Open folios warn but do not block completion.
- [ ] Completed/cancelled trips block operational mutations.
- [ ] Guest registration save/submit/document upload is denied for
      completed/cancelled trips.
- [ ] Trips and guests with operational history are not hard-deleted.
- [ ] UI shows lifecycle status, readiness, warnings, and transition
      controls.
- [ ] `gofmt` applied.
- [ ] `npm --prefix web run build` passes.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.

## Security Considerations

- Every lifecycle endpoint is organization scoped.
- Cruise Directors can transition only assigned trips.
- Org Admin emergency override requires a reason and is audit-visible.
- User-supplied reason text is length bounded before persistence and
  audit metadata.
- Guest endpoints expose only minimal closed/cancelled state.
- No lifecycle audit metadata includes document storage keys, local
  paths, full registration payloads, tokens, or secrets.

## Dependencies

- Sprint 014: guest registration.
- Sprint 015: guest folio checkout.
- Sprint 016: cabin layouts and berth assignments.
- Sprint 017: guest documents and audit.

## References

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-015.md`
- `docs/sprints/SPRINT-016.md`
- `docs/sprints/SPRINT-017.md`
