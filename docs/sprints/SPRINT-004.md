# Sprint 004: Build & Configuration System

## Overview

Sprint 003 stood up a runnable local stack but its configuration is incidental: the backend reads four env vars directly via `os.Getenv` with inline defaults, tests have a parallel env contract, the frontend has no env story, and the only "build" is `scripts/dev.sh`. As we add features (fleet, catalog, trips, ledger, future API integrations), config knobs and secrets will accumulate quickly. This sprint introduces the smallest opinionated config and build system that fits before that pressure arrives.

The shape:

- **One typed `Config` struct in Go** (`internal/config`) is the only thing in the app that reads `os.Getenv`. It loads non-secret defaults from a committed mode file and overlays secrets from the process environment, then validates and returns. Every consumer (auth, store, http, testdb) takes values from it.
- **Three modes** — `dev`, `test`, `production` — selected by `LIVEABOARD_MODE` (or `--mode` for the server binary). `production` is never the default; it must be explicit.
- **Single source of truth for non-secret config:** `config/<mode>.env`. Frontend overrides are *generated* into `web/.env.local` from the active mode file at build time, so Vite's native env loading still works without a parallel hierarchy to maintain.
- **Secrets never on disk in committed files.** They live in process env. For dev/test, the loader auto-sources `.env.local` at repo root after the mode file but before real env. Production refuses to read any dotfile.
- **`Makefile` as the canonical entrypoint:** `make dev`, `make test`, `make build`, `make lint`, `make fmt`, `make clean`. `make build` produces the production artifact: a single `bin/liveaboard` binary with the Vite bundle embedded via `//go:embed`.
- **Sprint 003's ad-hoc env vars are migrated** with zero behavior change. After this sprint, `git grep "os.Getenv"` returns only `internal/config/loader.go`.

We are explicitly not deploying to cloud yet. The production *build* path exists; production *deployment* does not.

## Use Cases

1. **Local dev:** `make dev` selects `dev`, sources `config/dev.env`, overlays `.env.local` if present, fails fast if a required dev value is missing, then runs the backend on `:8080` and Vite on `:5173` with `/api` proxied to the backend.
2. **Test runs:** `make test` selects `test`, requires `LIVEABOARD_TEST_DATABASE_URL` (env-only), uses `BcryptCost=4` for a fast suite, never points at the dev database, runs `go test ./... -count=1`.
3. **Production build:** `make build` runs `npm ci && npm run build`, then `go build -o bin/liveaboard ./cmd/server`. The binary embeds `web/dist` and serves `/api` and the SPA on the same origin. Running it with `LIVEABOARD_MODE=production` requires every `required:secret` field present; missing → exit non-zero with a named error before binding the listener.
4. **Adding a new config key:** developer adds the field to `Config`, the relevant `config/<mode>.env`, `.env.example`, and the table in `docs/CONFIG.md`. A loader test confirms it loads and validates.
5. **Rotating a secret:** developer updates their shell or `.env.local`. No committed-file change.

## Architecture

### Mode selection

```
  --mode flag (cmd/server only)   ──┐
                                     ├──► resolveMode() ──► "dev" | "test" | "production"
  $LIVEABOARD_MODE                ──┤      (production must be explicit)
                                     │
  default                         ──┘ ──► "dev"
```

`production` is rejected if it is the default fallback. It must come from a flag or explicit env var.

### Config layering

```
  config/<mode>.env       (committed, non-secret defaults)
        │
        ▼
  .env.local              (gitignored, dev/test only — production skips this layer)
        │
        ▼
  process environment     (real env wins; this is where secrets live)
        │
        ▼
  Config                  (typed, validated, immutable after Load)
        │
        ▼
  consumers (auth, store, httpapi, org, testdb)
```

### Secret boundary

