# Sprint 017 Codex Draft — Claude Critique

This is a critique of `docs/sprints/drafts/SPRINT-017-CODEX-DRAFT.md`. The
final merge should keep the strong parts, address the concerns below, and
close the listed gaps before the doc is renamed to `SPRINT-017.md`.

## Strengths Worth Preserving

- **Two-feature scoping is correct.** Treating audit + documents as
  one sprint is right: documents need an actor trail anyway, so the
  audit table is on the critical path. Codex split the scope cleanly.
- **Migration number `0015` is correct.** `0014_cabin_layouts.sql` is
  the latest committed migration; the next one is `0015`.
- **Audit schema shape is solid.** `actor_type` discriminator with the
  CHECK constraint forcing exactly one of `actor_user_id` /
  `actor_guest_user_id` / neither is the right model and matches how
  guests vs staff are represented elsewhere in the codebase.
- **Index choices are appropriate.** Org+created, partial trip_guest,
  and partial entity indexes cover the access patterns named in the
  intent (timeline by guest, browse by entity).
- **Storage decision (disk over bytea) is right.** Rationale matches
  the intent's "local-dev only for now, cloud later" constraint, and
  `storage_provider` + `storage_key` columns make the future migration
  cheap.
- **Reuse of `authorizeManifestAccess`.** This is the correct helper
  (`internal/httpapi/guest_manifest_handlers.go:175`) and correctly
  handles the Org Admin / assigned-Cruise-Director split.
- **Audit categories cover the intent's requirements** (passport/travel
  document, dive certification, dive insurance, waiver, medical, other).
- **Download is treated as an audit-worthy action.** This is correct
  for sensitive PII and matches the intent's Security Criteria.
- **Phase percentages roughly map to actual effort.** Phase 1 at ~30%
  is right given two new tables, store helpers, and tx variants.

## Major Concerns

### 1. Transactional audit claim is hand-waved

The draft says "Where transaction ownership already exists, use
`RecordAuditEventTx`," but the code today does not generally expose tx
ownership at the audit emission site. For example,
`auth.Service.InviteTripGuest`
(`internal/auth/guest_accounts.go:45`) calls
`s.Store.CreateTripGuestInvite` as a single shot — the tx is internal
to the store method. To make the invite + audit row atomic you must
either:

- refactor `CreateTripGuestInvite` to accept a callback / return the
  invite plus a tx, or
- add a service-layer tx that wraps both the invite creation and the
  audit insert.

Same shape applies to `ResendTripGuestInvite`, `RevokeTripGuestInvite`,
`AdjustStock`, and the payment settings update. The final sprint doc
must commit to one approach (probably: pass `pgx.Tx` into a new
`store.WithTx(ctx, fn)` helper at the auth/service layer, or expose
`...Tx` variants that already exist for folios and cabins) and list
the affected store methods explicitly. Today the draft makes this
sound free; it isn't.

### 2. Append-only is asserted but not enforced

The draft says "Audit events are append-only" but does nothing at the
DB layer to enforce it. Reasonable options:

- A trigger that raises on `UPDATE` or `DELETE` against `audit_events`.
- Document-only ("we don't write UPDATE/DELETE in code"), which is
  what the draft implicitly chose.

Either is defensible; the draft should pick one and write the
verification test. A test that issues a raw `UPDATE audit_events` and
asserts the row is unchanged (trigger path) or that simply asserts no
production code path issues `UPDATE`/`DELETE` against the table is
worth listing.

### 3. `uploaded_by_user_id ... ON DELETE RESTRICT` is too strict

The pattern elsewhere in this codebase (e.g.
`cabins.go` `assigned_by_user_id`, audit events themselves) is
`ON DELETE SET NULL` with the column nullable. `RESTRICT` here means
you can never delete a user who has ever uploaded a document, which
will surface as a surprise bug whenever a former employee is offboarded.
Recommend: make `uploaded_by_user_id` `NULL ... ON DELETE SET NULL`
and store the actor's name in audit metadata for posterity.

### 4. MIME validation is under-specified

