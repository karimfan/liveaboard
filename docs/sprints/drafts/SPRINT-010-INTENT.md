# Sprint 010 Intent: User Management — Cruise Director Rename, Rich Invitations, Director Landing Page

## Seed

> we will now work on the user management of the site. There are two
> types of users: Admins and Site Directors. First, we will change teh
> Site Director to Cruise Director. Admins can add Cruise Directors to
> their org. When they do so they enter meta data about them: name,
> email, contact info..etc and then an invitation email is sent to the
> user. The user than registers and is then able to access the site as
> a Cruise Director role. This role allows them to only view Trips they
> are assigned to. You will also need to design the IA for Site
> Directors and the initial look and feel of their landing page. I
> suggest showing trips they are assigned to amongst any other meta
> data relevant. We can change that and invest in it later on.

## Context

Sprint 008 already shipped a Site Director persona end-to-end (role
constant, RBAC middleware, sidebar role gating, server-side trip
scoping, an `assign director` PATCH endpoint, a stub landing page).
Sprint 009 replaced Clerk with a self-hosted invitation flow that
captures only `{email, role}` and asks the invitee for `full_name +
password` at accept time.

Sprint 010 evolves this stack on three axes:

1. **Rename:** "Site Director" becomes "Cruise Director" everywhere —
   role string, DB column on trips, JSON shapes, frontend copy, persona
   doc, user-story doc.
2. **Richer invitation:** the admin captures the prospective Cruise
   Director's full name (and optional phone) at invite-time. The
   invitee's accept page shows that name pre-filled (read-only display
   — they still set their own password).
