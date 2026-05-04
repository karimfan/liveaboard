# Sprint 010 Codex Draft: User Management — Cruise Director Rename, Rich Invitations, Director Landing Page

## Overview

Sprint 010 should be treated as an extension of Sprint 008 and Sprint
009, not as a fresh user-management rewrite. The admin shell, RBAC
pattern, self-hosted auth service, and invitation-token flow already
exist. This sprint makes that stack production-shaped for the second
persona by doing three tightly related things together:

1. Rename the shipped `site_director` role and surrounding product copy
   to `cruise_director`.
2. Upgrade invitations from `{email, role}` to a richer staff-onboarding
   flow where the admin captures the invitee's name and optional phone
   before the email is sent.
3. Replace the placeholder non-admin `/admin` screen with a real Cruise
   Director landing page that shows their assigned trips plus their own
   contact card.

The key planning principle is consistency. This sprint should not leave
`site_director` lingering in SQL column names, role strings, JSON keys,
UI labels, or docs just because that is cheaper short-term. The product
is still pre-customer, and the current stack is small enough that a
clean rename now avoids permanent translation layers later.

This draft resolves the open questions in the intent as follows:

- **Canonical role/name:** `cruise_director` becomes the only current
  role string for that persona.
- **Contact info modeling:** add `phone` directly to `users` and store
  invite-time `full_name` + `phone` directly on `invitations`. A generic
  contact-info table is premature.
- **Trip assignment column:** rename `trips.site_director_user_id` to
  `trips.cruise_director_user_id` in a forward migration.
- **Landing page scope:** keep it intentionally narrow for this sprint:
  contact card, summary counts, chronologically sorted assigned trips,
  and a clear empty state.
- **Profile editing:** add a minimal self-service profile section to
  `/admin/account` so the invite-time metadata is editable later.

## Use Cases

1. **Invite a Cruise Director**: Org Admin opens `/admin/users`, clicks
   `+ Invite Cruise Director`, enters full name, email, and optional
   phone, and sends an invitation.
2. **Review pending invitations**: Org Admin sees pending invites with
   the same captured metadata and can resend or revoke them.
3. **Accept invitation with prefilled identity**: Invitee opens the
   tokenized link, sees the organization name, their email, and their
   name presented as already known, sets a password, and joins as a
   `cruise_director`.
4. **Cruise Director lands on a useful dashboard**: After acceptance or
   login, the Cruise Director lands on `/admin` and immediately sees who
   they are, how to contact them, and which trips they own.
5. **Cruise Director sees only assigned work**: `/admin/trips` and the
   landing page show only trips assigned to that user, server-scoped by
   org + user id.
6. **Cruise Director updates profile details later**: The user can edit
   their own name and phone from `/admin/account` without involving an
   admin.
7. **Org Admin continues to assign leadership cleanly**: Existing trip
   assignment flows keep working, but now target Cruise Directors and
   surface the renamed field and copy.

## Architecture

### Scope boundaries

- Keep Sprint 009's auth mechanics: opaque session cookie, invitation
  token lookup/accept, Brevo SMTP delivery.
- Keep Sprint 008's admin IA shape: same shell, same routes, same role
  gating pattern.
- Add one new schema migration for the rename + metadata expansion.
- Avoid introducing new role types, multi-admin management, or a broad
  staff directory beyond what the sprint needs.

### Data model

This sprint should add a new migration `0008_cruise_director_rename_and_profile.sql`
that performs four related schema updates together:

1. Rename trip assignment column:
   - `trips.site_director_user_id` -> `trips.cruise_director_user_id`
   - corresponding index name updated as well
2. Expand `users`:
   - add `phone text NULL`
   - update role check so `cruise_director` replaces `site_director`
3. Expand `invitations`:
   - role check becomes `cruise_director`
   - add `full_name text NOT NULL`
   - add `phone text NULL`
4. Migrate existing enum-like data:
   - existing `users.role = 'site_director'` rows become
     `'cruise_director'`
   - existing `invitations.role = 'site_director'` rows become
     `'cruise_director'`

