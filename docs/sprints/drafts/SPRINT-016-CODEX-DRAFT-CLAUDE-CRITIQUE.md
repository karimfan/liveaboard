# Sprint 016 Codex Draft: Claude Critique

This is a critique of `SPRINT-016-CODEX-DRAFT.md` (cabin layouts and trip
berth assignments) against the intent file, the codebase, and the
project conventions in `CLAUDE.md` and `docs/sprints/README.md`. It is
not a rewrite. The intent is to surface issues and trade-offs the merge
should resolve before the final sprint doc is written.

## 1. Strengths Worth Preserving

These parts of the draft are correct and should carry through to the
merge largely untouched.

- **Two-table separation of layout vs. assignment.** `boat_cabins` +
  `boat_cabin_berths` (operator-owned, reusable) vs.
  `trip_cabin_assignments` (per-trip) is the right split. It mirrors
  the Sprint 013 pattern of org-scoped tables with store-level helpers
  enforcing ownership.
- **Snapshot fields on assignments.** Storing
  `cabin_label_snapshot` / `berth_label_snapshot` /
  `display_label_snapshot` on `trip_cabin_assignments` matches the
  Sprint 015 closed-folio precedent: history must survive layout
  edits. Constraint #8 in the intent ("Closed guest folios must not
  be affected by cabin assignment changes") is structurally honored.
- **Berth-level assignment.** Targeting berth, not cabin, is the
  correct interpretation of "more than one guest can share a cabin"
  from the seed.
- **`is_active` soft-deactivation** on cabins and berths preserves
  history and supports the "block destructive edits" requirement.
- **Reuse of `authorizeManifestAccess` for trip endpoints.** Verified
  in the codebase (`internal/httpapi/guest_manifest_handlers.go:168`,
  used by every Sprint 014/015 manifest-scoped handler). This is the
  right pattern.
- **Migration number 0014 is correct.** Last migration on disk is
  `0013_guest_folios.sql`.
- **Range generator + paste parser + preview-before-save** matches
  the user's stated need ("not tedious manual entry") and is the
  correct minimum scope.
- **Calling out the Cruise Director layout-edit permission as an
  interview question** is appropriate. The recommended default
  (directors edit trip assignments only, admins own reusable layout)
  is the right starting position.
- **Phase ordering** (schema → store → HTTP → fleet UI → trip UI →
  docs) is sensible.
- **Acknowledging that there is no first-class manual boat creation
  endpoint** and choosing not to invent one is correct restraint.

## 2. Major Concerns

### 2.1 Schema ambiguity around `trip_cabin_assignments` lifecycle

The draft is internally inconsistent about how assignments survive
layout deletions and unassignment.

- `cabin_id` and `berth_id` are nullable with `ON DELETE SET NULL`,
  but the draft also says "Layout edits that would orphan
  active/upcoming assignments are blocked or converted to inactive
  rows with preserved snapshots." Pick one model:
  - **Strict**: `ON DELETE RESTRICT`, with the application layer
    enforcing dependency checks before allowing destructive edits.
    This is closer to the Sprint 015 "atomic close" / "immutable
    history" pattern.
  - **Snapshot-preserving**: keep `ON DELETE SET NULL`, but
    explicitly document orphaned-but-snapshotted rows as a first
    class state, and define how the cabin board renders them.
- The verb "Unassign" is undefined: does it `DELETE` the row, or set
  `berth_id = NULL` and keep the row? If the latter, how does
  re-assignment behave (insert vs. update)?
- There is no constraint that `berth_id`'s parent cabin equals
  `cabin_id`. The two columns can drift out of sync silently.
  Either (a) drop `cabin_id` and derive it via join, or (b) add a
  trigger / generated check to enforce coherence.

The merge should pick a single lifecycle model and document it
explicitly.

### 2.2 Revoked-guest interaction is under-specified

Sprint 014 made `trip_guests` soft-deletable via `revoked_at`. The
draft says "Assignment rejects revoked guests" (write-time check) but
is silent on what happens to an existing assignment when a guest is
later revoked. Three plausible behaviors:

1. Auto-unassign on revoke (frees the berth).
2. Keep assignment, mark as orphaned in UI.
3. Block revoke if assigned (forces staff to unassign first).

The unique partial index `trip_cabin_assignments_one_berth_per_trip_idx`
on `(trip_id, berth_id) WHERE berth_id IS NOT NULL` does **not**
filter by guest revocation, so a revoked-but-still-assigned row will
block re-assignment of that berth to a different guest. That bug is
latent today.

The merge must pick a strategy. (Auto-unassign on revoke is the
cleanest match for the rest of the codebase.)

### 2.3 Active-row uniqueness pattern is missing

