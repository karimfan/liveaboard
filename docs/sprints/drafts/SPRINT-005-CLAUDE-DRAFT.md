# Sprint 005: Migrate to Clerk for Authentication & User Management

## Overview

The seed asks "should we do our own authentication or rely on a third party?
If so, which one and how do we migrate?" This draft answers all three.

**Recommendation: Buy, with Clerk as the primary and WorkOS as the named
backup.** The build path looks small but isn't — every Must-priority
backlog item that's currently blocked (US-6.1 invite Site Director, US-6.2
deactivate user, US-1.4 password reset, plus the soon-needed US-1.5
profile updates) requires a real email vendor, transactional templates, an
invitation lifecycle (accept / expire / resend), password reset with rate
limiting, and eventually MFA. That's a multi-sprint commitment to *build*,
plus an open-ended commitment to *operate* a security surface forever.

Clerk fits this product unusually well. It has a first-class
"Organizations" primitive that maps directly to our `organizations` table;
built-in invitation flows that fulfill US-6.1 / US-6.3 with no email
plumbing on our side; an RBAC primitive that maps to our existing
`org_admin` / `site_director` / `crew` roles; SDKs for both React and Go;
and a 10K MAU free tier that is comfortably above MVP and beyond.

The migration is sized as a one-sprint cutover specifically because we are
still in pre-customer dev. Sprint 003 just landed; there are no production
users to migrate. We wipe dev data, swap the auth surface, and keep
`users` / `organizations` as the canonical app tables — Clerk owns
credentials, sessions, and emails; we own everything else. This keeps the
escape hatch wide if we ever want to leave Clerk: re-issue passwords or
plug in another provider, re-link via `users.clerk_user_id`, done.

## Decision: Build vs Buy

### Scoring (1 = poor, 5 = excellent)

| Dimension | Build (extend Sprint 003) | Clerk | WorkOS | Stytch | Supabase Auth | Auth0 | Ory Kratos (self-host) |
|---|---|---|---|---|---|---|---|
| Time to ship US-6.1/6.2/6.3 | 2 | 5 | 4 | 4 | 4 | 4 | 2 |
| Time to ship US-1.4 (reset) | 2 | 5 | 5 | 5 | 5 | 5 | 3 |
| B2B Organizations primitive | n/a | 5 | 4 | 5 | 2 | 3 | 2 |
| Local dev story | 5 | 4 | 3 | 4 | 4 | 3 | 3 |
| Lock-in / escape hatch | 5 | 4 | 4 | 4 | 3 | 3 | 5 |
| Long-term ops cost (us) | 1 | 5 | 5 | 5 | 4 | 5 | 2 |
| MVP cost (USD/mo) | 0 + email vendor (~$15) | 0 (free ≤10K MAU) | 0 + per-active fee | 0 (free tier) | 0 | $35+ | 0 (servers + ops) |
| Future SSO/SAML readiness | 1 | 4 | 5 | 4 | 3 | 5 | 4 |
| **Total** | **18** | **37** | **35** | **36** | **30** | **32** | **26** |

Clerk wins on this product's profile. WorkOS is the credible alternate if
the team's strategic outlook is "we are an enterprise B2B product first";
the gap is small. Stytch is a near-tie; we choose Clerk for the React
component story and the Organizations UX maturity.

### Build path (rejected)

Building well means: pick an email vendor (Postmark / Resend / SES);
implement templated emails; build invitation tokens + accept flow; build
password reset with rate limiting and lockout; refactor session handling
for refresh; add MFA when a customer asks; build SSO/SAML when a customer
asks. We estimate **2–3 sprints** to get to feature parity with what Clerk
gives us out of the box, then permanent ownership of the security surface.

### Provider rejection notes

- **WorkOS** — strong B2B/enterprise; lighter on consumer UX components.
  Picked as backup; the choice between Clerk and WorkOS is reversible
  while we keep `users` canonical.
- **Stytch** — very close to Clerk; loses on hosted-UI maturity and the
  Organization invite UX feels less product-ready.
