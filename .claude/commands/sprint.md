---
description: Begin and complete the next incomplete sprint (tracker, Definition of Done, Documentation Manifest verification, build/test, commit/push).
---

## Task

Begin the next sprint. Follow these steps:

1. **Find the next incomplete sprint**
   - Run `go run docs/sprints/tracker.go stats` to see sprint status
   - Identify the lowest-numbered sprint that is NOT completed
   - Read that sprint document: `docs/sprints/SPRINT-NNN.md`
   - Locate its `## Documentation Manifest` section (added by the kplan skill). If the sprint doc has no Manifest, STOP and ask the user — the sprint was planned before the manifest requirement and needs one before implementation can mark complete.

2. **Mark sprint in progress**
   - Run `go run docs/sprints/tracker.go start NNN`

3. **Complete the sprint**
   - Work through ALL items in the Definition of Done
   - Implement all required functionality per the sprint document
   - Run `go vet ./...` to validate
   - Fix any build or test failures
   - Ensure all validation passes per repo standards

4. **Verify the Documentation Manifest landed**

   This is the load-bearing gate that prevents "code shipped but ADRs forgotten." Before staging the completion commit:

   - Re-read the `## Documentation Manifest` section of `docs/sprints/SPRINT-NNN.md`.
   - For each file under **New ADRs**: verify the file exists (`ls docs/adr/NNNN-*.md`).
   - For each file under **Amended ADRs**: verify the file has a new section referencing this sprint (e.g., `grep "Sprint NNN amendment" docs/adr/NNNN-*.md`).
   - For each file under **Cross-cutting docs** (`current_status.md`, `docs/product-architecture.md`, `local_setup.md`, etc.): verify the file was modified for this sprint (`git diff main -- <file>` shows non-trivial changes, OR `grep "Sprint NNN" <file>` finds the new entry).
   - If ANY required item is missing, do NOT proceed to commit. Either land the missing docs change OR explicitly negotiate with the user to remove it from the manifest (which means amending the sprint doc itself).

   Items under **Skipped (with reasoning)** are pre-approved — confirm the reasoning still holds; don't silently expand the skip set during implementation.

5. **Commit and push**
   - Stage all changes (code + docs)
   - Create a meaningful commit message summarizing the sprint work, naming each manifest item that landed
   - Push to the remote repository

6. **Mark sprint completed**
   - Run `go run docs/sprints/tracker.go complete NNN`
   - Commit the ledger update
   - Push the completion

## Why the Manifest gate matters

Sprints that ship code but skip ADR amendments leave the architectural record stale; future readers (and future LLM sessions) read the older ADR and assume nothing changed. The Manifest is the explicit contract between planning and implementation: items there are pre-negotiated documentation work that lands with the code, not a soft "nice to have" buried in a Phase 6 task list. The pre-commit verification step ensures the contract is honored.

If a manifest item turns out to be wrong during implementation (e.g., the sprint discovered the amendment isn't needed, or a new ADR is needed that wasn't planned), the right move is to UPDATE the sprint doc's Manifest section in the same commit — making the divergence explicit + auditable — not to silently skip the item.
