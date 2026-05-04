# Sprint 010: User Management — Cruise Director Rename, Rich Invitations, Director Landing Page

## Overview

The product owner has settled on **Cruise Director** as the canonical
name for the on-trip-leader persona (Sprint 002 called them Site
Directors). This sprint commits to that rename across the stack and
finishes the user-management story Sprint 008/009 left half-done: an
admin should be able to add a Cruise Director by entering name, email,
and optional contact info, and the invitee should arrive on a real
landing page tailored to their role rather than a stub.

The work is split into three concerns. First, a comprehensive rename:
role constant, DB constraints, the `trips.site_director_user_id`
column, every JSON key, every UI label, and the persona/user-story
docs. Second, an enrichment of the invitation flow to capture the
prospect's full name and phone at invite-time so the admin's mental
model is "I'm adding a Cruise Director" — not "I'm slinging a token at
a stranger." Third, a real Cruise Director landing page at `/admin`:
their assigned trips, their contact card, basic counts. Plus a
self-serve "My profile" panel so the Cruise Director can correct their
own contact info if the admin entered something stale.

This is an extension sprint, not a rewrite. The auth stack stays
intact, the admin chrome stays intact, the trips schema stays intact
modulo a single column rename. Risk is concentrated in the rename: a
clean atomic migration plus a rigorous grep-and-replace pass minimize
in-flight conflicts.

## Use Cases

1. **Add a Cruise Director.** Org Admin opens `/admin/users`, clicks
   `+ Invite Cruise Director`, fills `Name`, `Email`, optional
   `Phone`. Submits. An invitation row is created with the metadata.
   The invitee receives an email addressed by name with a link.
2. **Accept invitation.** Invitee clicks the link, lands on
   `/invitations/:token/accept`, sees `Hi <Name> — set a password to
   join <Org>`. Their email + name are pre-rendered (read-only). They
   set a password and are signed in immediately as `cruise_director`,
   redirected to `/admin`.
3. **See assigned trips on landing.** A Cruise Director who lands on
   `/admin` sees their contact card up top and a list of trips they're
   assigned to — boat, itinerary, dates, status. Empty state explains
   "no assigned trips yet; an admin will assign you when one is
   ready."
4. **Edit own profile.** A Cruise Director navigates to `/admin/
   account`, sees the existing change-password + change-email
   sections, plus a new `My profile` section to edit `Full name` and
   `Phone`. Saved values flow to `users.full_name` / `users.phone`.
5. **Admin views Cruise Director list.** Org Admin opens `/admin/
   users`, sees existing users + pending invitations with the new
   metadata: `Name · Email · Phone · Role · Status`. Resend / Revoke
   actions on pending invites unchanged from Sprint 009.
6. **Admin assigns Cruise Director to trip.** Org Admin opens a trip
   row in `/admin/trips`, picks a Cruise Director from a dropdown.
   Endpoint and UX unchanged in shape — only the labels and JSON key
   names rename.
7. **Direct-link landing for a Cruise Director.** Bookmarking
   `/admin` works post-login; bookmarking `/admin/trips` works.
   Bookmarking `/admin/fleet` or `/admin/users` 403s on the API and
   redirects to `/admin` on the client. (Already true post-Sprint-008,
   verified post-rename.)

## Architecture

### What the rename touches

