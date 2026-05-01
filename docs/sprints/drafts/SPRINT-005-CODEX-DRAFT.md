# Sprint 005: WorkOS Migration for Authentication and User Lifecycle

## Overview

Sprint 003 proved that the product can stand up its own auth stack, but it also exposed the exact surface area we would now need to own indefinitely: password storage rules, session invalidation, verification email delivery, invitation lifecycle, password reset, rate limiting, and eventually enterprise requirements like SSO and SCIM. Sprint 004 then made runtime configuration disciplined enough that we can safely integrate an external identity provider without scattering secrets or ad-hoc wiring across the codebase. The next blocked backlog items are not abstract future ideas; they are concrete user-management and recovery flows that all depend on real email and identity lifecycle tooling.

This sprint resolves the build-vs-buy decision in favor of **buy** and implements the migration. The primary recommendation is **WorkOS User Management + AuthKit**. The secondary fallback is **Stytch B2B**. WorkOS is the best fit for this repo because it is B2B-first, has a current Go SDK, supports invitations and organization memberships, offers an Events API specifically for syncing app data, and has a favorable current price curve for an MVP: the pricing page currently advertises User Management as free up to 1 million users, with enterprise add-ons layered separately. That matters here because we need email/password and invitations immediately, but we also do not want to paint ourselves into a corner if enterprise SSO becomes important later.

The migration should **not** move tenancy or domain roles out of our schema. `users.organization_id` and `users.role` remain the canonical application authorization model. WorkOS becomes the system of record for credentials, email verification, password recovery, invitations, and provider-managed session issuance. Our backend remains the policy enforcement point for application data. The frontend keeps the same high-level contract of “same-origin cookie auth + `/api/me` probe”, but the cookie contents and backend verification path change from local opaque sessions to provider-backed session tokens. Because the current environment is local-only and current users are few throwaway dev accounts, the migration can take the clean path: snapshot existing users for reference, wipe local credentials/session state, and cut directly to the provider-backed auth path with no long-lived dual auth mode in `main`.

## Decision Summary

| Option | Verdict | Why |
|------|--------|---------|
| Extend custom auth | Reject | Would deliver this sprint’s missing flows, but keeps us owning email deliverability, abuse controls, invitation lifecycle, and future enterprise auth surface. |
| WorkOS User Management + AuthKit | **Recommend** | B2B-first model, invitations, org memberships, Go SDK, Events API for sync, staged/test environments, strong pricing for MVP, clean path to future SSO/SCIM. |
| Stytch B2B | Viable fallback | Strong B2B primitives and transparent free tier, but a slightly less natural fit for our “Go backend remains the center” shape than WorkOS’s sync/event model. |
| Clerk | Not primary | Very good developer experience and B2B add-on, but pushes harder toward Clerk-managed frontend patterns and adds more product coupling around session/UI conventions than we need. |
| Supabase Auth | Not primary | Solid standalone auth, but it pulls us toward Supabase’s broader data/runtime model and its organization story is weaker for this product shape. |
| Auth0 | Not primary | Capable, but comparatively expensive and broader than needed for this stage. |
| Ory Kratos | Not primary | Preserves control and avoids vendor lock-in, but pushes operational burden, email plumbing, and security ergonomics back onto us. |

## Use Cases

1. **Recover account access**: an Org Admin can trigger password reset without revealing whether an email exists, receive a provider-managed recovery email, set a new password, and have old sessions revoked.
2. **Invite a Site Director**: an Org Admin can invite a user by email, the invitee receives a provider-managed invitation email, accepts it, and lands in the correct organization with the correct app role.
3. **Resend or revoke an invitation**: the app can list pending invitations, resend them, and deactivate memberships without re-implementing token lifecycle internally.
4. **Keep the app’s auth probe stable**: the frontend still uses same-origin cookies and `GET /api/me` to determine whether the browser has an authenticated session.
5. **Preserve app-level authorization**: authenticated requests still resolve to a local `users` row with `organization_id` and `role`, so business logic remains in our backend instead of leaking into the provider.
6. **Support future enterprise auth without another rewrite**: if B2B customers later require SSO or SCIM, the chosen provider already has a product path for it.

## Architecture

### Strategic boundary

```
Credentials / recovery / invitations / provider session issuance
    -> WorkOS

Application user, tenant, role, and data authorization
    -> liveaboard Postgres + Go backend
```

We explicitly do **not** let the provider become the only source of truth for tenant membership semantics. WorkOS memberships are integration data; `users.organization_id` is the application authority.

### Target auth flow

```
Browser
  |
  | same-origin requests with provider-backed session cookie
  v
Go HTTP API
  |
  | verify provider session token / refresh if needed
  v
Auth adapter
  |
  +--> WorkOS User Management
  |       - sign-in / sign-up / verification / recovery
  |       - invitations / memberships
  |       - session validation
  |
  +--> Local user sync + role mapping
          - users.organization_id
          - users.role
          - users.auth_provider_subject
          - users.is_active
```

