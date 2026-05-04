# Sprint 010: User Management — Cruise Director Rename, Rich Invitations, Director Landing Page

## Overview

The product owner has settled on **Cruise Director** as the canonical
name for the on-trip-leader persona (Sprint 002 named them Site
Directors). Sprint 010 commits to that rename across the stack and
finishes the user-management story Sprints 008/009 left half-done: an
admin should be able to add a Cruise Director by entering name, email,
and optional phone, and the invitee should arrive on a real landing
page tailored to their role rather than the Sprint 008 stub.

The work is three concerns in one sprint. First, a comprehensive
rename: role constant, DB CHECK constraints (`users_role_check` and
`invitations_role_check`), the `trips.site_director_user_id` column,
every JSON key, every UI label, and the persona/user-story docs.
Second, an enrichment of the invitation flow to capture the prospect's
full name and phone at invite time so the admin's mental model is "I'm
adding a Cruise Director" — not "I'm slinging a token at a stranger."
Third, a real Cruise Director landing page at `/admin`: their assigned
trips, their contact card, simple counts. Plus a self-serve `My
profile` section under `/admin/account` so the Cruise Director can
correct their own contact info if the admin entered something stale.

This is an extension sprint, not a rewrite. The auth stack stays
intact, the admin chrome stays intact, the trips schema stays intact
modulo a single column rename. Risk is concentrated in the rename: the
migration drops + recreates both role check constraints, updates row
values inside the same transaction, and **revokes every pre-Sprint-010
pending invitation** so the new "name from invitation row" contract
has no blank-name edge case.

## Use Cases

1. **Add a Cruise Director.** Org Admin opens `/admin/users`, clicks
   `+ Invite Cruise Director`, fills `Full name`, `Email`, optional
   `Phone`. Submits. An invitation row is created with the metadata.
   The invitee receives an email greeting them by name with a link.
2. **Accept invitation.** Invitee clicks the link, lands on
   `/invitations/:token/accept`, sees `Hi <Name> — set a password to
   join <Org>`. Their email and name are pre-rendered (read-only).
   They set a password and are signed in immediately as
   `cruise_director`, redirected to `/admin`.
3. **See assigned trips on landing.** A Cruise Director who lands on
   `/admin` sees their contact card up top, three at-a-glance counts
   (`upcoming`, `active`, `past`), and a list of trips they're
   assigned to — boat, itinerary, dates, status — sorted by start
   date. Empty state explains "no assigned trips yet; an admin will
   assign you when one is ready."
4. **Edit own profile.** A Cruise Director navigates to `/admin/
   account`, sees the existing change-password + change-email
   sections, plus a new `My profile` section to edit `Full name` and
   `Phone`. Saves flow to `users.full_name` / `users.phone`. The
   sidebar role label refreshes immediately.
5. **Admin reviews user list.** Org Admin opens `/admin/users`, sees
   existing users + pending invitations with the new metadata: `Name
   · Email · Phone · Role · Status`. Resend / Revoke actions on
   pending invites unchanged from Sprint 009.
6. **Admin assigns Cruise Director to trip.** The Sprint 008 PATCH
   endpoint at `/api/admin/trips/{id}` continues to accept an
   assignment; only the JSON key renames
   (`site_director_user_id` → `cruise_director_user_id`). **The
   admin-side UI for picking a director is not part of this sprint** —
   that's a follow-up. The endpoint remains exercisable from API tests
   and from the existing seed scripts.

## Architecture

### What the rename touches

