# Sprint 004: Build & Configuration System

## Overview

Sprint 003 established a runnable local stack, but the way it is configured is still incidental rather than designed. Backend settings are read directly from `os.Getenv` in `cmd/server/main.go`, tests have their own separate env contract, the frontend assumes a hard-coded local proxy, and the only developer workflow is `./scripts/dev.sh`. That is acceptable for initial scaffolding, but it is not a durable base for a multi-tenant SaaS application where correctness depends on using the right database, cookie settings, and runtime behavior in each mode.

This sprint defines a single configuration model for the backend, a matching build-time story for the frontend, and a small set of repeatable entrypoints for developers: `make dev`, `make test`, and `make build`. The system should be explicit about what is safe to commit versus what must stay in environment variables, and it should fail fast when required secrets are absent. The design should solve today's local workflows cleanly while preserving a credible path to a real production deployment later.

The recommended shape is: checked-in mode files for non-secret settings, environment variables for secrets and secret-adjacent overrides, a typed Go config package as the only backend config reader, and a thin shell/bootstrap layer that selects a mode and exports optional local secret files for convenience. For the frontend, dev keeps the existing same-origin `/api` proxy model, while production builds assume the frontend is served by the Go server so the browser continues to talk to `/api` on the same origin.

## Use Cases

1. **Developer starts the app in dev mode**: `make dev` loads the `dev` mode config, overlays local secrets from environment, starts the backend against the dev database, and runs Vite with the expected API base behavior.
2. **Developer runs the test suite safely**: `make test` loads `test` mode, requires the test database DSN, applies test-specific runtime knobs (for example lower bcrypt cost), and executes Go tests without pointing at the dev database by accident.
3. **Developer produces a production artifact**: `make build` creates a production frontend bundle and a backend binary that is ready to run in `production` mode with required secrets supplied by the environment.
4. **Runtime fails fast on invalid config**: startup errors clearly identify missing required secrets, bad boolean/number values, or unsupported modes before the server begins serving traffic.
5. **Engineer understands the contract**: `.env.example` and `docs/CONFIG.md` explain every setting, its source of truth, whether it is secret, and which modes care about it.

## Architecture

### Configuration model

```
                     LIVEABOARD_MODE=dev|test|production
                                   |
                                   v
                        config/<mode>.json (checked in)
                        non-secret defaults only
                                   |
                                   v
                     internal/config.Load(mode, env, fs)
                         |                     |
                         |                     +--> validation / parsing
                         v
                    typed config.Config
                         |
        +----------------+----------------+
        |                                 |
        v                                 v
   cmd/server/main.go                test helpers / build scripts
   http server wiring                mode-aware commands
```

The backend gets exactly one authoritative config object. All runtime consumers receive values from that object; no package outside `internal/config` should call `os.Getenv` for app settings.

### Secret boundary

```
Committed to git:
  config/dev.json
  config/test.json
  config/production.json
  .env.example
  docs/CONFIG.md

Never committed:
  LIVEABOARD_DATABASE_URL
  LIVEABOARD_TEST_DATABASE_URL
  any API keys / SMTP credentials / future secrets
  .env.local
  .env.dev.local
  .env.test.local
  .env.production.local
```

Mode files contain only non-secret defaults and behavior knobs: listen address, cookie secure flag, frontend serving mode, API base URL default, bcrypt cost, and similar values. Secrets stay in the process environment. Optional local `.env.*.local` files may be sourced by scripts for convenience, but they are gitignored and treated as developer-owned inputs, not canonical config.

### Frontend/runtime shape

```
Dev:
  Browser -> Vite (:5173) -> proxy /api -> Go API (:8080)
  Frontend uses relative "/api"

Production build:
  npm run build -> web/dist
  go build -> binary embeds / serves web/dist
  Browser -> Go server -> static assets + /api on same origin
```

Using the same-origin `/api` convention in both dev and production removes an unnecessary class of mode-specific frontend bugs. The sprint can still define `VITE_API_BASE_URL` with a default of `/api` so the repo retains flexibility for a later separate-frontend deployment.

## Implementation Plan

### Phase 1: Centralize backend config (~30%)

