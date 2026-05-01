# Configuration Reference

This document is the source of truth for every config key the app reads.

## Where values come from

The backend loads config in this order. Higher layers win.

1. **Defaults** baked into the `Config` struct via `default:"..."` tags.
2. **`config/<mode>.env`** — committed, non-secret only. Banner at the top of each file warns against putting secrets there.
3. **`.env.local`** at the repo root — gitignored, dev/test only. Production refuses to read this layer.
4. **Process environment** — wins over everything. This is where production secrets live.

The loader is `internal/config`. It is the only package allowed to call `os.Getenv` or read configuration files. Every other consumer takes typed values from the loaded `Config`.

## Modes

| Mode | Selection | Notes |
|---|---|---|
| `dev` | default if nothing else specified | local iteration; loads `config/dev.env` and optionally `.env.local`. |
| `test` | `LIVEABOARD_MODE=test` (set by `make test`) | loads `config/test.env`; `BcryptCost=4` for fast suite. |
| `production` | `LIVEABOARD_MODE=production` or `--mode production` | must be explicit; never the default. Refuses to read `.env.local`. Refuses to start if a required secret was sourced from a file. Requires `LIVEABOARD_COOKIE_SECURE=true`. |

## Keys

| Key | Required | Secret | Type | Default | Dev | Test | Production | What it controls |
|---|---|---|---|---|---|---|---|---|
| `LIVEABOARD_MODE` | no | no | enum | `dev` | `dev` | `test` | (must be explicit) | Selects mode. |
| `LIVEABOARD_ADDR` | yes | no | host:port | `:8080` | `:8080` | `:0` | `:8080` | Backend listen address. |
| `LIVEABOARD_DATABASE_URL` | yes | yes | DSN | (none) | local liveaboard DB | local liveaboard_test DB | env-only | Postgres DSN. |
| `LIVEABOARD_COOKIE_SECURE` | no | no | bool | `false` | `false` | `false` | `true` (enforced) | Secure flag on session cookie. |
| `LIVEABOARD_BCRYPT_COST` | no | no | int [4,31] | `12` | `12` | `4` | `12` | Password hashing cost. |
| `LIVEABOARD_SESSION_DURATION` | no | no | duration | `336h` | `336h` | `336h` | `336h` | Session lifetime. |
| `LIVEABOARD_VERIFICATION_DURATION` | no | no | duration | `24h` | `24h` | `24h` | `24h` | Email verification token lifetime. |
| `VITE_API_BASE` | no | no | URL or path | `/api` | `/api` | `/api` | `/api` | Frontend → backend base URL. Same-origin `/api` works for both `make dev` (Vite proxy) and `make build` (binary serves SPA + /api). |

The `VITE_*` keys are part of the schema only so the loader catches typos. The Go runtime never reads them; the Makefile / `scripts/lib/sync-web-env.sh` writes them into `web/.env.local` before `npm run dev` / `npm run build` so Vite can consume them.

## Secret boundary

```
Committed:                         Never committed:
  config/dev.env                     LIVEABOARD_DATABASE_URL (and any future API keys)
  config/test.env                    .env.local
  config/production.env              web/.env.local (generated, gitignored)
  .env.example                       bin/
  docs/CONFIG.md
```

`make lint` greps `config/*.env` for secret-shaped key=value pairs and fails the build if any are found.

## Adding a new config key

1. Add the field to `Config` in `internal/config/config.go` with `env:"..."` and any of `required:"true"`, `secret:"true"`, `default:"..."`.
2. Add the key to whichever `config/*.env` files should provide a non-secret default. If it's secret, document it in `.env.example` only.
3. If the key needs to reach the frontend, name it `VITE_*` and add it to the relevant `config/*.env`. The Makefile will propagate it into `web/.env.local`. Then add a typed entry in `web/src/lib/config.ts`.
4. Update the table above in this file.
5. Add a test in `internal/config/config_test.go` covering load + validation.

## Local secret provisioning (dev/test)

For dev and test, drop overrides into `.env.local` at the repo root (gitignored). For example:

```bash
# .env.local — gitignored, only read in dev/test
LIVEABOARD_DATABASE_URL=postgres://me@127.0.0.1:5432/my_local_db
```

Production never reads `.env.local`. In production, supply secrets via the process environment (CI secret store, secret manager output piped into env, etc.).

## Loader behavior worth knowing

- `Config.String()` redacts any field tagged `secret:"true"`. Use it (or structured-log helpers) when logging config; never log the struct directly.
- Production-mode validation refuses to start when a `secret:"true"` field was sourced from a file rather than the process env, even if the value happens to be the same.
- `LIVEABOARD_BCRYPT_COST` outside `[4, 31]` is rejected at load time.
- Durations use Go syntax (`336h`, `15m`, `2h30m`). Bad strings are rejected at load time with the offending key in the error.
- A malformed line in `config/*.env` (no `=`) fails load with file path and line number.
