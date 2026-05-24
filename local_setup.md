# Local Setup

Bring up the app end-to-end on your machine, with Postgres running
locally and email going to disk instead of Brevo. This is the path
for day-to-day dev, demos, and click-through testing — no SMTP
credentials required.

## 1. Postgres (one-time)

Install and start Postgres if you haven't already (macOS, Homebrew):

```bash
brew install postgresql@16
brew services start postgresql@16
```

Create the two databases the app expects. The test DB is separate so
`make test` is isolated from your dev data:

```bash
createdb liveaboard
createdb liveaboard_test
```

The app's migrations live in `internal/store/migrations/` and are
applied automatically by `store.Migrate()` at server startup — you
don't need to run them manually.

## 2. Local secrets file

Create `.env.local` at the repo root (gitignored, dev/test only):

```bash
cat > .env.local <<'EOF'
LIVEABOARD_EMAIL_TRANSPORT=filesystem
LIVEABOARD_EMAIL_FILESYSTEM_DIR=/tmp/inbox
EOF
```

That's it. No SMTP credentials needed — the filesystem transport
doesn't talk to Brevo at all.

> **Do not put `LIVEABOARD_DATABASE_URL` in `.env.local`** unless it
> already points at a `*_test` DB. `scripts/lib/load-env.sh` is shared
> by `make dev` and `make test`, and any value in `.env.local`
> overrides the mode file in both directions — a dev DSN here will
> cause `make test` to `TRUNCATE` your dev database. The default DSN
> in `config/dev.env` already targets the local `liveaboard` DB.

## 3. Run the app

```bash
make dev
```

That runs `scripts/dev.sh`, which boots:

- backend on `http://localhost:8080` (migrations + the `/dev/inbox`
  viewer mounted)
- Vite dev server for the SPA on `http://localhost:5173`

You should see this in the logs:

```
WARN email transport: filesystem (no SMTP delivery) inbox_dir=/tmp/inbox
INFO dev inbox viewer mounted path=/dev/inbox inbox_dir=/tmp/inbox
INFO listening addr=:8080
```

## 4. Exercise it

1. Open **http://localhost:5173** in a browser. Sign up with any
   synthetic email like `e2e+owner@example.invalid` — no real
   address needed.
2. Open **http://localhost:8080/dev/inbox** in another tab. The
   recipient is listed with the latest subject ("Confirm your
   email").
3. Click into the recipient → click the message → grab the
   verification link → paste into the SPA tab (or just `curl` it).

Alternative CLI access while you're in the terminal:

```bash
make inbox                                    # list recipients + subjects
scripts/inbox.sh e2e+owner@example.invalid    # pretty-print latest.json
make inbox-clear                              # wipe the inbox dir
```

## 5. Reset between runs (optional)

- Wipe just emails: `make inbox-clear`
- Wipe the dev DB: `make dev-reset` (existing helper — wipes
  users/orgs/sessions)
- Nuke and re-create the DB from scratch:
  `dropdb liveaboard && createdb liveaboard` (migrations re-run on
  next server start)

## Switching back to real email

Drop `LIVEABOARD_EMAIL_TRANSPORT=filesystem` from `.env.local` (or
set it to `smtp`) and supply Brevo credentials in the same file:

```bash
LIVEABOARD_SMTP_USERNAME=...
LIVEABOARD_SMTP_PASSWORD=...
LIVEABOARD_SMTP_FROM=Liveaboard <noreply@yourdomain.test>
```

See `docs/CONFIG.md` for the full key reference and
`docs/dev/email-inbox.md` for the filesystem transport details
(on-disk layout, JSON sidecar schema, security caveats).
