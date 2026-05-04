# Sprint 009 Codex Draft: Custom Auth Stack — Clerk Unwind + Brevo Email

## Overview

Sprint 009 should be executed as a **targeted unwind** of Sprint 005,
not as a broad auth rewrite. Sprint 008's admin chrome, route structure,
RBAC semantics, and `/api/admin/*` endpoint shapes stay in place. The
work is to remove the Clerk-controlled identity/session plumbing under
those surfaces and replace it with a small local auth stack that we own:
email + password, opaque cookie sessions, invitation acceptance,
password reset, change password, resend flows, and change-email
re-verification.

The planner's Phase 4 decisions tighten the scope in two important ways:

1. Migration `0007` is intentionally destructive and wipes legacy user
   and org rows up front.
2. Change email is **not** deferred. It ships now, which means the
   schema and handler design must account for pending-email state
   explicitly instead of trying to fake it with `users.email`.

This draft recommends a single forward-only migration, a local auth
service under `internal/auth`, an SMTP-backed email package under
`internal/email`, and a frontend that restores straightforward custom
forms while keeping Sprint 008's admin shell intact.

## Principles

- **Preserve Sprint 008; reverse Sprint 005 only.** Trips, boats,
  admin IA, RBAC, and the admin endpoint surface remain.
- **Do not mix unwind and rebuild in the same code path.** Remove Clerk
  handlers, exchange flow, webhook flow, and provider abstractions
  completely; then rebuild on a direct local-auth model.
- **Keep the cookie contract.** `lb_session` remains the browser/API
  contract so the SPA and admin chrome do not need a second auth model.
- **Model pending state in tables, not in implicit flags.** Invitation,
  password reset, verification, and email-change flows all need
  explicit lifecycle rows with expiry, consumption, and revocation.
- **Prefer generic auth errors externally and exact state internally.**
  The backend should know whether the failure was bad password, locked
  login, or unverified email, but user-facing login errors should not
  enumerate accounts.

## Use Cases

1. **Org Admin signup**: user signs up with org name, full name, email,
   and password; the org + first admin are created locally; verification
   email is sent; login is blocked until the link is consumed.
2. **Login/logout**: verified active user logs in with email/password,
   receives `lb_session`, and logs out by deleting the matching session
   row and clearing the cookie.
3. **Invite Site Director**: org admin sends an invitation email from
   the Users page; the invitee follows the link, sets full name +
   password, and becomes an active `site_director`.
4. **Resend/revoke invitation**: org admin can resend a still-pending
   invite or revoke it before acceptance.
5. **Forgot/reset password**: user requests reset, receives email, sets
   a new password, and all existing sessions except the new one are
   invalidated.
6. **Change password**: signed-in user confirms current password, sets a
   new password, keeps the current session, and invalidates others.
7. **Change email**: signed-in user submits a new email plus current
   password; the old email remains authoritative until the new address
   confirms the change link; only then does `users.email` switch.
8. **Resend verification**: unverified user can request a fresh
   verification email without learning whether some other account exists.

## Architecture

### What stays vs what comes out

| Area | Action | Reason |
|---|---|---|
| `internal/auth/cookie.go` | Keep with minor reuse | Cookie name and token hashing remain valid. |
| `internal/auth/middleware.go` | Rewrite, keep names | `RequireSession` / `RequireOrgAdmin` stay the API contract. |
| `internal/httpapi/admin.go` | Keep with targeted auth-call updates | Sprint 008 surface stays. |
| `internal/store/trips.go`, `boats.go`, `organizations.go` | Keep | Not part of the unwind. |
| `internal/auth/provider.go`, `clerk.go`, `stub.go` | Delete | No external auth provider remains. |
| `internal/auth/webhook*.go` | Delete | No Clerk webhook or sync path remains. |
| `internal/auth/exchange.go` | Delete | No provider-session exchange exists. |
| `internal/store/app_sessions.go` | Delete | Replace with direct `sessions` repository. |
| `internal/httpapi` Clerk route mounts | Delete/replace | Clerk endpoints are removed entirely. |
| `web` Clerk components/config | Delete/replace | Restore custom auth pages. |

### High-level flow