- **Supabase Auth** — couples us to a Postgres-as-a-service ecosystem we
  haven't adopted; using just Supabase Auth is awkward operationally.
- **Auth0** — incumbent; expensive; SDK ergonomics dated relative to
  Clerk; Organizations product is fine but trails on UX.
- **Ory Kratos / Keycloak** — open source, but moves the ops burden onto
  us at exactly the moment we are trying to *reduce* security ops. Keep
  in mind for a future "fully open-source" pivot if it ever happens.

## Use Cases (post-cutover)

1. **Sign up as Org Admin (US-1.1)**: user signs up via Clerk; webhook
   creates a `users` row + `organizations` row in our DB; user is admin
   of their new organization.
2. **Log in / log out (US-1.2 / US-1.3)**: Clerk hosted UI or React
   components handle the form; backend verifies the Clerk session JWT.
3. **Password reset (US-1.4)**: Clerk's built-in flow. We handle no
   tokens, no emails, no rate limiting.
4. **Invite Site Director (US-6.1)**: Org Admin sends a Clerk
   organization invitation; Clerk emails the invitee; on accept, webhook
   creates a `users` row in our DB with `role=site_director`.
5. **Deactivate user (US-6.2)**: Org Admin removes the membership in
   Clerk + flips `users.is_active = false`. All sessions invalidated.
6. **Resend invitation (US-6.3)**: Org Admin re-issues via Clerk API.
7. **Profile updates (US-1.5)**: handled in Clerk's `<UserProfile>`
   component; webhook syncs `full_name` / `email` back to `users`.

## Architecture

### High-level picture

```
                  Browser
                     │
       ┌─────────────┴──────────────┐
       │                             │
   Clerk-hosted UI                  React SPA
   (sign-in/sign-up/                (uses @clerk/clerk-react;
    user-profile)                    SignedIn / SignedOut /
       │                             OrganizationSwitcher)
       │                             │
       │                             │ Authorization: Bearer <Clerk JWT>
       └────────────►  api.clerk.com │ (or __session cookie)
                                     ▼
                              Liveaboard API (Go)
                                     │
                              Verify JWT (JWKS)
                                     │
                              Look up users row by clerk_user_id
                                     │
                              Attach (user, org) to ctx
                                     │
                                  Handlers
                                     │
                                Postgres
                                  ▲
                                  │ webhook (svix-signed)
                              api.clerk.com
                              (user.created, organization.created,
                               organizationMembership.created/deleted,
                               user.updated, user.deleted, ...)
```

### Identity model — what each side owns

| Concern | Clerk | Our DB |
|---|---|---|
| Email + password (or passwordless) credentials | ✅ | ❌ (column dropped) |
| Email verification | ✅ | ❌ |
| Password reset | ✅ | ❌ |
| Sessions | ✅ (JWT, server-side revocable) | ❌ (table dropped) |
| MFA / WebAuthn (future) | ✅ | ❌ |
| Invitation lifecycle | ✅ | ❌ |
| User identity (name, email) | ✅ (source) | ✅ (mirrored from webhook) |
| Organization identity | ✅ | ✅ (mirrored) |
| Org membership / role | ✅ | ✅ (mirrored, app uses local) |
| Domain data (boats, trips, ledger) | ❌ | ✅ |

The pattern: **`users` and `organizations` remain canonical for app data**.
Clerk owns identity. A webhook keeps our tables in sync. Our backend
queries our DB; we never call Clerk in a hot path beyond the JWT
verification (which is JWKS-based and offline-cacheable).

### Schema changes

```sql
-- 0002_auth_provider_clerk.sql

-- 1. Add Clerk linkage; nullable during cutover, NOT NULL after.
ALTER TABLE users           ADD COLUMN clerk_user_id   text UNIQUE;
ALTER TABLE organizations   ADD COLUMN clerk_org_id    text UNIQUE;

-- 2. Drop columns Clerk now owns.
ALTER TABLE users DROP COLUMN password_hash;
ALTER TABLE users DROP COLUMN email_verified_at;

-- 3. Drop tables Clerk now owns.
DROP TABLE sessions;
DROP TABLE email_verifications;

-- 4. Make linkage required.
ALTER TABLE users         ALTER COLUMN clerk_user_id  SET NOT NULL;
ALTER TABLE organizations ALTER COLUMN clerk_org_id   SET NOT NULL;
```

