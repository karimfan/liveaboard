# Sprint 014: Guest Management and Trip Registration

## Overview

Sprint 014 introduces the first guest-facing workflow in Liveaboard:
an Org Admin or assigned Cruise Director can add a guest to a trip,
send a secure registration link, and track whether that guest has
created a guest account and submitted trip registration details.

This sprint deliberately builds a trip-scoped registration foundation,
not a broad guest portal. Guests do not enter the admin chrome, do not
see org data, and do not interact with folios, payments, checkout,
inventory, or dive schedules. Registration data is modeled as generic
liveaboard onboarding information: identity, passport/travel document,
travel logistics, emergency contact, insurance, dive experience,
dietary/allergy notes, rental gear needs, and general notes. Gaia Love,
Indonesia, Raja Ampat, permit, and destination-specific assumptions are
not hard-coded.

The main architectural choice is to keep guest accounts separate from
staff `users`. The existing `users` table represents org staff and feeds
`/api/me`, `RequireOrgAdmin`, Cruise Director scoping, and admin chrome
authorization. Adding a `guest` role there would make every auth and RBAC
path more fragile. Instead, Sprint 014 adds `guest_users`,
`guest_sessions`, and trip-scoped tables that join guests to one
operator/trip at a time.

## Use Cases

1. **Add a guest to a planned trip**: Org Admin opens a trip manifest,
   enters guest name and email, and sends a registration invitation.
   The manifest count updates immediately without exceeding capacity.
2. **Add a guest to an assigned trip**: Cruise Director opens a trip
   assigned to them and adds a guest. This is allowed for assigned
   planned or active trips; completed/cancelled trips are read-only.
3. **Receive registration email**: Guest receives a link scoped to one
   trip invitation. The link expires, can be resent, and can be revoked.
4. **Create guest account from invite**: Guest opens the link, sees the
   operator/trip context, confirms the invite email, sets a password,
   and receives a guest session cookie. The email field is not editable.
5. **Reuse existing guest account**: A guest invited again with the same
   email can authenticate with their existing password through the
   token-bearing flow and link the new trip to the same guest account.
6. **Complete trip registration**: Guest submits identity, passport or
   travel-document details, arrival/departure logistics, emergency
   contact, dive insurance, dive profile, dietary/allergy notes, rental
   gear needs, and general notes.
7. **Save before submit**: Guest can save partial registration details
   as a draft, leave, and return through the invite link or active guest
   session before submitting.
8. **Track manifest readiness**: Admins and assigned Cruise Directors
   can see manifest rows with statuses: invited, account created,
   registration draft, submitted, revoked, and expired.
9. **Prevent cross-trip access**: Guest A cannot see another guest, a
   different trip, or admin routes. A Cruise Director cannot access a
   manifest for an unassigned trip.

## Architecture

### Core Rules

- `users` remains staff-only with roles `org_admin` and
  `cruise_director`.
- Guest identity lives in `guest_users`; guest sessions live in
  `guest_sessions` and use a separate `lb_guest_session` cookie.
- Every manifest table row includes `organization_id` and `trip_id`.
  Queries scope by both, even when a foreign key could imply one of
  them.
- `trip_guests` is the flat manifest row. There is no cabin model,
  berth assignment, folio, payment, receipt, or dive schedule in this
  sprint.
- A guest invitation proves access to exactly one `trip_guest` row.
- Invite tokens are opaque, hashed at rest, expire, and can be rotated
  on resend.
- Registration can be saved as draft and then submitted. Submitted data
  can still be edited by the guest until the trip is completed unless a
  later sprint adds locking/review semantics.
- File binary upload is deferred. The schema includes optional document
  metadata placeholders so the registration model has a place for
  passport, certification, liability, or insurance document references
  later without committing to local disk or cloud object storage now.

### Schema Migration `0012`