| Layer | Today | Sprint 010 |
|---|---|---|
| Role constant | `RoleSiteDirector = "site_director"` | `RoleCruiseDirector = "cruise_director"` |
| DB CHECK on invitations.role | `('site_director')` | `('cruise_director')` |
| Trip column | `trips.site_director_user_id` | `trips.cruise_director_user_id` |
| Trip Go field | `Trip.SiteDirectorUserID` | `Trip.CruiseDirectorUserID` |
| Trips JSON | `site_director_user_id` / `site_director_name` | `cruise_director_user_id` / `cruise_director_name` |
| AssignDirector handler | `HandleAssignDirector` | `HandleAssignCruiseDirector` |
| Frontend types | `role: "org_admin" \| "site_director"` | `role: "org_admin" \| "cruise_director"` |
| Sidebar / page copy | "Site Director" | "Cruise Director" |
| Personas doc | `Site Director` heading | `Cruise Director` heading |
| User-stories doc | mentions of "Site Director" | replaced; story IDs unchanged |
| Test seeds | `SeedSiteDirector` | `SeedCruiseDirector` |
| Email template body | "you've been invited as a Site Director" | "you've been invited as a Cruise Director by <inviter>" |

### Schema migration `0008_cruise_director_rename_and_invitation_metadata.sql`

```sql
-- 1. Rename trips.site_director_user_id → cruise_director_user_id.
ALTER TABLE trips
  RENAME COLUMN site_director_user_id TO cruise_director_user_id;
ALTER INDEX trips_site_director_user_id_idx
  RENAME TO trips_cruise_director_user_id_idx;

-- 2. Add invitation metadata: full_name + optional phone the admin
--    captured at invite time. Pre-Sprint-010 invitations (if any are
--    still pending) get an empty string default; new ones REQUIRE a
--    full_name.
ALTER TABLE invitations
  ADD COLUMN full_name text NOT NULL DEFAULT '',
  ADD COLUMN phone     text NULL;

-- 3. Add the same fields to users so they survive past acceptance.
ALTER TABLE users
  ADD COLUMN phone text NULL;

-- 4. Update the role CHECK on invitations to use the new label.
--    Drop+recreate is the cleanest path.
ALTER TABLE invitations
  DROP CONSTRAINT IF EXISTS invitations_role_check;
UPDATE invitations
  SET role = 'cruise_director'
  WHERE role = 'site_director';
ALTER TABLE invitations
  ADD CONSTRAINT invitations_role_check
  CHECK (role IN ('cruise_director'));

-- 5. Existing user rows: rename role string in place.
UPDATE users SET role = 'cruise_director' WHERE role = 'site_director';

-- 6. Drop the temporary default on invitations.full_name now that any
--    live rows are populated. New invitations must specify it.
ALTER TABLE invitations
  ALTER COLUMN full_name DROP DEFAULT;
```

The migration is in one file, one transaction (goose handles that).
Pre-Sprint-010 dev DBs migrate cleanly because the DEFAULT '' covers
existing rows and the role-string update is idempotent.

### Invitation flow shape (after Sprint 010)

```
Browser (admin)                    API                          DB
─────────────────                  ─────                        ─
POST /api/invitations              auth.Service.Invite          INSERT invitations(
  {email, full_name, phone?}                                      email, role,
                                                                  full_name, phone,
                                                                  invited_by, token_hash, …)
                                   email.Render(invitation, vars
                                       → InviterName,
                                         RecipientName,
                                         RecipientEmail,
                                         ActionURL)
                                   email.Send                   ─

Browser (invitee)                  API                          DB
─────────────────                  ─────                        ─
GET /api/invitations/lookup        auth.Service.Lookup          SELECT invitations + org
  ?token=…                                                      ↳ {full_name, role,
                                                                    org_name, expires_at}

POST /api/invitations/accept       auth.Service.Accept          BEGIN
  {token, password}                                             INSERT users(
                                                                  full_name = inv.full_name,
                                                                  phone     = inv.phone,
                                                                  role      = 'cruise_director',
                                                                  email_verified_at = now())
                                                                UPDATE invitations
                                                                  SET accepted_at = now(),
                                                                      accepted_user_id = u.id
                                                                INSERT sessions(user_id, …)
                                                                COMMIT
```

The invitee no longer types their own name at accept time — the admin
already entered it. The accept page collects only password.

### Cruise Director landing page

