# Sprint 005 Intent: Authentication — Build vs Buy Decision (and Migration if "Buy")

## Seed

> Should we do our own authentication or rely on a third party? If so, please
> suggest one to use and how to migrate our code to the new provider.

## Context

Sprint 003 shipped a working custom auth stack and Sprint 004 cleaned up
configuration. We are now at the point where the next several backlog items
all require capabilities that custom auth has *not* yet built:

- **US-1.4 — Password reset** (Should, deferred from Sprint 003)
- **US-1.5 — Profile updates** (Should, deferred from Sprint 003)
- **US-6.1 — Invite Site Director** (Must — blocking for trip operations)
- **US-6.2 — Deactivate user** (Must)
- **US-6.3 — Resend invitation** (Should)

All of these need a real email channel. Sprint 003 stubbed email by logging
verification tokens; we have no SMTP integration, no template system, no
deliverability story, and no rate limiting.

This is the natural decision point: either invest a sprint extending custom
auth (email service + invitation flow + password reset + tests) and continue
owning the security surface forever, or migrate to a third-party identity
provider that ships those capabilities and reduces our long-term maintenance
load.

This sprint resolves that question and, if the answer is "buy", produces a
concrete migration plan. If the answer is "build", it produces the missing
capabilities (email + invitation + password reset).

## Recent Sprint Context

- **Sprint 002 — Org Admin user-story backlog**: documented the full Org
  Admin backlog. User-management group (US-6.x) is a Must dependency for
  Site Director assignment (US-4.10), which is itself a Must.
- **Sprint 003 — Auth + Organization Foundation**: built signup, login,
  logout, email verification (logged), and `GET /api/organization`. Used
  bcrypt + opaque session tokens (sha256 in DB) + `lb_session` HTTP-only
  cookie. Schema: `organizations`, `users`, `sessions`, `email_verifications`.
  Out of scope (deferred to this/later sprints): real email, password reset,
  profile updates.
- **Sprint 004 — Build & Configuration System**: introduced typed `Config`
  struct + mode files + `Makefile` + production binary build. Auth knobs
  (`BcryptCost`, `SessionDuration`, `VerificationDuration`) flow through
  config. This sprint must keep the same config discipline for any new
  provider settings.

## Relevant Codebase Areas

| Area | Files |
|---|---|
| Auth domain | `internal/auth/auth.go`, `internal/auth/auth_test.go` |
| Store layer | `internal/store/users.go`, `internal/store/sessions.go`, `internal/store/email_verifications.go` |
| HTTP layer | `internal/httpapi/httpapi.go` (signup/login/logout/verify-email/me + session middleware) |
| Schema | `internal/store/migrations/0001_init.sql` |
| Frontend | `web/src/pages/{Signup,Login,VerifyEmail,Dashboard}.tsx`, `web/src/lib/api.ts`, `web/src/lib/RequireSession.tsx` |
| Config | `internal/config/config.go`, `config/{dev,test,production}.env` |
| Persona model | `docs/product/personas.md`, `docs/product/organization-admin-user-stories.md` |

The current auth API surface (server side):

- `POST /api/signup` → creates org + first org_admin + verification token (logged)
- `POST /api/verify-email` → consumes verification token
- `POST /api/login` → bcrypt compare, sets `lb_session` cookie
- `POST /api/logout` → invalidates session row, clears cookie
- `GET  /api/me` → returns current user (id, email, full_name, role, organization_id)
- session middleware on all `/api/*` reads behind it

The frontend assumes same-origin cookies and a simple `/api/me` probe to
discover whether the user is signed in. Any chosen provider must keep that
contract working or migrate it deliberately.

## Constraints

- Must follow project conventions in CLAUDE.md (Go backend with minimal
  deps; tests required; gofmt/go vet clean; multi-tenant data isolation).
- Must integrate with the typed `Config` system from Sprint 004 — any new
  secrets (provider API keys, webhook signing secrets, SMTP credentials)
  follow the existing `required:secret` pattern and live only in process env.
- Must preserve the multi-tenant data model: every authenticated request
  must still resolve to a `users.organization_id`. We do not want to push
  tenancy semantics into the identity provider in a way we cannot reverse.
- Must preserve the existing app-level roles (`org_admin`, `site_director`,
  `crew`). The provider can store them in claims or we can keep them in
  `users.role` — either is acceptable, but the boundary must be explicit.
- Must keep DESIGN.md aesthetic for any auth-related UI (custom forms or
  hosted/embedded provider UI, whichever path we pick).
