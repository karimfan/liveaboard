# ADR 0002: Revert to Self-Hosted Auth

**Status:** Accepted (Sprint 009, 2026-05-01)
**Supersedes:** [ADR 0001 — Authentication Provider (Clerk)](0001-auth-provider.md)

## Context

ADR 0001 chose Clerk to short-cut the email-driven flows (verification,
invitation, password reset) that the Sprint 003 custom stack lacked.
After landing Sprint 005 (Clerk integration) and operating it through
Sprint 008, the project's priorities shifted toward minimum dependencies
and full operational ownership. Concrete pain points:

- Two SDKs (`clerk-sdk-go`, `svix-webhooks`) plus a frontend package
  added enough surface area to dwarf the rest of the auth code.
- The webhook reconciliation path (Clerk -> us) introduced a class of
  failure modes (out-of-order events, signature drift, partial state)
  that have no equivalent in a self-contained model.
- Production cost / vendor risk: Clerk's free tier is generous but every
  added user pulls the project closer to a paid contract for a problem
  we are now confident we can own.
- Email delivery is a solved commodity (Brevo, SES, etc.); a custom
  invitation/verification token model is small and well-understood.

## Decision

Replace Clerk with a custom-auth stack:

- **Sessions:** opaque cookie tokens (sha256 in DB, HttpOnly, SameSite=Lax).
- **Passwords:** bcrypt cost 12 prod / 4 test.
- **Per-kind token tables:** `email_verifications`, `invitations`,
  `password_reset_tokens`, `email_change_requests`. Tokens are 64-char
  hex; only the sha256 lives in the DB.
- **Throttle:** DB-backed `login_attempts` with a graduated cooldown
  (1m → 5m → 15m).
- **Email:** Brevo SMTP via `net/smtp` + STARTTLS + PLAIN; multipart
  text+HTML messages assembled by hand.
- **Flows:** signup (org+admin), email verification, login, logout,
  forgot/reset/change password, invite/resend/revoke/accept, two-phase
  change-email with re-verification.

The frontend drops `@clerk/clerk-react` entirely and uses plain HTML
forms posting to the new `/api/auth/*` and `/api/account/*` endpoints.

## Consequences

**Positive**

- One fewer external dependency to monitor, version, and pay for.
- No webhook drift class of bugs; user/org state is local-only.
- Auth surface is small enough to reason about end-to-end.

**Negative**

- We now own SMTP delivery monitoring, anti-abuse, and password-policy
  evolution. Brevo handles deliverability; everything else is on us.
- No social login. A future ADR can add OAuth providers if needed.
- Test surface grew (per-flow integration tests instead of a stub
  provider).

## Migration

Migration `0007_replace_clerk_with_custom_auth.sql` is destructive: it
deletes every existing row in `users` / `organizations` / `trips` /
`boats` / Clerk webhook tables before changing constraints, then adds
the new columns and tables. The pre-revert state is preserved on the
`clerk-archive` git branch.
