# Sprint 021 Codex Draft: Filesystem Email Transport for End-to-End Testing

## Overview

Sprint 021 adds a second production-grade email transport behind the
existing `email.Sender` interface: a local filesystem inbox. The goal is
not to replace SMTP or the unit-test `MockSender`; it is to let a real
running server persist every rendered outbound email to disk so end-to-end
tests and manual smoke flows can recover token-bearing links without a live
relay or real recipient addresses.

The sprint keeps SMTP as the default and preserves the current centralized
construction point in `cmd/server/main.go`. Operators opt in explicitly via
config. In filesystem mode, every email is written to a per-recipient inbox
directory under a configurable root, with both the exact MIME message and a
small machine-readable sidecar that exposes links and metadata for test
automation.

## Use Cases

1. **Signup verification without SMTP.** A developer runs the server with
   filesystem transport, signs up with `e2e+signup@example.invalid`,
   opens the newest inbox artifact for that recipient, extracts the
   verification URL, and completes the flow.
2. **Invitation and reset smoke tests.** Staff-triggered invitation,
   change-email, and password-reset emails can be exercised end-to-end
   without mailbox provisioning.
3. **Operational email coverage.** Trip assignment, trip unassignment,
   guest registration invite, and folio-close emails land in the same
   inbox format, so the app's full email surface can be validated through
   one transport.
4. **Human-readable debugging.** A developer can `cat` a message artifact
   and confirm recipient, subject, and body contents when an email-driven
   workflow fails.
5. **Scriptable E2E automation.** A Playwright or shell test can locate a
   recipient's `latest.json`, read the extracted links array, and follow
   the right URL without parsing HTML or quoted-printable MIME.

## Architecture

### Core Decisions

- Keep `email.Sender` as the only transport abstraction.
- Add `FilesystemSender` in `internal/email/filesystem.go`; no caller
  outside sender construction learns about transports.
- Keep `MockSender` unchanged for unit tests.
- Keep SMTP as the default transport in every mode unless explicitly
  overridden.
- Use an enum selector instead of a boolean:
  `LIVEABOARD_EMAIL_TRANSPORT=smtp|filesystem`.
- Add `LIVEABOARD_EMAIL_FILESYSTEM_DIR` with default `/tmp/inbox`.
- Guard production use with a second explicit key:
  `LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION=true`.
- Do not auto-clear the inbox on startup. Retention is safer and simpler;
  docs can recommend `rm -rf` before smoke runs.
- Do not add an inbox HTTP viewer in this sprint. The transport itself is
  the sprint; browsing artifacts over HTTP is follow-up scope.
- No SPA work is required. This sprint is backend, config, docs, and test
  tooling only.

### Transport Selection and Startup Rules

`internal/config.Config` gains typed email transport fields:

- `EmailTransport string` from `LIVEABOARD_EMAIL_TRANSPORT`, default
  `smtp`
- `EmailFilesystemDir string` from
  `LIVEABOARD_EMAIL_FILESYSTEM_DIR`, default `/tmp/inbox`
- `EmailFilesystemAllowProduction bool` from
  `LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION`, default `false`

Validation rules:

- `smtp` requires the current SMTP settings exactly as today.
- `filesystem` requires a non-empty inbox directory.
- unknown transport values fail at load time.
- `production + filesystem` fails unless
  `LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION=true`.

`cmd/server/main.go` continues to be the single switch point:

- construct `SMTPSender` for `smtp`
- construct `FilesystemSender` for `filesystem`
- log the chosen transport and inbox dir for non-secret observability
- emit a loud warning when production is explicitly allowed to use
  filesystem transport

### Filesystem Inbox Layout

Inbox root:

```text
/tmp/inbox/
  e2e+signup@example.invalid/
    20260523T154501.123456Z-4f3c2a.eml
    20260523T154501.123456Z-4f3c2a.json
    latest.eml
    latest.json
```

Rules:

- one subdirectory per normalized recipient email address
- keep `@`, `.`, `+`, `_`, and `-`; lowercase the address; replace any
  other path-hostile character with `_`
- file stem is UTC timestamp plus random suffix so concurrent sends do not
  collide
- `latest.eml` and `latest.json` are overwritten atomically on each send
  for that recipient

Per-recipient directories are the right tradeoff here:

- smoke tests can target one inbox path directly
- humans do not need to grep a flat directory
- filename does not need to encode email kind, which the current
  `Message` type does not carry

### Artifact Format

Each send writes two files with the same stem.

1. `.eml`

- contains the exact MIME payload built from `buildMIME(msg)`
- preserves the real `From`, `To`, `Subject`, text part, and optional HTML
  part
- is human-readable enough for terminal inspection and realistic enough
  for opening in mail clients if needed

2. `.json`

