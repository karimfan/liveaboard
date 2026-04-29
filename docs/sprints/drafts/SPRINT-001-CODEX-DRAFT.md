# Sprint 001: Rewrite Sprint Tooling from Python to Go

## Overview

This sprint replaces the Python-based sprint ledger CLI (`docs/sprints/ledger.py`) with an equivalent single-file Go program runnable via `go run`, while preserving existing behavior and TSV compatibility.

In addition to the language migration, we’ll remove repo-specific references (notably “principalis”), rename the tool and data file from “ledger” to “tracker”, and update all skill/docs references so both Claude and Codex workflows invoke the Go implementation.

## Use Cases

1. **Track sprint lifecycle**: Add a sprint, set `planned` → `in_progress` → `completed` (or `skipped`), and view the current/next sprint.
2. **Status overview for planning**: Show counts by status and the current/next sprint for quick planning context.
3. **List and filter**: List all sprints, optionally filtered by status.
4. **Sync from sprint docs**: Create/update ledger entries by scanning `docs/sprints/SPRINT-*.md` headers.
5. **Skills remain functional**: Claude/Codex skills that previously invoked `python3 docs/sprints/ledger.py ...` now invoke `go run docs/sprints/tracker.go ...` with equivalent results.

## Architecture

Single-file Go CLI:

```
docs/sprints/
  tracker.go      # CLI entrypoint + all logic (single-file program)
  tracker.tsv     # TSV datastore (canonical)
  ledger.tsv      # Optional legacy fallback (read-only compatibility)
```

Core components inside `tracker.go`:

- **Model**: `SprintEntry` (id/title/status/timestamps), status validation, formatting for list output.
- **Store**: `SprintTracker` loads/saves TSV; stable sort by sprint number; timestamp generation in UTC (`YYYY-MM-DDTHH:MM:SSZ`).
- **Commands**: `stats`, `current`, `next`, `add`, `start`, `complete`, `skip`, `status`, `list --status`, `sync`.
- **Doc sync**: Scan `docs/sprints/SPRINT-*.md`, parse filename sprint id, parse markdown header `# Sprint NNN: Title` (multiline regex equivalent), update/create entries.
- **Path resolution**: Locate TSV relative to script location isn’t available in Go the same way as `__file__`; implement:
  1) prefer `docs/sprints/tracker.tsv` relative to current working dir
  2) else fallback to `docs/sprints/ledger.tsv` for legacy support
  3) else error with actionable message

Output parity targets:
- `stats` header lines and counts match Python output.
- `list` uses the same status icons mapping: `planned=" "`, `in_progress="*"`, `completed="+"`, `skipped="-"`.
- `current`/`next` verbose output fields match names and ordering where feasible.

## Implementation Plan

### Phase 1: Go CLI parity (~55%)

**Files:**
- `docs/sprints/tracker.go` - New Go implementation (single-file program).

**Tasks:**
- [ ] Implement `SprintEntry` normalization (3-digit IDs) and status validation (`planned|in_progress|completed|skipped`).
- [ ] Implement TSV load/save with header `sprint_id\ttitle\tstatus\tcreated_at\tupdated_at` and stable sorting by sprint number.
- [ ] Implement timestamp formatting in UTC (`...Z`) and ensure updates only touch `updated_at`.
- [ ] Implement CLI arg parsing and subcommands: `stats`, `current`, `next`, `add`, `start`, `complete`, `skip`, `status`, `list [--status X]`, `sync`.
- [ ] Implement `sync` to scan `docs/sprints/SPRINT-*.md` and extract title from `# Sprint NNN: Title` when present.
- [ ] Document/handle error cases equivalently (missing args, unknown command, invalid status, duplicate sprint).

### Phase 2: Rename + migration (~25%)

**Files:**
- `docs/sprints/tracker.tsv` - New canonical datastore (TSV).
- `docs/sprints/ledger.tsv` - Legacy datastore (kept only if required for compatibility).
- `docs/sprints/ledger.py` - Removed.

**Tasks:**
- [ ] Create `docs/sprints/tracker.tsv` with the same header as `ledger.tsv`.
- [ ] Decide on migration strategy:
  - Preferred: if `ledger.tsv` exists and `tracker.tsv` does not, migrate by renaming/copying and continue with `tracker.tsv`.
  - Keep `ledger.tsv` as fallback read source only (optional) so existing checkouts don’t break immediately.
- [ ] Delete `docs/sprints/ledger.py` and ensure no Python files remain under `docs/`.

