# Sprint 021 Claude Draft — Codex Critique

Critique of `SPRINT-021-CLAUDE-DRAFT.md` against:
- `SPRINT-021-INTENT.md`
- project conventions in `CLAUDE.md`
- sprint template guidance in `docs/sprints/README.md`
- current email/config wiring in `internal/email/*`,
  `internal/config/config.go`, and `cmd/server/main.go`

## Overall Assessment

Claude's draft is broadly on the right track. It keeps the switch point in
`cmd/server/main.go`, preserves SMTP as the default, proposes a
filesystem-backed `email.Sender`, and takes production safety seriously.
The `.eml` plus `.json` artifact split is a defensible answer to the
intent's human-readable plus machine-readable requirement.

The main problems are not with direction but with scope control and a few
implementation details that are either internally inconsistent or likely to
create avoidable rework. The biggest issues are:

1. the plan duplicates link-extraction logic instead of making it shared;
2. the `latest.*` pointer is specified as a symlink-only best-effort path
   while the Definition of Done treats it as guaranteed behavior;
3. the sprint is expanded with shell helpers, Make targets, and doc churn
   that the intent did not require and that are not necessary to ship the
   transport itself.

## Valid Strengths Worth Preserving

1. **Centralized transport selection in `cmd/server/main.go`.** This is
   the correct fit for the current codebase. The existing server already
   constructs one sender there and passes only the interface downstream.

2. **SMTP remains the default.** The draft preserves the most important
   safety property from the intent: production behavior does not change
   unless an operator explicitly opts in.

3. **Production guardrail with a second explicit key.** Requiring
   `LIVEABOARD_EMAIL_FILESYSTEM_ALLOW_PRODUCTION=true` is a good answer to
   the intent's safety question.

4. **Reuse of `buildMIME(msg)` for `.eml` artifacts.** This keeps the
   filesystem transport realistic and aligned with the actual SMTP payload
   shape rather than inventing an alternate primary message format.

5. **Per-recipient inbox directories.** This is the right operational
   choice for smoke tests and manual inspection.

6. **No caller churn in auth or HTTP handlers.** The draft correctly keeps
   the change transport-level rather than threading transport details
   through feature code.

## Major Concerns

### 1. Link Extraction Is Still Duplicated Instead of Shared

The intent explicitly calls out `MockSender.LinkFor(to, needle)` as the
existing parsing precedent and says the filesystem inbox should be
parseable the same way. Claude's draft adds a new
`FilesystemLinkFor(inboxDir, to, needle)` helper, but then explicitly says
`MockSender` is not changed.

That leaves two independent link-recovery implementations:

- in-memory parsing in `MockSender.LinkFor`
- on-disk JSON lookup in `FilesystemLinkFor`

This is a correctness risk. The contract we care about is not the helper
name; it is that the same message yields the same recoverable URL through
both test surfaces. The plan should factor URL extraction into a shared
helper and have both transports rely on it.

**Recommendation:** Refactor `MockSender.LinkFor` to reuse shared link
extraction logic, and make the filesystem sidecar derive from the same
helper.

### 2. `latest.*` Is Underspecified and Conflicts With the DoD

The draft makes `latest.eml` and `latest.json` symlinks, then says symlink
creation is "best-effort" and may be skipped if unsupported. But the
Definition of Done requires that `<recipient>/latest.eml` resolves to the
most recent message.

Those two statements conflict. Either:

- `latest.*` is a required contract, in which case the implementation
  should use a portable guaranteed mechanism such as copy/write, or
- `latest.*` is an optional optimization, in which case it should not be
  part of the DoD.

Given the intent's emphasis on easy smoke scripts, the safer answer is to
make `latest.*` real files updated atomically rather than symlinks.

### 3. The Draft Over-Scopes the Sprint With Unnecessary Tooling

The intent described a transport, config, and testability seam. Claude's
draft adds all of the following:

- `scripts/inbox.sh`
- `make inbox`
- `make inbox-clear`
- `.env.example` edits
- possible `CLAUDE.md` updates

None of that is required to satisfy the success criteria. It increases the
surface area of the sprint and creates review noise outside the core email
path. The current repo already supports straightforward local shell usage;
documenting `cat`, `jq`, or `rm -rf` is likely enough.