```
+--------------------------------------------------------------+
|  Liveaboard                                                  |
|  ────                                                        |
|  Overview          ←—  active                                |
|  Trips                                                       |
|  ────                                                        |
|  Maya Sanchez                                                |
|  maya@example.com · Cruise Director                          |
+--------------------------------------------------------------+
|  +-------------------------------+  +---------------------+  |
|  | Maya Sanchez                  |  | At a glance        |  |
|  |                               |  |                    |  |
|  | maya@example.com              |  | Upcoming trips  3  |  |
|  | +1 555 0142                   |  | Active trips    1  |  |
|  | Acme Diving · Cruise Director |  | Past trips      8  |  |
|  |                               |  |                    |  |
|  | [ Edit profile ]              |  +---------------------+  |
|  +-------------------------------+                           |
|                                                              |
|  My trips                                                    |
|  ──────────────────────────────────────────────────────────  |
|  STATUS    BOAT          ITINERARY              DATES        |
|  Active    Gaia Love     Komodo North          May 14-21    |
|  Planned   Seahorse      Raja Ampat            Jun 02-09    |
|  Planned   Gaia Love     Komodo                Jul 11-18    |
|  Past      Gaia Love     Banda Sea             Apr 02-13    |
|  …                                                           |
+--------------------------------------------------------------+
```

Components:

- **Contact card** (top-left). Read-only render of `users.full_name`,
  `users.email`, `users.phone`, `organizations.name`, role label.
- **At-a-glance counts** (top-right). Three numbers from a single
  endpoint: active / upcoming / past trips.
- **My trips table**. Same shape as the admin Trips list (already
  exists), but rendered against the Cruise-Director-scoped variant of
  the existing endpoint (`/api/admin/trips` already does role-aware
  scoping per Sprint 008).

### `/admin/account` — new "My profile" section

Two existing sections (Change password, Change email) gain a sibling
above:

```
+----------------------------+
|  My profile                |
|                            |
|  Full name [Maya Sanchez]  |
|  Phone     [+1 555 0142]   |
|  Email      maya@x.test    |  ← read-only; change via section below
|                            |
|              [ Save ]      |
+----------------------------+
```

Backed by `PATCH /api/account/profile` — `{full_name, phone}`. Org
Admins can use this too (their phone is optional).

### Admin "Add Cruise Director" modal

Replaces the current single-`email` modal in `/admin/users`. Fields:

```
Full name        [_________________________]   (required)
Email            [_________________________]   (required)
Phone (optional) [_________________________]
Role             ( • Cruise Director )         (single option)

[ Cancel ]   [ Send invitation ]
```

The role row is deliberately a single read-only option for now —
visible affordance for when more roles arrive, costs nothing today.

### Backend endpoint changes

| Method | Path | Change |
|---|---|---|
| POST | `/api/invitations` | Accepts `full_name` + optional `phone` in addition to `email` and `role`. `full_name` required. |
| GET | `/api/invitations/lookup` | Returns `full_name` so the accept page can greet by name. |
| POST | `/api/invitations/accept` | Drops `full_name` from input shape — name comes from the invitation row. Body: `{token, password}`. |
| GET | `/api/invitations` | Lists invitations including new metadata (`full_name`, `phone`). |
| PATCH | `/api/admin/trips/{id}` | JSON key `site_director_user_id` → `cruise_director_user_id`. |
| GET | `/api/admin/trips` | Trip JSON keys renamed: `cruise_director_user_id`, `cruise_director_name`. |
| GET | `/api/admin/users` | Adds `phone` to user rows. |
| GET | `/api/admin/overview` | Counts JSON: `cruise_directors` replaces `site_directors`. |
| GET | `/api/cruise-director/landing` | NEW. Returns `{user, org_name, counts: {upcoming, active, past}}` for the new landing page header (trips themselves keep coming from `/api/admin/trips`). |
| PATCH | `/api/account/profile` | NEW. `{full_name, phone}` — updates own user record. |