| Layer | Today | Sprint 010 |
|---|---|---|
| Role constant | `RoleSiteDirector = "site_director"` | `RoleCruiseDirector = "cruise_director"` |
| `users_role_check` constraint | `IN ('org_admin', 'site_director')` | `IN ('org_admin', 'cruise_director')` |
| `invitations_role_check` constraint | `IN ('site_director')` | `IN ('cruise_director')` |
| Trip column | `trips.site_director_user_id` | `trips.cruise_director_user_id` |
| Trip index | `trips_site_director_user_id_idx` | `trips_cruise_director_user_id_idx` |
| Trip Go field | `Trip.SiteDirectorUserID` | `Trip.CruiseDirectorUserID` |
| Trips JSON | `site_director_user_id` / `site_director_name` | `cruise_director_user_id` / `cruise_director_name` |
| Assign handler | `HandleAssignDirector` | `HandleAssignCruiseDirector` |
| Frontend role union | `"org_admin" \| "site_director"` | `"org_admin" \| "cruise_director"` |
| Sidebar / page copy | "Site Director" | "Cruise Director" |
| Personas doc | `Site Director` heading | `Cruise Director` heading |
| User-stories doc | mentions of "Site Director" | replaced; story IDs unchanged |
| Test seeds | `SeedSiteDirector` | `SeedCruiseDirector` |

### Schema migration `0008_cruise_director_rename_and_profile.sql`

The migration is one file, one transaction. Order matters: drop both
role-check constraints **before** the row rewrites; revoke legacy
pending invitations so the new `full_name` requirement has no blank
fallbacks; rename column + index together; recreate constraints with
the new label set.

```sql
-- 0008_cruise_director_rename_and_profile.sql

-- 1. Drop both role checks so we can update existing values + recreate
--    with the new allowed-set.
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE invitations
    DROP CONSTRAINT IF EXISTS invitations_role_check;

-- 2. Rewrite role strings on existing rows.
UPDATE users        SET role = 'cruise_director' WHERE role = 'site_director';
UPDATE invitations  SET role = 'cruise_director' WHERE role = 'site_director';

-- 3. Revoke every pre-Sprint-010 pending invitation. The new contract
--    requires invitations.full_name; any older pending row has none.
--    Pre-customer, so revoke and let admins re-invite with metadata.
UPDATE invitations
    SET revoked_at = now(), updated_at = now()
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- 4. Recreate role checks with the new value set.
ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('org_admin', 'cruise_director'));
ALTER TABLE invitations
    ADD CONSTRAINT invitations_role_check
    CHECK (role IN ('cruise_director'));

-- 5. Rename trip assignment column + supporting index.
ALTER TABLE trips
    RENAME COLUMN site_director_user_id TO cruise_director_user_id;
ALTER INDEX trips_site_director_user_id_idx
    RENAME TO trips_cruise_director_user_id_idx;

-- 6. Add the new metadata columns.
ALTER TABLE users
    ADD COLUMN phone text NULL;

-- invitations.full_name is required. We add it with a temporary
-- default of '' so the ADD COLUMN succeeds even if any non-revoked
-- rows survive (they shouldn't, given step 3, but the default is
-- defense-in-depth). Then we drop the default so future inserts must
-- specify a real name.
ALTER TABLE invitations
    ADD COLUMN full_name text NOT NULL DEFAULT '',
    ADD COLUMN phone     text NULL;

ALTER TABLE invitations
    ALTER COLUMN full_name DROP DEFAULT;
```

### Invitation lifecycle (after Sprint 010)

