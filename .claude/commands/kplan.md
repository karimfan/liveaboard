---
description: Multi-agent collaborative planning - draft, interview, compete with Codex, merge
---

# K Plan: Collaborative Multi-Agent Planning

You are orchestrating a sophisticated planning workflow that produces high-quality sprint documents through competitive ideation and synthesis with Codex.

## Seed Prompt

$ARGUMENTS

## Workflow Overview

This is a **6-phase workflow**:
1. **Orient** - Review project and recent sprints
2. **Intent** - Write concentrated intent document
3. **Draft** - Create your draft plan
4. **Interview** - Clarify with the human planner
5. **Compete** - Codex creates competing draft + critiques yours
6. **Merge** - Synthesize best ideas into final sprint document

Use TodoWrite to track progress through each phase.

---

## Phase 1: Orient

**Goal**: Understand current project state and recent direction.

### Steps:
1. Read `CLAUDE.md` for project conventions
2. Check sprint ledger status:
   ```bash
   go run docs/sprints/tracker.go stats
   ```
3. Read the **3 highest-numbered sprint documents** to understand recent work:
   - Use `ls docs/sprints/SPRINT-*.md | tail -3` to find them
   - Read each one to understand recent trajectory
4. Identify relevant code areas for the seed prompt:
   - Search for related modules, types, or patterns
   - Note existing implementations that this plan might extend

### Deliverable:
Write a brief **Orientation Summary** (3-5 bullet points) covering:
- Current project state relevant to the seed
- Recent sprint themes/direction
- Key modules/files likely involved
- Constraints or patterns to respect

---

## Phase 2: Intent

**Goal**: Create a concentrated intent document that both agents will use.

### Steps:

1. Determine the next sprint number:
   ```bash
   ls docs/sprints/SPRINT-*.md | tail -1
   ```
   Extract NNN and increment.

2. Create the drafts directory if needed:
   ```bash
   mkdir -p docs/sprints/drafts
   ```

3. Write the intent document to `docs/sprints/drafts/SPRINT-NNN-INTENT.md`:

```markdown
# Sprint NNN Intent: [Title]

## Seed

[The original $ARGUMENTS prompt]

## Context

[Your orientation summary from Phase 1]

## Recent Sprint Context

[Brief summaries of the 3 recent sprints you reviewed]

## Relevant Codebase Areas

[Key modules, files, patterns identified during orientation]

## Constraints

- Must follow project conventions in CLAUDE.md
- Must integrate with existing architecture
- [Any other constraints identified]

## Success Criteria

What would make this sprint successful?

## Open Questions

Questions that the drafts should attempt to answer.
```

---

## Phase 3: Draft (Claude)

**Goal**: Create your comprehensive draft plan.

### Write to: `docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT.md`

Follow the standard sprint template from `docs/sprints/README.md`:

```markdown
# Sprint NNN: [Title]

## Overview

2-3 paragraphs on the "why" and high-level approach.

## Use Cases

1. **Use case name**: Description
2. ...

## Architecture

Diagrams (ASCII art), component descriptions, data flow.

## Implementation Plan

### Phase 1: [Name] (~X%)

**Files:**
- `path/to/file.rs` - Description

**Tasks:**
- [ ] Task 1
- [ ] Task 2

### Phase 2: ...

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `path/to/file` | Create/Modify | Description |

## Definition of Done

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Tests pass
- [ ] No compiler warnings

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| ... | ... | ... | ... |

## Security Considerations

- Item 1
- Item 2

## Dependencies

- Sprint NNN (if any)
- External requirements

## Open Questions

Uncertainties needing resolution.
```

---

## Phase 4: Interview

**Goal**: Refine understanding through human dialogue.

### Conduct the interview:

Use AskUserQuestion to ask **2-4 targeted questions** covering:

1. **Scope validation**: "Does this scope match your intent, or should we expand/narrow?"
2. **Priority/trade-offs**: "Which aspects are most critical vs. nice-to-have?"
3. **Technical preferences**: Any strong opinions on approach?
4. **Sequencing**: Any external dependencies or ordering constraints?

Note the answers - incorporate refinements in the merge phase.

