# Sprint 004 Merge Notes

## Inputs Compared

- `SPRINT-004-INTENT.md`
- `SPRINT-004-CLAUDE-DRAFT.md` — typed backend Config, .env mode files, `Makefile`, `embed_assets` build tag, separate Vite `.env*` hierarchy, `make build` + `make build-prod` split.
- `SPRINT-004-CODEX-DRAFT.md` — typed backend Config (JSON mode files), `Makefile`, single `make build` as prod artifact path, simple production static-serve, `scripts/lib/load-env.sh`, no auto-mode detection.
- `SPRINT-004-CLAUDE-DRAFT-CODEX-CRITIQUE.md`

## Claude Draft Strengths

- Struct-tag–driven loader (`env`, `secret`, `required`, `default`) gives a small, opinionated loader that stays under ~150 LoC.
- Secret-redacting `Config.String()` plus loader warning when a `secret:"true"` field comes from a file (defense in depth).
- Production-mode validation rules (`CookieSecure=true`, etc.).
- Risks/Mitigations identifies the "dev pastes prod creds into committed file" failure mode and proposes the banner mitigation.
- Frontend story explicit: `VITE_API_BASE` defaults to `/api` so same-origin convention works in both modes.

## Codex Draft Strengths

- Same-origin `/api` convention is named explicitly as the design goal (rather than emerging from defaults).
- `scripts/lib/load-env.sh` factors the mode-bootstrap shell logic into a single shared file used by `dev.sh`, `test.sh`, `build.sh`.
- Cleaner production asset serving: just serve `web/dist` from Go in production mode, no build-tag indirection.
- Disciplined scope — sticks to migrating the existing concrete config surface, defers speculative knobs.
- Holds `make build` as the canonical production artifact target (matches the intent's stated developer contract).

## Critiques Accepted

1. **Drop `make build` / `make build-prod` split.** `make build` is the canonical production artifact target. A faster backend-only iteration is just `go build ./cmd/server` and doesn't need a Makefile target.
2. **Drop `embed_assets` build tag.** Use `//go:embed all:web/dist` unconditionally. Commit a `web/dist/.keep` placeholder (gitignored otherwise) so the embed compiles even before `npm run build` has run. Production mode serves embedded assets; dev mode goes through Vite on `:5173` and never touches the embedded handler.
3. **Drop test-mode auto-detection.** `make test` sets `LIVEABOARD_MODE=test` explicitly; `internal/testdb` calls `config.LoadForTest()` which sets the mode itself. No `flag.Lookup("test.v")` magic, no build-tag inference.
4. **Single repo-level mode model — no parallel Vite `.env*` hierarchy.** The frontend's only real config value is `VITE_API_BASE` and a future `VITE_APP_NAME`. The Makefile will *generate* `web/.env.local` from the backend's `config/<mode>.env` immediately before `npm run dev` / `npm run build`. One source of truth; we leverage Vite's native loading without maintaining parallel mode files. The generated `web/.env.local` is gitignored.
5. **Defer speculative controls (`LogLevel`, etc.).** Migrate the existing concrete surface only:
   - `LIVEABOARD_DATABASE_URL`
   - `LIVEABOARD_TEST_DATABASE_URL`
   - `LIVEABOARD_ADDR`
   - `LIVEABOARD_COOKIE_SECURE`
   - auth timing knobs (`SessionDuration`, `VerificationDuration`, `BcryptCost`) — added because we want test mode to use bcrypt cost 4 for speed.
   `LogLevel` may sneak in if needed during implementation but is not a hard deliverable.
6. **Tighten DoD: `os.Getenv` appears only in `internal/config/loader.go`.** No "while it bridges" exception. `internal/testdb` migrates to `config.LoadForTest()`.
7. **`.env.example` at repo root**, not under `config/`. Matches the intent's wording and developer expectation.
8. **Adopt Codex's `scripts/lib/load-env.sh`.** Shared bootstrap that resolves `LIVEABOARD_MODE`, sources `config/<mode>.env`, sources `.env.local` if dev/test, and exec's the target command. `dev.sh`, `test.sh`, and `build.sh` all delegate to it.

## Critiques Rejected (with reasoning)

1. **Two config systems — mostly accepted, but the idiom isn't worth fighting.** Vite has a strong native env-loading convention; users debugging build issues will look there first. Compromise: we *do* have one authoritative source (the backend's `config/<mode>.env`), but we propagate frontend values through Vite's own mechanism by generating `web/.env.local` at build time. So there is one source of truth and one mechanism for frontend overrides — not two parallel hierarchies. (Accept the spirit, reject the literal "no Vite env files at all".)
2. **JSON mode files (Codex) over `.env` mode files (Claude) — keep `.env`.** `.env` is friendlier for the developer who maintains `.env.local` (same syntax), aligns trivially with Vite's idiom, and the parser is ~30 LoC of stdlib. JSON would require an extra binding step plus a parallel `.env`-style local file syntax for secrets anyway. Tie goes to `.env`.

## Interview Refinements Applied

- **Modes:** `dev` / `test` / `production` (explicit selection always; `production` never inferred).
- **Build tool:** `Makefile`.
- **Production frontend:** embed in Go binary via `go:embed`.
- **Local secrets:** loader auto-sources `.env.local` for dev/test only; production refuses to read any dotfile.

## Final Decisions

- Three modes: `dev`, `test`, `production`. `production` must be explicit — never the default.
- Backend has one typed `Config` in `internal/config`. `MustLoad(mode)` is called once in `cmd/server/main.go`; everything else takes values from `Config`. `os.Getenv` outside `internal/config/loader.go` is forbidden.
- Mode file format is `.env` at `config/<mode>.env`, committed, non-secret only, with a leading "DO NOT PUT SECRETS HERE" banner.
- Secrets live in process env. For dev/test, `.env.local` at repo root is auto-sourced by the loader after the mode file but before real process env. Production refuses to read any dotfile.
- Auth knobs (`SessionDuration`, `VerificationDuration`, `BcryptCost`) become `Config` fields. Test mode sets `BcryptCost=4` for speed.
- `Makefile` exposes: `dev`, `test`, `build`, `lint`, `fmt`, `clean`, `help`. `make build` is the canonical production artifact.
- `make build` runs `npm ci && npm run build` first, then `go build -o bin/liveaboard ./cmd/server`. The Go binary unconditionally embeds `web/dist` via `//go:embed`. A `web/dist/.keep` placeholder is committed so embedding works pre-build.
- `scripts/lib/load-env.sh` resolves mode and sources files; `scripts/dev.sh`, `scripts/test.sh`, `scripts/build.sh` delegate to it.
- Frontend has one config knob this sprint: `VITE_API_BASE` (default `/api`). The Makefile writes `web/.env.local` from the active mode file before `npm run dev` / `npm run build`. `web/.env.local` is gitignored.
- `.env.example` lives at repo root; `docs/CONFIG.md` documents every key (default per mode, secret y/n, where it comes from, what it controls, what happens if missing).
- Definition of Done: `git grep "os.Getenv" -- ':!internal/config/loader.go'` returns nothing.