**Files:**
- `internal/config/config.go` - typed config model, mode enum, loader, validation.
- `internal/config/config_test.go` - loader unit tests.
- `cmd/server/main.go` - replace direct env reads with config loading.
- `internal/httpapi/httpapi.go` - consume cookie/security/static-serving settings from config.
- `internal/auth/auth.go` - replace hard-coded timing/cost constants with injected settings where appropriate.

**Tasks:**
- [ ] Add `internal/config` with a `Config` struct that covers current backend runtime knobs and leaves room for near-term additions.
- [ ] Define supported modes as `dev`, `test`, and `production`; reject unknown values.
- [ ] Implement mode selection precedence: CLI flag `--mode` overrides `LIVEABOARD_MODE`, which overrides default `dev`.
- [ ] Load non-secret defaults from `config/<mode>.json` using the Go stdlib JSON decoder.
- [ ] Overlay required secrets and explicit env overrides from process env.
- [ ] Validate required fields and parse types before returning `Config`.
- [ ] Refactor `cmd/server/main.go` to call the loader once and pass config into store/auth/http wiring.
- [ ] Replace auth package constants that should differ by mode, especially bcrypt cost, with config-driven values.

### Phase 2: Mode files and secret bootstrap (~20%)

**Files:**
- `config/dev.json` - checked-in dev defaults.
- `config/test.json` - checked-in test defaults.
- `config/production.json` - checked-in production defaults.
- `.env.example` - example secret inputs and mode selection.
- `.gitignore` - ignore local secret env files.
- `scripts/lib/load-env.sh` - shared mode/bootstrap helper.

**Tasks:**
- [ ] Create per-mode JSON config files containing only non-secret values.
- [ ] Document and enforce which keys are allowed in JSON versus env-only.
- [ ] Add optional sourcing of `.env.local` and `.env.<mode>.local` if present.
- [ ] Keep secrets out of generated artifacts and out of checked-in mode files.
- [ ] Ensure bootstrap scripts make it hard to accidentally run tests against the dev database.

### Phase 3: Developer entrypoints and build pipeline (~25%)

**Files:**
- `Makefile` - canonical `dev`, `test`, `build` targets.
- `scripts/dev.sh` - slim wrapper that delegates to shared config bootstrap.
- `scripts/test.sh` - mode-aware test runner.
- `scripts/build.sh` - production build orchestration.
- `RUNNING.md` - replace ad-hoc run instructions with the new commands.

**Tasks:**
- [ ] Add a `Makefile` with `dev`, `test`, and `build` as the supported user-facing commands.
- [ ] Make `make dev` load `dev` mode, run the backend, and start the Vite dev server.
- [ ] Make `make test` load `test` mode, require the test DSN, and run `go test ./...`.
- [ ] Make `make build` build the frontend first and then produce the Go binary in production mode.
- [ ] Preserve compatibility for existing direct env use so the open Sprint 003 workflow does not break mid-branch.

### Phase 4: Frontend production config and serving path (~15%)

**Files:**
- `web/vite.config.ts` - consume build-time API base setting with sane defaults.
- `web/src/lib/api.ts` - use configured API base instead of hard-coded relative fetch-only assumptions.
- `internal/httpapi/static.go` or `internal/httpapi/httpapi.go` - serve built frontend assets in production mode.

**Tasks:**
- [ ] Add `VITE_API_BASE_URL` support with a default of `/api`.
- [ ] Keep dev proxy behavior intact for local development.
- [ ] Serve the built frontend from the Go server in production mode so the compiled artifact is coherent.
- [ ] Ensure API calls and cookie behavior still work under same-origin serving.

### Phase 5: Documentation, verification, and migration cleanup (~10%)

**Files:**
- `docs/CONFIG.md` - authoritative configuration reference.
- `RUNNING.md` - local workflow documentation.
- `docs/sprints/SPRINT-004.md` - final sprint record when merged later.

**Tasks:**
- [ ] Document every config key, default, mode, and secret classification.
- [ ] Add tests for config loader precedence, missing required secrets, type errors, and per-mode defaults.
- [ ] Run `gofmt`, `go vet ./...`, `go test ./...`, and `npm run build`.
- [ ] Confirm the migrated system preserves current Sprint 003 behavior for dev login/signup flows.