```
Browser (admin)                    API                                  DB
─────────────────                  ─────                                ─
POST /api/invitations              auth.Service.Invite(...)             INSERT invitations(
  {email, full_name,                                                      email, full_name, phone,
   phone?, role?}                                                         role='cruise_director',
                                                                          token_hash, …)

                                   email.Render(KindInvitation, Vars{
                                     RecipientName, RecipientEmail,
                                     OrganizationName, InviterName,
                                     ActionURL, ExpiresAt})
                                   email.Send                            ─

Browser (invitee)                  API                                  DB
─────────────────                  ─────                                ─
GET /api/invitations/lookup        auth.Service.LookupInvitation        SELECT invitations + org
  ?token=…                                                              ↳ {full_name, role,
                                                                            organization_name,
                                                                            expires_at}

POST /api/invitations/accept       auth.Service.AcceptInvitation        BEGIN
  {token, password}                                                     INSERT users(
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

### Cruise Director landing page

Layout matches the interview-selected preview:

```
+----------------------------------------------------------------+
|  Liveaboard                                                    |
|  ────                                                          |
|  Overview          ←—  active                                  |
|  Trips                                                         |
|  ────                                                          |
|  Maya Sanchez                                                  |
|  maya@example.com · Cruise Director                            |
+----------------------------------------------------------------+
|  +----------------------------+  +-------------------------+   |
|  | Maya Sanchez               |  | At a glance             |   |
|  |                            |  |                         |   |
|  | maya@example.com           |  | Upcoming   3            |   |
|  | +1 555 0142                |  | Active     1            |   |
|  | Acme Diving · Cruise Dir.  |  | Past       8            |   |
|  |                            |  |                         |   |
|  | [ Edit profile ]           |  +-------------------------+   |
|  +----------------------------+                                |
|                                                                |
|  My trips                                                      |
|  ──────────────────────────────────────────────────────────    |
|  STATUS    BOAT          ITINERARY              DATES          |
|  Active    Gaia Love     Komodo North           May 14-21      |
|  Planned   Seahorse      Raja Ampat             Jun 02-09      |
|  Planned   Gaia Love     Komodo                 Jul 11-18      |
|  Past      Gaia Love     Banda Sea              Apr 02-13      |
+----------------------------------------------------------------+
```

The page is rendered by `Overview.tsx` for non-admin sessions (the
existing fork point), backed by a single endpoint (`GET
/api/admin/cruise-director-overview`) so counts and rows can never
disagree.

### `/admin/account` — new "My profile" section

```
+-----------------------------+
|  My profile                 |
|                             |
|  Full name [Maya Sanchez]   |
|  Phone     [+1 555 0142]    |
|  Email      maya@x.test     |  ← read-only; change via section below
|                             |
|              [ Save ]       |
+-----------------------------+
```

Sits above the existing **Change password** and **Change email**
sections (Sprint 009). Backed by `PATCH /api/account/profile` —
`{full_name, phone}`. Available to any authenticated user.

### Admin "Add Cruise Director" modal

Replaces the current single-`email` modal in `/admin/users`:

```
Full name        [_________________________]   (required)
Email            [_________________________]   (required)
Phone (optional) [_________________________]
Role             ( • Cruise Director )         (single option, fixed)

[ Cancel ]   [ Send invitation ]
```

The role row is a deliberate single fixed option — visible affordance
for when more roles arrive, costs nothing today.

## API Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/invitations` | RequireOrgAdmin | Create invitation. Body: `{full_name, email, phone?, role?}`. `role` defaults to `cruise_director`. **`full_name` required.** |
| GET | `/api/invitations` | RequireOrgAdmin | List pending invitations. Items now include `full_name`, `phone`. |
| GET | `/api/invitations/lookup?token=` | none | Public lookup. Response gains `full_name`. **Phone is *not* exposed.** |
| POST | `/api/invitations/accept` | none | Consume token. **Body shrinks to `{token, password}`.** User row inherits `full_name` and `phone` from the invitation. |
| POST | `/api/invitations/{id}/resend` | RequireOrgAdmin | Unchanged from Sprint 009. |
| DELETE | `/api/invitations/{id}` | RequireOrgAdmin | Unchanged from Sprint 009. |
| GET | `/api/admin/users` | RequireOrgAdmin | List users. Items gain `phone`. |
| GET | `/api/admin/overview` | RequireOrgAdmin | **Counts key renamed: `cruise_directors`.** |
| PATCH | `/api/admin/trips/{id}` | RequireOrgAdmin | **Body key renamed: `cruise_director_user_id`.** |
| GET | `/api/admin/trips` | RequireSession | **Trip rows: keys renamed.** Director scoping unchanged (admin sees all org trips; cruise director sees only their own). |
| GET | `/api/admin/boats/{id}/trips` | RequireOrgAdmin | Same key renames. |
| GET | `/api/admin/cruise-director-overview` | RequireSession + role check | **NEW.** Returns `{profile, stats, trips}`. **403 for `org_admin`** — admins use `/api/admin/overview`. |
| PATCH | `/api/account/profile` | RequireSession | **NEW.** `{full_name, phone}` — updates own user record. |

