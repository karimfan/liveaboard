# Sprint 003: Auth + Organization Foundation

## Overview

First implementation sprint. Greenfield codebase, so this sprint stands up the project skeleton (Go module, Postgres schema, Vite/React frontend) and ships the four foundational stories from the Org Admin backlog:

- US-1.1 — Sign up as an Organization Admin
- US-1.2 — Log in
- US-1.3 — Log out
- US-2.1 — View organization details

Story acceptance criteria are in `docs/product/organization-admin-user-stories.md`. Persona scope is in `docs/product/personas.md`. This doc only adds implementation-level decisions on top.

## Stack Decisions

| Area | Choice | Why |
|---|---|---|
| Frontend | Vite + React + TypeScript | Lighter than Next; no SSR need yet. |
| Routing | `react-router-dom` | Standard SPA routing. |
| Backend | Go 1.25, stdlib `net/http`, `chi` for routing | Stdlib-first per CLAUDE.md; chi is a tiny router with idiomatic middleware. |
| DB driver | `jackc/pgx/v5` | De facto Postgres driver for Go. |
| Migrations | `pressly/goose` | Plain SQL up/down files; CLI + library. |
| Password hashing | `golang.org/x/crypto/bcrypt` | Boring, standard, sufficient for MVP. |
| Sessions | Opaque random token in HTTP-only `Secure` (when TLS) cookie; sessions table | Easier to invalidate than JWTs. |
| Email "sending" | Stub logger for MVP | Real email out of scope. Verification token is logged for now. |

## Project Layout

```
/
├── go.mod, go.sum
├── cmd/server/main.go          # HTTP server entrypoint
├── internal/
│   ├── auth/                   # signup, login, logout, sessions, password hashing
│   ├── org/                    # organization read endpoints
│   ├── store/                  # pgx pool + repositories
│   └── httpapi/                # router, middleware (auth, request id, logging), JSON helpers
├── migrations/                 # goose SQL migrations
├── web/                        # Vite + React + TS app
│   ├── src/
│   │   ├── pages/{Signup,Login,Dashboard}.tsx
│   │   ├── lib/api.ts          # fetch wrapper
│   │   └── styles/             # design tokens from DESIGN.md
│   └── ...
└── scripts/dev.sh              # convenience runner
```

## Database Schema (initial)

```sql
organizations(id uuid pk, name text not null, currency text, created_at, updated_at)
users(
  id uuid pk,
  organization_id uuid not null references organizations(id),
  email citext unique not null,
  password_hash bytea not null,
  full_name text not null,
  role text not null check (role in ('org_admin','site_director','crew')),
  email_verified_at timestamptz null,
  is_active boolean not null default true,
  created_at, updated_at
)
sessions(
  id uuid pk,
  user_id uuid not null references users(id) on delete cascade,
  token_hash bytea not null unique,         -- store sha256(token), not the token itself
  created_at,
  last_seen_at,
  expires_at timestamptz not null
)
email_verifications(
  id uuid pk,
  user_id uuid not null references users(id) on delete cascade,
  token_hash bytea not null unique,
  expires_at timestamptz not null,
  consumed_at timestamptz null
)
```

Row-level security is in scope per CLAUDE.md but is deferred to a later sprint that introduces multi-org querying. For MVP, every query goes through repositories that take an explicit `organization_id` and the HTTP layer pulls that from the authenticated session — there are no implicit org-wide queries.

## API Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/signup` | Create org + first Org Admin. Issues verification token (logged, not emailed). |
| POST | `/api/verify-email` | Consume verification token. |
| POST | `/api/login` | Auth, set session cookie. |
| POST | `/api/logout` | Invalidate session. |
| GET  | `/api/me` | Return current user (id, email, name, org_id, role). |
| GET  | `/api/organization` | Return current user's organization (name, created_at, summary stats). |

Summary stats for US-2.1 return zeros for now — boats/trips/guests tables don't exist yet. The endpoint shape is the final shape; values fill in as later sprints add tables.

## Implementation Plan

### Phase 1: Project skeleton (~15%)

**Files:**
- `go.mod`, `cmd/server/main.go`, `internal/httpapi/router.go`, `migrations/0001_init.sql`, `web/` Vite scaffold, `scripts/dev.sh`, `.gitignore`.

**Tasks:**
- [ ] `go mod init github.com/karimfan/liveaboard`.
- [ ] Pull deps: chi, pgx, goose, bcrypt.
- [ ] Vite + TS + React app under `web/`.
- [ ] `.gitignore` for build artifacts, `.DS_Store`, `node_modules`, etc.

