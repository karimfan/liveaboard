# Sprint Management

This directory contains sprint planning documents for development.

## Quick Reference

```bash
# View sprint stats
go run docs/sprints/tracker.go stats

# List all sprints
go run docs/sprints/tracker.go list

# List by status
go run docs/sprints/tracker.go list --status completed

# Sync tracker from .md files (run after creating new sprints)
go run docs/sprints/tracker.go sync

# Start a sprint
go run docs/sprints/tracker.go start 018

# Complete a sprint
go run docs/sprints/tracker.go complete 018

# Add a new sprint manually
go run docs/sprints/tracker.go add 019 "Sprint Title"
```

## File Structure

```
docs/sprints/
├── README.md           # This file
├── tracker.tsv         # Sprint tracking database (TSV format)
├── tracker.go          # CLI tool for sprint management
├── SPRINT-001.md       # Sprint documents (zero-padded 3 digits)
├── SPRINT-002.md
└── ...
```

## Creating a New Sprint

1. **Determine the next sprint number**:
   ```bash
   ls docs/sprints/SPRINT-*.md | tail -1
   ```

2. **Create the sprint document**:
   ```bash
   # File: docs/sprints/SPRINT-NNN.md
   ```

3. **Use the standard template** (see below)

4. **Sync the tracker**:
   ```bash
   go run docs/sprints/tracker.go sync
   ```

## Sprint Document Template

```markdown
# Sprint NNN: Title

## Overview

Brief description of the sprint goals and motivation.

## Use Cases

1. **Use case name**: Description
2. ...

## Architecture

Diagrams, component descriptions, data flow.

## Implementation Plan

### Phase 1: Name (~X%)

**Files:**
- `path/to/file.go` - Description

**Tasks:**
- [ ] Task 1
- [ ] Task 2

### Phase 2: ...

## API Endpoints (if applicable)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/path`  | POST   | Description |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `path/to/file` | Create/Modify | Description |

## Definition of Done

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Tests pass
- [ ] No compiler warnings

## Security Considerations

- Item 1
- Item 2

## Dependencies

- Sprint NNN (if any)
- External requirements

## References

- [Link](url)
```

## Sprint Statuses

| Status | Meaning |
|--------|---------|
| `planned` | Sprint is defined but not started |
| `in_progress` | Actively being worked on |
| `completed` | All Definition of Done items met |
| `skipped` | Decided not to implement |

## Conventions

### Naming
- Files: `SPRINT-NNN.md` (zero-padded 3 digits)
- Title format: `# Sprint NNN: Short Descriptive Title`

### Content
- **Overview**: 1-2 paragraphs explaining the "why"
- **Use Cases**: Concrete scenarios this sprint enables
- **Implementation Plan**: Break into phases with percentage estimates
- **Definition of Done**: Checkboxes for acceptance criteria
- **Files Summary**: Table of files to create/modify

### Lifecycle
1. Create sprint doc with status `planned`
2. Run `tracker.go sync` to add to tracker
3. When starting: `tracker.go start NNN`
4. When done: `tracker.go complete NNN`

### For AI Assistants

When asked to create a sprint:
1. Check the highest existing sprint number
2. Create `SPRINT-{N+1}.md` using the template
3. Run `go run docs/sprints/tracker.go sync`
4. Update tracker status if starting immediately

When completing work:
1. Update the sprint document with results
2. Run `go run docs/sprints/tracker.go complete NNN`
