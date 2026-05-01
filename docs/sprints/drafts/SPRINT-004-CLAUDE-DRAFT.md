# Sprint 004: Build & Configuration System

## Overview

Today the backend reads four environment variables directly via `os.Getenv` with inline defaults; the frontend has no env story; the only "build" is `scripts/dev.sh`. As we add features (fleet, catalog, trips, ledger) we will accumulate more knobs (DB pool size, log level, future API keys, future SMTP creds, frontend API base URL), and we will start needing a real production binary. This sprint introduces a small, opinionated config and build system before that complexity arrives.

The shape is:

- **One typed `Config` struct in Go**, loaded from non-secret defaults baked into mode files plus secret overrides from the process environment. Loaded once at startup, validated, then passed to consumers. No more scattered `os.Getenv`.
- **Three modes:** `dev`, `test`, `production`. `LIVEABOARD_MODE` selects; defaults to `dev` when running locally.
- **`.env`-style mode files** at `config/dev.env`, `config/test.env`, `config/production.env`. Non-secret only. Committed.
- **`.env.local`** is per-developer, gitignored, sourced last. Where local secrets live during development.
- **A `Makefile`** as the build entrypoint: `make dev`, `make test`, `make build`, `make build-prod`. Small, universal, no new deps.
- **Frontend mirrors the same shape:** a typed `appConfig` reads from `import.meta.env.VITE_*` populated from the same mode `.env` files Vite already knows how to load.
- **Sprint 003 ad-hoc env vars are migrated** to the new loader with zero behavior change.

We deliberately keep the production story modest: there is a production *build* path (`make build-prod` produces a self-contained binary embedding the Vite bundle and the migrations) but no deployment automation. That fits "local-only for now" while not painting ourselves into a corner.

## Use Cases

1. **Local dev (default):** `make dev` starts backend + frontend, loads `config/dev.env`, overlays `.env.local` if present, fails fast if a required dev secret is missing.
2. **Test runs:** `make test` runs `go test ./...` with `LIVEABOARD_MODE=test`, loading `config/test.env` and pointing at `liveaboard_test`. Tests use a lower bcrypt cost and zero `Secure` cookie flag.
3. **Production build:** `make build-prod` builds the Vite bundle, embeds it into the Go binary alongside the migrations, and emits `bin/liveaboard`. The binary requires production secrets in env at runtime; it refuses to start if any are missing.
4. **Adding a new config key:** developer adds a field to `Config`, the mode files, `.env.example`, and `docs/CONFIG.md`. A test asserts the field is loaded and validated.
5. **Rotating a secret:** developer updates the value in their shell or in `.env.local`; no committed-file change required.
6. **CI** (future, not built this sprint): runs `make test` against `LIVEABOARD_MODE=test` with `LIVEABOARD_TEST_DATABASE_URL` provided as a CI secret.

## Architecture

### Mode taxonomy and selection

```
  process env $LIVEABOARD_MODE      ──┐
                                       ├──► resolveMode() ──► "dev" | "test" | "production"
  go test default                   ──┤
                                       │
  --mode flag (cmd/server)          ──┘   (production must be explicit; never inferred)
```

`production` must be explicitly selected; defaulting to it is forbidden. `dev` is the fallback when nothing is set. `test` is set automatically when running under `go test` (we detect via a build tag or by checking `flag.Lookup("test.v")`).

### Config layering

```
  config/<mode>.env       (committed; non-secret defaults)
        │
        ▼
  process environment     (secrets; CI/local shell; .env.local sourced for dev/test)
        │
        ▼
  Config struct           (typed; validated; immutable after Load)
        │
        ▼
  consumers               (Auth, Store, HTTP, Org)
```

Precedence (lowest → highest): mode file → `.env.local` (dev/test only) → real process env → CLI flag (where applicable). Real process env always wins over files. CLI flags only override a small whitelist (e.g., `--addr`).

### Backend layout