The draft says "MIME type must be validated by server policy, not
trusted solely from the browser." Good principle, no implementation.
Concretely:

- The `content_type` column has no CHECK; allowed list lives only in
  service code. Worth adding a CHECK so a misbehaving handler can't
  insert junk.
- The validation should sniff the first 512 bytes via
  `http.DetectContentType` and compare against the allowed set, not
  just trust the multipart `Content-Type` header.
- HEIC/HEIF detection by Go's stdlib sniffer is unreliable; if HEIC
  must be allowed, the draft should call out that detection falls
  back to extension + structural magic-byte check.

### 5. HEIC/HEIF in the allow-list creates a UX conflict

Most browsers cannot render HEIC inline, yet the draft proposes an
inline `Content-Disposition` header for "viewable" files. The final
doc should either:

- drop HEIC/HEIF from Sprint 017 (user policy: tell guests/staff to
  convert to JPEG before upload), or
- explicitly mark HEIC/HEIF downloads as `attachment` and accept that
  staff will save and open externally.

Leaving this ambiguous will produce a download that does nothing in
half of browsers. The intent's open question 2 should be answered
unambiguously.

### 6. Storage layout / config is missing

`var/uploads/guest-documents` is named once and never wired:

- No env var defined (e.g., `LIVEABOARD_DOCUMENTS_DIR`) and no
  fallback path.
- No mention in `internal/config` to load it.
- No `.gitignore` or `var/.gitkeep` instruction.
- No discussion of dev cleanup (truncate `guest_documents` ≠ delete
  the on-disk files; tests will accumulate cruft unless test runs
  use a `t.TempDir()`).
