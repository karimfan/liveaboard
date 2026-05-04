# Sprint 005 Merge Notes

## Claude Draft Strengths

- Crisp build-vs-buy verdict with a scored matrix and named alternates.
- Clear data-ownership boundary: Clerk owns credentials/sessions/email; our DB stays canonical for `users` / `organizations` / `users.role`.
- Concrete schema migration and file list (good for downstream execution).
- Local-dev story: stub mode (`LIVEABOARD_AUTH_STUB=1`) for offline tests.
- Webhook event list with idempotency table.
- Risks/Definition-of-Done are exhaustive.

## Codex Draft Strengths

- **Provider adapter interface** (`internal/auth.Provider` with `workos.go` + `fakeprovider.go`). Cleaner abstraction than my "Clerk client wrapper"; keeps the option to swap providers behind a single seam.
- **Two-migration shape**: add provider linkage first, validate cutover, *then* drop dead local auth state in a second cleanup migration. Lower-risk than my one-shot drop.
- **`auth_sync_cursors`** for reconciliation (in WorkOS Events API model). Useful concept even with Clerk webhooks: a periodic reconciler closes drift.
- **Honest pricing posture**: doesn't hard-anchor the recommendation on a specific MAU number.
- **Session shape kept cookie-based** at the app boundary even with a hosted provider — preserves the existing frontend contract.

## Valid Critiques Accepted

1. **Org-bootstrap is underspecified in my draft.** Clerk signup doesn't ask for org name; my draft assumed a webhook would create both `users` + `organizations` rows, but the webhook can't know the org name. **Fix:** add an explicit signup orchestration step. Frontend renders Clerk `<SignUp>` to collect credentials, then on first authenticated callback the SPA posts to `POST /api/signup-complete` with the chosen `organization_name`. The backend creates the local `organizations` row, creates the Clerk Organization via the admin SDK, links the membership, and creates the local `users` row. All atomic in one transaction (with the Clerk side as best-effort + reconciliation backstop).

2. **Bearer-token regression breaks the SPA contract.** The intent doc explicitly required preserving "same-origin cookies + `/api/me` probe." **Fix:** keep `lb_session` cookie as the app's auth surface. After Clerk authentication completes in the SPA, an exchange endpoint (`POST /api/auth/exchange`) takes the Clerk JWT once, verifies it, opens a backend session, and sets the `lb_session` cookie. Subsequent requests use the cookie; backend resolves it through a (much smaller) sessions table that maps `lb_session` → Clerk user/session id. Logout (`POST /api/logout`) stays a backend endpoint that revokes both.

3. **Webhook-only sync is race-prone.** A user can authenticate and hit `/api/me` before the webhook lands a local `users` row. **Fix:** make user/org upsert *synchronous* on the exchange path (when verifying a Clerk JWT and the local row is missing, fetch from Clerk admin API and upsert). Webhooks are the secondary repair path for events that happen out-of-band (e.g., admin disables a user from the Clerk dashboard). Add a periodic reconciler (out of scope this sprint, captured as follow-up).

4. **Cost framing is shaky.** I anchored on "10K MAU free tier"; that figure shouldn't drive the decision and may not be current. **Fix:** reframe rationale as "Organizations primitive + invitation UX + React/Go SDKs + B2B fit." Cost is documented but not load-bearing.

5. **Schema teardown is too aggressive in one migration.** **Fix:** split into two migrations within the same sprint. `0002` adds provider linkage and makes old columns nullable. `0003` (end of sprint, after end-to-end validation) drops `password_hash`, `email_verified_at`, `sessions`, `email_verifications`. One-shot cutover within a single sprint per the interview, but with an internal validation checkpoint.

## Critiques Rejected (with reasoning)

- **Codex picks WorkOS as primary; my draft picks Clerk.** Final decision: **Clerk**, per the user's explicit interview answer. WorkOS is named as the credible alternate (already in my draft and the ADR). The provider-adapter interface (Codex's idea) preserves the option to switch.

## Interview Refinements Applied

| Question | Answer | Effect on plan |
|---|---|---|
| Provider | **Clerk** | Locked in; WorkOS named as alternate; adapter interface preserved. |
| Cutover | **One-shot wipe & cutover** | Single sprint, single PR, dev DB wipe — but two migrations internally (Codex critique 5). |
| Role source | **`users.role` authoritative** | Clerk org role is mirrored, not authoritative. App reads our DB. |
| Local dev | **Stub mode + Clerk dev instance** | Both: a shared Clerk dev instance for real flows + offline stub for `go test`. |

## Final Decisions

- Adopt **Clerk** as the auth provider. WorkOS named as alternate in the ADR.
- Build a small **`internal/auth.Provider` interface** (Codex pattern). Concrete impls: `clerk.go` and `stub.go` for tests.
- **Preserve same-origin cookie auth.** Backend mints `lb_session` cookie after a Clerk authentication via `POST /api/auth/exchange`. Existing `requireSession` middleware mostly stays; only its session-resolution internals change. `POST /api/logout` is preserved on the backend.
- **Synchronous upsert on first auth** (resolves the webhook race). Webhooks become the secondary repair path. A periodic reconciler is captured as a follow-up but not built this sprint.
- **Custom signup orchestration** for org bootstrap: Clerk `<SignUp>` collects credentials; SPA posts `organization_name` to `POST /api/signup-complete`; backend creates local org + Clerk org + membership atomically.
- `users.role` stays authoritative; Clerk org role is a mirror (interview).
- **Two migrations within the sprint:**
  - `0002_auth_provider_clerk_link.sql` — add `clerk_user_id`, `clerk_org_id`, make `password_hash` and `email_verified_at` nullable, add `webhook_events` (idempotency) and `auth_sync_cursors` (reconciliation cursor placeholder, even if reconciler is deferred).
  - `0003_auth_provider_cleanup.sql` — drop `password_hash`, `email_verified_at`, `sessions` (replaced by a smaller `app_sessions` table), `email_verifications`. Run after end-to-end validation in Phase 6.
- Keep a **smaller `app_sessions` table** for backend-issued cookies (cookie token sha256 → Clerk user id + Clerk session id + expiry). This is the cookie ↔ provider bridge.
- Pricing rationale **reframed**: "favorable for our scale but not the deciding factor."
- ADR `docs/decisions/0001-auth-provider.md` records the decision and the rejected alternates.