`boat_cabins` has `UNIQUE (organization_id, boat_id, label)` and
`boat_cabin_berths` has `UNIQUE (organization_id, boat_id, display_label)`.
Neither is filtered by `is_active`. A deactivated berth keeps its
display label, so re-creating `1A` after deactivation collides.

The codebase already uses partial-unique-index patterns for exactly
this case — see
`guest_trip_invitations_trip_guest_pending_idx ... WHERE accepted_at IS NULL AND revoked_at IS NULL`
in `migrations/0012_guest_registration.sql:60-62`. Apply the same
pattern: `... UNIQUE WHERE is_active`.

### 2.4 `ReplaceBoatCabinLayout` semantics are hand-wavy

> Layout replacement only allowed when no upcoming/active assignment
> references any current berth, **or when the replacement exactly
> preserves existing assigned berth IDs**.

The second branch is undefined for a `PUT` body whose JSON examples
do not carry IDs. How does the server tell "this is a refinement that
preserves berth 1A's ID" from "this is a brand-new layout that
happens to also have a 1A"? Either:

- The preview response must return berth IDs that the `PUT` body
  echoes back; or
- The flow must be split: `PUT` is only available when the boat has
  zero assigned berths anywhere. After that, edits use per-cabin /
  per-berth `PATCH` and `DELETE`.

The latter is simpler and aligns with the "block destructive edits"
rule. Recommend the merge commit to that split and remove the
"exactly preserves IDs" clause from `PUT`.

### 2.5 Files Summary line for `BoatTabs.tsx` is misleading

The draft says `BoatTabs.tsx` should be modified to "Split or link to
cabins tab." Verified in code:

- `web/src/admin/pages/BoatDetail.tsx:99-119` contains the actual tab
  navigation (`<NavLink>` per tab) and `<Outlet />`.
- `web/src/admin/pages/BoatTabs.tsx` contains the *page content*
  components (`BoatTrips`, `BoatInventory`, `BoatNotes`).

So the tab nav addition belongs in `BoatDetail.tsx`, and the new
`BoatCabins` page component can live either as an additional export
in `BoatTabs.tsx` (matching the existing pattern) or in a new
`BoatCabins.tsx` (which the draft also proposes — pick one). The
existing `BoatNotes` placeholder may also need a decision: keep,
delete, or replace with cabins.

### 2.6 `guest_id` in URL paths is ambiguous

The draft's HTTP table uses
`/api/admin/trips/{id}/guests/{guest_id}/cabin-assignment`. The
project convention from Sprint 014/015 is that this `{guest_id}`
means `trip_guest_id` (the per-trip row), not `guest_user_id`. The
final spec must say this explicitly to avoid the implementer wiring
the wrong identifier.

### 2.7 Import response does not expose newly created boat IDs

The draft says: "For newly inserted boats, return enough metadata for
the UI to link to Fleet > Boat > Cabins." Today's import job returns
only counts (`boats_inserted`, `trips_inserted`, etc.) per
`migrations/0009_trip_imports.sql:26-29` and
`internal/httpapi/import_handlers.go:381`. There is no per-boat list.

To make the post-import setup link work, the implementer must either:

- Extend `import_jobs` with a per-boat result payload (schema
  change), or
- Compute "boats with zero active berths" on Fleet load and surface
  that as the unconfigured state (no schema change, weaker UX).

The draft picks the first ("return enough metadata") but does not
list the schema/storage change in Phase 1. The merge needs to pick
one path explicitly.

### 2.8 DESIGN.md is not referenced anywhere

`CLAUDE.md` mandates: "Always read `DESIGN.md` before making any
visual or UI decisions." Phase 3 and Phase 4 add substantial new UI
(Fleet > Boat > Cabins editor, trip cabin board, manifest assignment
column, post-import setup banners). The draft only mentions
modifying `web/src/styles/app.css` — no commitment to applying
DESIGN.md tokens. The merge must add a step "Apply DESIGN.md
typography, spacing, and color tokens" to Phases 3 and 4 and
QA-flag any deviations.

### 2.9 Phase 5 budget is too low

Phase 5 at ~5% covers updating `personas.md` and
`organization-admin-user-stories.md` to reverse the previously
"deferred" cabin model decision **and** running build/test
verification. The product policy update is non-trivial: it touches
US-3 (fleet) and US-4 (manifest) at minimum and changes the
deferred-decisions table. Bump to ~10% and split docs from
verification.

## 3. Missing Implementation Details

These are gaps the merged sprint doc must fill in.

1. **Paste parser grammar.** Examples mix `1 AB`, `Suite 4 A,B`, and
   `Upper 5 Port,Starboard`. Is `AB` two berths or one? Pin the
   grammar precisely. Recommend: separator is **comma only**, with a
   single optional whitespace after the cabin label. Reject
   ambiguous rows. List the rejection cases in the test plan.