```
Committed:                         Never committed:
  config/dev.env                     LIVEABOARD_*_DATABASE_URL (secrets)
  config/test.env                    any future API keys / SMTP creds
  config/production.env              .env.local
  .env.example                       web/.env.local (generated)
  docs/CONFIG.md                     bin/
```

### Frontend integration

```
Dev:
  Browser ──► Vite (:5173) ──► proxy /api ──► Go (:8080)
  Frontend uses VITE_API_BASE (default "/api")

Production build:
  npm run build → web/dist
  go build → bin/liveaboard (embeds web/dist via //go:embed)
  Browser ──► bin/liveaboard ──► serves SPA + /api on same origin
```

The Makefile generates `web/.env.local` from the active backend mode file immediately before `npm run dev` / `npm run build`. So Vite's native env loading works, but the source of truth is still `config/<mode>.env`. `web/.env.local` is gitignored.

### Backend layout

```
internal/config/
  config.go        # Config struct, Load(), MustLoad(), LoadForTest(), String() (with redaction)
  modes.go         # Mode enum, resolveMode(), production guard
  loader.go        # tiny .env parser + struct-tag binder; only file in the repo allowed to call os.Getenv
  config_test.go   # loader unit tests
config/
  dev.env
  test.env
  production.env
.env.example       # repo-root, every key documented
```

### Production binary

`make build`:
1. `cd web && npm ci && npm run build` (output: `web/dist/`)
2. `go build -o bin/liveaboard ./cmd/server`

The Go binary always embeds `web/dist` via `//go:embed all:web/dist`. To make this build cleanly before the first `npm run build`, a `web/dist/.keep` placeholder is committed. Dev mode never serves the embedded handler — the browser hits Vite on `:5173`. Production mode serves embedded assets with SPA fallback (any non-`/api` route returns `index.html`).

## Implementation Plan

### Phase 1: Config package + tests (~30%)

**Files:**
- `internal/config/config.go` — `Config` struct, `Load(Mode) (*Config, error)`, `MustLoad(Mode) *Config`, `LoadForTest() *Config` (sets mode=test, sources test mode file, looks up `LIVEABOARD_TEST_DATABASE_URL`), `Config.String()` with secret redaction.
- `internal/config/modes.go` — `Mode` enum (`Dev`, `Test`, `Production`), `resolveMode()` with the explicit-production guard.
- `internal/config/loader.go` — minimal `.env` parser (KEY=VALUE, `#` comments, `export ` prefix tolerated, no shell expansion). Struct-tag binder reading `env`, `secret`, `required`, `default`. The only file allowed to call `os.Getenv`.
- `internal/config/config_test.go` — happy paths, missing required, type errors, default fallback, precedence (file < .env.local < real env), production safety guards, secret redaction.

**Config struct (initial fields):**

| Field | Env | Default (dev) | Default (test) | Default (prod) | Tags |
|---|---|---|---|---|---|
| `Mode` | `LIVEABOARD_MODE` | `dev` | `test` | `production` | required |
| `Addr` | `LIVEABOARD_ADDR` | `:8080` | `:0` | `:8080` | required |
| `DatabaseURL` | `LIVEABOARD_DATABASE_URL` | `postgres://localhost:5432/liveaboard?sslmode=disable` | `postgres://localhost:5432/liveaboard_test?sslmode=disable` | (none) | required, secret |
| `CookieSecure` | `LIVEABOARD_COOKIE_SECURE` | `false` | `false` | `true` | — |
| `BcryptCost` | `LIVEABOARD_BCRYPT_COST` | `12` | `4` | `12` | — |
| `SessionDuration` | `LIVEABOARD_SESSION_DURATION` | `336h` | `336h` | `336h` | — |
| `VerificationDuration` | `LIVEABOARD_VERIFICATION_DURATION` | `24h` | `24h` | `24h` | — |

Production-mode validation enforces:
- `CookieSecure=true`
- All fields tagged `required` and `secret` are present in the process environment (not from a file).

