# Sprint 001: Rewrite Sprint Tooling from Python to Go

## Overview

The agent-template repo uses a Python script (`ledger.py`) to manage sprint planning and tracking. This sprint rewrites it to Go as a single-file program runnable via `go run`, removes references to "principalis" and other project-specific sources, and renames the tool from "ledger" to "tracker".

All skill files (Claude and Codex) and the sprint README must be updated to invoke the Go tool. The TSV data format is preserved, with a one-time migration from `ledger.tsv` to `tracker.tsv`.

## Use Cases

1. **Sprint lifecycle management**: Add sprints, transition through `planned` -> `in_progress` -> `completed` (or `skipped`), view current/next sprint.
2. **Status overview**: Show counts by status and current/next sprint for quick planning context.
3. **List and filter**: List all sprints, optionally filtered by status via `--status`.
4. **Sync from docs**: Create/update tracker entries by scanning `SPRINT-*.md` file headers.
5. **Skill integration**: Claude/Codex skills invoke `go run docs/sprints/tracker.go` seamlessly.

## Architecture

```
docs/sprints/
  tracker.go      # Single-file Go CLI (all logic in one file)
  tracker.tsv     # TSV datastore (renamed from ledger.tsv)
  README.md       # Updated sprint conventions docs
  drafts/         # Sprint planning artifacts
```

### Core Components (inside tracker.go)

- **SprintEntry**: Struct with id (3-digit zero-padded), title, status, created_at, updated_at. Status validation: `planned|in_progress|completed|skipped`.
- **SprintTracker**: Load/save TSV, sorted by sprint number. UTC timestamps in `2006-01-02T15:04:05Z` format.
- **CLI commands**: stats, current, next, add, start, complete, skip, status, list (with --status flag), sync.
- **Path resolution**: Look for `tracker.tsv` relative to CWD at `docs/sprints/tracker.tsv`. Fallback: if missing but `ledger.tsv` exists at same location, read it and write to `tracker.tsv` (one-time migration).

### Output Format

Status icons for list display:
- `planned` = `" "`
- `in_progress` = `"*"`
- `completed` = `"+"`
- `skipped` = `"-"`

## Implementation Plan

### Phase 1: Write Go tracker (~55%)

**Files:**
- `docs/sprints/tracker.go` - Complete Go rewrite

**Tasks:**
- [ ] Create `package main` with SprintEntry struct (3-digit ID normalization, status validation)
- [ ] Implement TSV load/save with header `sprint_id\ttitle\tstatus\tcreated_at\tupdated_at`, sorted by sprint number
- [ ] Implement UTC timestamp formatting (`2006-01-02T15:04:05Z`)
- [ ] Implement path resolution: prefer `docs/sprints/tracker.tsv`, fallback to `docs/sprints/ledger.tsv` with migration
- [ ] Implement all CLI subcommands: stats, current, next, add, start, complete, skip, status, list (with --status), sync
- [ ] Implement sync: scan `SPRINT-*.md`, extract title from `# Sprint NNN: Title` header
- [ ] Handle errors: missing args, unknown command, invalid status, duplicate sprint
- [ ] No references to "principalis" or external project names
- [ ] Verify with `go vet docs/sprints/tracker.go`

### Phase 2: Migrate data file (~5%)

**Files:**
- `docs/sprints/tracker.tsv` - Renamed from ledger.tsv

**Tasks:**
- [ ] Rename `ledger.tsv` to `tracker.tsv` (preserve content)
- [ ] Verify `go run docs/sprints/tracker.go stats` reads it correctly

### Phase 3: Update skill files and docs (~30%)

**Files:**
- `.claude/commands/megaplan.md`
- `.claude/commands/sprint.md`
- `.codex/skills/megaplan/SKILL.md`
- `.codex/skills/sprint/SKILL.md`
- `docs/sprints/README.md`

**Tasks:**
- [ ] In all 4 skill files: replace `python3 docs/sprints/ledger.py` with `go run docs/sprints/tracker.go`
- [ ] In sprint skills: update validation commands (replace `cargo build`/`cargo test` with appropriate commands or remove)
- [ ] Update `docs/sprints/README.md`: all command examples, file structure, lifecycle steps - Python->Go, ledger->tracker
- [ ] Remove any "principalis" references found in any file

### Phase 4: Cleanup and verify (~10%)

**Tasks:**
- [ ] Delete `docs/sprints/ledger.py`
- [ ] Verify: `find docs -name '*.py'` returns nothing
- [ ] Verify: grep repo for "principalis" returns nothing
- [ ] Verify: grep repo for "ledger.py" returns nothing
- [ ] Verify: grep repo for "python3 docs/sprints" returns nothing
- [ ] Verify: `go run docs/sprints/tracker.go stats` works
- [ ] Verify: `go run docs/sprints/tracker.go sync` works

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/tracker.go` | Create | Go rewrite of sprint management CLI |
| `docs/sprints/tracker.tsv` | Rename | Sprint data (from ledger.tsv) |
| `docs/sprints/ledger.py` | Delete | Replaced by tracker.go |
| `docs/sprints/README.md` | Modify | Update all Python/ledger references to Go/tracker |
| `.claude/commands/megaplan.md` | Modify | Update tool invocation to Go |
| `.claude/commands/sprint.md` | Modify | Update tool invocation and validation commands |
| `.codex/skills/megaplan/SKILL.md` | Modify | Update tool invocation to Go |
| `.codex/skills/sprint/SKILL.md` | Modify | Update tool invocation and validation commands |

## Definition of Done

- [ ] `go run docs/sprints/tracker.go stats` produces valid output
- [ ] All subcommands work: stats, current, next, add, start, complete, skip, status, list, sync
- [ ] `go vet docs/sprints/tracker.go` passes
- [ ] TSV format compatible with existing schema/header
- [ ] No Python files under `docs/`
- [ ] No "principalis" references in the repo
- [ ] No "ledger.py" or "python3 docs/sprints/ledger" references in the repo
- [ ] All 4 skill files reference `go run docs/sprints/tracker.go`
- [ ] `docs/sprints/README.md` reflects Go tracker workflow
- [ ] `tracker.tsv` is the canonical data file

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Go time format differences | Low | Low | Use Go's reference time layout `2006-01-02T15:04:05Z` |
| Path resolution from different CWDs | Medium | Medium | Document that tool is run from repo root; use CWD-relative paths |
| TSV migration edge cases | Low | Medium | Implement one-time fallback read from `ledger.tsv` |
| Missed references in skill files | Low | Medium | Grep verification in Phase 4 catches stragglers |

## Security Considerations

- Standard library only, no external dependencies
- File I/O scoped to `docs/sprints/` directory
- No network access, no user-controlled path traversal
- TSV/markdown inputs treated as data, never executed

## Dependencies

- Go toolchain (already configured in project settings)
- No external Go modules required