- No cloud deployment yet — local dev must continue to work. A "buy" path
  needs a workable local dev story (test API keys, local webhook tunnel, or
  a stub mode).
- Cost: prefer providers with a real free tier sufficient for the MVP and a
  pricing curve that does not punish multi-org B2B SaaS usage.
- Open-by-default operations data, but auth is sensitive — minimize PII the
  provider sees beyond what they need.

## Success Criteria

1. **A documented decision**: build vs buy is resolved with explicit
   rationale, scoring of options, and an architectural diagram for the
   chosen path.
2. **If buy**: a complete, testable migration plan covering provider
   selection, schema changes, dual-running strategy, password rehashing or
   re-enrollment, frontend changes, and rollback. Every backlog item that
   blocked this decision (US-1.4, US-1.5, US-6.1, US-6.2, US-6.3) has a
   line in the plan saying "delivered by provider feature X" or
   "implemented via provider API X".
3. **If build**: a complete, testable plan delivering email service +
   invitation flow + password reset + profile updates, with the same
   acceptance-criteria coverage. All Must items unblock.
4. **No partial migrations**: at end of sprint, exactly one auth path is
   live in dev. No half-cut-over state in main.
5. **Tests pass**: existing auth tests either continue to pass (build path)
   or are explicitly replaced by integration tests against the new
   provider in test mode (buy path).
6. **Cost & operational risk are documented**: pricing tier, vendor
   lock-in surface, escape hatch, blast radius if the provider is down.

## Open Questions

These are the questions the drafts must answer:

1. **Strategic**: are we a B2B SaaS that will eventually need enterprise
   SSO/SAML/SCIM (in which case WorkOS/Stytch/Clerk make sense), or a
   simpler B2C-flavored product (in which case Auth0/Supabase/Firebase
   compete differently, or self-hosting Ory/Keycloak is acceptable)?
2. **Identity model**: does the provider own the canonical user record, or
   do we keep `users` as the canonical table and the provider only owns
   credentials + sessions? The latter preserves more optionality but adds a
   sync seam.
3. **Multi-tenancy**: do we adopt the provider's "Organizations"
   primitive (Clerk, WorkOS, Stytch B2B all have one), or keep tenancy in
   our schema and treat the provider as plain user/auth?
4. **Sessions**: do we replace our cookie session with a provider-issued
   JWT/session? If so, the entire `sessions` table goes away — what does
   logout invalidation look like?
5. **Password migration**: existing users (very few in dev) have bcrypt
   hashes in `users.password_hash`. Do we re-hash on next login, force
   reset, or accept that current dev signups are throwaway and we wipe?
6. **Email**: do we let the provider send verification/invitation emails
   (most do), or pick a separate email vendor (Postmark/Resend/SES) and
   keep email orchestration in our backend?
7. **Cost ceiling**: what is the upper bound we are willing to pay per
   month before "buy" stops being worth it?
8. **Local dev**: can the chosen provider be exercised in tests without
   internet, or do we need a fake/test-mode harness in `internal/auth`?
9. **Rollback**: if we cut over and the provider becomes a problem, what
   is the recovery path? Keep the migration script reversible? Keep the
   `users.password_hash` column populated for N months?

## Recommended Provider Shortlist (for the drafts to evaluate)

The drafts should each independently evaluate at least these providers,
but are free to add others:

- **Clerk** — modern B2B SaaS auth, has Organizations + invitations + RBAC
  built in, React SDK, hosted email, strong dev experience.
- **WorkOS** — enterprise-flavored (SSO/SAML/SCIM/Directory Sync), Magic
  Link API, B2B-first, lighter UI primitives than Clerk.
- **Stytch** — B2B + B2C SDKs, Organizations primitive, password +
  passwordless, decent docs.
- **Supabase Auth** — open source, Postgres-friendly, but pulls in the
  Supabase ecosystem.
- **Auth0 (Okta)** — incumbent, generic, expensive at scale.
- **Ory Kratos** (self-hosted) — open source, no vendor lock-in, but adds
  ops burden.

The drafts should give a clear primary recommendation with rationale, plus
one credible alternate.

## Non-Goals

- Implementing SSO / SAML / SCIM in this sprint (provider readiness only).
- Onboarding multiple providers ("provider abstraction layer") — premature.
- Choosing an email vendor independently of the auth decision (handled as
  part of the build path or absorbed by the provider on the buy path).
- MFA / TOTP / WebAuthn UX (capture as follow-up regardless of decision).
- Replacing the frontend auth UI wholesale — keep the existing pages and
  swap the API client unless the provider mandates otherwise.
