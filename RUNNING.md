# Running the app locally

## Prerequisites

- Go 1.25+
- Node 20+
- PostgreSQL 14+ (running locally on default port; `psql` on `$PATH`)

## First-time setup

```bash
# 1. Create the dev database
createdb liveaboard

# 2. Install frontend deps
cd web && npm install && cd ..
```

## Run the dev stack

```bash
./scripts/dev.sh
```

This starts:
- Backend on http://localhost:8080 (auto-migrates the database on startup)
- Vite dev server on http://localhost:5173 (proxies `/api` to the backend)

Open http://localhost:5173 in a browser.

## Sign up flow (dev)

Email is not actually sent — the verification token is logged to the backend
output and also returned in the signup response. The signup page shows the
token and a one-click "Continue" link.

## Tests

```bash
# Create the test database (one-time)
createdb liveaboard_test

# Run all Go tests
LIVEABOARD_TEST_DATABASE_URL='postgres://localhost:5432/liveaboard_test?sslmode=disable' \
  go test ./...
```

Tests skip cleanly if `LIVEABOARD_TEST_DATABASE_URL` is unset.

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `LIVEABOARD_DATABASE_URL` | `postgres://localhost:5432/liveaboard?sslmode=disable` | Dev database DSN. |
| `LIVEABOARD_TEST_DATABASE_URL` | (unset; tests skip) | Test database DSN. |
| `LIVEABOARD_ADDR` | `:8080` | Backend listen address. |
| `LIVEABOARD_COOKIE_SECURE` | `false` | Set to `true` behind HTTPS. |