### Data ownership model

```
WorkOS user.id --------------------+
                                   |
WorkOS org membership / org id ----+--> local users.auth_subject / users.auth_membership_id
                                   |
Provider events -------------------+

local users table remains canonical for:
  - organization_id
  - role
  - active/inactive state for app access
  - downstream FK relationships
```

### Sync and reconciliation

```
WorkOS Events API poller
   |
   +--> auth_sync_cursors
   +--> reconcile local users rows
   +--> deactivate local access on membership deactivation
   +--> keep audit trail for auth-related mutations
```

Using the Events API instead of provider webhooks is the right fit for this repo because it keeps local development simple and matches WorkOS’s own recommendation for data syncing.

### HTTP/API compatibility plan

The frontend contract should remain intentionally small:

- `GET /api/me` still returns the current local user shape.
- Protected routes still rely on cookie auth.
- `POST /api/logout` still exists and clears the browser cookie, but now also signs the user out of the provider session.

The auth entrypoints are allowed to change more substantially:

- `POST /api/signup`
- `POST /api/login`
- `POST /api/verify-email`

For WorkOS, the cleanest end state is that these become orchestration endpoints or redirect/callback helpers around provider flows rather than local credential handlers. We should prefer provider-hosted verification/recovery/invitation handling over re-implementing those flows in our UI.

## Implementation Plan

### Phase 1: Provider integration skeleton and schema changes (~20%)

**Files:**
- `internal/config/config.go` - add WorkOS config surface.
- `config/dev.env`, `config/test.env`, `config/production.env` - non-secret knobs for provider integration.
- `docs/CONFIG.md` - document provider keys and callback URLs.
- `internal/store/migrations/0002_auth_provider_workos.sql` - add provider mapping and sync tables.
- `internal/store/users.go` - local user/provider lookup and mutation helpers.
- `internal/store/store_test.go` - migration coverage for new tables/columns.

**Tasks:**
- [ ] Add provider settings to typed config:
  - `LIVEABOARD_AUTH_PROVIDER=workos`
  - `WORKOS_API_KEY` (secret)
  - `WORKOS_CLIENT_ID` (secret-adjacent, env only for simplicity)
  - `WORKOS_COOKIE_PASSWORD` or equivalent local encryption/signing secret if we keep refresh material locally
  - callback URLs / app base URL
- [ ] Add schema fields to `users` for provider identity linkage, for example:
  - `auth_provider text`
  - `auth_subject text`
  - `auth_membership_id text`
  - `invited_at`, `deactivated_at` as needed
- [ ] Make `users.password_hash` nullable during migration, but do not delete it until cutover is complete.
- [ ] Add `auth_sync_cursors` table for Events API progress.
- [ ] Add uniqueness constraints around provider subject and membership identifiers.

### Phase 2: Auth adapter and session verification path (~25%)

**Files:**
- `internal/auth/auth.go` - convert from credential owner to provider adapter/facade.
- `internal/auth/auth_test.go` - replace local-password assumptions with adapter contract tests.
- `internal/httpapi/httpapi.go` - provider-backed session middleware and updated auth endpoints.
- `cmd/server/main.go` - wire provider client and auth adapter.
- `internal/httpapi/httpapi_test.go` - endpoint behavior under fake provider.
- `internal/auth/workos.go` - concrete WorkOS adapter.
- `internal/auth/fakeprovider.go` - offline test double.

**Tasks:**
- [ ] Introduce an internal provider interface so HTTP handlers and most auth logic do not depend directly on WorkOS SDK types.
- [ ] Implement WorkOS-backed session validation in middleware.
- [ ] Keep `GET /api/me` and protected-route behavior stable.
- [ ] Replace local session-table lookups with provider session verification plus local user resolution.
- [ ] Keep `POST /api/logout` as an app endpoint that clears local cookie state and terminates the provider session.
- [ ] Build an offline fake provider so `go test ./...` does not require internet access.

### Phase 3: Local user synchronization and authorization mapping (~20%)

**Files:**
- `internal/store/users.go`
- `internal/auth/sync.go`
- `internal/auth/sync_test.go`
- `internal/store/migrations/0002_auth_provider_workos.sql`

**Tasks:**
- [ ] Define the rule that local `users` remains canonical for `organization_id`, `role`, and `is_active`.
- [ ] On first successful provider authentication, upsert the local user by provider subject.
- [ ] Map provider membership/org context to a local organization row; do not infer orgs from email domain.
- [ ] Mirror provider membership deactivation to local `is_active=false`.
- [ ] Add a sync path for provider events so invite acceptance and deactivation can reconcile even if they happen outside the current browser flow.
- [ ] Preserve role mapping explicitly:
  - WorkOS organization role slugs may mirror `org_admin`, `site_director`, `crew`
  - local `users.role` remains the value business logic checks