2. **Preview / save wire format.** Does `PUT /cabins` accept the same
   shape as the `POST /cabins/preview` response? If not, document
   both. If so, document that the server re-validates rather than
   trusts the preview.
3. **Trip cabin board route shell.** The draft adds
   `/admin/trips/:id/cabins` but the existing trip routing model
   uses `/admin/trips/:id/manifest` (no `TripDetail` shell). State
   whether this is a sibling page or a nested tab and define the
   navigation pattern (link from `TripManifest`?). See
   `web/src/admin/pages/TripManifest.tsx`.
4. **Boat-deletion behavior.** With `boat_cabins ON DELETE CASCADE`,
   `boat_cabin_berths ON DELETE CASCADE`, and
   `trip_cabin_assignments.cabin_id / berth_id ON DELETE SET NULL`,
   deleting a boat strands assignment rows with snapshots intact
   but no live references. Specify whether this is intended (history
   preservation) or whether boats should be soft-deleted instead.
5. **Completed-trip behavior.** The draft says completed trips
   preserve labels, but does not say whether assignment edits are
   blocked, whether the cabin board view goes read-only after
   `end_date`, or whether admins can correct historical records.
   This matches Open Question 4 / Open Question 5 in the intent —
   merge must answer.
6. **Required vs. advisory at checkout.** Open Question 3 in the
   intent ("Should cabin assignment be required before guest
   checkout/completion?"). The draft is silent. Sprint 015 closes
   folios atomically; the answer affects whether folio close gates
   on assignment. Recommend: advisory in this sprint, document as
   deferred.
7. **`testdb.go` truncation order.** The draft mentions modifying
   it but does not specify order. The current truncate block is at
   `internal/testdb/testdb.go:120-149`. Cabin tables must be added
   in dependency order: `trip_cabin_assignments` first, then
   `boat_cabin_berths`, then `boat_cabins`, all *before* `trips`,
   `boats`, and `trip_guests` (which is already higher up).
8. **Audit on unassign.** `assigned_by_user_id` / `assigned_at`
   exist, but there is no `unassigned_by` audit trail. Either add
   `unassigned_by_user_id` + `unassigned_at` (soft-delete pattern,
   consistent with `revoked_at` elsewhere) or accept that unassign
   is destructive of audit. State the choice.
9. **Index for layout-dependency queries.** The draft creates
   `trip_cabin_assignments_org_trip_idx` but no index on
   `berth_id` alone. The "block delete if any upcoming/active trip
   references this berth" check will scan otherwise. Add
   `CREATE INDEX trip_cabin_assignments_berth_idx ON trip_cabin_assignments(berth_id) WHERE berth_id IS NOT NULL;`
   or similar.
10. **Cruise Director boat-layout *read* visibility.** The
    permission table says directors do not see boat layout in
    fleet, but they do see assigned-trip cabin board. The board
    needs the layout to render unassigned berths. So directors
    indirectly read layout through trip context. Spell that out
    so the implementer doesn't gate the trip board behind
    `RequireAdmin`.

## 4. Suggested Changes

### 4.1 Tighten schema

```sql
-- prefer ON DELETE RESTRICT for berth_id
berth_id uuid NOT NULL REFERENCES boat_cabin_berths(id) ON DELETE RESTRICT,
-- and drop cabin_id from the assignment row, derive via join.
```

If the merge keeps `cabin_id`, add a `CHECK` (via trigger) that
`cabin_id` matches `berth_id`'s parent.

Add active-only unique partial indexes:

```sql
CREATE UNIQUE INDEX boat_cabins_org_boat_label_active_idx
  ON boat_cabins(organization_id, boat_id, label) WHERE is_active;

CREATE UNIQUE INDEX boat_cabin_berths_display_label_active_idx
  ON boat_cabin_berths(organization_id, boat_id, display_label) WHERE is_active;
```

### 4.2 Resolve the open questions in the merged spec

Provide concrete, defaulted answers (not "pending interview") for at
least:

| Question | Recommended default |
|---|---|
| Director boat layout edit | No — admins only. |
| CSV upload in 016 | Defer; range + paste covers the user's stated need. |
| Required for checkout | Advisory in 016; document as deferred gate. |
| Completed-trip edits | Read-only after `end_date`; admin can override via per-row PATCH (later sprint). |
| Cabin-vs-berth label storage | Already correct: separate fields + generated `display_label`. |

### 4.3 Split layout-edit endpoints by use case

- `PUT /api/admin/boats/{id}/cabins`: only available when the boat
  has zero assigned berths (rejects with 409 otherwise).
- `PATCH` / `DELETE` per cabin and per berth: per-row dependency
  checks, used for post-assignment editing.

This removes the "exactly preserves berth IDs" clause and makes the
UI flow obvious.

### 4.4 Tighten the paste parser spec

Specify the grammar and add to the test list:

- Accepted: `<cabin>,<berth>[,<berth>...]` (commas only).
- One row per cabin.
- Whitespace tolerated around tokens.
- Empty / blank rows ignored.
- Duplicate cabin labels in input → row-level error.
- Berth label longer than N chars → row-level error.
- Mixed delimiters (space + comma) → reject with row number.
- Empty input → 400.

### 4.5 Verify import response shape change

If post-import boat-link UX is in scope, add an explicit task in
Phase 1: extend `import_jobs` (or its read model) with a per-boat
result payload, and update `internal/httpapi/import_handlers.go`
to return `created_boats: [{id, name}, ...]`. Otherwise, downgrade
the post-import UX to "show unconfigured boats on Fleet" and
remove the import-response claim.

### 4.6 Add DESIGN.md compliance task

Add to Phases 3 and 4: "Apply DESIGN.md typography, spacing, and
color tokens to all new components. Flag any deviation."

## 5. Risks the Final Merge Should Address

- **Stranded assignment rows** after layout deletion or guest
  revocation. Documented above. Without a clear lifecycle model,
  the cabin board will eventually render incoherent state.
- **Schema-migration time bomb on display_label uniqueness.** A
  deactivated `1A` plus a re-created `1A` will fail today. Will be
  encountered the first time an operator fixes a typo by
  deactivating + re-creating.
- **Importer race condition.** If liveaboard.com source slug
  changes, the importer creates a new boat row with new ID;
  existing cabins remain on the old boat ID and become orphaned.
  Probably acceptable for now (the operator just re-creates the
  layout on the new boat), but flag in Risks.
- **Test harness drift.** `testdb.go` truncate order must be
  updated atomically with the migration; otherwise CI starts
  silently keeping cabin rows between tests.
- **Director navigation gap.** Directors land on assigned trips but
  the cabin board route is new; the side nav (`web/src/admin/Shell.tsx`)
  may need a director-visible "Cabins" link from manifest, otherwise
  feature is discoverable only by URL.
- **Snapshot drift on relabel.** If an admin renames cabin `1` to
  `Suite Owner` while a guest is assigned, the assignment's
  `cabin_label_snapshot` is *not* automatically updated. Some
  operators expect the manifest to show the current label. Decide
  whether to refresh snapshots on rename for upcoming/active trips,
  or freeze at assign-time. Recommend: freeze at assign-time
  (matches Sprint 015 folio semantics).

## 6. Parts to Reject or Simplify

- **CSV upload as a stretch goal.** The draft says "CSV upload can
  reuse the same preview shape and can be a follow-up if scope
  pressure is high." Strike from sprint 016 entirely. Range + paste
  covers the user's stated need; CSV adds a parser surface and a
  file-upload surface for marginal value at this stage.
- **The "exactly preserves existing assigned berth IDs" branch of
  `ReplaceBoatCabinLayout`.** Replace with the split-by-state
  approach in 4.3.
- **The implication that `BoatTabs.tsx` holds the tab nav.** It
  doesn't. Update the Files Summary so the implementer modifies
  `BoatDetail.tsx` for the nav and adds `BoatCabins.tsx` for the
  page content (or extends `BoatTabs.tsx` with a new exported
  component).
- **Phase 5 lumping docs + verification.** Split into two phases or
  bump the budget; the persona/user-story changes are real product
  policy.
- **`cabin_id` on `trip_cabin_assignments`.** Drop it; derive via
  join. Removes a coherence constraint risk for free.

## 7. Convention Mismatches

- `docs/sprints/README.md` template has a `## References` section
  that the draft omits. Optional but easy to add.
- The draft does not include the per-phase "Files" + "Tasks" tables
  in the exact template form. Mostly cosmetic, but worth aligning.
- `CLAUDE.md` mandates working directly on `main` with focused
  commits. The draft does not list a commit slicing plan. Suggest
  adding to Phase 1: "Commit boundaries: (a) migration + truncate
  helper, (b) store + tests, (c) HTTP + tests, (d) admin layout UI,
  (e) trip board UI, (f) docs."

## 8. Summary

The draft has the right shape: schema split, snapshot semantics,
permission boundaries, and phase ordering all align with prior
sprints. The biggest issues are concentrated in (a) the
`trip_cabin_assignments` lifecycle (nullable FKs, unassign verb,
revoked-guest interaction), (b) under-specified boundaries (paste
parser grammar, layout-replace semantics, post-import response
shape), and (c) absent CLAUDE.md hard requirements (DESIGN.md). All
are fixable in the merge without restructuring the sprint.
