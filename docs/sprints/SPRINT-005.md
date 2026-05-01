# Sprint 005: Migrate Authentication to Clerk

## Overview

Sprint 003 stood up custom auth (bcrypt, opaque session tokens, cookie). Sprint 004 disciplined configuration. We are now at the natural decision point: every Must-priority backlog item that is currently blocked — **US-1.4** password reset, **US-6.1** invite Site Director, **US-6.2** deactivate user, **US-6.3** resend invitation, plus the soon-needed **US-1.5** profile updates — requires a real email vendor, transactional templates, an invitation lifecycle, password reset flows, rate limiting, and eventually MFA. That is a multi-sprint commitment to build, plus a permanent commitment to operate a security surface.

This sprint resolves the build-vs-buy question in favor of **buy** and migrates to **Clerk** (with WorkOS as the named alternate). Clerk fits this product unusually well: it has a first-class Organizations primitive that maps to our `organizations` table; built-in invitation flows that fulfill US-6.1 / US-6.3 with no email plumbing on our side; an RBAC primitive we mirror against our existing `users.role`; and SDKs for both React and Go.

The migration is sized as a one-sprint cutover because we are still pre-customer. There are no production users to preserve; dev data is throwaway. Sprint 003's auth tables are wiped. Crucially, we **preserve the existing same-origin cookie contract** at the app boundary — the SPA continues to authenticate via the `lb_session` cookie and `/api/me` probe. Clerk owns credentials, sessions, emails, and invitations; our backend continues to mint its own session cookie via a one-shot exchange after Clerk authentication. `users` and `organizations` remain canonical for app data, with a webhook keeping them in sync and a synchronous upsert on first auth as the primary write path (avoiding a webhook race condition).

## Decision: Build vs Buy

| Option | Verdict | Rationale |
|---|---|---|
| **Clerk** | **Recommended (chosen)** | Organizations primitive matches our personas; invite flow fulfills US-6.1 directly; React + Go SDKs; design tokens via `appearance` API. |
| WorkOS | Alternate | Strong B2B/enterprise (SSO/SCIM/Directory Sync ready). Equivalent fit; lighter on hosted-UI maturity. Kept as named fallback behind the provider adapter. |
| Stytch | Considered | Near-tie with Clerk on B2B; loses on hosted-UI maturity. |
| Supabase Auth | Rejected | Couples us to a Postgres-as-a-service ecosystem we haven't adopted. |
| Auth0 | Rejected | Pricing and SDK ergonomics dated relative to Clerk. |
| Ory Kratos / Keycloak | Rejected | Self-hosted; moves ops burden onto us at exactly the moment we want to *reduce* security ops. |
| Build (extend Sprint 003) | Rejected | Estimated 2–3 sprints to feature parity, plus permanent operational surface. |

Cost is favorable for our scale but is **not** the deciding factor. The decision rests on architectural fit (Organizations primitive, invitation UX, React + Go SDKs) and time-to-ship for the blocked Must-priority backlog items.

## Use Cases

1. **Sign up as Org Admin (US-1.1)**: User opens `/signup`. Clerk `<SignUp>` collects credentials and verifies email. SPA then submits `organization_name` to `POST /api/signup-complete`; backend creates local org, Clerk org, and membership atomically; first user's `users.role = org_admin`.
2. **Log in / out (US-1.2 / US-1.3)**: Clerk `<SignIn>` issues a session; SPA exchanges it once at `POST /api/auth/exchange` for an `lb_session` cookie. `POST /api/logout` revokes both backend cookie and Clerk session.
3. **Password reset (US-1.4)**: Entirely Clerk's flow. We handle no tokens, no emails, no rate limiting.
4. **Invite Site Director (US-6.1)**: Org Admin uses our UI to send a Clerk Organization invitation. Clerk emails the invitee. On accept, webhook + synchronous upsert create the local `users` row with `role = site_director`.
5. **Deactivate user (US-6.2)**: Org Admin removes Clerk membership and we set `users.is_active = false`. Backend session is revoked; future requests resolve to a 401.
6. **Resend invitation (US-6.3)**: Org Admin re-issues via Clerk admin API.
7. **Profile updates (US-1.5)**: `<UserProfile>` Clerk component; webhook syncs `email` / `full_name` back to `users`.

