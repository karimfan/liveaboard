# Sprint 025 Claude Draft — Codex Critique

## Valid Strengths Worth Preserving

- The draft correctly centers the interview locks: three palettes, three
  IA layouts, three motion modes, and a bottom-right runtime switcher.
- It treats the switcher as a temporary evaluation tool and names the
  follow-up cleanup sprint explicitly.
- The semantic-token approach is right. Pages should not know whether
  `abyss`, `glass`, or `sunlit` is active.
- It correctly calls out that `canvas` is the heaviest layout and that
  glass-theme contrast is a real risk.
- URL deep-linking for a combination (`?triptych=...`) is a useful
  addition for review sessions, as long as values are allowlisted.
- The security section correctly validates URL state before reflecting
  it into DOM attributes.

## Major Concerns

### 1. The sea gradient is not actually preserved verbatim

The intent is explicit: keep the Sprint 011 body-level sea gradient
verbatim, including the gradient value and fixed attachment. Claude's
draft weakens that in several places:

- Architecture diagram: "`abyss` darkens the bottom stops; sea identity
  preserved".
- Phase 1 task: "`--gradient-sea` matches the Sprint 011 hex codes
  exactly under all three themes (sub-stop colors may shift...)".
- Definition of Done: "Body-level sea gradient is preserved verbatim
  under all three themes (some sub-stops may shift...)".

"Verbatim" and "sub-stops may shift" conflict. The final sprint should
state that the exact `--gradient-sea` value from `tokens.css` remains
unchanged and themes only rebind working-surface tokens above it.

### 2. The draft backslides from the locked "all 25 pages, all variants" scope

The interview lock says all 25 admin pages migrate this sprint because
the variants need to be judged honestly. Claude's draft repeatedly
narrows evaluation to a subset:

- Use Case 1 says the user evaluates "the four hero pages and two
  representative deep-pages".
- Phase 4 migrates only four hero pages first.
- Phase 5 migrates the remaining pages, but the DoD only requires all
  27 combinations on the four hero pages.
- DoD: "All 25 admin pages render under at least one combo, and under
  all 3 themes when in the `spaces` layout".

That is not enough to judge 27 combinations. The final DoD should
require every admin page to render correctly under every palette,
layout, and motion mode, with a pragmatic manual smoke matrix for key
routes plus build/type checks for the rest. If the sprint is too large,
the plan should say so, not encode a weaker acceptance bar.

### 3. `canvas` relies on data the existing Overview API does not appear to provide

Phase 3 says:

- "`canvas` layout: SVG deck-plan renders the org's boats from the
  existing Overview API. Active trips are glowing arcs. Click a boat →
  trip dashboard."

Current `Overview.tsx` consumes setup steps, trips needing attention,
and Cruise Director trip lists. It does not show a boat inventory list
or enough spatial data to draw a deck-plan/fleet graph. Building a real
"boat-plan canvas" may require extra API data or a separate fleet fetch,
which violates the "no backend changes unless genuinely blocked" shape
unless the final plan names the frontend data source precisely.

Recommended change: define `canvas` as a spatial landing composed from
existing available Overview/Cruise Director trip data first. If it must
render boats, explicitly fetch an existing Fleet endpoint and document
the fallback when boat data is unavailable.

### 4. The shell plan risks duplicating role-gating and onboarding behavior

The draft proposes `RailShell`, `SpacesShell`, and `CanvasShell` as
separate shell variants, with `Shell.tsx` becoming a slim switch:

```tsx
switch (layout) {
  case 'rail': return <RailShell />;
  case 'spaces': return <SpacesShell />;
  case 'canvas': return <CanvasShell />;
}
```

`Shell.tsx` currently owns more than markup: `useMe`, sign-out state,
`UserMenu`, role-filtered nav, `RequireAdmin`, and the Sprint 023
onboarding auto-show hook. Three full shell implementations make it
easy for those behaviors to drift.

Final plan should keep one `AdminShell` behavioral owner and swap only
layout renderers/nav presentations underneath it. Nav data should be
computed once, role-filtered once, then rendered as rail/spaces/canvas.

### 5. The debug gate is build-time, not clearly runtime-silenceable

The switcher is gated by `VITE_TOPSIDE_SWITCHER=true`. That is simple,
but the intent says "gated behind a debug flag so it can be silenced for
production" and the repo already has `web/src/lib/devFlags.ts` for
frontend-visible development affordances.

A Vite env var is acceptable only if the final plan is explicit that
silencing requires a rebuild and that production defaults false. Better
fit with the repo is to extend `useDevFlags()` or use a small wrapper
that supports both a server debug flag and a Vite fallback.

### 6. The draft introduces test/tooling requirements the repo does not have

Phase 2 and DoD require:

- `web/src/admin/components/__tests__/*.tsx`
- "vitest props + snapshot tests"
- "`vitest` clean"

But `web/package.json` currently has no `test` script and no Vitest,
Testing Library, jsdom, or snapshot setup. Adding that tooling is not
wrong by itself, but it is a real dependency/tooling sprint inside an
already large UI migration. The draft lists "No external blockers" and
does not include the package changes or setup work.

Recommended change: either explicitly add Vitest setup to scope and
files summary, or replace this with the existing reliable checks:
`npm run build`, TypeScript, focused manual route smoke, and maybe
lightweight unit tests only if the test harness already exists.

