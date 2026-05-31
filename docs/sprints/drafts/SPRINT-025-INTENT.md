# Sprint 025 Intent: UI + IA Redesign (keep the sea, replace the rest)

## Seed

> we need to rethink the UI, color schemes and IA layout. It sucks ass. I like the background theme of the scuba diver. The rest is just crap

## Translation

The user wants a deep cosmetic + structural redesign of the admin
app. The body-level sea gradient (the "scuba diver" backdrop from
Sprint 011) is the one thing they like and must stay. Everything
else — color application on cards/forms/tables, sidebar IA,
typography hierarchy, page composition, density — is fair game.

The repo's product surface is already feature-complete enough to
support a redesign without re-litigating product scope. The goal
is to take the existing screens and rebuild their visual + IA
shell, not to add features.

## Context (from Phase 1 orientation)

- **One 1,902-line app.css** is the whole design system. 25 admin
  pages share raw class strings (`admin-page-header`,
  `admin-card`, `admin-grid`, `.chip--*`). No component library
  (`web/src/admin/components/` does not exist).
- **DESIGN.md** is the canonical design-system doc with a
  decisions log. Sprint 011 introduced the sea-blue body
  gradient and reversed an earlier "no blue in palette" rule.
  Working surfaces are warm slate + amber accent; the body-level
  cyan sea is the one decorative element.
- **IA today:** a fixed 220px sidebar with one level of NavLink
  nesting (Overview / Organization > Payments/Pricing / Audit /
  Fleet > Inventory / Trips > Import / Users / Reports). Role-
  filtered (Org Admin vs Cruise Director).
- **Page composition** is samey across reports / forms / ledgers
  — every screen is "title + subtitle + stack of `.admin-card`s".
  No rhythm or scanability that helps a Cruise Director during a
  fast-paced charter.