### `cruise-director-overview` response shape

```json
{
  "profile": {
    "id": "uuid",
    "full_name": "Maya Sanchez",
    "email": "maya@x.test",
    "phone": "+1 555 0142",
    "role": "cruise_director",
    "organization_name": "Acme Diving"
  },
  "stats": {
    "upcoming": 3,
    "active": 1,
    "past": 8
  },
  "trips": [
    {
      "id": "uuid",
      "boat_name": "Gaia Love",
      "itinerary": "Komodo North",
      "start_date": "2026-05-14",
      "end_date": "2026-05-21",
      "status": "active"
    }
  ]
}
```

Trips are sorted by `start_date` ascending. The endpoint returns at
most ~50 trips for the rendered table; pagination is a follow-up. The
list is implicitly grouped by status in the UI (active first, then
upcoming, then past) — the API returns them sorted; the UI handles
visual grouping.

## Implementation Plan

### Phase 1: Schema rename + role rename + invitation metadata (~20%)

**Files:**
- `internal/store/migrations/0008_cruise_director_rename_and_profile.sql` — Create, with the SQL above.
- `internal/store/users.go` — `RoleSiteDirector` → `RoleCruiseDirector`; add `Phone *string` to `User`; update every `scanUser` call site; add `UpdateUserProfile(userID, fullName, phone)` helper.
- `internal/store/trips.go` — `SiteDirectorUserID` → `CruiseDirectorUserID`; rename `AssignSiteDirector` → `AssignCruiseDirector`; rename `TripsForUser` (already director-scoped — keep semantics).
- `internal/store/invitations.go` — add `FullName` + `Phone` to `Invitation`; update `CreateInvitation`, `RotateInvitationToken`, `InvitationByToken`, `PendingInvitationsForOrg` SQL.
- `internal/testdb/testdb.go` — `SeedSiteDirector` → `SeedCruiseDirector`; helper now also accepts an optional `phone`.

**Tasks:**
- [ ] Migration `0008` with the SQL above (single transaction).
- [ ] Rename role constant + every `RoleSiteDirector` reference in `internal/`.
- [ ] Add `phone` field to `User`; touch all `userColumns` / `scanUser` sites.
- [ ] Rename `trips.site_director_user_id` reads / writes / fields / methods. Compile pass.
- [ ] Update `Invitation` struct + queries to include name + phone.
- [ ] `internal/store/store_test.go` and store-level tests still pass under the new column name.

### Phase 2: Invitation service + email templates + accept-flow change (~15%)

**Files:**
- `internal/auth/invitations.go` — `Invite(orgID, inviterID, email, fullName, phone, role)` — new signature. `AcceptInvitation(token, password)` — drop `fullName` parameter (now read from the invitation row). `LookupInvitation(token)` — return `full_name` in `InvitationView`.
- `internal/email/email.go` — `Vars` gains `RecipientName`.
- `internal/email/templates/invitation.{txt,html,subject}.tmpl` — greet by name; mention inviter and Cruise Director role.
- `internal/auth/auth.go` — `Service` knobs unchanged; small adjustments only if needed.

**Tasks:**
- [ ] Service signatures updated.
- [ ] Email templates use `{{.RecipientName}}` salutation: `Hi {{.RecipientName}},` / `{{.InviterName}} invited you to join {{.OrganizationName}} as Cruise Director.`
- [ ] Lookup returns `full_name` so the SPA can pre-render the greeting on the accept page.
- [ ] Existing tests rewritten: invite passes name; accept consumes token without name input; lookup returns name; email rendering test asserts `RecipientName` appears in both text and HTML bodies.