### Test seed renames

`internal/testdb/testdb.go` `SeedSiteDirector` → `SeedCruiseDirector`,
returning a verified user with role `cruise_director`.

## Implementation Plan

### Phase 1: Schema rename + role rename + invitation metadata (~20%)

**Files:**
- `internal/store/migrations/0008_cruise_director_rename_and_invitation_metadata.sql` — Create.
- `internal/store/users.go` — `RoleSiteDirector` → `RoleCruiseDirector`; add `Phone *string` to `User`; scan it everywhere.
- `internal/store/trips.go` — `SiteDirectorUserID` → `CruiseDirectorUserID`; rename function `AssignSiteDirector` → `AssignCruiseDirector`; rename `TripsForUser` (already director-scoped).
- `internal/store/invitations.go` — add `FullName` + `Phone` to `Invitation`; update `CreateInvitation`, `RotateInvitationToken`, `InvitationByToken`, `PendingInvitationsForOrg` SQL.
- `internal/testdb/testdb.go` — `SeedSiteDirector` → `SeedCruiseDirector`.

**Tasks:**
- [ ] Migration `0008` with the SQL above.
- [ ] Rename role constant + every `RoleSiteDirector` reference in
      `internal/`.
- [ ] Add `phone` field to `User`; touch all SQL scan sites.
- [ ] Rename `trips.site_director_user_id` reads / writes / fields /
      methods. Compile pass.
- [ ] Update `Invitation` struct + queries to include name + phone.
- [ ] `internal/store/store_test.go` and store-level tests still pass
      under the new column name.

### Phase 2: Invitation service + accept-flow change (~15%)

**Files:**
- `internal/auth/invitations.go` — `Invite(orgID, inviterID, email, fullName, phone, role)`. Accept: drop `fullName` parameter from `AcceptInvitation` (it now comes from the row).
- `internal/auth/auth.go` — adjust `Service` knobs if needed.
- `internal/email/templates/invitation.{txt,html}.tmpl` — greet by name, mention inviter and org.
- `internal/email/email.go` — extend `Vars` with `RecipientName` if not already there.

**Tasks:**
- [ ] Service signatures updated.
- [ ] Email templates use `{{.RecipientName}}` salutation:
      "Hi {{.RecipientName}}," / "{{.InviterName}} invited you to join
      {{.OrganizationName}} as Cruise Director."
- [ ] Lookup returns `full_name` so the SPA can pre-render the
      greeting on the accept page.
- [ ] Existing tests rewritten: invite passes name; accept consumes
      token without name input; lookup returns name.

### Phase 3: HTTP routes + handlers (~15%)

**Files:**
- `internal/httpapi/invitation_handlers.go` — request struct grows
  `full_name` + `phone`; accept request shrinks to `{token,
  password}`; lookup response gains `full_name`.
- `internal/httpapi/admin.go` — rename `HandleAssignDirector` →
  `HandleAssignCruiseDirector`; rename JSON keys; add `phone` to user
  rows.
- `internal/httpapi/auth_handlers.go` — new `handleUpdateProfile`
  behind `RequireSession`.
- `internal/httpapi/cruise_director.go` — NEW. Houses
  `HandleLanding` returning user + org + counts.
- `internal/httpapi/httpapi.go` — mount the new endpoints.

**Tasks:**
- [ ] Wire endpoints; update handler tests.
- [ ] `Overview` counts JSON renamed.
- [ ] Trip JSON keys renamed in both list + boat-trips endpoints.
- [ ] `GET /api/cruise-director/landing` mounted behind
      `RequireSession`. Org Admins can call it too — they just see
      their own contact info plus zero trips (or admin-side counts;
      pick the simpler shape).

### Phase 4: Admin tests + auth tests + store tests (~10%)

**Files:**
- `internal/httpapi/admin_test.go` — every reference to `site_director`
  swaps; the `bootstrapDirector` helper renames; `HandleAssignDirector`
  test renames.