- The body gradient is the explicit love-it asset; warm slate +
  amber on the working surfaces is what the user is rejecting
  (they read as "generic enterprise tool" rather than "operates
  on a tropical liveaboard").

## Recent Sprint Context

- **SPRINT-022 (Reports):** added analytical reports surface
  across Org Admin + Cruise Director + Guest. No visual system
  changes — just lots of new pages plugged into existing card
  chrome.
- **SPRINT-023 (Onboarding wizard):** added a four-step
  prescriptive onboarding flow with auto-show on first run.
  Cosmetic work was confined to `.onboarding*` classes;
  established no new design primitives.
- **SPRINT-024 (Auto FX rates):** backend-only. Added three-state
  rate chips and an auto-refresh stamp on Payments — same
  `.chip--*` system; no broader visual change.

The pattern: every recent sprint reaches for the existing CSS
toolkit and adds one more bespoke class. The toolkit itself
hasn't been revisited since Sprint 011.

## Relevant Codebase Areas

| Path | Why it matters |
|---|---|
| `web/src/styles/app.css` | The 1,902-line everything-sheet. Will be split or rewritten. |
| `web/src/admin/Shell.tsx` | Sidebar IA, nav structure, brand wordmark — touched directly by IA work. |
| `web/src/admin/pages/*.tsx` (25 files) | Every page using the chrome classes the redesign will replace. |
| `web/src/admin/{AssignDirector,CurrencyPicker,UserMenu,useMe}.tsx` | Misc admin primitives that should move into `components/`. |
| `web/src/pages/*.tsx` | Unauthenticated auth shell pages (Login, Signup, Verify). Sea gradient lives at this layer too. |
| `DESIGN.md` | Canonical design decisions log — must be amended. |
| `docs/decisions/` | Holds ADRs 0001-0003. The right home for a new "UI redesign" ADR. |
| `web/index.html` | Font loading (DM Sans, Geist, General Sans, JetBrains Mono). May shift. |

## Constraints

- **Keep the body-level sea gradient.** This is the one explicit
  asset the user loves; it survives the redesign verbatim
  (gradient value + `body { background-attachment: fixed }`).
- **Do not regress functionality.** Every existing page keeps
  working. No new features. No backend changes (unless something
  in the API genuinely blocks the IA we want — flag and discuss
  rather than ship).
- **Role-aware nav is non-negotiable.** Org Admin and Cruise
  Director see different sidebars today; the redesign preserves
  that — possibly even sharpens it.
- **CLAUDE.md rules apply per commit:** tests pass, gofmt, go
  vet, prettier, and `npm run build` clean.
- **Branch:** work lands as focused commits on `main` (no
  feature branches, no PRs unless asked).
- **No new external CSS framework** (Tailwind, Mantine, MUI,
  etc.) without explicit user approval. The current system is
  raw CSS — staying with raw CSS or moving to CSS modules
  scoped per component are both on the table; a 3rd-party
  design framework is not.

## Success Criteria

1. **Visual:** Opening the app feels coherent with the sea
   gradient — colors on cards, buttons, and chrome read as
   "from the same world" rather than "amber-on-grey laid over a
   blue photo."
2. **IA:** A Cruise Director landing in `/admin` during a
   charter can find Trips → today's manifest → consumption
   ledger in one mental hop. An Org Admin can get to Reports
   without a second-level expand.
3. **Component library:** `web/src/admin/components/` exists,
   contains the primitives used by every page (`Card`, `Page`,
   `Chip`, `Stat`, `Section`, etc.), and pages stop reaching
   for raw class strings.
4. **CSS:** `app.css` is either split into per-component
   stylesheets (CSS modules) or stays one file but is
   measurably smaller and organized into clear sections, not a
   1,902-line accretion.
5. **DESIGN.md is updated.** A new ADR in `docs/decisions/`
   records the rationale for the palette / IA / typography
   shifts so future sprints can stay coherent.
6. **No regressions.** Every page that worked before still
   works; visual diff is intentional, behavior diff is zero.

## Interview Locks (resolved 2026-05-30)

The Phase 4 interview answered the open questions as follows.
These supersede the "open questions" section below; the
competing draft must build to these.

1. **Aesthetic posture: BOLD.** The audience is adventurous
   scuba divers, not enterprise buyers. The "safe" cool-slate-
   plus-coral option is rejected. The Recommended bold pick
   wins on every axis. (Saved as
   `feedback-bold-aesthetic` memory.)
2. **Ship all three palette options behind a live runtime
   toggle.** Themes: `abyss` (dark navy + bioluminescent
   cyan-green), `glass` (translucent cards + backdrop-blur),
   `sunlit` (saturated coral + cyan duotone, light). User
   flips between them in the running app and picks one.
3. **Ship all three IA options behind a live runtime toggle.**
   Layouts: `rail` (⌘K command bar + 56px icon rail),
   `spaces` (three labeled groups Operations/Configuration/
   Insights in a full sidebar), `canvas` (spatial "Today is
   the deck plan" landing).
4. **Ship all three motion options behind a live runtime
   toggle.** Modes: `living` (slow caustic overlay + ripple
   hovers), `minimal` (no ambient motion), `full` (hero video
   on auth + animated counters + slide transitions).
5. **Switcher location.** Floating dock, bottom-right of every
   page. Always visible during the evaluation window; gated
   behind a debug flag so it can be silenced for production.
6. **Scope follows.** All 25 admin pages migrate this sprint;
   `app.css` is removed; the component library uses semantic
   tokens that each theme rebinds. No more "library only" or
   "hero pages only" half-measures — you need every page to
   judge the variants honestly.

## Open Questions (to surface in the interview)

- **Direction of the new palette.** Two natural options:
  (a) lean into the sea — replace warm slate with a cool
  cyan-on-deep-blue surface palette and pick a non-amber CTA
  color, vs.
  (b) keep working surfaces neutral but pick a *different*
  neutral (cool slate / paper / off-white) and a different
  accent so the cards stop fighting the body gradient.
- **IA structure.** Flatten the current nested NavLink tree
  vs. introduce explicit "spaces" (Operations / Configuration
  / Insights) that group today's eight roots?
- **Typography.** Keep the General Sans + DM Sans + Geist +
  JetBrains stack, or consolidate to fewer faces?
- **Density.** "Linear/Stripe density" was the original target;
  is that still right, or do we want a more relaxed,
  inhabited feel given the cyan brand?
- **Scope of this sprint.** All 25 pages in one sprint vs. ship
  the component library + 3-4 hero pages and migrate the
  rest in a follow-up?
- **Component approach.** Pure CSS modules (`Component.module.css`),
  vanilla extract, or a single restructured `app.css` with strict
  section conventions?
