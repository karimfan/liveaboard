# Sprint 021: Filesystem Email Transport for End-to-End Testing

## Overview

Sprint 021 adds a second `email.Sender` implementation that writes
every outbound message to a configurable directory on disk instead of
shipping it to Brevo over SMTP. Default behavior is unchanged: the
production binary still sends real email. Operators (and CI scripts)
who flip a single env var get a fully-functional app that never makes
external SMTP calls, with every rendered message — including the
token-bearing URLs the SPA needs to complete signup, verification,
invitation, reset, change-email, and trip-assignment flows —
persisted under `/tmp/inbox/<recipient>/`.

This unlocks two concrete capabilities:

1. **End-to-end test scripts on the dev box.** Sign up with
   `e2e+signup@example.invalid`, read the verification message off
   disk, follow the link, exercise the rest of the flow.
2. **Demo / staging runs without an SMTP relay.** Stand the server up,
   click through the product, inspect generated mail without setting up
   or paying for Brevo credentials.

The change is intentionally narrow: one new file in `internal/email`,
two new config keys, a transport selector in `cmd/server/main.go`, and
documentation. No callers change. No frontend changes. No new
dependencies.

## Use Cases

1. **End-to-end signup script.** A test harness POSTs `/api/auth/signup`
   with `e2e+signup@example.invalid`, finds
   `/tmp/inbox/e2e+signup@example.invalid/latest.eml`, extracts the
   verification URL, follows it, and confirms the user is now
   verified.
2. **Local invitation walkthrough.** A developer invites
   `e2e+director@example.invalid`, opens the most recent message in
   their inbox folder, copies the invitation link, accepts it in a
   second browser profile.
3. **Password reset smoke test.** A script triggers `ForgotPassword`,
   reads `password_reset` from the inbox, completes the reset, and
   verifies a new login works.
4. **Folio close demo.** During a screenshare, a Cruise Director closes
   a folio; the demo box shows the resulting `guest_folio_closed`
   message including the itemized lines and settlement totals from
   Sprint 020 — no real email account required.
5. **Production safeguard.** An operator who accidentally sets
   `LIVEABOARD_EMAIL_TRANSPORT=filesystem` in production sees a hard
   startup failure unless they *also* set an explicit production-allow
   opt-in.

## Architecture

### Transport Selection

Sender selection happens once in `cmd/server/main.go`. The loader
exposes two new keys; `main.go` picks an implementation and the rest
of the app continues to see `email.Sender`.

```
┌─────────────────────────────────────┐
│ cmd/server/main.go                  │
│                                     │
│   switch cfg.EmailTransport {       │
│   case "filesystem":                │
│     sender = email.NewFilesystem(   │
│       cfg.EmailInboxDir, log)       │
│   case "smtp" (default):            │
│     sender = &email.SMTPSender{...} │
│   }                                 │
└─────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ auth.Service / httpapi handlers     │
│                                     │
│   sender.Send(ctx, msg)             │
└─────────────────────────────────────┘
         │
         ▼
┌────────────────────┐   ┌────────────────────┐
│ SMTPSender (Brevo) │   │ FilesystemSender   │
└────────────────────┘   │ writes .eml + .json│
                         │ under inbox dir    │
                         └────────────────────┘
```

### Config Surface

Two new keys are added to `internal/config/config.go`:

| Key | Default | Purpose |
|---|---|---|
| `LIVEABOARD_EMAIL_TRANSPORT` | `smtp` | `smtp` or `filesystem`. |
| `LIVEABOARD_EMAIL_INBOX_DIR` | `/tmp/inbox` | Root directory when transport is `filesystem`. |

A third optional key gates production:

| Key | Default | Purpose |
|---|---|---|
| `LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION` | `false` | Required `true` to use filesystem transport in production mode. |

Validation rules in `internal/config/config.go`:

- `EmailTransport` must be `smtp` or `filesystem`. Anything else
  rejected at load time.
- In `production` mode with `EmailTransport=filesystem`,
  `EmailFilesystemAllowProduction` must be `true`. Otherwise the
  loader returns an error mentioning both keys.
- When `EmailTransport=smtp`, the existing SMTP-presence check in
  `cmd/server/main.go` continues to apply.
- When `EmailTransport=filesystem`, the SMTP env vars are not
  required.

### Filesystem Sender

New file: `internal/email/filesystem.go`.

