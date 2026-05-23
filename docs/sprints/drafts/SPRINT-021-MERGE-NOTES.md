# Sprint 021 Merge Notes

## Claude Draft Strengths

- Centralized transport selection in `cmd/server/main.go` — keeps the
  switch single-point and the rest of the app blind to transports.
- SMTP as the default in every mode.
- Per-recipient inbox directory with `e2e+signup@example.invalid`-style
  human-legible slugs.
- `.eml` reuses `buildMIME(msg)` so the on-disk artifact matches the
  exact bytes SMTP would transmit.
- Phased plan with reasonable percentage estimates and a concrete file
  list.
- Risks table called out the production-misconfiguration and
  filename-collision concerns up front.

## Codex Draft Strengths

- **Shared link extraction.** Codex correctly noted that adding a
  separate `FilesystemLinkFor` while leaving `MockSender.LinkFor`
  alone leaves two independent URL parsers that can drift. Better:
  factor extraction into one helper both transports use.
- **Atomic-file `latest.*` instead of symlinks.** Codex flagged that
  symlinks behave inconsistently across filesystems, and that making
  `latest.*` part of the Definition of Done while implementing it as
  best-effort symlinks is internally inconsistent. Copy/rewrite via
  `os.Rename` is portable and enforceable.
- **Trimmed `.json` sidecar.** Keep metadata + links; don't duplicate
  the full text/html bodies that are already preserved in the `.eml`.
  Less PII, smaller storage, single source of truth for the bodies.
- **Explicit coverage plan for all eight email kinds.** The original
  Claude draft promised "all eight kinds work" in the DoD but only
  scheduled one e2e test. The final plan should say *how* the full
  matrix is exercised.
- **Honest scope accounting.** Codex called out that the overview
  understated the file list — the final doc should match its own
  scope statement.
- **Corrected security wording.** Token URLs land in BOTH the `.eml`
  and the `.json`; the Claude draft incorrectly said only the `.json`
  carried them.

## Valid Critiques Accepted

1. **Share link extraction** — refactor `MockSender.LinkFor` to use a
   shared `ExtractLinks(msg Message) []string` helper; the filesystem
   sender's sidecar uses the same helper.
2. **`latest.eml` / `latest.json` as atomic-rewrite regular files**,
   not symlinks.
3. **Sidecar shape**: `from`, `to`, `subject`, `sent_at`, `eml_path`,
   `links`. No body duplication.
4. **All eight kinds must be proven** — add a render-and-send matrix
   test that drives each `email.Kind` through `FilesystemSender` and
   asserts the artifact is on disk with at least one extracted link
   (or none, in the rare case the template carries no URL).
5. **Honest overview**: the final doc lists every file explicitly.
6. **Security wording**: token URLs are present in both `.eml` and
   `.json`; operators choosing filesystem mode accept this.

## Critiques Rejected (with reasoning)

1. **Codex: "Trim scripts and Makefile work out of the sprint."**
   Rejected. The user explicitly answered in the interview that they
   want **all three** of: `make inbox` / `make inbox-clear`,
   `scripts/inbox.sh`, **and** a dev-mode HTTP inbox viewer. These
   are first-class deliverables for this sprint. Codex didn't see the
   interview answers and its critique was reasonable absent that
   context — but the user's preference is explicit.

2. **Codex: "Production allow-flag is the right answer."** Partially
   rejected — the structure is even simpler than Codex proposed. The
   user chose **forbid filesystem transport in production entirely**.
   So we drop `LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION` from both
   drafts. Two env vars total: `LIVEABOARD_EMAIL_TRANSPORT` and
   `LIVEABOARD_EMAIL_FILESYSTEM_DIR`.

## Interview Refinements Applied

| Question | Answer | Final-doc impact |
|---|---|---|
| File format | `.eml` + `.json` sidecar | Matches both drafts; keeps. |
| Production safety | Forbid filesystem transport in production entirely | Drop the third env var. Loader rejects `filesystem` outright in `ModeProduction`. |
| Dev ergonomics | `make inbox` + `make inbox-clear` + `scripts/inbox.sh` + dev-only HTTP inbox viewer | Add Phase 5 to cover the HTTP viewer; keep CLI in Phase 4. |
| E2E test scope | One representative end-to-end test | Keep; combine with per-kind matrix unit-style test for coverage. |

## Final Decisions

- **Two env vars**: `LIVEABOARD_EMAIL_TRANSPORT` (`smtp`|`filesystem`,
  default `smtp`) and `LIVEABOARD_EMAIL_FILESYSTEM_DIR` (default
  `/tmp/inbox`).
- **Production hard-rejects `filesystem`** at config-load time.
- **`.eml` + `.json` sidecar**, both written atomically via
  temp-file + rename. `latest.eml` / `latest.json` are atomically
  rewritten regular files, not symlinks.
- **One shared `ExtractLinks` helper** in `internal/email`;
  `MockSender.LinkFor` refactored to use it; the sidecar's `links`
  array is its output.
- **Per-recipient subdirectories** with the slug preserving
  `@`, `.`, `+`, `_`, `-`, lowercasing the address, and replacing
  any other character with `_`.
- **Filename**: UTC timestamp + 6-char random suffix; no kind in the
  name (Message doesn't carry kind).
- **All eight email kinds covered** by a render-and-send matrix test
  in `internal/email/filesystem_test.go`, plus one HTTP-level e2e
  test (`internal/httpapi/email_filesystem_e2e_test.go`).
- **Dev ergonomics shipped in scope**:
  - `make inbox` and `make inbox-clear`
  - `scripts/inbox.sh <recipient>` prints `latest.json`
  - dev-mode HTTP inbox viewer at `/dev/inbox` and
    `/dev/inbox/<recipient>` — only mounted when
    `cfg.Mode == ModeDev` and `cfg.EmailTransport == "filesystem"`.
- **No frontend changes**.
- **No changes to existing callers** in `internal/auth/*.go` or
  `internal/httpapi/*.go`.

## Phasing for the Final Doc

1. Phase 1 — Shared link extractor + Filesystem sender core (~30%)
2. Phase 2 — Config + Wiring (~20%)
3. Phase 3 — Dev ergonomics: CLI + Make targets + HTTP dev viewer (~20%)
4. Phase 4 — Tests: per-kind matrix + one HTTP e2e (~20%)
5. Phase 5 — Docs + verification (~10%)