**Tasks:**
- [ ] Build the loader and binder.
- [ ] Implement `Load`, `MustLoad`, `LoadForTest`, `String` (redaction).
- [ ] Production-mode validation.
- [ ] Tests covering every behavior listed above.

### Phase 2: Migrate consumers (~15%)

**Files:**
- `cmd/server/main.go` — `cfg := config.MustLoad(config.ResolveMode(...))`; flag for `--mode` and `--addr`. All wiring receives `cfg`.
- `internal/auth/auth.go` — accept `BcryptCost`, `SessionDuration`, `VerificationDuration` via `Service` fields (already a struct). Drop the package consts.
- `internal/httpapi/httpapi.go` — `Server.Secure` already exists; just receive it from `cfg.CookieSecure`.
- `internal/testdb/testdb.go` — call `config.LoadForTest()` instead of `os.Getenv("LIVEABOARD_TEST_DATABASE_URL")`. Keep the skip behavior when the URL is empty.

**Tasks:**
- [ ] Refactor consumers; behavior unchanged.
- [ ] Delete the package consts in `auth` (`SessionDuration`, `VerificationDuration`, `BcryptCost`).
- [ ] Run the full suite to confirm no behavior change.

### Phase 3: Mode files + bootstrap scripts (~20%)

**Files:**
- `config/dev.env`, `config/test.env`, `config/production.env` — committed, non-secret only, each starts with a `# DO NOT PUT SECRETS HERE — env-only` banner.
- `.env.example` (repo root) — every key, sane example values, secret keys clearly marked with `# secret` comments.
- `scripts/lib/load-env.sh` — resolves `LIVEABOARD_MODE` (default dev), sources `config/<mode>.env`, sources `.env.local` if mode is dev or test, then `exec "$@"`.
- `scripts/dev.sh` — slimmed; calls `load-env.sh` with the dev runner.
- `scripts/test.sh` — new; calls `load-env.sh` with `go test ./... -count=1`.
- `scripts/build.sh` — new; calls `load-env.sh` with the build pipeline.

**Tasks:**
- [ ] Write the three `config/*.env` files matching the table above (non-secret values only).
- [ ] Write `.env.example` at repo root with every key documented.
- [ ] Implement `scripts/lib/load-env.sh`.
- [ ] Slim `scripts/dev.sh`; add `scripts/test.sh`, `scripts/build.sh`.

### Phase 4: Makefile + frontend env generation (~15%)

**Files:**
- `Makefile` — targets: `help` (default), `dev`, `test`, `build`, `lint`, `fmt`, `clean`.
- `web/src/lib/config.ts` — typed reader for `import.meta.env.VITE_*` with validation at module load (throws on missing required).
- `web/src/lib/api.ts` — prefix all calls with `appConfig.apiBase`.
- `web/vite.config.ts` — keep dev proxy. No mode-file changes; Vite reads `web/.env.local`.

**Tasks:**
- [ ] Write `Makefile` (see targets below).
- [ ] Add a small awk/sh stanza in `make dev` and `make build` that converts `VITE_*` keys from `config/<mode>.env` into `web/.env.local`. Document this in `Makefile` comments.
- [ ] Add `web/src/lib/config.ts` and migrate `api.ts`.
- [ ] Add `VITE_API_BASE` to each `config/<mode>.env` (default `/api`).

**Makefile targets:**

| Target | What it does |
|---|---|
| `help` | Prints target list with one-line descriptions. Default target. |
| `dev` | `LIVEABOARD_MODE=dev ./scripts/dev.sh` |
| `test` | `LIVEABOARD_MODE=test ./scripts/test.sh` |
| `build` | `LIVEABOARD_MODE=production ./scripts/build.sh` (npm ci, npm run build, go build) |
| `lint` | `gofmt -l .` (fail if non-empty), `go vet ./...`, `npm --prefix web run lint` if present |
| `fmt` | `gofmt -w .` |
| `clean` | `rm -rf bin web/dist web/.env.local` |

