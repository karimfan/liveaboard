# Sprint 017 Intent: Audit Log and Guest Document Management

## Seed

kplan implement 7 and 8. for 8 (document management), this should all be done via viewing a guest's profile which is a function available for the directors and admins

Interpreted scope:

- **7: Audit Log** from the previous product roadmap answer.
- **8: Document Management** as clarified by the user, even though the earlier numbered list called document management item 9. This sprint should implement staff document management through the guest profile/detail view, available to Org Admins and assigned Cruise Directors.

## Context

Liveaboard now has core trip guest workflows:

- Staff can invite guests to trips.
- Guests can create guest accounts and complete trip registration.
- Staff can view per-guest registration details.
- Staff can assign guests to boat berth slots.
- Staff can close per-guest folios.

The system now needs accountability and document handling. Operators need to see who changed guest, cabin, folio, inventory, and settings state. They also need to upload/view staff-managed guest documents such as passport images, certification cards, insurance proof, liability waivers, and medical notes/files from the per-guest profile/detail page. Guest-facing upload can come later.

## Recent Sprint Context

### Sprint 014: Guest Management and Trip Registration

Added guest accounts, guest sessions, trip guests, invitations, and guest registration payloads. Binary document upload was explicitly deferred; passport/certification/liability items were represented as acknowledgements or notes only.

### Sprint 015: Guest Folio Checkout and Payment Settings

Added per-guest folio checkout, org payment settings, and closed-folio email. Folio close snapshots money data and is immutable, but there is no cross-feature audit log yet.

### Sprint 016: Cabin Layouts and Guest Berth Assignments

Added reusable boat cabin layouts and mandatory berth assignment during guest enrollment. Directors and Admins can change layouts and assignments in their permitted scope. Assignment rows have actor-ish fields, but there is no unified audit activity surface.

## Relevant Codebase Areas

- `internal/store/trip_guests.go` — guest invite, revoke, manifest, and staff guest rows.
- `internal/store/guest_registrations.go` — registration save/submit.
- `internal/store/guest_folios.go` — folio lines and close.
- `internal/store/cabins.go` — cabin layout and berth assignment mutations.
- `internal/store/inventory.go` — stock movements already have actor/source fields; should feed audit or be referenced by audit.
- `internal/store/payment_settings.go` — org payment settings mutation.
- `internal/httpapi/guest_manifest_handlers.go` — staff guest profile/registration detail handler and manifest authorization.
- `internal/httpapi/guest_folio_handlers.go`, `cabin_handlers.go`, `inventory_handlers.go`, `payment_settings_handlers.go` — mutation handlers needing audit events.
- `web/src/admin/pages/TripGuestDetail.tsx` — staff guest profile/details page where documents should live.
- `web/src/admin/api.ts` — API types and endpoints.
- `web/src/styles/app.css` — staff profile/document UI styling.
- `internal/testdb/testdb.go` — truncation order for new tables.

## Constraints

- Must follow project conventions in `CLAUDE.md`.
- Must follow UI direction in `DESIGN.md`.
- Must integrate with existing architecture and sprint conventions.
- Work directly on `main` when implementing; no feature branches.
- Every tenant-owned table must include `organization_id`.
- Staff document access must be limited to Org Admins or Cruise Directors assigned to the trip containing the `trip_guest`.
- Guest sessions must not access staff document endpoints in this sprint.
- Binary storage is local-dev only for now. Cloud object storage can come later.
- Uploaded documents may contain sensitive personal information, so API responses must not expose raw filesystem paths.
- Audit logging must not block primary operations if it can be safely best-effort, except for security-relevant document actions where recording should be part of the transaction if possible.

## Success Criteria

- A durable audit event table records important staff and guest-driven mutations with actor, org, entity, action, timestamp, and metadata.
- Staff guest profile shows a concise audit/activity timeline for that guest.
- Guest document records can be uploaded, listed, downloaded/viewed, and archived from the staff guest profile page.
- Document categories cover passport/travel document, dive certification, dive insurance, waiver, medical, and other.
- Uploaded files are validated for size and content type.
- Staff document access is scoped by organization and assigned-trip authorization.
- Document upload/download/archive actions generate audit events.
- Major existing guest-facing and staff-facing guest mutations emit audit events.
- Tests cover tenant isolation, director scoping, guest-session denial, upload validation, archive behavior, and audit creation.

## Open Questions

- Should uploaded files be stored on disk under a local configured directory, or in Postgres bytea for Sprint 017?
- Should guests be allowed to upload their own documents now, or is staff-only document management enough?
- Should audit events be immutable append-only with no UI for deletion?
- Should audit events be org-wide searchable in this sprint, or only surfaced on a guest profile?
- Which file types and maximum file size should be accepted initially?
