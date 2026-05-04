# Sprint 009: Custom Auth Stack — Replace Clerk; Email via Brevo

## Overview

Sprint 005 swapped custom auth for Clerk; Sprint 008 built the admin
chrome on top of that auth. The product owner has decided the dependency
surface and hosted-identity-provider risk aren't worth it for this
product. Sprint 009 reverses Sprint 005 only — Sprint 008's admin chrome,
RBAC semantics, and `/api/admin/*` endpoint shapes stay intact — and
introduces a small self-hosted auth stack plus a Brevo SMTP integration
for the email-driven flows (verification, invitation, password reset,
change email).

This is a "swap" sprint. From the user's point of view the admin chrome
is unchanged; underneath, Clerk's hosted flow + JWT exchange is replaced
with bcrypt + opaque session cookie + a small `internal/email` package
that talks to Brevo over SMTP. Migration `0007` is intentionally
destructive — pre-customer, no real data to preserve, clean invariants
matter more than continuity.

The implementation largely restores Sprint 003's auth design with three
additions: invitation flow with a dedicated `invitations` table, a
two-phase change-email flow with an `email_change_requests` table, and a
DB-backed `login_attempts` table for brute-force defense (Clerk used to
absorb that for us).

## Use Cases

1. **Org bootstrap.** New operator visits `/signup`, fills in email +
   password + name + organization name, receives a verification email,
   clicks the link, lands on the dashboard as Org Admin.
2. **Login / logout.** Email + password at `/login` → `lb_session`
   cookie. Logout clears the session row + cookie.
3. **Invite Site Director.** Org Admin clicks `+ Invite` on
   `/admin/users`, enters email + role; invitee receives a token-bearing
   email; clicking the link lands them at
   `/invitations/{token}/accept` to set name + password; they're
   logged in immediately as `site_director`.
4. **Resend / revoke pending invitation.** Admin can rotate the token
   and resend, or revoke a pending invite.
5. **Forgot password.** `/forgot-password` → email with reset link →
   `/reset-password?token=...` to set a new password. All other
   sessions invalidated; a fresh session created immediately so the
   user doesn't need to log in again.
6. **Change password.** Authenticated user goes to
   `/admin/account/security`, confirms current password, sets a new
   one. All *other* sessions invalidated; current session preserved.
7. **Resend verification email.** Unverified user who tries to log in
   gets a "resend verification?" prompt; clicking issues a new email.
8. **Change email (two-phase).** Authenticated user submits
   `new_email` + current password → confirmation email goes to the
   *new* address. Old email remains the active login identifier until
   the link is clicked. On confirmation, `users.email` and
   `users.email_verified_at` swap atomically; all *other* sessions
   invalidated since the login identifier rotated.

## Architecture

### What stays vs what comes out