- Storage key format ("generated server-side using
  org/trip_guest/document IDs") is not concrete. State the format
  exactly, e.g. `{org_id}/{trip_guest_id}/{document_id}` (no
  extension; extension is metadata, not a path component) so future
  cloud migration is mechanical.

### 7. Tracker update missing from Phase 5

`docs/sprints/README.md` mandates `go run docs/sprints/tracker.go
sync` after creating a sprint doc, and `tracker.go start 017` when
beginning work. Phase 5 lists `git diff --check` and the build
commands but not the tracker sync. Add it.

### 8. Audit overlap with `stock_movements` is unaddressed

`stock_movements` already records actor + delta + source for inventory
changes. Adding an `inventory.adjusted` audit event duplicates that
record. Either:

- treat `stock_movements` as the audit source for inventory and pull
  it into the activity timeline via a UNION or a view, or
- write both rows and accept duplicate truth (current draft).

The draft picks option 2 implicitly; that's fine, but the doc should
note the duplication so a future reader doesn't try to "fix" it.

### 9. Download audit semantics need tightening

`guest.document_downloaded` is listed alongside the transactional
events. But:

- You cannot roll back a download, so wrapping it in a tx with the
  read is meaningless.
- A naive implementation logs an event on every byte-range request,
  HEAD request, or 304. The doc should specify: log once per
  successful 200 response on the download endpoint, after the file
  exists check, before streaming.
- If the file streams and fails midway, do you still keep the audit
  row? (Probably yes — the access attempt is what matters.) Specify.

### 10. Hashing claim is fragile

"Server computes SHA-256 while writing the file." Implementation
detail, but the draft doesn't say what happens if the file is
truncated or the connection drops mid-upload. Acceptance criteria:
write to a temp path, fsync, hash, then rename into the final
storage key. Reject the request if size or hash deviate from
expected. Without this the `sha256_hex` column is decorative.

## Missing Implementation Details

- **No frontend tab pattern reference.** `TripGuestDetail.tsx` does
  not currently have a tab system; the draft assumes "Tabs/sections:
  Summary / Registration / Documents / Activity" without specifying
  whether these are URL-routed tabs, anchor sections, or a new
  TabBar component. Pick one. URL-routed tabs are best for
  shareability and back-button behaviour, but they are larger scope.
  Anchor sections are smaller scope and consistent with the dense
  operational surface DESIGN.md calls for.
- **No upload UX state spec.** Pending/uploading/complete/failed
  states; cancel; retry; size pre-check (reject before request).
- **No activity timeline rendering spec.** Action → human label
  mapping (e.g. `guest.cabin_assigned` → "Assigned to berth 1A").
  Without this mapping the UI ships as raw event names.
- **No pagination/cursor on the activity endpoint.** Draft passes
  `limit` only. Specify "most recent N, no pagination in Sprint 017"
  if that's the call, or add a cursor.
- **No guidance on metadata fields per action.** "Avoid sensitive
  payloads" is a principle. The doc should give one explicit example
  per action listing the metadata keys, e.g.
  `guest.cabin_assigned` → `{from_berth_id, to_berth_id, notes}`.
  Without per-action shape, every developer invents their own and
  the data becomes useless to query.
- **No CSRF discussion.** Multipart uploads under cookie auth are
  classic CSRF targets. If the codebase already mitigates via SameSite
  / origin checks, say so. If not, this is a real gap.
- **No reference for file size cap enforcement.** Go's
  `http.MaxBytesReader` should be applied at handler entry, not just
  inside the parser. State this.
- **No `Content-Length` / streaming guidance for download.** Should
  set `Content-Length` from `size_bytes` and use `io.Copy` from the
  file to the writer; spell it out so reviewers don't have to guess.

## Suggested Changes

1. **Add `actor_full_name_snapshot` to audit metadata.** Cheap insurance
   when a user is later renamed/deleted; the timeline UI should not
   need to JOIN `users` to render an event.
2. **Make `uploaded_by_user_id` nullable, `ON DELETE SET NULL`.** As
   above.
3. **Add a CHECK on `guest_documents.content_type`** that mirrors the
   service allow-list, to prevent rogue inserts.
4. **State exactly which existing methods get tx variants.** A short
   list in Phase 1: `CreateTripGuestInvite`, `ResendTripGuestInvite`,
   `RevokeTripGuestInvite`, `AdjustStock`, `UpdatePaymentSettings`,
   `OpenGuestFolio`, `CloseGuestFolio`, etc. — only those that need
   to land in a single tx with audit.
5. **Add Phase 5 sub-tasks**: `tracker.go sync` and `tracker.go start
   017`.
6. **Pin DESIGN.md compliance items.** Today the draft says "Use
   DESIGN.md: dense operational surfaces." Spell out the three
   highest-risk items: typography (DM Sans / Geist / General Sans),
   no decorative imagery, slate working surfaces. UI reviewers
   should be able to grep the diff for divergence.
7. **Resolve open questions inline.** The intent's five open questions
   (storage, guest self-upload, immutability, org-wide search, file
   types/size) should be closed in the final doc, not re-listed.
   Codex closed three but re-listed four "open questions" at the
   bottom — promote those to decisions.
8. **Document the truncation order update.** The draft says "Update
   test truncation order" without listing where the new tables go.
   They must precede `guest_users` and `users` because of FKs:
   `guest_documents` first (FKs to `users`, `trip_guests`,
   `organizations`), then `audit_events` (FKs to `users`,
   `guest_users`, `trips`, `trip_guests`, `organizations`), both
   ahead of the existing list near the top.

## Risks the Final Merge Should Address

- **Director-of-a-different-trip data leak.** If a Cruise Director is
  assigned to trip A but the URL contains trip B's ID and a guest_id
  from trip A, does the handler verify that `trip_guest.trip_id ==
  trip_id_in_route`? `authorizeManifestAccess` only validates the trip;
  the existing `tripAndGuestParams` helper doesn't bind them. Phase 2
  task list says "verify trip_guest_id belongs to the route trip" —
  good — but the test plan must include a director swapping in a
  different trip's `guest_id` against their assigned trip's URL.
- **Storage-key path traversal.** If `original_filename` is ever
  echoed into a path, you have an issue. Test must include an upload
  whose filename is `../../etc/passwd.jpg`.
- **MIME spoof.** Test must include an upload with
  `Content-Type: image/png` whose bytes are an EXE, and assert the
  upload is rejected by the byte sniffer.
- **HEIC undetectable by stdlib sniffer.** Even with `image/heic` in
  the allow-list, `http.DetectContentType` returns
  `application/octet-stream` for HEIC. Either accept that bytes-side
  detection won't fire (and lean entirely on the multipart header,
  with the resulting risk) or drop HEIC.
