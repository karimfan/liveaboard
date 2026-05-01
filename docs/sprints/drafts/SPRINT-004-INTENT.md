# Sprint 004 Intent: Build & Configuration System

## Seed

We need to work on a build and configuration system for this application. As a developer I should be able to build it in either production or test mode. Each mode will have its own config settings, e.g. API keys, backend databases and so on. Secret material should not be stored in plain text in the config file, but must be in env variables. The build script will create the necessary env variables based on the mode: production or test.

## Context

- **Project state:** Greenfield Go + React app. Sprint 002 (org admin user-story backlog, docs) and Sprint 003 (auth + dashboard implementation, project skeleton) are in open PRs. Local dev only — Postgres on the developer's machine. No cloud yet.
- **Existing config surface (small, ad-hoc):** `cmd/server/main.go` reads `LIVEABOARD_DATABASE_URL`, `LIVEABOARD_ADDR`, `LIVEABOARD_COOKIE_SECURE` directly via `os.Getenv` with hard-coded defaults. Tests read `LIVEABOARD_TEST_DATABASE_URL`. There is no central config struct, no validation, no separation of secret vs non-secret values.
- **Existing build/run surface:** Single `scripts/dev.sh` exports one env var and runs `go run ./cmd/server` + `npm run dev`. No production binary build, no production frontend bundle deploy path, no `.env` files, no Vite env vars.
- **Tech stack:** Go 1.25 stdlib + minimal deps; Vite + React + TypeScript frontend; PostgreSQL.
- **Personas for this work:** developer running locally (primary), test runner (CI later), and a future deployer of a production build (we should not paint ourselves into a corner but we are explicitly local-only for now).

## Recent Sprint Context

- **Sprint 001 (planned):** Rewrite sprint tracking tooling from Python to Go. Tooling/infra only.
- **Sprint 002 (planned, PR open):** Documentation sprint — Org Admin user story backlog at `docs/product/organization-admin-user-stories.md`, persona boundaries at `docs/product/personas.md`. No application code.
- **Sprint 003 (planned, PR open):** First implementation sprint. Project skeleton (Go module, Postgres schema, Vite/React) plus auth (signup/verify/login/logout) and `GET /api/organization`. Configuration is ad-hoc `os.Getenv` calls.

## Relevant Codebase Areas

- `cmd/server/main.go` — env reads with inline defaults.
- `internal/testdb/testdb.go` — separate test env var.
- `scripts/dev.sh` — only existing build/run script.
- `web/vite.config.ts` — dev proxy targets `http://localhost:8080`; no env-driven backend URL yet.
- `internal/store/store.go` — DSN consumer.
- `internal/auth/auth.go` — `BcryptCost`, `SessionDuration`, `VerificationDuration` are package constants today; some of these may eventually need to be config (e.g., reduced bcrypt cost in tests).
- No existing `.env`, no `.env.example`, no config package.

## Constraints

- Must follow CLAUDE.md: Go stdlib + minimal deps; tests required; `go vet` and gofmt clean; prettier clean for the frontend; multi-tenant data isolation preserved.
- Local development only for now. The system must work for `dev` and `test` today, and have a clearly-defined `production` path that we can grow into without redesigning. We are not deploying to cloud in this sprint.
- Secrets must never appear in committed config files. Secrets live in environment variables only. Non-secret config can live in version-controlled files.
- The boundary between "secret" and "non-secret" must be explicit and documented so future engineers can reason about what to commit.
- No new heavyweight dep just to load config (e.g., Viper). A typed loader using stdlib + small helpers is preferred.
- Frontend needs a parallel story: how does the React app know which backend to talk to in dev vs prod? Today it uses Vite's dev proxy; production builds need a real URL.
- Backwards compatibility: existing `LIVEABOARD_DATABASE_URL` style env vars work and should continue to work, ideally without breaking the open Sprint 003 PR.

## Success Criteria

- A developer can run `make dev`, `make test`, and `make build` (or equivalent) and get a correctly-configured dev server, test suite run, or production binary respectively.
- A single typed `Config` struct in Go is the authoritative source of all backend runtime knobs. Every consumer reads from it; no `os.Getenv` calls scattered through the code.
- Each mode (`dev`, `test`, `production`) loads non-secret defaults from a checked-in file and overlays secrets from the process environment. Missing required secrets fail fast at startup with a clear error.
- Secrets are never written to disk by the build system unless the developer asks for `.env.local` for convenience, and that file is `.gitignored`.
- Frontend has a matching story: build-time config injection so the production bundle hits the right API URL.
- A `.env.example` and a short `docs/CONFIG.md` document every config key, its mode-by-mode default, whether it's secret, and where it comes from.
- Tests cover the loader: missing-required, wrong-type, default fallback, mode selection precedence.
- The Sprint 003 ad-hoc env vars are migrated to the new system without behavioral change.

## Open Questions

1. **Mode taxonomy:** is it `dev` / `test` / `production`, or do we want a fourth `staging` placeholder now? The seed says "production or test" — does dev fold into test, or is `dev` a third mode for `go run` workflows?
2. **Config file format:** TOML, YAML, JSON, or `.env`? `.env` is simple and frontend-friendly; TOML is more typed and Go-friendly. Pick one to keep tooling small.
3. **Selection precedence:** `LIVEABOARD_MODE` env var, CLI flag, or both? What happens if both are set?
4. **Build tool:** plain `Makefile`, a `scripts/build.sh`, a Go-based `cmd/build`, or `just`? CLAUDE.md leans Go-stdlib; a `Makefile` is the smallest universal answer but we have no current build tool footprint.
5. **Secret provisioning in dev:** do we expect developers to maintain a personal `.env.local`, or do we shell-export their secrets manually? The seed says the build script "creates the necessary env variables" — does that mean it sources a file, prompts the user, or pulls from a secret manager (out of scope for local-only)?
6. **Frontend in prod mode:** do we ship the Vite bundle from the Go binary (embed) or serve it separately? Embedding is one fewer deploy thing to manage; serving separately matches more conventional setups.
7. **Bcrypt cost / other "fast in test" knobs:** should we expose these as config and lower them in test mode to make the suite faster? Currently it's a package const.