| Area | Action |
|---|---|
| `internal/auth/cookie.go` | Keep (generic cookie helpers) |
| `internal/auth/middleware.go` | Rewrite internals; keep `RequireSession` / `RequireOrgAdmin` names |
| `internal/auth/admin.go` (Sprint 005's invite handlers) | Refactor onto our new invitation service |
| `internal/httpapi/admin.go` (Sprint 008) | Keep entirely; only the auth wiring underneath swaps |
| `internal/store/trips.go`, `boats.go` | Keep entirely |
| `internal/store/users.go`, `organizations.go` | Drop Clerk cols; add `password_hash` + `email_verified_at` |
| `internal/auth/provider.go`, `clerk.go`, `stub.go`, `webhook*.go`, `exchange*.go` | **Delete** |
| `internal/store/app_sessions.go` | **Delete** (replaced by `sessions.go`) |
| `internal/store/webhook_events.go` | **Delete** |
| `internal/httpapi` Clerk route mounts (`/api/signup-complete`, `/api/auth/exchange`, `/api/webhooks/clerk`) | **Delete** |
| `web/package.json` `@clerk/clerk-react` | **Remove** |
| `web/src/lib/clerkAppearance.ts`, Clerk-tied parts of Signup/Login | **Delete** |

### High-level flow

```
Browser (React SPA)
    │
    ├─ POST /api/signup
    │     create org + user + email_verifications row
    │     send verification email; no session yet
    │
    ├─ POST /api/login
    │     login_attempts cooldown check
    │     bcrypt compare; reject if unverified
    │     create sessions row; Set-Cookie: lb_session
    │
    ├─ GET  /api/me, /api/admin/*
    │     RequireSession → sessions lookup → users row
    │
    ├─ POST /api/forgot-password   → password_reset_tokens row + email
    ├─ POST /api/reset-password    → consume token; rotate password;
    │                                  invalidate other sessions; mint
    │                                  fresh session for caller
    │
    ├─ POST /api/change-password   → confirm current; rotate; invalidate
    │                                  other sessions; KEEP current
    │
    ├─ POST /api/change-email      → email_change_requests row; email
    │                                  goes to NEW address; old email
    │                                  remains active until confirmation
    ├─ POST /api/change-email/confirm → swap users.email atomically;
    │                                  invalidate other sessions
    │
    └─ POST /api/invitations       → invitations row + invite email
       GET  /api/invitations/{token}            → metadata for accept page
       POST /api/invitations/{token}/accept     → create user; consume invite;
                                                  start session
```

### Schema (migration `0007`)

Single transactional migration. Ordering matters: destructive wipe
*before* tightening constraints, otherwise any Clerk-era dev DB fails
mid-statement.

```sql
-- 0007_replace_clerk_with_custom_auth.sql

-- 1. Wipe legacy rows. Pre-customer; clean slate.
DELETE FROM users;
DELETE FROM organizations;

-- 2. Drop Clerk integration tables.
DROP TABLE IF EXISTS app_sessions;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS auth_sync_cursors;

-- 3. Drop Clerk linkage columns.
ALTER TABLE users         DROP COLUMN IF EXISTS clerk_user_id;
ALTER TABLE organizations DROP COLUMN IF EXISTS clerk_org_id;

-- 4. Restore Sprint-003 columns dropped by migration 0004.
ALTER TABLE users
    ADD COLUMN password_hash bytea NOT NULL,
    ADD COLUMN email_verified_at timestamptz NULL;

-- 5. Restore Sprint-003 sessions / email_verifications tables.
CREATE TABLE sessions (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   bytea       NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz NULL
);
CREATE INDEX sessions_user_id_idx     ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx  ON sessions(expires_at);

CREATE TABLE email_verifications (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_verifications_user_id_idx ON email_verifications(user_id);

-- 6. New: invitations.
CREATE TABLE invitations (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id    uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email              citext      NOT NULL,
    role               text        NOT NULL CHECK (role IN ('site_director')),
    invited_by_user_id uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash         bytea       NOT NULL UNIQUE,
    expires_at         timestamptz NOT NULL,
    accepted_at        timestamptz NULL,
    accepted_user_id   uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    revoked_at         timestamptz NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invitations_org_idx     ON invitations(organization_id);
CREATE INDEX invitations_email_idx   ON invitations(email);
CREATE INDEX invitations_expires_idx ON invitations(expires_at);
-- Only one PENDING invitation per (org, email) at a time. Re-invite
-- after acceptance or revocation is allowed.
CREATE UNIQUE INDEX invitations_pending_unique_idx
    ON invitations(organization_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- 7. New: password reset tokens.
CREATE TABLE password_reset_tokens (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens(user_id);

-- 8. New: email-change requests (two-phase).
--    The new email lives here, NOT on users, until confirmation.
CREATE TABLE email_change_requests (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    new_email   citext      NOT NULL,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (new_email)
);
-- One unconsumed request per user; starting a new one supersedes the old.
CREATE UNIQUE INDEX email_change_requests_pending_unique_idx
    ON email_change_requests(user_id)
    WHERE consumed_at IS NULL;

-- 9. New: login attempts (per-email cooldown for brute-force defense).
CREATE TABLE login_attempts (
    email          citext      PRIMARY KEY,
    failed_count   integer     NOT NULL DEFAULT 0,
    last_failed_at timestamptz NOT NULL DEFAULT now(),
    locked_until   timestamptz NULL
);
```

### Backend layout

```
internal/auth/
  auth.go                  # Service: signup, login, logout, verify, resend-verification
  password.go              # forgot/reset/change password
  email_change.go          # request/confirm change-email
  invitations.go           # invite/resend/revoke/accept
  middleware.go            # RequireSession, RequireOrgAdmin (names preserved)
  cookie.go                # cookie helpers (kept from Sprint 005)
  tokens.go                # New(), Hash() — sha256(32 bytes from crypto/rand)
  throttle.go              # login_attempts cooldown logic
  passwordrules.go         # bcrypt + complexity validation
  *_test.go

internal/email/
  smtp.go                  # SMTPSender (net/smtp + STARTTLS + PLAIN auth)
  message.go               # multipart/alternative builder; CRLF; quoted-printable
  templates.go             # render Subject/Text/HTML for one of four kinds
  templates/
    verification.{subject,txt,html}.tmpl
    invitation.{subject,txt,html}.tmpl
    password_reset.{subject,txt,html}.tmpl
    change_email.{subject,txt,html}.tmpl
  fakesmtp_test.go         # in-process minimal SMTP listener for transport tests
  email_test.go            # render + multipart structure assertions

internal/store/
  sessions.go              # NEW (replaces app_sessions.go)
  email_verifications.go   # NEW
  invitations.go           # NEW
  password_reset_tokens.go # NEW
  email_change_requests.go # NEW
  login_attempts.go        # NEW
  users.go                 # password_hash + email_verified_at; remove ClerkUserID
  organizations.go         # remove ClerkOrgID

internal/store/migrations/
  0007_replace_clerk_with_custom_auth.sql

internal/config/config.go  # add LIVEABOARD_SMTP_*, LIVEABOARD_APP_BASE_URL; drop CLERK_*
internal/httpapi/httpapi.go     # mount custom auth routes; admin chrome unchanged
internal/httpapi/auth_handlers.go      # NEW
internal/httpapi/invitation_handlers.go # NEW

cmd/server/main.go         # construct auth.Service, email.SMTPSender; drop Clerk wiring

scripts/dev_reset/main.go  # local-only DB wipe (no more Clerk SDK calls)

web/src/
  main.tsx                 # drop ClerkProvider
  lib/api.ts               # drop Clerk JWT plumbing; add new endpoints
  lib/RequireSession.tsx   # plain /api/me probe
  lib/clerkAppearance.ts   # DELETE
  pages/
    Signup.tsx             # restored: email + password + name + org form
    Login.tsx              # restored: email + password form; resend-verification affordance
    VerifyEmail.tsx        # restored: ?token=... consumer
    ForgotPassword.tsx     # NEW
    ResetPassword.tsx      # NEW
    AcceptInvitation.tsx   # NEW
  admin/pages/Account.tsx  # NEW: change password + change email sections
  admin/pages/Users.tsx    # wire `+ Invite` modal end-to-end
```

### Token strategy

- Per-kind tables: `email_verifications`, `password_reset_tokens`,
  `invitations`, `email_change_requests`. Each stores
  `sha256(raw_token)`; the raw token only exists in the email link.
- Tokens are 32 bytes from `crypto/rand` → 64 hex chars.
- Default expiries:
  | Kind | Expiry |
  |---|---|
  | Email verification | **24h** |
  | Password reset | **1h** |
  | Invitation | **7d** |
  | Email change | **1h** |
- All consume-once: `consumed_at` (or `accepted_at` / `revoked_at`
  for invitations).

### Cookie / session

- Same shape as Sprint 003 / Sprint 005's `lb_session`: opaque random
  token, sha256 in DB, HTTP-only, `Secure` in production, SameSite=Lax.
- Default TTL **14 days**; `last_seen_at` bumped on every
  `RequireSession` resolve.
- Logout deletes the row.
- Reset password: invalidate ALL sessions for that user, then mint a
  fresh one for the caller.
- Change password: invalidate all sessions EXCEPT the caller's.
- Change email confirmation: invalidate all sessions EXCEPT the
  caller's (the email is the login identifier, so rotation is
  appropriate).