We are still pre-customer; this is a destructive migration and that is
acceptable. The migration is destructive on purpose — no dual-running
state. (See the wipe-dev-data note in the cutover phase.)

### Backend layout

```
internal/auth/
  clerk.go             # Clerk client wrapper (session verifier + admin SDK calls)
  middleware.go        # NEW: requireSession reads JWT, verifies, looks up user
  webhook.go           # NEW: /api/webhooks/clerk handler with svix signature verify
  webhook_handlers.go  # NEW: per-event sync logic (user.created -> insert users row)
  stub.go              # NEW: offline test/dev stub that issues fake JWTs (LIVEABOARD_AUTH_STUB=1)
  auth.go              # DELETED (superseded)
  auth_test.go         # DELETED
  middleware_test.go   # NEW
  webhook_test.go      # NEW

internal/store/
  sessions.go          # DELETED
  email_verifications.go # DELETED
  users.go             # KEEP — add CreateExternalUser, UpdateExternalUser
  organizations.go     # NEW — split from users.go; add CreateExternal/UpdateExternal

internal/httpapi/
  httpapi.go           # MODIFIED — strip signup/login/logout/verify-email; keep /me, /organization;
                       # add /webhooks/clerk; replace requireSession to use auth/middleware.go

internal/config/
  config.go            # ADD: ClerkSecretKey (secret), ClerkPublishableKey, ClerkWebhookSecret (secret),
                       #      ClerkJWKSURL (derived), ClerkInstanceID; AuthStubEnabled bool (test only).
```

### Frontend layout

```
web/src/
  main.tsx             # MODIFIED — wrap <App> in <ClerkProvider publishableKey={...}>
  pages/
    Signup.tsx         # REPLACED — renders <SignUp> from @clerk/clerk-react
    Login.tsx          # REPLACED — renders <SignIn>
    VerifyEmail.tsx    # DELETED — Clerk handles verification
    Dashboard.tsx      # MODIFIED — uses useUser/useOrganization hooks; logout via signOut()
  lib/
    api.ts             # MODIFIED — fetch wrapper attaches Clerk session token (Bearer)
    RequireSession.tsx # REPLACED — uses Clerk's <SignedIn>/<SignedOut> and <RedirectToSignIn>
    config.ts          # MODIFIED — adds VITE_CLERK_PUBLISHABLE_KEY
```

### Local dev

Clerk has a "development instance" mode that ships test API keys, a
testable hosted UI, and a "test email address" pattern (e.g. anything
ending `+clerk_test@example.com` is auto-verified) that works fully
offline once JWKS is cached. For air-gapped tests we add an
`internal/auth/stub.go` that, when `LIVEABOARD_AUTH_STUB=1` (test mode
only), accepts a fake `Authorization: Bearer test-<uuid>` header and
returns a synthesized user. The stub is the only way `go test ./...`
runs without network access; production refuses to start with the stub
enabled.

## API Endpoints

### Removed

| Endpoint | Reason |
|---|---|
| `POST /api/signup` | Replaced by Clerk `<SignUp>` flow + webhook. |
| `POST /api/verify-email` | Clerk owns verification. |
| `POST /api/login` | Replaced by Clerk `<SignIn>`. |
| `POST /api/logout` | Replaced by Clerk `signOut()`. |

### Added

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/webhooks/clerk` | POST | Receive Clerk events; svix-verify signature; sync users/organizations. |

### Kept (semantics unchanged from frontend's perspective)

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/me` | GET | Returns local user row joined to org. Now requires Clerk JWT in `Authorization`. |
| `/api/organization` | GET | Org dashboard. Same as before; auth flips. |

## Implementation Plan

