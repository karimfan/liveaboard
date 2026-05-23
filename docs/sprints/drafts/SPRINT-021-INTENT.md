# Sprint 021 Intent: Filesystem Email Transport for End-to-End Testing

## Seed

> I want you to implement a way to mock all emails flow to go thru the
> local filesystem. So instead of sending an email to a user write a
> file in /tmp/inbox/user ..etc. This way we can test the application
> with mock emails. The app should be configurable in this mode. The
> default is to send real emails. This would be for testing purposes.
> All links in the emails should be persisted in the files in the
> user's inbox. This way we can test the flow end to end without using
> real emaila addresses.

## Context

- The backend already has a clean `email.Sender` interface
  (`internal/email/email.go`) with two implementations: `SMTPSender`
  (Brevo, used in production) and `MockSender` (in-memory, tests only).
- All eight email kinds (`verification`, `invitation`, `password_reset`,
  `change_email`, `trip_assigned`, `trip_unassigned`,
  `guest_registration_invite`, `guest_folio_closed`) flow through one
  call path: `email.Render(kind, vars) → Message → Sender.Send(ctx, msg)`.
- Sender construction lives in `cmd/server/main.go`. Today it builds
  `SMTPSender` unconditionally and the process exits if any SMTP env var
  is empty.
- The `internal/config` loader is the only package allowed to read env
  vars; every other consumer takes typed values from the `Config`
  struct. Modes are `dev`, `test`, and `production`, each with its own
  committed `config/<mode>.env`.
- A precedent already exists for extracting links from rendered bodies:
  `MockSender.LinkFor(to, needle)` scans the most recent body for the
  first `http(s)://` URL on a line containing `needle`. The filesystem
  inbox should be parseable the same way (or by simply opening the file
  in a browser/terminal).

## Recent Sprint Context

- **Sprint 018 (Trip Lifecycle)**: Added explicit trip statuses and
  emergency override flows. Emails (assignment/unassignment) are part
  of those workflows.
- **Sprint 019 (Real-Time Consumption Ledger)**: Made guest folios the
  live operational ledger; folio close still emails the guest.
- **Sprint 020 (Pricing Overrides and Currency Defaults)**: Latest
  shipped sprint. Settlement totals on the folio-closed email now
  reflect override snapshots.

This sprint is intentionally smaller and infra-shaped — it does not
add user-visible features; it makes the application testable end-to-end
without an SMTP relay or real recipient addresses.

## Relevant Codebase Areas

- `internal/email/email.go` — `Sender` interface, `MockSender`, `Vars`.
- `internal/email/smtp.go` — `SMTPSender` (production transport).
- `internal/email/message.go` — MIME serialization.
- `internal/email/templates.go` — `Render` + embedded templates.
- `internal/config/config.go` — typed config + loader.
- `cmd/server/main.go` — currently hard-requires SMTP env vars at
  startup.
- `config/dev.env`, `config/test.env`, `config/production.env`,
  `docs/CONFIG.md` — config surface and documentation.
- Senders embedded in flows:
  - `internal/auth/auth.go` (verification)
  - `internal/auth/password.go` (password reset)
  - `internal/auth/invitations.go` (organization invitations)
  - `internal/auth/email_change.go` (change-email confirm)
  - `internal/auth/guest_accounts.go` (guest registration invite)
  - `internal/httpapi/cruise_director_assign.go` (trip assigned/unassigned)
  - `internal/httpapi/guest_folio_handlers.go` (folio closed)

## Constraints

- **Default is SMTP** — the existing production behavior must not
  change unless an operator explicitly opts in to filesystem transport.
- **Single switch point** — sender construction stays centralized in
  `cmd/server/main.go`; no caller learns about transports.
- **No new external dependencies** — backend remains stdlib + the
  existing minimal-deps set.
- **Config goes through `internal/config`** — no `os.Getenv` outside
  the loader; new env vars get `env`/`default`/`secret` tags and an
  entry in `docs/CONFIG.md`.
