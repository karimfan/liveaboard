# Sprint 009: Custom Auth Stack — Replace Clerk; Email via Brevo

## Overview

Sprint 005 swapped custom auth for Clerk. The user has decided that
hosted-identity-provider risk + dependency surface isn't worth it for
this product, so this sprint replaces Clerk with a small self-hosted
auth stack that the team owns end-to-end. Email-driven flows (account
verification, invitation, password reset) become possible because
this sprint also introduces a Brevo SMTP integration — Brevo handles
deliverability so we don't have to host an SMTP server.

The product surface visible to users is identical: signup, login,
admin invites Site Director, password reset works, change password
works, the Sprint 008 admin chrome is unchanged. What changes is what
happens when those buttons are pressed: instead of Clerk's hosted
flow + a JWT exchange, the backend hashes a password with bcrypt,
issues an opaque session token via the existing `lb_session` cookie
contract, and sends operational emails through Brevo.

The implementation is largely a restoration of Sprint 003's auth
plumbing with three new tables (`invitations`, `password_reset_tokens`,
re-added `email_verifications`) and a new `internal/email` package.
The Sprint 008 admin chrome and the `/api/admin/*` endpoint shapes do
not change — Sprint 008's `RequireSession` and `RequireOrgAdmin`
middleware are renamed/refactored if needed, but the contract stays.

## Use Cases

1. **Org bootstrap.** A new operator goes to `/signup`, fills in
   email + password + name + organization name, gets a verification
   email, clicks the link, lands on the dashboard as an Org Admin.
2. **Login.** An existing user enters email + password at `/login`,
   gets a session cookie, lands on the dashboard.
3. **Logout.** Click "Log out" → session row deleted, cookie
   cleared, redirected to `/login`.
4. **Invite Site Director.** Org Admin clicks `+ Invite` on
   `/admin/users`, enters an email and role, the invitee receives an
   invitation email with a token-bearing URL.
5. **Accept invitation.** Invitee clicks the link, lands on
   `/invitations/{token}/accept`, sets their full name + password,
   logs in automatically.
6. **Resend or revoke invitation.** Admin can resend a pending
   invitation (regenerates the token + sends a fresh email) or revoke
   a pending one.
7. **Forgot password.** User goes to `/forgot-password`, enters
   email, receives a reset email; clicking the link lands them on
   `/reset-password/{token}` to set a new password. All existing
   sessions for that user are invalidated when the new password is
   set.
8. **Change password.** Authenticated user goes to
   `/admin/account/security` (or similar), enters current + new
   password, all OTHER sessions are invalidated; current session
   stays valid.
9. **Resend verification email.** A user who hasn't verified yet but
   tries to log in gets a clear "email not verified — resend?" prompt
   with a button.

## Architecture

### Schema (migration `0007`)

```sql
-- 0007_replace_clerk_with_custom_auth.sql

-- Drop Clerk integration tables.
DROP TABLE IF EXISTS app_sessions;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS auth_sync_cursors;

-- Drop Clerk linkage columns.
ALTER TABLE users         DROP COLUMN IF EXISTS clerk_user_id;
ALTER TABLE organizations DROP COLUMN IF EXISTS clerk_org_id;

-- Re-add Sprint-003 columns dropped by migration 0004.
ALTER TABLE users
    ADD COLUMN password_hash bytea NULL,
    ADD COLUMN email_verified_at timestamptz NULL;

-- Re-add Sprint-003 sessions/email_verifications tables.
CREATE TABLE sessions (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   bytea       NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
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

-- New tables for invitation + password-reset flows.
CREATE TABLE invitations (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id    uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email              citext      NOT NULL,
    role               text        NOT NULL CHECK (role IN ('org_admin','site_director')),
    invited_by_user_id uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash         bytea       NOT NULL UNIQUE,
    expires_at         timestamptz NOT NULL,
    accepted_at        timestamptz NULL,
    accepted_user_id   uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    revoked_at         timestamptz NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    -- Only one *pending* invitation per (org, email).
    UNIQUE (organization_id, email)
        DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX invitations_org_idx      ON invitations(organization_id);
CREATE INDEX invitations_email_idx    ON invitations(email);
CREATE INDEX invitations_expires_idx  ON invitations(expires_at);

CREATE TABLE password_reset_tokens (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens(user_id);
```