### Login response shape (verification-required clarification)

The login endpoint returns:

| Outcome | Status | Body |
|---|---|---|
| Success | 200 | `{ "ok": true }` + `Set-Cookie: lb_session=...` |
| Bad credentials | 401 | `{ "error": "invalid_credentials" }` |
| Locked out | 429 | `{ "error": "too_many_attempts", "retry_after_seconds": N }` |
| Email not verified | 401 | `{ "error": "verification_required" }` |
| Inactive account | 401 | `{ "error": "invalid_credentials" }` (no enumeration) |

The `verification_required` code only fires *after* a clean credential
check (correct password, account exists, active). That preserves the
generic-failure invariant for truly bad credentials while letting the
SPA show a "resend verification?" affordance only for legitimately
unverified accounts.

### Brute-force defense (`login_attempts`)

- Failed login: insert-or-update `login_attempts(email)`; bump
  `failed_count`; recompute `locked_until`.
- Cooldown schedule:
  - Failures 1–4: no lock
  - Failure 5: `locked_until = now + 1m`
  - Failure 6: `+ 5m`
  - Failure 7+: `+ 15m` (cap)
- Successful login: delete the row.
- Unknown email: still update a row (using the normalized email) so
  timing doesn't enumerate. The login handler returns the *same*
  `invalid_credentials` either way.

### Config additions