### Phase 1: Provider proof-of-concept + decision lock-in (~10%)

**Files:** none code-wise; this phase is the spike.

**Tasks:**
- [ ] Create a Clerk development instance; capture publishable key, secret key, JWKS URL, webhook signing secret.
- [ ] Manually verify a Clerk-issued JWT in a one-off Go test (JWKS fetch + verify).
- [ ] Confirm Organizations are enabled and invite emails fire from the dev instance.
- [ ] Lock the decision in `docs/decisions/0001-auth-provider.md` (ADR).

### Phase 2: Backend integration (~25%)

**Files:** `internal/auth/clerk.go`, `internal/auth/middleware.go`, `internal/auth/stub.go`, `internal/auth/middleware_test.go`, `internal/config/config.go`, `cmd/server/main.go`.

**Tasks:**
- [ ] Add Clerk config fields to `Config` (publishable key, secret key, webhook secret, JWKS URL — secret-tagged where appropriate).
- [ ] Implement `clerk.Client` wrapper with: JWT verifier (JWKS-cached), `CreateOrganization`, `CreateUser`, `RemoveOrganizationMembership`, `RevokeSession`.
- [ ] Implement new `requireSession` middleware: extract Bearer token, verify with JWKS, resolve `users` row by `clerk_user_id`, attach to ctx. Maintain the existing `userFromContext` shape so handlers don't change.
- [ ] Implement stub verifier behind `cfg.AuthStubEnabled` (test mode only).
- [ ] Wire Clerk client into `httpapi.Server`; replace old auth wiring in `cmd/server/main.go`.
- [ ] Tests: middleware accepts valid JWT, rejects expired, rejects missing, returns 401 on unknown user; stub mode behavior; production refuses stub.

### Phase 3: Webhook receiver + sync (~20%)

**Files:** `internal/auth/webhook.go`, `internal/auth/webhook_handlers.go`, `internal/auth/webhook_test.go`, `internal/store/users.go` (extensions), `internal/store/organizations.go` (split).

**Tasks:**
- [ ] Implement `POST /api/webhooks/clerk` with svix signature verification. Reject any unsigned request.
- [ ] Handle events: `user.created`, `user.updated`, `user.deleted`, `organization.created`, `organization.updated`, `organization.deleted`, `organizationMembership.created`, `organizationMembership.updated`, `organizationMembership.deleted`.
- [ ] Idempotency: a `webhook_events(id text PRIMARY KEY, received_at timestamptz)` table; we no-op if `id` already seen.
- [ ] Map Clerk org roles → our `users.role` (`admin` → `org_admin`, `director` → `site_director`, `member` → `crew`).
- [ ] Tests with replayed Clerk webhook payloads (fixtures in `internal/auth/testdata/`); signature-verification tests.

### Phase 4: Schema migration + table teardown (~10%)

**Files:** `internal/store/migrations/0002_auth_provider_clerk.sql`, repository updates in `internal/store/users.go`, `internal/store/sessions.go` (delete), `internal/store/email_verifications.go` (delete).