```sql
guest_users (
  id uuid primary key default gen_random_uuid(),
  email citext not null unique,
  password_hash bytea not null,
  email_verified_at timestamptz not null,
  is_active boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)

guest_sessions (
  id uuid primary key default gen_random_uuid(),
  guest_user_id uuid not null references guest_users(id) on delete cascade,
  token_hash bytea not null unique,
  created_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now(),
  expires_at timestamptz not null,
  revoked_at timestamptz null
)

trip_guests (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  guest_user_id uuid null references guest_users(id) on delete set null,
  invited_by_user_id uuid null references users(id) on delete set null,
  full_name text not null check (length(trim(full_name)) > 0),
  email citext not null,
  status text not null check (status in (
    'invited', 'account_created', 'registration_draft',
    'submitted', 'revoked', 'expired'
  )),
  invite_sent_at timestamptz null,
  account_created_at timestamptz null,
  registration_submitted_at timestamptz null,
  revoked_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (organization_id, trip_id, email)
)

guest_trip_invitations (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  email citext not null,
  token_hash bytea not null unique,
  expires_at timestamptz not null,
  accepted_at timestamptz null,
  revoked_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)

guest_trip_registrations (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  guest_user_id uuid not null references guest_users(id) on delete cascade,
  status text not null check (status in ('draft', 'submitted')),
  payload jsonb not null default '{}'::jsonb,
  submitted_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (trip_guest_id)
)

guest_document_metadata (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  guest_user_id uuid not null references guest_users(id) on delete cascade,
  document_type text not null,
  original_filename text not null,
  content_type text null,
  byte_size bigint null,
  storage_backend text not null default 'deferred',
  storage_key text null,
  uploaded_at timestamptz null,
  created_at timestamptz not null default now()
)
```

Indexes:

- `guest_sessions_guest_user_id_idx`, `guest_sessions_expires_at_idx`.
- `trip_guests_org_trip_idx` on `(organization_id, trip_id, status)`.
- `trip_guests_guest_user_idx` on `(guest_user_id)`.
- `guest_trip_invitations_trip_guest_pending_idx` partial unique on
  `(trip_guest_id)` where `accepted_at IS NULL AND revoked_at IS NULL`.
- `guest_trip_invitations_expires_idx` on `expires_at`.
- `guest_trip_registrations_org_trip_idx` on
  `(organization_id, trip_id, status)`.

`payload` is JSONB because the registration shape is large, partly
optional, and likely to evolve. The API still validates it against
typed Go structs; the database stores the server-approved document.
Future configurable per-org fields can append a `custom` section
without a schema rewrite.

### Registration Payload Contract

The API accepts and returns this sectioned shape:

```json
{
  "identity": {
    "legal_name": "Maya Sanchez",
    "preferred_name": "Maya",
    "date_of_birth": "1989-04-12",
    "nationality": "US",
    "phone": "+1 555 0100"
  },
  "travel_document": {
    "document_type": "passport",
    "document_number": "123456789",
    "issuing_country": "US",
    "expires_on": "2030-01-15"
  },
  "travel_logistics": {
    "arrival_airline": "UA",
    "arrival_flight_number": "UA123",
    "arrival_at": "2026-08-10T14:30:00Z",
    "arrival_location": "Airport or hotel",
    "departure_airline": "UA",
    "departure_flight_number": "UA456",
    "departure_at": "2026-08-18T09:00:00Z",
    "departure_location": "Airport or hotel",
    "hotel_before_trip": "",
    "hotel_after_trip": ""
  },
  "emergency_contact": {
    "name": "Alex Sanchez",
    "relationship": "Spouse",
    "phone": "+1 555 0123",
    "email": "alex@example.test"
  },
  "dive_insurance": {
    "provider": "DAN",
    "policy_number": "DAN-123",
    "expires_on": "2027-01-01"
  },
  "dive_profile": {
    "certification_level": "Advanced Open Water",
    "certification_agency": "PADI",
    "logged_dives": 120,
    "last_dive_on": "2026-04-01",
    "nitrox_certified": true
  },
  "dietary": {
    "dietary_requirements": "Vegetarian",
    "allergies": "Peanuts",
    "medical_notes": ""
  },
  "rental_gear": {
    "needs_rental_gear": true,
    "bcd_size": "M",
    "wetsuit_size": "M",
    "fins_size": "EU 42",
    "mask": false,
    "regulator": true,
    "dive_computer": false,
    "notes": ""
  },
  "notes": {
    "general": ""
  },
  "documents": []
}
```

Validation:

- Required on final submit: legal name, date of birth, nationality,
  emergency contact name/phone, certification level, logged dives, and
  explicit dietary/allergy acknowledgement even when empty.
- Passport/travel-document fields are optional in draft; on submit
  require either complete document fields or an explicit "will provide
  later" flag.
- Flight/logistics fields are optional because guests may not have
  booked travel yet.
- Dates use `YYYY-MM-DD`; datetimes use RFC3339.
- Country fields use ISO-like uppercase two-letter values when known,
  but validation permits blank for drafts.

### Backend Flow

```
Admin/Director           API/Auth service              DB/Email
----------------         ----------------              --------
POST manifest guest      validate trip scope           INSERT trip_guests
                         check capacity                INSERT guest_trip_invitations
                         mint token                    SEND guest_registration_invite

Guest opens link         lookup token                  SELECT invitation + trip context
GET guest invite         reject expired/revoked

Guest sets password      validate token/email          INSERT or authenticate guest_users
POST accept              create guest session          UPDATE trip_guest + invitation
                         set lb_guest_session

Guest registration       guest session middleware      UPSERT guest_trip_registrations
GET/PATCH/POST           enforce trip_guest ownership  UPDATE status timestamps

Admin/Director manifest  staff session middleware      SELECT trip_guests + registration status
GET                      enforce org/trip scope
```

### Authorization

Staff routes:

- Org Admin can list manifests, add guests, resend invites, revoke
  invites, and view submitted registration for any org trip that is not
  cancelled.
- Cruise Director can list manifests, add guests, resend invites, and
  view submitted registration only for trips where a
  `trip_cruise_directors` row assigns them. They cannot access other
  trips by guessing IDs.
- Mutations are blocked for completed/cancelled trips. Planned and
  active are allowed in Sprint 014 for both Org Admin and assigned
  Cruise Director, matching the intent that directors can add guests to
  assigned trips and especially active trips.

Guest routes:

- Public token lookup and accept routes require no session but reveal
  only minimal context: operator name, boat name, trip dates, itinerary,
  guest email, guest name, and expiry.
- Guest registration routes require `lb_guest_session` and verify that
  the authenticated guest owns the target `trip_guest` row.
- Guest routes never call `auth.UserFromContext`, never set `lb_session`,
  and never return staff `Me` payloads.

### Email

Add `email.KindGuestRegistrationInvite` with subject/text/html
templates. Variables should include:

- `AppName`
- `OrganizationName`
- `BoatName`
- `TripDates`
- `Itinerary`
- `RecipientName`
- `RecipientEmail`
- `ActionURL`
- `ExpiresAt`

The link format should be:

`{AppBaseURL}/guest/invitations/{token}`

### Frontend Routes

Public/guest routes outside admin chrome:

- `/guest/invitations/:token` - lookup invite, accept/create guest
  account, then redirect to registration.
- `/guest/trips/:tripGuestId/register` - guest registration wizard.

Admin routes:

- `/admin/trips` adds manifest counts and a `Manifest` action per row.
- `/admin/trips/:id/manifest` shows trip context, manifest table, add
  guest form, status chips, resend/revoke actions, and submitted
  registration detail drawer.

The UI should follow `DESIGN.md`: dense tables, restrained forms,
slate/amber working surfaces, no tourism-style marketing layout, no
large hero treatment for the guest form.

## Implementation Plan

### Phase 1: Schema and Store Layer (~30%)

**Files:**
- `internal/store/migrations/0012_guest_registration.sql` - Create
  guest account, guest session, manifest, invitation, registration, and
  document metadata tables.
- `internal/store/guest_users.go` - Guest account and guest session
  helpers.
- `internal/store/trip_guests.go` - Manifest CRUD, capacity checks,
  status transitions, invitation token lookup.
- `internal/store/guest_registrations.go` - Draft/save/submit
  registration persistence and submitted view helpers.
- `internal/store/trip_guests_test.go` - Manifest, token, duplicate,
  capacity, and scoping tests.
- `internal/store/guest_registrations_test.go` - Registration draft,
  submit, status, and ownership tests.

**Tasks:**
- [ ] Add migration `0012_guest_registration.sql` with constraints and
      indexes.
