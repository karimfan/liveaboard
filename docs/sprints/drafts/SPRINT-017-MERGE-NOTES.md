# Sprint 017 Merge Notes

## Codex Draft Strengths

- Correctly paired audit logging with document management because document actions require accountability.
- Chose a reusable `audit_events` table with staff, guest, and system actor types.
- Chose local disk storage plus Postgres metadata for documents, leaving a future path to cloud object storage.
- Scoped staff document access through existing manifest authorization.
- Put document management on the staff guest profile page.
- Included document upload/download/archive and document action audit events.

## Claude Code Critique Strengths

- Identified that transactional audit is not free with current store/service method boundaries.
- Flagged that append-only audit should be enforced or explicitly accepted as code discipline.
- Caught `uploaded_by_user_id ON DELETE RESTRICT` as inconsistent with existing `SET NULL` patterns.
- Tightened file validation around byte sniffing, `MaxBytesReader`, storage-key format, and temp-file rename.
- Flagged HEIC inline rendering as a browser compatibility issue.
- Required explicit metadata shapes so audit rows remain queryable and safe.
- Called out route guest/trip mismatch risks for Director access.

## Valid Critiques Accepted

- Add explicit transaction-ownership tasks and list methods that need tx variants or handler-owned transaction wrappers.
- Enforce audit append-only with a database trigger that rejects update/delete.
- Make `uploaded_by_user_id` nullable with `ON DELETE SET NULL`.
- Add a DB check for allowed document content types.
- Validate uploads with `http.MaxBytesReader`, byte sniffing, and HEIC/HEIF fallback handling.
- Store files through temp file + hash + rename into a generated server-side storage key.
- Use a concrete storage-key format: `{organization_id}/{trip_guest_id}/{document_id}`.
- Tests must cover route trip/guest mismatch, path traversal filenames, MIME spoofing, guest-session denial, and audit metadata safety.
- Add `gofmt`, Prettier/build checks, and `tracker.go sync` to verification.

## Critiques Rejected or Modified

- Claude suggested dropping HEIC/HEIF or making it attachment-only. The user confirmed HEIC/HEIF are accepted and documents should open inline with download option. Final plan supports inline where browsers can render; HEIC/HEIF may still return inline disposition but UI provides an explicit download action and should show a note when browser preview is unavailable.
- Claude suggested dropping the separate activity endpoint or embedding timeline in the existing guest detail endpoint. Final plan keeps separate endpoints because Sprint 017 also requires org-wide audit search and pagination/filtering; staff guest profile can fetch a scoped activity slice.
- Claude suggested dropping `guest.invite_resent`. Final plan keeps it as an audit action but requires UI grouping/collapse for repeated resends.

## Interview Refinements Applied

- Guests must upload their own documents as part of trip registration before trip start. Trip lifecycle gating itself is deferred to the next sprint.
- Staff document management remains available from the guest profile for Org Admins and assigned Cruise Directors.
- Accepted file types are PDF, JPEG, PNG, HEIC, and HEIF up to 10 MiB.
- Documents should be viewable inline where possible, with an explicit download option.
- Audit must include an org-wide searchable page, not only a guest profile timeline.

## Final Decisions

- Sprint 017 includes both guest-upload and staff-upload document flows.
- Guest registration page gets document upload/list requirements and status; guest document endpoints are guest-session scoped to that trip guest.
- Staff guest profile gets Documents and Activity sections; no new tab routing.
- Organization Admin gets an org-wide Audit page. Assigned Cruise Directors can view audit events scoped to assigned trips.
- Audit is append-only at the database layer via trigger.
- Local document storage is configured with `LIVEABOARD_DOCUMENTS_DIR`, defaulting to a repo-local dev path outside SPA static assets.
- Audit metadata shapes are explicitly specified in the sprint doc and must avoid raw tokens, full registration payloads, storage keys, and secrets.