| Field | Env | Default | Notes |
|---|---|---|---|
| `SMTPHost` | `LIVEABOARD_SMTP_HOST` | `smtp-relay.brevo.com` | non-secret |
| `SMTPPort` | `LIVEABOARD_SMTP_PORT` | `587` | non-secret |
| `SMTPUsername` | `LIVEABOARD_SMTP_USERNAME` | (none) | **secret**, required |
| `SMTPPassword` | `LIVEABOARD_SMTP_PASSWORD` | (none) | **secret**, required |
| `SMTPFrom` | `LIVEABOARD_SMTP_FROM` | (none) | non-secret; required |
| `AppBaseURL` | `LIVEABOARD_APP_BASE_URL` | `http://localhost:5173` | for email links; must be `https://` in prod |

Removed: `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`,
`CLERK_WEBHOOK_SECRET`, `VITE_CLERK_PUBLISHABLE_KEY`. Drop from
`Config`, `.env.example`, `config/*.env`. User removes them from their
`.env.local`.

## Implementation Plan

### Phase 1: Archive + schema + config (~15%)

**Files:** `internal/store/migrations/0007_replace_clerk_with_custom_auth.sql`, `internal/config/config.go`, `internal/testdb/testdb.go`, `.env.example`, `config/dev.env`, `config/test.env`, `CLAUDE.md`.

**Tasks:**
- [ ] `git branch clerk-archive main && git push -u origin clerk-archive`.
- [ ] Migration `0007` with the destructive-first ordering above.
- [ ] `internal/testdb/testdb.go` truncate list updated: drop
      `app_sessions`, `webhook_events`, `auth_sync_cursors`; add
      `sessions`, `email_verifications`, `invitations`,
      `password_reset_tokens`, `email_change_requests`,
      `login_attempts`.
- [ ] Config: drop CLERK_*; add `LIVEABOARD_SMTP_*` (secret-tagged),
      `LIVEABOARD_APP_BASE_URL`. Production validate enforces
      `https://` prefix on `AppBaseURL`.
- [ ] `.env.example` documents the new keys with placeholder values.
- [ ] CLAUDE.md gains a short "Auth stack" section linking to
      `docs/auth.md` (rewritten in Phase 6).

### Phase 2: Unwind Clerk cleanly (~15%)

**Files to delete:** `internal/auth/provider.go`, `clerk.go`,
`clerk_test.go`, `stub.go`, `webhook.go`, `webhook_handlers.go`,
`webhook_test.go`, `exchange.go`, `exchange_test.go`,
`internal/store/app_sessions.go`, `internal/store/app_sessions_test.go`,
`internal/store/webhook_events.go`, `web/src/lib/clerkAppearance.ts`.

**Files to modify:** `internal/httpapi/httpapi.go` (drop Clerk route
mounts), `cmd/server/main.go` (drop Clerk wiring),
`web/package.json` + `web/package-lock.json` (npm uninstall
`@clerk/clerk-react`), `web/src/main.tsx` (drop `<ClerkProvider>`).

**Tasks:**
- [ ] Delete the files above.
- [ ] `go mod tidy` — confirms `clerk-sdk-go/v2` and
      `svix-webhooks/go` drop.
- [ ] `git grep "clerk"` inside `internal/` returns nothing.
- [ ] SPA still compiles (will be broken at the page level until
      Phase 6 restores the forms).

### Phase 3: Core auth + sessions (~25%)

**Files:** `internal/auth/auth.go`, `internal/auth/middleware.go`
(rewrite internals; keep names),
`internal/auth/cookie.go` (small touch-ups), `internal/auth/tokens.go`,
`internal/auth/passwordrules.go`, `internal/auth/throttle.go`,
`internal/store/sessions.go`, `internal/store/email_verifications.go`,
`internal/store/login_attempts.go`, `internal/store/users.go`,
`internal/store/organizations.go`, `internal/auth/auth_test.go`,
`internal/auth/middleware_test.go`.

**Tasks:**
- [ ] `auth.Service` with: `Signup(ctx, in) (*User, error)`,
      `Login(ctx, email, password)`, `Logout(ctx, token)`,
      `VerifyEmail(ctx, token)`, `ResendVerification(ctx, email)`,
      `ResolveSession(ctx, token)`.
- [ ] `RequireSession` reads `sessions` directly (drops
      `app_sessions`).
- [ ] `RequireOrgAdmin` keeps its existing semantics (role check
      from the resolved user).
- [ ] `tokens.New()` / `tokens.Hash(raw)` — 32 bytes crypto/rand →
      hex; sha256 hash for storage.
- [ ] `passwordrules.Validate` — 8+ chars, upper/lower/digit (Sprint
      003 rule).