3. **Cruise Director landing page:** flesh out the stub at `/admin`
   (Overview route) for non-admins into a real "My Trips" surface
   (trips assigned to me, sorted by date, with a header band that
   surfaces the user's contact card and trip totals).

## Recent Sprint Context

- **Sprint 007** — produced three IA candidates and recommended Option
  A "Control Tower" for the Org Admin. Site Director UX was explicitly
  deferred. The Cruise Director landing in this sprint is the first
  concrete take on that deferred surface.
- **Sprint 008** — implemented Control Tower with real data and RBAC.
  Sidebar already filters role-only items; `useMe()` exposes role; the
  Trips endpoint scopes server-side for non-admins. The "My Trips"
  list surface largely already exists at `/admin/trips`.
- **Sprint 009** — replaced Clerk with custom auth + Brevo SMTP.
  Invitation tokens are 7-day per-org-email-pending-unique; accept page
  collects full_name + password. Sprint 010 extends the metadata and
  the email body, not the wire-level token mechanics.

## Relevant Codebase Areas

| Area | Files |
|---|---|
| Role constant | `internal/store/users.go` (`RoleSiteDirector`) |
| DB CHECK | `internal/store/migrations/0007_replace_clerk_with_custom_auth.sql` (invitations.role) |
| Trip column | `internal/store/migrations/0006_trip_site_director.sql` (`trips.site_director_user_id`) |
| Invitation service | `internal/auth/invitations.go` (`Invite`, `AcceptInvitation`, `LookupInvitation`) |
| Invitation store | `internal/store/invitations.go` (`Invitation` struct + queries) |
| Admin handlers | `internal/httpapi/admin.go` (`HandleAssignDirector`, `HandleListUsers`, `Overview` counts) |
| Auth handlers | `internal/httpapi/invitation_handlers.go` |
| Email templates | `internal/email/templates/invitation.{subject,txt,html}.tmpl` |
| Frontend types | `web/src/admin/api.ts`, `web/src/admin/useMe.tsx`, `web/src/lib/api.ts` |
| Sidebar | `web/src/admin/Shell.tsx` |
| Stub landing | `web/src/admin/pages/Overview.tsx` (`SiteDirectorOverview`) |
| Trips list | `web/src/admin/pages/Trips.tsx` |
| Users page | `web/src/admin/pages/Users.tsx` (Invite modal — `email` only today) |
| Persona doc | `docs/product/personas.md` |
| User stories | `docs/product/organization-admin-user-stories.md` |
| Test seed | `internal/testdb/testdb.go` (`SeedSiteDirector`) |

## Constraints

- Follow CLAUDE.md (Go stdlib + minimal deps, gofmt + vet + tests
  green per commit, work directly on `main`, focused commits, no PRs
  unless asked).
- Match `DESIGN.md` tokens (warm slate background, amber CTA, General
  Sans display + DM Sans body + Geist mono numerics, comfortable
  density, text-only nav labels).
- Preserve self-hosted auth (Sprint 009) — invitations remain
  token-bearing email links delivered via Brevo SMTP.
- Preserve multi-tenant isolation: every Cruise Director list / trip
  query stays org-scoped at the SQL layer.
- Preserve the security boundary: API enforces RBAC; UI hides nav
  items as a UX nicety only.
- Do not regress Sprint 008 admin chrome or Sprint 009 auth tests.

## Success Criteria

- Backend rename is complete and atomic: `Cruise Director` is the
  canonical name in role constants, DB CHECK constraints, trip column
  name, JSON keys, and persona docs. No lingering "Site Director" copy
  except in historical sprint docs (which are archives).
- Migration applies cleanly on the existing dev DB and is idempotent
  enough that a Sprint 008-era test DB can advance to Sprint 010's
  schema in a single `make migrate`.
- Admin can open `/admin/users → + Invite Cruise Director`, fill
  name + email + optional phone, submit, and the invitee receives an
  email addressed to them by name. The pending invitation list shows
  the same metadata.
- Invitee clicks the link, sees `Welcome <Name>` (read-only), sets
  password, lands at `/admin` (their landing page). Their `users` row
  carries the name + phone the admin entered — they can edit later.
- Cruise Director's landing page at `/admin` shows the trips assigned
  to them — boat, itinerary, dates, status — sorted by start date.
  Their contact card (name, email, phone, role) sits above. The
  sidebar shows Overview + Trips only (already true post-Sprint-008).
- All existing admin and auth tests pass post-rename. New tests cover:
  invitation with extra metadata seeds the user; invitation HTML email
  greets by name; Cruise Director landing API returns scoped trips +
  user contact card.
- `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm --prefix web run
  build` all clean. Live smoke against Brevo: invite a real address,
  receive the email by name, accept, land on the new dashboard, see
  zero or many assigned trips.

## Open Questions

1. **Phone-number storage:** add `phone` to `users` directly, or to a
   separate `user_contact_info` table with `(label, value)` pairs?
   Phone is the only real "etc" the seed mentions; over-engineering
   contact info now costs us churn later if requirements change.
2. **Mandatory vs optional fields at invite-time:** name is clearly
   required; phone is clearly optional. Should the admin also be able
   to set a *display name* / preferred name distinct from full name,
   or is one name enough at MVP?
3. **Rename strategy for `trips.site_director_user_id`:** rename the
   column (cleaner; one migration; touches every query), or keep the
   column name and only rename in code (cheaper; mismatch between SQL
   and Go names)? The former is the right answer for a pre-customer
   product.
4. **Cruise Director landing page scope:** strictly "trips list" + a
   contact-card header (the seed's "we can invest later" framing), or
   layer in a "current/upcoming/past" tab grouping plus a single
   prominent "next trip" card? Decision affects how much UI real
   estate this sprint must paint.
5. **Editing the Cruise Director's own contact info:** does the
   Cruise Director themselves get a "My profile" page where they edit
   phone/name, or is that admin-only at MVP? (Probably they should be
   able to edit their own — change-password already lives at
   `/admin/account`.)
6. **Email delivery wording:** the invitation email currently uses a
   generic "you've been invited" tone. Should it explicitly
   "introduce" the Cruise Director to the org by name and contact, or
   stay personal-to-recipient?

## Out-of-scope follow-ups

- Multi-admin / custom roles.
- Cruise Director self-deactivation, "leave organization" flow.
- Trip-level chat or messaging between admin and director.
- Notification system (push / SMS) for new trip assignment.
- Analytics on director utilization.
- Revisiting the trip detail page itself (still a list-row drill-down
  in Sprint 008's chrome).
