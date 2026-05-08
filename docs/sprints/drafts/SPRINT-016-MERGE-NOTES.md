# Sprint 016 Merge Notes

## Codex Draft Strengths

- Preserved the separation between reusable boat cabin layout and per-trip berth assignments.
- Correctly modeled assignment at berth level instead of cabin level.
- Included snapshot labels on assignments so trip history survives later layout edits.
- Used existing manifest authorization patterns for trip-scoped assignment routes.
- Planned range generation and paste-preview flows to avoid tedious manual berth entry.
- Identified that directors should reach cabin work through assigned trips, not Fleet.

## Claude Code Critique Strengths

- Identified lifecycle ambiguity in nullable assignment FKs and destructive layout edits.
- Caught the revoked-guest issue where a revoked assigned guest could continue blocking a berth.
- Recommended active-only partial unique indexes for cabin/berth labels.
- Pointed out that post-import setup links require import result payload changes, not just UI changes.
- Caught the missing `DESIGN.md` UI requirement from `CLAUDE.md`.
- Clarified that `{guest_id}` in existing routes means `trip_guest_id`.

## Valid Critiques Accepted

- `trip_cabin_assignments` should reference `berth_id` with `ON DELETE RESTRICT`; `cabin_id` should be derived by join instead of duplicated.
- Unassign should be soft-audited with `unassigned_at` and `unassigned_by_user_id`, and active uniqueness should only consider rows where `unassigned_at IS NULL`.
- Guest invite revocation should auto-unassign the guest's active cabin assignment.
- `PUT /boats/{id}/cabins` bulk replacement should be allowed only when no active assignments reference the boat layout. Later edits use per-row patch/deactivate routes.
- Cabin and berth uniqueness should be active-only partial unique indexes.
- Import jobs need a result payload that can include created/unconfigured boats.
- UI phases must explicitly follow `DESIGN.md`.
- The final plan should settle open questions instead of punting them.

## Critiques Rejected or Modified

- Claude recommended deferring CSV upload, but the user explicitly wants "all of the above." CSV upload is included in Sprint 016 with a strict schema hint/template.
- Claude recommended Admin-only reusable layout edits. The user explicitly said Cruise Directors can change both layout and assignments. The final plan allows assigned Cruise Directors to edit the layout of boats for their assigned trips, with the same destructive-edit guards as Admins.
- Claude recommended cabin assignment be advisory. The user clarified it is mandatory when binding/enrolling a guest onto a trip. The final plan makes berth assignment part of add-guest/enrollment and supports later changes.
- Claude suggested completed trips be read-only by default. The final plan keeps assignment changes allowed for Admins and assigned Directors during Sprint 016 because the user emphasized later changes by director/admin; destructive layout edits remain guarded.

## Interview Refinements Applied

- Cruise Directors can edit both reusable boat layout and trip assignments when they are assigned to a trip on that boat.
- Range generation, paste parsing, and CSV upload are all in scope.
- CSV upload must provide a clear schema hint/template before upload.
- Cabin assignment is required at guest enrollment: adding/binding a guest to a trip must include a berth, or enrollment is rejected.
- Cabin assignments may be changed later by Org Admins or assigned Cruise Directors.

## Final Decisions

- Boat layout is reusable and operator-owned, but assigned Cruise Directors may edit it through assigned-trip context because that is an operational requirement.
- Bulk layout replacement is for unassigned/unconfigured boats only; once assignments exist, layout changes are incremental.
- Trip assignments soft-delete on unassign for audit and to release uniqueness constraints.
- Revoking a trip guest automatically unassigns their active berth.
- The manifest add-guest API and UI will require `berth_id`.
- Existing invited guests without assignments must be handled by a backfill/assignment-required state before future critical actions; Sprint 016 should expose a clear "needs cabin" status.
- CSV schema is: `cabin_label,berth_label,deck,sort_order,notes`, one berth per row.