- [ ] `throttle` package: `Check(email)` returns `Locked(retryAfter)`
      or `OK`; `RecordFailure(email)` / `Clear(email)`.
- [ ] Tests: signup happy path; login generic error on bad creds;
      unverified can't log in; verification token consume-once;
      resend verification supersedes old; cooldown escalates and
      resets on success.

### Phase 4: Invitations + password mgmt + change email (~25%)

**Files:** `internal/auth/invitations.go`, `internal/auth/password.go`,
`internal/auth/email_change.go`, `internal/store/invitations.go`,
`internal/store/password_reset_tokens.go`,
`internal/store/email_change_requests.go`,
`internal/auth/invitations_test.go`, `internal/auth/password_test.go`,
`internal/auth/email_change_test.go`.

**Tasks:**
- [ ] Invitation service: `Invite(orgID, inviterID, email, role)`,
      `Lookup(token)`, `Accept(token, fullName, password)`,
      `ListPending(orgID)`, `Resend(orgID, invID)`,
      `Revoke(orgID, invID)`. Pending-only uniqueness handled by the
      partial unique index.
- [ ] Password service: `ForgotPassword(email)` (always 200 outward;
      issues a token only if the user exists),
      `ResetPassword(token, newPassword)` — invalidates all
      sessions and mints a fresh one for the caller,
      `ChangePassword(userID, current, new)` — invalidates all
      sessions EXCEPT the caller's.
- [ ] Email-change service: `RequestChange(userID, newEmail,
      currentPassword)`, `ConfirmChange(token)`. Old email stays
      authoritative until confirmation; on success swap
      `users.email` + `users.email_verified_at` atomically; supersede
      any previous unconsumed request; invalidate other sessions.
- [ ] Tests for every flow including the "what stays valid"
      invariants.

### Phase 5: Email service (~10%)

**Files:** `internal/email/smtp.go`, `message.go`, `templates.go`,
`templates/*.tmpl`, `email_test.go`, `fakesmtp_test.go`.

**Tasks:**
- [ ] `SMTPSender` over `net/smtp` with STARTTLS + PLAIN auth.
- [ ] Multipart/alternative builder: text + HTML, CRLF line endings,
      quoted-printable bodies, generated MIME boundary, correct
      `From`/`To`/`Subject`/`MIME-Version`/`Content-Type`.
- [ ] Four template kinds: verification, invitation, password_reset,
      change_email. Each with `subject`, `txt`, `html` files.
      `text/template` for text/subject; `html/template` for HTML.
      `//go:embed` so `bin/liveaboard` is self-contained.
- [ ] `MockSender` recorder for service tests (no network).
- [ ] `fakesmtp_test.go`: a hand-rolled in-process listener that
      accepts EHLO / AUTH PLAIN / MAIL FROM / RCPT TO / DATA / QUIT.
      Used to verify the SMTP wire format end-to-end.
- [ ] `email_test.go`: render each template against fixture vars;
      assert MIME structure (boundary parsable, both parts present,
      subject/recipient correct, action URL contains the
      `LIVEABOARD_APP_BASE_URL` prefix).

### Phase 6: HTTP routes + admin compatibility (~10%)

**Files:** `internal/httpapi/httpapi.go`, `auth_handlers.go`,
`invitation_handlers.go`, `cmd/server/main.go`,
`internal/httpapi/admin_test.go` (rewrite the admin-helpers to use
the new `signInAsAdmin` flow).

**Tasks:**
- [ ] Mount the 12 auth endpoints under `/api/*`.
- [ ] Sprint 008's `/api/admin/*` routes mount unchanged behind the
      same `s.Session.Wrap` + `auth.RequireOrgAdmin` middleware.
- [ ] `cmd/server/main.go` constructs `auth.Service`,
      `auth.InvitationService`, `email.SMTPSender`; drops Clerk
      wiring.
- [ ] Admin test harness (`signInAsAdmin`, `bootstrapDirector`)
      rewritten to use the new signup/verify/login flow against the
      `MockSender`. Helper signatures preserved so existing admin
      tests need minimal changes.

### Phase 7: Frontend swap (~15%)

**Files:** `web/src/main.tsx`, `web/src/lib/api.ts`,
`web/src/lib/RequireSession.tsx`, `web/src/pages/Signup.tsx`,
`web/src/pages/Login.tsx`, `web/src/pages/VerifyEmail.tsx`,
`web/src/pages/ForgotPassword.tsx`, `web/src/pages/ResetPassword.tsx`,
`web/src/pages/AcceptInvitation.tsx`,
`web/src/admin/pages/Account.tsx`, `web/src/admin/pages/Users.tsx`,
`web/package.json`.