```go
// FilesystemSender writes outbound messages to disk under InboxDir
// instead of speaking SMTP. Intended for local development, demos,
// and end-to-end test harnesses. Never produces network I/O.
type FilesystemSender struct {
    InboxDir string
    Now      func() time.Time     // injectable for deterministic tests
    Log      *slog.Logger
    UUID     func() string        // injectable, default uuid.NewString
}

func NewFilesystemSender(inboxDir string, log *slog.Logger) *FilesystemSender
func (f *FilesystemSender) Send(ctx context.Context, msg Message) error
```

#### Directory layout

```
<InboxDir>/
  <recipient-slug>/
    2026-05-20T14-22-31Z-7c2a1d-verification.eml
    2026-05-20T14-22-31Z-7c2a1d-verification.json
    latest.eml      → symlink to most recent .eml
    latest.json     → symlink to most recent .json
```

- `<recipient-slug>` is the recipient address lowercased, with `@`,
  `.`, `+`, and `-` preserved; anything else replaced with `_`. This
  keeps `e2e+signup@example.invalid` legible.
- `.eml` is the same MIME bytes the `SMTPSender` would have transmitted
  (we reuse `buildMIME(msg)`). Any MUA on the host can open it.
- `.json` is a sidecar envelope, written atomically alongside the
  `.eml`, with the shape:

  ```json
  {
    "id": "7c2a1db1-…",
    "received_at": "2026-05-20T14:22:31Z",
    "from": "Liveaboard <noreply@…>",
    "to": "e2e+signup@example.invalid",
    "subject": "Confirm your email",
    "text_body": "Please confirm…",
    "html_body": "<p>Please confirm…</p>",
    "links": [
      "http://localhost:5173/verify-email?token=…"
    ]
  }
  ```

  `links` is built once at write time by scanning both `TextBody` and
  `HTMLBody` for `https?://…` substrings (deduplicated, order
  preserved). This is the single source of truth for "what URLs were
  in this email" and removes the burden of regex-grepping bodies in
  tests.

#### Atomic writes & symlinks

- Each file is written as `<name>.tmp` then `os.Rename`d into place,
  so a concurrent reader either sees the full file or no file.
- `latest.eml` / `latest.json` are atomically updated by writing
  `latest.eml.tmp` as a symlink and renaming it. Best-effort: if the
  filesystem rejects symlinks (rare on macOS/Linux), the sender logs
  a warning once and skips the pointer; the timestamped files remain.

#### Determinism for tests

- `Now` defaults to `time.Now().UTC()`. The tests in
  `filesystem_test.go` inject a frozen clock.
- `UUID` defaults to `uuid.NewString`. Tests inject a counter.
- Filenames sort lexically by timestamp so `ls` in chronological
  order works without additional sorting.

#### Error handling

- Missing inbox dir is created on the fly with `0o755`.
- Per-recipient subdirectory created with `0o755` on first write for
  that recipient.
- Filesystem errors propagate up; the auth flow currently treats a
  send failure as the whole-request failure (e.g. signup rolls back).
  Keeping that behavior matches SMTP semantics — a write failure is
  a "send" failure.

### Link Extraction

To keep tests cleanly decoupled from filenames, add one helper to the
`email` package:

```go
// FilesystemLinkFor finds the most recent message in InboxDir for `to`
// containing `needle` in its subject/text/html, and returns the first
// URL captured during that message's write. Mirrors MockSender.LinkFor.
func FilesystemLinkFor(inboxDir, to, needle string) (string, error)
```

This reads from the on-disk `.json` files (cheap, no MIME parse) and
returns the first matching message's `links[0]`. It is the file-backed
twin of `MockSender.LinkFor`.

`MockSender` itself is not removed and is not changed — unit tests
continue to use it. `FilesystemLinkFor` is for end-to-end harnesses
running against a real server.

### Production Guardrail

`internal/config/config.go` (`validate`):

```go
if c.Mode == ModeProduction && c.EmailTransport == "filesystem" &&
    !c.EmailFilesystemAllowProduction {
    return fmt.Errorf(
        "config: production mode with LIVEABOARD_EMAIL_TRANSPORT=filesystem requires LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION=true",
    )
}
```

`cmd/server/main.go` updates the SMTP-required check so it only runs
when `cfg.EmailTransport == "smtp"`. The filesystem path also emits a
single `log.Warn("email transport: filesystem", "inbox", cfg.EmailInboxDir)`
so it's obvious in logs that real email isn't going out.

### What does NOT change

- `email.Sender` interface signature.
- `email.SMTPSender` behavior.
- `email.MockSender` behavior — unit tests continue to inject it
  directly.