```
internal/config/
  config.go        # Config struct, Load(), Validate(), MustLoad()
  modes.go         # Mode enum, resolveMode(), production guard
  loader.go        # parseEnvFile(), applyEnv(), structtag-driven binding
  secrets.go       # tags + validation: required, secret, default
  config_test.go   # all the loader tests

config/
  dev.env          # committed defaults for dev
  test.env         # committed defaults for test
  production.env   # committed *non-secret* defaults for production
  .env.example     # documents every key
```

### Frontend layout

```
web/
  .env             # committed defaults (VITE_API_BASE=/api)
  .env.development # committed dev overrides
  .env.production  # committed prod build defaults
  .env.local       # gitignored, per-developer
  src/lib/config.ts # typed access to import.meta.env, validated at module load
```

Vite already understands `.env`, `.env.<mode>`, `.env.local`, `.env.<mode>.local` with the right precedence — we use that mechanism rather than reinventing it.

### Production binary

`make build-prod`:
1. `cd web && npm ci && npm run build` (output: `web/dist/`)
2. `go build -tags=embed_assets -o bin/liveaboard ./cmd/server`

A new `cmd/server` build tag `embed_assets` switches between two `assets.go` files:
- `assets_embed.go` (under `embed_assets` tag): `//go:embed all:web/dist` and serves the SPA.
- `assets_proxy.go` (default): no embed; relies on Vite dev server (404 the static routes).

The migrations are already embedded in `internal/store` so no extra work there.

## Implementation Plan

### Phase 1: Config package + tests (~30%)

**Files:**
- `internal/config/config.go` — `Config` struct, `Load()`, `MustLoad()`, `String()` (with secret redaction).
- `internal/config/modes.go` — `Mode` enum (`Dev`, `Test`, `Production`), `resolveMode()`, `IsTest()`.
- `internal/config/loader.go` — minimal `.env` parser (KEY=VALUE, `#` comments, no shell expansion), env binder driven by struct tags.
- `internal/config/config_test.go` — happy paths + validation failures.
- `config/dev.env`, `config/test.env`, `config/production.env`, `config/.env.example`.

**Tasks:**
- [ ] Define `Config` struct with all current keys: `Mode`, `Addr`, `DatabaseURL`, `CookieSecure`, `BcryptCost`, `SessionDuration`, `VerificationDuration`, `LogLevel`.
- [ ] Use struct tags: `env:"LIVEABOARD_DATABASE_URL"`, `secret:"true"`, `required:"true"`, `default:":8080"`.
- [ ] Implement small `.env` file parser (no quoting magic; `export` prefix tolerated; trims whitespace).
- [ ] Implement `Load(mode Mode)` that reads `config/<mode>.env`, then `.env.local` (only if mode is dev or test), then process env, returns validated `Config`.
- [ ] `Validate()` enforces: required-not-empty, durations parseable, addr looks valid, mode-specific rules (production requires `CookieSecure=true`, `LogLevel != debug`, etc.).
- [ ] `String()` redacts any field tagged `secret`.
- [ ] Tests: missing required → fail; default fallback; secret redaction; precedence (file < env); production safety guards.

### Phase 2: Migrate consumers off `os.Getenv` (~15%)

**Files:**
- `cmd/server/main.go` — call `config.MustLoad()`, pass `cfg` into all wiring.
- `internal/auth/auth.go` — accept `BcryptCost`, `SessionDuration`, `VerificationDuration` via `Service` fields rather than package consts.
- `internal/httpapi/httpapi.go` — `Secure` already on `Server`; keep.
- `internal/testdb/testdb.go` — read DSN from a small helper that respects `LIVEABOARD_MODE=test` and falls back to env (preserves existing skip behavior).

**Tasks:**
- [ ] Refactor consumers to take values from `cfg`.
- [ ] Keep current env var names so the open Sprint 003 PR still works after rebase.
- [ ] Run the full suite to confirm no behavior change.

### Phase 3: Frontend config (~15%)

**Files:**
- `web/.env`, `web/.env.development`, `web/.env.production`, `web/.env.example`.
- `web/src/lib/config.ts` — typed reader for `import.meta.env.VITE_*` with validation at module load.
- `web/vite.config.ts` — keep dev proxy, but make target read from `VITE_API_BASE_DEV` for advanced setups.

