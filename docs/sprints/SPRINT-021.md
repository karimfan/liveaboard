# Sprint 021: Filesystem Email Transport for End-to-End Testing

## Overview

Sprint 021 adds a second `email.Sender` implementation that writes
every outbound message to a configurable on-disk inbox instead of
shipping it to Brevo over SMTP. Default behavior is unchanged in
every mode: the production binary still sends real email. Developers
and CI scripts who flip a single env var get a fully-functional app
that never makes external SMTP calls, with every rendered message —
including the token-bearing URLs the SPA needs to complete signup,
verification, invitation, reset, change-email, and trip-assignment
flows — persisted under `<recipient>/` directories inside the inbox
root.

This unlocks two concrete capabilities:

1. **End-to-end test scripts on the dev box.** Sign up with
   `e2e+signup@example.invalid`, read the verification email off
   disk, follow the link, exercise the rest of the flow — all
   against a real HTTP server, with no SMTP relay and no real
   recipient addresses involved.
2. **Demos and staging walkthroughs without an SMTP relay.** Stand
   the server up, click through the product, inspect generated mail
   in a browser via the dev-mode `/dev/inbox` viewer.

The change is intentionally narrow. One shared link-extraction
helper, one new sender, two new config keys, a small dev-mode HTTP
viewer, and CLI ergonomics. No new external dependencies. No caller
changes anywhere in `internal/auth/*` or `internal/httpapi/*`. No
frontend changes.

## Use Cases

1. **End-to-end signup script.** A test harness POSTs
   `/api/auth/signup` with `e2e+signup@example.invalid`, reads
   `<inbox>/e2e+signup@example.invalid/latest.json`, extracts the
   verification URL from `links[0]`, follows it, and confirms the
   user is now verified.
2. **Local invitation walkthrough.** A developer invites
   `e2e+director@example.invalid`, opens `/dev/inbox` in a browser,
   clicks the invitation link directly.
3. **Password reset smoke test.** A script triggers
   `ForgotPassword`, reads `password_reset` from the inbox,
   completes the reset, verifies a new login.
4. **Folio close demo.** During a screenshare, a Cruise Director
   closes a folio; the demo box shows the resulting
   `guest_folio_closed` message in `/dev/inbox`, including the
   itemized lines and settlement totals from Sprint 020 — no real
   mail account required.
5. **Production safeguard.** An operator who accidentally sets
   `LIVEABOARD_EMAIL_TRANSPORT=filesystem` in production sees a
   hard startup failure: filesystem transport is rejected outright
   in `ModeProduction` and the binary will not start.

## Architecture

### Core Decisions

- Keep `email.Sender` as the single transport abstraction.
- Add `FilesystemSender` in `internal/email/filesystem.go`. No caller
  outside sender construction learns about transports.
- Keep `MockSender` for unit tests — but refactor it to share link
  extraction with the filesystem sender, so the two paths cannot
  drift.
- Keep SMTP as the default transport in every mode unless explicitly
  overridden.
- Use an enum selector: `LIVEABOARD_EMAIL_TRANSPORT=smtp|filesystem`.
- Add `LIVEABOARD_EMAIL_FILESYSTEM_DIR` with default `/tmp/inbox`.
- **Production hard-rejects `filesystem`** at config-load time. No
  opt-in flag, no warning-only mode — production cannot use this
  transport.
- Reuse `buildMIME(msg)` so the `.eml` artifact contains the exact
  bytes SMTP would have transmitted.
- Add a small dev-mode HTTP inbox viewer, mounted only when
  `Mode=dev` and `EmailTransport=filesystem`.
- Do not auto-clear the inbox on startup. `make inbox-clear` is the
  cleanup primitive; the server itself never deletes files.

### Transport Selection

Sender selection happens once in `cmd/server/main.go`.

