# Sprint 025: Runtime UI / IA Redesign Matrix

## Overview

Sprint 025 replaces the current admin chrome and page styling with a
runtime evaluation system: three palettes, three IA layouts, and three
motion modes can be combined live from a floating bottom-right dock.
The goal is not to pick a final winner yet. The goal is to make all 27
combinations honest enough in the running app that the next sprint can
choose from real product use rather than static mockups.

The one visual asset that stays fixed is the Sprint 011 body-level sea
gradient, including `background-attachment: fixed`. Everything above
that gradient changes: cards, tables, buttons, form fields, nav chrome,
typography hierarchy, density, motion, and page composition. The
audience is adventurous scuba divers and liveaboard operators, so the
default posture is bold, saturated, and maritime rather than safe
enterprise slate.

This sprint is frontend-only except for any existing test/build hooks.
No backend endpoints or product capabilities change. The redesign must
preserve Org Admin and Cruise Director route access, existing forms,
existing data fetching, and existing user workflows.

## Use Cases

1. **Live design evaluation.** The user opens the app, uses a
   bottom-right switcher, and flips between `abyss`, `glass`, and
   `sunlit`; `rail`, `spaces`, and `canvas`; `living`, `minimal`, and
   `full` without rebuilding or changing routes.
2. **Charter-speed Cruise Director.** A Cruise Director lands on
   `/admin`, sees a layout that prioritizes current trips, manifests,
   ledger, cabins, and guest details, and can reach today's work in one
   mental hop.
3. **Org Admin reporting.** An Org Admin can reach Reports, Payments,
   Pricing, Users, Fleet, Import, and Audit without nested sidebar
   hunting, while still seeing a coherent operational hierarchy.
4. **Production silence.** The switcher is always visible during the
   evaluation window only when the debug flag allows it. When disabled,
   the app renders one configured default combination with no evaluation
   dock.
5. **Full migration.** Every existing admin page renders through the
   new page, section, card, table, chip, action, stat, and empty-state
   primitives. No page remains a raw stack of legacy `.admin-card`
   markup.

## Architecture

### Runtime Design State

Create a small design-mode layer under `web/src/admin/design/`:

```ts
export type PaletteMode = "abyss" | "glass" | "sunlit";
export type LayoutMode = "rail" | "spaces" | "canvas";
export type MotionMode = "living" | "minimal" | "full";

export type DesignMode = {
  palette: PaletteMode;
  layout: LayoutMode;
  motion: MotionMode;
};
```

`DesignModeProvider` owns the selected combination, persists it to
`localStorage`, and applies stable attributes to the admin root:

```html
<div
  class="admin-shell"
  data-palette="abyss"
  data-layout="rail"
  data-motion="living"
>
```

The body sea gradient is not rebound by themes. Theme attributes only
change working-surface tokens and component behavior above the body.

### Debug-Gated Floating Switcher

Extend `web/src/lib/devFlags.ts` with a frontend-visible flag for the
evaluation dock, for example:

```ts
type DevFlags = {
  filesystem_email: boolean;
  ui_redesign_switcher: boolean;
};
```

`DesignSwitcher` renders fixed in the bottom-right corner of every
admin page when that flag is true. It has three segmented controls:

- Palette: `abyss`, `glass`, `sunlit`
- IA: `rail`, `spaces`, `canvas`
- Motion: `living`, `minimal`, `full`

The switcher is not a modal and not route-specific. It must remain
usable on desktop and mobile without covering primary form submits,
table pagination, or critical ledger actions.

If adding a backend dev flag is larger than expected, use an existing
frontend-safe env flag as the fallback, but the final implementation
must have a production-silent path and document how it is disabled.

### Semantic Tokens

Replace the current warm-slate/amber design tokens with semantic tokens
that every palette rebinds:

```css
--surface-app
--surface-panel
--surface-panel-strong
--surface-raised
--text-primary
--text-muted
--text-inverse
--border-subtle
--border-strong
--accent-primary
--accent-secondary
--accent-danger
--accent-success
--focus-ring
--shadow-panel
```

Palette intent:

- `abyss`: dark navy depth, bioluminescent cyan-green accents,
  high-contrast panels, glowing focus/active states used sparingly.