- [ ] Implement `CreateTripGuestWithInvite` transaction: verify trip
      belongs to org, lock existing manifest rows for count, reject
      capacity overflow when `trips.num_guests` is present, insert
      manifest row, insert invitation row.
- [ ] Use `trips.num_guests` as the current trip capacity cap. If it is
      `NULL`, allow manifest growth but show capacity as unknown.
- [ ] Implement pending invite lookup by token hash with expired,
      accepted, and revoked states surfaced distinctly to the service.
- [ ] Implement resend by rotating token hash, extending expiry, and
      preserving the same `trip_guest` row.
- [ ] Implement revoke by marking invitation and manifest row revoked.
- [ ] Implement `FindOrCreateGuestUserForInvite`: create a guest user
      if email is new, or authenticate the existing guest user password
      if email already exists.
- [ ] Implement separate guest session create/lookup/revoke helpers.
- [ ] Implement registration upsert for draft and submit.
- [ ] Keep all list/read helpers scoped by `organization_id` and
      `trip_id`.

### Phase 2: Guest Auth and Registration Service (~20%)

**Files:**
- `internal/auth/guest_accounts.go` - Guest invite accept/account
  creation, password validation, guest session minting.
- `internal/auth/guest_middleware.go` - `lb_guest_session` middleware
  and guest context helpers.
- `internal/auth/tokens.go` - Reuse opaque token hashing helpers.
- `internal/email/templates.go` - Add guest invite email kind.
- `internal/email/templates/guest_registration_invite.subject.tmpl` -
  Create.
- `internal/email/templates/guest_registration_invite.txt.tmpl` -
  Create.
- `internal/email/templates/guest_registration_invite.html.tmpl` -
  Create.
- `internal/auth/guest_accounts_test.go` - Token lifecycle, duplicate
  email, password, and session tests.

**Tasks:**
- [ ] Add guest-session cookie constants with `HttpOnly`, `SameSite=Lax`,
      and the same Secure behavior as staff sessions.
- [ ] Add guest invitation duration knob, defaulting to 7 days.
- [ ] Add service methods: `InviteTripGuest`, `LookupGuestInvite`,
      `AcceptGuestInvite`, `ResendGuestInvite`, `RevokeGuestInvite`.
- [ ] Ensure guest account creation marks email verified because the
      invitation proves possession of that email.
- [ ] Ensure existing guest account acceptance requires the correct
      password and does not reveal whether the email exists outside the
      token-bearing context.
- [ ] Render and send guest registration emails with trip context.

### Phase 3: Backend HTTP API (~20%)

**Files:**
- `internal/httpapi/guest_manifest_handlers.go` - Staff manifest
  endpoints.
- `internal/httpapi/guest_registration_handlers.go` - Public invite and
  guest registration endpoints.
- `internal/httpapi/httpapi.go` - Mount staff, public, and guest routes.
- `internal/httpapi/guest_manifest_handlers_test.go` - RBAC, capacity,
  token, resend/revoke, and tenant isolation tests.
- `internal/httpapi/guest_registration_handlers_test.go` - Public
  lookup, accept, guest session, validation, submit, and cross-trip
  rejection tests.

**Tasks:**
- [ ] Mount public token routes:
      `GET /api/guest/invitations/lookup?token=...` and
      `POST /api/guest/invitations/accept`.
- [ ] Mount guest-session routes:
      `GET /api/guest/trip-registrations/{trip_guest_id}` and
      `PATCH /api/guest/trip-registrations/{trip_guest_id}` for draft,
      plus `POST /api/guest/trip-registrations/{trip_guest_id}/submit`.
- [ ] Mount staff manifest routes under authenticated staff session:
      `GET /api/admin/trips/{id}/manifest`,
      `POST /api/admin/trips/{id}/guests`,
      `POST /api/admin/trips/{id}/guests/{guest_id}/resend`,
      `DELETE /api/admin/trips/{id}/guests/{guest_id}/invite`,
      and `GET /api/admin/trips/{id}/guests/{guest_id}/registration`.
- [ ] Add helper `canManageTripManifest(ctx, user, tripID)` that grants
      Org Admin by org scope and Cruise Director only through
      `trip_cruise_directors`.
