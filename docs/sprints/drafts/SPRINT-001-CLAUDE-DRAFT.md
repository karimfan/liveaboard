# Sprint 001: Rewrite Sprint Tooling from Python to Go

## Overview

The agent-template repo currently uses a Python script (`ledger.py`) to manage sprint planning and tracking. This sprint rewrites it to Go for consistency with the project's Go-first tooling approach, removes references to "principalis" and other external sources, and renames the tool from "ledger" to "tracker" for clearer intent.

All four skill files (two Claude, two Codex) that invoke the Python script must be updated to call the Go equivalent via `go run`. The TSV data format and all CLI subcommands must be preserved so existing workflows continue unmodified.

## Use Cases

1. **Sprint management via CLI**: Developers and agents run `go run docs/sprints/tracker.go stats` to check sprint status, start/complete sprints, and sync from markdown files.
2. **Skill invocation**: Claude's `/megaplan` and `/sprint` commands, and Codex's equivalent skills, invoke the tracker tool as part of their workflows.
3. **Ledger persistence**: The TSV file remains the source of truth for sprint state, readable by both humans and tools.

## Architecture

```
docs/sprints/
  tracker.go      <-- New: Go rewrite of ledger.py (standalone, no go.mod needed)
  tracker.tsv     <-- Renamed from ledger.tsv (same TSV format)
  README.md       <-- Updated: python3 -> go run, ledger -> tracker
  drafts/         <-- Sprint planning artifacts

.claude/commands/
  megaplan.md     <-- Updated: python3 -> go run, ledger -> tracker
  sprint.md       <-- Updated: python3 -> go run, ledger -> tracker

.codex/skills/
  megaplan/SKILL.md  <-- Updated: python3 -> go run, ledger -> tracker
  sprint/SKILL.md    <-- Updated: python3 -> go run, ledger -> tracker
```

### Data Flow

```
User/Agent -> `go run tracker.go <cmd>` -> reads/writes tracker.tsv
                                        -> reads SPRINT-*.md (for sync)
```

## Implementation Plan

### Phase 1: Write Go tracker (~60%)

**Files:**
- `docs/sprints/tracker.go` - Complete Go rewrite of ledger.py

**Tasks:**
- [ ] Create `tracker.go` with `package main`
- [ ] Implement SprintEntry struct with TSV parse/serialize
- [ ] Implement SprintTracker (load, save, add, update_status, sync_from_docs, etc.)
- [ ] Implement all CLI subcommands: stats, current, next, add, start, complete, skip, status, list, sync
- [ ] Match output format exactly with Python version
- [ ] Handle `--status` flag for list command
- [ ] Use `tracker.tsv` as default filename (not `ledger.tsv`)
- [ ] Remove all references to "principalis"
- [ ] Test with `go run docs/sprints/tracker.go stats`

### Phase 2: Rename data file (~5%)

**Files:**
- `docs/sprints/tracker.tsv` - Renamed from ledger.tsv

**Tasks:**
- [ ] Rename `ledger.tsv` to `tracker.tsv`
- [ ] Verify Go tool reads it correctly

### Phase 3: Update skill files (~25%)

**Files:**
- `.claude/commands/megaplan.md`
- `.claude/commands/sprint.md`
- `.codex/skills/megaplan/SKILL.md`
- `.codex/skills/sprint/SKILL.md`

**Tasks:**
- [ ] Replace all `python3 docs/sprints/ledger.py` with `go run docs/sprints/tracker.go`
- [ ] Update any other references to "ledger" in skill file text
- [ ] Remove any references to "principalis" if present
- [ ] Update `docs/sprints/README.md` - replace all Python invocations with Go equivalents, rename ledger references to tracker

### Phase 4: Remove Python file and verify (~10%)

**Files:**
- `docs/sprints/ledger.py` - Delete

**Tasks:**
- [ ] Delete `ledger.py`
- [ ] Verify no remaining Python files under docs/
- [ ] Grep entire repo for "principalis" - confirm zero hits
- [ ] Grep entire repo for "ledger.py" - confirm zero hits
- [ ] Run `go run docs/sprints/tracker.go stats` - confirm working
- [ ] Run `go run docs/sprints/tracker.go sync` - confirm working

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/tracker.go` | Create | Go rewrite of sprint management CLI |
| `docs/sprints/tracker.tsv` | Rename | Sprint data (from ledger.tsv) |
| `docs/sprints/ledger.py` | Delete | Replaced by tracker.go |
| `docs/sprints/README.md` | Modify | Update Python->Go references, ledger->tracker |
| `.claude/commands/megaplan.md` | Modify | Update tool invocation to Go |
| `.claude/commands/sprint.md` | Modify | Update tool invocation to Go |
| `.codex/skills/megaplan/SKILL.md` | Modify | Update tool invocation to Go |
| `.codex/skills/sprint/SKILL.md` | Modify | Update tool invocation to Go |

## Definition of Done

- [ ] `go run docs/sprints/tracker.go stats` produces valid output
- [ ] All subcommands (stats, current, next, add, start, complete, skip, status, list, sync) work
- [ ] `go vet docs/sprints/tracker.go` passes
- [ ] No Python files under docs/
- [ ] No "principalis" references in the repo
- [ ] All 4 skill files reference `go run docs/sprints/tracker.go`
- [ ] `tracker.tsv` is the data file (ledger.tsv removed)
- [ ] Sync from SPRINT-*.md files works correctly

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Go standard library differences in time formatting | Low | Low | Use time.Now().UTC().Format() with matching layout |
| TSV parsing edge cases (tabs in titles) | Low | Medium | Use bufio.Scanner with explicit tab splitting |
| README.md references may be missed | Low | Low | Grep for remaining python3/ledger references after updates |
| No go.mod means no external dependencies | Low | Low | Standard library is sufficient for this tool |

## Security Considerations

- No external dependencies - standard library only
- File I/O limited to docs/sprints/ directory
- No network access
- No user input beyond CLI args

## Dependencies

- Go toolchain installed (already confirmed in settings)
- No external Go modules needed

## Open Questions

1. Should `tracker.tsv` be renamed to something else, or is "tracker" the right name?
2. Should we create a go.mod file at the repo root for future Go files?