### Phase 5: Production asset serving (~10%)

**Files:**
- `internal/httpapi/assets.go` — `//go:embed all:web/dist` served as static files; non-`/api` routes fall back to `index.html` (SPA routing).
- `internal/httpapi/httpapi.go` — wire the asset handler under the chi router so `/api/*` is unaffected.
- `web/dist/.keep` — committed placeholder so `//go:embed` compiles before the first `npm run build`.
- `.gitignore` — already ignores `web/dist`; add an exception for `web/dist/.keep`.

**Tasks:**
- [ ] Add the embed directive and a tiny static handler.
- [ ] SPA fallback: any GET that doesn't match `/api/*` and doesn't resolve to a file under `web/dist` returns `index.html` with status 200.
- [ ] In dev mode (when running `./cmd/server` standalone), the embedded assets are still served (so `bin/liveaboard` works without Vite). Vite is the dev *experience* but not required for the binary to run.

### Phase 6: Docs + cleanup + verify (~10%)

**Files:**
- `docs/CONFIG.md` — keys table (name, mode-by-mode default, secret y/n, where it comes from, what it controls, what happens if missing). Includes the "adding a new config key" runbook (5 steps).
- `RUNNING.md` — updated to reference `make` targets.
- `.gitignore` — add `.env.local`, `web/.env.local`, `bin/`. Keep the `web/dist/.keep` exception.