- `internal/httpapi/httpapi_test.go` — `signupAndVerify` / accept-
  invitation tests update for new shape.
- `internal/store/organizations_test.go` — references to
  `RoleSiteDirector` rename.
- New: `internal/httpapi/cruise_director_test.go` covering
  `/api/cruise-director/landing` for both roles.

**Tasks:**
- [ ] All existing test names + variable names updated.
- [ ] New test for the landing endpoint (admin: returns own card +
      empty/admin counts; cruise director: returns own card + scoped
      trip counts).
- [ ] All tests green.

### Phase 5: Frontend rename + types (~10%)

**Files:**
- `web/src/admin/api.ts` — rename JSON keys + types
  (`site_director_user_id` → `cruise_director_user_id`, etc.).
  `role: "org_admin" | "cruise_director"`.
- `web/src/admin/useMe.tsx` — same rename.
- `web/src/lib/api.ts` — `Invitation` and `InvitationLookup` types
  gain `full_name` and `phone` (optional).
- `web/src/admin/Shell.tsx` — sidebar copy + footer role label.
- `web/src/admin/pages/Trips.tsx` — column header + cell copy.
- `web/src/admin/pages/BoatTabs.tsx` — same.
- `web/src/admin/pages/Users.tsx` — invite modal grows `full_name`
  required + `phone` optional; pending list renders new fields.
- `web/src/pages/AcceptInvitation.tsx` — stop asking for name; greet
  by name from lookup.
- DESIGN.md — no change. (The new modal/landing card use existing
  tokens.)

**Tasks:**
- [ ] Single-pass rename of types + copy.
- [ ] Invite modal updated (3 fields, `Send invitation` button).
- [ ] Accept page redesigned to show `Hi {name}` + password-only form.
- [ ] `npm run build` clean; `tsc -b` happy.

### Phase 6: Cruise Director landing page + profile editor (~20%)

**Files:**
- `web/src/admin/pages/Overview.tsx` — replace `SiteDirectorOverview`
  stub with the real `CruiseDirectorLanding` component that calls
  `/api/cruise-director/landing` + `/api/admin/trips` and renders the
  contact card + counts + trips table.
- `web/src/admin/pages/Account.tsx` — add `MyProfile` section above
  the existing change-password / change-email blocks.
- `web/src/admin/api.ts` — `cruiseDirectorLanding()` and
  `updateProfile({full_name, phone})` methods.
- `web/src/styles/app.css` — small additions: contact card,
  at-a-glance card, profile form.

**Tasks:**
- [ ] Landing page renders against real data.
- [ ] Profile edit POSTs and refreshes `useMe` so the sidebar updates
      live.
- [ ] Empty state ("no assigned trips yet") covered.
- [ ] Sidebar role label updates immediately after profile save.

### Phase 7: Persona / docs / smoke (~10%)

**Files:**
- `docs/product/personas.md` — heading + every body mention of "Site
  Director" → "Cruise Director". Add `phone` to the "Owns" list (a
  Cruise Director's contact info follows them across orgs / trips).
- `docs/product/organization-admin-user-stories.md` — story copy
  mentioning Site Director updates. Story IDs unchanged.
- `docs/auth.md` — invitation table row updated (extra fields).
- `RUNNING.md` — "invite a Cruise Director" example.
- `CLAUDE.md` — no change required (it doesn't mention the role by
  name).

**Tasks:**
- [ ] Docs updated.
- [ ] **Live smoke** against Brevo:
      1. Admin invites a real address with name + phone.
      2. Email arrives addressed to that name; click link.
      3. Accept page greets by name; set password; land on
         `/admin`.
      4. Landing renders contact card + zero trips empty state.
      5. Admin assigns the new Cruise Director to a trip; reload the
         landing — trip appears in `My trips` table.
