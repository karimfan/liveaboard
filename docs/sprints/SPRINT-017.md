# Sprint 017: Audit Log and Guest Document Management

## Overview

Sprint 017 adds two operational foundations: an append-only audit log
and guest document management. Guests must be able to upload required
documents during trip registration before trip start. Org Admins and
assigned Cruise Directors can view, upload, download, and archive those
documents from the staff guest profile page.

This sprint also adds an org-wide audit surface. Audit is not reporting
or analytics; it is operational accountability: who changed a guest,
registration, cabin assignment, folio, inventory item, payment setting,
or document, and when. Trip lifecycle enforcement will come next, but
this sprint creates the document and audit foundation that lifecycle
checklists will use.

## Use Cases

1. **Guest uploads documents during registration.** A guest uploads
   passport/travel document, dive certification, dive insurance,
   liability waiver, medical, or other documents while completing trip
   registration.
2. **Staff uploads on behalf of a guest.** Org Admin or assigned Cruise
   Director uploads a missing or corrected document from the guest
   profile.
3. **Staff reviews documents.** Staff sees document category, display
   name, original filename, uploader, uploaded time, size, and archive
   status on the guest profile.
4. **Inline view with download option.** Staff can open viewable
   documents inline and can explicitly download any document.
5. **Archive outdated documents.** Staff archives incorrect or expired
   files while preserving metadata and history.
6. **Guest activity timeline.** Staff sees guest-specific audit events
   on the guest profile.
7. **Org-wide audit search.** Org Admin can search audit events across
   the organization; assigned Cruise Directors can view events scoped to
   their assigned trips.
8. **Secure access.** Unassigned Directors cannot view guest documents
   or audit events for unrelated trips. Guest sessions cannot use staff
   document or audit endpoints.

## Architecture

### Core Rules

- Audit events are append-only and enforced by database trigger.
- Every audit event includes `organization_id`.
- Audit actors are `staff`, `guest`, or `system`.
- Staff actors use `actor_user_id`; guest actors use
  `actor_guest_user_id`; system events use neither.
- Audit metadata is JSONB and must not contain raw tokens, passwords,
  storage keys, full local paths, full registration payloads, or large
  PII blobs.
- Guest document metadata is stored in Postgres; binary files are stored
  on local disk in Sprint 017.
- API responses never expose raw filesystem paths or storage keys.
- Guest documents are scoped by `organization_id`, `trip_id`, and
  `trip_guest_id`.
- Guest upload endpoints require a guest session and verify the guest
  owns the target `trip_guest`.
- Staff document endpoints require staff session and manifest access:
  Org Admin for any org trip, Cruise Director only assigned trips.
- Archived documents remain visible with muted archived status; hard
  delete is out of scope.
- Trip guest rows are historical records. Guests are revoked or archived
  from active workflows, never hard-deleted from the database.
- Document downloads are audit-worthy. Log one event per successful
  document access endpoint after authorization and file existence check,
  before streaming. Keep the audit row even if streaming fails midway.

### Storage

Use local disk for Sprint 017.

- Config: `LIVEABOARD_DOCUMENTS_DIR`.
- Default local-dev path: `var/uploads/guest-documents`.
- The directory must not be served by the SPA static handler.
- Tests must use `t.TempDir()` or an equivalent temp upload directory.
- Storage key format:

```text
{organization_id}/{trip_guest_id}/{document_id}
```

No user filename is used in the storage path. Original filename is
metadata only.

Write flow:

1. Apply `http.MaxBytesReader` at handler entry.
2. Stream upload into a temp file.
3. Hash bytes with SHA-256 while writing.
4. Validate size, declared content type, sniffed content type, and
   extension fallback for HEIC/HEIF.
5. Insert document metadata.
6. Rename temp file into final storage key.
7. Record audit event.
8. On failure, remove temp/final partial file.

### Accepted Files

Max size: 10 MiB.

Allowed content types:

- `application/pdf`
- `image/jpeg`
- `image/png`
- `image/heic`
- `image/heif`

Validation:

- Browser-provided multipart content type is not trusted alone.
- Use `http.DetectContentType` for PDF/JPEG/PNG.
- HEIC/HEIF may sniff as `application/octet-stream`; allow only when
  declared type and extension are HEIC/HEIF and a minimal magic/brand
  check passes.
- Store safe display metadata, not raw paths.

Download/view:

- Use `Content-Type` from validated metadata.
- Use sanitized filename in `Content-Disposition`.
- Prefer `inline` for browser-viewable files.
- UI provides a separate explicit download action. HEIC/HEIF may not
  render in all browsers, so staff can use the download action.

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

CREATE INDEX audit_events_action_idx
  ON audit_events(organization_id, action, created_at DESC);
```

Add a trigger that rejects `UPDATE` and `DELETE` on `audit_events`.

### Document Schema

```sql
guest_documents (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  uploaded_by_user_id uuid null references users(id) on delete set null,
  uploaded_by_guest_user_id uuid null references guest_users(id) on delete set null,
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
  content_type text not null check (content_type in (
    'application/pdf',
    'image/jpeg',
    'image/png',
    'image/heic',
    'image/heif'
  )),
  size_bytes bigint not null check (size_bytes > 0),
  sha256_hex char(64) not null,
  storage_key text not null,
  notes text null,
  archived_at timestamptz null,
  archived_by_user_id uuid null references users(id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (
    (uploaded_by_user_id is not null and uploaded_by_guest_user_id is null) or
    (uploaded_by_user_id is null and uploaded_by_guest_user_id is not null)
  )
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

### Audit Actions and Metadata

Initial action set:

| Action | Entity | Metadata |
|---|---|---|
| `guest.invited` | `trip_guest` | `{guest_email_domain, berth_label}` |
| `guest.invite_resent` | `trip_guest` | `{guest_email_domain}` |
| `guest.invite_revoked` | `trip_guest` | `{reason}` optional |
| `guest.registration_saved` | `guest_registration` | `{status}` |
| `guest.registration_submitted` | `guest_registration` | `{status}` |
| `guest.cabin_assigned` | `trip_cabin_assignment` | `{display_label}` |
| `guest.cabin_unassigned` | `trip_cabin_assignment` | `{previous_display_label}` |
| `guest.document_uploaded` | `guest_document` | `{category, display_name, content_type, size_bytes}` |
| `guest.document_downloaded` | `guest_document` | `{category, display_name, content_type}` |
| `guest.document_archived` | `guest_document` | `{category, display_name}` |
| `guest.folio_opened` | `guest_folio` | `{status}` |
| `guest.folio_line_added` | `guest_folio_line` | `{line_type, item_name, quantity}` |
| `guest.folio_line_updated` | `guest_folio_line` | `{line_type, quantity}` |
| `guest.folio_line_deleted` | `guest_folio_line` | `{line_type, item_name}` |
| `guest.folio_closed` | `guest_folio` | `{payment_method, settlement_currency, total_usd_cents}` |
| `inventory.adjusted` | `stock_movement` | `{movement_type, delta_quantity, catalog_item_id}` |
| `organization.payment_settings_updated` | `organization_payment_settings` | `{default_currency, supported_currencies, card_fee_basis_points}` |

Repeated invite resends should be grouped/collapsed in the UI when
rendered close together.

`stock_movements` already serves as an inventory audit source. Sprint
017 also records `inventory.adjusted` audit events for org-wide audit
consistency; this duplication is intentional.

### Transaction Ownership

Audit should be transactionally coupled where the domain mutation already
has or should have a transaction boundary.

Add either `...Tx` variants or handler/service-owned transaction wrappers
for:

- `CreateTripGuestInvite`
- `ResendTripGuestInvite`
- `RevokeTripGuestInvite`
- `SaveGuestRegistration`
- `AssignTripGuestBerth`
- `UnassignTripGuestBerth`
- `OpenGuestFolio`
- `AddGuestFolioLine`
- `UpdateGuestFolioLine`
- `DeleteGuestFolioLine`
- `CloseGuestFolio`
- `AdjustStock`
- `UpdatePaymentSettings`
- guest document metadata create/archive

Where a mutation sends email after commit, audit the durable state
change, not every email attempt unless the state change itself records
email status.

### Store and Service Layer

Create:

- `internal/store/audit.go`
- `internal/store/guest_documents.go`
- `internal/documents/documents.go`

Audit helpers:

- `AuditEvent`
- `AuditActor`
- `AuditEventInput`
- `RecordAuditEvent(ctx, input)`
- `RecordAuditEventTx(ctx, tx, input)`
- `AuditEvents(ctx, orgID, filters)`
- `AuditEventsForTripGuest(ctx, orgID, tripID, tripGuestID, limit)`

Document helpers:

- `GuestDocument`
- `GuestDocumentInput`
- `CreateGuestDocument(ctx, input)`
- `GuestDocumentsForTripGuest(ctx, orgID, tripID, tripGuestID, includeArchived)`
- `GuestDocumentByID(ctx, orgID, tripID, tripGuestID, documentID)`
- `ArchiveGuestDocument(ctx, orgID, tripID, tripGuestID, documentID, actorID)`

Document service:

- `UploadGuestDocument`
- `OpenGuestDocument`
- `ArchiveGuestDocument`
- `RemoveOrphanedFile`

### HTTP API

Guest-session routes:

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/guest/trip-registrations/{trip_guest_id}/documents` | GET | List the guest's documents for this trip. |
| `/api/guest/trip-registrations/{trip_guest_id}/documents` | POST | Upload a document during registration. |
| `/api/guest/trip-registrations/{trip_guest_id}/documents/{document_id}` | GET | Inline view/download own document. |

Staff routes:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/guests/{guest_id}/documents` | GET | Admin or assigned CD | List guest documents. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents` | POST | Admin or assigned CD | Upload a guest document. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents/{document_id}` | GET | Admin or assigned CD | Inline view/download document. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents/{document_id}` | DELETE | Admin or assigned CD | Archive document. |
| `/api/admin/trips/{id}/guests/{guest_id}/activity` | GET | Admin or assigned CD | Guest activity timeline. |
| `/api/admin/audit-events` | GET | Admin or assigned CD | Org-wide or assigned-trip audit search. |

`/api/admin/audit-events` behavior:

- Org Admin sees all org events.
- Cruise Director sees events only for trips assigned to them.
- Filters: `action`, `entity_type`, `trip_id`, `trip_guest_id`,
  `actor_type`, `date_from`, `date_to`, `limit`.
- Sprint 017 uses most-recent-first plus `limit`; cursor pagination is
  deferred.

Multipart upload fields:

- `file`
- `category`
- `display_name` optional
- `notes` optional

Route safety:

- Staff document/activity handlers must verify `trip_guest_id` belongs
  to the route `trip_id`; `authorizeManifestAccess` alone is not
  enough.
- Guest document handlers use `AssertTripGuestAccess`.

### Frontend

Guest registration page:

- Add Documents section to `/guest/trips/:tripGuestId/register`.
- Show accepted type/size hint.
- Allow upload/list/view/download of guest's own documents.
- Show missing recommended categories as preparation guidance. Hard
  trip-start gating is deferred until trip lifecycle sprint.

Staff guest profile page:

- Keep existing page structure; do not add routed tabs.
- Add Documents section after the summary.
- Add Activity section after Documents/Registration.
- Documents section includes staff upload form, list, inline view link,
  download link, and archive action.
- Activity section uses human labels, not raw action names where
  possible.

Org-wide audit page:

- Add Admin nav item or Organization submenu entry for Audit.
- Use dense filters and table: time, actor, action, entity, trip,
  guest, summary.
- Admin can search all org events; Director view is scoped to assigned
  trips.

Apply `DESIGN.md`: DM Sans/Geist typography, slate working surfaces,
amber accents, dense operational layout, no decorative imagery.

## Implementation Plan

### Phase 1: Schema, Config, Store (~30%)

**Files:**

- `internal/store/migrations/0015_audit_and_guest_documents.sql`
- `internal/config/config.go`
- `internal/store/audit.go`
- `internal/store/guest_documents.go`
- `internal/store/audit_test.go`
- `internal/store/guest_documents_test.go`
- `internal/testdb/testdb.go`

**Tasks:**

- [ ] Add `audit_events` table, indexes, and immutable trigger.
- [ ] Add `guest_documents` table and indexes.
- [ ] Add `LIVEABOARD_DOCUMENTS_DIR` config with local dev default.
- [ ] Add audit store helpers and filters.
- [ ] Add document metadata store helpers.
- [ ] Add required tx variants/wrappers for audited mutations.
- [ ] Update test truncation order: `guest_documents`, then
      `audit_events`, before user/guest/trip tables.
- [ ] Add store tests for append-only trigger, tenant isolation,
      document archive, document lookup scoping, and audit filtering.

### Phase 2: Document Service and HTTP API (~25%)

**Files:**

- `internal/documents/documents.go`
- `internal/httpapi/document_handlers.go`
- `internal/httpapi/activity_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/document_handlers_test.go`

**Tasks:**

- [ ] Add local filesystem document service with temp-file write,
      SHA-256 hash, validation, and rename.
- [ ] Enforce `http.MaxBytesReader`, size cap, content-type policy, byte
      sniffing, and HEIC/HEIF fallback checks.
- [ ] Add guest document list/upload/view endpoints.
- [ ] Add staff document list/upload/view/archive endpoints.
- [ ] Add guest activity endpoint.
- [ ] Add org-wide audit endpoint with filters.
- [ ] Verify route trip/guest ownership on every staff guest document
      and activity request.
- [ ] Emit audit events for document upload/download/archive.
- [ ] Add HTTP tests for Admin, assigned Director, unassigned Director,
      trip/guest mismatch, guest session denial, MIME spoofing, path
      traversal filename, archive behavior, and download headers.

### Phase 3: Audit Existing Mutations (~20%)

**Files:**

- `internal/auth/guest_accounts.go`
- `internal/httpapi/guest_registration_handlers.go`
- `internal/httpapi/guest_manifest_handlers.go`
- `internal/httpapi/cabin_handlers.go`
- `internal/httpapi/guest_folio_handlers.go`
- `internal/httpapi/inventory_handlers.go`
- `internal/httpapi/payment_settings_handlers.go`
- Store files owning transaction boundaries.

**Tasks:**

- [ ] Record audit events for guest invite, resend, and revoke.
- [ ] Record guest actor audit events for registration save/submit.
- [ ] Record audit events for cabin assignment/unassignment.
- [ ] Record audit events for folio open, line add/update/delete, and
      close.
- [ ] Record audit events for inventory adjustment.
- [ ] Record audit events for payment settings update.
- [ ] Add one focused test per wired action family, asserting
      `actor_type`, `entity_type`, action, and expected metadata keys.
- [ ] Add metadata-safety tests proving no raw tokens, storage paths, or
      full request payloads are recorded.

### Phase 4: Frontend Guest Docs, Staff Profile, Audit Page (~20%)

**Files:**

- `web/src/lib/api.ts`
- `web/src/pages/GuestRegistration.tsx`
- `web/src/admin/api.ts`
- `web/src/admin/Shell.tsx`
- `web/src/admin/pages/TripGuestDetail.tsx`
- `web/src/admin/pages/AuditEvents.tsx`
- `web/src/main.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [ ] Add guest document API methods.
- [ ] Add staff document, activity, and audit API methods.
- [ ] Add document upload/list/view/download section to guest
      registration.
- [ ] Add Documents section to staff guest profile.
- [ ] Add Activity section to staff guest profile.
- [ ] Add org-wide Audit page with filters and scoped Director view.
- [ ] Use inline view links plus explicit download links.
- [ ] Add upload pending/success/error states and client-side size/type
      hints.
- [ ] Apply `DESIGN.md` typography, spacing, color, and density rules.

### Phase 5: Product Docs and Verification (~5%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-017.md`
- `docs/sprints/tracker.tsv`

**Tasks:**

- [ ] Update persona boundaries for guest upload, staff document
      management, and audit visibility.
- [ ] Add user stories for guest documents and audit search.
- [ ] Run `go run docs/sprints/tracker.go sync`.
- [ ] Run `gofmt` on Go changes.
- [ ] Run Prettier or project-equivalent formatting for TypeScript.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `npm run build` in `web`.
- [ ] Run `git diff --check`.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/guest/trip-registrations/{trip_guest_id}/documents` | GET | Guest lists own trip documents. |
| `/api/guest/trip-registrations/{trip_guest_id}/documents` | POST | Guest uploads own trip document. |
| `/api/guest/trip-registrations/{trip_guest_id}/documents/{document_id}` | GET | Guest inline views/downloads own document. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents` | GET | Staff lists guest documents. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents` | POST | Staff uploads guest document. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents/{document_id}` | GET | Staff inline views/downloads document. |
| `/api/admin/trips/{id}/guests/{guest_id}/documents/{document_id}` | DELETE | Staff archives guest document. |
| `/api/admin/trips/{id}/guests/{guest_id}/activity` | GET | Staff reads guest activity timeline. |
| `/api/admin/audit-events` | GET | Staff reads scoped audit search results. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0015_audit_and_guest_documents.sql` | Create | Audit and document metadata schema. |
| `internal/config/config.go` | Modify | Document storage directory config. |
| `internal/store/audit.go` | Create | Audit event store helpers. |
| `internal/store/guest_documents.go` | Create | Guest document metadata helpers. |
| `internal/documents/documents.go` | Create | Local file storage service. |
| `internal/httpapi/document_handlers.go` | Create | Guest/staff document endpoints. |
| `internal/httpapi/activity_handlers.go` | Create | Guest activity and org audit endpoints. |
| Existing mutation handlers/store methods | Modify | Emit audit events. |
| `internal/testdb/testdb.go` | Modify | Truncate new tables. |
| `web/src/lib/api.ts` | Modify | Guest document API. |
| `web/src/pages/GuestRegistration.tsx` | Modify | Guest document upload/list. |
| `web/src/admin/api.ts` | Modify | Staff document/activity/audit API. |
| `web/src/admin/Shell.tsx` | Modify | Add Audit navigation. |
| `web/src/admin/pages/TripGuestDetail.tsx` | Modify | Documents and Activity sections. |
| `web/src/admin/pages/AuditEvents.tsx` | Create | Org-wide/scoped audit page. |
| `web/src/main.tsx` | Modify | Route audit page. |
| `web/src/styles/app.css` | Modify | Document and audit UI styles. |
| Product docs | Modify | Persona and story updates. |

## Definition of Done

- [ ] Guests can upload/list/view/download their own trip documents from
      registration.
- [ ] Staff can upload/list/view/download/archive guest documents from
      guest profile.
- [ ] Document upload validates category, size, content type, sniffed
      bytes, and safe filename handling.
- [ ] Documents open inline where browser-supported and expose a
      download option.
- [ ] Document APIs never expose local paths or storage keys.
- [ ] Audit events persist for scoped Sprint 017 mutations.
- [ ] Audit events are append-only at the database layer.
- [ ] Staff guest profile shows activity timeline.
- [ ] Org-wide Audit page exists with filters and Director scoping.
- [ ] Org Admin can access all org document/activity/audit data.
- [ ] Cruise Director can access only assigned-trip document/activity
      and audit data.
- [ ] Guest sessions cannot access staff document/activity/audit routes.
- [ ] Document actions emit audit events.
- [ ] Metadata safety tests prevent raw tokens, storage paths, and large
      PII payloads.
- [ ] Product docs reflect guest upload, staff document management, and
      audit scope.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `npm run build` passes.
- [ ] `git diff --check` passes.

## Security Considerations

- Guest documents may contain passport, medical, insurance, waiver, and
  certification data.
- Staff access must verify both route trip authorization and that the
  `trip_guest` belongs to that route trip.
- Guest document routes must verify authenticated guest ownership via
  `AssertTripGuestAccess`.
- Local storage paths and storage keys are internal only.
- Original filenames must be sanitized for response headers and never
  used in filesystem paths.
- Multipart upload uses `http.MaxBytesReader` to reduce disk-fill risk.
- Server validates bytes and content type; browser headers are not
  trusted alone.
- Audit metadata must avoid secrets, raw tokens, storage paths, full
  request payloads, and large PII blobs.
- Cookie-auth multipart endpoints should preserve existing session
  protections and reject guest cookies on staff routes.

## Dependencies

- Sprint 014 guest registration and staff guest detail.
- Sprint 015 guest folio checkout.
- Sprint 016 cabin assignment and staff trip scoping.
- `DESIGN.md` for guest/staff document and audit UI.

## Risks and Mitigations

- **Transactional audit complexity.** Explicitly add tx variants or
  service-owned transaction wrappers for audited mutations.
- **Sensitive metadata leakage.** Specify metadata shapes and add safety
  tests against raw tokens, paths, and full payloads.
- **MIME spoofing.** Validate with byte sniffing and HEIC/HEIF fallback
  checks, not browser headers alone.
- **Browser HEIC preview limitations.** Provide explicit download
  action and keep inline best-effort.
- **Director cross-trip leakage.** Test mismatched route trip/guest IDs.
- **Disk accumulation in dev.** Tests use temp dirs; local cleanup of
  `var/uploads/guest-documents` is acceptable in development.

## References

- `CLAUDE.md`
- `DESIGN.md`
- `docs/sprints/SPRINT-014.md`
- `docs/sprints/SPRINT-015.md`
- `docs/sprints/SPRINT-016.md`
- `docs/sprints/drafts/SPRINT-017-INTENT.md`
- `docs/sprints/drafts/SPRINT-017-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-017-CODEX-DRAFT-CLAUDE-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-017-MERGE-NOTES.md`
