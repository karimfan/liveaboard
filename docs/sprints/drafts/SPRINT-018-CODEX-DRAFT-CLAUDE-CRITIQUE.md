# Sprint 018 — Critique of Codex Draft (Claude)

Reviewing `SPRINT-018-CODEX-DRAFT.md` against `SPRINT-018-INTENT.md`,
`docs/sprints/README.md`, `CLAUDE.md`, the actual codebase state at HEAD,
and the four interview answers the user has confirmed as final product
input.

The draft is structurally solid and reuses the right primitives. It also
makes several decisions that directly contradict the confirmed interview
answers, over-engineers the exception path, and leaves a few necessary
backend touchpoints out of the file lists. Detailed findings below.

## 1. Strengths Worth Preserving

- **Status taxonomy is right.** `planned / active / completed /
  cancelled` matches the intent and is the minimum needed taxonomy. The
  default-to-`planned` for existing imported trips is correct.
- **Readiness aggregated in the store layer.** `TripLifecycleReadiness`
  computed in `internal/store/trip_lifecycle.go` is the right call;
  scattering the same checks across handlers would duplicate logic and
  drift. Keep it.
- **Transactional audit coupling.** Wrapping every transition with
  `RecordAuditEventTx` in the same transaction as the column update is
  the right invariant; it means we cannot ship a trip into `active`
  without the audit row, and vice versa. Keep this property as a
  Definition-of-Done item, not just a phase task.
- **Mutation guards listed by handler.** The enumerated list of
  cancelled/completed-trip mutation guards (invite, cabin, document,
  registration, folio) is more useful than a hand-wave; it is exactly
  what reviewers will check against.
- **Migration `0017_trip_lifecycle.sql` numbering is correct.** Existing
  migrations end at `0016_trip_guests_no_delete.sql`, so 0017 is the
  next valid slot.
- **The (organization_id, status, start_date) composite index** is the
  right shape for the trip list view's status filter + date sort.
- **Phase 4 includes tracker sync, doc updates, gofmt, `go vet`, and
  build.** This is the convention; keep.
- **Closed/cancelled guest registration return "clear API error +
  closed/cancelled state UI"** — this is the right behavior for the
  guest token endpoint.

## 2. Major Concerns — Direct Conflicts with Confirmed Answers

These are not subjective; the user has given explicit final answers and
the draft must be made consistent with all four. The merged sprint doc
must resolve each.

### 2.1 Org Admin emergency override (Answer #3) — REJECTED in the draft

Draft says (Architecture > Core Rules):
> "Org Admin has read oversight for all trips but cannot start or
> complete trips in Sprint 018."

And in DoD:
> "[ ] Org Admins cannot start/complete trips."

Confirmed answer: Org Admins **must** have an emergency override to
start AND complete trips.

This is not a small change — it touches:
- `authorizeLifecycleTransition` permission helper (must accept Org
  Admin in addition to assigned CD).
- A distinct audit metadata field (`override_used: true`,
  `override_role: "org_admin"`, plus a required reason string) so we can
  tell post-hoc which transitions came through emergency override
  versus the normal CD-assigned path.
- DoD inversion: the DoD bullet `Org Admins cannot start/complete
  trips` must become `Org Admins can start/complete any org trip with a
  required override reason; the audit row records `override_used=true`
  and the reason`.
- Frontend: the start/complete buttons must be visible to Org Admins on
  any org trip in the corresponding lifecycle state, with a different
  modal copy that requires the override reason.

Suggested merged rule:
- Start: assigned CD (no reason required) OR Org Admin (reason required,
  audit metadata flags override).
- Complete: same.
- Cancel: Org Admin only (already correct in draft).

### 2.2 Folio-blocking on completion (Answer #2) — OVER-ENGINEERED

Draft proposes a `trip_lifecycle_exceptions` table with structured
issue codes, entity references, and reason text, gated through
`exceptions: [...]` payloads, and recommends "allowing exceptions only
through structured records."