The `users.password_hash` column is created NULLABLE, then a follow-up
migration (`0008`) makes it NOT NULL after dev-reset wipes legacy
rows. Or — since we're pre-customer — just delete legacy rows in 0007
itself and create as NOT NULL. **Recommendation: delete + NOT NULL in
0007** (we're pre-customer; orphan Clerk-linked rows can't log in
anyway).

### Backend layout

```
internal/auth/
  auth.go             # Service: signup, login, verify, reset, change, accept-invite
  invitations.go      # Service: create, resend, revoke
  middleware.go       # Cookie-based RequireSession (kept; rewritten internals)
  cookie.go           # Cookie helpers (kept from Sprint 005; generic)
  admin.go            # /api/invitations handlers (refactored)
  password.go         # bcrypt + complexity validation
  tokens.go           # Token generation + hashing helpers
  auth_test.go        # Service-level tests (signup → verify → login → logout)
  invitations_test.go # Invitation flow tests
  middleware_test.go  # Cookie middleware tests
  password_test.go    # Complexity rule tests

internal/email/
  email.go            # SMTPSender interface + brevoSMTPSender impl
  templates.go        # Templates: verification, invitation, password reset
  templates/          # *.txt + *.html.tmpl files (text/template)
  email_test.go       # Multipart format + template render tests

internal/store/
  sessions.go         # Re-added (replaces app_sessions.go)
  email_verifications.go  # Re-added
  invitations.go      # New
  password_reset_tokens.go  # New
  users.go            # Add password_hash; remove clerk_user_id
  organizations.go    # Remove clerk_org_id

internal/store/migrations/
  0007_replace_clerk_with_custom_auth.sql  # The unwind

internal/config/
  config.go           # Add SMTP* + AppBaseURL; remove CLERK_*

internal/httpapi/
  httpapi.go          # Replaced auth routes; admin chrome routes unchanged

cmd/server/main.go    # Wires custom auth + email; drops Clerk wiring

scripts/dev_reset/
  main.go             # Local-only wipe (no Clerk API calls)
```

### Public HTTP surface

```
POST /api/signup                        — body: {email, password, full_name, organization_name}
                                           Creates org + first user (org_admin); issues verification token via email.
POST /api/login                         — body: {email, password}
                                           Issues session cookie. Generic error on bad creds; "verify_email_required" if unverified.
POST /api/logout                        — invalidates session row + clears cookie.
POST /api/verify-email                  — body: {token}
                                           Marks user.email_verified_at; idempotent.
POST /api/resend-verification           — body: {email}
                                           Always 200 (non-enumerating). Issues a new token if user exists + unverified.
POST /api/forgot-password               — body: {email}
                                           Always 200 (non-enumerating). Issues reset token if user exists.
POST /api/reset-password                — body: {token, new_password}
                                           Sets new password; invalidates all existing sessions.
POST /api/change-password               — body: {current_password, new_password}; auth required.
                                           Verifies current; sets new; invalidates other sessions.

GET  /api/me                            — same as today (returns auth'd user).
GET  /api/organization                  — same as today.

POST /api/invitations                   — admin only; body: {email, role}
                                           Generates token + sends invitation email.
GET  /api/invitations                   — admin only; lists pending invitations for org.
POST /api/invitations/{id}/resend       — admin only; reissues token + email.
POST /api/invitations/{id}/revoke       — admin only; sets revoked_at.

GET  /api/invitations/by-token/{token}  — UNAUTH; returns {email, role, org_name} for the accept page.
POST /api/invitations/{token}/accept    — UNAUTH; body: {full_name, password}
                                           Creates user row; consumes invitation; issues session cookie.

(All Sprint 008 /api/admin/* endpoints unchanged.)
```

### Email service

```go
package email

type Sender interface {
    Send(ctx context.Context, msg Message) error
}

type Message struct {
    To       string  // single recipient for our flows
    Subject  string
    TextBody string
    HTMLBody string  // optional; if empty, send text-only
}

type SMTPSender struct {
    Host     string
    Port     int
    Username string
    Password string
    From     string
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error { /* net/smtp PLAIN auth + STARTTLS */ }
```

Templates live in `internal/email/templates/`:

```
templates/
  verification.subject.tmpl   "Verify your email for {{.OrgName}}"
  verification.txt.tmpl
  verification.html.tmpl
  invitation.subject.tmpl     "{{.InviterName}} invited you to {{.OrgName}}"
  invitation.txt.tmpl
  invitation.html.tmpl
  password_reset.subject.tmpl "Reset your password"
  password_reset.txt.tmpl
  password_reset.html.tmpl
```