```json
{
  "from": "Liveaboard <noreply@example.com>",
  "to": "e2e+signup@example.invalid",
  "subject": "Verify your email",
  "sent_at": "2026-05-23T15:45:01.123456Z",
  "eml_path": "/tmp/inbox/e2e+signup@example.invalid/20260523T154501.123456Z-4f3c2a.eml",
  "links": [
    "http://localhost:5173/verify-email?token=..."
  ]
}
```

Link extraction rules:

- extract from `TextBody` first, line by line, using the same
  `firstURL(...)` logic style already used by `MockSender.LinkFor`
- scan both text and HTML bodies so emails whose link appears only in HTML
  are still recoverable
- preserve all discovered `http://` and `https://` URLs in send order
- allow duplicate links if they appear in both body forms; tests can pick
  the first or de-duplicate if needed

The `.json` sidecar is the machine-readable contract for E2E tests. The
`.eml` remains the human/debug artifact and the ground truth for the exact
rendered message.

### Sender Behavior

`FilesystemSender.Send(ctx, msg)` should:

1. validate `msg.From` and `msg.To` with the same address parsing rules as
   `SMTPSender`
2. create the recipient directory with `0755`
3. build MIME bytes through the existing `buildMIME(msg)` helper
4. extract links from the rendered bodies
5. write `<stem>.eml` and `<stem>.json` via temp-file + rename so readers
   never see partial artifacts
6. refresh `latest.eml` and `latest.json` via the same atomic write path

It should not:

- perform cleanup
- mutate the message
- reach outside the configured inbox root

### Helper APIs for Tests and Tools

Add filesystem-friendly helpers beside `MockSender` in `internal/email`,
for example:

- `ExtractLinks(msg Message) []string`
- `InboxLinkFor(rootDir, to, needle string) (string, error)`
- `LatestInboxMetadata(rootDir, to string) (InboxMessage, error)`

The priority is to share parsing logic instead of re-implementing URL
scanning in future end-to-end tests. `MockSender.LinkFor` should be
refactored to reuse the same extraction helper so the in-memory and
filesystem transports follow the same contract.

## Implementation Plan

### Phase 1: Config and Transport Selection (~20%)

**Files:**
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/server/main.go`
- `config/dev.env`
- `config/test.env`
- `config/production.env`
- `docs/CONFIG.md`

**Tasks:**
- [ ] Add typed config fields for transport selector, filesystem dir, and
  production allow flag.
- [ ] Validate selector values and production guardrails in
  `internal/config`.
- [ ] Keep SMTP env requirements enforced only when transport is `smtp`.
- [ ] Update mode env files and config docs with safe defaults and
  examples.
- [ ] Switch sender construction in `cmd/server/main.go` from fixed SMTP
  to transport-based construction.

### Phase 2: Filesystem Sender and Shared Link Extraction (~35%)

**Files:**
- `internal/email/filesystem.go`
- `internal/email/email.go`
- `internal/email/message.go`
- `internal/email/filesystem_test.go`
- `internal/email/message_test.go`

**Tasks:**
- [ ] Implement `FilesystemSender`.
- [ ] Add a small `InboxMessage` metadata struct for `.json` artifacts.
- [ ] Factor shared URL extraction out of `MockSender.LinkFor`.
- [ ] Reuse `buildMIME(msg)` so `.eml` artifacts mirror real SMTP payloads.
- [ ] Write atomically to both stemmed files and `latest.*`.
- [ ] Cover address validation, directory normalization, latest-pointer
  refresh, link extraction, and concurrent-send filename uniqueness.

### Phase 3: Startup Safety, Logging, and Operational Docs (~15%)

**Files:**
- `cmd/server/main.go`
- `docs/CONFIG.md`
- `docs/sprints/drafts/SPRINT-021-CODEX-DRAFT.md`

**Tasks:**
- [ ] Log active email transport at startup without exposing secrets.
- [ ] Log a warning when filesystem transport is enabled.
- [ ] In production mode, refuse filesystem transport unless the explicit
  allow flag is set.
- [ ] Document local smoke-test workflow: set transport, clear inbox
  manually if desired, restart server, inspect `latest.json`.

### Phase 4: End-to-End Test Surface and Manual Smoke Coverage (~20%)

**Files:**
- `internal/httpapi/httpapi_test.go`
- `internal/httpapi/guest_registration_test.go`
- `internal/httpapi/invitation_metadata_test.go`
- `internal/email/filesystem_test.go`
- `docs/testing/email-filesystem-transport.md`

**Tasks:**
- [ ] Add focused tests that prove all existing email-producing flows still
  render link-bearing messages through shared extraction helpers.
- [ ] Add at least one integration-style test that boots a server-side flow
  with `FilesystemSender` and reads the resulting inbox artifact from disk.
- [ ] Write a concise smoke doc covering verification, invitation, and
  password reset as representative flows.
- [ ] Explicitly document that unit tests should keep using `MockSender`;
  filesystem transport is for running the app and E2E-style tests.

### Phase 5: Verification and Cleanup (~10%)

**Files:**
- `Makefile`
- `docs/testing/email-filesystem-transport.md`

**Tasks:**
- [ ] Add or document a simple command sequence for local inbox cleanup;
  keep cleanup outside the server startup path.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run the frontend build to confirm no unintended regressions.

## API Endpoints

No new HTTP endpoints are required for Sprint 021.

The sprint intentionally keeps inbox inspection on the filesystem rather
than exposing it over the app's HTTP surface.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/email/filesystem.go` | Create | Filesystem-backed `email.Sender` implementation |
| `internal/email/filesystem_test.go` | Create | Sender and artifact-format tests |
| `internal/email/email.go` | Modify | Shared link extraction helpers; keep `MockSender` aligned |
| `internal/config/config.go` | Modify | Email transport config surface and validation |
| `internal/config/config_test.go` | Modify | Config load and guardrail coverage |
| `cmd/server/main.go` | Modify | Centralized transport selection and startup logging |
| `config/dev.env` | Modify | Safe local default transport config |
| `config/test.env` | Modify | Test-mode transport config knobs for E2E use |
| `config/production.env` | Modify | Explicit filesystem-production override documentation |
| `docs/CONFIG.md` | Modify | Document new keys and production safety rules |
| `docs/testing/email-filesystem-transport.md` | Create | Smoke-test and E2E usage guide |

