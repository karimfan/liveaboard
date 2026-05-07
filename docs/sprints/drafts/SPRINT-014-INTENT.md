# Sprint 014 Intent: Guest Management and Trip Registration

## Seed

kplan I now want you to build the guest management experience. We will
need to add guests to a trip. The admin or director can do that. Adding
a guest sends them a registration email. once they create an account
(just a pssword + email is needed) they will then register onto the
trip. See here for trip regiustration data - make sure you make this
applicable to non-Gaia and non-Indonesia trips:
https://divegaia.com/guest-registration/

## Context

Liveaboard is a multi-tenant SaaS platform for scuba diving liveaboard
operators. The backend is Go with PostgreSQL and chi; the frontend is
React/Vite. The current admin chrome supports organizations, boats,
trips, Cruise Director assignment, imports, user invitations, catalog,
inventory, FX, and checkout quote foundations.

The next planning direction is guest management: operators need a flat
trip manifest, can add guests to a trip, and guests should receive an
email link that lets them create a guest account and complete
trip-specific registration. The guest persona has been future-scoped so
far, but the data model already anticipates guest self-service. This
sprint should introduce only the account and registration surfaces
needed for trip onboarding, not guest folios, checkout, dive schedules,
or broad guest portal functionality.

The referenced Gaia Love registration page collects identity/passport,
travel logistics, emergency contact, dive insurance, dive experience,
dietary/allergy notes, and rental gear details. The implementation must
model these as generic liveaboard trip registration data. Avoid Gaia,
Indonesia, Raja Ampat, or destination-specific assumptions. Destination
or operator-specific permit fields should be represented as optional
generic travel/permit notes or deferred configurable custom fields, not
hard-coded schema.

## Recent Sprint Context

- Sprint 011 added the user menu, profile entry point, sign out, and
  sea-background design update. Frontend changes must continue to
  follow `DESIGN.md`: dense operational surfaces, slate/amber working
  UI, and no tourism-style layout.
- Sprint 012 added native trip import from liveaboard.com and
  spreadsheets. Trips now carry `num_guests` as expected/imported
  capacity data, but there is not yet a real manifest table.
- Sprint 013 is in progress for catalog, per-boat inventory,
  stock movements, manual FX rates, and checkout quote snapshots. It
  explicitly defers guest checkout UI, folio posting, payments,
  receipts, and taxes.

## Relevant Codebase Areas

- `internal/store/migrations` - add guest, trip guest, guest invite,
  registration, and file metadata tables.
- `internal/store/users.go` - current `users` table represents org
  staff (`org_admin`, `cruise_director`). Guest accounts likely need
  either a distinct table or a carefully expanded auth model.
- `internal/auth/invitations.go` and `internal/store/invitations.go` -
  staff invitation flow can inspire guest invitation token handling,
  but should not be overloaded if guest accounts are not org staff.
- `internal/email/templates` and `internal/email/templates.go` - add
  guest registration invitation email templates.
- `internal/httpapi/httpapi.go` - add admin/director guest-management
  routes and public token lookup/accept routes.
- `internal/httpapi/admin.go` and trip handlers - current trip listing
  supports org admins and Cruise Directors with different scopes.
- `web/src/admin/pages/Trips.tsx`, `web/src/admin/pages/BoatTabs.tsx`,
  and `web/src/admin/api.ts` - add manifest entry points and admin/
  director APIs.
- `web/src/pages` - add guest invite accept/account creation and
  registration wizard pages outside the admin chrome.
- `docs/product/personas.md` and
  `docs/product/organization-admin-user-stories.md` - update persona
  boundaries and backlog to include guest registration.

## Constraints

- Must follow project conventions in `CLAUDE.md` and `CODEX.md`.
- Must integrate with the existing Go/React architecture and sprint
  document conventions in `docs/sprints/README.md`.
- Must preserve strict tenant isolation. Every manifest and
  registration query must be scoped to `organization_id` and `trip_id`.
- Org Admin can add guests to planned trips as pre-departure manifest
  work. Cruise Directors can add guests to trips assigned to them,
  especially once a trip is active.
- Guest accounts must not gain org staff access or admin chrome access.
- A guest registration link should prove access to one trip invite;
  guest account creation should require only email + password where the
  email comes from the invite.
- Registration data must be generic across liveaboard operators and
  destinations. No hard-coded Gaia Love, Indonesia visa, or Raja Ampat
  permit fields.
- Avoid payment, checkout, receipts, guest folios, inventory depletion,
  and dive schedule scope in this sprint.
- File uploads for passport/liability/certification documents must be
  planned carefully. If implemented locally, store metadata and files in
  a development-safe local storage directory; design the interface so a
  cloud object-store backend can replace it later.
- Registration data contains sensitive personal, passport, health,
  emergency, and travel information. Include privacy, access-control,
  retention, and audit considerations.

## Success Criteria

- Admins and eligible Cruise Directors can add a guest to a trip from a
  trip/manifest view.
- Adding a guest creates a pending trip guest row and sends a
  registration email with a secure, expiring token.
- The guest can open the link, create a guest account with email +
  password, and land in a trip-specific registration flow.
- The guest can submit operator-neutral registration sections:
  identity, passport/travel document, arrival/departure logistics,
  emergency contact, dive insurance, dive profile, dietary/allergy
  notes, rental gear needs, and general notes.
- Admins and assigned Cruise Directors can view manifest status:
  invited, account created, registration incomplete, submitted.
- Manifest count respects trip/boat capacity rules.
- Guest users cannot access admin routes or data for other trips.
- Tests cover token lifecycle, guest account creation, duplicate email
  behavior, trip scoping, role authorization, registration validation,
  and status transitions.

## Open Questions

- Should guest account auth reuse the `users` table with a new `guest`
  role, or use a separate `guest_users` table to avoid org staff
  semantics bleeding into guest access?
- Should Sprint 014 implement actual file uploads, or create document
  metadata placeholders and defer binary storage to a storage sprint?
- Should Admins be allowed to add guests only while trips are planned,
  while Cruise Directors can add guests only on assigned planned/active
  trips, or should both roles be able to add guests in both states?
- Does the operator need invite resend/revoke flows in the first guest
  management sprint?
- Is one registration submission per guest/trip enough, or should
  guests be able to save drafts before final submit?
- Should the registration schema be fixed for MVP, or should the sprint
  introduce configurable per-org required fields immediately?
