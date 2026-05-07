# Sprint 014: Guest Management and Trip Registration

## Overview

Sprint 014 introduces the first guest-facing workflow in Liveaboard.
An Org Admin or assigned Cruise Director can add a guest to a trip,
send a secure registration email, and track whether that guest has
created an account and submitted trip registration.

This is a trip-registration sprint, not a general guest portal. Guests
do not enter the admin chrome, do not see organization data, and do not
interact with folios, checkout, payments, inventory, receipts, or dive
schedules. Registration data is modeled in generic liveaboard terms
based on the Gaia reference form: identity, travel document, travel
logistics, emergency contact, dive insurance, dive profile,
dietary/allergy notes, rental needs, and general notes. Gaia Love,
Indonesia, Raja Ampat, permit, and destination-specific fields are not
hard-coded.

The key architectural decision is to keep guest accounts separate from
staff accounts. The current `users` table is staff-only and drives
`/api/me`, `RequireOrgAdmin`, Cruise Director scoping, and the admin
chrome. Sprint 014 adds separate guest account/session tables and
guest-only middleware so guest access is always trip-scoped.

## Use Cases

1. **Add a guest to a trip.** Org Admin opens a trip manifest, enters
   guest name and email, and sends a registration invitation.
2. **Cruise Director adds a guest.** Cruise Director can add guests only
   to trips assigned to them through `trip_cruise_directors`.
3. **Registration email.** Guest receives a secure expiring link scoped
   to one trip guest row.
4. **Create guest account.** Guest opens the link, sees operator/trip
   context, confirms the invite email, sets a password, and receives a
   guest session. The email field is not editable.
5. **Existing guest account.** A guest invited again with the same email
   can authenticate inside the token-bearing flow and link the new trip
   to the existing guest account.
6. **Save draft.** Guest can save partial registration, leave, and
   return before final submit.
7. **Submit registration.** Guest submits generic trip registration
   sections required for operator preparation.
8. **Track readiness.** Admins and assigned Cruise Directors see
   manifest status: invite not sent, invited, expired, revoked, account
   created, draft saved, or submitted.
9. **Prevent data leakage.** Guests cannot see other guests, other
   trips, the admin chrome, or staff `/api/me`; Cruise Directors cannot
   access unassigned trip manifests.

## Architecture

### Core Rules

- `users` remains staff-only with `org_admin` and `cruise_director`.
- Guest identity lives in `guest_users`.
- Guest sessions live in `guest_sessions` and use a separate
  `lb_guest_session` cookie.
- `trip_guests` is the flat manifest row. No cabin/berth model is added.
- Every manifest and registration row carries `organization_id` and
  `trip_id`; store helpers scope by both.
- A guest invitation proves access to exactly one `trip_guest` row.
- Invite and session tokens are opaque, hashed at rest, expiring, and
  revocable.
- Registration can be saved as draft and then submitted.
- Staff list views show registration status only; sensitive registration
  detail is fetched explicitly and only after submission.
- Binary document upload is deferred. Passport/certification/liability
  documents are represented only by acknowledgements or notes in the
  registration payload.
- Password hashing and verification stay in `internal/auth`; store
  helpers only create, fetch, and update rows.
- Re-inviting a revoked guest reuses the existing `trip_guests` row,
  clears revoke/send-failure state, and creates a fresh invitation row.

### Current Model Constraints

The current schema does not have trip lifecycle status or boat capacity.
Sprint 014 must not pretend those columns exist.