- [ ] Return consistent JSON errors: `invalid_input`, `forbidden`,
      `not_found`, `capacity_exceeded`, `token_invalid`,
      `token_expired`.
- [ ] Use `http.MaxBytesReader` for registration JSON bodies with a
      bounded size.
- [ ] Avoid logging registration payloads or invite tokens.

### Phase 4: Admin Manifest UI (~15%)

**Files:**
- `web/src/admin/pages/Trips.tsx` - Add manifest count/status columns
  and row link to manifest view.
- `web/src/admin/pages/TripManifest.tsx` - Create trip manifest page.
- `web/src/admin/api.ts` - Add manifest and registration admin types
  and API wrappers.
- `web/src/main.tsx` - Add `/admin/trips/:id/manifest` route for both
  roles.
- `web/src/styles/app.css` - Add manifest table, status chips, side
  drawer/modal, and compact form styling.

**Tasks:**
- [ ] Extend trip list response or add a manifest summary endpoint so
      `/admin/trips` can show `submitted / total` and remaining capacity.
- [ ] Add a manifest link/action for each trip visible to the current
      role.
- [ ] Build manifest page with trip header, capacity summary, add guest
      form, manifest table, status chips, and registration detail drawer.
- [ ] Add guest form fields: full name and email. Email is required in
      this sprint because registration email is central to the workflow.
- [ ] Show resend and revoke actions only for pending/expired invites.
- [ ] Disable mutations for completed/cancelled trips with clear inline
      text.
- [ ] Keep Cruise Director UI scoped to assigned trips only.

### Phase 5: Guest Registration UI (~15%)

**Files:**
- `web/src/pages/GuestInvitation.tsx` - Create invite lookup and
  password/account step.
- `web/src/pages/GuestRegistration.tsx` - Create sectioned
  registration wizard.
- `web/src/lib/api.ts` - Add guest invite/session/registration types
  and API wrappers.
- `web/src/main.tsx` - Add guest routes.
- `web/src/styles/app.css` - Add guest auth and registration form
  styles reusing auth/admin tokens.

**Tasks:**
- [ ] Build invite lookup page with operator/trip context and immutable
      email field.
- [ ] Accept/create account with password only.
- [ ] Redirect accepted guests into their trip registration route.
- [ ] Build sectioned registration form with tabs or stepper:
      Identity, Travel, Emergency, Diving, Dietary, Gear, Notes.
- [ ] Implement save draft and submit actions.
- [ ] Show validation messages per section on submit.
- [ ] Keep the guest route outside `RequireSession` and admin chrome.
- [ ] Ensure guest session failures redirect to the token accept page or
      show a useful "open your registration link again" state.

### Phase 6: Product Docs, Smoke, and Hardening (~10%)

**Files:**
- `docs/product/personas.md` - Move guest registration from future-only
  into limited self-service scope and clarify admin/director manifest
  boundaries.
- `docs/product/organization-admin-user-stories.md` - Update manifest
  stories to require guest email/registration invite and status tracking.
- `DESIGN.md` - No palette changes expected; only update if the guest
  pages require a documented pattern.

**Tasks:**
- [ ] Update persona docs to say Guest owns only their own trip
      registration in this release; folio, schedule, trip details, and
      checkout remain future.
- [ ] Update Org Admin stories US-4.5/US-4.6/US-7.1 to reflect email
      invitation and registration status.
- [ ] Run backend tests and frontend build.
- [ ] Smoke happy paths: admin add, director add assigned trip, email
      link, guest accept, draft save, submit, manifest status update.
- [ ] Smoke negative paths: capacity exceeded, expired token, revoked
      token, wrong guest, wrong director, cross-org trip ID.