- [ ] Final QA: `gofmt -l .`, `go vet ./...`, `go test ./...`,
      `npm --prefix web run build` all clean.

## API Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/invitations` | RequireOrgAdmin | Create invitation. **Request grows `full_name` (required) + `phone` (optional).** |
| GET | `/api/invitations/lookup?token=` | none | Public lookup. **Response gains `full_name`.** |
| POST | `/api/invitations/accept` | none | Consume token. **Request shrinks to `{token, password}`.** |
| GET | `/api/invitations` | RequireOrgAdmin | List pending. **Response items gain `full_name`, `phone`.** |
| GET | `/api/admin/users` | RequireOrgAdmin | List users. **Response items gain `phone`.** |
| GET | `/api/admin/overview` | RequireOrgAdmin | **Count key renamed: `cruise_directors`.** |
| PATCH | `/api/admin/trips/{id}` | RequireOrgAdmin | **Body key renamed: `cruise_director_user_id`.** |
| GET | `/api/admin/trips` | RequireSession | **Trip rows: keys renamed.** Director scoping unchanged. |
| GET | `/api/admin/boats/{id}/trips` | RequireOrgAdmin | **Same key renames.** |
| GET | `/api/cruise-director/landing` | RequireSession | **NEW.** `{user, org_name, role, counts: {upcoming, active, past}}`. |
| PATCH | `/api/account/profile` | RequireSession | **NEW.** `{full_name, phone}`. |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-010.md` | Create | This sprint doc. |
| `docs/sprints/drafts/SPRINT-010-*.md` | Create | Planning artifacts. |
| `docs/product/personas.md` | Modify | Rename + phone field. |
| `docs/product/organization-admin-user-stories.md` | Modify | Rename in story copy. |
| `docs/auth.md` | Modify | Invitation request shape. |
| `RUNNING.md` | Modify | Cruise Director invite example. |
| `internal/store/migrations/0008_cruise_director_rename_and_invitation_metadata.sql` | Create | Schema rename + new columns. |
| `internal/store/users.go` | Modify | Role rename, `Phone` field. |
| `internal/store/trips.go` | Modify | Column rename, method rename. |
| `internal/store/invitations.go` | Modify | New `FullName` / `Phone` columns. |
| `internal/testdb/testdb.go` | Modify | `SeedCruiseDirector`. |
| `internal/auth/invitations.go` | Modify | Service signatures. |
| `internal/auth/auth.go` | Modify | Possibly small if anything mentions role. |
| `internal/email/email.go` | Modify | `Vars` adds `RecipientName`. |
| `internal/email/templates/invitation.{txt,html,subject}.tmpl` | Modify | Greet by name. |
| `internal/httpapi/admin.go` | Modify | Rename handler + JSON keys + counts. |
| `internal/httpapi/invitation_handlers.go` | Modify | New invite shape. |
| `internal/httpapi/auth_handlers.go` | Modify | Add `handleUpdateProfile`. |
| `internal/httpapi/cruise_director.go` | Create | Landing endpoint. |
| `internal/httpapi/httpapi.go` | Modify | Mount new routes. |
| `internal/httpapi/admin_test.go` | Modify | Rename helpers. |
| `internal/httpapi/httpapi_test.go` | Modify | New invitation flow. |
| `internal/httpapi/cruise_director_test.go` | Create | Landing tests. |
| `internal/store/organizations_test.go` | Modify | Rename in tests. |
| `web/src/admin/api.ts` | Modify | Type + JSON key renames; new methods. |
| `web/src/admin/useMe.tsx` | Modify | Role union rename. |
| `web/src/admin/Shell.tsx` | Modify | Copy update. |
| `web/src/admin/pages/Trips.tsx` | Modify | Column copy. |
| `web/src/admin/pages/BoatTabs.tsx` | Modify | Column copy. |
| `web/src/admin/pages/Users.tsx` | Modify | Invite modal expansion + table columns. |
| `web/src/admin/pages/Overview.tsx` | Modify | New `CruiseDirectorLanding`. |
| `web/src/admin/pages/Account.tsx` | Modify | Add `MyProfile`. |
| `web/src/lib/api.ts` | Modify | Invitation type expansion + new endpoints. |
| `web/src/pages/AcceptInvitation.tsx` | Modify | Greet by name; password-only form. |
| `web/src/styles/app.css` | Modify | Contact card + landing styles. |

## Definition of Done

- [ ] Migration `0008` applies cleanly to a Sprint 009-state DB and to
      a fresh DB. The role-string `UPDATE` and DEFAULT-trick keep
      pre-existing rows valid.
- [ ] No `site_director` / `Site Director` strings remain in
      `internal/`, `web/src/`, `cmd/`, or `docs/product/` except for
      explicitly historical references in `docs/sprints/SPRINT-007.md`
      and earlier (those sprints are immutable archives).
- [ ] **Backend tests pass.** New tests:
      - Invite with full_name + phone seeds them on accept.
      - Accept consumes token without a name input.
      - Lookup returns full_name.
      - Email template renders with `{{.RecipientName}}` populated.
      - `/api/cruise-director/landing` returns: for an admin, their
        contact card + zero trip counts; for a Cruise Director, their
        contact card + scoped counts.
      - `PATCH /api/account/profile` updates the user's name + phone
        and is rejected with 401 when unauthenticated.
- [ ] **Frontend tests pass / build clean.** SPA build is green.
- [ ] **Live smoke** through Brevo:
      - Invite a real address with full name + phone; email arrives
        with name in greeting; accept; land on `/admin`; see the
        contact card.
      - Admin assigns the new Cruise Director to a trip; reload
        `/admin` (as the Cruise Director); the trip is in the My
        Trips table.
      - Profile edit (changing phone) sticks in the contact card +
        the admin's `/admin/users` table.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm --prefix
      web run build` all clean.