## Architecture

### High-level flow

```
                    Browser (React SPA)
                           │
              ┌────────────┼────────────┐
              │            │             │
       <SignUp>/<SignIn>   │      App pages (Dashboard, etc.)
       Clerk components    │      use lb_session cookie
              │            │             │
              ▼            │             │
        api.clerk.com      │             │
              │            │             │
              │ session    │             │
              ▼            │             │
       SPA receives session▼             ▼
                  POST /api/auth/exchange (one-time, cookie-set)
                           │
                           ▼
                  Liveaboard API (Go)
                           │
                  Verify Clerk JWT (JWKS, cached)
                           │
                  Synchronous upsert of users + org row if missing
                           │
                  Mint lb_session, write app_sessions row
                           │
                  Set-Cookie: lb_session=...
                           │
              All future /api/* calls (cookie auth)
                           │
                           ▼
                  app_sessions lookup → users row → handlers
                           │
                  ▲ webhook reconciler (svix)
                  │  api.clerk.com → POST /api/webhooks/clerk
```

### Identity & data ownership

| Concern | Clerk | Our DB |
|---|---|---|
| Email + password (or passwordless) | ✅ | — |
| Email verification | ✅ | — |
| Password reset | ✅ | — |
| Provider session (Clerk JWT) | ✅ | — |
| Backend session (`lb_session` cookie) | — | ✅ (`app_sessions`) |
| MFA / WebAuthn (future) | ✅ | — |
| Invitation lifecycle | ✅ | — |
| User identity (email, full_name) | ✅ source | ✅ mirror |
| Organization identity (name) | ✅ source | ✅ mirror |
| App-level role (`org_admin`/`site_director`/`crew`) | mirror only | ✅ **authoritative** |
| Domain data (boats, trips, ledger) | — | ✅ |

`users.role` is authoritative for application authorization. Clerk's organization role is mirrored for the hosted UI's display purposes only; backend handlers never trust a Clerk role claim.

### Schema migrations (two)

```sql
-- 0002_auth_provider_clerk_link.sql  (Phase 4 — early)

ALTER TABLE users          ADD COLUMN clerk_user_id  text UNIQUE;
ALTER TABLE organizations  ADD COLUMN clerk_org_id   text UNIQUE;
ALTER TABLE users          ALTER COLUMN password_hash     DROP NOT NULL;
ALTER TABLE users          ALTER COLUMN password_hash     SET DEFAULT NULL;

CREATE TABLE app_sessions (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash      bytea       NOT NULL UNIQUE,
    user_id         uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    clerk_user_id   text        NOT NULL,
    clerk_session_id text       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL
);
CREATE INDEX app_sessions_user_id_idx     ON app_sessions(user_id);
CREATE INDEX app_sessions_expires_at_idx  ON app_sessions(expires_at);

CREATE TABLE webhook_events (
    id          text        PRIMARY KEY,    -- Clerk event id (svix-id)
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth_sync_cursors (
    name      text        PRIMARY KEY,
    cursor    text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- placeholder for a future reconciler; populated by a follow-up sprint.
```

```sql
-- 0003_auth_provider_cleanup.sql   (Phase 6 — after end-to-end validation)

ALTER TABLE users          ALTER COLUMN clerk_user_id  SET NOT NULL;
ALTER TABLE organizations  ALTER COLUMN clerk_org_id   SET NOT NULL;
ALTER TABLE users          DROP COLUMN password_hash;
ALTER TABLE users          DROP COLUMN email_verified_at;

DROP TABLE sessions;
DROP TABLE email_verifications;
```

### Backend layout

