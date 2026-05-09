# Sprint 018 Codex Draft: Trip Lifecycle and Readiness Gates

## Overview

Sprint 018 introduces explicit trip lifecycle state and operational
transition workflows. Today the application stores trip dates and some
surfaces infer upcoming/active/past from the calendar. That is not
enough for onboard operations: a trip must be intentionally started by
an assigned Cruise Director, completed after checkout, or cancelled by
an Org Admin while still planned.

This sprint makes lifecycle state a first-class part of the trip model.
It also adds readiness gates that connect the work from Sprints 014-017:
guest enrollment, registration, documents, cabin/berth assignments,
folio checkout, and audit. The goal is not broad analytics. The goal is
to make the operational state of a trip explicit, auditable, and hard to
transition accidentally.

## Use Cases

1. **See explicit trip state.** Staff can distinguish planned, active,
   completed, and cancelled trips independent of date-derived labels.
2. **Review readiness before start.** Assigned Cruise Director opens a
   planned trip and sees whether every active guest has registration,
   required document coverage, and a berth assignment.
3. **Start an assigned trip.** Assigned Cruise Director starts a planned
   trip once readiness requirements pass or permitted exceptions are
   recorded.
4. **Run active trip operations.** Active trips remain editable for
   manifest, documents, cabins, inventory consumption, and checkout by
   assigned Directors.
5. **Complete an active trip.** Assigned Cruise Director completes the
   trip after all active guest folios are closed, or with explicit
   recorded exceptions if the product allows that.
6. **Cancel planned trip.** Org Admin can cancel a planned trip while
   preserving trip, guest, invite, registration, document, cabin, and
   audit history.
7. **Audit lifecycle changes.** Start, complete, cancel, and exception
   decisions are visible in the org audit log and per-trip activity.

## Architecture

### Core Rules

- `trips.status` is the operational source of truth.
- Valid statuses: `planned`, `active`, `completed`, `cancelled`.
- Existing imported trips default to `planned`.
- Date-derived labels may still be displayed as schedule context, but
  they must not drive operational permissions.
- Org Admin can cancel planned trips.
- Cruise Director can start planned trips and complete active trips only
  when assigned to the trip.
- Org Admin has read oversight for all trips but cannot start or
  complete trips in Sprint 018.
- Completed and cancelled trips are read-only for operational mutations
  except documented follow-ups such as reporting and email resend.
- Guest rows remain historical records; cancellation/revocation never
  hard-deletes `trip_guests`.
- Lifecycle transitions are transactionally coupled with audit events.
- Guest registration access for cancelled or completed trips is denied
  with a clear status response.

### Readiness Model

Create a store/service readiness summary rather than scattering checks
through handlers.

`TripLifecycleReadiness`:

- trip id and status
- can_start / can_complete booleans
- blocking issues
- warnings
- guest rows with per-guest readiness:
  - not revoked
  - registration status
  - document categories present
  - berth assignment present
  - folio status

Start readiness should require:

- trip status is `planned`
- caller is an assigned Cruise Director
- at least one assigned Cruise Director exists
- boat has active cabin/berth layout
- every non-revoked guest has an active berth assignment
- every non-revoked guest has submitted registration
- every non-revoked guest has at least one active document

Open question: whether missing registration/documents are hard blockers
or warnings with required acknowledgement. The first draft recommends
hard blockers for submitted registration and berth, warning-only for
documents until required document categories become configurable.

Complete readiness should require:

- trip status is `active`
- caller is an assigned Cruise Director
- every non-revoked guest has a closed folio

Open question: whether completion can proceed with exceptions. The first
draft recommends allowing exceptions only through structured
`trip_lifecycle_exceptions` records with reason text.

### Schema

Migration `0017_trip_lifecycle.sql`.

```sql
ALTER TABLE trips
  ADD COLUMN status text NOT NULL DEFAULT 'planned'
    CHECK (status IN ('planned', 'active', 'completed', 'cancelled')),
  ADD COLUMN started_at timestamptz NULL,
  ADD COLUMN started_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN completed_at timestamptz NULL,
  ADD COLUMN completed_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN cancelled_at timestamptz NULL,
  ADD COLUMN cancelled_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN cancellation_reason text NULL;

CREATE INDEX trips_org_status_start_idx
  ON trips(organization_id, status, start_date);
```

Optional exception table:

```sql
trip_lifecycle_exceptions (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  transition text not null check (transition in ('start','complete')),
  issue_code text not null,
  entity_type text null,
  entity_id uuid null,
  reason text not null,
  created_by_user_id uuid not null references users(id) on delete restrict,
  created_at timestamptz not null default now()
)
```

### Audit Actions

Add audit actions:

| Action | Entity | Metadata |
|---|---|---|
| `trip.started` | `trip` | `{previous_status, new_status, warning_count, exception_count}` |
| `trip.completed` | `trip` | `{previous_status, new_status, exception_count}` |
| `trip.cancelled` | `trip` | `{previous_status, new_status, reason}` |
| `trip.lifecycle_exception_recorded` | `trip_lifecycle_exception` | `{transition, issue_code, entity_type}` |

### Store Layer

Create `internal/store/trip_lifecycle.go`:

- `TripLifecycleStatus` constants
- `TripLifecycleReadiness`
- `TripReadinessIssue`
- `TripLifecycleException`
- `TripLifecycleExceptionInput`
- `TripReadiness(ctx, orgID, tripID, now)`
- `StartTrip(ctx, orgID, tripID, actorID, exceptions, now)`
- `CompleteTrip(ctx, orgID, tripID, actorID, exceptions, now)`
- `CancelTrip(ctx, orgID, tripID, actorID, reason, now)`

Modify `internal/store/trips.go`:

- Add status and transition columns to `Trip`.
- Update `tripColumns`, `prefixedTripColumns`, and `scanTrip`.
- Ensure import upserts preserve existing lifecycle columns.
- Ensure stale trip delete behavior is revisited: imported trips with
  guests, documents, audit, or non-planned status should not be deleted.
  Prefer marking stale planned imported trips as cancelled or leaving
  stale-delete behavior for a follow-up only if the current data model
  still expects deletion.

### HTTP API

Staff routes:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/lifecycle` | GET | Admin or assigned CD | Read lifecycle status and readiness. |
| `/api/admin/trips/{id}/start` | POST | Assigned CD | Start a planned trip. |
| `/api/admin/trips/{id}/complete` | POST | Assigned CD | Complete an active trip. |
| `/api/admin/trips/{id}/cancel` | POST | Org Admin | Cancel a planned trip. |

Payloads:

```json
{
  "exceptions": [
    {
      "issue_code": "missing_document",
      "entity_type": "trip_guest",
      "entity_id": "uuid",
      "reason": "Guest supplied paper copy onboard"
    }
  ]
}
```

```json
{
  "reason": "Operator cancelled departure"
}
```

### Permission Changes

- `authorizeManifestAccess` still controls read access.
- New helper `authorizeLifecycleTransition` should enforce:
  - start/complete: Cruise Director role and assigned to trip
  - cancel: Org Admin role and org-scoped trip
- Existing mutation handlers should reject cancelled/completed trips:
  - add/resend/revoke guest invite
  - cabin assignment changes
  - guest document upload/archive
  - guest registration save/submit
  - folio open/line mutations/close
  - inventory adjustments may remain org-admin only and not trip-bound
    unless tied to active-trip workflow later.

### Frontend

Trip list:

- Add lifecycle status chip.
- Add filters for status.
- Cancelled trips are visible via filter but not hidden from the
  database.

Trip manifest:

- Add lifecycle banner with status, start/complete/cancel actions, and
  readiness summary.
- Start button visible to assigned Directors on planned trips.
- Complete button visible to assigned Directors on active trips.
- Cancel button visible to Org Admins on planned trips.
- Completed/cancelled trips show read-only copy and disabled operational
  actions.

Cruise Director landing:

- Use persisted `status` counts instead of date classification.
- Link assigned trips to manifest/lifecycle view.

Guest registration:

- If trip is cancelled or completed, return a clear API error and show a
  simple closed/cancelled state instead of the form.

## Implementation Plan

### Phase 1: Schema and Store (~30%)

**Files:**

- `internal/store/migrations/0017_trip_lifecycle.sql`
- `internal/store/trips.go`
- `internal/store/trip_lifecycle.go`
- `internal/store/trip_lifecycle_test.go`
- `internal/testdb/testdb.go`

**Tasks:**

- [ ] Add lifecycle columns and indexes to trips.
- [ ] Add exception table if exception-based override is accepted.
- [ ] Update trip scanning and import preservation.
- [ ] Add readiness query aggregating manifest, documents, cabins, and folios.
- [ ] Add start/complete/cancel store methods with transaction-owned audit.
- [ ] Add tests for valid transitions, invalid transitions, role-agnostic store guards, readiness issue calculation, and no hard deletes.

### Phase 2: HTTP API and Guards (~25%)

**Files:**

- `internal/httpapi/trip_lifecycle_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/guest_manifest_handlers.go`
- `internal/httpapi/guest_registration_handlers.go`
- `internal/httpapi/document_handlers.go`
- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/cabin_handlers.go`
- `internal/httpapi/trip_lifecycle_handlers_test.go`