- `email.Render` and the embedded templates.
- All caller code in `internal/auth/*.go` and `internal/httpapi/*.go`.
- The frontend.

## Implementation Plan

### Phase 1: Filesystem Sender (~35%)

**Files:**
- `internal/email/filesystem.go` — new `FilesystemSender`,
  `NewFilesystemSender`, `FilesystemLinkFor`.
- `internal/email/filesystem_test.go` — new tests covering write,
  slug, JSON sidecar, link extraction, symlink update, concurrent
  writes, and atomic rename.

**Tasks:**
- [ ] Implement `FilesystemSender.Send` writing `.eml` + `.json` and
      updating `latest.*` symlinks.
- [ ] Recipient-slug helper that preserves `@.+-`, replaces anything
      else with `_`, and lowercases.
- [ ] Link extraction (scan text + html, dedupe, preserve order).
- [ ] `FilesystemLinkFor` helper reading the `.json` sidecars.
- [ ] Tests asserting: file contents reproduce SMTP MIME, JSON sidecar
      schema, slug normalization, deterministic timestamp ordering,
      concurrent writes from multiple goroutines all land, symlink
      best-effort fallback when symlinks unavailable.

### Phase 2: Config + Wiring (~25%)

**Files:**
- `internal/config/config.go` — add `EmailTransport`, `EmailInboxDir`,
  `EmailFilesystemAllowProduction`.
- `internal/config/config_test.go` — validation tests for the new keys.
- `cmd/server/main.go` — pick transport, gate SMTP-required check.
- `config/dev.env` — document the new keys as comments.
- `config/test.env` — document the new keys as comments.
- `config/production.env` — document the new keys as comments.

**Tasks:**
- [ ] Add the three new fields with `env`/`default`/`required` tags.
- [ ] Validate `EmailTransport ∈ {smtp, filesystem}` at load time.
- [ ] Validate production mode + filesystem transport requires the
      allow-production opt-in.
- [ ] Update `cmd/server/main.go` to switch on transport, build the
      filesystem sender when selected, gate the existing SMTP-present
      check to `smtp`-only, log a `Warn` on filesystem transport.
- [ ] Test cases: default loads SMTP; setting `filesystem` loads with
      no SMTP creds; production + filesystem without allow fails;
      production + filesystem with allow succeeds.

### Phase 3: Docs + Tooling (~25%)

**Files:**
- `docs/CONFIG.md` — document the three new keys.
- `docs/dev/email-inbox.md` — new short doc explaining the filesystem
  inbox layout, how to enable it, and the `latest.eml` convention.
- `scripts/inbox.sh` — small helper (or a Make target) to print
  `latest.json` for a recipient, e.g. `./scripts/inbox.sh
  e2e+signup@example.invalid`.
- `Makefile` — add `make inbox-clear` (rm -rf inbox dir contents,
  recreates the directory) and `make inbox` (prints contents).
- `.env.example` — note the filesystem transport opt-in for local dev.

**Tasks:**
- [ ] Write `docs/dev/email-inbox.md` with: the layout, how the SPA
      links open, the JSON sidecar schema, and a worked
      signup-→-verification example.
- [ ] Add CLI helpers and document them in the same doc.
- [ ] Update `docs/CONFIG.md` keys table.

### Phase 4: End-to-End Smoke Coverage (~15%)

**Files:**
- `internal/httpapi/email_filesystem_e2e_test.go` — new integration
  test that boots the HTTP server with a filesystem sender into a
  temp directory and walks signup → verification → login.

**Tasks:**
- [ ] Add a single representative e2e test using the filesystem
      transport (the rest stay on `MockSender` for speed).
- [ ] Run `go test ./...`, `go vet ./...`, frontend build.
- [ ] Update CLAUDE.md or sprint doc if anything about local dev
      workflow shifts (likely just a pointer to `docs/dev/email-inbox.md`).

## API Endpoints