**Tasks:**
- [ ] Add `VITE_API_BASE` (defaults to `/api` in dev so the proxy works; absolute URL in prod build).
- [ ] Refactor `web/src/lib/api.ts` to prefix calls with `appConfig.apiBase`.
- [ ] Type-validate at module load; throw with a clear message if a required `VITE_*` is missing.
- [ ] Add `.env.local` to `.gitignore` for the web tree.

### Phase 4: Build entrypoints (~20%)

**Files:**
- `Makefile` — `dev`, `test`, `build`, `build-prod`, `migrate`, `lint`, `fmt`, `clean`, `help`.
- `scripts/dev.sh` — slimmed down; called by `make dev`.
- `cmd/server/assets_proxy.go` — default; no-op handler.
- `cmd/server/assets_embed.go` — `//go:embed all:web/dist`; SPA fallback handler under `//go:build embed_assets`.

**Tasks:**
- [ ] `make dev` → `LIVEABOARD_MODE=dev ./scripts/dev.sh`.
- [ ] `make test` → `LIVEABOARD_MODE=test go test ./... -count=1`.
- [ ] `make build` → `go build -o bin/liveaboard ./cmd/server` (no embed; for backend-only iteration).
- [ ] `make build-prod` → `cd web && npm ci && npm run build && cd .. && go build -tags=embed_assets -ldflags="-X main.version=$(git describe --always --dirty)" -o bin/liveaboard ./cmd/server`.
- [ ] `make help` lists targets; first target is `help`.

### Phase 5: Docs + .env.example + .gitignore (~10%)

**Files:**
- `docs/CONFIG.md` — keys table (name, mode-by-mode default, secret y/n, where it comes from, what it controls, what happens if missing).
- `RUNNING.md` — updated to reference `make` targets.
- `config/.env.example` — every key, sane example values, secrets clearly marked.
- `web/.env.example` — same idea for the frontend.
- `.gitignore` — add `.env.local`, `web/.env.local`, `bin/`.

**Tasks:**
- [ ] Write the keys table.
- [ ] Write a short "adding a new config key" runbook (4 steps).
- [ ] Update `RUNNING.md` to point at `make` targets.

### Phase 6: Cleanup + verify (~10%)