**Tasks:**

- [ ] Add lifecycle read/start/complete/cancel routes.
- [ ] Enforce transition permissions.
- [ ] Add mutation guards for completed/cancelled trips.
- [ ] Return structured readiness responses.
- [ ] Record audit events for every transition and exception.
- [ ] Add handler tests for Admin, assigned Director, unassigned
      Director, guest session denial, cancelled registration denial, and
      invalid transitions.

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
- [ ] Add status chips and status filter to trip list.
- [ ] Add manifest lifecycle banner and readiness issue panel.
- [ ] Add start/complete/cancel modal or inline confirmation controls.
- [ ] Update Cruise Director overview to use persisted lifecycle status.
- [ ] Add guest registration closed/cancelled state.
- [ ] Keep layout dense and operational per `DESIGN.md`.

### Phase 4: Product Docs and Verification (~20%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-018.md`
- `docs/sprints/tracker.tsv`

**Tasks:**

- [ ] Tighten lifecycle persona boundaries.
- [ ] Document no hard-delete behavior for guests and trips.
- [ ] Update story acceptance criteria for start/complete/cancel.
- [ ] Run tracker sync.
- [ ] Run gofmt, frontend build, `go test ./...`, and `go vet ./...`.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/trips/{id}/lifecycle` | GET | Lifecycle status and readiness. |
| `/api/admin/trips/{id}/start` | POST | Assigned Director starts planned trip. |
| `/api/admin/trips/{id}/complete` | POST | Assigned Director completes active trip. |
| `/api/admin/trips/{id}/cancel` | POST | Org Admin cancels planned trip. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0017_trip_lifecycle.sql` | Create | Persist trip status and transition metadata. |
| `internal/store/trips.go` | Modify | Add lifecycle fields to trip model and reads. |
| `internal/store/trip_lifecycle.go` | Create | Readiness and transition mutations. |
| `internal/httpapi/trip_lifecycle_handlers.go` | Create | Lifecycle API handlers. |
| `web/src/admin/pages/Trips.tsx` | Modify | Status chips and filters. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Lifecycle banner/actions/readiness. |
| `web/src/admin/pages/Overview.tsx` | Modify | Persisted lifecycle counts. |
| `web/src/pages/GuestRegistration.tsx` | Modify | Closed/cancelled trip state. |

## Definition of Done

- [ ] Trips have persisted lifecycle status and transition metadata.
- [ ] Existing imported trips default safely to `planned`.
- [ ] Import upserts do not reset lifecycle status.
- [ ] Assigned Directors can start planned assigned trips.
- [ ] Assigned Directors can complete active assigned trips.
- [ ] Org Admins can cancel planned trips.
- [ ] Org Admins cannot start/complete trips.
- [ ] Unassigned Directors cannot transition unrelated trips.
- [ ] Completed/cancelled trips block operational mutations.
- [ ] Cancelled/completed trip registration links no longer accept save/submit.
- [ ] Lifecycle events are audit logged with safe metadata.
- [ ] UI shows lifecycle status, readiness, and transition controls.
- [ ] `npm --prefix web run build` passes.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.

## Security Considerations

- Every lifecycle read/mutation is organization scoped.
- Start/complete require assigned Director role, not merely any staff
  session.
- Cancel requires Org Admin role.
- Guest registration must not expose cancelled/completed trip details
  beyond a minimal status message.
- Audit metadata must not include full registration payloads, document
  paths, or secrets.
- Historical guest, document, folio, and trip records are retained.

## Dependencies

- Sprint 014 guest registration.
- Sprint 015 guest folio checkout.
- Sprint 016 cabin assignments.
- Sprint 017 documents and audit.

## Open Questions

1. Are missing guest documents hard blockers for start, or warnings with
   mandatory acknowledgement until required categories are configurable?
2. Can a trip be completed with open folios if the Director records an
   exception, or should open folios always block completion?
3. Should Admins have emergency override for start/complete, or remain
   read/cancel only?
4. Should imports mark missing source trips as cancelled instead of
   deleting stale planned trips when historical child records exist?