```
internal/auth/
  provider.go          # Provider interface (VerifyJWT, FetchUser, FetchOrganization,
                       #                      CreateOrganization, InviteToOrganization,
                       #                      RemoveMembership, RevokeSession)
  clerk.go             # Clerk implementation of Provider (clerk-sdk-go/v2)
  stub.go              # Offline stub Provider for tests (LIVEABOARD_AUTH_STUB=1)
  exchange.go          # POST /api/auth/exchange handler + signup-complete handler
  middleware.go        # requireSession reads lb_session cookie -> app_sessions -> users
  webhook.go           # POST /api/webhooks/clerk (svix signature)
  webhook_handlers.go  # Per-event sync (user.*, organization.*, organizationMembership.*)
  upsert.go            # Synchronous upsert helpers used by exchange + webhook
  middleware_test.go
  exchange_test.go
  webhook_test.go
  testdata/            # Captured Clerk webhook payloads
  auth.go              # DELETED  (Sprint 003 service)
  auth_test.go         # DELETED

internal/store/
  app_sessions.go      # NEW — small repo for the bridge cookie sessions
  organizations.go     # NEW — split from users.go; adds Clerk-linked ops
  users.go             # MODIFIED — adds clerk_user_id; UpsertExternalUser, DeactivateUser
  sessions.go          # DELETED in 0003
  email_verifications.go # DELETED in 0003

internal/httpapi/
  httpapi.go           # MODIFIED — strip signup/login/logout/verify-email handlers;
                       # mount /auth/exchange, /signup-complete, /logout, /webhooks/clerk;
                       # swap requireSession to read app_sessions

internal/config/
  config.go            # NEW fields:
                       #  ClerkPublishableKey (non-secret)
                       #  ClerkSecretKey      (secret, required)
                       #  ClerkWebhookSecret  (secret, required)
                       #  ClerkJWKSURL        (derived from secret key issuer)
                       #  AuthStubEnabled     (test mode only; production rejects true)
```

### Frontend layout

```
web/src/
  main.tsx             # Wrap <App> in <ClerkProvider publishableKey={...}>
  pages/
    Signup.tsx         # REPLACED — <SignUp> + a small "Name your organization" step
                       # submitting to POST /api/signup-complete
    Login.tsx          # REPLACED — <SignIn> + post-success POST /api/auth/exchange
    VerifyEmail.tsx    # DELETED (Clerk handles)
    Dashboard.tsx      # MODIFIED — useUser/useOrganization for identity + our /api/me
                       # for app data; logout via POST /api/logout
  lib/
    api.ts             # Cookie-only fetch wrapper; no Bearer tokens; same as today
    RequireSession.tsx # MODIFIED — checks /api/me as before; Clerk session is invisible
                       # to most of the app
    config.ts          # Adds VITE_CLERK_PUBLISHABLE_KEY
  package.json         # +@clerk/clerk-react
```

The SPA's auth contract is unchanged at the API layer: same-origin cookies, `/api/me` probe, `POST /api/logout`. Clerk's session is exchanged exactly once (at `/api/auth/exchange`) and never leaves that page.

### Local dev

- **Real flows** use a shared Clerk *development instance* (test API keys; deliverable to seeded test inboxes; emails like `karim+clerk_test@example.com` auto-verify).
- **Offline tests** use the stub provider behind `LIVEABOARD_AUTH_STUB=1`. The stub honors `Authorization: Bearer test-<uuid>` (or a `/api/auth/exchange` body containing a stub token), issues a fake user, and lets `go test ./...` run with no network. Production startup rejects `AuthStubEnabled=true`.

## API Endpoints

### Removed

| Endpoint | Reason |
|---|---|
| `POST /api/signup` | Split: Clerk `<SignUp>` collects credentials; `POST /api/signup-complete` creates org + membership. |
| `POST /api/verify-email` | Clerk owns verification. |
| `POST /api/login` | Clerk `<SignIn>` issues session; SPA calls `POST /api/auth/exchange`. |

### Added