None. This sprint is entirely transport-level.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/email/filesystem.go` | Create | `FilesystemSender`, slug helper, link extractor. |
| `internal/email/filesystem_test.go` | Create | Unit tests for the new sender. |
| `internal/config/config.go` | Modify | Three new fields + validation. |
| `internal/config/config_test.go` | Modify | Coverage for new fields. |
| `cmd/server/main.go` | Modify | Transport switch + gated SMTP check. |
| `config/dev.env` | Modify | Document new keys as comments. |
| `config/test.env` | Modify | Document new keys as comments. |
| `config/production.env` | Modify | Document new keys as comments. |
| `docs/CONFIG.md` | Modify | Keys table additions. |
| `docs/dev/email-inbox.md` | Create | How to use the filesystem inbox. |
| `scripts/inbox.sh` | Create | Convenience CLI. |
| `Makefile` | Modify | `make inbox`, `make inbox-clear`. |
| `.env.example` | Modify | Note filesystem opt-in. |
| `internal/httpapi/email_filesystem_e2e_test.go` | Create | End-to-end coverage. |

## Definition of Done

- [ ] `LIVEABOARD_EMAIL_TRANSPORT=filesystem` runs the server with no
      SMTP credentials and no external network calls for email.
- [ ] Default behavior (no env override) is unchanged — production
      still requires the four SMTP env vars.
- [ ] Every one of the eight email kinds lands on disk under the
      configured inbox dir.
- [ ] `<recipient>/latest.eml` resolves to the most recent message for
      that recipient.
- [ ] The `.json` sidecar lists every `http(s)://` URL from the
      rendered body, with order preserved.
- [ ] Production mode rejects `filesystem` transport unless the
      explicit allow-production opt-in is set.
- [ ] `make inbox` lists messages; `make inbox-clear` empties the
      directory.
- [ ] Existing Go unit tests using `MockSender` still pass unchanged.
- [ ] New unit + end-to-end tests pass.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `npm run build` (frontend) passes.
- [ ] `docs/CONFIG.md` and `docs/dev/email-inbox.md` reflect the new
      surface.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| An operator silently turns on filesystem transport in production and stops sending real mail. | Low | High | Loader rejects it without explicit allow-production opt-in; startup log warns on filesystem mode. |
| `/tmp/inbox` fills disk over time in long-running dev/demo sessions. | Medium | Low | `make inbox-clear`; document manual cleanup; consider future automatic rotation if it becomes a problem. |
| Symlink updates race with concurrent writers and leave stale `latest.eml`. | Low | Low | Atomic-rename symlinks; if rename fails, log once and skip — timestamped files remain authoritative. |
| Filename collisions on rapid sequential sends. | Low | Low | Timestamp + 6-char UUID suffix in filenames. |
| Recipient slug collisions (e.g. two emails that normalize to the same dir). | Very low | Low | Normalization preserves the full local-part and domain; only non-`[a-z0-9@.+-]` chars become `_`. |
| Tests on macOS with stricter `/tmp` permissions. | Low | Low | Sender accepts an arbitrary dir; tests use `t.TempDir()` rather than `/tmp/inbox`. |

## Security Considerations

- The filesystem inbox is a **local-dev / staging affordance** and must
  not be enabled in production by default. Production-mode validation
  refuses transport=filesystem without the explicit allow flag, and a
  startup warning fires whenever filesystem transport is in use.
- Tokens written to disk grant the same privileges as the real email
  would. Operators choosing filesystem mode are accepting that anyone
  with shell access to the host can complete any in-flight verification,
  invitation, or reset.
- `LIVEABOARD_EMAIL_INBOX_DIR` should not point at a directory served
  by any HTTP handler. The default `/tmp/inbox` is outside both the
  static asset roots and `LIVEABOARD_DOCUMENTS_DIR`.
- The sender does not log token URLs in plaintext; the `.json` sidecar
  is the only on-disk artifact containing them.
- `MockSender` remains the right choice for unit tests; the filesystem
  sender does I/O and is reserved for end-to-end runs.

## Dependencies

- Sprint 009 (custom auth + Brevo SMTP) established the `email.Sender`
  abstraction this sprint extends.
- No new external dependencies. `github.com/google/uuid` is already in
  use elsewhere in the codebase.

## Open Questions

- Whether to add a built-in HTTP "dev inbox viewer" page so devs can
  click links without leaving the browser. Recommended **not** for this
  sprint — keep the surface narrow; revisit if friction warrants it.
- Whether to support a second slug strategy (hash-based) for shared
  test environments with overlapping email addresses. Defer until we
  see the case.
- Whether to rotate or cap the inbox dir size automatically. Defer;
  `make inbox-clear` is enough for now.

## References

- `internal/email/email.go` — `Sender` interface and `MockSender`
  precedent for link extraction.
- `internal/email/smtp.go` — existing transport whose API shape this
  sprint mirrors.
- `internal/email/message.go` — `buildMIME` reused for the `.eml`
  artifact.
- `internal/config/config.go` — typed config + loader pattern.
- `docs/CONFIG.md` — config documentation contract.
- `docs/sprints/SPRINT-009.md` — original email infrastructure context.
