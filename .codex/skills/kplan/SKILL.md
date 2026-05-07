---
name: kplan
description: Multi-agent collaborative sprint planning workflow (orient, intent, draft with Codex, interview, critique with Claude Code, merge).
---

# K Plan: Collaborative Multi-Agent Planning

You are orchestrating a sophisticated planning workflow that produces high-quality sprint documents through competitive ideation, independent critique, and synthesis across Codex and Claude Code.

## Seed Prompt

$ARGUMENTS

## Workflow Overview

This is a **6-phase workflow**:

1. **Orient** - Review project and recent sprints
2. **Intent** - Write concentrated intent document
3. **Draft** - Codex creates the primary draft plan
4. **Interview** - Clarify with the human planner
5. **Critique** - Claude Code critiques Codex's draft
6. **Merge** - Synthesize the draft, critique, and interview refinements into final sprint document

Use TodoWrite to track progress through each phase.

---

## Phase 1: Orient

**Goal**: Understand current project state and recent direction.

### Steps

1. Read `CLAUDE.md` for project conventions.

2. Check sprint ledger status:

   ```bash
   go run docs/sprints/tracker.go stats
   ```

3. Read the **3 highest-numbered sprint documents** to understand recent work:

   ```bash
   ls docs/sprints/SPRINT-*.md | sort | tail -3
   ```

   Read each returned sprint document to understand recent trajectory.

4. Identify relevant code areas for the seed prompt:
   - Search for related modules, types, or patterns
   - Note existing implementations that this plan might extend
   - Note architectural constraints, naming conventions, test conventions, and existing abstractions

### Deliverable

Write a brief **Orientation Summary** covering:

- Current project state relevant to the seed
- Recent sprint themes and direction
- Key modules/files likely involved
- Constraints or patterns to respect

---

## Phase 2: Intent

**Goal**: Create a concentrated intent document that both agents will use.

### Steps

1. Determine the next sprint number:

   ```bash
   ls docs/sprints/SPRINT-*.md | sort | tail -1
   ```

   Extract `NNN` and increment.

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
   - Must follow sprint conventions in docs/sprints/README.md
   - [Any other constraints identified]

   ## Success Criteria

   What would make this sprint successful?

   ## Open Questions

   Questions that the draft and critique should attempt to answer.
   ```

---

## Phase 3: Draft (Codex)

**Goal**: Have Codex create the primary comprehensive sprint plan.

### Execute Codex

Run this command, substituting the actual sprint number for `NNN`:

```bash
codex exec \
  --model gpt-5.5 \
  --sandbox workspace-write \
  "Please read docs/sprints/drafts/SPRINT-NNN-INTENT.md. This is the concentrated intent for our next sprint.

Fully familiarize yourself with our sprint planning style by reading docs/sprints/README.md. Then read CLAUDE.md for project conventions and project goals. Review the recent sprint context referenced in the intent document, and inspect the relevant codebase areas before drafting.

Write a comprehensive sprint plan to docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md.

Follow the standard sprint template from docs/sprints/README.md. Include overview, use cases, architecture, implementation plan, files summary, definition of done, risks and mitigations, security considerations, dependencies, and open questions.

Do not write the final sprint document. Only write the Codex draft."
```

### Wait for Codex to complete

Codex should produce:

```text
docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md
```

### Read the output

Read `docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md`.

---

## Phase 4: Interview

**Goal**: Refine understanding through human dialogue.

### Conduct the interview

Use AskUserQuestion to ask **2-4 targeted questions** covering:

1. **Scope validation**: Does the Codex draft match the planner's intent, or should the sprint expand/narrow?
2. **Priority/trade-offs**: Which aspects are critical versus nice-to-have?
3. **Technical preferences**: Are there strong opinions on architecture, sequencing, naming, or implementation approach?
4. **Sequencing**: Are there external dependencies, ordering constraints, or risks the draft missed?

Note the answers and incorporate refinements in the merge phase.

---

## Phase 5: Critique (Claude Code)

**Goal**: Have Claude Code independently critique Codex's draft.

### Execute Claude Code

Run this command, substituting the actual sprint number for `NNN`:

```bash
claude -p \
  "Please read docs/sprints/drafts/SPRINT-NNN-INTENT.md, docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md, docs/sprints/README.md, and CLAUDE.md.