**Recommendation:** Keep CLI helpers and Make targets out of Sprint 021
unless the planner explicitly wants them. Focus the sprint on transport,
config, tests, and docs.

### 4. The Draft's Own Summary Understates the Real Scope

The Overview says:

- "two new config keys"
- "one new file in `internal/email`"
- "documentation"

But the draft later defines:

- three config keys
- a new helper API
- a new sender test file
- an end-to-end test file
- docs
- scripts
- Makefile changes
- `.env.example` changes

This mismatch matters because it hides the actual implementation size. The
final sprint doc should be explicit about the real scope and then trim it
if needed.

### 5. The `.json` Sidecar Stores More Than the Intent Needs

Claude's JSON example includes `text_body` and `html_body` in full. That
is probably unnecessary for the sprint's goals:

- the `.eml` file already preserves the full message
- the success criteria only require links to be recoverable
- duplicating whole bodies increases storage and PII footprint

This is not a blocker, but it is extra surface without clear benefit.

**Recommendation:** Keep the sidecar minimal: sender metadata, timestamp,
artifact path, and extracted links. Let `.eml` remain the full-fidelity
body artifact.

### 6. The Security Section Contains One Incorrect Claim

The draft says:

> "the `.json` sidecar is the only on-disk artifact containing them."

That is false if the `.eml` file contains the rendered bodies, because the
token URLs will also be present in the plain-text and/or HTML MIME parts.

This should be corrected because it affects how the final sprint frames the
security tradeoff. Filesystem mode writes token-bearing links to disk in
both artifacts.

### 7. Verification Plan Does Not Fully Match the DoD

The Definition of Done says every one of the eight email kinds lands on
disk and that existing `MockSender` tests continue unchanged. The concrete
test plan, however, only guarantees:

- unit tests for the sender itself
- one representative end-to-end test

That is not enough on its own to prove all eight email kinds work through
the filesystem transport. The final sprint should explicitly state how that
coverage will be achieved, whether by:

- shared helper tests over representative rendered messages,
- one targeted assertion per email-producing flow, or
- a small matrix of render-plus-send tests.

## Suggested Changes

1. **Share link extraction logic.** Add a reusable helper in
   `internal/email` and make both `MockSender.LinkFor` and filesystem
   metadata generation use it.

2. **Replace symlink-only `latest.*` with atomically rewritten files.**
   This keeps the smoke-test pointer portable and makes the DoD
   enforceable.

3. **Trim scripts and Makefile work out of the sprint.** Keep the docs,
   but remove `scripts/inbox.sh`, `make inbox`, and `make inbox-clear`
   unless the user asks for them.

4. **Shrink the `.json` contract.** Store metadata plus links, not full
   duplicate message bodies.

5. **Make the coverage plan explicit for all eight email kinds.** Add
   tests or tasks that prove the transport works across the whole existing
   email surface, not just signup.

6. **Fix the security wording.** State plainly that filesystem mode writes
   token-bearing links to disk in the `.eml` artifact and also exposes them
   in the `.json` sidecar for automation.

7. **Keep the scope accounting honest.** Update the overview/phase summary
   so the plan's stated size matches the actual file and task list.

## Risks the Final Merge Should Address

### Contract Drift Between Test Surfaces

If `MockSender.LinkFor` and `FilesystemLinkFor` evolve separately, the app
can pass unit tests while failing end-to-end automation. Shared extraction
logic is the clean fix.

### Portable "Latest" Pointer Behavior

If `latest.*` depends on symlink support, smoke scripts become
environment-sensitive. Since the draft treats the pointer as part of the
acceptance contract, it should be implemented in a way that does not vary
by host behavior.

### Scope Creep on a Small Infra Sprint

Helper scripts and Make targets are easy to add but easy to regret. They
pull review attention away from the transport itself and increase the
chance that the sprint ships half-finished because too many secondary tasks
were bundled in.

## Parts That Should Be Rejected or Simplified

### Reject: `scripts/inbox.sh` and `Makefile` Targets in the Base Sprint

These are convenience layers, not core transport behavior. The intent did
not require them, and the sprint already has enough real work in config,
sender implementation, testing, and docs.

### Simplify: `.json` Sidecar Body Duplication

Keep the sidecar focused on the automation contract:

- `from`
- `to`
- `subject`
- `sent_at`
- `eml_path`
- `links`

Anything more should need a specific reason.
