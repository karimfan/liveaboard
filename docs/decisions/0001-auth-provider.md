# ADR 0001: Authentication Provider

**Status:** Accepted (Sprint 005, 2026-05-01)

## Context

Sprint 003 shipped a custom auth stack (bcrypt + opaque session tokens
in HTTP-only cookies). Several Must-priority backlog items immediately
on deck — password reset (US-1.4), invite Site Director (US-6.1),
deactivate user (US-6.2), resend invitation (US-6.3) — require email
delivery, transactional templates, an invitation lifecycle, and rate
limiting. None of those exist in the custom stack.

Decision point: extend custom auth (estimated 2–3 sprints to feature
parity, plus permanent ownership of a security surface) or migrate to a
third-party identity provider.

## Decision

**Buy. Adopt Clerk.**

Clerk fits this product unusually well:

- First-class **Organizations** primitive that maps directly to our
  `organizations` table and maintenance-free invitation flows for
  US-6.1 / US-6.3.
- **React + Go SDKs** with active maintenance.
- **JWKS-based session verification** that does not require a network
  round-trip per request after warm-up.
- Mature hosted UI components (`<SignIn>`, `<SignUp>`, `<UserProfile>`)
  that we render inside our own page chrome via `appearance` overrides,
  so the SPA contract stays intact.

## Alternatives Considered

| Provider | Why not chosen |
|---|---|
| WorkOS | Strong B2B/enterprise (SSO/SCIM-ready), fewer rough edges around Directory Sync, but lighter on hosted-UI maturity. Kept as the **named alternate** behind the Provider interface. |
| Stytch | Near-tie with Clerk on B2B; loses on hosted-UI maturity and on the Organization invitation UX. |
| Supabase Auth | Couples us to a Postgres-as-a-service ecosystem we haven't adopted. |
| Auth0 (Okta) | Pricing and SDK ergonomics dated relative to Clerk. |
| Ory Kratos / Keycloak (self-hosted) | Moves ops burden onto us at the moment we are trying to *reduce* security ops. |
| Build (extend Sprint 003) | Estimated 2–3 sprints to parity; permanent operational surface. |

Cost is favorable but not load-bearing for this decision; it rests on
architectural fit and time-to-ship.

## Consequences

### Positive

- US-1.4 / US-1.5 / US-6.1 / US-6.2 / US-6.3 unblocked without us
  building or operating an email vendor integration.
- Future MFA / WebAuthn / SAML SSO available behind product flags
  rather than a rebuild.
- Security-sensitive surfaces (password storage, rate limiting,
  brute-force defense, password reset) move outside our codebase.

### Negative

- A new external dependency (`clerk-sdk-go/v2`) and a vendor on the
  request path.
- A new external dependency (`@clerk/clerk-react`) on the SPA.
- Outage of Clerk blocks new sign-ins (existing `lb_session` cookies
  continue to authenticate via the local `app_sessions` table for their
  TTL).
- Network round-trip to Clerk's JWKS once per server cold start; cached
  thereafter.
- Webhook delivery introduces an at-least-once event channel that needs
  idempotency and ordering tolerance.

### Neutral

- The Provider interface (`internal/auth/provider.go`) and the canonical
  local `users` / `organizations` tables preserve the option to migrate
  away. Clerk owns identity; we own the app data.

## Escape Hatch

If Clerk becomes a problem, the migration path is:

1. Stand up a replacement provider (WorkOS, Stytch, etc.).
2. Implement `Provider` for that vendor in `internal/auth/`.
3. Run a one-off backfill: list users from Clerk, create or look up
   matching identities in the new provider, populate `users.<new>_id`
   alongside `clerk_user_id` (or replace it).
4. Swap the `Provider` constructor in `cmd/server/main.go` and ship.
5. Communicate the password-reset requirement to existing users
   (passwords cannot be migrated between providers).

The webhook contract (`webhook_events` idempotency, the per-event
handlers) and the cookie contract (`lb_session` minted by
`/api/auth/exchange`) stay in place across provider swaps.

## Data Ownership

| Concern | Clerk | Our DB |
|---|---|---|
| Email + password (or social) credentials | ✅ | — |
| Email verification | ✅ | — |
| Password reset | ✅ | — |
| Provider session (Clerk JWT) | ✅ | — |
| Backend session (`lb_session` cookie) | — | ✅ (`app_sessions`) |
| MFA / WebAuthn (future) | ✅ | — |
| Invitation lifecycle | ✅ | — |
| User identity (email, full_name) | ✅ source | ✅ mirror |
| Organization identity (name) | ✅ source | ✅ mirror |
| App-level role (`org_admin`/`site_director`) | mirror only | ✅ **authoritative** |
| Domain data (boats, trips, ledger) | — | ✅ |

## Implementation Reference

- `internal/auth/provider.go` — Provider interface seam.
- `internal/auth/clerk.go` — Clerk implementation against
  `clerk-sdk-go/v2`.
- `internal/auth/stub.go` — in-memory Provider for tests.
- `internal/auth/exchange.go` — `/api/signup-complete` and
  `/api/auth/exchange`, the cookie-bridge endpoints.
- `internal/auth/middleware.go` — `lb_session`-cookie auth.
- `internal/auth/webhook.go` + `webhook_handlers.go` — Clerk → us sync.
- `internal/store/migrations/0002_auth_provider_clerk_link.sql` — adds
  provider linkage.
- `internal/store/migrations/0004_auth_provider_cleanup.sql` — drops
  the legacy local-credentials columns and tables.