### Phase 3: HTTP routes + handlers (~15%)

**Files:**
- `internal/httpapi/invitation_handlers.go` — request struct grows `full_name` + `phone`; accept request shrinks to `{token, password}`; lookup response gains `full_name`.
- `internal/httpapi/admin.go` — rename `HandleAssignDirector` → `HandleAssignCruiseDirector`; rename JSON keys; add `phone` to user rows; update `Overview` count key (`cruise_directors`).
- `internal/httpapi/auth_handlers.go` — new `handleUpdateProfile` behind `RequireSession`. Body: `{full_name, phone}`.
- `internal/httpapi/cruise_director.go` — NEW. Houses `HandleCruiseDirectorOverview`. Returns 403 if the caller is an `org_admin`; returns 403 if any non-`cruise_director` role surfaces in future.
- `internal/httpapi/httpapi.go` — mount the new endpoints under the existing `/api/admin/*` and `/api/account/*` groups.

**Tasks:**
- [ ] Wire endpoints; rename JSON keys at `Trip` views.
- [ ] `Overview` counts JSON renamed.
- [ ] `GET /api/admin/cruise-director-overview` mounted behind `RequireSession` + an explicit role gate (not the existing `RequireOrgAdmin`).
- [ ] `PATCH /api/account/profile` updates `users.full_name` + `users.phone` only — explicitly *not* email or role.

### Phase 4: Backend tests (~10%)

**Files:**
- `internal/httpapi/admin_test.go` — every reference to `site_director` swaps; `bootstrapDirector` helper renames; `HandleAssignDirector` test renames.
- `internal/httpapi/httpapi_test.go` — `signupAndVerify` flow unchanged; invitation accept-flow tests update for new shape (no name in accept body).
- `internal/store/organizations_test.go` — references to `RoleSiteDirector` rename.
- `internal/httpapi/cruise_director_test.go` — NEW. Covers `/api/admin/cruise-director-overview`.
- `internal/httpapi/profile_test.go` — NEW. Covers `PATCH /api/account/profile`.
- `internal/email/email_test.go` — assert `RecipientName` rendering.

**Tasks:**
- [ ] All existing test names + variable names updated.
- [ ] New test: invite with `full_name` + `phone` → accept → user row has both.
- [ ] New test: invite without `full_name` → 400 `invalid_input`.
- [ ] New test: cruise-director-overview as a Cruise Director → 200 with profile + stats + scoped trips.
- [ ] New test: cruise-director-overview as an Org Admin → 403.
- [ ] New test: cruise-director-overview unauthenticated → 401.
- [ ] New test: `PATCH /api/account/profile` updates the calling user's row only; cannot mutate other users.
- [ ] New test: `PATCH /api/account/profile` rejects `email` and `role` keys (DisallowUnknownFields).
- [ ] All tests green.

### Phase 5: Frontend rename + types + invite/accept rewrites (~15%)

**Files:**
- `web/src/admin/api.ts` — rename JSON keys + types (`site_director_user_id` → `cruise_director_user_id`, etc.). `role: "org_admin" | "cruise_director"`. Add `cruiseDirectorOverview()` and `updateProfile()` methods.
- `web/src/admin/useMe.tsx` — same role union rename; `Me` type gains `phone`.
- `web/src/lib/api.ts` — `Invitation` and `InvitationLookup` types gain `full_name` and `phone` (optional). `acceptInvitation(token, password)` — drop the `fullName` parameter.
- `web/src/admin/Shell.tsx` — sidebar copy + footer role label.
- `web/src/admin/pages/Trips.tsx` — column header + cell copy.
- `web/src/admin/pages/BoatTabs.tsx` — same.
- `web/src/admin/pages/Users.tsx` — invite modal grows `full_name` (required) + `phone` (optional); pending list renders new fields.
- `web/src/pages/AcceptInvitation.tsx` — stop asking for name; greet by name from lookup; password-only form.
- DESIGN.md — no change. (The new modal/landing card use existing tokens.)