- **Audit metadata leak.** Without per-action metadata shapes,
  developers will paste in entire request bodies. The audit tests
  must scan a representative event's metadata for token-shaped
  strings, full email addresses, and storage paths and fail on hit.
- **Disk fill / DoS in dev.** `var/uploads/guest-documents` will
  accumulate. Add a single line about local cleanup
  (`rm -rf` is fine; this is dev-only) and a `t.TempDir()` rule for
  tests so nothing leaks into the repo.
- **Guest session denial regressions.** The draft lists guest-session
  denial tests, but should explicitly cover: a guest session with a
  valid guest cookie hitting the document POST endpoint (must 401
  or 403, not silently succeed because the route is under
  `r.Group(r.Use(s.Session.Wrap))` and a guest session is not a
  staff session).

## Parts to Reject or Simplify

- **The four-tab UI.** Reject in favour of two new sections appended
  to the existing `TripGuestDetail.tsx` — Documents and Activity —
  unless the user explicitly asks for tabs. Tabs are scope creep
  versus the intent ("Document management … done via viewing a
  guest's profile"), and they introduce routing decisions that don't
  belong in this sprint.
- **`/api/admin/trips/{id}/guests/{guest_id}/activity`.** Consider
  embedding the timeline in the existing staff guest registration
  GET (`handleStaffGuestRegistration`) instead of adding a separate
  endpoint, mirroring the way registration is already returned with
  the trip-guest payload. Two round trips per page load is wasteful
  for a feature that always ships together. If staying separate, do
  it for caching reasons and say so.
- **`storage_provider` column with a default of `'local'`.** Either
  commit to multi-provider readiness now (and add a CHECK) or omit
  the column and let the future cloud migration add it. The current
  shape is decorative until cloud is real.
- **`guest.invite_resent` action.** Resends are operational noise,
  not state changes. Recording every resend will drown the timeline.
  Suggest dropping it from the Sprint 017 wired-actions list and
  keeping only `guest.invited` and `guest.invite_revoked`. If kept,
  collapse runs in the UI ("3 resends in 5 minutes").
- **Phase 3's "focused tests proving representative audit events
  exist."** Too vague. Replace with: one test per wired action that
  asserts the audit row exists with the right `actor_type`,
  `entity_type`, and at least one expected metadata key.

## Convention Mismatches

- **Sprint doc structure.** Codex's draft mostly follows the README
  template but lacks a `## References` section. Optional in the
  template, but Sprints 014–016 all include external refs (DESIGN.md,
  prior sprints). Add one.
- **`organization_payment_settings` vs `payment_settings`.** Codex
  references `internal/store/payment_settings.go`; the actual table
  is `organization_payment_settings`. Naming in the audit action
  (`organization.payment_settings_updated`) is fine, but the doc
  should be precise about table vs file names so reviewers don't
  hunt for a non-existent table.
- **Phase 5 lists `npm run build` in `web` but not `npm run lint` /
  `prettier --check`.** CLAUDE.md mandates prettier-clean TypeScript.
  Add it.
- **No mention of `gofmt`.** CLAUDE.md mandates it. Add to Phase 5.

## Bottom Line

The draft is structurally sound and the schema choices are right,
but it skips over the hard parts: transactional audit wiring across
heterogeneous service code, MIME enforcement, storage configuration,
HEIC's incompatibility with the inline-view assumption, and per-action
metadata shapes. The final merge should answer the intent's five open
questions decisively, pin down tx ownership across each wired action,
specify metadata schemas so audit data stays queryable, and either
drop HEIC or commit to attachment-only downloads for it. Trim the
four-tab UI to two new sections, drop `guest.invite_resent` (or
collapse it in UI), and add the missing `tracker.go sync` and
`gofmt`/`prettier` steps to Phase 5.