| Endpoint | Method | Purpose |
|---|---|---|
| `POST /api/signup-complete` | POST | After Clerk signup, creates local org + Clerk org + membership; sets `lb_session`. |
| `POST /api/auth/exchange` | POST | One-shot: verify Clerk session, synchronously upsert local user, mint `lb_session`. |
| `POST /api/webhooks/clerk` | POST | Svix-verified Clerk events; idempotent via `webhook_events`. |
| `POST /api/invitations` | POST | Org Admin invites via Clerk Organization invite API. |
| `POST /api/invitations/{id}/resend` | POST | Resend pending invitation. |
| `POST /api/users/{id}/deactivate` | POST | Org Admin removes membership; sets `users.is_active = false`. |

### Kept (SPA contract preserved)

| Endpoint | Method | Purpose |
|---|---|---|
| `POST /api/logout` | POST | Revokes Clerk session + clears `lb_session`. **Stays a backend endpoint.** |
| `GET /api/me` | GET | Cookie-authenticated; same response shape as today. |
| `GET /api/organization` | GET | Unchanged. |

## Implementation Plan

### Phase 1: Provider POC + decision lock-in (~10%)

**Files:** `docs/decisions/0001-auth-provider.md`.

**Tasks:**
- [ ] Create a Clerk development instance; capture publishable key, secret key, JWKS URL, webhook signing secret.
- [ ] Manually verify a Clerk-issued JWT in a one-off Go test (JWKS fetch + verify).
- [ ] Confirm Organizations are enabled and invite emails fire from the dev instance.
- [ ] Lock the decision in `docs/decisions/0001-auth-provider.md` (ADR): chosen provider, alternates considered, escape-hatch design.

### Phase 2: Provider interface + Clerk adapter + stub (~15%)

**Files:** `internal/auth/provider.go`, `internal/auth/clerk.go`, `internal/auth/stub.go`, `internal/config/config.go`, `cmd/server/main.go`.

**Tasks:**
- [ ] Define `internal/auth.Provider` interface (the seam Codex recommended): `VerifyJWT(ctx, token) (Claims, error)`; `FetchUser(ctx, id) (User, error)`; `FetchOrganization(ctx, id) (Organization, error)`; `CreateOrganization(ctx, name) (string, error)`; `AddMembership(ctx, orgID, userID, role string) error`; `InviteToOrganization(ctx, orgID, email, role string) (InviteID, error)`; `ResendInvitation(ctx, inviteID) error`; `RemoveMembership(ctx, orgID, userID) error`; `RevokeSession(ctx, sessionID) error`.
- [ ] Implement `clerk.go` against `clerk-sdk-go/v2`. JWKS verifier with in-memory cache.
- [ ] Implement `stub.go` (no network); accepts `test-<uuid>` tokens; in-memory user/org map.
- [ ] Add Clerk config fields. Production-mode validation rejects `AuthStubEnabled=true` and demands `ClerkSecretKey` + `ClerkWebhookSecret` from process env.
- [ ] Wire the chosen provider into `httpapi.Server` and `cmd/server/main.go`.

### Phase 3: Schema migration 0002 + app_sessions repo (~10%)

**Files:** `internal/store/migrations/0002_auth_provider_clerk_link.sql`, `internal/store/app_sessions.go`, `internal/store/users.go`, `internal/store/organizations.go`.

**Tasks:**
- [ ] Write migration `0002` per the schema above (NULLABLE old columns, new linkage columns, `app_sessions`, `webhook_events`, `auth_sync_cursors`).
- [ ] Add `app_sessions` repo (`Create`, `ByTokenHash`, `DeleteByTokenHash`, `DeleteByUser`, `Touch`).
- [ ] Add `users.UpsertExternalUser(orgID, clerkUserID, email, fullName, role)` and `DeactivateUser`.
- [ ] Split organization queries into `internal/store/organizations.go`; add `UpsertExternalOrganization`.

### Phase 4: Exchange + signup-complete + middleware (~20%)

**Files:** `internal/auth/exchange.go`, `internal/auth/upsert.go`, `internal/auth/middleware.go`, `internal/auth/middleware_test.go`, `internal/auth/exchange_test.go`, `internal/httpapi/httpapi.go`.