- **Existing test contract preserved** — Go unit tests continue to use
  `MockSender`; the filesystem transport is for *running the app*
  end-to-end (the dev server, smoke tests, possibly Playwright/CI),
  not as a replacement for the unit-test mock.
- **Production safety** — production mode must either refuse to start
  with filesystem transport, or require an explicit opt-in flag with a
  loud warning. Default must keep SMTP.
- **Token-bearing links must be recoverable** — the file format has to
  preserve every `http(s)://` URL from the rendered body. The existing
  `LinkFor`-style needle/URL extraction should still work over the
  on-disk contents.
- **No PII leaks beyond the dev machine** — `/tmp/inbox` is a local-dev
  artifact. The transport must not be selectable as a side-effect of
  some other flag, and the documentation must say so.

## Success Criteria

- An operator can flip one env var, restart the server, and have every
  outbound email land on disk under a configurable directory
  (default `/tmp/inbox`).
- An automated end-to-end test (or a manual smoke script) can:
  1. Sign up with a synthetic email like `e2e+signup@example.invalid`.
  2. Find the verification email in the inbox directory.
  3. Extract the verification URL and follow it.
  4. Repeat for invitations, password reset, change email, trip
     assignment, guest registration, and folio close.
- All eight existing email kinds work identically through both
  transports.
- The on-disk format is human-readable (a person can `cat` a file and
  see the subject, recipient, and links) **and** machine-readable
  (programmatic extraction of links).
- Production-mode loader still refuses to start without SMTP unless
  filesystem transport is explicitly chosen with an audible warning.
- Existing Go unit tests using `MockSender` keep passing unchanged.
- `go test ./...`, `go vet ./...`, and the frontend build all pass.

## Open Questions

The drafts should each propose answers to the following — interview
phase will resolve any disagreements.

1. **Transport selector shape**: One env var
   (`LIVEABOARD_EMAIL_TRANSPORT=smtp|filesystem`) versus a boolean
   (`LIVEABOARD_EMAIL_FILESYSTEM=true`)? Default `smtp`.
2. **Inbox directory layout**: One subdirectory per recipient
   (`/tmp/inbox/<recipient>/<timestamp>-<kind>.eml`) versus a flat
   directory of files named with the recipient (`<recipient>.<id>.eml`)?
3. **File format**: A real `.eml` (the same MIME we'd send over SMTP,
   reusable in any MUA) versus a structured JSON envelope with subject,
   to, html, text, and an explicit `links: []` array?
4. **Link extraction**: Do we surface links as a sidecar `.links` /
   `.json` file alongside each message, or rely on grepping the body?
   The seed asks that links be "persisted in the files" — answered by
   the format choice but worth being explicit.
5. **Latest pointer**: Should we maintain `/tmp/inbox/<recipient>/latest`
   or `latest.eml` for the most recent message, so simple smoke
   scripts don't have to sort timestamps?
6. **Production guardrail**: Reject filesystem transport in production
   mode entirely, or allow it with an explicit
   `LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION=true` second-key
   opt-in?
7. **Inbox cleanup on startup**: Truncate the directory at server
   start, retain forever, or expose a small CLI / Make target to
   inspect-and-clear?
8. **Test-mode default**: Does `test` mode (used by `make test`)
   default to filesystem transport, stay on in-memory `MockSender`, or
   default to neither (skip)? The current convention is unit tests
   inject `MockSender` directly, so this matters only for
   end-to-end-style tests.
9. **Recipient address normalization**: How aggressive should the
   per-recipient folder name sanitization be — full lowercase with
   `@`/`.` allowed, or replace everything non-`[a-z0-9-]` with `_`?
10. **HTTP "inbox viewer"** (stretch): Is a tiny built-in handler that
    lists messages from disk (gated to dev mode) worth doing now, or
    deferred?
11. **Frontend ergonomics**: Does any change touch the SPA, or is the
    sprint entirely backend + docs?