**Tasks:**
- [ ] `npm uninstall @clerk/clerk-react`. `clerkAppearance.ts`
      already deleted in Phase 2.
- [ ] `<ClerkProvider>` removed from `main.tsx`. Drop the
      `signInFallbackRedirectUrl` etc props.
- [ ] Restore `Signup.tsx`, `Login.tsx`, `VerifyEmail.tsx` as plain
      forms. Login surfaces "Resend verification?" only when the
      backend returns `verification_required`.
- [ ] New: `ForgotPassword.tsx` (always shows "if an account exists,
      we sent a link" on submit, regardless of result).
- [ ] New: `ResetPassword.tsx` (token-based; on success the backend
      mints a fresh session, SPA navigates to `/admin`).
- [ ] New: `AcceptInvitation.tsx` (route
      `/invitations/:token/accept`; pre-fills the email read-only;
      collects full_name + password).
- [ ] New: `Account.tsx` mounted at `/admin/account` with two
      sections: change password + change email. Change-email shows
      "pending verification at new@example.com — resend / cancel".
- [ ] Wire `+ Invite` modal on `/admin/users` to
      `POST /api/invitations`; refresh table.
- [ ] `api.ts`: drop Clerk JWT plumbing; add the 12 auth methods.
- [ ] `RequireSession.tsx`: simplified to `/api/me` probe → redirect
      to `/login` on 401.

### Phase 8: dev-reset + smoke + docs (~10%)

**Files:** `scripts/dev_reset/main.go`, `RUNNING.md`,
`docs/auth.md`, `docs/decisions/0001-auth-provider.md`,
`docs/decisions/0002-revert-clerk.md` (new ADR).

**Tasks:**
- [ ] `dev_reset` simplified to local-only DB wipe.
- [ ] RUNNING.md updated with Brevo SMTP setup.
- [ ] `docs/auth.md` rewritten to describe the custom stack
      (env vars, template locations, token expiries, how to test
      locally).
- [ ] Mark `0001-auth-provider.md` as **Superseded by Sprint 009**.
- [ ] New `docs/decisions/0002-revert-clerk.md` — short ADR
      capturing the reversal rationale.
- [ ] **Live smoke** against the user's Brevo account:
      1. Signup → verification email arrives → click → login → land
         on dashboard.
      2. Admin invites a second address → accept → land in admin as
         site_director.
      3. Forgot password → email → reset → fresh session → all old
         sessions invalidated.
      4. Change email → confirmation email at new address → click →
         old email no longer logs in.

## API surface

```
POST /api/signup
POST /api/resend-verification
POST /api/verify-email
POST /api/login
POST /api/logout
POST /api/forgot-password
POST /api/reset-password
POST /api/change-password           (auth required)
POST /api/change-email              (auth required)
POST /api/change-email/confirm
GET  /api/me                        (auth required)        [unchanged]
GET  /api/organization              (auth required)        [unchanged]
POST /api/invitations               (admin only)
GET  /api/invitations               (admin only)
POST /api/invitations/{id}/resend   (admin only)
POST /api/invitations/{id}/revoke   (admin only)
GET  /api/invitations/by-token/{t}  (unauth)               metadata for accept page
POST /api/invitations/{token}/accept (unauth)              creates user; starts session

(All Sprint 008 /api/admin/* endpoints unchanged.)
```

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-009.md` | Create | This document. |
| `docs/sprints/drafts/SPRINT-009-*.md` | Create | Intent / drafts / critique / merge notes. |
| `docs/decisions/0001-auth-provider.md` | Modify | Mark superseded by Sprint 009. |
| `docs/decisions/0002-revert-clerk.md` | Create | Lightweight ADR capturing the reversal. |
| `docs/auth.md` | Rewrite | Custom-stack runbook. |
| `internal/store/migrations/0007_replace_clerk_with_custom_auth.sql` | Create | Schema unwind + new tables. |
| `internal/store/sessions.go` | Create | Re-add (replaces `app_sessions.go`). |
| `internal/store/app_sessions.go`, `webhook_events.go` | Delete | Clerk-specific. |
| `internal/store/email_verifications.go` | Create | Re-add. |
| `internal/store/invitations.go`, `password_reset_tokens.go`, `email_change_requests.go`, `login_attempts.go` | Create | New repos. |
| `internal/store/users.go`, `organizations.go` | Modify | Drop Clerk cols; restore password/verification fields. |
| `internal/auth/auth.go` | Create | Service. |
| `internal/auth/password.go`, `email_change.go`, `invitations.go` | Create | Domain services. |
| `internal/auth/middleware.go` | Modify | Direct sessions table. |
| `internal/auth/cookie.go`, `tokens.go`, `throttle.go`, `passwordrules.go` | Create/modify | Helpers. |
| `internal/auth/clerk.go`, `clerk_test.go`, `provider.go`, `stub.go`, `webhook.go`, `webhook_handlers.go`, `webhook_test.go`, `exchange.go`, `exchange_test.go` | Delete | Clerk-specific. |
| `internal/auth/admin.go` | Refactor | Use our invitation service. |
| `internal/email/*` | Create | Brevo SMTP service + 4 templates + tests. |
| `internal/config/config.go` | Modify | Add `LIVEABOARD_SMTP_*` + `AppBaseURL`; drop CLERK_*. |
| `internal/httpapi/httpapi.go`, `auth_handlers.go`, `invitation_handlers.go` | Modify/Create | Auth route mounts + handlers. |
| `internal/httpapi/admin_test.go` | Modify | Rewrite the sign-in helper. |
| `cmd/server/main.go` | Modify | Wire custom auth + email. |
| `scripts/dev_reset/main.go` | Modify | Local-only wipe. |
| `web/package.json`, `web/package-lock.json` | Modify | Drop `@clerk/clerk-react`. |
| `web/src/main.tsx`, `lib/api.ts`, `lib/RequireSession.tsx` | Modify | Drop Clerk plumbing; new auth endpoints. |
| `web/src/lib/clerkAppearance.ts` | Delete | — |
| `web/src/pages/Signup.tsx`, `Login.tsx`, `VerifyEmail.tsx` | Rewrite | Plain forms. |
| `web/src/pages/ForgotPassword.tsx`, `ResetPassword.tsx`, `AcceptInvitation.tsx` | Create | New pages. |
| `web/src/admin/pages/Account.tsx` | Create | Change password + change email. |
| `web/src/admin/pages/Users.tsx` | Modify | Wire `+ Invite` modal. |
| `RUNNING.md` | Modify | Brevo SMTP setup. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 009. |

## Definition of Done

- [ ] `clerk-archive` branch exists locally and on origin pointing
      at the pre-Sprint-009 main HEAD.
- [ ] `go.mod` no longer imports `clerk-sdk-go` or `svix-webhooks`
      (verified by `go mod tidy`).
- [ ] `git grep "clerk"` inside `internal/` returns nothing.
- [ ] `web/package.json` does not list `@clerk/clerk-react`.
- [ ] Migration `0007` applies cleanly on a fresh `liveaboard` AND
      on a pre-Sprint-009 `liveaboard_test` (which has Clerk-era
      rows). The destructive ordering is what makes the second case
      work.
- [ ] **Auth flows pass automated tests:**
      - Signup → verify → login → logout
      - Login generic error on bad creds + on inactive user
      - Login `verification_required` on unverified user (clean
        creds)
      - Login cooldown escalates and clears on success
      - Resend verification supersedes prior pending token
      - Forgot password → reset → all sessions invalidated → fresh
        session minted for caller
      - Change password → other sessions invalidated; current
        session still valid
      - Change email → request creates row, email goes to NEW
        address, old email still active until confirm; confirm
        atomically swaps + invalidates other sessions
- [ ] **Invitation flows pass automated tests:**
      - Create → accept → user has correct role + active session
      - Resend → new token, old superseded
      - Revoke → token rejected
      - Expired invitation → token rejected
      - Pending-only uniqueness enforced
- [ ] Email rendering tests assert subject/text/html for all four
      kinds against fixture vars.
- [ ] Hand-rolled SMTP listener test verifies multipart/alternative
      structure end-to-end.
- [ ] Sprint 008 admin chrome and `/api/admin/*` tests pass
      unchanged (helper rewrite is the only diff).
- [ ] **Live smoke against Brevo:**
      - Signup-to-dashboard cycle completes; verification email
        delivered to `mr.karim.fanous@gmail.com`.
      - Invite-to-acceptance cycle completes; invitation email
        delivered to a second inbox.
      - Forgot-password and change-email flows tested manually.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `make build`
      all clean.
- [ ] `CLAUDE.md`, `RUNNING.md`, `docs/auth.md` updated.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Brevo deliverability hiccups during smoke | Medium | Low | Use `mr.karim.fanous@gmail.com` as both sender + first recipient; if delivery fails, log the link and click it directly. |
| Migration 0007 breaks a Clerk-era dev DB mid-statement | High → mitigated | Low | Destructive ordering: rows DELETED before constraints tightened; tested against the user's actual current DB state. |
| Forgot-password enumeration leak | Medium | Medium | `/api/forgot-password` returns the same shape regardless of email match. |
| Brute-force defense leaks via timing | Medium | Low | Unknown emails still update `login_attempts` so the timing is identical. |
| SMTP creds end up in committed config | Low | High | Only the loader reads env; `make lint` already greps mode files for secret-shaped key=value; `.env.example` uses placeholders. |
| `Sprint 008 admin tests rewrite is invasive | Medium | Low | Keep `signInAsAdmin(t, h)` etc helper signatures; only the bodies change to use the new flow. |
| HTML email broken in some clients | Medium | Low | Inline styles + simple table layouts; manual smoke check in gmail. |
| Sender domain not verified for production | Medium | Low | Document the production domain-auth task in `docs/auth.md`; gmail-as-from is fine for dev. |
| Invitation `accepted_user_id` FK leaves orphaned rows after `users` deletion | Low | Low | `ON DELETE SET NULL` on the FK; invitation row preserved for audit. |
| Two-phase change-email leaves a "limbo" window | Low | Low | Clear UI: pending change shows on `/admin/account`. The user can cancel the pending request. |
| Race between two concurrent change-email requests for the same `new_email` from different users | Low | Medium | `email_change_requests.new_email UNIQUE` + uniqueness check at confirm time prevents both from succeeding. |

## Security Considerations

- **Password storage.** bcrypt cost 12 in production, 4 in tests
  (Sprint 004 config). Same as Sprint 003.
- **Token storage.** sha256-hashed in DB; raw token in URL only.
- **Token expiries.** 24h verification, 1h reset, 7d invitation, 1h
  email change. All consume-once.
- **Session invalidation:** Reset → all sessions revoked + fresh
  one minted. Change-password → all-other-sessions revoked. Change-
  email confirm → all-other-sessions revoked.
- **Email enumeration.** `/api/login` returns
  `invalid_credentials` for unknown email + bad password +
  inactive; only verified-but-unverified gets a different code AFTER
  a clean credential check. `/api/forgot-password` always 200.
  `/api/signup` 200 with verification-sent wording even for
  already-taken emails.
- **Brute force.** `login_attempts` per-email cooldown (1m → 5m →
  15m). Real distributed rate-limiter is a follow-up sprint when we
  add IP-based limiting.
- **CSRF.** SameSite=Lax cookie + JSON-only POSTs. Anti-CSRF tokens
  are a future sprint.
- **Production hardening.** `LIVEABOARD_COOKIE_SECURE=true`
  enforced; SMTP creds env-only; `AppBaseURL` must be `https://` in
  production (validated at startup).
- **No raw token in logs.** Email-send failures log `user_id` + the
  token *kind*, never the raw token or its hash.

## Dependencies

- **Brevo SMTP account** — already provisioned; creds in
  `.env.local`.
- **Sprint 003** — original custom auth design we're restoring.
- **Sprint 008** — admin chrome that this swap leaves intact.
- **No new Go module deps.** `net/smtp`, `crypto/rand`,
  `text/template`, `html/template` are stdlib;
  `golang.org/x/crypto/bcrypt` already in `go.mod`.
- **Drop Go module deps:** `github.com/clerk/clerk-sdk-go/v2`,
  `github.com/svix/svix-webhooks/go`. `go mod tidy` after Phase 2.
- **Drop npm dep:** `@clerk/clerk-react`.

## Out of Scope (captured as follow-ups)

- IP-based rate limiting / distributed rate-limiter middleware.
- MFA / TOTP / WebAuthn.
- Audit-log surface (`auth_events` table). For Sprint 009 we use
  structured `slog` for auth events.
- "Log out everywhere" / session-list UI.
- Account deletion (already deferred per personas.md).
- Domain-verified sender at Brevo for production. Captured in
  `docs/auth.md`.
- Anti-CSRF tokens.

## References

- Sprint 003 — `docs/sprints/SPRINT-003.md` (original custom auth).
- Sprint 005 — `docs/sprints/SPRINT-005.md` (Clerk migration; this
  sprint reverses it).
- Sprint 008 — `docs/sprints/SPRINT-008.md` (admin chrome that this
  sprint preserves).
- ADR — `docs/decisions/0001-auth-provider.md` (marked superseded).
- ADR — `docs/decisions/0002-revert-clerk.md` (created in Phase 8).
- Codex critique — `docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-009-MERGE-NOTES.md`.