```
                    ┌─────────────────────────┐
                    │ cmd/server/main.go      │
                    │                         │
                    │ switch cfg.EmailTransport│
                    │ case "filesystem":       │
                    │   email.NewFilesystem(   │
                    │     cfg.EmailFilesystemDir,
                    │     log)                 │
                    │ case "smtp" (default):   │
                    │   &email.SMTPSender{...} │
                    └────────────┬────────────┘
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │ auth.Service / httpapi  │
                    │                         │
                    │ sender.Send(ctx, msg)   │
                    └────────────┬────────────┘
                                 │
              ┌──────────────────┴──────────────────┐
              ▼                                     ▼
   ┌──────────────────────┐                ┌──────────────────────┐
   │ SMTPSender (Brevo)   │                │ FilesystemSender     │
   │                      │                │  writes .eml+.json   │
   │                      │                │  under inbox dir     │
   └──────────────────────┘                └──────────────────────┘
```

### Config Surface

Two new keys in `internal/config/config.go`:

| Key | Default | Required | Purpose |
|---|---|---|---|
| `LIVEABOARD_EMAIL_TRANSPORT` | `smtp` | no | `smtp` or `filesystem`. |
| `LIVEABOARD_EMAIL_FILESYSTEM_DIR` | `/tmp/inbox` | no | Inbox root when transport is `filesystem`. |

Validation in `Config.validate()`:

- `EmailTransport` ∈ `{smtp, filesystem}`. Anything else rejected
  with both the offending value and the allowed set in the error.
- `EmailTransport=filesystem` in `ModeProduction` is rejected
  unconditionally with: `"production mode does not permit
  LIVEABOARD_EMAIL_TRANSPORT=filesystem"`.
- `EmailTransport=filesystem` requires a non-empty
  `EmailFilesystemDir`.
- When `EmailTransport=smtp`, the existing SMTP-presence check in
  `cmd/server/main.go` continues to apply unchanged.
- When `EmailTransport=filesystem`, the SMTP env vars are no longer
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
    UUID     func() string        // injectable; default uuid.NewString
}

func NewFilesystemSender(inboxDir string, log *slog.Logger) *FilesystemSender
func (f *FilesystemSender) Send(ctx context.Context, msg Message) error
```

#### Directory layout

```
<InboxDir>/
  e2e+signup@example.invalid/
    20260520T142231Z-7c2a1d.eml
    20260520T142231Z-7c2a1d.json
    latest.eml       (regular file, atomically rewritten)
    latest.json      (regular file, atomically rewritten)