Templates are embedded with `//go:embed` so `bin/liveaboard` is
self-contained.

### Token strategy

- Per-kind tables: `email_verifications`, `password_reset_tokens`,
  `invitations`. Each row stores `token_hash = sha256(raw_token)`;
  the raw token only exists in the email link.
- Tokens are 32 bytes of `crypto/rand` → hex (64 chars).
- Default expiries:
  - email verification: **24h**
  - password reset: **1h**
  - invitation: **7 days**
- All consume-once via `consumed_at` (or for invitations,
  `accepted_at` / `revoked_at`).
- The token in the URL is lowercase hex; routes accept it as a path
  param (`/verify-email?token=...`) for verification + reset, and
  `/invitations/{token}/accept` for invites.

### Cookie / session

- Same as Sprint 003's design: opaque random token, sha256 in DB,
  HTTP-only `lb_session` cookie, `Secure` flag in prod.
- Default session TTL **14 days**; `last_seen_at` bumped on every
  `RequireSession` call.

### Config additions

| Field | Env | Default | Notes |
|---|---|---|---|
| `SMTPHost` | `LIVEABOARD_SMTP_HOST` | `smtp-relay.brevo.com` | non-secret |
| `SMTPPort` | `LIVEABOARD_SMTP_PORT` | `587` | non-secret |
| `SMTPUsername` | `LIVEABOARD_SMTP_USERNAME` | (none) | **secret** |
| `SMTPPassword` | `LIVEABOARD_SMTP_PASSWORD` | (none) | **secret** |
| `SMTPFrom` | `LIVEABOARD_SMTP_FROM` | (none) | non-secret; sender address |
| `AppBaseURL` | `LIVEABOARD_APP_BASE_URL` | `http://localhost:5173` | for email links |

The user's `JSA_*` Brevo creds get renamed to `LIVEABOARD_SMTP_*` in
their `.env.local`. The `JSA_*` form is from another project.