### 7. "No `className` escape hatches" conflicts with the migration plan

Phase 2 says no component accepts `style` or `className` escape hatches.
Phase 5 says each page may have "a small page-local `.module.css` for
layout-only concerns."

Those two requirements conflict unless every primitive exposes a rich
layout API from day one. For a 25-page migration, some controlled
`className` slots or `variant`/`density` props are likely necessary.
The final plan should forbid raw visual color/style escape hatches, not
all class composition.

### 8. Font replacement is unnecessary churn and not justified by the locks

Phase 5 requires:

- "Remove DM Sans / General Sans / Geist (display) loads from
  `index.html` — Inter (variable) + Geist Mono only."

The intent notes typography is fair game, but the resolved interview
locks do not pick a font direction. DESIGN.md currently names General
Sans, DM Sans, Geist, and JetBrains Mono. Replacing the type system may
be a valid design decision, but the draft gives no rationale beyond
cleanup and adds a new external font dependency. This is especially
questionable while also saying no new external dependencies beyond
fonts/video.

Recommended change: keep font consolidation optional, or specify the
typographic rationale and verification criteria. Do not make font
replacement a required Phase 5 task unless the final merge decides it.

### 9. Renaming Overview to Today is avoidable route/file churn

Phase 3 creates `Today.tsx`, deletes `Overview.tsx`, and changes the
index route. A label change from "Overview" to "Today" can be valuable,
but a file-level rename is not required and increases diff noise during
a broad migration. It also has to preserve the Sprint 023 onboarding
auto-show condition that checks `location.pathname === "/admin"`.

Recommended change: keep the route and file stable unless implementation
proves the rename is worth it. Render "Today" as IA copy if desired.

## Missing or Weak Implementation Details

- The final sprint needs a precise list of all admin pages counted in
  the "25 pages" migration. Claude says "other 21 pages" after four
  hero pages, but the repo contains nested pages and non-page admin
  primitives that should be included deliberately.
- The draft says `full` motion mounts an underwater video, but the repo
  has no cited video asset. The Open Question allows fallback to still
  image, but the DoD should not require a video unless an asset source is
  chosen.
- The `CommandBar` scope is too high for this sprint if it becomes a
  real `⌘K` command registry "registered by pages." A route launcher
  based on shell nav data is sufficient for evaluating the `rail` and
  `canvas` IA options.
- "Manual visual review by the user" appears in the DoD. Sprint DoD
  should require the implementer to make review possible and perform
  smoke checks; user selection belongs to Sprint 026. Requiring the user
  to review before Sprint 025 can be marked complete blurs ownership.
- The draft adds a `CLAUDE.md` rule. That may be useful, but changing
  repo operating instructions is heavier than updating `DESIGN.md` and
  an ADR. If retained, keep it very narrow and avoid making page-local
  CSS impossible.
- The risk mitigation says if `canvas` stalls, ship only `rail` +
  `spaces`. That directly violates the resolved interview lock to ship
  all three IA options. A mitigation can reduce fidelity, but it should
  not drop the third layout from Sprint 025.

## Suggested Changes for the Final Sprint

- Preserve the Sprint 011 `--gradient-sea` value exactly. Remove all
  "sub-stops may shift" language.
- Raise the acceptance bar so all admin pages are migrated and work
  under all 27 combinations, with key-route manual smoke across
  breakpoints.
- Keep one behavioral `AdminShell`; split nav/layout renderers below it
  instead of three independent shell owners.
- Define `canvas` using existing frontend data sources, or explicitly
  name the existing endpoint it will call. Do not assume Overview has
  boat data.
- Use semantic tokens and component primitives, but allow controlled
  layout class slots or variants for migration practicality.
- Do not require Vitest unless the sprint also installs and configures
  it. Otherwise rely on `npm run build` plus manual route smoke.
- Treat font consolidation and underwater video as optional design
  refinements unless the final merge chooses them explicitly.
- Gate the switcher through the repo's dev-flag pattern or document the
  Vite env var's production behavior clearly.
- Keep `/admin` and `Overview.tsx` stable unless the rename has a clear
  implementation payoff.

## Parts to Reject or Simplify

- Reject any theme-specific mutation of the body sea gradient.
- Reject the DoD that tests all 27 combinations only on four hero pages.
- Reject dropping `canvas` if it stalls; instead reduce the fidelity of
  `canvas` while preserving it as a selectable IA mode.
- Reject mandatory Vitest/snapshot coverage unless test tooling is
  added as explicit scope.
- Reject a hard "no `className`" component API rule; replace it with
  "no page-local color literals or raw legacy chrome classes."
- Simplify `CommandBar` to a nav-backed route launcher for Sprint 025.
- Simplify shell variants so role gating, onboarding auto-show, sign-out,
  and user menu state stay centralized.

## Risks the Merge Should Address

- The sprint is already large. Adding a video asset pipeline, font
  migration, command registry, and test framework can crowd out the
  locked deliverable: honest 27-combo evaluation across the live app.
- A partial DoD would let the sprint finish with only showcase pages
  polished, repeating the "hero pages only" half-measure the interview
  explicitly rejected.
- Three independent shell components can create subtle authorization or
  navigation regressions even if the backend still protects data.
- If `glass` and `abyss` are allowed to alter the underlying sea
  gradient, Sprint 025 will violate the one design element the user
  explicitly said they like.