Because the app is already on PostgreSQL and the current schema is
small, doing the rename in the database is cleaner than carrying a
permanent code alias.

### Invitation lifecycle

The invitation flow should evolve from:

```text
admin enters: email + role
invitee enters: full_name + password
```

to:

```text
admin enters: full_name + email + optional phone + role(cruise_director)
invitee sees: full_name (read-only), email, org, role
invitee enters: password only
user row created with stored invite metadata
```

That implies a service contract change:

- `auth.Service.Invite(...)` accepts `fullName`, `email`, `phone`, `role`
- `auth.Service.LookupInvitation(...)` returns `full_name` and `phone`
- `auth.Service.AcceptInvitation(...)` no longer asks the invitee for
  `full_name`; it creates the user from invitation metadata

The invitation row becomes the source of truth between send and accept.
This keeps the accept flow deterministic and preserves the admin-entered
 identity even if the invitee opens the link later on another device.

### Cruise Director landing API

The current `/api/admin/overview` handler is admin-specific and should
remain so. Overloading it with two unrelated payload shapes would make
the frontend and tests harder to reason about. Instead, add a dedicated
session-authenticated endpoint:

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/admin/cruise-director-overview` | `RequireSession` + role check | Return the signed-in Cruise Director's contact card, counts, and assigned trips. |

Recommended response shape:

```json
{
  "profile": {
    "id": "uuid",
    "full_name": "Maya Chen",
    "email": "maya@example.com",
    "phone": "+62 812 ...",
    "role": "cruise_director"
  },
  "stats": {
    "assigned_trips": 3,
    "upcoming_trips": 2,
    "active_trips": 0
  },
  "trips": [
    {
      "id": "uuid",
      "boat_name": "Gaia Love",
      "itinerary": "Komodo North",
      "start_date": "2026-06-02",
      "end_date": "2026-06-09",
      "status": "planned"
    }
  ]
}
```

The existing `/api/admin/trips` endpoint should keep returning the full
trip rows for both personas, but its internal field names and JSON keys
should change from `site_director_*` to `cruise_director_*`.

### Frontend composition

The frontend already has the right structural seams:

- `useMe()` establishes the signed-in role
- `Overview.tsx` already forks admin vs non-admin rendering
- `Users.tsx` already owns invite modal + pending invite list
- `Account.tsx` already exists post-Sprint-009 for password/email

Sprint 010 should extend those seams instead of adding parallel ones:

- `Overview.tsx`
  - org admin branch stays intact
  - Cruise Director branch fetches the new overview endpoint and renders
    a real dashboard surface
- `Users.tsx`
  - rename UI copy
  - invite modal collects `full_name`, `email`, `phone`
  - pending list shows name, email, phone, role, expiry
- `Trips.tsx`
  - rename displayed role fields and labels
- `Account.tsx`
  - add a profile form above security/email sections for full name +
    phone
- `Shell.tsx`
  - footer role copy changes to `cruise director`

### Landing page IA

The Cruise Director landing page should remain deliberately minimal and
consistent with `DESIGN.md`:

```text
+---------------------------------------------------------------+
| My Trips                                                      |
| Your assigned trips and contact details                       |
|                                                               |
| +---------------------------+  +---------------------------+  |
| | Maya Chen                 |  | Assigned trips           |  |
| | Cruise Director           |  | 3 total                  |  |
| | maya@example.com          |  | 2 upcoming               |  |
| | +62 812 ...               |  | 0 active                 |  |
| +---------------------------+  +---------------------------+  |
|                                                               |
| Assigned trips                                                 |
| Gaia Love   Komodo North   Jun 2-9   Planned                  |
| Seahorse    Raja Ampat     Jul 4-11  Planned                  |
+---------------------------------------------------------------+
```

Notes:

- no charts
- no map/calendar widget
- no admin metrics unrelated to the signed-in user's work
- empty state should explicitly tell the Cruise Director they have no
  assigned trips yet

## Implementation Plan

### Phase 1: Schema rename + data contract cleanup (~25%)

**Files:**
- `internal/store/migrations/0008_cruise_director_rename_and_profile.sql` - rename role/column and add phone + invitation metadata
- `internal/store/trips.go` - rename assignment field and related query scan/filters
- `internal/store/users.go` - add `Phone`, role rename, profile update helper
- `internal/store/invitations.go` - add `FullName` and `Phone` fields and query changes
- `internal/testdb/testdb.go` - replace site-director seed helper naming/role

**Tasks:**
- [ ] Add migration `0008` that renames `trips.site_director_user_id` to `cruise_director_user_id`.
- [ ] Rename the supporting trip-assignment index.
- [ ] Add `users.phone text null`.
- [ ] Add `invitations.full_name text not null` and `invitations.phone text null`.
- [ ] Update the `users.role` and `invitations.role` constraints to use `cruise_director`.
- [ ] Migrate existing row values from `site_director` to `cruise_director`.
- [ ] Rename store constants/helpers/test seeds to `CruiseDirector`.

### Phase 2: Backend invitation + profile services (~25%)

**Files:**
- `internal/auth/invitations.go` - richer invite payload, metadata lookup, acceptance from stored metadata
- `internal/httpapi/invitation_handlers.go` - request/response shape changes
- `internal/httpapi/auth_handlers.go` - ensure `/api/me` includes `phone` if needed
- `internal/store/users.go` - self-profile update helper
- `internal/auth/auth.go` or account service files - self-profile update service
- `internal/email/templates/invitation.subject.tmpl` - rename role/copy
- `internal/email/templates/invitation.txt.tmpl` - greet by name
- `internal/email/templates/invitation.html.tmpl` - greet by name and reflect cruise-director copy

**Tasks:**
- [ ] Change invite request validation to require `full_name`, require valid `email`, allow optional `phone`, and default role to `cruise_director`.
- [ ] Persist invite metadata on the invitation row.
- [ ] Change invitation email rendering to address the recipient by name.
- [ ] Update invitation lookup payload to include `full_name` and `phone`.
- [ ] Remove `full_name` from invitation acceptance input; accept page sets password only.
- [ ] Create invited user rows from invitation metadata and persist phone on `users`.
- [ ] Add a self-profile update path for `full_name` + `phone` under authenticated account routes.

### Phase 3: Admin/trip API rename + Cruise Director overview endpoint (~20%)

**Files:**
- `internal/httpapi/admin.go` - role rename, trip JSON rename, new overview handler
- `internal/httpapi/httpapi.go` - mount new cruise-director overview route
- `internal/store/trips.go` - shared query helpers for assigned-trip summary
- `internal/store/users.go` - current-user profile read support if needed

**Tasks:**
- [ ] Rename `site_director_user_id`/`site_director_name` JSON keys to `cruise_director_user_id`/`cruise_director_name`.
- [ ] Rename admin overview copy from "Invite a Site Director" to "Invite a Cruise Director".
- [ ] Add `GET /api/admin/cruise-director-overview`.
- [ ] Enforce that only `cruise_director` callers can read that endpoint.
- [ ] Return contact-card profile data plus assigned-trip stats plus chronologically sorted trips.
- [ ] Keep `/api/admin/trips` server-scoped for non-admins with the renamed role.

### Phase 4: Frontend user-management and landing-page UI (~20%)

**Files:**
- `web/src/lib/api.ts` - richer invitation/profile request types
- `web/src/admin/api.ts` - renamed trip fields + cruise-director overview call
- `web/src/admin/pages/Users.tsx` - new invite form and richer pending list
- `web/src/pages/AcceptInvitation.tsx` - read-only name/email + password-only acceptance
- `web/src/admin/pages/Overview.tsx` - real Cruise Director landing page
- `web/src/admin/pages/Trips.tsx` - role/field rename
- `web/src/admin/pages/Account.tsx` - profile section for full name + phone
- `web/src/admin/Shell.tsx` - footer role rename
- `web/src/admin/useMe.tsx` - expose phone if needed by profile/overview

**Tasks:**
- [ ] Add `full_name` and `phone` fields to the invite modal.
- [ ] Rename all visible "Site Director" copy to "Cruise Director".
- [ ] Show pending invitation metadata in the users screen.
- [ ] Update accept-invitation screen to present `Welcome {full_name}` and remove editable name input.
- [ ] Build the Cruise Director `/admin` landing page with profile card, counts, and assigned-trips list.
- [ ] Add profile editing UI for `full_name` + `phone` on `/admin/account`.
- [ ] Ensure empty/loading/error states fit the existing admin visual language.

### Phase 5: Test coverage, smoke validation, and doc updates (~10%)

**Files:**
- `internal/httpapi/admin_test.go` - renamed role expectations + new overview test cases
- `internal/httpapi/invitation_handlers_test.go` - richer invitation payload coverage
- `internal/auth/*_test.go` - invitation metadata + acceptance behavior
- `internal/email/email_test.go` - greeting-by-name assertions
- `docs/product/personas.md` - rename persona and boundaries
- `docs/product/organization-admin-user-stories.md` - rename stories and acceptance language

**Tasks:**
- [ ] Update test helpers and assertions from `site_director` to `cruise_director`.
- [ ] Add handler/service tests proving invite metadata persists through acceptance.
- [ ] Add email-render tests asserting name-aware invitation copy.
- [ ] Add endpoint tests for `GET /api/admin/cruise-director-overview`.
- [ ] Add tests for self-profile update authorization and persistence.
- [ ] Rename persona/product-doc references outside historical sprint archives.
- [ ] Run full backend/frontend validation plus a live Brevo smoke test.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/invitations` | `POST` | Create a Cruise Director invitation with `full_name`, `email`, `phone?`, `role`. |
| `/api/invitations` | `GET` | List pending invitations including `full_name`, `email`, `phone`, `role`, `expires_at`. |
| `/api/invitations/lookup?token=...` | `GET` | Return invitation identity metadata for the accept page. |
| `/api/invitations/accept` | `POST` | Accept invitation with `token` + `password`; create the user from stored metadata. |
| `/api/admin/trips` | `GET` | Same route as Sprint 008, but renamed Cruise Director fields in the response. |
| `/api/admin/trips/{id}` | `PATCH` | Assign/clear `cruise_director_user_id` on a trip. |
| `/api/admin/cruise-director-overview` | `GET` | Return the signed-in Cruise Director's profile, stats, and assigned trips. |
| `/api/account/profile` | `PATCH` | Update signed-in user's `full_name` and `phone`. |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-010.md` | Create later | Final merged sprint doc. |
| `internal/store/migrations/0008_cruise_director_rename_and_profile.sql` | Create | Schema rename and metadata expansion. |
| `internal/store/users.go` | Modify | Add phone + profile update helpers + role rename. |
| `internal/store/invitations.go` | Modify | Store full name and phone on invitations. |
| `internal/store/trips.go` | Modify | Rename assignment column/field and keep scoped trip queries. |
| `internal/auth/invitations.go` | Modify | Rich invite flow and metadata-based acceptance. |
| `internal/httpapi/invitation_handlers.go` | Modify | New request/response payloads. |
| `internal/httpapi/admin.go` | Modify | Renamed fields and new Cruise Director overview handler. |
| `internal/httpapi/httpapi.go` | Modify | Mount new account/profile and overview routes. |
| `internal/httpapi/admin_test.go` | Modify | Role rename and new overview coverage. |
| `internal/httpapi/invitation_handlers_test.go` | Create/Modify | Invitation metadata endpoint tests. |
| `internal/email/templates/invitation.*.tmpl` | Modify | Name-aware Cruise Director invitation copy. |
| `web/src/lib/api.ts` | Modify | Rich invite and acceptance request types. |
| `web/src/admin/api.ts` | Modify | Renamed trip fields and overview call. |
| `web/src/admin/pages/Users.tsx` | Modify | Rich invite modal + pending metadata table. |
| `web/src/pages/AcceptInvitation.tsx` | Modify | Password-only acceptance with read-only name/email. |
| `web/src/admin/pages/Overview.tsx` | Modify | Real Cruise Director landing page. |
| `web/src/admin/pages/Trips.tsx` | Modify | Cruise Director label/field rename. |
| `web/src/admin/pages/Account.tsx` | Modify | Add self-profile editing. |
| `docs/product/personas.md` | Modify | Rename persona and responsibility text. |
| `docs/product/organization-admin-user-stories.md` | Modify | Rename user-management/trip-assignment story language. |

## Definition of Done

- [ ] Migration `0008_cruise_director_rename_and_profile.sql` applies cleanly after Sprint 009 schema.
- [ ] No live code, API payload, schema constraint, or current product doc still uses `site_director`; only historical sprint docs retain the old term.
- [ ] Admin can invite a Cruise Director with name + email + optional phone from `/admin/users`.
- [ ] Pending invitations list shows name, email, phone, role, and expiry.
- [ ] Invitation emails greet the recipient by name and use Cruise Director wording.
- [ ] Accept-invitation flow uses stored metadata, asks only for password, logs the user in, and creates a `users` row with `full_name` + `phone`.
- [ ] Cruise Director landing page at `/admin` shows profile card, summary counts, and assigned trips in chronological order.
- [ ] `/api/admin/trips` and the landing endpoint remain server-scoped to the signed-in Cruise Director's org + user id.
- [ ] Signed-in users can update their own `full_name` and `phone` from `/admin/account`.
- [ ] Backend tests cover rename-sensitive paths, invitation metadata persistence, overview payload, and RBAC.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, and `npm --prefix web run build` pass.
- [ ] Manual Brevo smoke test verifies invite delivery, acceptance, and post-login landing behavior.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Partial rename leaves mixed `site_` and `cruise_` terms across JSON, DB, and UI | Medium | High | Centralize the rename in migration + constants first, then update all callers/tests in one pass. |
| Accept-invitation flow breaks because existing frontend still submits `full_name` | Medium | Medium | Update backend and frontend request/response contracts together and cover with handler tests. |
| Existing dev DBs fail on constraint updates while old values still exist | Medium | High | In migration `0008`, update row values before adding the tighter `cruise_director` checks. |
| Profile editing scope expands beyond MVP | Low | Medium | Keep account changes limited to `full_name` + `phone`; email/password remain on the existing Sprint 009 forms. |
| Landing page becomes over-designed or drifts from `DESIGN.md` | Medium | Medium | Keep layout card-and-table based, no new visual language, no speculative widgets. |

## Security Considerations

- All reads and mutations remain organization-scoped at the SQL layer.
- UI role gating remains a convenience only; API handlers must enforce
  admin-only and cruise-director-only access explicitly.
- Invitation acceptance still proves email possession through the token;
  removing the editable name field must not weaken token validation.
- Profile update endpoints must only mutate the signed-in user's own
  row, never arbitrary user ids from the client.
- Invitation list and users list remain admin-only.
- Renaming the role string must not accidentally widen access in
  `RequireOrgAdmin`, `RequireSession`, or route guards.

## Dependencies

- Depends on Sprint 008's admin shell, role-gated navigation, and
  server-scoped trip list.
- Depends on Sprint 009's custom auth service, invitations table, SMTP
  email delivery, and accept-invitation route.
- Must stay within `DESIGN.md` tokens and layout rules.

## Open Questions

1. Should admins also be able to edit a Cruise Director's phone/name
   after invitation acceptance from `/admin/users`, or is self-service
   account editing sufficient for this sprint?
2. Should the Cruise Director landing page show trip status chips only,
   or also surface a single "next departure" hero row if at least one
   upcoming trip exists?
3. Is phone formatting/normalization intentionally loose (`text` with
   UI hints) for MVP, or do we want lightweight canonicalization now?
4. Should invitation resend preserve the original expiry date for audit
   clarity, or continue Sprint 009's rotate-and-extend behavior?

## References

- `docs/sprints/SPRINT-008.md`
- `docs/sprints/SPRINT-009.md`
- `docs/sprints/drafts/SPRINT-010-INTENT.md`
- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `DESIGN.md`