**Tasks:**
- [ ] Implement `POST /api/auth/exchange`: read Clerk JWT (from JSON body or `Authorization`), verify, **synchronously upsert** local user via the provider (resolves the Codex race-condition critique); create `app_sessions` row; set `lb_session` cookie.
- [ ] Implement `POST /api/signup-complete`: requires a Clerk JWT, takes `organization_name`; creates local org + Clerk org + membership atomically (Clerk side via admin SDK, local side via `CreateOrgAndAdmin` rewritten to take a `clerk_user_id`); sets `lb_session`. First user gets `role=org_admin`.
- [ ] Replace `requireSession` middleware: read `lb_session` cookie, sha256 → `app_sessions` row → `users` row → context. Return 401 if expired/missing/inactive.
- [ ] Implement `POST /api/logout`: revoke Clerk session via provider, delete `app_sessions` row, clear cookie.
- [ ] Tests for: valid exchange, expired Clerk JWT, missing local user (synchronous upsert path), signup-complete idempotency on retry, logout invalidation, deactivated user blocked.

### Phase 5: Webhook receiver + invitation/deactivation handlers + frontend (~25%)

**Files:** `internal/auth/webhook.go`, `internal/auth/webhook_handlers.go`, `internal/auth/webhook_test.go`, `internal/auth/testdata/*.json`, `internal/httpapi/httpapi.go`, `web/src/main.tsx`, `web/src/pages/{Signup,Login,Dashboard}.tsx`, `web/src/lib/{api.ts,RequireSession.tsx,config.ts}`, `web/package.json`.

**Tasks:**

Backend:
- [ ] `POST /api/webhooks/clerk`: svix signature verification, idempotency via `webhook_events`, dispatch by event type.
- [ ] Handlers for: `user.created`, `user.updated`, `user.deleted`, `organization.created`, `organization.updated`, `organization.deleted`, `organizationMembership.created`, `organizationMembership.updated`, `organizationMembership.deleted`.
- [ ] Map Clerk org role → our `users.role` (`admin` → `org_admin`, `director` → `site_director`, `member` → `crew`).
- [ ] `POST /api/invitations`, `POST /api/invitations/{id}/resend`, `POST /api/users/{id}/deactivate` (calls into the Provider).
- [ ] Tests with replayed webhook fixtures (signature pass + replay idempotency).

Frontend:
- [ ] Add `@clerk/clerk-react`; wrap `<App>` in `<ClerkProvider>`.
- [ ] `Signup.tsx`: `<SignUp>` then a small org-name step that posts to `/api/signup-complete`.
- [ ] `Login.tsx`: `<SignIn>` then a one-shot post to `/api/auth/exchange`.
- [ ] `Dashboard.tsx`: keep `/api/me` and `/api/organization` calls; logout via `POST /api/logout` then `signOut()`.
- [ ] `RequireSession.tsx`: keep using `/api/me` (cookie probe). No change to the SPA's auth contract.
- [ ] Apply DESIGN.md tokens to Clerk components via `appearance` prop.
- [ ] Delete `VerifyEmail.tsx`.

### Phase 6: Cleanup migration 0003 + delete dead code + docs (~20%)

**Files:** `internal/store/migrations/0003_auth_provider_cleanup.sql`, `internal/auth/auth.go` (DELETE), `internal/auth/auth_test.go` (DELETE), `internal/store/sessions.go` (DELETE), `internal/store/email_verifications.go` (DELETE), `docs/auth.md`, `docs/decisions/0001-auth-provider.md`, `RUNNING.md`, `.env.example`, `config/{dev,test,production}.env`, `Makefile`.

