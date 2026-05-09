# Sprint 018 Intent: Trip Lifecycle and Readiness Gates

## Seed

kplan Trip lifecycle (see your earlier response on some functionality for this)

## Context

The product now has the operational prerequisites for explicit trip
lifecycle: guest invites and registration, mandatory cabin/berth
assignment, folio checkout, guest documents, and an audit log. Trips
currently have dates and are sometimes classified as upcoming/active/past
from calendar math, but there is no persisted lifecycle state or
transition workflow. Product docs already establish the target boundary:
Org Admin creates/configures/cancels planned trips; Cruise Directors
start planned trips and complete active trips; Admins retain oversight
read access. Guest rows must not be hard-deleted.

## Recent Sprint Context

- Sprint 015 added one end-of-trip folio per guest/trip, offline payment
  closure, folio email, org payment settings, stock decrement on close,
  and immutable closed folios.
- Sprint 016 added reusable boat cabin layouts and mandatory berth
  assignment when adding a guest to a trip.
- Sprint 017 added guest document upload/management, org-wide and
  assigned-trip audit events, guest activity, private local document
  storage, and a trigger preventing hard deletes of `trip_guests`.

## Relevant Codebase Areas

- `internal/store/trips.go` owns trip reads/import upserts and currently
  lacks lifecycle fields.
- `internal/store/migrations/0005_boats_and_trips.sql` created `trips`;
  Sprint 018 likely needs a new migration to add lifecycle columns.
- `internal/httpapi/admin.go`, `cruise_director.go`, and `httpapi.go`
  expose trip list/overview endpoints and currently infer date status in
  the Cruise Director landing page.
- `internal/httpapi/guest_manifest_handlers.go`,
  `guest_registration_handlers.go`, `document_handlers.go`,
  `guest_folio_handlers.go`, and cabin handlers are the readiness inputs.
- `internal/store/audit.go` and `internal/httpapi/audit_helpers.go`
  should be reused for lifecycle audit events.
- Frontend surfaces: `web/src/admin/pages/Trips.tsx`,
  `TripManifest.tsx`, `TripGuestDetail.tsx`, `Overview.tsx`,
  `admin/api.ts`, and `lib/api.ts`.
- Product docs: `docs/product/personas.md` and
  `docs/product/organization-admin-user-stories.md`.

## Constraints

- Must follow project conventions in `CLAUDE.md`.
- Must integrate with existing Go store/handler architecture and React
  admin chrome.
- Must follow sprint conventions in `docs/sprints/README.md`.
- Must preserve historical records: trip guests are revoked, not deleted.
- Must maintain strict organization scoping and assigned-trip Director
  scoping.
- Must not process payments online.
- Must not block planning on analytics/reporting; lifecycle is an
  operational workflow, not a reporting sprint.

## Success Criteria

- Trips have explicit persisted statuses: `planned`, `active`,
  `completed`, and `cancelled`.
- Assigned Cruise Directors can start only assigned planned trips when
  readiness checks pass or warnings are consciously acknowledged.
- Assigned Cruise Directors can complete only assigned active trips when
  checkout/document/manifest completion rules are satisfied or approved
  exceptions are recorded.
- Org Admins can cancel planned trips but cannot start/complete trips.
- Admins can see lifecycle status and readiness across the trip list and
  trip manifest/detail surfaces.
- Lifecycle transitions emit safe audit events.
- Date-based “upcoming/active/past” classification is replaced or
  clearly separated from persisted operational status.

## Open Questions

1. Should starting a trip require all active guests to have submitted
   registration, at least one active document, and a berth assignment, or
   should missing documents/registration be warning-only?
2. Should completing a trip require every active guest folio to be
   closed, or should Directors be able to complete with recorded
   exceptions?
3. Should Org Admins have an emergency override for start/complete, or
   remain read/cancel only per persona boundaries?
4. Should cancelled trips preserve guest invite links as unusable and
   hide guest registration access immediately?