## API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/admin/trips/{id}/manifest` | GET | Staff manifest view for an org-scoped or assigned trip |
| `/api/admin/trips/{id}/guests` | POST | Add guest and send registration invite |
| `/api/admin/trips/{id}/guests/{guest_id}/resend` | POST | Rotate invitation token and resend email |
| `/api/admin/trips/{id}/guests/{guest_id}/invite` | DELETE | Revoke pending invite / manifest invitation |
| `/api/admin/trips/{id}/guests/{guest_id}/registration` | GET | View submitted or draft registration details |
| `/api/guest/invitations/lookup?token=...` | GET | Public token lookup for invite context |
| `/api/guest/invitations/accept` | POST | Create/authenticate guest account and set guest session |
| `/api/guest/trip-registrations/{trip_guest_id}` | GET | Load guest-owned registration |
| `/api/guest/trip-registrations/{trip_guest_id}` | PATCH | Save draft registration |
| `/api/guest/trip-registrations/{trip_guest_id}/submit` | POST | Validate and submit registration |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/store/migrations/0012_guest_registration.sql` | Create | Guest, manifest, invitation, registration, document metadata schema |
| `internal/store/guest_users.go` | Create | Guest account and guest session persistence |
| `internal/store/trip_guests.go` | Create | Trip manifest, invite lifecycle, capacity checks |
| `internal/store/guest_registrations.go` | Create | Registration draft/submit persistence |
| `internal/auth/guest_accounts.go` | Create | Guest account service and invite acceptance |
| `internal/auth/guest_middleware.go` | Create | Separate guest session middleware |
| `internal/email/templates.go` | Modify | Register guest registration invite email kind |
| `internal/email/templates/guest_registration_invite.*.tmpl` | Create | Guest invite subject/text/html templates |
| `internal/httpapi/guest_manifest_handlers.go` | Create | Staff manifest API |
| `internal/httpapi/guest_registration_handlers.go` | Create | Public invite and guest registration API |
| `internal/httpapi/httpapi.go` | Modify | Mount new routes |
| `web/src/admin/pages/Trips.tsx` | Modify | Surface manifest counts and manifest links |
| `web/src/admin/pages/TripManifest.tsx` | Create | Admin/director manifest management page |
| `web/src/admin/api.ts` | Modify | Add manifest API types and wrappers |
| `web/src/pages/GuestInvitation.tsx` | Create | Guest invite accept/account page |
| `web/src/pages/GuestRegistration.tsx` | Create | Guest registration wizard |
| `web/src/lib/api.ts` | Modify | Add guest API types and wrappers |
| `web/src/main.tsx` | Modify | Add admin manifest and guest routes |
| `web/src/styles/app.css` | Modify | Add dense manifest and guest form styles |
| `docs/product/personas.md` | Modify | Update guest and manifest boundaries |
| `docs/product/organization-admin-user-stories.md` | Modify | Update manifest and oversight stories |
| `internal/store/*_test.go` | Create/Modify | Store tests for schema and workflows |
| `internal/httpapi/*_test.go` | Create/Modify | API, RBAC, token, and validation tests |
| `internal/auth/*_test.go` | Create/Modify | Guest auth/session tests |

## Definition of Done

- [ ] `users` remains staff-only; guest accounts use separate
      `guest_users` and `guest_sessions`.
- [ ] Org Admin can add a guest to an org trip and send a registration
      email.
- [ ] Assigned Cruise Director can add a guest only to assigned trips.
- [ ] Manifest capacity prevents adding more guests than `trips.num_guests`
      when that value is set.
- [ ] Guest registration invite tokens are opaque, hashed at rest,
      expiring, revocable, and rotated on resend.
- [ ] Guest can accept invite with email + password where email comes
      from the invite.
- [ ] Existing guest email behavior is handled without creating duplicate
      guest accounts.
- [ ] Guest can save a draft registration and submit a validated
      registration.
- [ ] Admins and assigned Cruise Directors can view manifest status and
      registration details.
- [ ] Guest cannot access admin routes, staff `/api/me`, other guests,
      other trips, or other organizations.
- [ ] Cruise Director cannot access a manifest for an unassigned trip.
- [ ] Registration fields are operator-neutral and destination-neutral.
- [ ] File binary upload is either not exposed or implemented only as
      metadata placeholders; no ad hoc local file storage is added unless
      explicitly implemented behind an interface.
- [ ] Product docs reflect the limited guest registration scope.
- [ ] Backend tests cover token lifecycle, guest account creation,
      duplicate email behavior, trip scoping, role authorization,
      capacity enforcement, registration validation, and status
      transitions.
- [ ] Frontend build passes.
- [ ] `go test ./...` passes.

## Risks and Mitigations

- **Risk: Guest role bleeds into staff authorization.** Mitigation:
  separate guest account/session tables and middleware; do not add a
  `guest` role to `users`.
- **Risk: Registration scope grows into a full guest portal.**
  Mitigation: routes are limited to invite acceptance and one
  trip-specific registration form; no folio, payment, schedule, checkout,
  or general guest dashboard.
- **Risk: Sensitive personal data is overexposed.** Mitigation:
  staff registration detail routes require org/trip scope; guest routes
  require ownership; logs must not include payloads; UI shows data only
  from explicit detail actions.
- **Risk: Destination-specific fields leak into schema.** Mitigation:
  use generic travel-document, logistics, and notes sections; defer
  configurable custom fields.
- **Risk: Capacity is not a true boat capacity yet.** Mitigation:
  use `trips.num_guests` as the per-trip cap from import data when
  present; show unknown capacity otherwise. Do not invent cabin capacity
  in this sprint.
- **Risk: JSONB payload becomes unvalidated free-form storage.**
  Mitigation: API validates against typed Go request structs and stores
  only server-approved section keys.
- **Risk: Existing guest account recovery is incomplete.** Mitigation:
  token-bearing accept flow supports existing password; generic guest
  login/password reset is documented as a follow-up, not required for
  first invite-driven registration.
- **Risk: Email send failure creates orphan manifest rows.** Mitigation:
  create manifest/invite transaction first, attempt send second, and mark
  status/invite_sent_at only after successful send or surface a retryable
  "invite not sent" state if send fails.

## Security Considerations

- Store only token hashes, never raw invite or session tokens.
- Use separate cookies for staff and guests: `lb_session` and
  `lb_guest_session`.
- Set guest cookie flags consistently with staff sessions:
  `HttpOnly`, `SameSite=Lax`, path `/`, bounded expiry, Secure in secure
  environments.
- Scope every manifest and registration query by `organization_id` and
  `trip_id`; do not rely solely on row IDs.
- Enforce Cruise Director authorization through `trip_cruise_directors`.
- Avoid email enumeration except inside a valid token-bearing invite
  context where the email is already disclosed to the invite recipient.
- Do not log registration payloads, passport numbers, policy numbers,
  invite tokens, or session tokens.
- Treat registration payload as sensitive personal data. Keep API
  responses minimal in list views and require explicit detail fetches.
- Bound JSON request sizes with `http.MaxBytesReader`.
- Add future retention/deletion policy hooks by keeping timestamps and
  document metadata separable from the registration payload.

## Dependencies

- Sprint 009 custom auth and email infrastructure.
- Sprint 010 invitation patterns, profile/account conventions, and
  Cruise Director role naming.
- Sprint 011 design system and admin chrome style.
- Sprint 012 `trips.num_guests` import field, used as current capacity.
- Sprint 013 `trip_cruise_directors` join table for assigned-director
  trip scoping.
- Local PostgreSQL with `citext` and `pgcrypto` extensions already used
  by existing migrations.
- No new third-party dependency is required for the MVP if file binary
  upload is deferred.

## Open Questions

- Should guests be able to edit submitted registration until trip
  completion, or should submission lock the form until staff reopens it?
- Should failed email send leave a manifest row in `invited` with a
  "send failed" flag, or should the API roll back the guest add?
- Should expired guest invitations automatically set `trip_guests.status`
  to `expired`, or should expiry be computed dynamically at read time?
- Should the first implementation include guest logout, or is closing
  the browser/session expiry enough for the invite-driven MVP?
- Should generic guest password reset/login be included immediately for
  returning guests, or deferred until there is a broader guest portal?
- Should passport/travel-document fields be required for all operators
  at submit time, or should the MVP allow "will provide later" to avoid
  hard-coding operator policy?
- What is the first retention policy for submitted registration data
  after a trip completes?

## References

- `docs/sprints/drafts/SPRINT-014-INTENT.md`
- `docs/sprints/SPRINT-011.md`
- `docs/sprints/SPRINT-012.md`
- `docs/sprints/SPRINT-013.md`
- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- Gaia Love guest registration reference: `https://divegaia.com/guest-registration/`