Your job is to critique Codex's sprint draft, not to rewrite it.

Evaluate the draft for correctness, architectural fit, implementation sequencing, missing files or modules, over-scoping, under-scoping, unclear definition of done, testing gaps, security gaps, dependency risks, and mismatch with project conventions.

Write your critique to docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT-CLAUDE-CRITIQUE.md.

The critique should be specific and actionable. Include:
- Valid strengths worth preserving
- Major concerns
- Missing implementation details
- Suggested changes
- Risks the final merge should address
- Any parts of the Codex draft that should be rejected or simplified

Do not edit the Codex draft. Do not write the final sprint document." \
  --allowedTools "Read,Write,Glob,Grep"
```

### Wait for Claude Code to complete

Claude Code should produce:

```text
docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT-CLAUDE-CRITIQUE.md
```

### Read the output

Read `docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT-CLAUDE-CRITIQUE.md`.

---

## Phase 6: Merge

**Goal**: Synthesize the best ideas into a final sprint document.

### Merge process

1. Analyze Claude Code's critique of Codex's draft:
   - Which criticisms are valid?
   - What did Codex miss?
   - What should be preserved from Codex's draft?
   - What should be simplified, rejected, or resequenced?

2. Compare the draft, critique, and interview answers:
   - Architecture fit
   - Phasing and ordering
   - Risk identification
   - Definition of Done completeness
   - Testing strategy
   - Security considerations
   - Project convention alignment

3. Write merge notes to `docs/sprints/drafts/SPRINT-NNN-MERGE-NOTES.md`:

   ```markdown
   # Sprint NNN Merge Notes

   ## Codex Draft Strengths

   - ...

   ## Claude Code Critique Strengths

   - ...

   ## Valid Critiques Accepted

   - ...

   ## Critiques Rejected or Modified

   - ...

   ## Interview Refinements Applied

   - ...

   ## Final Decisions

   - ...
   ```

4. Write the final sprint document to `docs/sprints/SPRINT-NNN.md`, incorporating:
   - Best ideas from Codex's draft
   - Valid critique from Claude Code
   - Interview refinements
   - Project conventions from CLAUDE.md
   - Sprint conventions from docs/sprints/README.md

5. Update the ledger:

   ```bash
   go run docs/sprints/tracker.go sync
   ```

6. Show the user the final document and ask for approval.

---

## File Structure

After kplan completes, you'll have:

```text
docs/sprints/
├── drafts/
│   ├── SPRINT-NNN-INTENT.md                         # Concentrated intent
│   ├── SPRINT-NNN-CODEX-DRAFT.md                    # Codex primary draft
│   ├── SPRINT-NNN-CODEX-DRAFT-CLAUDE-CRITIQUE.md    # Claude Code critique
│   └── SPRINT-NNN-MERGE-NOTES.md                    # Synthesis notes
└── SPRINT-NNN.md                                    # Final merged sprint
```

---

## Output Checklist

At the end of this workflow, you should have:

- [ ] Orientation summary complete
- [ ] Intent document written (`docs/sprints/drafts/SPRINT-NNN-INTENT.md`)
- [ ] Codex draft written (`docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md`)
- [ ] Interview conducted with 2-4 questions answered
- [ ] Claude Code critique written (`docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT-CLAUDE-CRITIQUE.md`)
- [ ] Merge notes written (`docs/sprints/drafts/SPRINT-NNN-MERGE-NOTES.md`)
- [ ] Final sprint document written (`docs/sprints/SPRINT-NNN.md`)
- [ ] Ledger updated via `go run docs/sprints/tracker.go sync`
- [ ] User approved the final document

---

## Reference

- Sprint conventions: `docs/sprints/README.md`
- Project overview: `CLAUDE.md`
- Recent sprints: `docs/sprints/SPRINT-*.md` using the highest numbers
- Codex draft: `docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT.md`
- Claude Code critique: `docs/sprints/drafts/SPRINT-NNN-CODEX-DRAFT-CLAUDE-CRITIQUE.md`