`trips.num_guests` is an expected/imported guest count from Sprint 012,
not a capacity constraint. The manifest uses it for occupancy display
and warning states only. Hard blocking on capacity is deferred until an
operator-owned boat/trip capacity model exists.

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
  invite_send_status text not null default 'not_sent'
    check (invite_send_status in ('not_sent','sent','failed')),
  invite_last_sent_at timestamptz null,
  invite_last_error text null,
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
  status text not null check (status in ('draft','submitted')),
  payload jsonb not null default '{}'::jsonb,
  submitted_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (trip_guest_id)
)
```

Indexes:

- `guest_sessions_guest_user_id_idx`
- `guest_sessions_expires_at_idx`
- `trip_guests_org_trip_idx` on `(organization_id, trip_id)`
- `trip_guests_guest_user_idx` on `(guest_user_id)`
- partial unique `guest_trip_invitations_trip_guest_pending_idx` on
  `(trip_guest_id)` where `accepted_at IS NULL AND revoked_at IS NULL`
- `guest_trip_invitations_expires_idx`
- `guest_trip_registrations_org_trip_idx` on `(organization_id, trip_id)`

Manifest status is computed for responses from durable state:

| Response status | Source |
|---|---|
| `revoked` | `trip_guests.revoked_at` or active invite revoked |
| `expired` | active invite expired and not accepted |
| `invite_failed` | `trip_guests.invite_send_status = 'failed'` |
| `invited` | active invite sent, not accepted |
| `account_created` | `trip_guests.guest_user_id` set, no registration |
| `registration_draft` | registration row status `draft` |
| `submitted` | registration row status `submitted` |

### Registration Payload

The API validates typed Go structs and stores the validated document as
JSONB. Payload keys are fixed for Sprint 014:

```json
{
  "identity": {
    "legal_name": "",
    "preferred_name": "",
    "date_of_birth": "",
    "nationality": "",
    "country_of_residence": "",
    "phone": ""
  },
  "travel_document": {
    "document_type": "passport",
    "document_number": "",
    "issuing_country": "",
    "expires_on": "",
    "will_provide_later": false
  },
  "travel_logistics": {
    "arrival_from": "",
    "arrival_flight_number": "",
    "arrival_at": "",
    "arrival_location": "",
    "departure_to": "",
    "departure_flight_number": "",
    "departure_at": "",
    "departure_location": "",
    "hotel_before_trip": "",
    "hotel_after_trip": ""
  },
  "emergency_contact": {
    "name": "",
    "relationship": "",
    "phone": "",
    "email": ""
  },
  "dive_insurance": {
    "provider": "",
    "policy_number": "",
    "expires_on": "",
    "will_provide_later": false
  },
  "dive_profile": {
    "certification_agency": "",
    "certification_level": "",
    "logged_dives": null,
    "last_dive_on": "",
    "nitrox_certified": null,
    "strong_current_experience": null,
    "camera": null
  },
  "dietary": {
    "dietary_requirements": "",
    "allergies": "",
    "medical_notes": "",
    "no_dietary_or_allergy_notes": false
  },
  "rental_gear": {
    "needs_rental_gear": false,
    "items": [],
    "height": "",
    "weight": "",
    "bcd_size": "",
    "wetsuit_size": "",
    "fins_size": "",
    "notes": ""
  },
  "notes": {
    "general": "",
    "destination_or_permit_notes": ""
  }
}
```

Required on final submit:

- legal name, date of birth, nationality, emergency contact name/phone
- certification agency/level and logged dive count
- explicit dietary/allergy acknowledgement, even when empty
- dive insurance details or `will_provide_later`
- travel-document details or `will_provide_later`

Travel logistics are optional because flights and hotels may not be
booked yet. Dates use `YYYY-MM-DD`; datetimes use RFC3339 when a time is
provided.

### Backend Flow

```
Staff adds guest       validate staff scope      insert trip_guests
POST manifest          create invite token       insert guest_trip_invitations
                       send email                update invite_send_status

Guest opens link       lookup token              return minimal trip context
GET path token         reject revoked/expired

Guest accepts          create/auth guest_user    mark invite accepted
POST accept            mint guest session        set account_created_at

Guest registers        guest middleware          save draft or submit payload
GET/PATCH/POST         enforce row ownership     update registration status