Confirmed answer: completion with open folios is "a warning /
acknowledgement path, not a blocker." This is not a structured
exception entity; it is a warning that the Director acknowledges.

Recommendation:
- **Drop the `trip_lifecycle_exceptions` table entirely** for Sprint
  018. It is unused if the only acknowledged warning is "open folios."
- The complete handler accepts an optional
  `acknowledged_warnings: ["open_folios", "missing_documents", ...]`
  array (or simpler: a single boolean +
  `acknowledgement_reason` string).
- The audit metadata for `trip.completed` carries
  `{open_folio_count, acknowledged_warnings, reason}`.
- Hard blockers for `complete`: trip is `active`, caller is allowed,
  every guest has been added/revoked (no in-flight enrollment). That's
  it. Open folios become a warning, not a blocker.

If we ever need a structured exception record (e.g., insurance,
compliance reporting), it can be added later as a follow-up sprint.
Putting it in 018 is speculative scaffolding — exactly the kind of
"design for hypothetical future requirements" CLAUDE.md cautions
against.

### 2.3 Document-blocking on start (Answer #1) — partially correct, must be unambiguous

Draft says (Architecture > Readiness Model > Start readiness should
require):
> "every non-revoked guest has at least one active document"

But then in the same section:
> "The first draft recommends hard blockers for submitted registration
> and berth, warning-only for documents until required document
> categories become configurable."

Confirmed answer: missing documents are **warnings, not blockers**.

The merged sprint doc should pick the second statement and delete the
first. Concretely:
- Start hard blockers: `trip status == planned`, caller authorized,
  every non-revoked guest has an active berth assignment, every
  non-revoked guest has submitted registration.
- Start warnings (acknowledged via `acknowledged_warnings`): missing
  documents (any), missing optional registration fields, and any
  other "soft" gates we add later.
- Boat layout / "at least one assigned CD exists" probably belong in
  warnings too, not blockers, because the Org Admin override implies a
  real human is making the call (see 2.1).

### 2.4 Stale imported-trip handling (Answer #4) — UNADDRESSED as a distinct state

Draft acknowledges this as Open Question #4 and proposes either
"mark stale planned imported trips as cancelled" or punt.

Confirmed answer: imported trips that disappear from the source can be
**removed from the operational ledger/list UI**, but **backend records
must be retained for analytics and search/history**.

This is a third thing — neither `cancelled` (which is a deliberate
human act with a `cancellation_reason`) nor a hard delete (which is
what `ReplaceFutureScrapedTrips` does today via `DELETE FROM trips
WHERE source_trip_key <> ALL(...)`).

Recommendation for the merged sprint:
- Add a new boolean/timestamp column, e.g.
  `removed_from_source_at timestamptz NULL`, on `trips`. Set when a
  reconciliation pass would have deleted the row. Unset means "still
  visible in source" or "not source-imported."
- Replace the `DELETE FROM trips ...` in
  `ReplaceFutureScrapedTrips` with `UPDATE trips SET
  removed_from_source_at = $now WHERE source_trip_key <> ALL($keys)
  AND removed_from_source_at IS NULL`. This is also more conservative
  with the existing trip_guests no-delete invariant — the current
  delete path will already fail if any of the stale rows have
  trip_guests, so this is also a latent bug Codex correctly flagged
  but did not finish solving.
- The trip list UI default filters out
  `removed_from_source_at IS NOT NULL`, with an explicit "Removed from
  source" filter to reveal them. Status chip becomes
  `removed_from_source` only in the "show all" filter.
- Spreadsheet imports (`ReplaceSpreadsheetTrips`) need the same
  treatment — same code path issue.
- This must be reflected in the Files Summary
  (`internal/store/trips.go` modification scope) and DoD.

This is small in code surface but central to the user's mental model
of what 018 means; do not punt it to a follow-up.

## 3. Missing Implementation Details

### 3.1 `internal/httpapi/cruise_director.go` is missing from the file list

The draft says:
> "Use persisted `status` counts instead of date classification."