**Tasks:**
- [ ] Write `docs/CONFIG.md`.
- [ ] Update `RUNNING.md`.
- [ ] Verify `git grep "os.Getenv" -- ':!internal/config/loader.go'` returns nothing.
- [ ] Verify `git grep -E '(password|secret|api[_-]?key|token)\s*=' -- 'config/*.env'` returns nothing.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `make build` all clean from a fresh checkout.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/config/config.go` | Create | Typed Config; Load / MustLoad / LoadForTest; String with redaction. |
| `internal/config/modes.go` | Create | Mode enum, resolveMode, production guard. |
| `internal/config/loader.go` | Create | .env parser + struct-tag binder. Only file allowed to call os.Getenv. |
| `internal/config/config_test.go` | Create | Loader unit tests. |
| `config/dev.env` | Create | Non-secret dev defaults. Banner: do not put secrets here. |
| `config/test.env` | Create | Non-secret test defaults. |
| `config/production.env` | Create | Non-secret prod defaults. |
| `.env.example` | Create | Every key documented; secret keys flagged. |
| `cmd/server/main.go` | Modify | config.MustLoad; --mode / --addr flags; pass cfg into wiring. |
| `internal/auth/auth.go` | Modify | Accept knobs from Service fields; drop package consts. |
| `internal/httpapi/httpapi.go` | Modify | Receive Secure from cfg; mount static handler. |
| `internal/httpapi/assets.go` | Create | //go:embed web/dist; static + SPA fallback. |
| `internal/testdb/testdb.go` | Modify | Use config.LoadForTest. |
| `web/dist/.keep` | Create | Placeholder so go:embed compiles pre-build. |
| `web/src/lib/config.ts` | Create | Typed VITE_ access; validates at module load. |
| `web/src/lib/api.ts` | Modify | Use appConfig.apiBase. |
| `web/vite.config.ts` | Modify | Document that Vite reads web/.env.local generated by Makefile. |
| `Makefile` | Create | help / dev / test / build / lint / fmt / clean. |
| `scripts/lib/load-env.sh` | Create | Shared mode + dotfile bootstrap. |
| `scripts/dev.sh` | Modify | Slim; delegates to load-env.sh. |
| `scripts/test.sh` | Create | Test runner via load-env.sh. |
| `scripts/build.sh` | Create | Production build via load-env.sh. |
| `docs/CONFIG.md` | Create | Keys reference + add-a-key runbook. |
| `RUNNING.md` | Modify | Point at make targets. |
| `.gitignore` | Modify | Add .env.local, web/.env.local, bin/. Keep web/dist/.keep exception. |
| `docs/sprints/SPRINT-004.md` | Create | This document. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 004. |

## Definition of Done

- [ ] `internal/config` exists with full test coverage of load + validate + redact + precedence + production safety guards.
- [ ] `git grep "os.Getenv" -- ':!internal/config/loader.go'` returns nothing.
- [ ] `git grep -E '(password|secret|api[_-]?key|token)\s*=' -- 'config/*.env'` returns nothing.
- [ ] All three modes load their committed defaults; missing required secrets in production fail fast with a named error before the listener binds.
- [ ] Production mode refuses to start with `CookieSecure=false` or with any required secret sourced from a file rather than the process env.
- [ ] `make help`, `make dev`, `make test`, `make build`, `make lint`, `make fmt`, `make clean` all work from a clean checkout.
- [ ] `make build` produces `bin/liveaboard` that, when run with the production env, serves both the API and the SPA on a single port.
- [ ] `web/src/lib/config.ts` validates required `VITE_*` at module load.
- [ ] `docs/CONFIG.md` lists every config key with mode-by-mode defaults and secret/non-secret status.
- [ ] `.env.local`, `web/.env.local`, `bin/` are gitignored; `web/dist/.keep` is committed.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm --prefix web run build` all clean.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Reinventing a config library badly | Medium | Medium | Keep the loader tiny (~150 LoC); cover with tests; explicitly out of scope: Consul/etcd/secret-managers. |
| Production mode accidentally selected by default | Low | High | `production` must be explicit; loader rejects `production` without `CookieSecure=true` and without all required secrets present in process env. |
| Secret accidentally committed to `config/*.env` | Medium | High | `make lint` greps mode files for secret-shaped lines; banner in each file warns; `.env.example` lives at root and is the canonical place for secret keys (with placeholder values). |
| Open Sprint 003 PR breaks on rebase | Medium | Low | Preserve env var names so a manual rebase only touches `cmd/server/main.go`. |
| `web/dist/.keep` placeholder confuses developers | Low | Low | Comment in `.gitignore`; `.keep` contains a one-line README. |
| `make` not available on some Windows dev box | Low | Low | Out of scope; document Linux/macOS only; capture as follow-up. |
| Generated `web/.env.local` drifts from `config/<mode>.env` | Low | Medium | Always regenerated in `make dev` and `make build`; never edited by hand; documented in CONFIG.md. |

## Security Considerations

- Secrets never live on disk in committed files. The `config/*.env` files are explicitly non-secret (banner) and `make lint` greps for secret-shaped key=value pairs.
- `Config.String()` and structured-log helpers redact any field tagged `secret:"true"`. Tests assert that a `Config` containing a fake DB URL with a password component does not appear in `String()` output.
- Production-mode validation enforces `CookieSecure=true`, presence of every `required:secret` field, and refuses to read any dotfile.
- `//go:embed all:web/dist` inlines only the SPA bundle; never any of `config/*.env` or `.env.local`.
- `.gitignore` blocks `.env.local`, `web/.env.local`, and `bin/`. The PR includes verification that none of these are tracked.

## Dependencies

- Sprint 003 (auth + org foundation, PR open) — this sprint refactors its `os.Getenv` usage. The two should land in this order, or this sprint rebases on top after 003 lands.
- No new Go module dependencies (stdlib only for the loader).
- No new npm dependencies.
- Sprint 002 (org admin user-story backlog, PR open) — independent; either order.

## Out of Scope

- Cloud deployment automation, container builds, infrastructure-as-code.
- Secret manager integration (Vault, AWS SM, GCP SM).
- Per-tenant runtime config.
- Hot config reloading.
- Configurable log level / structured-log formatter (defer until a concrete need).
- A `staging` mode (revisit when there is a real staging environment).
- Windows-native build path.