```

- `<recipient-slug>` is the recipient address lowercased, with
  `@ . + _ -` preserved; anything else replaced with `_`. Path
  traversal characters (`/`, `..`, control bytes) are never
  forwarded.
- `.eml` is the exact MIME bytes from `buildMIME(msg)`. Any MUA on
  the host can open it.
- `.json` is the sidecar envelope (schema below).
- `latest.eml` and `latest.json` are **regular files**, not
  symlinks. On each send they are written via temp-file +
  `os.Rename` so a concurrent reader sees either the previous
  message or the new one — never a partial file. This is the
  contract smoke scripts can rely on.

#### `.json` sidecar schema

```json
{
  "id": "7c2a1db1-…",
  "from": "Liveaboard <noreply@…>",
  "to": "e2e+signup@example.invalid",
  "subject": "Confirm your email",
  "sent_at": "2026-05-20T14:22:31Z",
  "eml_path": "<inbox>/e2e+signup@example.invalid/20260520T142231Z-7c2a1d.eml",
  "links": [
    "http://localhost:5173/verify-email?token=…"
  ]
}
```

The sidecar deliberately does **not** duplicate `text_body` or
`html_body`. The `.eml` is the full-fidelity body artifact;
the sidecar is the automation contract for "what was sent, where is
it, and which URLs does it contain".

#### Atomic writes

- Each file is written as `<name>.tmp` then `os.Rename`d into place.
- `latest.eml` / `latest.json` are updated by writing
  `latest.eml.tmp` / `latest.json.tmp` and atomically renaming over
  the previous version.
- Inbox dir and per-recipient subdir are created on the fly with
  `0o755` permissions.

#### Shared link extraction

New helper in `internal/email/email.go` (or new
`internal/email/links.go`):

```go
// ExtractLinks scans the text and HTML bodies of msg for http(s) URLs
// and returns them in first-occurrence order. Duplicates are removed.
func ExtractLinks(msg Message) []string
```

`MockSender.LinkFor` is refactored to use the same helper internally;
`FilesystemSender` calls it to populate `links[]` in the sidecar.
This is the explicit fix for the contract-drift risk: both transports
recover URLs through one implementation.

#### Determinism for tests

- `Now` defaults to `time.Now().UTC()`. Tests inject a frozen clock.
- `UUID` defaults to `uuid.NewString`. Tests inject a counter.
- Filenames sort lexically by timestamp; `ls` yields chronological
  order without extra sorting.

#### Error handling

- Address validation runs first (same `net/mail` parse as
  `SMTPSender`).
- Filesystem errors propagate up. Auth flows currently treat a send
  failure as the whole-request failure — keeping that behavior
  matches SMTP semantics.

### Dev-Mode HTTP Inbox Viewer

New file: `internal/httpapi/dev_inbox_handlers.go`. Mounted only when:

```go
cfg.Mode == config.ModeDev && cfg.EmailTransport == "filesystem"
```

Three endpoints, all GET-only, all served on the regular API port:

| Endpoint | Returns |
|---|---|
| `/dev/inbox` | HTML index: recipients with their latest message timestamp + subject. |
| `/dev/inbox/{recipient}` | HTML list of messages for that recipient (newest first), each linking to the artifact. |
| `/dev/inbox/{recipient}/{id}` | The `.eml` rendered as HTML if available, or the raw `.eml` if not. |

Behavior:

- Recipient names in the URL are validated against the same slug
  rules as the writer. Anything that does not match is rejected
  with `400`.
- The handler does **not** read outside `cfg.EmailFilesystemDir`.
- The handler is wired only when both conditions above hold;
  `internal/httpapi/httpapi.go` skips the route mount otherwise. A
  request to `/dev/inbox` in any other configuration returns
  `404` from the existing default handler.
- No auth on these endpoints — they are dev-mode only and the route
  is not mounted in production.

### CLI Ergonomics

- `scripts/inbox.sh <recipient>`: pretty-prints
  `<InboxDir>/<recipient>/latest.json`. Resolves `<InboxDir>` from
  `LIVEABOARD_EMAIL_FILESYSTEM_DIR` or defaults to `/tmp/inbox`.
- `make inbox` lists every recipient and the latest message under
  the configured inbox dir.
- `make inbox-clear` removes contents of the inbox dir and
  recreates the root. Confirms before deleting unless `FORCE=1`.

### What Does NOT Change

- `email.Sender` interface signature.
- `email.SMTPSender` behavior.
- `email.Render` and the embedded templates.
- All caller code in `internal/auth/*.go` and
  `internal/httpapi/*.go` (except for one new file mounting the
  dev-only inbox viewer).
- The frontend.

## Implementation Plan

### Phase 1: Shared Link Extractor + Filesystem Sender (~30%)

**Files:**
- `internal/email/links.go` — new shared `ExtractLinks(msg Message)
  []string` helper.
- `internal/email/email.go` — refactor `MockSender.LinkFor` to use
  `ExtractLinks`.
- `internal/email/filesystem.go` — new sender.
- `internal/email/filesystem_test.go` — unit tests for the sender.
- `internal/email/email_test.go` — extended tests for the shared
  extractor (or a new test file).

**Tasks:**
- [x] Implement `ExtractLinks` scanning both text and HTML bodies,
      preserving first-occurrence order, deduping case-sensitively.
- [x] Refactor `MockSender.LinkFor` to call `ExtractLinks` internally
      while preserving its existing signature.
- [x] Implement `FilesystemSender.Send` writing `.eml` + `.json`
      atomically and rewriting `latest.*` files atomically.
- [x] Recipient-slug helper that preserves `@.+_-`, replaces anything
      else with `_`, lowercases.
- [x] Tests asserting: `.eml` reproduces SMTP MIME byte-for-byte;
      sidecar JSON schema; slug normalization including hostile
      characters; deterministic timestamp ordering; concurrent
      writes from multiple goroutines all land without collision;
      `latest.*` content matches the newest send under concurrent
      writers.

### Phase 2: Config + Wiring (~20%)

**Files:**
- `internal/config/config.go` — add `EmailTransport`,
  `EmailFilesystemDir`; validation rules.
- `internal/config/config_test.go` — coverage for the new keys.
- `cmd/server/main.go` — pick transport; gate the existing SMTP
  presence check to `smtp` only; log a `Warn` on filesystem mode.
- `config/dev.env` — document new keys as comments.
- `config/test.env` — document new keys as comments.
- `config/production.env` — document that filesystem mode is
  rejected.

**Tasks:**
- [x] Add the two new fields with `env`/`default` tags.
- [x] Validate `EmailTransport ∈ {smtp, filesystem}`.
- [x] Hard-reject `EmailTransport=filesystem` in `ModeProduction`.
- [x] Validate `EmailFilesystemDir` non-empty when transport is
      filesystem.
- [x] Update `cmd/server/main.go` to switch on transport, build
      `FilesystemSender` when selected, gate the SMTP-presence check
      to `smtp` only, log `slog.Warn("email transport: filesystem",
      "inbox_dir", …)` once on the filesystem path.
- [x] Config tests: default loads SMTP; `filesystem` loads without
      SMTP creds; `production+filesystem` is rejected at load time
      with a clear error message; bogus transport string rejected.

### Phase 3: Dev Ergonomics — HTTP Viewer + CLI + Make Targets (~20%)

**Files:**
- `internal/httpapi/dev_inbox_handlers.go` — new viewer.
- `internal/httpapi/httpapi.go` — conditional route mount.
- `internal/httpapi/dev_inbox_handlers_test.go` — handler tests.
- `scripts/inbox.sh` — new CLI helper.
- `Makefile` — `make inbox` and `make inbox-clear` targets.

**Tasks:**
- [x] Implement three GET-only handlers for `/dev/inbox`,
      `/dev/inbox/{recipient}`, `/dev/inbox/{recipient}/{id}`.
- [x] Wire route mount conditional on `Mode=dev` and
      `EmailTransport=filesystem`.
- [x] Slug-validate path parameters; reject anything else with `400`.
- [x] Refuse reads outside `EmailFilesystemDir`.
- [x] `scripts/inbox.sh <recipient>` reads
      `${LIVEABOARD_EMAIL_FILESYSTEM_DIR:-/tmp/inbox}/<recipient>/latest.json`
      and `jq .` it.
- [x] `make inbox` and `make inbox-clear` targets. `make inbox-clear`
      prompts for confirmation unless `FORCE=1`.
- [x] Handler tests: route NOT mounted in `Mode=test` even with
      filesystem transport; route NOT mounted in `Mode=dev` with
      `smtp` transport; route returns `400` for slug-violating
      paths; handler refuses to traverse outside the inbox dir.

### Phase 4: Tests — Per-Kind Matrix + One HTTP E2E (~20%)

**Files:**
- `internal/email/filesystem_test.go` — per-kind matrix test.
- `internal/httpapi/email_filesystem_e2e_test.go` — new HTTP-level
  end-to-end test.

**Tasks:**
- [x] Per-kind matrix test: for each of the eight `email.Kind`
      values, render with representative `Vars`, send through
      `FilesystemSender` into `t.TempDir()`, assert the `.eml`
      exists, the `.json` parses, the `links` slice matches what
      `ExtractLinks` produces on the rendered `Message`.
- [x] One HTTP-level e2e: boot the test HTTP server with
      `FilesystemSender` into `t.TempDir()`, POST
      `/api/auth/signup` with a synthetic recipient, read
      `latest.json` for that recipient, follow the verification
      URL through `/api/auth/verify-email`, confirm the user is now
      verified.
- [x] Existing unit tests using `MockSender` continue to pass with
      no code changes (verify the refactored `LinkFor` is
      behavior-compatible).

### Phase 5: Docs and Verification (~10%)

**Files:**
- `docs/CONFIG.md` — document the two new keys.
- `docs/dev/email-inbox.md` — new doc.

**Tasks:**
- [x] Add the two keys to the `docs/CONFIG.md` keys table.
- [x] Write `docs/dev/email-inbox.md` covering: enabling filesystem
      mode, layout, the `latest.json` contract, the `/dev/inbox`
      viewer, the `scripts/inbox.sh` and `make` targets, and the
      hard production rejection.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `npm run build`.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/dev/inbox` | GET | Dev-mode-only index of recipients. |
| `/dev/inbox/{recipient}` | GET | Dev-mode-only message list per recipient. |
| `/dev/inbox/{recipient}/{id}` | GET | Dev-mode-only message viewer. |

All three endpoints are mounted only when `Mode=dev` AND
`EmailTransport=filesystem`. None are mounted in any other
configuration; the production binary never exposes them.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/email/links.go` | Create | Shared `ExtractLinks` helper. |
| `internal/email/email.go` | Modify | `MockSender.LinkFor` uses `ExtractLinks`. |
| `internal/email/filesystem.go` | Create | `FilesystemSender` and slug helper. |
| `internal/email/filesystem_test.go` | Create | Sender + per-kind matrix tests. |
| `internal/email/email_test.go` | Modify | Cover the shared extractor. |
| `internal/config/config.go` | Modify | Add `EmailTransport` and `EmailFilesystemDir`; validation. |
| `internal/config/config_test.go` | Modify | Coverage for new fields and the production rejection. |
| `cmd/server/main.go` | Modify | Transport switch; gated SMTP-presence check; startup log line. |
| `internal/httpapi/dev_inbox_handlers.go` | Create | Dev-mode HTTP inbox viewer. |
| `internal/httpapi/dev_inbox_handlers_test.go` | Create | Viewer mount-conditional + handler tests. |
| `internal/httpapi/httpapi.go` | Modify | Conditional route mount for the dev viewer. |
| `internal/httpapi/email_filesystem_e2e_test.go` | Create | One end-to-end signup → verify flow. |
| `config/dev.env` | Modify | Comment-document the new keys. |
| `config/test.env` | Modify | Comment-document the new keys. |
| `config/production.env` | Modify | Note that filesystem transport is rejected. |
| `scripts/inbox.sh` | Create | `latest.json` pretty-printer per recipient. |
| `Makefile` | Modify | `make inbox` and `make inbox-clear`. |
| `docs/CONFIG.md` | Modify | Document the two new keys. |
| `docs/dev/email-inbox.md` | Create | Operator and dev guide for the inbox. |

## Definition of Done

- [x] `LIVEABOARD_EMAIL_TRANSPORT=filesystem` runs the server with no
      SMTP credentials and no external network calls for email.
- [x] Default behavior (no env override) is unchanged in every mode;
      production still requires the four SMTP env vars.
- [x] All eight email kinds (`verification`, `invitation`,
      `password_reset`, `change_email`, `trip_assigned`,
      `trip_unassigned`, `guest_registration_invite`,
      `guest_folio_closed`) land on disk under the configured inbox
      dir when filesystem transport is selected, proven by the
      per-kind matrix test.
- [x] `<recipient>/latest.eml` and `<recipient>/latest.json` are
      regular files (not symlinks) that always reflect the most
      recent message for that recipient, even under concurrent
      writers.
- [x] `.eml` artifacts are byte-identical to what `SMTPSender` would
      have transmitted for the same `Message`.
- [x] `.json` sidecar matches the schema (`id`, `from`, `to`,
      `subject`, `sent_at`, `eml_path`, `links`) and `links` is
      produced by the shared `ExtractLinks` helper.
- [x] `MockSender.LinkFor` is refactored to use `ExtractLinks`
      internally and existing unit tests pass unchanged.
- [x] Production mode hard-rejects `EmailTransport=filesystem` at
      config-load time with a clear error message.
- [x] `/dev/inbox` viewer is mounted only when `Mode=dev` AND
      `EmailTransport=filesystem`; it is never mounted in other
      configurations and is never available in production.
- [x] `/dev/inbox` viewer rejects path-traversal attempts and
      slug-violating recipient names with `400`.
- [x] `make inbox`, `make inbox-clear`, and `scripts/inbox.sh` are
      documented and work against the default inbox dir.
- [x] One HTTP-level e2e test boots the server with filesystem
      transport, completes a signup → verification flow by reading
      the inbox, and asserts the user is now verified.
- [x] `docs/CONFIG.md` and `docs/dev/email-inbox.md` reflect the
      new surface.
- [x] `go test ./...` passes.
- [x] `go vet ./...` passes.
- [x] `npm run build` (frontend) passes.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `MockSender` and `FilesystemSender` link extraction drift. | Was: Medium | High | Single shared `ExtractLinks` helper; both transports route through it. |
| Symlink `latest.*` behaves inconsistently across hosts. | N/A | N/A | Use atomic-rewrite regular files instead of symlinks. |
| Operator accidentally turns on filesystem mode in production. | Low | High | Loader hard-rejects `production+filesystem` at startup; no opt-in flag exists. |
| `/tmp/inbox` fills disk in long demo sessions. | Medium | Low | `make inbox-clear`; documented; server never auto-deletes. |
| Filename collisions on rapid sequential sends. | Low | Low | UTC timestamp (microsecond) + 6-char UUID suffix. |
| Recipient slug collisions. | Very low | Low | Normalization preserves the full local-part and domain; only non-`[a-z0-9@.+_-]` chars become `_`. Tests assert this. |
| Dev inbox viewer reachable in unintended modes. | Very low | Medium | Route is mounted only when both `Mode=dev` and `EmailTransport=filesystem` hold; handler tests assert non-mount cases. |
| Dev inbox viewer used as a directory-traversal pivot. | Low | High | Path parameters slug-validated; handler refuses paths outside the inbox dir. |

## Security Considerations

- The filesystem inbox is a **local-dev / staging affordance** and is
  hard-rejected in production mode. The config loader will refuse
  to start the binary if `Mode=production` and
  `EmailTransport=filesystem` are both set.
- Token-bearing URLs are written to disk in **both** the `.eml` and
  the `.json` artifacts. Anyone with shell access to the host can
  complete any in-flight verification, invitation, or reset.
- `EmailFilesystemDir` should not point at a directory served by any
  HTTP handler other than the dev-mode `/dev/inbox` viewer. The
  default `/tmp/inbox` is outside both the static asset roots and
  `LIVEABOARD_DOCUMENTS_DIR`.
- The sender does not log token URLs in plaintext. Startup logs the
  inbox directory (which is not a secret) and the chosen transport.
- The dev inbox viewer is not authenticated. It is only mounted in
  `Mode=dev`, never in test or production; this is enforced both at
  route-mount time and by the production rejection of filesystem
  transport.
- Recipient-derived paths are normalized through a single helper
  before any disk write; path traversal characters are stripped.

## Dependencies

- Sprint 009 (custom auth + Brevo SMTP) established the
  `email.Sender` abstraction this sprint extends.
- Sprint 010, 018, 019, 020 added the seven other email kinds; this
  sprint must support all of them through the new transport.
- No new external Go dependencies. `github.com/google/uuid` is
  already in use elsewhere in the codebase.

## References

- `internal/email/email.go` — `Sender` interface and existing
  `MockSender.LinkFor` precedent.
- `internal/email/smtp.go` — existing transport whose API shape this
  sprint mirrors.
- `internal/email/message.go` — `buildMIME` reused for the `.eml`
  artifact.
- `internal/config/config.go` — typed config + loader pattern.
- `docs/CONFIG.md` — config documentation contract.
- `docs/sprints/SPRINT-009.md` — original email infrastructure
  context.
- `docs/sprints/drafts/SPRINT-021-INTENT.md`
- `docs/sprints/drafts/SPRINT-021-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-021-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-021-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-021-MERGE-NOTES.md`