But the file list does **not** include
`internal/httpapi/cruise_director.go` — only
`web/src/admin/pages/Overview.tsx`. The CD landing endpoint
(`handleCruiseDirectorOverview`) currently calls `classifyTripStatus`
at line 59 and emits `upcoming/active/past`. To switch to persisted
lifecycle counts, the backend has to change too.

Add to Phase 2 file list and tasks:
- `internal/httpapi/cruise_director.go` — modify
  `handleCruiseDirectorOverview` to bucket trips by persisted status
  (`planned`/`active`/`completed`/`cancelled`) and either delete or
  rename `classifyTripStatus` to make its narrower role explicit.

The merged sprint must also be explicit about whether
`classifyTripStatus` survives at all. Recommendation: keep it as a
date-only helper but rename to `classifyTripDateBucket` so the call
site documents that it returns scheduling context, not lifecycle.

### 3.2 Trip-level no-delete protection is missing

Sprint 016 added a `trip_guests_no_delete` trigger because guest rows
are historical. Once trips have lifecycle state and audit history,
hard-deleting `trips` rows would orphan audit events
(`audit_events.trip_id`) and violate the same invariant.

The current scrape reconciliation code (`ReplaceFutureScrapedTrips`,
`ReplaceSpreadsheetTrips`) hard-deletes `trips` rows. That conflicts
with both 3.1 above and the new `removed_from_source_at` model from
2.4. Add to migration 0017 (or a sibling 0018) a similar trigger:
`reject_trip_delete_when_history_exists` — block DELETE on `trips`
when there are referencing `trip_guests`, `audit_events`, folios, or
documents. The new soft-removal column is what the import path uses
instead.

This is a structural correctness item, not a polish item.

### 3.3 Cruise Director assignment lifecycle interactions are unspecified

`internal/httpapi/cruise_director_assign.go` is not in the file list
and the draft is silent on:
- Can a CD be assigned to a `completed` or `cancelled` trip? (Probably
  not — that mutates a frozen record.)
- Can a CD be unassigned from an `active` trip mid-flight? If so, can
  the remaining CD complete? If not, who can?
- If the only assigned CD is unassigned mid-active, does the trip
  "lose" its assigned-Director constraint, requiring Org Admin
  override to complete?

Add an explicit rule and corresponding handler guard. The simplest
ruleset: assignment changes are blocked on `completed` and
`cancelled`; allowed on `planned` and `active`; and if no CD remains,
only Org Admin can complete (via override path).

### 3.4 `TripsNeedingAttention` not updated for lifecycle

`TripsNeedingAttention` returns trips needing CD assignment in the
next 90 days regardless of status. After 018 it should explicitly
exclude `cancelled`, `completed`, and `removed_from_source_at IS NOT
NULL` rows. Add this to the store-layer task list in Phase 1.

### 3.5 Index design for the trip list

The proposed
`trips_org_status_start_idx (organization_id, status, start_date)` is
fine for the default "planned and active" filter, but the most common
operational query — "active trips for this org right now" — would
benefit from a partial index:
`CREATE INDEX trips_org_active_idx ON trips(organization_id,
start_date) WHERE status = 'active';`. Optional; mention it as a
follow-up if the query plan shows the composite is good enough.

### 3.6 Schema specifics worth tightening

- `cancellation_reason text NULL` should have an explicit length
  constraint (e.g., `CHECK (char_length(cancellation_reason) <= 500)`)
  to avoid unbounded text in JSON responses and audit metadata.
- `started_by_user_id` / `completed_by_user_id` /
  `cancelled_by_user_id` use `ON DELETE SET NULL`. Audit columns
  generally use `ON DELETE RESTRICT` in this codebase to preserve
  traceability. Pick one and apply consistently with how
  `audit_events.actor_user_id` is handled today (RESTRICT in
  `0015_audit_and_guest_documents.sql`).

### 3.7 No CHANGELOG / no test-data fixtures plan

DoD lists `npm --prefix web run build`, `go test ./...`, `go vet`. It
does not mention `CHANGELOG.md`. The repo has one and the recent
sprints update it; add a DoD bullet.

