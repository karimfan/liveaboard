# Sprint 001 Merge Notes

## Claude Draft Strengths
- Clean phased structure with clear percentage estimates
- Good verification steps (grep for principalis, ledger.py)
- Correctly identified all 6 files needing modification (including README.md)

## Codex Draft Strengths
- Excellent detail on path resolution challenge (Go has no `__file__` equivalent with `go run`)
- Legacy `ledger.tsv` fallback/migration idea is practical
- Output parity checklist idea - specifying exact icon mappings and format details
- Caught that sprint skill references `cargo build`/`cargo test` which should be updated

## Valid Critiques Accepted
1. **TSV migration**: Should handle `ledger.tsv` -> `tracker.tsv` gracefully. Will implement: if `tracker.tsv` missing but `ledger.tsv` exists, read from `ledger.tsv` and write to `tracker.tsv`.
2. **Path resolution**: Document that the tool expects to be run from repo root (`go run docs/sprints/tracker.go`). Use CWD-relative paths.
3. **Output parity specifics**: Include explicit icon mapping and format details in the plan.
4. **Skill validation commands**: Update `cargo build`/`cargo test` references in sprint skills to appropriate Go or generic commands.
5. **Verify no .py files remain**: Add `find docs -name '*.py'` verification step.

## Critiques Rejected (with reasoning)
1. **Byte-for-byte output parity**: Not needed. Semantically equivalent is fine - this is a fresh repo with no existing automation depending on exact output.
2. **Keep `ledger.tsv` indefinitely**: No. One-time migration is sufficient. After migration, only `tracker.tsv` exists.

## Interview Refinements Applied
- Naming confirmed: `tracker.go` / `tracker.tsv`
- No go.mod
- README.md needs updating (was binary README.doc, now markdown with Python references)

## Final Decisions
- Tool name: `tracker.go`, data file: `tracker.tsv`
- One-time migration: read `ledger.tsv` if `tracker.tsv` doesn't exist, then write to `tracker.tsv`
- Path resolution: CWD-relative, document that tool runs from repo root
- Sprint skill validation: change `cargo build`/`cargo test` to `go vet` or remove (this is a template repo)
- Output: semantically equivalent, not byte-for-byte identical