**Tasks:**
- [ ] `git grep "os.Getenv"` returns only the inside of `internal/config/loader.go`.
- [ ] `go vet`, `gofmt -l .`, `go test ./...`, `npm run build` all clean.
- [ ] `make build-prod` produces a working `bin/liveaboard` that serves the embedded SPA on `/` and the API on `/api`.
- [ ] Smoke run of `make dev`, `make test`, `make build-prod` documented in PR description.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/config/config.go` | Create | Typed Config, Load, MustLoad, redaction. |
| `internal/config/modes.go` | Create | Mode enum + selection rules. |
| `internal/config/loader.go` | Create | .env parser + struct-tag binder. |
| `internal/config/config_test.go` | Create | Loader unit tests. |
| `config/dev.env` | Create | Non-secret dev defaults (committed). |
| `config/test.env` | Create | Non-secret test defaults (committed). |
| `config/production.env` | Create | Non-secret prod defaults (committed). |
| `config/.env.example` | Create | Documents every key with example values. |
| `cmd/server/main.go` | Modify | Use `config.MustLoad()`; remove os.Getenv. |
| `cmd/server/assets_proxy.go` | Create | Default no-embed asset handler. |
| `cmd/server/assets_embed.go` | Create | `embed_assets`-tagged SPA handler. |
| `internal/auth/auth.go` | Modify | Accept BcryptCost/durations from Service fields. |
| `internal/testdb/testdb.go` | Modify | Read DSN via config helper. |
| `web/.env` + `.env.development` + `.env.production` + `.env.example` | Create | Frontend config. |
| `web/src/lib/config.ts` | Create | Typed VITE_ access. |
| `web/src/lib/api.ts` | Modify | Use `appConfig.apiBase`. |
| `Makefile` | Create | Build entrypoints. |
| `scripts/dev.sh` | Modify | Slimmed down; called by make. |
| `docs/CONFIG.md` | Create | Keys reference + runbook. |
| `RUNNING.md` | Modify | Point at make targets. |
| `.gitignore` | Modify | Add `.env.local`, `web/.env.local`, `bin/`. |

## Definition of Done

- [ ] `internal/config` exists with full test coverage of load + validate + redact + precedence.
- [ ] `git grep "os.Getenv"` returns only `internal/config/loader.go` (and `internal/testdb` while it bridges).
- [ ] All three modes (`dev`, `test`, `production`) load their committed defaults; missing required secrets fail fast with a named error.
- [ ] Production mode refuses to start with `CookieSecure=false` or with debug log level.
- [ ] `make help`, `make dev`, `make test`, `make build`, `make build-prod` all work from a clean checkout.
- [ ] `make build-prod` produces `bin/liveaboard` that serves both API and SPA on a single port.
- [ ] `web/src/lib/config.ts` validates required `VITE_*` at module load.
- [ ] `docs/CONFIG.md` lists every config key with mode-by-mode defaults and secret/non-secret status.
- [ ] `.env.local` is gitignored; no secret values committed to either `config/*.env` or `web/.env*`.
- [ ] `go vet`, `gofmt -l .`, `go test ./...`, `npm run build` clean.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Reinventing a config library badly | Medium | Medium | Keep the loader tiny (~150 LoC); cover with tests; explicitly out-of-scope: Consul/etcd/secret-managers. |
| Production mode accidentally selected in dev | Low | High | `production` must be explicit; loader rejects production without `CookieSecure=true` and without all required secrets present. |
| Secret accidentally committed in `config/*.env` | Medium | High | Pre-commit-style scan in `make lint` for substrings like `password=`, `secret=`, `key=` followed by non-empty values; document the "non-secret only" rule in `config/.env.example` header. |
| Open Sprint 003 PR breaks on rebase | Medium | Low | Preserve env var names; keep `Service` constructors backwards-compatible by keeping defaults equal to the prior consts. |
| Embedded SPA bloats binary | Low | Low | Keep behind `embed_assets` build tag so the default `make build` is fast. |
| Devs paste prod creds into a committed `.env` by habit | Medium | High | `config/*.env` files lead with a `# THIS FILE IS COMMITTED — DO NOT PUT SECRETS HERE` banner; secret keys live only in `.env.example` with placeholder values. |
| `make` not available on some Windows dev box | Low | Low | `scripts/build.sh` shadowing the most-used Make targets is offered as an alternative — out of scope this sprint, captured as follow-up. |

## Security Considerations

- Secrets never live on disk in committed files. The `config/*.env` files are explicitly non-secret and the loader logs a warning if a key tagged `secret:"true"` is sourced from a file rather than the process environment.
- `Config.String()` and structured-log helpers redact any field tagged `secret:"true"`. Tests assert that a `Config` containing a fake `LIVEABOARD_DB_PASSWORD` does not appear in its `String()` output.
- Production-mode validation enforces `CookieSecure=true`, `LogLevel != debug`, and presence of every `required:"true"` field tagged `secret:"true"`. Defaulting is forbidden for secrets in production.
- The `embed_assets` build tag inlines `web/dist/` only; never any of `config/*.env`.
- `.gitignore` blocks `.env.local`, `web/.env.local`, and `bin/`. The PR includes a check that none of these are tracked.

## Dependencies

- Sprint 003 (auth + org foundation, PR open) — this sprint refactors its `os.Getenv` usage. The two should land in this order, or this sprint rebases on top after 003 lands.
- No new Go module dependencies (stdlib only for the loader).
- No new npm dependencies.

## Open Questions

1. Should we expose `BcryptCost` as config so test mode can use cost 4 for a faster suite, or leave it as a package const in `auth`?
2. Is `Makefile` acceptable, or do you want a Go-based `cmd/build` so the build tool itself is in our primary language?
3. Should `make build-prod` embed the SPA or produce two artifacts? (Default proposed: embed.)
4. Is a `staging` mode worth scaffolding now (as `production`'s laxer twin), or strictly out of scope?
5. For dev, do we want the loader to auto-source `.env.local`, or is that magical — should the developer `direnv` / `source` it themselves?
