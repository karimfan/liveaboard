# Sprint 009 Intent: Custom Auth Stack — Replace Clerk; Email via Brevo

## Seed

> I have a change of heart and want the minimum dependencies possible.
> We will use our own auth stack and not use clerk. You should place
> what we have in main now into a branch for posterity and undo what we
> did with clerk. Our auth stack should support creating a new org
> account with an email and password. It should support sending out
> invitations from the admin to the site directors via email. Site
> directors can then create their passwords. The auth stack should
> support resetting passwords if forgotten as well as changing them.
> Along with any other features you think are necessary. For sending
> emails, we can use Brevo. I already have an account.
> [Brevo SMTP creds: smtp-relay.brevo.com:587, JSA_ prefix vars]

## Context

Sprint 005 migrated to Clerk; Sprint 008 built the admin chrome on top
of it. The user now wants to reverse Sprint 005 specifically — keep
everything Sprint 006/007/008 built (boats, trips, admin IA, RBAC, the
admin endpoints) but rip Clerk out and replace it with a small,
self-hosted auth stack plus a Brevo-SMTP-backed email service.

This is a pure "swap" sprint. The chrome stays; what's underneath the
chrome changes. The admin's Sprint 008 surface (`/api/admin/*`) keeps
its shape — endpoints continue to be guarded by `RequireSession` /
`RequireOrgAdmin` middleware — only the cookie-issuance, signup,
invitation, and password-management mechanics underneath change.

## Recent Sprint Context

- **Sprint 005 — Migrate Authentication to Clerk.** Replaced Sprint
  003's bcrypt+session auth with Clerk-issued JWTs + a backend
  `lb_session` cookie minted via `/api/auth/exchange`. Added
  `clerk_user_id` / `clerk_org_id` linkage columns, an `app_sessions`
  bridge table, and a webhook receiver. **This sprint reverses it.**
- **Sprint 006 — Boat + Trip Scraper.** Added `boats` and `trips` and
  the dev-time scraper CLI. **Kept entirely.**
- **Sprint 007 — Admin UX (3 options, picked Control Tower).**
  Documents only. Kept entirely.
- **Sprint 008 — Admin Chrome + RBAC + Real Data.** Wired Sprint 007's
  IA to live data, role-gated chrome, 8 `/api/admin/*` endpoints, +
  `trips.site_director_user_id` column. **Kept entirely.** The
  endpoints' middleware (`RequireSession`, `RequireOrgAdmin`) keeps
  the same names; what changes is how `RequireSession` resolves the
  cookie underneath.

## Relevant Codebase Areas

| Area | What lives here today | What this sprint does |
|---|---|---|
| `internal/auth/clerk.go`, `provider.go`, `stub.go`, `webhook.go`, `webhook_handlers.go`, `webhook_test.go`, `exchange.go`, `exchange_test.go` | Clerk integration + abstraction | **Delete.** |
| `internal/auth/admin.go` | Sprint 005's invite/deactivate handlers (call Clerk admin SDK) | Refactor to use our own `invitations` table + email service. |
| `internal/auth/middleware.go`, `cookie.go` | Cookie helpers + `RequireSession` reading `app_sessions` | Keep cookie helpers; rewrite middleware to read `sessions` directly. |
| `internal/store/app_sessions.go` | Cookie ↔ Clerk-session bridge | **Delete**; replaced by simpler `sessions.go` (re-add). |
| `internal/store/users.go`, `organizations.go` | Has `clerk_user_id` / `clerk_org_id` NOT NULL | Drop those columns; re-add `password_hash` + `email_verified_at`. |
| `internal/store/migrations/0001…0006` | History so far | Add **0007** that undoes the Clerk-specific bits and adds custom-auth tables. Forward-only — no rewriting history. |
| `internal/config/config.go` | `CLERK_*` env keys | Replace with `LIVEABOARD_SMTP_*` (Brevo) + `LIVEABOARD_APP_BASE_URL` (for email links). |
| `cmd/server/main.go` | Wires Clerk provider + webhook into `httpapi.Server` | Wire custom auth service + email service. |
| `internal/httpapi/httpapi.go` | Mounts Clerk routes (`/api/signup-complete`, `/api/auth/exchange`, `/api/webhooks/clerk`, etc.) | Replace with `/api/signup`, `/api/login`, `/api/logout`, `/api/verify-email`, `/api/forgot-password`, `/api/reset-password`, `/api/change-password`, `/api/invitations`, `/api/invitations/{token}`, `/api/invitations/{token}/accept`. |
| `web/package.json` | `@clerk/clerk-react` dep | **Remove.** |
| `web/src/main.tsx`, `Signup.tsx`, `Login.tsx`, `RequireSession.tsx`, `clerkAppearance.ts` | Clerk UI integration | Restore custom forms (Sprint 003 style); add Forgot/Reset/AcceptInvitation/ChangePassword pages. |
| `scripts/dev_reset/main.go` | Wipes Clerk users + local rows | Simplify to local-only wipe. |
| **NEW** `internal/email/` | — | SMTP client + templates (verification, invitation, reset). |