**Tasks:**
- [ ] Write migration (see schema above) with non-trivial Down (it's a destructive migration; Down recreates empty tables but cannot recover data — comment as such).
- [ ] Add `webhook_events` table (idempotency) in the same migration.
- [ ] Delete `internal/store/sessions.go`, `internal/store/email_verifications.go`, all references; remove session repository methods from `Pool`.
- [ ] Update `users.go` with `clerk_user_id` field and helpers.

### Phase 5: Frontend integration (~20%)

**Files:** `web/src/main.tsx`, `web/src/pages/{Signup,Login,Dashboard}.tsx`, `web/src/lib/RequireSession.tsx`, `web/src/lib/api.ts`, `web/src/lib/config.ts`, `web/package.json`.

**Tasks:**
- [ ] Add `@clerk/clerk-react` dependency.
- [ ] Wrap `<App>` in `<ClerkProvider publishableKey={appConfig.clerkPublishableKey}>`.
- [ ] Replace Signup/Login pages with `<SignUp>` / `<SignIn>` Clerk components, styled per DESIGN.md (Clerk's `appearance` API).
- [ ] Replace `RequireSession` with Clerk's `<SignedIn>` / `<SignedOut>` + `<RedirectToSignIn>`.
- [ ] Update `api.ts` to attach Clerk session token via `useAuth().getToken()` (or pass it in via a hook + closure).
- [ ] Delete `VerifyEmail.tsx`.
- [ ] Dashboard uses `useUser()` / `useOrganization()` for identity, plus our `/api/me` and `/api/organization` for app data.

### Phase 6: Cutover, cleanup, docs (~15%)

**Files:** `internal/auth/auth.go` (DELETE), `internal/auth/auth_test.go` (DELETE), `docs/auth.md`, `docs/decisions/0001-auth-provider.md`, `RUNNING.md`, `CHANGELOG.md` (if present), `.env.example`, `config/{dev,test,production}.env`.

**Tasks:**
- [ ] Wipe local dev DB; reseed by signing up via Clerk in dev.
- [ ] Delete `internal/auth/auth.go` and `internal/auth/auth_test.go`.
- [ ] Add Clerk env vars to `.env.example` (with secret markers) and to `config/dev.env` / `config/production.env` (publishable key only — secrets stay env-only).
- [ ] Write `docs/auth.md`: how Clerk is wired, where to rotate keys, how to add a new webhook event handler, how to escape Clerk if we ever need to.
- [ ] Update `RUNNING.md` with the Clerk setup steps for new contributors.
- [ ] `gofmt`, `go vet`, `go test`, `npm run build` clean.
- [ ] Manual smoke: signup via Clerk → webhook fires → `users` row exists → invite a Site Director → accept invite → second `users` row exists → both can log in → org admin deactivates the second user → second user cannot log in.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/decisions/0001-auth-provider.md` | Create | ADR locking in Clerk; rationale and rejected alternatives. |
| `docs/auth.md` | Create | Operational guide for Clerk integration (rotation, webhooks, escape hatch). |
| `internal/auth/clerk.go` | Create | Clerk client wrapper. |
| `internal/auth/middleware.go` | Create | New `requireSession` using Clerk JWT verification. |
| `internal/auth/stub.go` | Create | Offline/test-mode fake verifier. |
| `internal/auth/webhook.go` | Create | Svix-signed webhook receiver. |
| `internal/auth/webhook_handlers.go` | Create | Per-event sync logic. |
| `internal/auth/middleware_test.go` | Create | JWT verification, stub mode, 401 paths. |
| `internal/auth/webhook_test.go` | Create | Replay + signature verification tests. |
| `internal/auth/testdata/*.json` | Create | Captured Clerk webhook payloads. |
| `internal/auth/auth.go` | Delete | Superseded. |
| `internal/auth/auth_test.go` | Delete | Superseded. |
| `internal/store/migrations/0002_auth_provider_clerk.sql` | Create | Schema cutover. |
| `internal/store/users.go` | Modify | Add `clerk_user_id`; add `CreateExternalUser`, `UpdateExternalUser`, `DeactivateUser`. |
| `internal/store/organizations.go` | Create | Split from users.go; add Clerk-linked CRUD. |
| `internal/store/sessions.go` | Delete | Clerk owns sessions. |
| `internal/store/email_verifications.go` | Delete | Clerk owns verification. |
| `internal/store/store.go` | Modify | Drop session/verification accessors. |
| `internal/httpapi/httpapi.go` | Modify | Strip auth handlers; mount webhook; swap middleware. |
| `internal/httpapi/httpapi_test.go` | Modify | Replace auth-handler tests with webhook + middleware tests. |
| `internal/config/config.go` | Modify | Add Clerk fields (some `secret`, all documented). |
| `cmd/server/main.go` | Modify | Wire Clerk client; remove old auth.Service construction. |
| `web/src/main.tsx` | Modify | `<ClerkProvider>` at the root. |
| `web/src/pages/Signup.tsx` | Replace | `<SignUp>` Clerk component. |
| `web/src/pages/Login.tsx` | Replace | `<SignIn>` Clerk component. |
| `web/src/pages/VerifyEmail.tsx` | Delete | Clerk handles. |
| `web/src/pages/Dashboard.tsx` | Modify | Use Clerk hooks; logout via `signOut()`. |
| `web/src/lib/RequireSession.tsx` | Replace | Use `<SignedIn>` / `<RedirectToSignIn>`. |
| `web/src/lib/api.ts` | Modify | Bearer token from Clerk session. |
| `web/src/lib/config.ts` | Modify | `VITE_CLERK_PUBLISHABLE_KEY`. |
| `web/package.json` | Modify | Add `@clerk/clerk-react`. |
| `.env.example` | Modify | Document Clerk keys. |
| `config/dev.env` | Modify | Non-secret Clerk values (publishable key, instance URL). |
| `config/production.env` | Modify | Non-secret Clerk values; secrets remain env-only. |
| `RUNNING.md` | Modify | New contributor Clerk setup steps. |
| `Makefile` | Modify | Optional `make stub-test` target that runs Go tests with `LIVEABOARD_AUTH_STUB=1`. |
| `docs/sprints/SPRINT-005.md` | Create | This sprint document (after merge). |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 005. |

## Definition of Done

- [ ] ADR `docs/decisions/0001-auth-provider.md` exists and lists the chosen provider, rejected alternatives, and the rationale.
- [ ] `internal/auth/auth.go` and `internal/auth/auth_test.go` are deleted; `git grep "bcrypt"` returns nothing in `internal/`.
- [ ] `internal/store/sessions.go` and `internal/store/email_verifications.go` are deleted.
- [ ] Migration `0002_auth_provider_clerk.sql` runs cleanly on a fresh `liveaboard` and `liveaboard_test` database.
- [ ] `users.clerk_user_id` and `organizations.clerk_org_id` are NOT NULL UNIQUE.
- [ ] All `/api/*` endpoints behind `requireSession` accept a Clerk JWT and resolve to a local `users` row; an unsigned/expired token returns 401.
- [ ] `POST /api/webhooks/clerk` rejects a missing or invalid svix signature; accepts a valid one; is idempotent on replay.
- [ ] Webhook handlers cover the 9 events listed in Phase 3, each with at least one test.
- [ ] Backend tests pass with `LIVEABOARD_AUTH_STUB=1` (no internet required); production refuses to start with stub enabled.
- [ ] Frontend builds (`npm run build`) and uses Clerk components; DESIGN.md tokens are applied via Clerk `appearance`.
- [ ] Manual smoke documented in Phase 6 passes end-to-end against the dev Clerk instance.
- [ ] All Must-priority backlog items unblocked (US-1.1, US-1.2, US-1.3, US-1.4, US-6.1, US-6.2) have a "delivered by Clerk feature X" line in `docs/auth.md`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...` clean. `npm run build` clean.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Vendor lock-in (Clerk) | Medium | High | Keep `users` and `organizations` as canonical app tables; treat Clerk as a credential provider; webhook-driven sync means leaving = "swap the verifier + webhook + import users into a new provider". |
| Clerk outage blocks login | Low (per their SLA) | High | Document as accepted risk for MVP; status page subscription; future task: a brief grace window where existing JWTs still verify against cached JWKS. |
| Webhook reliability | Medium | High | Idempotency table + svix retries + alert on >5min lag in user_created events; cron reconciler script (out of scope this sprint, captured as follow-up). |
| Cost surprise above free tier | Low | Medium | Free tier is 10K MAU; alert at 8K via Clerk dashboard; revisit before crossing. |
| JWT verification needs JWKS network round-trip | Low | Medium | Library does an in-memory JWKS cache; refresh interval set to 1h; misconfig caught in startup health check. |
| Breakage of an in-flight branch (Sprint 003 follow-ups) | Medium | Low | Land this sprint as one PR; coordinate with anyone holding open auth-related branches. |
| Clerk Organizations one-user-per-org assumption broken | Low | Medium | Clerk allows multi-org per user; we explicitly forbid it in our webhook handler (return 200 + log if a user joins a second org we don't expect). |
| Local-dev contributors hate "you need a Clerk account" | Medium | Low | Stub mode (`LIVEABOARD_AUTH_STUB=1`) lets `go test` and a fake login work fully offline. |
| Migration drops password_hash before all dev users have re-enrolled | High | Low | Wipe the dev DB; we are pre-customer; documented in the migration comment and `docs/auth.md`. |
| Hosted Clerk UI doesn't match DESIGN.md | Medium | Low | Use Clerk's `appearance` API to inject our tokens (color, font, radii); fall back to embedded `<SignIn>` inside our own page chrome if needed. |

## Security Considerations

- All Clerk secrets (`CLERK_SECRET_KEY`, `CLERK_WEBHOOK_SECRET`) flow through the Sprint 004 `Config` system, tagged `secret` and `required`. Production refuses to start without them.
- Webhook endpoint validates svix signatures *before* parsing the body; an unsigned POST is rejected with 401 and is not logged with body content.
- JWT verification uses Clerk's JWKS over HTTPS; we pin the issuer and audience claims to our specific Clerk instance.
- `users.clerk_user_id` is unique; a webhook trying to register a duplicate is rejected with a logged error.
- We never persist Clerk session tokens; they're verified per request and discarded.
- Logout invalidation: Clerk's `signOut()` revokes the session; backend never relies on a local revocation list. (No more `sessions` table to keep in sync.)
- Multi-tenant isolation continues at the data layer (`organization_id`); Clerk's identity does not bypass our row-level scoping. The fact that Clerk knows about an org never grants implicit access at the DB layer.
- Stub mode is feature-gated; `MustLoad` in production mode rejects `AuthStubEnabled=true`.

## Dependencies

- **Sprint 003** — provides the auth/store/httpapi structure this sprint surgically modifies.
- **Sprint 004** — provides the typed `Config` system used for new Clerk secrets.
- **External: Clerk dev instance** — created in Phase 1; project owner registers the account.
- **New Go module deps**: `github.com/clerk/clerk-sdk-go/v2`, `github.com/svix/svix-webhooks/go` (for signature verification). Both are mainstream; verified for license + maintenance before merge.
- **New npm dep**: `@clerk/clerk-react`.

## Out of Scope

- MFA / TOTP / WebAuthn (Clerk supports it; we enable it in a follow-up sprint when the product calls for it).
- SSO / SAML / SCIM (Clerk supports it on a paid tier; defer until a customer asks).
- A reconciler cron that catches webhook-loss drift (captured as a follow-up).
- Multi-org-per-user UX (Clerk supports it; we keep our 1:1 invariant).
- Replacing Clerk's hosted UI with fully custom forms (we use Clerk components inside our own chrome — that's enough).
- Migrating real production users (there are none; we wipe dev data).
- Fall-forward to WorkOS (named as an alternate; not implemented).

## Open Questions

These remain for the merge phase, the interview, or a Phase-1 spike:

1. Do we use **Clerk's hosted UI URL** (`accounts.<our-domain>.clerk.accounts.dev`) or **embedded React components**? Recommendation: embedded, for design control.
2. Do we want **per-org Clerk RBAC** to be the source of truth for app role, or do we keep `users.role` as authoritative and treat Clerk's role as a mirror? Recommendation: keep `users.role` authoritative; Clerk role is for hosted UI display only.
3. Do we adopt **Clerk Organizations** or just have flat user lists with our own `users.organization_id` linkage? Recommendation: adopt Organizations — the invitation feature alone is worth it.
4. Webhook delivery: do we add a **secondary safety-net cron** that reconciles users via Clerk's list API once an hour? Recommendation: defer, capture as follow-up.
5. Do we ship a **`make seed-clerk`** script that creates a dev org + admin in our DB so contributors don't have to do the signup dance every time they wipe? Recommendation: yes, in Phase 6.