```text
Browser (React SPA)
    │
    ├─ POST /api/signup
    │      create org + user + verification row
    │      send verification email
    │
    ├─ POST /api/login
    │      verify credentials + auth throttles
    │      create sessions row
    │      Set-Cookie: lb_session=...
    │
    ├─ GET /api/me, /api/admin/*
    │      RequireSession -> sessions lookup -> users row
    │
    ├─ POST /api/forgot-password
    │      create password_reset_tokens row
    │      send reset email
    │
    ├─ POST /api/invitations
    │      create invitations row
    │      send invite email
    │
    └─ POST /api/change-email
           create email_change_requests row
           send verification email to NEW address
           old address remains active until confirmation
```

### Backend package layout

```text
internal/auth/
  auth.go                 # password auth service, session minting, token issuance
  middleware.go           # RequireSession, RequireOrgAdmin
  cookie.go               # cookie read/write helpers, reused
  signup.go               # POST /api/signup, POST /api/resend-verification
  login.go                # POST /api/login, POST /api/logout
  password.go             # forgot/reset/change password handlers
  invitations.go          # send/resend/revoke/accept invitation handlers
  email_change.go         # request/confirm change-email handlers
  throttle.go             # login attempt checks + cooldown decisions

internal/email/
  smtp.go                 # SMTPSender over net/smtp
  message.go              # multipart message builder
  templates.go            # template loading/rendering helpers
  templates/
    verify_email.{txt,html}.tmpl
    invite.{txt,html}.tmpl
    reset_password.{txt,html}.tmpl
    change_email.{txt,html}.tmpl

internal/store/
  sessions.go
  email_verifications.go
  invitations.go
  password_reset_tokens.go
  email_change_requests.go
  login_attempts.go
```

The old provider abstraction should not be kept "just in case." It adds
indirection for a provider we are explicitly removing. A local auth
service is simpler to test and harder to mis-wire.

## Schema

### Migration 0007 strategy

`0007` should be a single transactional migration that both removes the
Clerk-specific artifacts and installs the local-auth schema needed by
the new flows. Because the planner made the wipe explicit, the migration
can favor clean invariants over data preservation.

Recommended ordering inside `0007`:

1. `DELETE FROM users;`
2. `DELETE FROM organizations;`
3. Drop tables that depend on old identities:
   `app_sessions`, `webhook_events`, `auth_sync_cursors`
4. Drop Clerk linkage columns:
   `users.clerk_user_id`, `organizations.clerk_org_id`
5. Recreate local-auth tables:
   `sessions`, `email_verifications`, `invitations`,
   `password_reset_tokens`, `email_change_requests`,
   `login_attempts`
6. Restore required user columns and invariants:
   `password_hash bytea NOT NULL`, `email_verified_at timestamptz NULL`
7. Preserve non-auth app schema from later sprints unchanged
   (`trips.site_director_user_id`, etc.)

This ordering matters. The destructive wipe must happen **before**
making `password_hash` non-null again; otherwise any dev DB containing
Clerk-era rows will fail the migration midway through.

### Table recommendations

#### `sessions`

Restore Sprint 003's direct session table, with one addition:
`revoked_at timestamptz NULL` is useful for explicit invalidation tests
and auditability even if rows are later cleaned up.

```sql
sessions(
  id uuid pk,
  user_id uuid not null references users(id) on delete cascade,
  token_hash bytea not null unique,
  created_at timestamptz not null,
  last_seen_at timestamptz not null,
  expires_at timestamptz not null,
  revoked_at timestamptz null
)
```

#### `email_verifications`

Keep the Sprint 003 shape and add `resend_count` only if the code truly
needs it. Otherwise keep the table lean and let the handler replace old
pending rows when resending.

#### `invitations`

This needs more than token storage. It is also the admin-facing pending
invite record.

```sql
invitations(
  id uuid pk,
  organization_id uuid not null references organizations(id) on delete cascade,
  email citext not null,
  role text not null check (role in ('site_director')),
  invited_by_user_id uuid not null references users(id) on delete restrict,
  token_hash bytea not null unique,
  expires_at timestamptz not null,
  accepted_at timestamptz null,
  revoked_at timestamptz null,
  created_at timestamptz not null,
  updated_at timestamptz not null
)
```