### Phase 2: DB schema + repositories (~15%)

**Files:** `migrations/0001_init.sql`, `internal/store/*.go`.

**Tasks:**
- [ ] Migration with `organizations`, `users`, `sessions`, `email_verifications`, citext extension.
- [ ] `pgxpool` setup; `LIVEABOARD_DATABASE_URL` env var.
- [ ] Repositories with explicit `organization_id` scoping.

### Phase 3: Auth backend (~30%)

**Files:** `internal/auth/*.go`, `internal/httpapi/auth_handlers.go`.

**Tasks:**
- [ ] Password hashing (bcrypt cost 12).
- [ ] Signup: validate inputs, create org, create user, issue verification token (log it).
- [ ] Verify-email: consume token, set `email_verified_at`.
- [ ] Login: bcrypt compare, generic "invalid credentials" error, create session, set cookie.
- [ ] Logout: invalidate session row.
- [ ] Session middleware: pull token from cookie, look up by sha256, reject if expired.
- [ ] Tests: integration tests against real Postgres (skip if `LIVEABOARD_TEST_DATABASE_URL` not set).

### Phase 4: Org dashboard endpoint (~10%)

**Files:** `internal/org/*.go`, `internal/httpapi/org_handlers.go`.

**Tasks:**
- [ ] `GET /api/organization` returning name, created_at, currency, summary stats.
- [ ] Test asserting response shape and org-scoping.

### Phase 5: Frontend (~25%)

**Files:** `web/src/**`.

**Tasks:**
- [ ] Pull design tokens from `DESIGN.md` into `styles/tokens.css`.
- [ ] Pages: Signup, Login, Dashboard.
- [ ] `lib/api.ts` fetch wrapper with credentials: 'include'.
- [ ] Router: `/signup`, `/login`, `/` (dashboard, requires session — bounce to `/login` if 401).
- [ ] Logout button on dashboard.

### Phase 6: Verification (~5%)

**Tasks:**
- [ ] `go vet ./...`, `go test ./...`, `gofmt -l .` clean.
- [ ] `npm run build` clean.
- [ ] Manual smoke: signup → verify (consume the logged token) → login → see dashboard → logout.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `go.mod`, `go.sum` | Create | Go module + deps. |
| `cmd/server/main.go` | Create | HTTP entrypoint. |
| `internal/auth/*.go` | Create | Auth domain. |
| `internal/org/*.go` | Create | Organization domain (read). |
| `internal/store/*.go` | Create | Postgres repositories. |
| `internal/httpapi/*.go` | Create | Router, middleware, handlers. |
| `migrations/0001_init.sql` | Create | Initial schema. |
| `web/` | Create | Vite/React/TS frontend. |
| `scripts/dev.sh` | Create | Run Postgres-aware backend + frontend dev together. |
| `.gitignore` | Create | Ignore noise. |
| `docs/sprints/SPRINT-003.md` | Create | This doc. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 003. |

## Definition of Done

- [ ] `go vet ./...`, `go test ./...`, `gofmt` all clean.
- [ ] `web/` builds (`npm run build`).
- [ ] Manual flow works: signup → consume verification token → login → dashboard shows org name → logout invalidates session.
- [ ] Login returns a generic error for invalid credentials (no email enumeration).
- [ ] Sessions expire and can be invalidated by logout.
- [ ] Passwords are bcrypt-hashed; no plaintext storage anywhere.
- [ ] Every DB query takes an explicit `organization_id` (no global selects).
- [ ] README updated with run instructions (or new `RUNNING.md`).

## Out of Scope

- Real email sending (verification token is logged).
- Password reset (US-1.4 — Sprint 008).
- Profile updates (US-1.5 — Sprint 008).
- Org settings update / currency selector UI (US-2.2, US-2.3 — Sprints 005/008).
- Postgres row-level security (deferred to a later sprint).
- Rate limiting (deferred; AC says "rate-limited" — captured as follow-up).

## Risks

| Risk | Mitigation |
|---|---|
| Spec creeps into Sprint 004+ stories | Hard-stop at four stories; everything else is a follow-up. |
| Postgres-required tests slow CI | Tests skip cleanly when `LIVEABOARD_TEST_DATABASE_URL` is unset. |
| Cookie/CORS pain in dev | Vite dev proxy forwards `/api` to backend on same origin. |
