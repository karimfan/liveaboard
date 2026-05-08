# Sprint 017 Codex Draft: Audit Log and Guest Document Management

## Overview

Sprint 017 adds two connected operational foundations: a durable audit
log for important mutations and staff-managed guest documents. Document
management lives inside the staff guest profile/detail page, available
to Org Admins and assigned Cruise Directors. Staff can upload, view,
download, and archive guest documents such as passport images, dive
certification cards, dive insurance proof, liability waivers, medical
documents, and other operator files.

The audit log is intentionally broader than document management but
does not try to become a full reporting product. It creates a reusable
append-only event table and helper, then wires the highest-value
mutation paths: guest invitation/revoke, registration submit, cabin
assignment/unassignment, document upload/archive/download, folio line
changes/close, inventory adjustments, and payment settings updates.
The first UI surface for audit events is the guest profile activity
timeline.

## Use Cases

1. **Upload a guest document.** Org Admin or assigned Cruise Director
   opens a guest profile and uploads passport, certification, insurance,
   waiver, medical, or other documents.
2. **View document readiness.** Staff sees each guest's uploaded
   documents, category, filename, upload time, uploader, and archive
   status from the guest profile.
3. **Download/view a document.** Staff downloads or views a document
   without seeing raw filesystem paths.
4. **Archive a document.** Staff archives an outdated or incorrect
   document while preserving metadata and audit history.
5. **Inspect guest activity.** Staff sees a chronological guest activity
   timeline showing invitation, registration, cabin assignment, folio,
   and document events.
6. **Preserve accountability.** Important staff actions record actor,
   organization, trip, guest, entity, action, and compact metadata.
7. **Deny unauthorized access.** Unassigned Cruise Directors and guest
   sessions cannot view or manipulate staff document records.

## Architecture

### Core Rules

- Audit events are append-only.
- Every audit event includes `organization_id`.
- Audit events should include `actor_user_id` for staff actions,
  `actor_guest_user_id` for guest actions, or `actor_type = system`
  for automated events.
- Audit metadata is JSONB and must avoid storing secrets, raw tokens,
  passwords, full file paths, or large payloads.
- Document binary storage is an implementation detail; API responses
  expose stable document IDs and safe metadata, never raw paths.
- Document records are scoped to `organization_id`, `trip_id`, and
  `trip_guest_id`.
- Staff document endpoints reuse manifest authorization:
  Org Admin can access any org trip guest; Cruise Director only assigned
  trips.
- Guests cannot access document endpoints in Sprint 017.
- Archived documents remain listed by default on staff guest profile
  with muted status; hard delete is out of scope.
- Downloads are audit-worthy because documents may contain sensitive
  PII.

### Storage Choice

Recommended Sprint 017 choice: store binary files on local disk under a
configured directory such as `var/uploads/guest-documents`, while
storing metadata in Postgres.

Rationale:

- Local development only is the current environment.
- Avoids pushing potentially large binary data into Postgres.
- Keeps future cloud object storage migration straightforward because
  metadata can carry `storage_provider`, `storage_key`, size, and hash.

Postgres bytea is simpler operationally for tests but creates worse
long-term habits for document storage. Tests can use a temp upload dir.

### Audit Schema

Migration `0015_audit_and_guest_documents.sql`.

```sql
audit_events (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  actor_type text not null check (actor_type in ('staff','guest','system')),
  actor_user_id uuid null references users(id) on delete set null,
  actor_guest_user_id uuid null references guest_users(id) on delete set null,
  action text not null,
  entity_type text not null,
  entity_id uuid null,
  trip_id uuid null references trips(id) on delete set null,
  trip_guest_id uuid null references trip_guests(id) on delete set null,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  check (
    (actor_type = 'staff' and actor_user_id is not null and actor_guest_user_id is null) or
    (actor_type = 'guest' and actor_guest_user_id is not null and actor_user_id is null) or
    (actor_type = 'system' and actor_user_id is null and actor_guest_user_id is null)
  )
)
```