- [ ] `docs/product/personas.md` and the two recent sprint personas
      (Sprint 008, 009) are coherent post-rename. (Their final docs
      stay frozen as historical record; only `personas.md` and the
      user-stories doc reflect the rename.)
- [ ] `tracker.tsv` registers Sprint 010 as completed via
      `go run docs/sprints/tracker.go complete 010`.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Rename misses a string and a test fails post-merge | Medium | Low | Final pass: `git grep -i 'site.director\|sitedirector\|site_director'` returns zero hits except in `docs/sprints/SPRINT-00[1-9].md`. |
| Migration fails on a pre-Sprint-010 DB with legacy `site_director` invitation rows | Low | Medium | Update step (5) handles the `users.role` rename in-place; invitations CHECK is dropped+recreated; all in one transaction. |
| Foreign-keys to `trips.site_director_user_id` break during column rename | Low | Low | The column has only a single index; we rename it explicitly. No FK targets `trips.<old>` from outside the column itself; FK from `trips → users` is on `cruise_director_user_id` post-rename automatically (`REFERENCES users(id)`). |
| Existing Sprint 008 trip-list scoping for non-admins regresses | Low | High | Phase 1 store-layer rename is first; phase 4 runs the existing scope tests. Adding the landing endpoint is additive. |
| Accept page's "name from invitation" surprises a user who wants to change their name | Low | Low | Mention that they can edit their profile from `/admin/account` after accepting. The contact card on the landing has an "Edit profile" link. |
| Phone number storage opens "what about country code, validation?" rabbit hole | Medium | Low | Store as plain `text` (no validation, no normalization). E.164 normalization is a follow-up sprint. |
| Tracker / merge notes for Sprint 010 reference both old and new names | Low | Low | Mention in merge notes that the rename is canonical going forward. |
| Renaming JSON keys (`site_director_user_id` → `cruise_director_user_id`) breaks any external integration | Very low | Low | No external consumers exist (pre-customer); SPA is the only client. |
| Live Brevo smoke gets throttled mid-test | Low | Low | Use the same recipient address as Sprint 009's smoke (mr.karim.fanous@gmail.com). |