### Phase 4: Invitation, recovery, and profile-management flows (~20%)

**Files:**
- `internal/httpapi/httpapi.go`
- `internal/auth/workos.go`
- `internal/store/users.go`
- `web/src/lib/api.ts`
- `web/src/lib/RequireSession.tsx`
- `web/src/pages/Signup.tsx`
- `web/src/pages/Login.tsx`
- `web/src/pages/VerifyEmail.tsx`
- `web/src/pages/Dashboard.tsx`

**Tasks:**
- [ ] Replace the local verification-token UX with provider-managed verification and callback handling.
- [ ] Implement app endpoints for:
  - invite Site Director
  - resend invitation
  - deactivate user / membership
  - start password recovery
  - complete logout
- [ ] Decide deliberately where UI remains custom versus redirects to provider-hosted screens.
- [ ] Keep the current auth pages if practical, but reduce them to safe orchestration shells rather than password-processing owners.
- [ ] Ensure error behavior remains non-enumerating at the app boundary.

### Phase 5: Migration execution and dev cutover (~10%)

**Files:**
- `internal/store/migrations/0003_auth_cutover_cleanup.sql`
- `scripts/workos-dev-bootstrap.sh`
- `docs/AUTH-MIGRATION.md`
- `docs/CONFIG.md`

**Tasks:**
- [ ] Export or snapshot current dev users for reference only.
- [ ] Choose the explicit migration rule for current local users: **force fresh provider enrollment / password reset, not password-hash migration**.
- [ ] Clear local sessions and email verification rows during cutover.
- [ ] Remove the old local login path from active routing once WorkOS flow is live in dev.
- [ ] Decide whether to drop `sessions` and `email_verifications` in this sprint or leave them inert for one sprint with a documented removal follow-up. Preferred: remove them if the branch lands cleanly.

### Phase 6: Verification, tests, and documentation (~5%)

**Files:**
- `internal/auth/auth_test.go`
- `internal/httpapi/httpapi_test.go`
- `docs/CONFIG.md`
- `docs/AUTH-MIGRATION.md`
- `RUNNING.md`

**Tasks:**
- [ ] Add contract tests for provider adapter behavior using the fake provider.
- [ ] Add integration tests for `/api/me`, logout, invite, resend-invite, deactivate-user, and recovery-initiation endpoints.
- [ ] Document local dev setup with WorkOS staging keys and localhost callback URLs.
- [ ] Run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `npm --prefix web run build`.

## API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/me` | `GET` | Return the current local app user resolved from the provider-backed session. |
| `/api/logout` | `POST` | Clear the app cookie and terminate provider session state. |
| `/api/invitations` | `POST` | Invite a Site Director or crew member through WorkOS. |
| `/api/invitations/{id}/resend` | `POST` | Resend a pending invitation. |
| `/api/users/{id}/deactivate` | `POST` | Deactivate a local user and corresponding provider membership. |
| `/api/auth/recovery/start` | `POST` | Initiate password recovery without email enumeration. |
| `/api/auth/callback` | `GET` | Handle provider redirect/callback and establish app session state. |

