# Sprint 009 Merge Notes

## Claude Draft Strengths

- Detailed per-phase file table that maps every change to a concrete
  path. Easy to execute against.
- Risk register includes the "Sprint 008 admin tests rewrite is
  invasive" line, which Codex missed.
- Explicit per-token expiry table (24h verification, 1h reset, 7d
  invite) — preserved.
- Frontend page enumeration is exhaustive (Signup / Login /
  VerifyEmail / ForgotPassword / ResetPassword / AcceptInvitation /
  ChangePassword).
- Dependency call-out: no new Go deps, drops `clerk-sdk-go` +
  `svix-webhooks`, drops `@clerk/clerk-react`. Clean.

## Codex Draft Strengths

- **Migration 0007 ordering is sharper.** The destructive wipe must
  happen *before* tightening `password_hash` to NOT NULL, otherwise
  any Clerk-era dev DB fails the migration mid-statement. Adopt.
- **`email_change_requests` schema.** A separate table with `UNIQUE
  (new_email)` + a partial unique index on `user_id WHERE
  consumed_at IS NULL` is the right design. Old email stays
  authoritative until confirmation; swap is atomic on success. Adopt.
- **`login_attempts` table for brute-force defense.** Persistent and
  testable, vs my in-memory map which dies on restart. Adopt.
- **Invitation role check = site_director only.** The intent
  specifically says admin-to-site-director invitation; allowing
  `org_admin` adds review surface for nothing. Adopt.
- **Partial unique on `(organization_id, email)` for invitations**
  WHERE `accepted_at IS NULL AND revoked_at IS NULL`. Permits
  re-invite after acceptance/revocation; my `UNIQUE (org_id, email)`
  blocked legitimate re-invitations. Adopt.
- **Phase ordering**: archive + schema + config → unwind Clerk →
  core local auth + sessions → invitations + password + change-email
  → email service + verification. Cleaner separation than my
  archive-then-everything-else flow. Adopt.
- **Post-reset session UX recommendation**: create a fresh session
  immediately after successful password reset. Cleaner than forcing
  re-login. Adopt.
- **Post-change-email-confirm rotation**: invalidate other sessions
  on confirmation since email is a primary identifier. Adopt.

## Valid Critiques Accepted

1. **Change-email is in scope, not deferred.** My draft kept it as a
   future open question even though the planner's Phase 4 made it
   binding. Final sprint includes the full two-phase flow:
   `email_change_requests` table, `POST /api/change-email`,
   `POST /api/change-email/confirm`, frontend page, and template.
2. **Migration 0007 SQL must be explicitly destructive.** The actual
   SQL block now starts with `DELETE FROM users; DELETE FROM
   organizations;` — not relegated to prose. `users.password_hash`
   lands NOT NULL in the same migration.
3. **Invitation constraints tightened.** Role check is
   `('site_director')` only. Uniqueness uses a partial index keyed to
   pending invites.
4. **Brute-force defense is DB-backed.** New `login_attempts` table
   tracks per-email failure counts + lockout windows. In-memory map
   approach dropped.
5. **Change-email template added.** Email package now ships four
   templates: verification, invitation, password_reset, change_email.
6. **SMTP test strategy made explicit.** Service tests use a
   `MockSender` and assert message content (subject, recipient,
   token URLs); transport tests use a hand-rolled in-process SMTP
   listener and assert MIME structure. Two distinct concerns, two
   distinct test paths.
7. **`verification_required` machine-readable code.** Login response
   shape pinned: `{ "error": "invalid_credentials" }` on truly bad
   creds; `{ "error": "verification_required" }` on unverified
   accounts. The SPA shows a "resend verification" button only on
   the second case. Generic-error guarantee preserved at the wire
   level since both shapes are observable only after a clean
   credential check.
8. **DoD lists change-email + resend-verification explicitly.**

## Critiques Adjusted (not fully accepted)

- **Test SMTP server: hand-rolled vs library.** Codex left this open;
  I lock it in: hand-rolled minimal listener that accepts EHLO /
  AUTH PLAIN / MAIL FROM / RCPT TO / DATA / QUIT. ~150 LoC. No new
  test deps.

## Critiques Rejected (with reasoning)

None of Codex's findings were rejected outright. Each landed as
"accepted" or "adjusted."

## Interview Refinements Applied (already in intent doc)

| Question | Answer | Effect on plan |
|---|---|---|
| Migration aggression | Wipe legacy rows in 0007 | `DELETE FROM users; DELETE FROM organizations;` at the top of the migration; `password_hash` lands NOT NULL same migration. |
| Email verification | Required before login | Login of unverified account returns `verification_required`; no session issued. |
| Optional features | All four (resend verification, resend invitation, revoke invitation, change email) | All four ship in this sprint. Change email is the largest scope addition. |
| Env-var naming | `LIVEABOARD_SMTP_*` | Project convention. User updates `.env.local` once. |

## Final Decisions

- Migration `0007` is single, transactional, forward-only.
  Destructive ordering: DELETE rows → DROP tables → DROP cols →
  CREATE local-auth tables → ADD restored cols (NOT NULL) → preserve
  later-sprint schema. Boats/trips/sprint 008 schema untouched.
- Six new tables: `sessions`, `email_verifications`, `invitations`,
  `password_reset_tokens`, `email_change_requests`, `login_attempts`.
- Twelve auth endpoints under `/api/*` (signup, resend-verification,
  verify-email, login, logout, forgot-password, reset-password,
  change-password, change-email, change-email/confirm, invitations
  surface).
- New `internal/email` package: SMTPSender + multipart builder +
  4 templates + a hand-rolled test listener.
- New `login_attempts` table with cooldown schedule (1m → 5m → 15m).
- Frontend: drop `@clerk/clerk-react`, restore custom forms, add 4
  new pages.
- Sprint 008 admin chrome stays unchanged from the user's POV. The
  middleware swap underneath is a one-line behavioral change.
- Branch `clerk-archive` from current main HEAD; push to origin.
- Phasing: archive+schema+config → unwind clerk → core auth →
  invitations+password+change-email → email service → smoke + docs.
