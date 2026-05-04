# Auth — operational guide

This is the runbook for the self-hosted auth stack landed in Sprint 009.
For the why, see [`docs/decisions/0002-revert-clerk.md`](decisions/0002-revert-clerk.md).
The previous Clerk-backed iteration (Sprint 005) is preserved on the
`clerk-archive` branch.

## Components

- **`internal/auth`** — service layer (signup / verify / login / logout,
  forgot/reset/change password, invitations, change-email) plus the
  session middleware and the cookie helpers.
- **`internal/store`** — Postgres-backed persistence: `users`,
  `organizations`, `sessions`, `email_verifications`, `invitations`,
  `password_reset_tokens`, `email_change_requests`, `login_attempts`.
- **`internal/email`** — `Sender` interface, `MockSender` for tests, and
  `SMTPSender` (Brevo via `net/smtp` + STARTTLS).
- **`internal/httpapi`** — wires the service into HTTP routes under
  `/api/auth/*`, `/api/account/*`, `/api/invitations*`, `/api/me`,
  `/api/organization`.

## Local dev setup

1. Sign up at https://www.brevo.com (free tier covers ~300 mails/day).
2. *Senders & IPs → SMTP & API* — generate an SMTP key.
3. Drop the secrets into `.env.local` at the repo root (gitignored):

   ```
   LIVEABOARD_SMTP_HOST=smtp-relay.brevo.com
   LIVEABOARD_SMTP_PORT=587
   LIVEABOARD_SMTP_USERNAME=<your SMTP user>
   LIVEABOARD_SMTP_PASSWORD=<your SMTP key>
   LIVEABOARD_SMTP_FROM=<verified sender email>
   LIVEABOARD_APP_BASE_URL=http://localhost:5173
   ```

4. `make dev` boots the backend on `:8080` and Vite on `:5173`. Open
   http://localhost:5173/signup, fill the form, and watch your inbox
   for the verification email.

## Flows

| Flow                    | Public route                          | Authenticated? |
|-------------------------|---------------------------------------|----------------|
| Signup (creates org)    | `POST /api/auth/signup`               | No             |
| Verify email            | `POST /api/auth/verify-email`         | No (token)     |
| Resend verification     | `POST /api/auth/resend-verification`  | No             |
| Login                   | `POST /api/auth/login`                | No             |
| Logout                  | `POST /api/auth/logout`               | Yes (cookie)   |
| Forgot password         | `POST /api/auth/forgot-password`      | No             |
| Reset password          | `POST /api/auth/reset-password`       | No (token)     |
| Change password         | `POST /api/account/change-password`   | Yes            |
| Request email change    | `POST /api/account/request-email-change`   | Yes       |
| Confirm email change    | `POST /api/account/confirm-email-change`   | Token     |
| List pending email      | `GET  /api/account/pending-email-change`   | Yes       |
| Cancel email change     | `POST /api/account/cancel-email-change`    | Yes       |
| Invite Cruise Director  | `POST /api/invitations`               | Yes (admin)    |
| Resend invitation       | `POST /api/invitations/{id}/resend`   | Yes (admin)    |
| Revoke invitation       | `DELETE /api/invitations/{id}`        | Yes (admin)    |
| List pending invites    | `GET  /api/invitations`               | Yes (admin)    |
| Lookup invitation       | `GET  /api/invitations/lookup?token=` | No (token)     |
| Update profile          | `PATCH /api/account/profile`          | Yes (any role) |
| Cruise Director landing | `GET  /api/admin/cruise-director-overview` | Yes (CD-only) |
| Accept invitation       | `POST /api/invitations/accept`        | No (token)     |

## Invitation payload (Sprint 010)

The invitation `POST` body now includes name + optional phone:

```
POST /api/invitations
{
  "email":     "maya@example.com",
  "full_name": "Maya Sanchez",
  "phone":     "+1 555 0142",   // optional
  "role":      "cruise_director" // defaults if omitted
}
```

The accept page at `/invitations/<token>/accept` greets by name and
asks only for a password. The user row inherits `full_name` and
`phone` from the invitation. The recipient can edit either later from
`/admin/account → My profile`.

The `lookup` endpoint exposes `full_name` so the SPA can render the
greeting; it does **not** expose `phone`.

## Non-enumeration guarantees

`signup`, `forgot-password`, and `resend-verification` always return 200
with the same shape regardless of whether the email matches a real
account. Login distinguishes "wrong password" from "verification
required" only after a clean credential check, and emits
`ErrInvalidCredentials` for any state that should look identical to an
attacker (unknown email, inactive user, bad password).

## Tokens

Every flow that emails a link uses 64-char hex random tokens. Only the
sha256 is persisted; the raw token is the link itself, so a leaked DB
dump cannot reconstruct unconsumed links.

| Kind                 | Default TTL |
|----------------------|-------------|
| Email verification   | 24h         |
| Invitation           | 7d          |
| Password reset       | 1h          |
| Change-email confirm | 1h          |

## Throttle

`login_attempts` is incremented on every failed login keyed by email.
Cooldown schedule: 1–4 strikes free, 5 = 1 minute, 6 = 5 minutes, 7+ =
15 minutes. Successful login + password reset both clear the counter.

## Production checklist

- `LIVEABOARD_COOKIE_SECURE=true` (the loader rejects production startup
  otherwise).
- All SMTP creds + `LIVEABOARD_DATABASE_URL` come from the process env
  (the loader rejects production startup if a dotfile sources them).
- `LIVEABOARD_APP_BASE_URL` matches the public host the SPA is served
  from. Email links pin to it; misconfigure and links will 404.
- A reverse proxy strips `Cookie` from request logs, since the session
  cookie is bearer-equivalent.