`internal/testdb/testdb.go` is in the Phase 1 file list with no
context. Spell out what changes there: presumably new constructors
that produce trips in non-`planned` states for handler/store tests,
otherwise every existing test that creates a trip silently picks up
the new default and that's fine — but the helper is needed for the
new transition tests.

## 4. Suggested Changes (Concrete Edits to the Final Sprint Doc)

When merging, apply at minimum:

1. **Override rules.** Replace the Org-Admin-can-only-cancel rule with:
   "Org Admin can start, complete, or cancel any trip in their org.
   Start and complete via Org Admin require a reason string and write
   `override_used=true` to audit metadata. Cancel reason is already
   required."
2. **Drop the exceptions table.** Remove
   `trip_lifecycle_exceptions` and the `trip.lifecycle_exception_recorded`
   audit action. Replace with `acknowledged_warnings` (string array or
   single boolean + reason) on the start/complete request payloads,
   surfaced in audit metadata.
3. **Documents are warnings, not blockers, for start.** Remove
   "every non-revoked guest has at least one active document" from
   start hard blockers; move it to warnings. Keep berth and
   submitted-registration as hard blockers.
4. **`removed_from_source_at` column + import path rewrite.** Replace
   stale-import deletion in `ReplaceFutureScrapedTrips` and
   `ReplaceSpreadsheetTrips` with soft removal. Add a default UI
   filter that hides removed-from-source trips. Add explicit DoD
   bullet: "Stale-imported trips no longer hard-delete; backend rows
   retained, operational list UI hides them by default."
5. **Add cruise_director.go and cruise_director_assign.go to the
   file list.** Specify the lifecycle interaction rules for
   assignment.
6. **Add trip-level no-hard-delete trigger / dependency check.**
7. **Tighten schema constraints on `cancellation_reason` length and
   pick consistent ON DELETE behavior.**
8. **DoD adjustments:**
   - Replace the "Org Admins cannot start/complete trips" bullet with
     the override bullet above.
   - Add: "Stale-imported trips retain backend rows and are hidden
     from default operational list."
   - Add: "Override starts/completes are audit-flagged with the actor
     role and a required reason."
   - Add: "CHANGELOG updated."
   - Tighten "Cancelled/completed trip registration links no longer
     accept save/submit" to also explicitly cover guest invite-resend
     and any other guest-token endpoint, not just the registration
     form.

## 5. Testing Gaps to Address in the Merge

The draft's test list is generic. Specify these scenarios:

- Org Admin override start/complete writes
  `override_used=true` and the reason; missing reason returns 400.
- Assigned CD start with no override reason succeeds and writes
  `override_used=false`.
- Concurrent double-start: two requests to `/start` race; only one
  wins, the other receives a clear conflict error and no second audit
  row is written. Use a SELECT ... FOR UPDATE or status-precondition
  WHERE clause inside `StartTrip`.
- Reverse transitions: `active → planned`, `completed → active`,
  `cancelled → anything` are all rejected at the store layer. Test
  store and handler levels.
- Cancelled trip rejects every mutation listed in the guard table —
  one test per handler is fine (cabin, doc, invite, registration,
  folio).
- `removed_from_source_at` set by import does not flip status; the
  trip remains `planned` but list endpoint default filter hides it.
- Stale import where `trip_guests` exist: the new import path soft-
  removes rather than failing on the trip_guests no-delete trigger.
  Test that the bug class is gone.
- Folio close after trip completion: confirm the desired behavior
  (probably blocked, but the merged spec should say so explicitly)
  and add the corresponding test.
- Guest token endpoints on completed trip: registration save,
  registration submit, document upload all return the same closed
  status response, not 500s.

## 6. Risks the Final Merge Should Address

1. **Behavior of `import` against existing in-flight trips.** A
   liveaboard.com refresh that runs while a trip is `active` must not
   mutate lifecycle columns. The current `ON CONFLICT DO UPDATE SET
   ...` clauses do not touch lifecycle columns, which is correct, but
   the merged doc should call this out explicitly so a future
   reviewer doesn't add `status = EXCLUDED.status` by reflex.