Constraints:

- Unique partial index on `(organization_id, email)` where
  `accepted_at IS NULL AND revoked_at IS NULL` so one org cannot spam the
  same address with multiple active invites.
- Acceptance must create or activate the local user row inside the same
  transaction that marks the invitation accepted.

#### `password_reset_tokens`

Use a dedicated table. Reset tokens have different invalidation rules
than invite/verify tokens and should not share lifecycle state.

```sql
password_reset_tokens(
  id uuid pk,
  user_id uuid not null references users(id) on delete cascade,
  token_hash bytea not null unique,
  expires_at timestamptz not null,
  consumed_at timestamptz null,
  created_at timestamptz not null
)
```

On successful reset:

- mark token consumed,
- update `users.password_hash`,
- delete or revoke every other session for that user,
- optionally clear outstanding reset tokens for that user.

#### `email_change_requests`

This is the new schema requirement driven by the planner's binding
decision. A simple token table is not enough. The app must preserve the
current email while a future email is pending verification.

```sql
email_change_requests(
  id uuid pk,
  user_id uuid not null references users(id) on delete cascade,
  new_email citext not null,
  token_hash bytea not null unique,
  expires_at timestamptz not null,
  consumed_at timestamptz null,
  created_at timestamptz not null,
  UNIQUE (new_email)
)
```

Behavioral constraints:

- `new_email` must be globally unique across `users.email`.
- Only one unconsumed request per user at a time; a partial unique index
  on `user_id` where `consumed_at IS NULL` is appropriate.
- Starting a new change-email request should invalidate any previous
  unconsumed request for that user.
- Confirmation must re-check that `new_email` is still unused before
  swapping `users.email`.
- On success, set `users.email = new_email` and
  `users.email_verified_at = now()` in the same transaction, then delete
  or consume the request row.

The system should **not** add `pending_email` to `users`. That couples a
rare transitional state to the canonical identity row and makes cleanup
harder. A separate request table is cleaner and matches the planner's
stated implication.

#### `login_attempts`

Losing Clerk means we now own online brute-force protection. Minimal
viable protection should be explicit in schema and code, not hand-waved
as "later middleware."

Recommended shape:

```sql
login_attempts(
  email citext primary key,
  failed_count integer not null,
  last_failed_at timestamptz not null,
  locked_until timestamptz null
)
```

This is intentionally simple. It gives the login handler a place to
apply per-email exponential cooldown without building a full distributed
rate-limiter yet.

## API Surface