- `glass`: translucent surfaces over the fixed sea gradient,
  `backdrop-filter: blur(...)`, strong borders and overlays so tables
  and forms remain readable.
- `sunlit`: bright light mode, saturated coral and cyan duotone,
  confident buttons, high-energy active states without washing out data.

Do not create one-off color literals in page files. Page components
consume tokens through shared primitives.

### IA Layout Modes

The IA data should be represented once and rendered three ways.
`Shell.tsx` stops hard-coding nested `NavLink` markup directly in the
render path and instead builds role-aware navigation sections:

```ts
type NavSpace = {
  key: "operations" | "configuration" | "insights";
  label: string;
  items: NavItem[];
};
```

Required modes:

- `rail`: a 56px icon rail plus command-bar affordance. Since the repo
  has no icon dependency, use concise text glyphs or initials only if a
  dependency is not approved; do not add an icon library just for this
  sprint. The command bar can be a route launcher, not a full fuzzy
  search engine.
- `spaces`: a full sidebar grouped as Operations, Configuration, and
  Insights. This is the most explicit IA comparison against today's
  sidebar.
- `canvas`: `/admin` becomes a spatial operations landing surface
  centered on today's deck-plan/charter view. It should still use the
  same underlying routes and role filtering; the shell must not trap
  users on Overview.

Role filtering is preserved in all modes. Cruise Directors must not see
or reach Org Admin-only nav entries through the switcher.

### Motion Modes

Motion is also data-attribute driven:

- `minimal`: no ambient motion, no page slide transitions, only
  functional hover/focus transitions.
- `living`: slow caustic overlay over the admin surface plus subtle
  ripple/shine hover states. Must respect `prefers-reduced-motion`.
- `full`: living ambience plus auth hero video treatment, animated
  counters where stat values already exist, and page-enter slide/fade
  transitions.

Use CSS animations and existing React state only. No new animation
library. All motion modes must have a reduced-motion fallback.

### Component Library

Create `web/src/admin/components/` and migrate all 25 admin pages to
shared primitives:

```text
web/src/admin/components/
├── AdminPage.tsx
├── ActionBar.tsx
├── Button.tsx
├── Card.tsx
├── Chip.tsx
├── DataTable.tsx
├── EmptyState.tsx
├── Field.tsx
├── FormSection.tsx
├── Metric.tsx
├── PageHeader.tsx
├── Section.tsx
└── Tabs.tsx
```

The exact file split can change during implementation, but the
principle cannot: pages stop composing raw legacy class strings for
common layout, card, chip, form, table, and stat patterns.

CSS may be split into component stylesheets or kept in organized global
files, but `web/src/styles/app.css` must be removed from the import path
by the end of the sprint. The recommended shape:

```text
web/src/styles/
├── base.css          # reset, fonts, body sea gradient
├── tokens.css        # global spacing/type/sea tokens
├── themes.css        # palette rebindings
├── motion.css        # motion modes
└── admin.css         # shell/component composition
```

### Auth Surface

Unauthenticated pages keep the fixed sea gradient and are included in
palette/motion styling where it matters:

- `full` mode gives auth pages the hero-video/immersive treatment.
- `living` uses ambience without large video.
- `minimal` remains static.

If actual video assets are not already in the repo, implement the hero
video slot as a CSS/bitmap-ready container and document the missing
asset rather than inventing an external dependency.

## Implementation Plan

### Phase 1: Design Mode Infrastructure (~15%)

**Files:**
- `web/src/admin/design/*` - runtime mode types, provider, storage.
- `web/src/lib/devFlags.ts` - switcher gate.
- `web/src/admin/Shell.tsx` - wrap admin chrome in provider and root
  data attributes.
- `web/src/styles/base.css`, `themes.css`, `motion.css`, `admin.css` -
  initial split.
- `web/src/main.tsx` - replace `app.css` import with new styles.

**Tasks:**
- [ ] Define palette/layout/motion union types and defaults.
- [ ] Persist selected mode to `localStorage` with validation.
- [ ] Apply `data-palette`, `data-layout`, and `data-motion` on the
  admin root.