## Security Considerations

- **Invitation metadata leakage.** The invitee's `full_name` and
  `phone` come from the admin who invited them. The lookup endpoint
  exposes `full_name` (so the SPA can greet by name) but **not**
  `phone`. Phone is only visible to admins of the same org and to the
  invitee themselves once they accept.
- **Profile editing surface.** `PATCH /api/account/profile` updates
  only `full_name` + `phone` on the calling user's row. Email is
  untouched (existing change-email flow handles that). Role is
  untouched. No tenancy fields are mutable. Other users' rows are
  inaccessible.
- **Multi-tenant isolation.** All queries that touch the new fields
  scope by `organization_id` (admin list, admin overview) or by
  `user_id` from the session (profile, landing).
- **Migration ordering.** Update step runs before constraint recreate;
  no row can violate the new CHECK because step (4) has already
  rewritten role strings.
- **Existing security boundaries unchanged.** RBAC middleware,
  RequireOrgAdmin, server-side trip scoping, session cookie semantics
  all unchanged.

## Dependencies

- **Sprint 008** (admin chrome, RBAC, trip scoping) — built upon.
- **Sprint 009** (custom auth + Brevo) — built upon.
- No new Go module deps; no new npm deps.
- No external services beyond Brevo SMTP (already in
  `LIVEABOARD_SMTP_*`).

## Out of Scope (captured as follow-ups)

- Phone-number validation / normalization (E.164).
- Per-user notification preferences.
- Cruise Director self-deactivation or "leave organization" flow.
- Cross-org Cruise Director (a director who works for multiple
  operators) — single-org per user remains the rule.
- Trip-detail page and the assignment UX itself (already exists in
  Sprint 008's chrome).
- Custom roles / multi-admin / role administration UI.
- Audit log of invitation events.

## Open Questions

1. **Single role option in the modal.** Show a fixed `Cruise
   Director` role line, or omit the field entirely until more roles
   exist? (Lean toward a fixed visible row for affordance.)
2. **Profile fields beyond name/phone.** Add a `notes` field on the
   invitation (admin's private note) — useful operationally but
   leaks scope. Defer.
3. **Display name vs full name.** Invitation captures `full_name`;
   the landing greeting is `{{.RecipientName}}`. Treat them as the
   same thing for now; revisit if operators ask.
4. **Should the admin's `/admin/users` page show Cruise Directors
   with a "View profile" affordance?** Out of scope for this sprint —
   the existing list already shows everything we'd need to display.
5. **Where does the Cruise Director's profile-edit live?** Inline at
   `/admin/account` (proposed) is consistent with the existing
   change-password / change-email pattern, but a dedicated
   `/admin/profile` would be cleaner. Lean toward `/admin/account`
   for now — it's the existing surface and a tab there is one less
   sidebar item to evaluate.

## References

- Sprint 002 — `docs/sprints/SPRINT-002.md` (original "Site Director"
  user-story backlog).
- Sprint 007 — `docs/sprints/SPRINT-007.md` (Control Tower IA
  recommendation; Site Director UX explicitly deferred to a future
  sprint — that future is now).
- Sprint 008 — `docs/sprints/SPRINT-008.md` (Site Director RBAC + stub
  landing; the trip-scope endpoint we now lean on).
- Sprint 009 — `docs/sprints/SPRINT-009.md` (custom invitation flow we
  extend).
- Personas — `docs/product/personas.md`.
- Stories — `docs/product/organization-admin-user-stories.md`.
- Design — `DESIGN.md`.
- Codex critique — `docs/sprints/drafts/SPRINT-010-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-010-MERGE-NOTES.md`.