Existing endpoints like `/api/signup`, `/api/login`, and `/api/verify-email` may remain as compatibility shims, but they should terminate in provider-managed flows, not local credential processing.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/config/config.go` | Modify | Add provider settings and validation. |
| `config/dev.env` | Modify | Add non-secret provider defaults and localhost callback URLs. |
| `config/test.env` | Modify | Add test-mode provider settings for fake/integration flows. |
| `config/production.env` | Modify | Add production-mode provider knobs. |
| `docs/CONFIG.md` | Modify | Document WorkOS settings and local dev contract. |
| `internal/store/migrations/0002_auth_provider_workos.sql` | Create | Provider linkage, sync cursor, and migration columns/tables. |
| `internal/store/migrations/0003_auth_cutover_cleanup.sql` | Create | Remove or retire obsolete local auth storage. |
| `internal/store/users.go` | Modify | Provider lookup/upsert/deactivation helpers. |
| `internal/auth/auth.go` | Modify | Convert auth service into provider-facing facade. |
| `internal/auth/workos.go` | Create | Concrete WorkOS adapter. |
| `internal/auth/fakeprovider.go` | Create | Offline fake provider for tests. |
| `internal/auth/sync.go` | Create | Events API synchronization and reconciliation. |
| `internal/auth/auth_test.go` | Modify | Adapter and migration-focused tests. |
| `internal/auth/sync_test.go` | Create | Event reconciliation tests. |
| `internal/httpapi/httpapi.go` | Modify | Provider-backed middleware and new invitation/recovery endpoints. |
| `internal/httpapi/httpapi_test.go` | Modify | HTTP behavior under provider-backed auth. |
| `cmd/server/main.go` | Modify | Wire provider client and adapter. |
| `web/src/lib/api.ts` | Modify | Match new auth/invite endpoints and callback flow. |
| `web/src/lib/RequireSession.tsx` | Modify | Preserve `/api/me` session probe behavior. |
| `web/src/pages/Signup.tsx` | Modify | Redirect/orchestrate provider signup. |
| `web/src/pages/Login.tsx` | Modify | Redirect/orchestrate provider login. |
| `web/src/pages/VerifyEmail.tsx` | Modify | Provider callback / confirmation UX. |
| `web/src/pages/Dashboard.tsx` | Modify | Add user-management entry points if included here. |
| `scripts/workos-dev-bootstrap.sh` | Create | Local provider setup helper. |
| `docs/AUTH-MIGRATION.md` | Create | Step-by-step migration and rollback instructions. |
| `RUNNING.md` | Modify | Update local setup workflow. |

## Definition of Done

- [ ] Build-vs-buy is resolved in code and docs in favor of a single live provider-backed auth path.
- [ ] WorkOS is integrated in dev using staging/test credentials and documented localhost callbacks.
- [ ] `GET /api/me` still returns the current local app user and remains the frontend session probe.
- [ ] Password reset is provider-backed and no longer relies on logged local tokens.
- [ ] Site Director invitation, resend invitation, and deactivation flows are implemented against provider APIs.
- [ ] `users.organization_id` and `users.role` remain the canonical authorization source in the app.
- [ ] No local password verification remains in the active login path.
- [ ] Existing dev accounts are migrated via explicit re-enrollment/reset policy; no silent broken state remains.
- [ ] Tests cover provider adapter behavior and auth-protected HTTP behavior without needing internet access.
- [ ] `go vet ./...`, `go test ./...`, and `npm --prefix web run build` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Provider/org model leaks into app authorization and makes future exit hard | Medium | High | Keep `users.organization_id` and `users.role` canonical; store provider IDs only as integration references. |
| Hosted-provider flow causes more frontend churn than expected | Medium | Medium | Preserve `/api/me` and same-origin cookies; limit frontend changes to orchestration pages and callback handling. |
| Session refresh/verification logic becomes flaky | Medium | High | Isolate provider logic behind an adapter and cover with deterministic fake-provider tests. |
| Event sync drifts from provider truth | Low | High | Persist cursor, process events serially, and provide manual reconciliation tooling. |
| Current dev users are stranded by migration | Low | Medium | Use an explicit wipe/re-enroll plan and document it; dev users are intentionally treated as disposable. |
| WorkOS integration adds too much vendor coupling | Medium | Medium | Keep local user table authoritative and avoid storing business data only in provider metadata. |

## Security Considerations

- Provider secrets must follow the existing config discipline: env-only, typed config, production validation.
- Recovery and invitation flows must remain non-enumerating at the app boundary.
- Session verification must validate signature, expiry, issuer, audience/client, and organization context before resolving a local user.
- Local authorization must continue to use local org/role data; provider claims can inform but not replace backend checks.
- Logout must revoke provider session state, not just clear a browser cookie.
- Event sync endpoints or pollers must be idempotent and safe to replay.

## Dependencies

- Sprint 003 auth/domain baseline.
- Sprint 004 typed config and build system.
- `docs/product/organization-admin-user-stories.md` stories US-1.4, US-1.5, US-6.1, US-6.2, US-6.3.
- WorkOS account with staging credentials and configured redirect URLs.

## References

- [WorkOS User Management](https://workos.com/user-management)
- [WorkOS Pricing](https://workos.com/pricing)
- [WorkOS Invitations Docs](https://workos.com/docs/user-management/invitations)
- [WorkOS Sessions Docs](https://workos.com/docs/user-management/sessions/introduction)
- [WorkOS Events Data Syncing](https://workos.com/docs/events/data-syncing)
- [WorkOS Go SDK](https://workos.com/docs/sdks/go)
- [Stytch Pricing](https://stytch.com/pricing)
- [Stytch B2B Sessions Overview](https://stytch.com/docs/api-reference/b2b/api/sessions/overview)
- [Stytch Member Migration Guide](https://stytch.com/docs/b2b/guides/migrations/migrating-user-data)
- [Clerk Pricing](https://clerk.com/pricing)
- [Clerk Organization Invitations](https://clerk.com/docs/organizations/invitations)
- [Auth0 Pricing](https://auth0.com/pricing)
- [Auth0 Organizations Overview](https://auth0.com/docs/organizations/organizations-overview)
- [Supabase Auth Overview](https://supabase.com/docs/guides/auth)
- [Ory Kratos Overview](https://www.ory.sh/kratos)
