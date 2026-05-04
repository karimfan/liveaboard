# Sprint 010 Merge Notes

## Claude Draft Strengths

- Clean three-axis framing (rename, richer invitations, real landing
  page) sets the sprint's scope crisply.
- Comprehensive file-list grounded in the actual code surface (sidebar
  copy, Trips column rename, accept-page redesign).
- Proposed Phase 4 ASCII landing layout matches the user's selected
  preview from the interview verbatim.
- Profile editing under `/admin/account` (consistent with Sprint 009's
  change-password / change-email pattern) is the right ergonomic
  decision.
- Sequencing: schema first → service → handlers → tests → frontend
  matches how the previous successful sprints were executed.

## Codex Draft Strengths

- Identified that `users_role_check` (added in migration 0003) blocks
  any in-place role rename and must be dropped + recreated. Claude's
  migration sketch only handled `invitations_role_check`. Migrate-time
  bug caught.
- Flagged the underspecified handling of legacy pending invitations
  whose `full_name` would be `''` after `ALTER TABLE … DEFAULT ''`.
- Argued for a single Cruise-Director-only landing payload that
  bundles profile + stats + trips, instead of Claude's two-endpoint
  split. Sharper contract, fewer races, easier tests.
- Pushed back on Claude's "admins can call it too" loosening of the
  landing endpoint — correctly diagnosed as a security/clarity smell.
- Suggested the cleaner endpoint name
  `/api/admin/cruise-director-overview` keeping it in the existing
  `/api/admin/*` namespace.

## Valid Critiques Accepted

1. **`users_role_check` must be dropped + recreated.** Migration 0008
   in the final doc explicitly handles both `users_role_check` and
   `invitations_role_check`.
2. **Pre-existing pending invitations.** The final migration revokes
   them outright (sets `revoked_at = now()` on every pending invitation
   at migrate time). Pre-customer; cleaner than carrying empty-name
   invariants. Admins can re-issue with full metadata.
3. **Landing endpoint shape.** Single payload — `profile + stats +
   trips` — under `/api/admin/cruise-director-overview` with explicit
   `cruise_director`-only role check. The frontend issues one fetch.
4. **Trip-assignment UX clarity.** Use cases re-worded to say
   "existing PATCH endpoint, no new UI in this sprint." Trip-assignment
   UI is a follow-up sprint.
5. **Phone formatting.** Stay loose (`text`) — captured as a follow-up.

## Critiques Rejected (with reasoning)

- None. All five critiques are valid and adopted as written.

## Interview Refinements Applied

1. **Full rename across the stack.** DB column rename, role string
   rename, JSON keys, UI copy, persona doc — all in one migration
   pass, matching the user's "Full rename (recommended)" choice.
2. **Invite-time metadata: full name (required) + phone (optional)
   only.** No private admin note, no emergency contact — those become
   follow-up sprints if requested.
3. **Landing page = contact card + at-a-glance counts + assigned-trips
   table** with the exact preview the user selected. No "next
   departure" hero card; no chart widgets.
4. **Profile editing in `/admin/account`** as a `My profile` section
   above the existing change-password / change-email blocks.

## Final Decisions

- **Migration `0008_cruise_director_rename_and_profile.sql`** handles:
  - `users_role_check` drop+recreate.
  - `invitations_role_check` drop+recreate.
  - `users.role` row update from `site_director` → `cruise_director`.
  - `invitations.role` row update.
  - `trips.site_director_user_id` → `cruise_director_user_id` rename
    + index rename.
  - `users.phone text NULL` add.
  - `invitations.full_name text NOT NULL` + `invitations.phone text
    NULL` add.
  - **Pre-existing pending invitations are revoked** in the same
    migration — cleaner than carrying blank-name fallbacks.
  - Default-then-drop trick on `invitations.full_name` so the migration
    is single-statement-friendly even though all pre-existing rows are
    revoked.
- **Endpoint: `GET /api/admin/cruise-director-overview`** — single
  payload `{profile, stats, trips}`. Cruise-director-only (403 for
  org_admin). Org Admin already has `/api/admin/overview` for their
  variant.
- **Endpoint: `PATCH /api/account/profile`** — `{full_name, phone}`,
  any authenticated user.
- **Invitation request shape:** `{email, full_name, phone?, role?}`.
  `role` defaults to `cruise_director`. `full_name` required.
- **Invitation lookup response:** adds `full_name`. Phone *not*
  exposed in lookup (only inside the org).
- **Invitation accept request shape:** `{token, password}` only —
  `full_name` is now read from the invitation row.
- **Email template** adds `RecipientName` to `Vars`; greets by name;
  mentions inviter and Cruise Director role explicitly.
- **Frontend:** invite modal grows two fields (name + phone). Accept
  page greets by name and asks for password only. Cruise Director
  landing renders the agreed three-card layout. `Account.tsx` adds a
  `My profile` section.
- **Tests:** new tests cover invite metadata persistence, name-aware
  email rendering, the new cruise-director-overview endpoint, and
  profile updates. Existing tests are renamed but not redesigned.
- **Docs:** `personas.md` and the user-stories doc are renamed in
  place (these are living documents). Sprint 002–009 docs stay frozen.
- **Out of scope:** phone normalization, admin-edits-other-user's-
  profile, custom roles, trip-assignment UX overhaul.

## Phase Sequencing (final)

- Phase 1: schema + role rename + invitation metadata (~20%)
- Phase 2: invitation service + email template + accept-flow change
  (~15%)
- Phase 3: HTTP routes + handlers (admin rename, new
  cruise-director-overview, profile patch) (~15%)
- Phase 4: backend tests (~10%)
- Phase 5: frontend types + admin pages + invite/accept rewrites
  (~15%)
- Phase 6: Cruise Director landing page + profile editor (~15%)
- Phase 7: persona/docs + smoke (~10%)