Indexes:

```sql
CREATE INDEX audit_events_org_created_idx
  ON audit_events(organization_id, created_at DESC);

CREATE INDEX audit_events_trip_guest_idx
  ON audit_events(organization_id, trip_guest_id, created_at DESC)
  WHERE trip_guest_id IS NOT NULL;

CREATE INDEX audit_events_entity_idx
  ON audit_events(organization_id, entity_type, entity_id, created_at DESC)
  WHERE entity_id IS NOT NULL;
```

Initial action names:

- `guest.invited`
- `guest.invite_resent`
- `guest.invite_revoked`
- `guest.registration_saved`
- `guest.registration_submitted`
- `guest.cabin_assigned`
- `guest.cabin_unassigned`
- `guest.document_uploaded`
- `guest.document_downloaded`
- `guest.document_archived`
- `guest.folio_opened`
- `guest.folio_line_added`
- `guest.folio_line_updated`
- `guest.folio_line_deleted`
- `guest.folio_closed`
- `inventory.adjusted`
- `organization.payment_settings_updated`

### Document Schema

```sql
guest_documents (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  uploaded_by_user_id uuid not null references users(id) on delete restrict,
  category text not null check (category in (
    'travel_document',
    'dive_certification',
    'dive_insurance',
    'liability_waiver',
    'medical',
    'other'
  )),
  display_name text not null,
  original_filename text not null,
  content_type text not null,
  size_bytes bigint not null check (size_bytes > 0),
  sha256_hex char(64) not null,
  storage_provider text not null default 'local',
  storage_key text not null,
  notes text null,
  archived_at timestamptz null,
  archived_by_user_id uuid null references users(id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

Indexes:

```sql
CREATE INDEX guest_documents_trip_guest_idx
  ON guest_documents(organization_id, trip_guest_id, created_at DESC);

CREATE INDEX guest_documents_active_category_idx
  ON guest_documents(organization_id, trip_guest_id, category)
  WHERE archived_at IS NULL;