**Tasks:**
- [ ] Single-pass rename of types + copy across `web/src/`.
- [ ] Invite modal updated (3 fields, single fixed-role row, `Send invitation` button).
- [ ] Accept page redesigned: `Welcome, <Name>` + password-only form + submit.
- [ ] `npm --prefix web run build` clean; `tsc -b` happy.

### Phase 6: Cruise Director landing page + profile editor (~15%)

**Files:**
- `web/src/admin/pages/Overview.tsx` — replace `SiteDirectorOverview` stub with the real `CruiseDirectorLanding` component that calls `/api/admin/cruise-director-overview` and renders the contact card + counts + trips table.
- `web/src/admin/pages/Account.tsx` — add `MyProfile` section above the existing change-password / change-email blocks. Reads from `useMe()` for initial values; `updateProfile()` and refreshes `useMe` so the sidebar role label updates live.
- `web/src/styles/app.css` — small additions: contact card, at-a-glance card, profile form. Token-driven; no new colors.

**Tasks:**
- [ ] Landing page renders against real data.
- [ ] Profile edit POSTs and refreshes `useMe` so the sidebar updates live.
- [ ] Empty state for "no assigned trips yet" — hint says an admin will assign.
- [ ] Sidebar role label updates immediately after profile save (re-fetch `me`).

### Phase 7: Persona / docs / smoke (~10%)

