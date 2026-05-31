# Sprint 025 Merge Notes

## Claude Draft Strengths (worth preserving)

- Concrete token table per palette (hex codes, hover variants,
  shadow tokens). Lets implementation start from real values.
- Architecture diagram showing the layered system (switcher
  → data attrs → tokens/layouts/motion → waterline).
- URL deep-linking with `?triptych=` query — useful when the
  user wants to share a specific combo with the reviewer.
- Risks section enumerates the realistic failure modes
  (canvas heaviness, glass contrast, neon-accent misread).
- Documentation Manifest section is filled in honestly with
  Skipped reasoning, not just a list of files.

## Codex Draft Strengths (worth adopting)

- **Clean directory boundary** at `web/src/admin/design/` for
  the runtime mode provider — better than my admin-root
  `useTopsideState.ts`.
- **CSS layout** `base.css / tokens.css / themes.css /
  motion.css / admin.css` separates rebindings from
  foundations more cleanly than my version.
- **Stronger framing of the switcher as a Sprint 026 cleanup
  point.** Names the temporary nature up front.
- **Validates persisted localStorage values** before
  reflecting them into DOM attributes. Subtle correctness win.
- **Uses the repo's existing `devFlags` pattern** for the
  switcher gate instead of inventing a new Vite env var.
- **Doesn't promise vitest** when the repo has no test
  harness for web. Honest about scope.

## Valid Critiques Accepted

1. **Sea gradient must be truly verbatim.** Drop every
   "sub-stops may shift" qualifier. Theme rebinds only the
   surface tokens above the gradient; the gradient itself is
   immutable.

2. **DoD scope backslid from the lock.** Interview said all 25
   pages migrate. Final DoD: every admin page renders under
   every (palette × layout) combination. Motion modes are
   CSS-only and verifiable independently. Manual smoke covers
   key routes; build/type checks cover the rest. No "hero
   pages only" half-measure.

3. **Canvas needs a real data source.** The existing Overview
   API does not return boat-fleet data. Either use an existing
   Fleet/boats endpoint explicitly, or scope `canvas` to a
   spatial landing composed from the data Overview already
   ships (active trips + setup state). I'll choose the latter
   — no backend changes, and the spatial idiom still reads.

4. **One behavioral shell, three nav presentations.** Keep
   `AdminShell` as the single owner of `useMe`, role filter,
   sign-out, onboarding auto-show, and `RequireAdmin`. Below
   it, swap **nav renderers** (`RailNav` / `SpacesNav` /
   `CanvasNav`) based on `data-layout`. Avoid three full
   parallel `Shell` components that can drift.

5. **Adopt `useDevFlags()` for the switcher gate.** Matches
   the Sprint 021 pattern (`dev/inbox` viewer). Cleaner than
   a new Vite env var.

6. **Drop the vitest promise.** Repo has no `web/` test
   harness today. Adding it is a separate concern that would
   crowd the migration. Rely on TypeScript + `npm run build`
   + manual route smoke (which the implementer performs).

7. **"No `className`" rule is wrong.** Correct rule: **no
   page-local color literals or raw legacy chrome classes.**
   Components may accept `className` for layout composition.

8. **Typography is out of scope.** Interview didn't lock font
   direction. Keep the existing General Sans + DM Sans + Geist
   + JetBrains Mono stack. A future sprint can consolidate if
   the chosen palette demands it.

9. **Don't rename Overview → Today as a file.** The Sprint 023
   onboarding auto-show hook checks `location.pathname ===
   "/admin"`; a route rename ripples. The sidebar label says
   "Today" but the file + route stay `Overview.tsx` / `/admin`.

10. **`full` motion's underwater video falls back gracefully.**
    No DoD requirement that a video asset exists. `full` mode
    works with a still image if no video is provided; if a
    user-supplied video lands, it slots in.

11. **Drop "manual user review" from the DoD.** Implementer
    smokes the routes. User selection is Sprint 026.

12. **Risk mitigation can't drop canvas.** Interview locked all
    three layouts. Replace the "if canvas stalls, ship 2"
    mitigation with "if canvas stalls, reduce its fidelity
    (drop animated arcs, keep static deck plan) but ship it."

## Critiques Rejected (with reasoning)

- **Codex suggests the `CommandBar` is too heavy.** Partially
  accept: I'll keep the bar but it consumes the same nav
  data structure the sidebar uses, not a separate registry.
  That's basically a route launcher, which is what Codex
  recommends. Rename the prop `Command[]` → `NavItem[]` so
  it's obvious there's one source of truth.

- **Codex calls the CLAUDE.md addition "heavier than needed."**
  Reject. CLAUDE.md is read at every session start; without
  this pointer, future sprints will reach for raw classes
  again. Keep it. One line.

## Interview Refinements Applied

- All 3 palettes shipped (`abyss / glass / sunlit`) with
  semantic-token rebinds.
- All 3 layouts shipped (`rail / spaces / canvas`) via nav
  renderers under one shell.
- All 3 motion modes shipped (`living / minimal / full`) via
  CSS data-attribute rules.
- Floating switcher dock, bottom-right, on every page.
- Persists to localStorage with URL override.
- Gated via the existing `useDevFlags()` pattern.

## Final Decisions

| Decision | Resolution |
|---|---|
| Sea gradient under each theme | Untouched; tokens only rebind surfaces above it. |
| Page scope | All 25 admin pages. Every page works under every (palette × layout) combo. |
| Shell architecture | One `AdminShell` owns behavior; nav renderers swap by `data-layout`. |
| `canvas` data source | Composed from existing Overview API only — no new endpoints. |
| Component library tests | Skipped this sprint (no harness). TypeScript + manual smoke. |
| Component `className` policy | Allowed for layout composition; forbidden for color/chrome. |
| Typography | Unchanged from current DESIGN.md. |
| File renames | None. Sidebar label "Today" → `Overview.tsx` file stays. |
| Switcher gate | `useDevFlags()` pattern, like `dev/inbox`. |
| Underwater video | Optional asset; `full` works with still image fallback. |
| DoD review responsibility | Implementer smokes routes; user picks the winner in Sprint 026. |
| Cleanup sprint | Documented explicitly in the ADR. |
