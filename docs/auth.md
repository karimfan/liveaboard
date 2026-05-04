# Auth (Clerk) — operational guide

This is the runbook for the Clerk-backed auth stack landed in Sprint 005.
For the why, see [`docs/decisions/0001-auth-provider.md`](decisions/0001-auth-provider.md).

## Local dev setup

1. Sign up at https://clerk.com (free).
2. Create a new application — name it "Liveaboard (dev)". Pick **React** + **Go** when asked.
3. *Configure → Email, Phone, Username* — enable email + password (or social, your call).
4. *Configure → Organizations* — turn on. Add custom org-role slugs `org_admin` and `site_director` so they map directly to `users.role`.
5. *API Keys* — copy:
   - `CLERK_PUBLISHABLE_KEY` (`pk_test_...`)
   - `CLERK_SECRET_KEY` (`sk_test_...`)
6. *Configure → Webhooks → Add Endpoint*:
   - URL: a tunnel pointing at `http://localhost:8080/api/webhooks/clerk` (e.g. `cloudflared tunnel --url http://localhost:8080`).
   - Subscribe to: `user.*`, `organization.*`, `organizationMembership.*`.
   - Copy the resulting **Signing Secret** (`whsec_...`) — this is `CLERK_WEBHOOK_SECRET`.
7. Drop all four into `.env.local` (gitignored) at the repo root, plus the duplicate `VITE_CLERK_PUBLISHABLE_KEY` Vite needs:

   ```bash
   CLERK_PUBLISHABLE_KEY=pk_test_...
   CLERK_SECRET_KEY=sk_test_...
   CLERK_WEBHOOK_SECRET=whsec_...
   VITE_CLERK_PUBLISHABLE_KEY=pk_test_...
   ```

8. `make dev` and visit http://localhost:5173.

## Backlog mapping

| Story | Provided by |
|---|---|
| US-1.1 sign up as Org Admin | Clerk `<SignUp>` + our `POST /api/signup-complete` (creates Clerk org + local org + first admin) |
| US-1.2 log in | Clerk `<SignIn>` + `POST /api/auth/exchange` (mints `lb_session`) |
| US-1.3 log out | `POST /api/logout` (revokes Clerk session + clears `lb_session`) |
| US-1.4 password reset | Clerk's hosted reset flow (no app code) |
| US-1.5 profile updates | Clerk `<UserProfile>` component (no app code; `user.updated` webhook syncs name/email) |
| US-6.1 invite Site Director | `POST /api/invitations` → Clerk emails the invitee → `organizationMembership.created` webhook creates the local row |
| US-6.2 deactivate user | `POST /api/users/{id}/deactivate` → Clerk `RemoveMembership` + local `is_active=false` + revokes `app_sessions` |
| US-6.3 resend invitation | `POST /api/invitations/{id}/resend` → revokes the old Clerk invitation and creates a fresh one |

## Request paths

### Sign up (org bootstrap)

```
SPA -> Clerk <SignUp> -> Clerk JS issues session -> SignedIn ->
SPA collects organization_name -> POST /api/signup-complete (Bearer JWT) ->
backend verifies JWT, creates Clerk org, creates local org+admin atomically,
mints lb_session -> SPA navigates to /
```

### Log in (existing user)

```
SPA -> Clerk <SignIn> -> Clerk session -> SignedIn ->
SPA gets JWT, POST /api/auth/exchange -> backend verifies JWT, looks up
local user by clerk_user_id, mints lb_session -> SPA navigates to /
```

### Subsequent requests

```
SPA fetch with credentials: 'include' -> backend reads lb_session cookie,
verifies app_sessions row, attaches users row to context -> handler runs.
```

### Webhook (Clerk -> us)

```
Clerk -> POST /api/webhooks/clerk (svix-signed) -> backend verifies
signature, dedupes via webhook_events.id, dispatches to per-event handler.
```

## Key rotation

- **Publishable key (`pk_test_*` / `pk_live_*`)**: not a secret, but
  changing it re-issues the dev instance. Update `.env.local` and
  `.env.example` together; redeploy.
- **Secret key (`sk_test_*` / `sk_live_*`)**: dashboard *API Keys* →
  Roll. Update `.env.local` (or process env in production), restart the
  backend. The next JWT verification call uses the new key.
- **Webhook signing secret (`whsec_*`)**: dashboard *Webhooks → Endpoint
  → Reveal/Roll Signing Secret*. Update `.env.local`/process env, restart
  the backend. In-flight Clerk retries signed with the old secret will
  be rejected; you may want to log an event id list and re-deliver from
  the dashboard if you care about the gap.

## Adding a new webhook event handler

1. Subscribe to the event in the Clerk dashboard's webhook endpoint
   config.
2. Add a `case` for the event type in `internal/auth/webhook.go`'s
   `dispatch` method.
3. Implement `handleX` in `internal/auth/webhook_handlers.go`. Decode
   the relevant payload subset; do not error on extra fields; return
   `nil` on no-op events so Clerk does not retry.
4. Add a captured-payload test in `internal/auth/webhook_test.go`. Use
   `signedRequest(t, "evt_X", payload)` to build a signed request.
5. The `webhook_events` PRIMARY KEY (svix-id) handles idempotency
   automatically — replays return 200 without re-running the handler.

## Escape hatch (leaving Clerk)

See ADR `docs/decisions/0001-auth-provider.md` § "Escape Hatch".

## Stub / offline tests

Backend tests use `auth.NewStubProvider()`. The stub implements the full
Provider interface in memory and lets tests exercise every code path
without network. See `internal/auth/provider_test.go` and
`internal/auth/exchange_test.go`.

There is no live integration test by default. The single test that
contacts Clerk (`TestClerkProviderRejectsBogusJWT`) skips when
`CLERK_SECRET_KEY` is unset.

## Production deployment

Production refuses to start unless every Clerk secret is supplied via
the **process environment** (not a dotfile):

- `CLERK_SECRET_KEY` — backend SDK calls.
- `CLERK_WEBHOOK_SECRET` — webhook signature verification.

`CLERK_PUBLISHABLE_KEY` and `VITE_CLERK_PUBLISHABLE_KEY` are not
secrets but should still be passed through env for consistency.
`LIVEABOARD_COOKIE_SECURE=true` is also enforced.

The webhook endpoint (`/api/webhooks/clerk`) must be reachable from
Clerk's egress at the configured webhook URL. In production this is
just your public hostname; in dev, use a tunnel as described above.