## API Endpoints (if applicable)

No new public API endpoints are required for this sprint. The user-facing change is operational: configuration, build outputs, and static asset serving.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/config/config.go` | Create | Central typed configuration loader and validator. |
| `internal/config/config_test.go` | Create | Unit tests for mode selection, overlays, and validation failures. |
| `cmd/server/main.go` | Modify | Replace scattered env reads with config loading. |
| `internal/auth/auth.go` | Modify | Make runtime knobs like bcrypt cost configurable by mode. |
| `internal/httpapi/httpapi.go` | Modify | Consume config-driven cookie/static serving settings. |
| `config/dev.json` | Create | Checked-in non-secret defaults for development. |
| `config/test.json` | Create | Checked-in non-secret defaults for tests. |
| `config/production.json` | Create | Checked-in non-secret defaults for production builds. |
| `.env.example` | Create | Document required environment variables and local secret file pattern. |
| `.gitignore` | Modify | Ignore local env secret files and build artifacts as needed. |
| `Makefile` | Create | Canonical `dev`, `test`, and `build` workflows. |
| `scripts/lib/load-env.sh` | Create | Shared mode/bootstrap logic for scripts. |
| `scripts/dev.sh` | Modify | Reuse shared bootstrap instead of exporting one ad-hoc variable. |
| `scripts/test.sh` | Create | Safe test entrypoint with test-mode config. |
| `scripts/build.sh` | Create | Production bundle + binary build script. |
| `web/vite.config.ts` | Modify | Inject configurable API base and keep dev proxy. |
| `web/src/lib/api.ts` | Modify | Respect frontend API base setting. |
| `docs/CONFIG.md` | Create | Human-readable config contract. |
| `RUNNING.md` | Modify | Replace manual env guidance with mode-aware commands. |

## Definition of Done

- [ ] `make dev` starts the backend and frontend with `dev` mode config.
- [ ] `make test` runs `go test ./...` against test-mode settings and fails fast if the test DSN secret is missing.
- [ ] `make build` produces a production-ready frontend bundle and Go binary.
- [ ] No application package outside `internal/config` reads app config directly from `os.Getenv`.
- [ ] Required secrets are env-only and validated at startup.
- [ ] Checked-in config files contain no secret material.
- [ ] Frontend API base behavior works in both Vite dev and production build modes.
- [ ] `docs/CONFIG.md` and `.env.example` fully document the configuration surface.
- [ ] `gofmt -w`, `go vet ./...`, `go test ./...`, and `npm run build` all pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Config schema grows ad hoc again after this sprint | Medium | High | Establish `internal/config` as the only loader and reject direct env reads in review. |
| Test mode accidentally points at the dev database | Medium | High | Separate `test` mode defaults, require explicit test DSN, and document the contract clearly. |
| Production-serving work broadens the sprint too much | Medium | Medium | Limit scope to serving `web/dist` from Go without adding deployment infrastructure. |
| Optional `.env.local` sourcing causes confusion about precedence | Medium | Medium | Document a strict precedence order in code and docs, and test it. |
| Existing Sprint 003 branch assumptions break | Low | Medium | Keep current env variable names working as overlays and preserve `/api` frontend behavior. |

## Security Considerations

- Secrets must remain environment-only inputs and must never be committed in JSON config files.
- Startup should fail before serving traffic if required production secrets are missing or malformed.
- Test helpers and scripts must avoid cross-mode database reuse that could leak or destroy data.
- Production cookie settings must remain config-driven so HTTPS deployments can require `Secure=true`.
- Static asset serving must not expose arbitrary filesystem paths; serve only the built asset tree.

## Dependencies

- Sprint 003 codebase being merged or at least stable enough that the current config surface is accurate.
- Go 1.25 toolchain.
- Node 20+ for frontend builds.
- Local PostgreSQL for `dev` and `test`.

## References

- `docs/sprints/README.md`
- `CLAUDE.md`
- `RUNNING.md`
- `docs/sprints/drafts/SPRINT-004-INTENT.md`