- [ ] Preserve the exact Sprint 011 body sea gradient in base styles.
- [ ] Add the debug-gated bottom-right `DesignSwitcher`.
- [ ] Keep switcher accessible by keyboard and usable on mobile.

### Phase 2: Tokenized Palettes (~15%)

**Files:**
- `web/src/styles/themes.css`
- `web/src/styles/tokens.css`
- `DESIGN.md`

**Tasks:**
- [ ] Define semantic working-surface tokens.
- [ ] Implement `abyss`, `glass`, and `sunlit` token rebindings.
- [ ] Remove direct warm-slate/amber assumptions from reusable admin
  styles.
- [ ] Ensure focus, disabled, error, warning, success, and info states
  have sufficient contrast in all three palettes.
- [ ] Document the temporary three-palette evaluation decision in
  `DESIGN.md`.

### Phase 3: Shell and IA Modes (~20%)

**Files:**
- `web/src/admin/Shell.tsx`
- `web/src/admin/components/ShellNav.tsx`
- `web/src/admin/components/CommandLauncher.tsx`
- `web/src/admin/pages/Overview.tsx`
- `web/src/styles/admin.css`

**Tasks:**
- [ ] Convert nav definition to role-aware Operations /
  Configuration / Insights data.
- [ ] Render `spaces` as grouped full sidebar.
- [ ] Render `rail` as compact rail plus command launcher.
- [ ] Render `canvas` with a spatial `/admin` landing while preserving
  all routes and role filtering.
- [ ] Keep Org Admin-only route guards unchanged.
- [ ] Verify Cruise Director nav remains limited to allowed surfaces.

### Phase 4: Component Library and Page Migration (~30%)

**Files:**
- `web/src/admin/components/*`
- `web/src/admin/pages/*.tsx`
- `web/src/admin/AssignDirector.tsx`
- `web/src/admin/CurrencyPicker.tsx`
- `web/src/admin/UserMenu.tsx`
- `web/src/styles/admin.css`

**Tasks:**
- [ ] Create shared primitives for pages, headers, sections, cards,
  chips, metrics, tables, tabs, fields, buttons, empty states, and
  action bars.
- [ ] Migrate Overview, Trips, Trip Manifest, Trip Dashboard,
  Consumption Ledger, Trip Cabins, Guest Detail, and Guest Folio first
  because they define the Cruise Director evaluation path.
- [ ] Migrate Organization, Payments, Pricing, Users, Reports, Audit,
  Account, Fleet, Boat Detail, Boat Tabs, Cabins, Inventory, Import,
  Import Liveaboard, Import Spreadsheet, Import Job, and Onboarding.
- [ ] Remove legacy `.admin-card`, `.admin-page-header`, `.chip--*`,
  and similar page-level raw class usage once primitives cover them.
- [ ] Keep existing data loading, forms, mutations, and route
  parameters unchanged.

### Phase 5: Motion Modes and Auth Treatment (~10%)

**Files:**
- `web/src/styles/motion.css`
- `web/src/pages/*.tsx`
- `web/src/admin/components/Metric.tsx`

**Tasks:**
- [ ] Implement `minimal`, `living`, and `full` motion behavior from
  data attributes.
- [ ] Add caustic ambience and hover ripple treatment for `living`.
- [ ] Add auth hero media slot and stat/counter/page transitions for
  `full`.
- [ ] Respect `prefers-reduced-motion` in all modes.
- [ ] Ensure motion does not interfere with form input, table scanning,
  or ledger use.

### Phase 6: Documentation, ADR, and Verification (~10%)

**Files:**
- `DESIGN.md`
- `docs/decisions/0004-runtime-ui-redesign-matrix.md`
- `docs/sprints/SPRINT-025.md` (during final merge, not this draft)
- `web/src/styles/app.css`

**Tasks:**
- [ ] Add ADR 0004 explaining why Sprint 025 ships all 27 combinations
  behind a debug-gated runtime switcher.
- [ ] Update `DESIGN.md` decisions log with the bold-aesthetic
  reversal, three palettes, three IA modes, and three motion modes.
- [ ] Remove `web/src/styles/app.css` and its import.
- [ ] Run frontend build checks.
- [ ] Run repo test/build checks required by `CLAUDE.md`.
- [ ] Manually exercise every admin route in at least one Org Admin and
  one Cruise Director session.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `web/src/admin/design/*` | Create | Runtime design mode provider, types, storage validation. |