**Files:**
- `docs/product/personas.md` — heading + every body mention of "Site Director" → "Cruise Director". Add `phone` to the "Owns" list (a Cruise Director's contact info follows them across orgs / trips).
- `docs/product/organization-admin-user-stories.md` — story copy mentioning Site Director updates. Story IDs unchanged.
- `docs/auth.md` — invitation table row updated (extra fields).
- `RUNNING.md` — "invite a Cruise Director" example.
- `CLAUDE.md` — no change required (it doesn't mention the role by name).

**Tasks:**
- [ ] Docs updated.
- [ ] **Live smoke** against Brevo:
      1. Admin invites a real address with name + phone.
      2. Email arrives addressed to that name; click link.
      3. Accept page greets by name; set password; land on `/admin`.
      4. Landing renders contact card + zero trips empty state.
      5. Admin assigns the new Cruise Director to a trip via PATCH (curl is fine — no admin UI yet); reload the landing — trip appears in `My trips` table.
      6. Profile edit (changing phone) sticks in the contact card + the admin's `/admin/users` table.
- [ ] Final QA: `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm --prefix web run build` all clean.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-010.md` | Create | This sprint doc. |
| `docs/sprints/drafts/SPRINT-010-*.md` | Create | Planning artifacts (intent, drafts, critique, merge notes). |
| `docs/product/personas.md` | Modify | Rename + phone field. |
| `docs/product/organization-admin-user-stories.md` | Modify | Rename in story copy. |
| `docs/auth.md` | Modify | Invitation request shape. |
| `RUNNING.md` | Modify | Cruise Director invite example. |
| `internal/store/migrations/0008_cruise_director_rename_and_profile.sql` | Create | Schema rename + new columns + revoke legacy invitations. |
| `internal/store/users.go` | Modify | Role rename, `Phone` field, `UpdateUserProfile`. |
| `internal/store/trips.go` | Modify | Column rename, method rename, JSON-shape rename. |
| `internal/store/invitations.go` | Modify | New `FullName` / `Phone` fields. |
| `internal/store/store_test.go`, `organizations_test.go` | Modify | Rename helpers + assertions. |
| `internal/testdb/testdb.go` | Modify | `SeedCruiseDirector`. |
| `internal/auth/invitations.go` | Modify | Service signatures. |
| `internal/auth/auth.go` | Modify | Small if any role string references. |
| `internal/email/email.go` | Modify | `Vars` adds `RecipientName`. |
| `internal/email/templates/invitation.{txt,html,subject}.tmpl` | Modify | Greet by name. |
| `internal/email/email_test.go` | Modify | Assert `RecipientName` rendering. |
| `internal/httpapi/admin.go` | Modify | Rename handler + JSON keys + counts. |
| `internal/httpapi/invitation_handlers.go` | Modify | New invite + accept shapes. |
| `internal/httpapi/auth_handlers.go` | Modify | Add `handleUpdateProfile`. |
| `internal/httpapi/cruise_director.go` | Create | Cruise-director-overview endpoint. |
| `internal/httpapi/cruise_director_test.go` | Create | Overview endpoint tests. |
| `internal/httpapi/profile_test.go` | Create | Profile-patch tests. |
| `internal/httpapi/httpapi.go` | Modify | Mount new routes. |
| `internal/httpapi/admin_test.go` | Modify | Rename helpers + cruise_director assertions. |
| `internal/httpapi/httpapi_test.go` | Modify | New invitation flow shape. |
| `web/src/admin/api.ts` | Modify | Type + JSON key renames; new methods. |
| `web/src/admin/useMe.tsx` | Modify | Role union rename + `phone`. |
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

- [ ] Migration `0008` applies cleanly to a Sprint 009-state DB and to a fresh DB. Both `users_role_check` and `invitations_role_check` are recreated with `cruise_director`.
- [ ] **Pre-Sprint-010 pending invitations are revoked by the migration.** A `SELECT count(*) FROM invitations WHERE accepted_at IS NULL AND revoked_at IS NULL` on a migrated DB returns the same count as the post-migration row count of brand-new invitations (i.e., zero pending immediately after migration).
- [ ] No `site_director` / `Site Director` strings remain in `internal/`, `web/src/`, `cmd/`, `scripts/`, or `docs/product/`. Historical references in `docs/sprints/SPRINT-00[1-9].md` stay frozen as archive.
- [ ] **Backend tests pass.** New tests cover:
  - Invite with `full_name` + `phone` seeds them on accept.
  - Invite without `full_name` returns 400.
  - Accept consumes token without a name input.
  - Lookup returns `full_name`.
  - Lookup does **not** return `phone`.
  - Email template renders `{{.RecipientName}}` in both text and HTML.
  - `/api/admin/cruise-director-overview` returns the right payload shape for a Cruise Director and 403 for an Org Admin.
  - `PATCH /api/account/profile` updates name + phone only; rejects unknown fields.
- [ ] **Frontend tests / build clean.** SPA build is green.
- [ ] **Live smoke** through Brevo:
  - Invite a real address with full name + phone; email arrives with name in greeting; accept; land on `/admin`; see the contact card.
  - Admin assigns the new Cruise Director to a trip (via PATCH to `/api/admin/trips/{id}`); reload `/admin` (as the Cruise Director); the trip is in the My Trips table; the count chip is correct.
  - Profile edit (changing phone) reflects in the contact card + the admin's `/admin/users` table.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm --prefix web run build` all clean.
- [ ] `docs/product/personas.md` and `docs/product/organization-admin-user-stories.md` are post-rename.
- [ ] `tracker.tsv` registers Sprint 010 as completed via `go run docs/sprints/tracker.go complete 010`.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Rename misses a string and a test fails post-merge | Medium | Low | Final pass: `git grep -i 'site.director\|sitedirector\|site_director'` returns zero hits except in `docs/sprints/SPRINT-00[1-9].md`. |
| Migration fails on a pre-Sprint-010 DB because both role checks aren't dropped | High → mitigated | High | Migration explicitly drops both `users_role_check` and `invitations_role_check` before the data rewrite. |
| Pending invitations from a Sprint 009 dev DB become broken (blank-name acceptance edge case) | High → mitigated | Medium | Migration revokes all pre-Sprint-010 pending invitations. Admins re-invite with the new modal. |
| Renaming `trips.site_director_user_id` breaks an unseen consumer | Low | Low | The column is referenced only inside `internal/store/trips.go` and `internal/httpapi/admin.go`; both are updated in Phases 1 + 3. SPA reads via the renamed JSON key. |
| Existing Sprint 008 trip-list scoping for non-admins regresses | Low | High | Phase 1 is the schema/store rename pass; existing scope tests run after Phase 1 and after Phase 4. |
| Accept page's "name from invitation" surprises a user who wants to change their name | Low | Low | The contact card on the landing has an "Edit profile" link to `/admin/account`. The first thing they see post-accept includes that affordance. |
| Phone number storage opens "what about country code, validation?" rabbit hole | Medium | Low | Store as plain `text` (no validation, no normalization). E.164 normalization is a follow-up sprint. |
| Renaming JSON keys (`site_director_user_id` → `cruise_director_user_id`) breaks any external integration | Very low | Low | No external consumers exist (pre-customer); SPA is the only client. |
| Live Brevo smoke gets throttled mid-test | Low | Low | Same throttle envelope as Sprint 009; reuse `mr.karim.fanous@gmail.com`. |
| Cruise-director-overview drift between profile/counts/trips | Low (mitigated) | Low | Single-payload endpoint — counts and rows come from the same query result. |

## Security Considerations

- **Invitation metadata leakage.** `full_name` and `phone` come from the admin who invited the prospect. The lookup endpoint exposes `full_name` (so the SPA can greet by name) but **not** `phone`. Phone is only visible to admins of the same org and to the invitee themselves once they accept.
- **Profile editing surface.** `PATCH /api/account/profile` updates only `full_name` + `phone` on the calling user's row. Email is untouched (existing change-email flow handles that). Role is untouched. No tenancy fields are mutable. Other users' rows are inaccessible.
- **Multi-tenant isolation.** All queries that touch the new fields scope by `organization_id` (admin list, admin overview) or by `user_id` from the session (profile, landing).
- **Role boundary on the new endpoint.** `/api/admin/cruise-director-overview` is **`cruise_director`-only**. Org Admins receive 403. This avoids a fuzzy two-persona shape.
- **Migration ordering.** Both role checks are dropped *before* the row rewrite; recreated *after*; all in one transaction so partial state cannot persist.
- **Existing security boundaries unchanged.** RBAC middleware, `RequireOrgAdmin`, server-side trip scoping, session cookie semantics all unchanged.

## Dependencies

- **Sprint 008** (admin chrome, RBAC, server-scoped trip list) — built upon.
- **Sprint 009** (custom auth + Brevo invitations) — built upon.
- No new Go module deps; no new npm deps.
- No external services beyond Brevo SMTP (already in `LIVEABOARD_SMTP_*`).

## Out of Scope (captured as follow-ups)

- Phone-number validation / normalization (E.164).
- Per-user notification preferences.
- Cruise Director self-deactivation or "leave organization" flow.
- Cross-org Cruise Director (a director who works for multiple operators) — single-org per user remains the rule.
- Trip-detail page and the assignment UX itself (the PATCH endpoint exists; UI to drive it is a separate sprint).
- Custom roles / multi-admin / role administration UI.
- Audit log of invitation events.
- Admin editing other users' contact info (today they can resend or revoke invitations only).
- Email template polish (logo, footer disclaimer, branded sender domain).

## References

- Sprint 002 — `docs/sprints/SPRINT-002.md` (original "Site Director" backlog).
- Sprint 007 — `docs/sprints/SPRINT-007.md` (Control Tower IA recommendation; Site Director UX explicitly deferred).
- Sprint 008 — `docs/sprints/SPRINT-008.md` (RBAC + stub landing + the trip-scope endpoint we lean on).
- Sprint 009 — `docs/sprints/SPRINT-009.md` (custom invitation flow we extend).
- Personas — `docs/product/personas.md`.
- Stories — `docs/product/organization-admin-user-stories.md`.
- Design — `DESIGN.md`.
- Codex critique — `docs/sprints/drafts/SPRINT-010-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-010-MERGE-NOTES.md`.