---

## Phase 5: Compete (Codex)

**Goal**: Get Codex's independent draft and critique of your draft.

### Execute Codex:

Run this command (substitute the actual sprint number for NNN):

```bash
codex --model gpt-5.4 --full-auto exec "Please read docs/sprints/drafts/SPRINT-NNN-INTENT.md - this is a concentrated intent for our next sprint. Fully familiarize yourself with our sprint planning style (see docs/sprints/README.md) and project structure (see CLAUDE.md) and project goals. Then I want you to draft docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md. Only AFTER your draft is complete, I want you to read Claude's draft at docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT.md and write docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT-CODEX-CRITIQUE.md"
```

### Wait for Codex to complete.

Codex will produce:
- `docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md` - Its independent draft
- `docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT-CODEX-CRITIQUE.md` - Its critique of your draft

### Read the outputs:

Once Codex completes, read both files:
1. `docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md`
2. `docs/sprints/drafts/SPRINT-NNN-CLAUDE-DRAFT-CODEX-CRITIQUE.md`

---

## Phase 6: Merge

**Goal**: Synthesize the best ideas into a final sprint document.

### Merge process:

1. **Analyze Codex's critique** of your draft:
   - Which criticisms are valid?
   - What did you miss?
   - What should you defend?

2. **Compare the two drafts**:
   - Architecture approach differences
   - Phasing/ordering differences
   - Risk identification gaps
   - Definition of Done completeness

3. **Document the synthesis**:

   Write to `docs/sprints/drafts/SPRINT-NNN-MERGE-NOTES.md`:
   ```markdown
   # Sprint NNN Merge Notes

   ## Claude Draft Strengths
   - ...

   ## Codex Draft Strengths
   - ...

   ## Valid Critiques Accepted
   - ...

   ## Critiques Rejected (with reasoning)
   - ...

   ## Interview Refinements Applied
   - ...

   ## Final Decisions
   - ...
   ```

4. **Write the final sprint document**:

   Create `docs/sprints/SPRINT-NNN.md` incorporating:
   - Best ideas from both drafts
   - Responses to valid critiques
   - Interview refinements

5. **Update the ledger**:
   ```bash
   go run docs/sprints/tracker.go sync
   ```

6. **Show the user** the final document and ask for approval.

---

## File Structure

After megaplan completes, you'll have:

```
docs/sprints/
├── drafts/
│   ├── SPRINT-NNN-INTENT.md                    # Concentrated intent (Phase 2)
│   ├── SPRINT-NNN-CLAUDE-DRAFT.md              # Your draft (Phase 3)
│   ├── SPRINT-NNN-CODEX-DRAFT.md               # Codex draft (Phase 5)
│   ├── SPRINT-NNN-CLAUDE-DRAFT-CODEX-CRITIQUE.md  # Codex critique (Phase 5)
│   └── SPRINT-NNN-MERGE-NOTES.md               # Synthesis notes (Phase 6)
└── SPRINT-NNN.md                               # Final merged sprint
```

---

## Output Checklist

At the end of this workflow, you should have:
- [ ] Orientation summary complete
- [ ] Intent document written (`drafts/SPRINT-NNN-INTENT.md`)
- [ ] Claude draft written (`drafts/SPRINT-NNN-CLAUDE-DRAFT.md`)
- [ ] Interview conducted (2-4 questions answered)
- [ ] Codex executed and completed
- [ ] Codex draft received (`drafts/SPRINT-NNN-CODEX-DRAFT.md`)
- [ ] Codex critique received (`drafts/SPRINT-NNN-CLAUDE-DRAFT-CODEX-CRITIQUE.md`)
- [ ] Merge notes written (`drafts/SPRINT-NNN-MERGE-NOTES.md`)
- [ ] Final sprint document written (`SPRINT-NNN.md`)
- [ ] Ledger updated via `go run docs/sprints/tracker.go sync`
- [ ] User approved the final document

---

## Reference

- Sprint conventions: `docs/sprints/README.md`
- Project overview: `CLAUDE.md`
- Recent sprints: `docs/sprints/SPRINT-*.md` (highest numbers)