**Tasks:**
- [ ] Run end-to-end manual smoke against the dev Clerk instance (signup → invite → accept → login → deactivate → 401). Block 0003 until it passes.
- [ ] Apply migration `0003`: drop `password_hash`, `email_verified_at`, `sessions`, `email_verifications`; promote linkage columns to NOT NULL.
- [ ] Delete `internal/auth/auth.go`, `internal/auth/auth_test.go`, `internal/store/sessions.go`, `internal/store/email_verifications.go`.
- [ ] `git grep "bcrypt"` returns nothing in `internal/`.
- [ ] Add Clerk config keys to `.env.example` and the three `config/*.env` files (publishable key only — secrets stay env-only).
- [ ] Write `docs/auth.md`: how Clerk is wired, how to rotate keys, how to add a new webhook event handler, escape-hatch procedure (re-enroll users / link a new provider via the `Provider` interface), what each backlog item maps to.
- [ ] Update `RUNNING.md` with Clerk dev-instance setup for new contributors.
- [ ] Add `make stub-test` target running tests with `LIVEABOARD_AUTH_STUB=1`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm run build` clean.
- [ ] `go run docs/sprints/tracker.go sync`.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/decisions/0001-auth-provider.md` | Create | ADR locking in Clerk; alternates and escape hatch. |
| `docs/auth.md` | Create | Operational guide (rotation, webhooks, escape, story-to-feature mapping). |
| `internal/auth/provider.go` | Create | Provider interface (verifier + admin operations). |
| `internal/auth/clerk.go` | Create | Clerk implementation. |
| `internal/auth/stub.go` | Create | Offline stub for tests. |
| `internal/auth/exchange.go` | Create | `/api/auth/exchange` and `/api/signup-complete`. |
| `internal/auth/upsert.go` | Create | Synchronous user/org upsert (used by exchange + webhook). |
| `internal/auth/middleware.go` | Create | New `requireSession` reading `lb_session`. |
| `internal/auth/webhook.go` | Create | `/api/webhooks/clerk` with svix signature. |
| `internal/auth/webhook_handlers.go` | Create | Per-event sync logic. |
| `internal/auth/middleware_test.go` | Create | Cookie-auth path tests. |
| `internal/auth/exchange_test.go` | Create | Exchange and signup-complete tests. |
| `internal/auth/webhook_test.go` | Create | Replay + signature tests. |
| `internal/auth/testdata/*.json` | Create | Captured Clerk webhook payloads. |
| `internal/auth/auth.go` | Delete | Sprint 003 service superseded. |
| `internal/auth/auth_test.go` | Delete | Superseded. |
| `internal/store/migrations/0002_auth_provider_clerk_link.sql` | Create | Add linkage; nullable old columns; `app_sessions`, `webhook_events`, `auth_sync_cursors`. |
| `internal/store/migrations/0003_auth_provider_cleanup.sql` | Create | Drop dead local auth state; promote linkage to NOT NULL. |
| `internal/store/app_sessions.go` | Create | Cookie-session bridge repo. |
| `internal/store/organizations.go` | Create | Split from users.go; Clerk-linked ops. |
| `internal/store/users.go` | Modify | Add `clerk_user_id`; `UpsertExternalUser`, `DeactivateUser`. |
| `internal/store/sessions.go` | Delete | Superseded by `app_sessions`. |
| `internal/store/email_verifications.go` | Delete | Clerk owns verification. |
| `internal/store/store.go` | Modify | Drop session/verification accessors. |
| `internal/httpapi/httpapi.go` | Modify | Strip old auth handlers; mount exchange/signup-complete/webhook/invitations/deactivate; swap middleware. |
| `internal/httpapi/httpapi_test.go` | Modify | Replace auth-handler tests with cookie + webhook tests. |
| `internal/config/config.go` | Modify | Add Clerk fields + `AuthStubEnabled` (test-only). |
| `cmd/server/main.go` | Modify | Construct Clerk Provider; wire middleware. |
| `web/src/main.tsx` | Modify | `<ClerkProvider>`. |
| `web/src/pages/Signup.tsx` | Replace | `<SignUp>` + org-name step → `/api/signup-complete`. |
| `web/src/pages/Login.tsx` | Replace | `<SignIn>` + `/api/auth/exchange`. |
| `web/src/pages/VerifyEmail.tsx` | Delete | Clerk handles. |
| `web/src/pages/Dashboard.tsx` | Modify | Logout via backend; identity via Clerk hooks. |
| `web/src/lib/RequireSession.tsx` | Modify | Keep `/api/me` probe; auth contract unchanged. |
| `web/src/lib/api.ts` | Modify | Add invite/deactivate calls. |
| `web/src/lib/config.ts` | Modify | `VITE_CLERK_PUBLISHABLE_KEY`. |
| `web/package.json` | Modify | `@clerk/clerk-react`. |
| `.env.example` | Modify | Document Clerk keys. |
| `config/dev.env` | Modify | Non-secret Clerk values. |
| `config/test.env` | Modify | Non-secret + stub flag. |
| `config/production.env` | Modify | Non-secret values; secrets remain env-only. |
| `RUNNING.md` | Modify | Clerk dev-instance setup. |
| `Makefile` | Modify | `make stub-test`. |
| `docs/sprints/SPRINT-005.md` | Create | This document. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 005 via `tracker.go sync`. |

