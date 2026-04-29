# Sprint 001 Intent: Rewrite Sprint Tooling from Python to Go

## Seed

Rewrite all Python files under the docs folder to Go. Remove any references to "principalis" or other sources. Rename files from "ledger" into something similar. Ensure that the megaplan skill for both Claude and Codex, which is in this repo, is fixed to invoke the correct Go files. Files should be executable via `go run <filename>`.

## Context

- Fresh agent-template repo with no CLAUDE.md and no existing sprint documents
- The only Python file is `docs/sprints/ledger.py` - a sprint ledger manager CLI (~407 lines)
- Four skill files reference `python3 docs/sprints/ledger.py`: two for Claude (`.claude/commands/{megaplan,sprint}.md`) and two for Codex (`.codex/skills/{megaplan,sprint}/SKILL.md`)
- The ledger.py docstring references "ai-principalis" which must be removed
- Go tooling is already configured in the project settings (go build, go run, go test, etc.)
- `docs/sprints/README.doc` is a binary file that may also contain references

## Recent Sprint Context

No prior sprints exist. This is the first sprint for this repo.

## Relevant Codebase Areas

| File | Role |
|------|------|
| `docs/sprints/ledger.py` | Python CLI for sprint ledger management (stats, add, start, complete, sync, list, etc.) |
| `docs/sprints/ledger.tsv` | TSV data file (header only, no entries) |
| `docs/sprints/README.md` | Sprint conventions reference (references Python tools, needs updating) |
| `.claude/commands/megaplan.md` | Claude megaplan skill - references `python3 docs/sprints/ledger.py` |
| `.claude/commands/sprint.md` | Claude sprint skill - references `python3 docs/sprints/ledger.py` |
| `.codex/skills/megaplan/SKILL.md` | Codex megaplan skill - references `python3 docs/sprints/ledger.py` |
| `.codex/skills/sprint/SKILL.md` | Codex sprint skill - references `python3 docs/sprints/ledger.py` |

## Constraints

- Must preserve all existing CLI functionality (stats, current, next, add, start, complete, skip, status, list, sync)
- Must remain a single-file Go program runnable via `go run`
- TSV format must remain compatible with existing `ledger.tsv`
- No references to "principalis" or other project-specific sources in the new code
- All four skill files must be updated to use `go run` instead of `python3`

## Success Criteria

1. `go run docs/sprints/tracker.go stats` (or similar name) works identically to `python3 docs/sprints/ledger.py stats`
2. All CLI subcommands produce equivalent output
3. No Python files remain under docs/
4. No references to "principalis" anywhere in the repo
5. All skill files correctly reference the Go file
6. `ledger.tsv` continues to work as the data store

## Interview Answers

1. **Naming**: `tracker.go` and `tracker.tsv` confirmed
2. **Go module**: No go.mod needed - standalone script via `go run`
3. **README**: `README.doc` has been renamed to `README.md` - it references Python tools and must also be updated