Recommended auth endpoints:

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/signup` | Create org + first org admin; send verification email. |
| POST | `/api/resend-verification` | Re-issue verification for an unverified user. |
| POST | `/api/verify-email` | Consume verification token. |
| POST | `/api/login` | Email/password login with cooldown checks. |
| POST | `/api/logout` | Revoke current session and clear cookie. |
| POST | `/api/forgot-password` | Create reset token and email it. |
| POST | `/api/reset-password` | Consume reset token; rotate password; invalidate other sessions. |
| POST | `/api/change-password` | Authenticated password change. |
| POST | `/api/change-email` | Authenticated request for new email verification. |
| POST | `/api/change-email/confirm` | Consume email-change token; swap email. |
| POST | `/api/invitations` | Create and send invitation. |
| GET | `/api/invitations/{token}` | Resolve invitation metadata for the accept screen. |
| POST | `/api/invitations/{token}/accept` | Set name/password and accept invite. |
| POST | `/api/invitations/{id}/resend` | Resend a pending invite. |
| POST | `/api/invitations/{id}/revoke` | Revoke a pending invite. |

The intent mentions "five new auth endpoints," but the binding planner
decisions now require a larger concrete surface. The final sprint doc
should not artificially compress distinct flows into one endpoint if it
reduces clarity.

## Flow details

### Signup and verification

- `POST /api/signup` accepts `organization_name`, `full_name`, `email`,
  `password`.
- Handler creates org + admin user + verification row in one
  transaction.
- Password is bcrypt-hashed before insert.
- Verification email is sent after commit. If SMTP send fails, the
  request should return 500 and log enough context to re-trigger in dev;
  do not pretend the email was sent.
- Login for unverified users must not create a session. Return a generic
  invalid-credentials response plus a machine-readable code such as
  `verification_required` so the SPA can offer `Resend verification`
  without disclosing whether the email exists on truly bad credentials.

### Invitation acceptance

- Invitation creation must require `RequireOrgAdmin`.
- Invite token lookup page should reveal only minimal metadata:
  organization name, email, role, expiry, and pending/revoked/expired
  status.
- Accepting an invitation should:
  - verify token is active,
  - create the user row if absent, or reuse an inactive/unclaimed row if
    the design later introduces one,
  - set password hash and `email_verified_at = now()`,
  - set `is_active = true`,
  - mark invitation accepted,
  - optionally start a session immediately.

Starting a session immediately after invite acceptance is recommended; it
matches user expectation and reduces one redundant login step.

### Forgot/reset password

- `POST /api/forgot-password` always returns 204 even if the email is
  absent or inactive.
- For active users, create a short-lived reset token and send email.
- `POST /api/reset-password` consumes the token exactly once.
- Successful reset invalidates all existing sessions for the user, then
  optionally creates a fresh current session as part of the reset
  response. Either approach is fine, but the sprint doc should pick one.

Recommendation: create a fresh session immediately after successful
reset. The user has just proven possession of the email link and new
password, and the UX is cleaner.

### Change password

- Requires current password confirmation.
- Reject if new password equals current password.
- Update hash in a transaction and invalidate all other sessions.
- Keep the session performing the change alive.

### Change email

This is the flow most likely to be under-specified unless handled
directly in the sprint doc.

Recommended sequence:

1. Authenticated user submits `new_email` + `current_password`.
2. Handler verifies password and that `new_email != current email`.
3. Handler checks `new_email` is unused by any current user and not
   already reserved by another pending email-change request or active
   invitation where uniqueness matters.
4. Handler invalidates any existing unconsumed email-change request for
   the user.
5. Handler creates a new `email_change_requests` row and emails the NEW
   address.
6. Old email remains the login identifier until confirmation.
7. Confirmation endpoint consumes the token and swaps
   `users.email`/`users.email_verified_at` atomically.

Open implementation choice:

- Whether confirming a change-email request should invalidate all other
  sessions. Recommendation: yes, except the confirming session if one
  exists. Email is a primary login identifier; changing it should rotate
  session trust similarly to password reset.

## Brute-force defenses

Clerk absorbed this before. We need an explicit minimum defense now.

Recommended behavior:

- Track failed login attempts per normalized email in `login_attempts`.
- Cooldown schedule:
  - failures 1-4: no lock
  - failure 5: 1 minute
  - failure 6: 5 minutes
  - failure 7+: 15 minutes max cap
- Successful login clears the row.
- Unknown email addresses should still update a normalized in-memory or
  DB-backed attempt bucket so timing does not expose account existence.
- All credential failures return the same outward message.

This is not a substitute for IP-based rate limiting, but it is enough to
avoid a regression from Sprint 005's hosted protections.

## Email delivery and formatting

Brevo via SMTP is adequate, but the email package should not treat email
as "just write a string to a socket."

### Message requirements

- Send **multipart/alternative** messages with both `text/plain` and
  `text/html`.
- Set `From`, `To`, `Subject`, `MIME-Version`, and `Content-Type`
  correctly.
- Use a generated MIME boundary and CRLF line endings.
- Keep URLs absolute using `LIVEABOARD_APP_BASE_URL`.
- Avoid including secrets or raw token hashes in logs.
- Keep templates separate per email type; invitation/reset/verify/change
  email have different copy and risk profiles.

### Template approach

- Use `text/template` for text bodies.
- Use `html/template` for HTML bodies.
- Shared data model:
  `AppName`, `OrganizationName`, `RecipientEmail`, `ActionURL`,
  `ExpiresAt`, `SupportEmail` as needed.
- Store templates in `internal/email/templates/` so operational updates
  are localized and CLAUDE.md can point to a single directory.

The final sprint doc should call out that HTML rendering must be escaped
by default and that the plain-text part is not optional.

## Frontend

Sprint 008's admin shell remains. Clerk pages/components are replaced
with local forms and token-driven routes.

Recommended pages:

- `Signup`
- `Login`
- `ForgotPassword`
- `ResetPassword`
- `AcceptInvitation`
- `VerifyEmailResult`
- `ChangePassword` section in account/admin area
- `ChangeEmail` section in account/admin area

Frontend behavior notes:

- `RequireSession` continues to hinge on `/api/me`.
- Login should surface a resend-verification affordance only when the
  backend returns the machine-readable verification-required state.
- Invitation acceptance page should prefill the invited email as
  read-only.
- Reset/change-password pages should do client-side password-confirm
  checks but still rely on server validation.

## Implementation Plan

### Phase 1: Archive + schema + config (~15%)

**Files:**
- `docs/sprints/SPRINT-009.md`
- `internal/store/migrations/0007_custom_auth_unwind.sql`
- `internal/config/config.go`
- `scripts/dev_reset/main.go`
- `CLAUDE.md`

**Tasks:**
- [ ] Create/push `clerk-archive` from the current `main` tip.
- [ ] Add migration `0007` that wipes users/orgs, drops Clerk artifacts,
      and installs local-auth tables.
- [ ] Replace `CLERK_*` config with `LIVEABOARD_SMTP_*`,
      `LIVEABOARD_APP_BASE_URL`, and auth-expiry settings as needed.
- [ ] Update dev reset to local-only auth data.
- [ ] Document env vars, template locations, and local auth smoke flow
      in `CLAUDE.md`.

### Phase 2: Unwind Clerk cleanly (~20%)

**Files:**
- `internal/auth/provider.go`
- `internal/auth/clerk.go`
- `internal/auth/stub.go`
- `internal/auth/webhook*.go`
- `internal/auth/exchange.go`
- `internal/store/app_sessions.go`
- `internal/httpapi/httpapi.go`
- `cmd/server/main.go`
- `web/package.json`
- `web/src/main.tsx`
- `web/src/lib/clerkAppearance.ts`
- `web/src/pages/Signup.tsx`
- `web/src/pages/Login.tsx`

**Tasks:**
- [ ] Delete Clerk provider, webhook, and exchange code.
- [ ] Remove Clerk route mounts and wiring.
- [ ] Remove `@clerk/clerk-react` and related frontend setup.
- [ ] Ensure no `clerk` or `svix` imports remain outside sprint history.

This phase should land before the new auth flows are considered done.
Trying to leave the old provider seam around while adding new local auth
would blur responsibilities and invite regressions.

### Phase 3: Core local auth + sessions (~25%)

**Files:**
- `internal/auth/auth.go`
- `internal/auth/signup.go`
- `internal/auth/login.go`
- `internal/auth/middleware.go`
- `internal/store/users.go`
- `internal/store/sessions.go`
- `internal/store/email_verifications.go`
- `internal/httpapi/httpapi.go`

**Tasks:**
- [ ] Implement signup, verify-email, resend-verification, login,
      logout, and direct session middleware.
- [ ] Restore `password_hash` and `email_verified_at` handling in
      `users` repository methods.
- [ ] Add login cooldown enforcement.
- [ ] Keep `RequireSession` / `RequireOrgAdmin` semantics unchanged for
      Sprint 008 handlers.

### Phase 4: Invitations + password management + change email (~25%)

**Files:**
- `internal/auth/invitations.go`
- `internal/auth/password.go`
- `internal/auth/email_change.go`
- `internal/store/invitations.go`
- `internal/store/password_reset_tokens.go`
- `internal/store/email_change_requests.go`
- `web/src/pages/AcceptInvitation.tsx`
- `web/src/pages/ForgotPassword.tsx`
- `web/src/pages/ResetPassword.tsx`
- `web/src/admin/pages/Account.tsx` or equivalent account surface

**Tasks:**
- [ ] Implement send/resend/revoke/accept invitation.
- [ ] Implement forgot/reset/change password.
- [ ] Implement request/confirm change email with explicit pending
      request table semantics.
- [ ] Re-enable Sprint 008's invite UX end-to-end from the Users page.

### Phase 5: Email service + verification (~15%)

**Files:**
- `internal/email/*`
- `.env.local.example` if present
- tests under `internal/email` and `internal/auth`

**Tasks:**
- [ ] Build SMTP sender and multipart message builder.
- [ ] Add template pairs for verification, invite, reset, and
      change-email.
- [ ] Smoke at least signup verification and invitation acceptance
      against a real Brevo inbox.
- [ ] Keep automated tests off the real network.

## Test Strategy

The email tests need more detail than "dummy listener" because
`net/smtp` behavior and MIME formatting are easy to get subtly wrong.

### Backend unit/integration coverage

- Signup creates org/user/verification row.
- Login rejects bad password generically.
- Login rejects unverified users and exposes
  `verification_required` internally to the SPA.
- Login cooldown escalates and clears on success.
- Resend verification replaces or supersedes the prior pending token.
- Invitation resend/revoke obey pending-only rules.
- Invitation acceptance is one-time and transactionally creates or
  updates the user.
- Password reset invalidates prior sessions.
- Change password invalidates other sessions only.
- Change email leaves old email active until confirmation and swaps
  atomically on confirmation.
- Sprint 008 admin tests continue to pass with the new middleware.

### SMTP tests without Brevo

Recommended approach:

- Define a small sender interface in `internal/email`, but test the real
  SMTP implementation too.
- Start a local dummy SMTP server in tests, either:
  - a tiny in-process listener that accepts the minimum SMTP commands we
    use (`EHLO`, `AUTH`, `MAIL FROM`, `RCPT TO`, `DATA`, `QUIT`), or
  - a minimal third-party test server only if stdlib-first becomes
    impractical.
- Point `SMTPSender` at that listener and assert:
  - connection/auth sequence happens,
  - the message contains `multipart/alternative`,
  - both text and HTML parts are present,
  - rendered URLs and subjects match the template input.

If the in-process server is too much code, the package should at least
split message rendering from delivery so MIME construction is tested
deterministically without sockets, and the SMTP transport itself is
covered by a narrow integration test against the dummy listener.

### Manual smoke

- Signup -> receive verification mail in `mr.karim.fanous@gmail.com` ->
  verify -> login -> dashboard.
- Admin invite -> receive invite -> accept -> land in admin as Site
  Director.
- Forgot password -> receive reset -> set new password -> old session no
  longer works.
- Change email -> receive mail at new address -> confirm -> old email no
  longer logs in.

## Security Considerations

- Passwords remain bcrypt-hashed; no plaintext or reversible storage.
- Auth tokens in DB are stored hashed, never plaintext.
- Invitation, reset, verification, and change-email tokens are
  single-use and expiry-bound.
- Login errors remain generic to reduce enumeration risk.
- Unverified users cannot start sessions.
- Password reset and email change should rotate trust by invalidating
  other sessions.
- SMTP credentials stay in env only; no committed `.env.local`.

## Definition of Done

- [ ] `clerk-archive` exists locally and on origin at the pre-unwind tip.
- [ ] Migration `0007` applies cleanly on a Clerk-era dev database.
- [ ] No `clerk` or `svix` imports remain in runtime code.
- [ ] `@clerk/clerk-react` is removed and the SPA builds.
- [ ] Signup, verify, login, logout, invite, resend/revoke invite,
      forgot/reset password, change password, resend verification, and
      change email all work end-to-end in dev.
- [ ] Sprint 008 admin chrome behavior is preserved.
- [ ] Automated tests cover auth flow edge cases and SMTP formatting
      without hitting Brevo.
- [ ] At least one real Brevo verification or invite email is observed
      in the target inbox during smoke.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, and
      `npm run build` pass.

## Open choices to lock in the merged sprint

1. **Post-reset session behavior**: create a fresh session immediately
   after password reset, or force a follow-up login. Recommendation:
   create the fresh session.
2. **Post-change-email session rotation**: invalidate other sessions on
   confirmation. Recommendation: yes.
3. **Dummy SMTP server implementation**: hand-rolled minimal server vs a
   narrowly scoped test dependency. Recommendation: hand-rolled first;
   accept a tiny test-only dependency only if protocol coverage becomes
   noisy.