## Definition of Done

- [ ] ADR `docs/decisions/0001-auth-provider.md` exists and lists the chosen provider, rejected alternates, and the escape-hatch procedure.
- [ ] `internal/auth.Provider` interface exists with `clerk.go` and `stub.go` implementations; `git grep "bcrypt"` returns nothing in `internal/`.
- [ ] Migration `0002` runs cleanly on a fresh `liveaboard` and `liveaboard_test`; migration `0003` runs cleanly only after Phase 6 manual smoke passes.
- [ ] `internal/auth/auth.go`, `internal/auth/auth_test.go`, `internal/store/sessions.go`, `internal/store/email_verifications.go` are deleted.
- [ ] `users.clerk_user_id` and `organizations.clerk_org_id` are NOT NULL UNIQUE after `0003`.
- [ ] `app_sessions` is the only sessions table; `lb_session` cookie is the only auth token the SPA sees.
- [ ] `POST /api/auth/exchange`: verifies Clerk JWT, synchronously upserts local user (no webhook race), mints `lb_session`.
- [ ] `POST /api/signup-complete`: creates local org + Clerk org + membership; first user is `org_admin`.
- [ ] `POST /api/logout`: revokes Clerk session and clears `lb_session`.
- [ ] `POST /api/webhooks/clerk` rejects missing/invalid svix signatures; is idempotent on replay; covers the 9 event types listed in Phase 5 with at least one test each.
- [ ] `POST /api/invitations`, `POST /api/invitations/{id}/resend`, `POST /api/users/{id}/deactivate` are implemented and protected by `requireSession` + `org_admin` role check.
- [ ] Backend tests pass with `LIVEABOARD_AUTH_STUB=1` (no internet); `make stub-test` works.
- [ ] Production refuses to start with `AuthStubEnabled=true` or with `ClerkSecretKey`/`ClerkWebhookSecret` missing.
- [ ] Frontend builds (`npm run build`); DESIGN.md tokens applied via Clerk `appearance`.
- [ ] SPA contract preserved: `/api/me` probe, `lb_session` cookie, `POST /api/logout` all behave as before.
- [ ] Manual end-to-end smoke (Phase 6) passes against the dev Clerk instance.
- [ ] All Must-priority backlog items unblocked (US-1.1, US-1.2, US-1.3, US-1.4, US-6.1, US-6.2) have a "delivered by Clerk feature X" line in `docs/auth.md`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...` clean. `npm run build` clean.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Vendor lock-in (Clerk) | Medium | High | `Provider` interface seam; `users` and `organizations` canonical; documented escape via re-enrollment + new provider impl. |
| Clerk outage blocks login | Low | High | Existing `lb_session` cookies continue to authenticate via the local `app_sessions` table for their lifetime — Clerk only blocks *new* sessions; status-page subscription. |
| Webhook delivery race ("user authenticates before webhook lands") | High → mitigated | Medium | Synchronous upsert at `/api/auth/exchange` eliminates the race; webhooks are reconciliation only. |
| Webhook reliability for out-of-band events | Medium | Medium | `webhook_events` idempotency; svix retries; `auth_sync_cursors` table reserved for a follow-up reconciler. |
| Cost surprise | Low | Medium | Cost is documented in the ADR but not load-bearing; no specific MAU number anchored in the plan. |
| JWT verification needs JWKS round-trip | Low | Medium | In-memory JWKS cache (1h TTL); failure surfaces at startup health check. |
| Org-bootstrap race (Clerk org created, local insert fails) | Low | Medium | `signup-complete` runs in a transaction with a try/compensate path: if local insert fails, delete the just-created Clerk org via admin SDK; idempotent on retry by checking `clerk_user_id`. |
| Hosted Clerk UI doesn't match DESIGN.md | Medium | Low | `appearance` API injects our tokens; embed `<SignIn>` / `<SignUp>` inside our own page chrome rather than redirecting to a hosted URL. |
| Multi-org-per-user violates our 1:1 invariant | Low | Medium | Webhook handler rejects (200 + log) any membership event that would put a user into a second org; one-org-per-user is enforced at upsert time. |
| Local-dev contributors hate "you need a Clerk account" | Medium | Low | `LIVEABOARD_AUTH_STUB=1` runs all backend tests offline; only the Clerk dev instance requires an account, and it's optional for core dev. |
| Migration drops password_hash before validation | High → mitigated | Low | Two migrations: `0002` makes columns nullable; `0003` drops them only after the Phase 6 manual smoke passes. Single sprint, internal validation gate. |

## Security Considerations

- All Clerk secrets (`ClerkSecretKey`, `ClerkWebhookSecret`) flow through the Sprint 004 `Config` system, tagged `secret` and `required`. Production refuses to start without them.
- The webhook endpoint validates svix signatures **before** parsing the body; unsigned POSTs are rejected with 401, body is not logged.
- JWT verification uses Clerk's JWKS over HTTPS with a pinned issuer and audience.
- `users.clerk_user_id` and `organizations.clerk_org_id` are unique; duplicate webhook attempts are rejected and logged.
- The SPA never sees a Clerk JWT in long-term storage; it is exchanged once at `/api/auth/exchange` for an `lb_session` cookie. The cookie is HttpOnly + Secure (production) + SameSite=Lax.
- Logout invalidates *both* the local `app_sessions` row and the Clerk session.
- Multi-tenant isolation continues at the data layer (`organization_id`); Clerk identity does not bypass row-level scoping.
- Stub mode is feature-gated; `MustLoad` in production rejects `AuthStubEnabled=true`.
- App-level role (`users.role`) is authoritative; Clerk role claims in JWTs are ignored at the policy layer.

## Dependencies

- **Sprint 003** — provides the auth/store/httpapi structure this sprint surgically modifies.
- **Sprint 004** — provides the typed `Config` system used for new Clerk secrets.
- **External: Clerk dev instance** — created in Phase 1; project owner registers the account.
- **New Go module deps**: `github.com/clerk/clerk-sdk-go/v2`, `github.com/svix/svix-webhooks/go`. Both mainstream; verified for license + maintenance before merge.
- **New npm dep**: `@clerk/clerk-react`.

## Out of Scope

- MFA / TOTP / WebAuthn (Clerk supports it; enable in a follow-up sprint when the product asks).
- SSO / SAML / SCIM (Clerk supports it on a paid tier; defer until a customer asks).
- A reconciler cron that catches webhook-loss drift (`auth_sync_cursors` table reserved; follow-up sprint).
- Multi-org-per-user UX (Clerk supports it; we keep our 1:1 invariant).
- Replacing Clerk's hosted UI with fully custom forms (we use Clerk components inside our own chrome).
- Migrating real production users (there are none; we wipe dev data).
- Implementing the WorkOS adapter as a fall-forward (named alternate in the ADR; not built).

## References

- Sprint 003 — `docs/sprints/SPRINT-003.md` (auth foundation).
- Sprint 004 — `docs/sprints/SPRINT-004.md` (config system).
- Stories — `docs/product/organization-admin-user-stories.md` (US-1.x, US-6.x).
- Personas — `docs/product/personas.md`.
- ADR — `docs/decisions/0001-auth-provider.md` (created in Phase 1).