## Definition of Done

- [ ] SMTP remains the default transport when no new env var is set.
- [ ] Server startup still fails for missing SMTP config when transport is
  `smtp`.
- [ ] Server startup succeeds without SMTP credentials when transport is
  `filesystem`.
- [ ] Filesystem transport writes every outbound email kind to disk:
  verification, invitation, password reset, change email, trip assigned,
  trip unassigned, guest registration invite, and guest folio closed.
- [ ] Each send writes a human-readable `.eml` artifact and a
  machine-readable `.json` artifact containing extracted links.
- [ ] `latest.eml` and `latest.json` always point to the most recent email
  for a recipient.
- [ ] `MockSender` unit tests continue to pass without call-site changes.
- [ ] Production mode refuses filesystem transport unless the explicit
  allow flag is set.
- [ ] Link extraction logic is shared enough that in-memory and
  filesystem-based tests recover URLs the same way.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] Frontend build passes.

## Risks and Mitigations

### MIME Parsing Drift

Risk:
If the filesystem transport writes a different representation than SMTP,
smoke tests may pass while real emails fail.

Mitigation:
Write the exact bytes from `buildMIME(msg)` to `.eml` and keep metadata as
a separate sidecar rather than inventing a new primary message format.

### Production Misconfiguration

Risk:
An operator could accidentally route real production mail to local disk.

Mitigation:
Default to `smtp`, require an explicit selector change, and add a second
production allow flag plus a loud startup warning.

### Brittle Link Recovery

Risk:
If E2E tests must parse quoted-printable MIME directly, they become
fragile and template-sensitive.

Mitigation:
Persist a JSON sidecar with extracted links while also storing the exact
MIME message for realism and debugging.

### Inbox Growth Over Time

Risk:
`/tmp/inbox` can accumulate many artifacts across repeated runs.

Mitigation:
Do not hide cleanup inside server startup. Document manual cleanup so
tests can choose when to preserve or clear artifacts.

### Address Normalization Edge Cases

Risk:
Unsafe or surprising recipient path normalization could create ambiguous
folders.

Mitigation:
Centralize normalization in one helper, keep common email characters
readable, and test unusual addresses explicitly.

## Security Considerations

- Filesystem transport is local-development/testing infrastructure, not a
  general delivery path.
- The transport must be selectable only through explicit config, never as a
  side effect of missing SMTP secrets or mode changes.
- Production mode requires an additional explicit allow flag before
  filesystem transport is accepted.
- Message artifacts contain token-bearing URLs and recipient addresses, so
  docs must state that the inbox directory is local-machine data and should
  not be served publicly or committed.
- Sender path handling must prevent directory traversal; recipient-derived
  paths are normalized before any write.
- Startup and send logs must not print full token URLs or message bodies.

## Dependencies

- Sprint 009: existing email/auth foundation and `email.Sender`
  abstraction.
- Sprint 010: invitation email naming and richer recipient metadata.
- Sprint 018: trip assignment/unassignment email workflows.
- Sprint 019: folio close email remains part of the operational path.
- Sprint 020: folio-close totals continue to reflect pricing override
  snapshots; this sprint must not change rendering behavior.

## Open Questions

1. Should `InboxLinkFor(rootDir, to, needle)` land in Sprint 021 itself, or
   is `latest.json` alone sufficient as the machine contract for E2E
   tests?
2. Should `latest.eml` / `latest.json` be copied files or symlinks? Copying
   is simpler and more portable; symlinks are smaller but less predictable
   on some environments.
3. Do we want a tiny `make inbox-clear` helper in scope, or is
   documentation-only cleanup enough for this sprint?