Removed:
- `CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `CLERK_WEBHOOK_SECRET`,
  `VITE_CLERK_PUBLISHABLE_KEY` — drop from `Config`, `.env.example`,
  `config/*.env`. The user removes them from their `.env.local`.

### Frontend layout

```
web/src/
  main.tsx                  — drop ClerkProvider + clerkAppearance
  pages/
    Signup.tsx              — restored: email + password + name + org form; shows verification token in dev
    Login.tsx               — restored: email + password form
    VerifyEmail.tsx         — restored: consumes ?token=
    ForgotPassword.tsx      — NEW: email entry → "if account exists, email sent"
    ResetPassword.tsx       — NEW: ?token=...; new password + confirm
    AcceptInvitation.tsx    — NEW: /invitations/:token/accept; shows org+role; sets full_name + password
    ChangePassword.tsx      — NEW: lives within admin chrome at /admin/account
  lib/
    api.ts                  — drop Clerk JWT plumbing; add forgot/reset/change/invite endpoints
    RequireSession.tsx      — same shape (probe /api/me); drop Clerk fallback
  admin/
    pages/Users.tsx         — wire the "+ Invite" button to a modal that posts /api/invitations
  styles/                   — unchanged
```

Removed: `clerkAppearance.ts`. Removed dep: `@clerk/clerk-react`.

## Implementation Plan

### Phase 1: Archive + scaffolding (~5%)

**Tasks:**
- [ ] `git branch clerk-archive main` and `git push origin
      clerk-archive`. (User explicitly requested archival.)
- [ ] Add `LIVEABOARD_SMTP_*` + `LIVEABOARD_APP_BASE_URL` to
      `internal/config/config.go` (mark SMTP creds `secret:"true"`).
- [ ] Update `.env.example` with the new keys; remove CLERK_*.
- [ ] Update `config/dev.env` with the non-secret defaults; remove
      VITE_CLERK_PUBLISHABLE_KEY.
- [ ] CLAUDE.md: add a short "Auth stack" section pointing at the
      future `docs/auth.md`.

### Phase 2: Schema unwind (~10%)

**Files:** `internal/store/migrations/0007_replace_clerk_with_custom_auth.sql`,
`internal/testdb/testdb.go`, `internal/store/users.go`,
`internal/store/organizations.go`.

**Tasks:**
- [ ] Write migration `0007` per the schema above. **Drop existing
      user/org rows** at the top of the migration (pre-customer); add
      `password_hash` and `email_verified_at` as NOT NULL afterward.
- [ ] Update `testdb.Pool` truncate list: drop `app_sessions`,
      `webhook_events`, `auth_sync_cursors`; add `sessions`,
      `email_verifications`, `invitations`,
      `password_reset_tokens`.
- [ ] Update `User` struct: drop `ClerkUserID`; add `PasswordHash
      []byte`, `EmailVerifiedAt *time.Time`.
- [ ] Update `Organization` struct: drop `ClerkOrgID`.
- [ ] Update existing `users.go` / `organizations.go` queries;
      remove `BoatBySourceSlug` confusions; ensure `userColumns`
      matches the new schema.

### Phase 3: Custom auth service + repos (~25%)

**Files:** `internal/auth/auth.go`, `internal/auth/invitations.go`,
`internal/auth/middleware.go`, `internal/auth/cookie.go` (kept),
`internal/auth/password.go`, `internal/auth/tokens.go`,
`internal/store/sessions.go`, `internal/store/email_verifications.go`,
`internal/store/invitations.go`,
`internal/store/password_reset_tokens.go`, plus tests.

**Tasks:**
- [ ] Implement `auth.Service` with: `Signup`, `Login`,
      `Logout(token)`, `VerifyEmail(token)`,
      `ResendVerification(email)`, `ForgotPassword(email)`,
      `ResetPassword(token, newPassword)`,
      `ChangePassword(userID, currentPassword, newPassword)`,
      `ResolveSession(token)`.
- [ ] Implement `auth.InvitationService` with: `Invite(orgID,
      inviterID, email, role)`, `Lookup(token)`,
      `Accept(token, fullName, password)`,
      `ListPending(orgID)`, `Resend(orgID, invID)`,
      `Revoke(orgID, invID)`.
- [ ] Implement `password.Hash`, `password.Compare`,
      `password.Validate` (8+ chars, upper/lower/digit — same as
      Sprint 003).
- [ ] Implement `tokens.New()` (returns raw + hash) and
      `tokens.Hash(raw)`.
- [ ] Reimplement `RequireSession` middleware against the new
      `sessions` table.
- [ ] Implement repos: `sessions` (Create / ByTokenHash / Delete /
      DeleteForUser / DeleteForUserExcept(token)),
      `email_verifications`, `invitations`,
      `password_reset_tokens`.
- [ ] Tests:
      - `auth_test.go`: full happy path, generic error on bad
        creds, unverified-account-cannot-login, password-complexity
        rejection, `ChangePassword` invalidates other sessions but
        keeps current, `ResetPassword` invalidates all sessions,
        verification token consume-once.
      - `invitations_test.go`: create → email queued (asserted via
        in-memory mock sender) → accept → user created → can log
        in; resend → new token, old revoked; revoke; expired
        invitation rejected.

### Phase 4: Email service (~10%)

**Files:** `internal/email/email.go`,
`internal/email/templates.go`, `internal/email/templates/*.tmpl`,
`internal/email/email_test.go`.

**Tasks:**
- [ ] `Sender` interface + `SMTPSender` impl using `net/smtp` with
      PLAIN auth + STARTTLS. Inject `Dialer` for tests.
- [ ] Multipart message builder: text + HTML alternative; correct
      MIME headers; quoted-printable encoding for both parts.
- [ ] In-memory `MockSender` for service tests (records every send).
- [ ] Three templates (verification, invitation, password reset),
      each as `.subject.tmpl` + `.txt.tmpl` + `.html.tmpl`. Embed via
      `//go:embed`.
- [ ] `email.Build(kind, vars)` returns a `Message` ready for
      `Sender.Send`.
- [ ] Tests: render each template against fixtures; SMTP-format
      tests against a local `net.Listener` capturing the wire bytes;
      verify TO/FROM/SUBJECT/multipart structure.

### Phase 5: HTTP wiring + Sprint 008 admin compatibility (~15%)

**Files:** `internal/httpapi/httpapi.go`,
`internal/httpapi/auth_handlers.go` (new file consolidating the
auth routes), `internal/httpapi/invitation_handlers.go`,
`cmd/server/main.go`.

**Tasks:**
- [ ] Mount the new `/api/*` routes per the Public HTTP surface
      table above. Reuse Sprint 008's middleware names
      (`s.Session.Wrap`, `auth.RequireOrgAdmin`).
- [ ] Drop Clerk-related routes (`/api/signup-complete`,
      `/api/auth/exchange`, `/api/webhooks/clerk`).
- [ ] `cmd/server/main.go`: replace Clerk + webhook construction
      with `auth.Service`, `auth.InvitationService`, `email.SMTPSender`.
- [ ] Sprint 008's `/api/admin/*` endpoints stay as-is — only the
      auth wiring underneath changes.
- [ ] Update `httpapi_test.go` harness: drop Clerk stub plumbing;
      add a `MockSender` for the email service; helper that signs in
      via `/api/signup` → `/api/verify-email` → `/api/login` for the
      admin-test fixtures.

### Phase 6: Frontend swap (~20%)

**Files:** `web/src/main.tsx`, `web/src/pages/Signup.tsx`,
`web/src/pages/Login.tsx`, `web/src/pages/VerifyEmail.tsx`,
`web/src/pages/ForgotPassword.tsx`,
`web/src/pages/ResetPassword.tsx`,
`web/src/pages/AcceptInvitation.tsx`,
`web/src/admin/pages/ChangePassword.tsx`,
`web/src/admin/pages/Users.tsx` (wire `+ Invite` modal),
`web/src/lib/api.ts`, `web/src/lib/RequireSession.tsx`,
`web/src/lib/clerkAppearance.ts` (DELETE), `web/package.json`,
`docs/auth.md` (rewrite).

**Tasks:**
- [ ] `npm uninstall @clerk/clerk-react`.
- [ ] Drop `<ClerkProvider>` and `clerkAppearance.ts`. Drop the
      `signInFallbackRedirectUrl` etc props.
- [ ] Restore `Signup.tsx`, `Login.tsx`, `VerifyEmail.tsx` as plain
      forms (Sprint 003 style).
- [ ] Add `ForgotPassword.tsx`, `ResetPassword.tsx`,
      `AcceptInvitation.tsx`.
- [ ] Add a `+ Invite` modal on `/admin/users`: form with email +
      role; calls `POST /api/invitations`; refreshes the table.
- [ ] Add a `ChangePassword` page mounted at `/admin/account` with
      a sidebar nav entry "Account" near the footer.
- [ ] `api.ts`: drop Clerk JWT plumbing (no more `signupComplete` /
      `exchange`); add `forgotPassword`, `resetPassword`,
      `changePassword`, `acceptInvitation`, `lookupInvitation`.
- [ ] `RequireSession.tsx`: simplify to "probe `/api/me`; on 401,
      redirect to `/login`".

### Phase 7: dev-reset + smoke + docs (~15%)

**Files:** `scripts/dev_reset/main.go` (simplify),
`docs/auth.md` (rewrite), `RUNNING.md` (update),
`docs/decisions/0001-auth-provider.md` (deprecate; add a note that
the decision was reversed in Sprint 009).

**Tasks:**
- [ ] Simplify `scripts/dev_reset` to local-only DB wipe; drop
      Clerk SDK calls.
- [ ] Update `make dev-reset` description.
- [ ] Rewrite `docs/auth.md` to describe the custom stack (env
      vars, email templates, token expiries, how to test locally).
- [ ] Update `RUNNING.md` with the new Brevo SMTP setup steps.
- [ ] Mark `docs/decisions/0001-auth-provider.md` as superseded;
      reference Sprint 009 as the reversal.
- [ ] **Live smoke** against the user's Brevo account:
      signup → verification email arrives → click link → login →
      send invitation to a second address → click → accept →
      forgot-password flow → reset → change-password.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-009.md` | Create | Final sprint doc. |
| `docs/sprints/drafts/SPRINT-009-INTENT.md` | Create | Intent (already exists). |
| `docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md` | Create | This document. |
| `docs/sprints/drafts/SPRINT-009-CODEX-DRAFT.md` | Create | Codex's competing proposal. |
| `docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT-CODEX-CRITIQUE.md` | Create | Codex's critique. |
| `docs/sprints/drafts/SPRINT-009-MERGE-NOTES.md` | Create | Synthesis. |
| `docs/auth.md` | Rewrite | Custom-stack runbook. |
| `docs/decisions/0001-auth-provider.md` | Update | Mark superseded. |
| `internal/store/migrations/0007_replace_clerk_with_custom_auth.sql` | Create | Schema unwind + new tables. |
| `internal/store/sessions.go` | Create | Re-add (replaces app_sessions). |
| `internal/store/app_sessions.go` | Delete | Superseded. |
| `internal/store/email_verifications.go` | Create | Re-add. |
| `internal/store/invitations.go` | Create | New repo. |
| `internal/store/password_reset_tokens.go` | Create | New repo. |
| `internal/store/users.go`, `organizations.go` | Modify | Drop Clerk cols; add password_hash etc. |
| `internal/store/webhook_events.go` | Delete | No longer used. |
| `internal/auth/auth.go` | Create | Service. |
| `internal/auth/invitations.go` | Create | Invitation service. |
| `internal/auth/password.go`, `tokens.go` | Create | Helpers. |
| `internal/auth/middleware.go` | Modify | Read sessions table directly. |
| `internal/auth/cookie.go` | Keep | Generic cookie helpers. |
| `internal/auth/admin.go` | Refactor | Use our invitation service. |
| `internal/auth/clerk.go`, `provider.go`, `stub.go`, `webhook.go`, `webhook_handlers.go`, `webhook_test.go`, `exchange.go`, `exchange_test.go` | Delete | Clerk-specific. |
| `internal/email/email.go`, `templates.go`, `templates/*.tmpl`, `email_test.go` | Create | Brevo SMTP service. |
| `internal/config/config.go` | Modify | Add SMTP* + AppBaseURL; drop CLERK_*. |
| `internal/httpapi/httpapi.go` | Modify | Replace auth routes; admin chrome stays. |
| `internal/httpapi/auth_handlers.go` | Create | New auth route handlers. |
| `internal/httpapi/invitation_handlers.go` | Create | Invitation handlers. |
| `internal/httpapi/admin_test.go` | Modify | Use the new sign-in fixture. |
| `cmd/server/main.go` | Modify | Wire custom auth + email. |
| `web/package.json`, `web/package-lock.json` | Modify | Remove `@clerk/clerk-react`. |
| `web/src/main.tsx` | Modify | Drop ClerkProvider. |
| `web/src/lib/clerkAppearance.ts` | Delete | — |
| `web/src/lib/api.ts`, `RequireSession.tsx` | Modify | Drop Clerk JWT plumbing. |
| `web/src/pages/Signup.tsx`, `Login.tsx`, `VerifyEmail.tsx` | Rewrite | Plain forms. |
| `web/src/pages/ForgotPassword.tsx`, `ResetPassword.tsx`, `AcceptInvitation.tsx` | Create | New pages. |
| `web/src/admin/pages/ChangePassword.tsx` | Create | Authenticated change-password. |
| `web/src/admin/pages/Users.tsx` | Modify | Wire `+ Invite` modal. |
| `scripts/dev_reset/main.go` | Modify | Local-only wipe. |
| `RUNNING.md` | Modify | Brevo SMTP setup. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 009. |

## Definition of Done

- [ ] `clerk-archive` branch exists locally and on origin pointing
      at the pre-Sprint-009 main HEAD.
- [ ] `go.mod` no longer imports `clerk-sdk-go` or `svix-webhooks`.
- [ ] `git grep "clerk"` inside `internal/` returns nothing
      (docs / sprint history are fine).
- [ ] `web/package.json` does not list `@clerk/clerk-react`.
- [ ] Migration 0007 applies cleanly on a fresh `liveaboard` and
      `liveaboard_test`.
- [ ] All seven new auth flows pass automated tests (signup, login,
      logout, verify, forgot, reset, change).
- [ ] Invitation flow passes automated tests (create → accept,
      resend, revoke, expired).
- [ ] Email templates render against fixture vars.
- [ ] SMTP wire format verified by an `email_test.go` that captures
      raw bytes against a local listener.
- [ ] Sprint 008's admin chrome works unchanged from the user's POV
      (admin endpoints + RBAC test suite still passes).
- [ ] Live smoke against Brevo: signup-to-dashboard cycle and
      invite-to-acceptance cycle send real emails to two real
      inboxes.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `make build`
      all clean.
- [ ] CLAUDE.md / RUNNING.md / docs/auth.md updated.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Brevo deliverability hiccups during smoke | Medium | Low | Test with the user's gmail address (already sender + recipient); fall back to console-logging the link in dev if SMTP fails. |
| Migration 0007 breaks existing dev DBs | High → mitigated | Low | Migration explicitly DELETEs legacy rows pre-customer; user runs `make dev-reset` after. Documented in RUNNING.md. |
| Token-table sprawl (3 separate tables) | Low | Low | Each table has the same shape and a single repo file; the symmetry is intentional and documented. |
| Forgot-password enumeration leak | Medium | Medium | `/api/forgot-password` always returns 200 with the same response shape regardless of email existence. |
| Brute-force login attacks | Medium | High | Per-email cooldown after N failed attempts (count in a small in-memory map; reset after success or N minutes). Documented as a follow-up to add a real rate-limiter sprint. |
| SMTP creds end up in committed config files | Low | High | Only the loader reads env; lint already greps mode files for secret-shaped key=value; `.env.example` lists the keys but with placeholder values. |
| Sprint 008 admin tests rewrite is invasive | Medium | Low | Keep the helper signatures (`signInAsAdmin(t, h)` etc) the same; only the body changes. |
| Sender domain not verified at Brevo for production | Medium | Low | Document the production task in `docs/auth.md`; for dev, gmail-as-from is fine because Brevo accepts it. |
| `docs/decisions/0001-auth-provider.md` becomes lying-by-omission | Low | Low | Add a "Status: Superseded by Sprint 009" header to the ADR and a short paragraph explaining the reversal. |
| HTML email looks broken in some clients | Medium | Low | Stick to inline styles + simple table layout; render in Litmus or in user's gmail during smoke. |

## Security Considerations

- **Password storage.** bcrypt cost 12 in production; cost 4 in tests
  (per Sprint 004 config). Same as Sprint 003.
- **Token storage.** Only sha256 hashes hit the DB. Raw tokens live
  in URLs only. All tokens are 32 bytes from `crypto/rand`.
- **Token expiry.** 24h for verification, 1h for reset, 7d for
  invitation. All consume-once.
- **Session invalidation.** Reset password ⇒ all sessions revoked.
  Change password ⇒ all sessions EXCEPT current revoked. Logout ⇒
  current session row deleted.
- **Email enumeration.** Login uses generic "invalid credentials" for
  unknown email + bad password + unverified email + deactivated.
  `/api/forgot-password` always 200. `/api/signup` 200 with
  "verification email sent" wording even for already-taken emails
  (Sprint 003 already handled this).
- **Brute force.** Minimal per-email cooldown after 5 failed login
  attempts in 15 minutes (in-memory map; resets on success). Real
  rate-limiter is a follow-up sprint.
- **Production hardening.** `LIVEABOARD_COOKIE_SECURE=true` enforced
  (already from Sprint 004); SMTP creds env-only; `AppBaseURL` must
  be `https://` in production. `Config.validate()` enforces.
- **CSRF.** SameSite=Lax cookie + JSON-only POSTs is sufficient at
  this scale. Explicit anti-CSRF tokens are a future sprint when we
  add cross-origin browser tools.
- **Token leakage via referer.** Reset/verification links are in the
  URL; the SPA strips `?token=...` from history after consumption to
  reduce browser-cache exposure. Minor; documented.

## Dependencies

- **Brevo SMTP account** — provided.
- **Sprint 003** — original custom auth design we're restoring.
- **Sprint 008** — admin chrome that consumes the new middleware.
- **No new Go module deps.** `net/smtp` is stdlib;
  `golang.org/x/crypto/bcrypt` already in `go.mod`.
- **Drop Go module deps:** `github.com/clerk/clerk-sdk-go/v2`,
  `github.com/svix/svix-webhooks/go`. Run `go mod tidy`.
- **Drop npm dep:** `@clerk/clerk-react`.

## Open Questions

1. **Single migration or two?** A single 0007 doing both unwind +
   new tables is simpler. Recommendation: single.
2. **`change_email` flow** — defer to a follow-up. Captured here.
3. **MFA / 2FA** — defer.
4. **Audit log** — defer; use `slog` for now.
5. **Rate-limit middleware** — defer; per-email cooldown is enough
   for Sprint 009.
6. **`docs/decisions/0002-revert-clerk.md`** ADR? Not strictly
   needed (the sprint doc itself records the reversal), but a small
   ADR is nice. Recommendation: yes, lightweight.
