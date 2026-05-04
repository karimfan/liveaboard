# Running the app locally

## Prerequisites

- Go 1.25+
- Node 20+
- PostgreSQL 14+ (running locally on default port; `psql` on `$PATH`)
- GNU `make` (preinstalled on macOS and most Linux distros)

## First-time setup

```bash
createdb liveaboard            # dev database
createdb liveaboard_test       # test database
(cd web && npm install)
```

## Daily commands

```bash
make help     # list all targets
make dev      # backend on :8080 + Vite on :5173 (dev mode)
make test     # go test ./... in test mode
make build    # production binary at bin/liveaboard with embedded SPA
make lint     # gofmt + go vet + secret-scan of config/*.env
make fmt      # gofmt -w
make clean    # remove bin/, web/dist contents, web/.env.local
```

Open http://localhost:5173 after `make dev`.

## Sign-up flow (dev)

Authentication is provided by [Clerk](https://clerk.com). Before `make dev`
will work end-to-end you need a Clerk dev instance and four env vars in
`.env.local`. See [`docs/auth.md`](docs/auth.md) for the 8-step setup.

After signup, the SPA collects the organization name, posts it to
`POST /api/signup-complete`, and your local `lb_session` cookie is set.

## Configuration

`make dev`, `make test`, and `make build` each select a mode (`dev`/`test`/`production`) and load `config/<mode>.env`. For dev/test, they also source `.env.local` at the repo root if it exists.

To override a value locally, drop it into `.env.local` (gitignored):

```bash
# .env.local — gitignored, dev/test only
LIVEABOARD_DATABASE_URL=postgres://me@127.0.0.1:5432/my_local_db
```

Production never reads any dotfile; supply secrets via the process environment when running `bin/liveaboard`.

For the full key reference, see `docs/CONFIG.md`.

## Production binary

```bash
make build
# Produces bin/liveaboard with the Vite bundle embedded.

LIVEABOARD_MODE=production \
  LIVEABOARD_DATABASE_URL='postgres://...' \
  ./bin/liveaboard
# Serves SPA + /api on the same port.
```

The binary refuses to start if `LIVEABOARD_COOKIE_SECURE` isn't `true` or if any required secret was sourced from a file rather than the process env.

## Tests

```bash
make test
```

Tests skip cleanly if no Postgres is available. They use `config/test.env`'s `liveaboard_test` DSN by default; override via `.env.local` or process env.
