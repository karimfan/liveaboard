# Codex Critique of Claude Draft (SPRINT-001-CLAUDE-DRAFT.md)

## Overall Assessment

Claude’s draft captures the core intent well: rewrite `docs/sprints/ledger.py` to a standalone Go CLI, rename “ledger” → “tracker”, preserve TSV format + subcommands, and update all four skill files to use `go run`. The phased plan is sensible and scoped appropriately for a first sprint in a fresh repo.

That said, there are a few ambiguities and missing details that are likely to cause rework or subtle compatibility breaks—especially around the `ledger.tsv` ↔ `tracker.tsv` transition, output parity expectations, and how the Go tool locates files when run from different working directories.

## Strengths

- Clear articulation of goals (Go rewrite, reference removal, rename, skill updates).
- Correct identification of the four skill files that must change.
- Good emphasis on preserving CLI subcommands and output format.
- Includes verification steps (repo-wide grep for `principalis` and `ledger.py`).

## Gaps / Issues

### 1) `ledger.tsv` vs `tracker.tsv` is underspecified (and conflicts with the intent doc)

- Claude’s draft says to rename `ledger.tsv` → `tracker.tsv` and remove `ledger.tsv`.
- The intent document’s “Success Criteria” includes: “`ledger.tsv` continues to work as the data store”, while the “Interview Answers” say “`tracker.go` and `tracker.tsv` confirmed.”

This needs an explicit decision and migration story. If we hard-rename to `tracker.tsv`, existing checkouts (or any tooling/docs still expecting `ledger.tsv`) break unless we provide a compatibility path.

**Recommendation:** Make `tracker.tsv` canonical, but implement backward compatibility:
- On startup, if `tracker.tsv` missing but `ledger.tsv` exists: migrate (copy/rename) or read `ledger.tsv` and write `tracker.tsv`.
- Optionally keep `ledger.tsv` as a deprecated fallback for one sprint (or forever) to satisfy “continues to work”.

### 2) File path resolution in Go needs an explicit design

The Python tool tries to locate the TSV relative to the script directory or CWD. Go won’t have `__file__` semantics; it’s very easy for `go run docs/sprints/tracker.go ...` to be executed from different working directories.

**Recommendation:** Define a deterministic lookup order (document it in `docs/sprints/README.md`):
- Prefer `docs/sprints/tracker.tsv` relative to repo root/CWD.
- Fall back to `docs/sprints/ledger.tsv` for legacy.
- If neither exists, error with an actionable message.

(If we want true “script-relative” behavior, we’d need to use `os.Executable()` which is awkward with `go run` because the binary is placed in a temp dir.)

### 3) Output parity is stated but not operationalized

Claude says “Match output format exactly with Python version” but doesn’t list what “exactly” means. In practice, the following details matter and should be called out so we don’t ship subtle diffs:
- Status icons mapping in list output (`planned=" "`, `in_progress="*"`, `completed="+"`, `skipped="-"`).
- `stats` header formatting, ordering of statuses, and blank lines.
- Verbose fields ordering for `current`/`next`.
- Error messages + usage text for missing args / unknown command.

**Recommendation:** Add a small parity checklist in the sprint doc (or in tasks) and validate each command against a small fixture TSV.

### 4) Skill docs update scope should include non-CLI assumptions

The Codex sprint skill currently instructs running `cargo build`/`cargo test` (even though this repo is Go-focused per the intent doc). Claude’s draft only mentions swapping the `python3` invocation, not the broader validation commands.

**Recommendation:** In the “Update skill files” phase, explicitly update:
- `python3 docs/sprints/ledger.py` → `go run docs/sprints/tracker.go`
- repo validation commands (`cargo ...`) → `go test ./...` (or whatever is correct for this repo)

### 5) “No Python files under docs/” should be verified against all `docs/**`

Claude plans to delete `docs/sprints/ledger.py`, which is correct, but the intent explicitly says “Rewrite all Python files under the docs folder to Go.” Today that’s only one file, but the plan should include a verification step (e.g., search for `*.py` under `docs/`) to prevent missing a future addition.

### 6) Missing mention of `README.doc` / binary artifacts

The intent doc mentions a `docs/sprints/README.doc` binary that “may contain references,” and an interview answer claims it “has been renamed to `README.md`.” The repo currently has `docs/sprints/README.md` but no `README.doc`.

**Recommendation:** Add an explicit “discover and handle” task:
- Search for `README.doc` (and any other `.doc`/binary artifacts) and scan for forbidden terms (`principalis`).
- Don’t assume the rename happened; verify.

## Concrete Improvements to Apply to the Sprint Plan

- Add a decision section for TSV naming + backward compatibility policy.
- Add explicit path lookup rules for TSV and sprint docs in Go.
- Add a short output-parity checklist (what must match) and a small sample TSV fixture for manual verification.
- Expand “Update skill files” tasks to adjust validation commands (not just tool invocation).
- Add verification tasks: `find docs -name '*.py'`, and a repo-wide search for forbidden terms and `ledger.py` references, including binary artifacts.

## Suggested “Open Questions” (tighter, action-driving)

1. Should `tracker.go` transparently migrate `ledger.tsv` → `tracker.tsv`, or should we keep `ledger.tsv` indefinitely as a supported alternate filename?
2. What is the expected default repo validation for skills (`go test ./...`, `go vet`, or something else)?
3. Do we require byte-for-byte identical output, or “semantically equivalent” output (same info, minor whitespace differences acceptable)?