## Constraints

- Must follow CLAUDE.md (work directly on main; no feature branches
  for ongoing work — but archiving the current state on a branch is
  explicitly requested by the user as part of this sprint).
- Stdlib + minimal deps. Reuse `golang.org/x/crypto/bcrypt` from
  Sprint 003. SMTP via `net/smtp` (stdlib). No HTML-templating
  library — `text/template` + `html/template` from stdlib are
  sufficient.
- Brevo SMTP credentials are secrets — they live in `.env.local`
  (gitignored). Production must source them from process env per the
  Sprint 004 config-loader rule.
- The Sprint 008 admin endpoint shapes do not change. Their auth
  middleware names (`RequireSession`, `RequireOrgAdmin`) and
  semantics do not change. Only the cookie-issuance and user-creation
  paths underneath are different.
- Existing dev databases will need a `make dev-reset` after migration
  0007 lands — old rows have NULL `password_hash` and can't log in
  without going through password reset. Dev-reset is the canonical
  way to start fresh.
- We are pre-customer. No real production users to migrate.

## Success Criteria

1. **Archive landed.** Branch `clerk-archive` exists locally and on
   origin pointing at the current main tip. The user can revisit any
   time.
2. **Migration 0007 applies cleanly.** Clerk artifacts dropped (cols,
   tables); custom-auth schema restored (`password_hash`,
   `email_verified_at`, `sessions`, `email_verifications`); two new
   tables (`invitations`, `password_reset_tokens`) land.
3. **Custom auth flows work end-to-end** in dev:
   - Sign up → email verification email arrives in inbox → click
     link → account active → login.
   - Admin sends invitation → invitee receives email → clicks link
     → sets password → logs in as Site Director.
   - Forgot password → email arrives → click link → set new password
     → previous sessions invalidated → login with new password.
   - Change password (signed in) → confirm with current password →
     other sessions invalidated → token cookie still valid.
4. **No `clerk` or `svix` import remains** in `internal/`.
   `git grep "clerk"` returns only docs / sprint history.
5. **`@clerk/clerk-react` is gone** from `web/package.json`. SPA still
   builds clean.
6. **Sprint 008 admin chrome unchanged from the user's point of view**
   — same screens, same RBAC behavior, different auth underneath.
7. **Brevo SMTP integration verified** — at least one email delivered
   to `mr.karim.fanous@gmail.com` during smoke. (Connectivity probe
   on startup is acceptable.)
8. **Tests pass.** Backend unit tests for auth flows; SMTP service
   unit-tested via a local dummy listener.
9. **CLAUDE.md updated** with the auth stack's operational notes
   (env vars, email-template location, how to test locally).

## Open Questions

1. **Token format for invitation / reset / verification.** Three
   reasonable options: (a) one shared `tokens` table with a `kind`
   column; (b) one table per kind (Sprint 003 used per-kind for
   `email_verifications`); (c) signed JWT-style tokens (no DB row).
   Recommendation: **per-kind tables**, mirroring Sprint 003's clean
   pattern. Easier to reason about expiry/lifecycle/audit per kind.
2. **Email verification: required before login, or grace?** Sprint
   003 required verification before login. Recommendation: **keep
   that** — it's the cleanest invariant.
3. **Invitation email + signup email: separate entry points?**
   Recommendation: yes — `/signup` is for an admin creating a new
   org; `/invitations/{token}/accept` is for an invitee. They share
   no UI.