```

### File Validation

Initial limits:

- Max size: 10 MiB per file.
- Allowed content types:
  - `application/pdf`
  - `image/jpeg`
  - `image/png`
  - `image/heic`
  - `image/heif`
- Filename is stored for display but not trusted as a path.
- Storage key is generated server-side using org/trip_guest/document IDs.
- Server computes SHA-256 while writing the file.

### Store Layer

Create:

- `internal/store/audit.go`
- `internal/store/guest_documents.go`

Audit types/methods:

- `AuditEvent`
- `AuditActor`
- `AuditEventInput`
- `RecordAuditEvent(ctx, input)`
- `RecordAuditEventTx(ctx, tx, input)`
- `AuditEventsForTripGuest(ctx, orgID, tripID, tripGuestID, limit)`

Document types/methods:

- `GuestDocument`
- `GuestDocumentInput`
- `CreateGuestDocument(ctx, input)`
- `GuestDocumentsForTripGuest(ctx, orgID, tripID, tripGuestID, includeArchived)`
- `GuestDocumentByID(ctx, orgID, tripID, tripGuestID, documentID)`
- `ArchiveGuestDocument(ctx, orgID, tripID, tripGuestID, documentID, actorID)`

Document binary writing can live in a small service under
`internal/documents` so store does not know filesystem details:

- `Service.UploadGuestDocument`
- `Service.OpenGuestDocument`
- `Service.ArchiveGuestDocument`

### HTTP API

Endpoints are staff-only under existing authenticated admin routes:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/guests/{guest_id}/documents` | GET | Admin or assigned CD | List guest documents. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents` | POST | Admin or assigned CD | Multipart upload a guest document. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents/{document_id}` | GET | Admin or assigned CD | Download/view document. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents/{document_id}` | DELETE | Admin or assigned CD | Archive document. |
| `/api/admin/trips/{id}/guests/{guest_id}/activity` | GET | Admin or assigned CD | Guest activity timeline. |

Multipart upload fields:

- `file`
- `category`
- `display_name` optional; defaults to original filename without path.
- `notes` optional.

Download response:

- `Content-Type` from stored metadata.
- `Content-Disposition: inline; filename="safe-name.ext"` for viewable
  files or attachment if implementation chooses.
- No JSON wrapper for file download.

### Staff Guest Profile UI

Extend `web/src/admin/pages/TripGuestDetail.tsx`.

Tabs/sections:

- Summary
- Registration
- Documents
- Activity

Documents section:

- Upload form with category, display name, notes, file picker.
- Accepted-file hint: PDF, JPEG, PNG, HEIC/HEIF up to 10 MiB.
- Table/list with category, name, filename, uploaded by, uploaded at,
  size, archived status, actions.
- Download/view action.
- Archive action with confirmation.

Activity section:

- Reverse chronological list.
- Compact actor/action/time text.
- Metadata rendered only as safe labels, not raw JSON blobs unless
  necessary.

Use `DESIGN.md`: dense operational surfaces, no decorative hero, no
tourism imagery.

### Audit Wiring Scope

Sprint 017 should wire audit into these actions:

- `InviteTripGuest`
- `ResendTripGuestInvite`
- `RevokeTripGuestInvite`
- guest registration save/submit
- cabin assignment/unassignment
- document upload/download/archive
- folio open, line add/update/delete, close
- inventory manual adjustment
- payment settings update

Where transaction ownership already exists, use `RecordAuditEventTx`.
For post-commit side effects like email status, record only the durable
state change, not every email attempt unless already explicit in the
domain.

## Implementation Plan

### Phase 1: Schema and Store (~30%)

**Files:**

- `internal/store/migrations/0015_audit_and_guest_documents.sql`
- `internal/store/audit.go`
- `internal/store/guest_documents.go`
- `internal/store/audit_test.go`
- `internal/store/guest_documents_test.go`
- `internal/testdb/testdb.go`

**Tasks:**

- [ ] Add `audit_events` table and indexes.
- [ ] Add `guest_documents` table and indexes.
- [ ] Add audit store helpers, including transactional helper.
- [ ] Add guest document metadata store helpers.
- [ ] Update test truncation order.
- [ ] Add tests for append-only audit, trip_guest timeline ordering,
      tenant isolation, document archive, and document lookup scoping.

### Phase 2: Document Service and HTTP API (~25%)

**Files:**

- `internal/documents/documents.go`
- `internal/httpapi/document_handlers.go`
- `internal/httpapi/activity_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/document_handlers_test.go`

**Tasks:**

- [ ] Add local filesystem document service using generated storage
      keys.
- [ ] Enforce size and content-type validation.
- [ ] Add document list/upload/download/archive endpoints.
- [ ] Add guest activity endpoint.
- [ ] Reuse `authorizeManifestAccess` and verify `trip_guest_id`
      belongs to the route trip.
- [ ] Emit audit events for upload/download/archive.
- [ ] Add HTTP tests for Admin, assigned Director, unassigned Director,
      guest session denial, upload validation, archive behavior, and
      download headers.

### Phase 3: Audit Wiring for Existing Mutations (~20%)

**Files:**

- `internal/auth/guest_accounts.go`
- `internal/httpapi/guest_registration_handlers.go`
- `internal/httpapi/guest_manifest_handlers.go`
- `internal/httpapi/cabin_handlers.go`
- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/inventory_handlers.go`
- `internal/httpapi/payment_settings_handlers.go`
- `internal/store/*` as needed where transaction ownership lives.

**Tasks:**

- [ ] Record audit events for guest invite, resend, revoke.
- [ ] Record audit events for guest registration save and submit.
- [ ] Record audit events for cabin assignment and unassignment.
- [ ] Record audit events for folio open, line mutation, and close.
- [ ] Record audit events for inventory adjustment.
- [ ] Record audit events for payment settings update.
- [ ] Ensure metadata avoids sensitive payloads and raw tokens.
- [ ] Add focused tests proving representative audit events exist.