### Phase 3: Update skills + docs (~15%)

**Files:**
- `docs/sprints/README.md` - Update all command examples and references from Python to Go and from `ledger` to `tracker`.
- `.claude/commands/megaplan.md` - Replace `python3 docs/sprints/ledger.py ...` with `go run docs/sprints/tracker.go ...`.
- `.claude/commands/sprint.md` - Replace `python3 ...` with `go run ...`; update validation commands to Go equivalents (or repo-appropriate defaults).
- `.codex/skills/megaplan/SKILL.md` - Replace `python3 ...` with `go run ...` and update guidance accordingly.
- `.codex/skills/sprint/SKILL.md` - Replace `python3 ...` with `go run ...`; update validation commands to Go equivalents (or repo-appropriate defaults).

**Tasks:**
- [ ] Update all four skill files to invoke `go run docs/sprints/tracker.go ...`.
- [ ] Update `docs/sprints/README.md` quick reference and lifecycle steps to use Go tooling and new filenames.

### Phase 4: Repo hygiene & reference removal (~5%)

**Files:**
- (Various) - Remove prohibited references.

**Tasks:**
- [ ] Remove “ai-principalis” / “principalis” references (including docstrings and any docs/binaries that contain it).
- [ ] Confirm there are no remaining references to `ledger.py` or `python3 docs/sprints/ledger.py` in the repo.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/tracker.go` | Create | Go rewrite of the sprint ledger CLI (single-file program). |
| `docs/sprints/tracker.tsv` | Create | Canonical TSV datastore for the tracker. |
| `docs/sprints/ledger.tsv` | Modify/Keep | Optional legacy fallback or migration source. |
| `docs/sprints/ledger.py` | Delete | Remove Python implementation under `docs/`. |
| `docs/sprints/README.md` | Modify | Update sprint tooling docs to Go + tracker naming. |
| `.claude/commands/megaplan.md` | Modify | Update megaplan command references to Go tracker. |
| `.claude/commands/sprint.md` | Modify | Update sprint workflow command references to Go tracker. |
| `.codex/skills/megaplan/SKILL.md` | Modify | Update Codex megaplan skill references to Go tracker. |
| `.codex/skills/sprint/SKILL.md` | Modify | Update Codex sprint skill references to Go tracker. |

## Definition of Done

- [ ] `go run docs/sprints/tracker.go stats` works and produces equivalent output to the prior Python version.
- [ ] All supported subcommands exist and behave equivalently: `stats`, `current`, `next`, `add`, `start`, `complete`, `skip`, `status`, `list`, `sync`.
- [ ] TSV remains compatible with the existing schema/header and preserves ordering and timestamps correctly.
- [ ] No Python files remain under `docs/`.
- [ ] No references to “principalis” (or “ai-principalis”) exist anywhere in the repo.
- [ ] All four skill files reference `go run docs/sprints/tracker.go ...` (no `python3 docs/sprints/ledger.py` remains).
- [ ] `docs/sprints/README.md` reflects the new Go tracker workflow and filenames.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Output mismatch breaks skill expectations | Medium | Medium | Create a small parity checklist and compare outputs for each command on empty TSV + small sample TSV. |
| TSV migration confusion (`ledger.tsv` vs `tracker.tsv`) | Medium | Medium | Implement deterministic path priority + one-time migration behavior; document clearly in `README.md`. |
| Title parsing differences in `sync` | Low | Medium | Mirror the exact header pattern (`# Sprint NNN: Title`); keep conservative fallback title when missing. |
| Hidden “principalis” reference in non-text files | Low | High | Add a repo-wide search step and explicitly handle any binary/doc artifacts discovered. |

## Security Considerations

- Treat TSV and markdown inputs as untrusted: avoid executing content; only parse.
- Avoid writing outside `docs/sprints/` and ensure file paths are fixed (no user-controlled path traversal).

## Dependencies

- Go toolchain available (assumed configured per repo context).
- No external Go dependencies; standard library only to preserve single-file `go run` simplicity.

## Open Questions

1. Should `docs/sprints/ledger.tsv` be removed entirely, or kept as a legacy fallback for one sprint to avoid breaking old checkouts?
2. Should we add a small Go test (`tracker_test.go`) for TSV parsing and `sync` title extraction, or keep the implementation strictly single-file with manual parity checks only?
3. For the sprint skill validation step, what’s the desired default verification command (e.g., `go test ./...` vs. a repo-specific build/test command)?