4. **Brute-force defense.** Rate limiting and lockout were on the
   "we'll get to it" list when Sprint 005 swapped to Clerk. Now we
   own this surface. Minimal viable: **per-email login attempt
   counter with exponential cooldown**, plus generic error messages
   that don't enumerate. Capture proper rate-limit middleware as a
   follow-up sprint.
5. **Template rendering.** Plain-text emails or HTML+text multipart?
   Recommendation: **multipart with both** — HTML for normal mail
   clients, text fallback. Use stdlib `text/template` + `html/template`.
6. **Email sender domain.** Brevo creds use
   `mr.karim.fanous@gmail.com` as `JSA_SMTP_FROM`. For production
   we'd want a domain-verified sender. For dev, gmail-as-from is
   fine; flag the domain-auth task for production.
7. **Where does the "session table" live again?** Sprint 003's
   `sessions` table was dropped in migration 0004. We need to either
   recreate it (preferred, exact same shape) or reuse `app_sessions`
   stripped of Clerk fields. Recommendation: recreate `sessions`
   cleanly; drop `app_sessions`.
8. **Scope of admin invitation UI.** Sprint 008 disabled the `+
   Invite` button on the Users page. Does this sprint enable it
   end-to-end (form + endpoint + email + acceptance), or just the
   backend? Recommendation: **end-to-end** — invitations are the
   user's named must-have.
9. **`change email` flow.** Mentioned as "any other features you
   think are necessary." It's a real feature but adds re-verification
   complexity. Recommendation: **defer to a follow-up**; ship the
   five core flows (signup, login, verify, invite, reset) plus
   change-password.
10. **Audit log.** Auth events (failed logins, password resets,
    invitation acceptance) are useful evidence later. Recommendation:
    **defer** — add a `auth_events` table in a future sprint. Use
    structured `slog` for now so the events live in logs.

## Non-Goals

- Multi-factor authentication / TOTP / WebAuthn. Future sprint when
  there's a real customer.
- "Log out everywhere" / session-list UI. Future.
- Self-service account deletion. Already deferred per personas.md.
- Email-change re-verification flow. Captured as follow-up (see open
  question 9).
- Replacing the Sprint 008 admin chrome's IA or any of its endpoints'
  shapes.
- A full audit log surface. Captured as follow-up (open question 10).
- Migrating existing Clerk-side user data. None to migrate; this is a
  dev-only environment.

## Phase 4 Refinements (planner answers — binding)

Confirmed in the Phase 4 interview before Codex's draft. The final
sprint doc resolves the relevant Open Questions accordingly.

1. **Migration 0007 wipes legacy rows.** Pre-customer; clean slate.
   `DELETE FROM users; DELETE FROM organizations;` at the top of the
   migration. `users.password_hash` lands NOT NULL.
2. **Email verification required before login.** Sprint 003's
   invariant. Login returns a generic "invalid credentials" with
   intent + a separate "verification required" hint after a clean
   credential check. Unverified accounts cannot start a session.
3. **All four optional features ship in this sprint:**
   - Resend verification email (`POST /api/resend-verification`).
   - Resend pending invitation (`POST /api/invitations/{id}/resend`).
   - Revoke pending invitation (`POST /api/invitations/{id}/revoke`).
   - **Change email with re-verification.** Two-phase: authenticated
     user submits new email + current password →
     `email_change_requests` row + verification email sent to the
     NEW address → recipient clicks the link → `users.email` and
     `users.email_verified_at` swap to the new address. Old email
     remains active until that link is clicked. Implies a new
     `email_change_requests` table.
4. **Env-var naming: `LIVEABOARD_SMTP_*`** (project convention from
   Sprint 004). User updates their `.env.local` once.

## Recommended approach (for the drafts to evaluate)

Both drafts should propose:

- **Single migration 0007** that does the unwind + new tables in one
  transactional statement.
- **Five new auth endpoints** under `/api/auth/*` (or top-level
  matching Sprint 003) plus invitation endpoints.
- **A small `internal/email` package** with one `SMTPSender` type,
  one templating helper, and three template files.
- **Three new web pages** (`AcceptInvitation`, `ForgotPassword`,
  `ResetPassword`) plus a "Change password" section in the admin
  chrome's account area.
- **Verified end-to-end smoke** — at minimum, signup-to-dashboard
  and invite-to-acceptance run against a real Brevo inbox.