2. **Race with stale-delete + soft-remove.** Once we move from delete
   to update, `removed_from_source_at` should clear automatically when
   a previously-stale source key reappears. The merge should specify:
   on UPDATE in the import upsert, `SET removed_from_source_at = NULL`
   so a re-listed trip un-removes itself.
3. **Pagination / counting.** `TripCountForOrg` is used by the
   Overview's setup completeness card and currently counts all trips.
   Decide whether `cancelled` and `removed_from_source_at IS NOT
   NULL` count toward "is this org set up." Document the decision.
4. **Audit metadata size.** Override reasons and acknowledgement
   reasons are user-supplied; bound them to ~500 chars at the
   handler layer before writing to audit metadata.
5. **Frontend mismatch on Overview tile semantics.** The CD landing
   currently shows upcoming/active/past based on dates, not status.
   When this flips to status-based counts, "past" disappears as a
   separate concept (replaced by `completed`). The merge must
   describe the new tiles to avoid silent regression of the CD
   landing UX.
6. **Time skew on transitions.** `started_at` / `completed_at` are
   server timestamptz. The frontend probably renders them in user
   local time; not a regression but worth a sentence in the doc.

## 7. Parts of the Draft That Should Be Rejected or Simplified

- **Reject `trip_lifecycle_exceptions` table.** Replaced by simple
  `acknowledged_warnings` payload and audit metadata. The structured
  exceptions design is speculative scaffolding for a use case the user
  has explicitly said is just a warning + ack.
- **Reject the "first draft recommends" block in §Readiness Model.**
  Both recommendations conflict with confirmed interview answers (#1
  hard-blocks docs, #2 mandates structured exceptions). The merged
  doc should be declarative, not a debate transcript.
- **Reject the "Org Admin cannot start/complete" rule and DoD bullet.**
- **Simplify the Audit Actions table.** With the exceptions table
  gone, `trip.lifecycle_exception_recorded` goes too. The remaining
  three (`trip.started`, `trip.completed`, `trip.cancelled`) need
  their metadata schemas updated to include
  `override_used`, `override_role`, and `override_reason` (nullable).
- **Reject "stale-delete behavior for a follow-up" as an option.**
  The user has answered question #4. Soft-remove is in scope.

## 8. Mismatch with Project Conventions

- Draft uses `internal/store/migrations/0017_trip_lifecycle.sql` —
  matches the existing numbering. ✅
- Draft does not specify `-- +goose Up` / `-- +goose Down` block
  structure used by every existing migration in this repo. The
  merged migration should include both blocks; the down block needs
  `ALTER TABLE trips DROP COLUMN ...` and `DROP INDEX
  trips_org_status_start_idx`. Trivial, but worth flagging because
  the draft only shows the up side.
- DoD bullet "`go test ./...` passes" is correct convention. Add an
  explicit gofmt mention; the current bullet list omits it even
  though Phase 4 task list mentions it.
- The sprint title in the draft (`Trip Lifecycle and Readiness Gates`)
  matches the intent; keep.
- Branching: `CLAUDE.md` says "Work directly on `main`." The draft
  doesn't violate this but the merged doc should not include any
  branch-name references.

## 9. Bottom Line

Codex's draft is a good skeleton with the right shape. Three of the
four interview answers are not yet reflected in it, and one of those
(Org Admin override) requires non-trivial revision to the permission
model, audit metadata, and DoD. The exception-table design is the
single largest piece of over-engineering and should be removed before
the merge. The fourth answer (stale-import soft-removal) needs a small
but real schema/import-path change that the draft only flags as an
open question.

Concretely: the merged Sprint 018 doc should drop one table, add one
column, expand one permission helper, add two new audit metadata
fields, modify one more handler file (`cruise_director.go`),
introduce one new no-delete safeguard (or dependency check) on
`trips`, and rewrite the DoD to match the four confirmed answers.
Everything else in the draft can be carried forward largely as-is.