### Phase 4: Staff Guest Profile UI (~20%)

**Files:**

- `web/src/admin/api.ts`
- `web/src/admin/pages/TripGuestDetail.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [ ] Add document and activity API types/methods.
- [ ] Add Documents section to guest profile.
- [ ] Add upload form with category, display name, notes, and file
      picker.
- [ ] Add file-type and size hints.
- [ ] Add document list with download and archive actions.
- [ ] Add Activity section with reverse chronological timeline.
- [ ] Apply `DESIGN.md` typography, spacing, color, and density rules.

### Phase 5: Product Docs and Verification (~5%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-017.md`

**Tasks:**

- [ ] Update persona boundaries for staff document management and audit
      visibility.
- [ ] Add/adjust user stories for guest documents and audit timeline.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `npm run build` in `web`.
- [ ] Run `git diff --check`.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0015_audit_and_guest_documents.sql` | Create | Audit and guest document metadata schema. |
| `internal/store/audit.go` | Create | Audit event store helpers. |
| `internal/store/guest_documents.go` | Create | Guest document metadata store helpers. |
| `internal/documents/documents.go` | Create | Local file storage service and validation. |
| `internal/httpapi/document_handlers.go` | Create | Guest document list/upload/download/archive endpoints. |
| `internal/httpapi/activity_handlers.go` | Create | Guest activity timeline endpoint. |
| `internal/httpapi/httpapi.go` | Modify | Route document and activity endpoints. |
| Existing mutation handlers | Modify | Emit audit events. |
| `internal/testdb/testdb.go` | Modify | Truncate new tables. |
| `web/src/admin/api.ts` | Modify | Document/activity API types and methods. |
| `web/src/admin/pages/TripGuestDetail.tsx` | Modify | Documents and activity sections. |
| `web/src/styles/app.css` | Modify | Guest profile document/activity styles. |
| Product docs | Modify | Document/audit persona and story updates. |

## Definition of Done

- [ ] Audit events persist for the scoped Sprint 017 mutation set.
- [ ] Audit events are append-only and scoped by organization.
- [ ] Staff guest profile shows guest activity timeline.
- [ ] Staff can upload guest documents from guest profile.
- [ ] Staff can list active and archived guest documents from guest
      profile.
- [ ] Staff can download/view guest documents without raw storage path
      exposure.
- [ ] Staff can archive guest documents.
- [ ] Document upload validates category, size, and content type.
- [ ] Org Admin can access document/activity endpoints for any org trip.
- [ ] Cruise Director can access document/activity endpoints only for
      assigned trips.
- [ ] Guest sessions cannot access document/activity endpoints.
- [ ] Document actions emit audit events.
- [ ] Product docs reflect staff document management and audit scope.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `npm run build` passes.
- [ ] `git diff --check` passes.

## Security Considerations

- Documents may contain passport, medical, insurance, and certification
  data; every endpoint must enforce staff trip scope.
- API responses must never reveal local filesystem paths.
- File names must be sanitized for response headers.
- MIME type must be validated by server policy, not trusted solely from
  the browser.
- Audit metadata must avoid raw tokens, passwords, complete registration
  payloads, document storage keys, and full PII payloads.
- Local storage directory must not be under SPA static assets.

## Dependencies

- Sprint 014 guest registration and staff guest detail.
- Sprint 015 guest folio checkout.
- Sprint 016 cabin assignment and staff trip scoping.
- `DESIGN.md` for guest profile UI.

## Open Questions

1. Should guests upload their own documents in this sprint, or is
   staff-only upload the intended first version?
2. Is 10 MiB per file acceptable, and are PDF/JPEG/PNG/HEIC/HEIF the
   right initial file types?
3. Should document download be inline where possible or always
   attachment?
4. Should audit have an org-wide searchable view now, or only the guest
   profile activity timeline?