Staff manifest         staff middleware          list manifest + computed status
GET/detail             enforce org/assignment    return submitted details only
```

### Authorization

Staff:

- Org Admin can list, add, resend, revoke, and view submitted
  registration details for any trip in their organization.
- Cruise Director can perform the same actions only for trips assigned
  to them.
- Current schema has no trip status, so completed/cancelled read-only
  behavior is deferred until lifecycle status exists.

Guest:

- Public invite lookup reveals only operator name, boat name, trip
  dates, itinerary, invited guest name/email, and expiry.
- Public invite routes carry the raw token in the URL path, not a query
  string, matching the existing staff invitation URL style.
- Guest registration routes require `lb_guest_session`.
- Every guest registration request verifies that the authenticated guest
  owns the target `trip_guest` row.
- Guest routes never use `auth.UserFromContext`, never set
  `lb_session`, and never return staff user payloads.

### Email

Add `email.KindGuestRegistrationInvite` plus subject/text/html
templates:

- `internal/email/templates/guest_registration_invite.subject.tmpl`
- `internal/email/templates/guest_registration_invite.txt.tmpl`
- `internal/email/templates/guest_registration_invite.html.tmpl`

Variables:

- `AppName`
- `OrganizationName`
- `BoatName`
- `TripDates`
- `Itinerary`
- `RecipientName`
- `RecipientEmail`
- `ActionURL`
- `ExpiresAt`

Link format:

`{AppBaseURL}/guest/invitations/{token}`

### Frontend Routes

Admin/staff:

- `/admin/trips` adds manifest count/status summary and a `Manifest`
  action.
- `/admin/trips/:id/manifest` shows trip context, expected guest count,
  current manifest count, warnings, add guest form, manifest table,
  resend/revoke actions, and submitted registration detail drawer.

Guest:

- `/guest/invitations/:token` looks up invite context and accepts the
  invite with password.
- `/guest/trips/:tripGuestId/register` shows the sectioned registration
  form.

The guest pages reuse the auth/admin visual language from `DESIGN.md`:
dense forms, restrained typography, slate/amber working surfaces, and no
tourism-style landing page.

## Implementation Plan

### Phase 1: Schema and Store Layer (~25%)

**Files:**
- `internal/store/migrations/0012_guest_registration.sql` - guest
  account, guest session, manifest, invite, and registration tables.
- `internal/store/guest_users.go` - guest account/session persistence.
- `internal/store/trip_guests.go` - manifest CRUD, invitation lifecycle,
  manifest summary, and scope helpers.
- `internal/store/guest_registrations.go` - draft/save/submit helpers.
- `internal/store/guest_users_test.go` - guest account/session store
  tests.
- `internal/store/trip_guests_test.go` - manifest, invite, summary, and
  scoping tests.
- `internal/store/guest_registrations_test.go` - registration validation
  and ownership tests.

**Tasks:**
- [x] Add migration with constraints and indexes.
- [x] Implement create manifest row + invite transaction.
- [x] Add invite resend/revoke token rotation.
- [x] Add invite send state updates after email attempts.
- [x] Add guest account create/find helpers.
- [x] Add separate guest session helpers.
- [x] Add registration draft and submit persistence.
- [x] Add manifest summary lookup for trip lists.
- [x] Support re-invite after revoke by reusing the existing
      `trip_guests` row and inserting a new active invitation.
- [x] Treat `trips.num_guests` as expected count only; return warnings,
      not hard errors.

### Phase 2: Guest Auth and Email (~20%)

**Files:**
- `internal/auth/guest_accounts.go` - invite lookup/accept and guest
  account service.
- `internal/auth/guest_middleware.go` - `lb_guest_session` middleware.
- `internal/auth/cookie.go` - add guest cookie helpers or shared helper
  support.
- `internal/email/templates.go` - register guest invite email kind.
- `internal/email/templates/guest_registration_invite.*.tmpl` - email
  templates.
- `internal/auth/guest_accounts_test.go` - token, password, duplicate
  email, and session tests.

**Tasks:**
- [x] Reuse password validation and token hashing.
- [x] Add `lb_guest_session` cookie with `HttpOnly`, `SameSite=Lax`,
      path `/`, bounded expiry, and Secure behavior matching staff
      cookies.
- [x] Add `GuestSessionDuration` knob with a documented 30-day default.
- [x] Guest invite accept creates a guest account when email is new.
- [x] Existing guest email requires password verification in the
      token-bearing flow.
- [x] Mark guest email verified because the invite proves possession.
- [x] Apply login-style throttling to guest invite acceptance so a valid
      token cannot be used to brute-force an existing guest password.
- [x] Render and send registration invite emails.
- [x] Add guest logout endpoint if a guest session is minted.

### Phase 3: Backend HTTP API (~20%)

**Files:**
- `internal/httpapi/guest_manifest_handlers.go` - staff manifest API.
- `internal/httpapi/guest_registration_handlers.go` - public invite and
  guest registration API.
- `internal/httpapi/httpapi.go` - mount routes.
- `internal/httpapi/guest_manifest_handlers_test.go` - RBAC, resend,
  revoke, summary, and tenant tests.
- `internal/httpapi/guest_registration_handlers_test.go` - lookup,
  accept, session, ownership, and validation tests.

**Tasks:**
- [x] Mount public routes with no staff or guest middleware:
      `GET /api/guest/invitations/{token}` and
      `POST /api/guest/invitations/{token}/accept`.
- [x] Mount `POST /api/guest/logout`.
- [x] Mount guest-session registration routes:
      `GET/PATCH /api/guest/trip-registrations/{trip_guest_id}` and
      `POST /api/guest/trip-registrations/{trip_guest_id}/submit`.
- [x] Mount staff routes:
      `GET /api/admin/trips/{id}/manifest`,
      `POST /api/admin/trips/{id}/guests`,
      `POST /api/admin/trips/{id}/guests/{guest_id}/resend`,
      `DELETE /api/admin/trips/{id}/guests/{guest_id}/invite`,
      `GET /api/admin/trips/{id}/guests/{guest_id}/registration`.
- [x] Add `canManageTripManifest` helper for Org Admin and assigned
      Cruise Director access.
- [x] In `httpapi.go`, keep three explicit chi groups: public guest
      invite routes with no middleware, guest routes with
      guest-session middleware, and staff manifest routes with staff
      `SessionMiddleware`.
- [x] Redact tokenized `/api/guest/invitations/{token}` paths in the
      request logger so raw invite tokens do not land in API access
      logs.
- [x] Bound registration JSON body size with `http.MaxBytesReader`.
- [x] Avoid logging invite tokens or registration payloads.

### Phase 4: Admin Manifest UI (~15%)

**Files:**
- `web/src/admin/pages/Trips.tsx` - manifest summary and link.
- `web/src/admin/pages/TripManifest.tsx` - manifest page.
- `web/src/admin/api.ts` - staff manifest types/wrappers.
- `web/src/main.tsx` - admin manifest route.
- `web/src/styles/app.css` - manifest table, chips, drawer/modal, form
  styles.

**Tasks:**
- [x] Show manifest count and expected count on `/admin/trips`.
- [x] Add `Manifest` action for each visible trip.
- [x] Build manifest page with trip header, add guest form, table,
      status chips, warnings, resend/revoke, and submitted detail drawer.
- [x] Require guest full name and email.
- [x] Show invite send failures with retry affordance.
- [x] Keep Cruise Director UI scoped to assigned trips.

### Phase 5: Guest Registration UI (~15%)

**Files:**
- `web/src/pages/GuestInvitation.tsx` - invite lookup and password step.
- `web/src/pages/GuestRegistration.tsx` - registration form.
- `web/src/lib/api.ts` - guest API wrappers.
- `web/src/main.tsx` - guest routes.
- `web/src/styles/app.css` - guest auth/registration styles.

**Tasks:**
- [x] Lookup invite and show operator/trip context.
- [x] Accept invite with password only; email is fixed from invite.
- [x] Redirect accepted guest to registration.
- [x] Build sectioned form: Identity, Travel, Emergency, Diving,
      Dietary, Gear, Notes.
- [x] Implement save draft and submit.
- [x] Show per-section validation on submit.
- [x] Keep guest pages outside `RequireSession` and admin chrome.
- [x] Handle expired/revoked invite and missing guest session states.

### Phase 6: Product Docs and Verification (~5%)

**Files:**
- `docs/product/personas.md` - limited guest self-service scope.
- `docs/product/organization-admin-user-stories.md` - guest invite,
  registration, and manifest status stories.

**Tasks:**
- [x] Update Guest persona from future-only to limited trip registration.
- [x] Update Org Admin manifest stories for required email invite and
      status tracking.
- [x] Note capacity model, document upload, guest login/reset, and
      configurable fields as follow-ups.
- [x] Run `go test ./...`.
- [x] Run `npm run build`.
- [x] Smoke admin add, director add, resend, revoke, email link, guest
      accept, draft save, submit, manifest status update, cross-trip
      rejection.

## API Endpoints

| Endpoint | Method | Auth | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/manifest` | GET | Staff session | List trip manifest and summary. |
| `/api/admin/trips/{id}/guests` | POST | Staff session | Add guest and send registration invite. |
| `/api/admin/trips/{id}/guests/{guest_id}/resend` | POST | Staff session | Rotate token and resend invite. |
| `/api/admin/trips/{id}/guests/{guest_id}/invite` | DELETE | Staff session | Revoke active invite. |
| `/api/admin/trips/{id}/guests/{guest_id}/registration` | GET | Staff session | View submitted registration details. |
| `/api/guest/invitations/{token}` | GET | Public token | Lookup invite context by path token. |
| `/api/guest/invitations/{token}/accept` | POST | Public token | Create/auth guest account and set guest session. |
| `/api/guest/logout` | POST | Guest session | Revoke guest session cookie. |
| `/api/guest/trip-registrations/{trip_guest_id}` | GET | Guest session | Load guest-owned registration. |
| `/api/guest/trip-registrations/{trip_guest_id}` | PATCH | Guest session | Save draft registration. |
| `/api/guest/trip-registrations/{trip_guest_id}/submit` | POST | Guest session | Validate and submit registration. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0012_guest_registration.sql` | Create | Guest, session, manifest, invite, registration schema. |
| `internal/store/guest_users.go` | Create | Guest account and session persistence. |
| `internal/store/guest_users_test.go` | Create | Guest account/session store tests. |
| `internal/store/trip_guests.go` | Create | Manifest, invite lifecycle, summary helpers. |
| `internal/store/guest_registrations.go` | Create | Registration draft/submit persistence. |
| `internal/auth/guest_accounts.go` | Create | Guest invite/account service. |
| `internal/auth/guest_middleware.go` | Create | Guest session middleware. |
| `internal/auth/cookie.go` | Modify | Guest cookie helpers. |
| `internal/email/templates.go` | Modify | Guest invite email kind. |
| `internal/email/templates/guest_registration_invite.*.tmpl` | Create | Guest invite templates. |
| `internal/httpapi/guest_manifest_handlers.go` | Create | Staff manifest API. |
| `internal/httpapi/guest_registration_handlers.go` | Create | Guest/public registration API. |
| `internal/httpapi/httpapi.go` | Modify | Mount new routes. |
| `web/src/admin/pages/Trips.tsx` | Modify | Manifest summary and links. |
| `web/src/admin/pages/TripManifest.tsx` | Create | Staff manifest page. |
| `web/src/admin/api.ts` | Modify | Manifest API types/wrappers. |
| `web/src/pages/GuestInvitation.tsx` | Create | Guest invite accept page. |
| `web/src/pages/GuestRegistration.tsx` | Create | Guest registration form. |
| `web/src/lib/api.ts` | Modify | Guest API wrappers. |
| `web/src/main.tsx` | Modify | Guest and manifest routes. |
| `web/src/styles/app.css` | Modify | Manifest and guest form styles. |
| `docs/product/personas.md` | Modify | Guest registration scope. |
| `docs/product/organization-admin-user-stories.md` | Modify | Manifest/registration stories. |

## Definition of Done

- [x] Staff `users` remains staff-only; guest accounts use
      `guest_users` and `guest_sessions`.
- [x] Org Admin can add a guest to an organization trip and send a
      registration email.
- [x] Assigned Cruise Director can add a guest only to assigned trips.
- [x] Guest invite tokens are opaque, hashed at rest, expiring,
      revocable, and rotated on resend.
- [x] Invite email send failures are visible and retryable.
- [x] Guest can accept invite with email from invite plus password.
- [x] Existing guest email is handled without duplicate guest accounts.
- [x] Existing guest password verification happens in `internal/auth`,
      not `internal/store`.
- [x] Guest invite acceptance is throttled.
- [x] Guest sessions use a configurable `GuestSessionDuration` defaulting
      to 30 days.
- [x] Guest can log out and revoke `lb_guest_session`.
- [x] Guest can save draft registration and submit validated
      registration.
- [x] Re-invite after revoke is supported for the same guest email and
      trip.
- [x] Registration fields are operator-neutral and destination-neutral.
- [x] Admins and assigned Cruise Directors can view manifest status and
      submitted registration details.
- [x] Guest cannot access admin routes, staff `/api/me`, other guests,
      other trips, or other organizations.
- [x] Cruise Director cannot access unassigned trip manifests.
- [x] `trips.num_guests` is used only for expected-count display and
      warnings, not hard capacity enforcement.
- [x] Binary document upload is not exposed in Sprint 014.
- [x] Product docs reflect limited guest registration scope.
- [x] Backend tests cover token lifecycle, guest account creation,
      duplicate email behavior, trip scoping, role authorization,
      expected-count warnings, registration validation, and status
      transitions.
- [x] `npm run build` passes.
- [x] `go test ./...` passes.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---:|---:|---|
| Guest access bleeds into staff auth | Medium | High | Separate guest account/session tables, cookie, and middleware. |
| Scope grows into full guest portal | High | High | Limit routes to invite acceptance and trip registration only. |
| Sensitive data overexposed | Medium | High | Minimal list responses, explicit detail fetches, ownership checks, no payload logs. |
| Destination-specific form fields hard-code one operator | Medium | Medium | Generic sections plus notes; configurable custom fields deferred. |
| Capacity is misunderstood | High | Medium | Treat `trips.num_guests` as expected count only. |
| JSONB becomes unvalidated free-form data | Medium | High | Validate typed Go structs and store server-approved shape. |
| Existing guest account recovery is incomplete | Medium | Medium | Support token-bearing existing-password flow; generic login/reset deferred. |
| Email send failure leaves unclear state | Medium | Medium | Store invite send status and expose retry. |
| Raw invite token lands in API logs | Low | High | Redact tokenized guest invite paths in request logging. |

## Security Considerations

- Store only token hashes, never raw invite or session tokens.
- Carry public invite tokens in path parameters, matching existing
  invitation URL style, and redact those paths from request logs.
- Use separate cookies: `lb_session` for staff and `lb_guest_session` for
  guests.
- Guest cookie flags match staff cookie security.
- Scope every manifest and registration query by `organization_id` and
  `trip_id`.
- Enforce Cruise Director access via `trip_cruise_directors`.
- Avoid email enumeration except inside a valid token-bearing invite
  context.
- Throttle guest invite acceptance/password attempts.
- Do not log registration payloads, passport numbers, insurance policy
  numbers, invite tokens, or session tokens.
- Bound registration JSON bodies with `http.MaxBytesReader`.
- Add retention/export/delete policy as a follow-up before any broad
  reporting or export feature.

## Dependencies

- Sprint 009 custom auth and email infrastructure.
- Sprint 010 staff invitation and profile conventions.
- Sprint 011 design system and admin chrome.
- Sprint 012 `trips.num_guests` expected-count field.
- Sprint 013 `trip_cruise_directors` join table for assigned-director
  scoping.
- PostgreSQL `citext` and `pgcrypto` extensions already used by
  migrations.

## Follow-Ups

- Real boat/trip capacity model.
- Trip lifecycle status and lifecycle-gated manifest rules.
- Generic guest login and password reset.
- Binary document upload with storage backend abstraction.
- Configurable per-organization registration fields.
- Staff review/lock workflow for submitted registrations.
- Registration retention/export/delete policy.

## References

- Gaia Love guest registration reference:
  https://divegaia.com/guest-registration/
- `docs/sprints/drafts/SPRINT-014-INTENT.md`
- `docs/sprints/drafts/SPRINT-014-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-014-CODEX-DRAFT-CLAUDE-CRITIQUE.md`
- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