| `web/src/admin/components/*` | Create | Shared admin UI primitives used by all admin pages. |
| `web/src/admin/Shell.tsx` | Modify | Role-aware IA data, layout modes, provider wiring. |
| `web/src/admin/pages/*.tsx` | Modify | Migrate all admin pages to shared primitives and new layout rhythm. |
| `web/src/admin/{AssignDirector,CurrencyPicker,UserMenu}.tsx` | Modify/Move | Align existing admin primitives with component library. |
| `web/src/pages/*.tsx` | Modify | Auth surface support for palette/motion treatment. |
| `web/src/lib/devFlags.ts` | Modify | Add production-silent switcher gate. |
| `web/src/styles/base.css` | Create | Reset, fonts, fixed Sprint 011 sea gradient. |
| `web/src/styles/themes.css` | Create | Three palette token rebindings. |
| `web/src/styles/motion.css` | Create | Three motion modes and reduced-motion fallbacks. |
| `web/src/styles/admin.css` | Create | Shell and component styling. |
| `web/src/styles/app.css` | Delete | Retire monolithic legacy stylesheet. |
| `DESIGN.md` | Modify | Record temporary evaluation system and bold direction. |
| `docs/decisions/0004-runtime-ui-redesign-matrix.md` | Create | ADR for the runtime 27-combination matrix. |

## Definition of Done

- [ ] The Sprint 011 `body` sea gradient value and fixed attachment are
  preserved.
- [ ] A bottom-right floating switcher is visible on every admin page
  when the debug flag is enabled.
- [ ] The switcher can select all 3 palettes, all 3 IA layouts, and all
  3 motion modes at runtime without rebuilds.
- [ ] The selected combination persists across reloads and validates
  bad persisted values back to defaults.
- [ ] `abyss`, `glass`, and `sunlit` are visually distinct and use
  semantic tokens rather than page-local color literals.
- [ ] `rail`, `spaces`, and `canvas` are visually and structurally
  distinct while preserving role-aware access.
- [ ] `living`, `minimal`, and `full` are distinct and all respect
  `prefers-reduced-motion`.
- [ ] Org Admin and Cruise Director nav visibility and route guards do
  not regress.
- [ ] All current admin pages render through shared primitives in
  `web/src/admin/components/`.
- [ ] `web/src/styles/app.css` is removed from the import path and
  deleted.
- [ ] Existing admin workflows still work: onboarding, organization
  settings, payments, pricing, users, reports, audit, fleet, inventory,
  imports, trips, manifests, cabins, ledger, guest detail, and folio.
- [ ] `DESIGN.md` documents the bold redesign evaluation state.
- [ ] ADR 0004 records the runtime matrix rationale and the plan to
  choose a winner later.
- [ ] `npm run build` passes from `web/`.
- [ ] Repo checks required by `CLAUDE.md` pass, including Go tests,
  `go vet`, formatting, and frontend build.

## Security Considerations

- The switcher is a presentation-only debug affordance. It must not
  bypass route guards, reveal Org Admin-only navigation to Cruise
  Directors, or change API calls.
- The debug flag must have a production-silent path so the evaluation
  dock can be disabled without code deletion.
- `localStorage` values are untrusted. Validate palette/layout/motion
  strings before applying them to DOM attributes.
- The glass palette must preserve readable contrast for form labels,
  validation errors, and table data over the sea gradient.

## Dependencies

- Sprint 011 body sea gradient decision stays in force.
- Sprint 022 reports, Sprint 023 onboarding, and Sprint 024 payments
  surfaces must all migrate without behavior changes.
- No new external CSS framework, component framework, or animation
  library without explicit user approval.
- No icon dependency unless explicitly approved; the rail layout must
  be achievable with existing assets or simple text marks.

## References

- `docs/sprints/drafts/SPRINT-025-INTENT.md`
- `docs/sprints/README.md`
- `CLAUDE.md`
- `DESIGN.md`
- `web/src/admin/Shell.tsx`
- `web/src/styles/app.css`
- `web/src/styles/tokens.css`
