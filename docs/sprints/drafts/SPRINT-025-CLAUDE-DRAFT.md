# Sprint 025: Triptych — Three Bold Redesigns, One Live Switcher

## Overview

The audience for Liveaboard is adventurous — scuba operators
running charters in places like Raja Ampat, the Galapagos, and
the Maldives. The current UI ("warm slate + amber on white
cards") looks like an accounting tool. The user explicitly likes
only the body-level sea gradient from Sprint 011; the rest is
disposable.

Rather than pre-commit to one bold direction, this sprint ships
**three complete redesigns side-by-side** behind a floating
runtime switcher, so the choice between them is made in the live
app instead of from mockups. Three axes, three options each, 27
combinations to evaluate:

- **Palette:** `abyss` · `glass` · `sunlit`
- **Layout:** `rail` (⌘K + icon rail) · `spaces` (three labeled
  groups) · `canvas` (Today is a deck plan)
- **Motion:** `living` (caustics + ripple) · `minimal` · `full`
  (hero video + animated stats)

A floating dock at the bottom-right of every page exposes three
segmented controls. Selection persists to `localStorage`;
`<html>` carries `data-theme`, `data-layout`, `data-motion`
attributes; every component reads CSS variables that rebind per
attribute. The dock is gated behind a debug flag
(`VITE_TOPSIDE_SWITCHER=true`) so it can be silenced for
production once a winner is picked.

After the user lives with all 27 combinations for a few days, a
follow-up sprint deletes the two losing themes / layouts /
motion modes and removes the switcher.

Sprint 025 is **named Triptych** — three panels, side-by-side
for comparison.

## Use Cases

1. **The user picks a final design.** Boots `make dev`, sees
   the switcher dock, clicks through every palette × layout ×
   motion combination on the four hero pages and two
   representative deep-pages (Trip Manifest, Reports), takes
   notes, and tells me which combination wins. Sprint 026
   collapses to that combination.
2. **Cruise Director uses any of the three layouts during a
   charter.** Today / Trips / Manifest / Ledger must be one
   click apart in every layout option, not just the
   recommended one.
3. **Org Admin sets up a boat in any palette.** Form pages
   (Organization, Payments, Pricing, Users) must remain legible
   in the dark `abyss` theme and the translucent `glass` theme
   — high-contrast text, real focus rings.
4. **Either role on a smaller screen.** Each layout has a
   ≤ 900px collapsed form: `rail` stays the same; `spaces`
   collapses to a 56px icon rail; `canvas` collapses to a
   stacked card list.
5. **Sprint 026 makes the choice permanent.** The two losing
   themes / layouts / motion modes are deletable in one
   focused PR — token files, layout components, motion
   stylesheets — without touching any page.

## Architecture

### Naming: "Triptych"

```
┌──────────────────────────────────────────────────────────────────┐
│ FLOATING SWITCHER (bottom-right, every page, debug-flag-gated)   │
│ ┌────────────────────────────────────────────────────────────┐   │
│ │ Palette:  [abyss] [glass] [sunlit]                         │   │
│ │ Layout:   [rail]  [spaces] [canvas]                        │   │
│ │ Motion:   [living][minimal][full]                          │   │
│ └────────────────────────────────────────────────────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│ <html data-theme="abyss" data-layout="rail" data-motion="living">│
│                                                                  │
│   ┌──────────────┐  ┌────────────────────────────────────────┐   │
│   │ LAYOUT       │  │ PAGE (Page, Card, Stat, Chip, …)       │   │
│   │ rail | spaces│  │ tokens drive every color/space/font    │   │
│   │ | canvas     │  │ → no page knows which theme is active  │   │
│   └──────────────┘  └────────────────────────────────────────┘   │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│ WATERLINE: --gradient-sea (Sprint 011) — UNCHANGED               │
│   linear-gradient(180deg, sea-50 → sea-200 → sea-400 → sea-600)  │
│ (`abyss` darkens the bottom stops; sea identity preserved)       │
└──────────────────────────────────────────────────────────────────┘
```

### Token strategy: semantic tokens + per-theme rebind

Every component references **semantic tokens** only —
`var(--c-surface)`, `var(--c-surface-border)`, `var(--c-text)`,
`var(--c-text-muted)`, `var(--c-accent)`, `var(--c-accent-hover)`,
`var(--c-selection)`, `var(--shadow-elevated)`. Tokens are
rebound per `[data-theme=...]`:

```css
:root { /* default = abyss */
  --c-surface:        #0A1428;
  --c-surface-border: rgba(110,210,250,0.18);
  --c-text:           #E8F4FA;
  --c-text-muted:     #7C94AB;
  --c-accent:         #00E6D7;        /* bioluminescent */
  --c-accent-hover:   #4FFFEE;
  --c-selection:      #FF6F61;        /* coral flash */
  --shadow-elevated:  0 0 40px -8px rgba(0,230,215,0.35);
}

[data-theme="glass"] {
  --c-surface:        rgba(255,255,255,0.72);
  --c-surface-border: rgba(255,255,255,0.45);
  --c-text:           #0E1726;
  --c-text-muted:     #475569;
  --c-accent:         #FF6F61;
  --c-accent-hover:   #E55A4D;
  --c-selection:      #FF6F61;
  --shadow-elevated:  0 12px 40px -12px rgba(8,86,128,0.35);
  --backdrop-blur:    18px;
}

[data-theme="sunlit"] {
  --c-surface:        #FFFFFF;
  --c-surface-border: #6DCEF0;        /* sea-300 — a real color, not grey */
  --c-text:           #08294A;
  --c-text-muted:     #086998;
  --c-accent:         #FF4D3D;        /* sun-coral */
  --c-accent-hover:   #E03525;
  --c-secondary:      #00B4D8;        /* open-ocean cyan */
  --c-selection:      #FF4D3D;
  --shadow-elevated:  0 8px 24px -8px rgba(8,41,74,0.18);
}
```

Sea-palette + semantic colors (success / warning / error / info)
also rebind per theme so badges/chips harmonize.

### Layout strategy: three Shell variants

`Shell.tsx` reads `data-layout` and renders one of:

```tsx
function AdminShell() {
  const layout = useTopsideState().layout; // 'rail' | 'spaces' | 'canvas'
  switch (layout) {
    case 'rail':   return <RailShell />;
    case 'spaces': return <SpacesShell />;
    case 'canvas': return <CanvasShell />;
  }
}
```

| Layout | Sidebar | Today | Other pages |
|---|---|---|---|
| `rail` | 56px icon rail, labels on hover; ⌘K command bar at the top of every page | Existing Overview but reshaped to a sparse stat grid | Unchanged composition; new chrome |
| `spaces` | 220px sidebar, three labeled groups: Operations · Configuration · Insights; "Today" replaces Overview | "Today" with the operational triage triplet (setup, active trips, exceptions) | Unchanged composition |
| `canvas` | No persistent sidebar; Today landing is a spatial deck plan of the fleet; secondary nav is the command bar | "Today" is the boat-plan canvas — each boat is a node, active trips are glowing arcs, click into them | Other pages still need top-bar nav since canvas only fits the landing |

`spaces` and `rail` share most of the chrome. `canvas` ships its
own `TodayCanvas.tsx` with an SVG/CSS deck-plan rendering of the
org's boats fed by the existing Overview API; no backend change.

### Motion strategy: three CSS-only switches

Per `data-motion`:

| Mode | Behavior |
|---|---|
| `living` | Body gets a slow (90s) animated caustic-light overlay via a `::after` pseudo-element with `mix-blend-mode: soft-light`. Hover states have a 600ms ease-out ripple via `@keyframes`. Page transitions are 220ms cross-fade. |
| `minimal` | All ambient motion suppressed via `@media (prefers-reduced-motion)` patterns. Hover/focus/active are color-only. Auth shell uses a still image. |
| `full` | Auth shell mounts a 12-second loop underwater video as the hero background. Stats animate-on-mount with `requestAnimationFrame` count-up. Page transitions use a 280ms slide. Caustics from `living` are also on. |

All three are CSS modules + small JS hooks toggleable at runtime.
No theme-coupled motion: every motion mode works under every
theme.

### The Switcher dock

```tsx
// web/src/admin/components/TopsideSwitcher.tsx
// Floating bottom-right; only renders when VITE_TOPSIDE_SWITCHER === 'true'.

<aside className={styles.dock} aria-label="Triptych theme switcher">
  <Segmented label="Palette" value={theme}  onChange={...} options={['abyss','glass','sunlit']} />
  <Segmented label="Layout"  value={layout} onChange={...} options={['rail','spaces','canvas']} />
  <Segmented label="Motion"  value={motion} onChange={...} options={['living','minimal','full']} />
  <button className={styles.copyShare} onClick={copyShareLink}>Copy ?triptych= link</button>
</aside>
```

State is held in `useTopsideState()` which:
1. Reads URL `?triptych=abyss,rail,living` if present (lets the
   user share specific combos).
2. Falls back to `localStorage["topside"]`.
3. Falls back to a "first-run" default of `abyss / spaces / living`
   (the bold pick).
4. Writes `data-theme`, `data-layout`, `data-motion` to `<html>`
   on every change.

### Component library

```
web/src/admin/components/
  Page.tsx           — <Page title subtitle actions> shell
  Card.tsx           — <Card title> with optional <Card.Footer>
  Stat.tsx           — <Stat label value trend> with count-up under `full`
  Chip.tsx           — semantic variant
  Empty.tsx          — empty state
  Section.tsx        — within a card
  DataTable.tsx      — typed-column table wrapper
  CommandBar.tsx     — ⌘K palette (active in `rail` and `canvas`)
  RailShell.tsx      — Shell variant
  SpacesShell.tsx    — Shell variant
  CanvasShell.tsx    — Shell variant
  TodayCanvas.tsx    — deck-plan landing for `canvas`
  TopsideSwitcher.tsx — the floating dock
  index.ts
```

CSS modules per component. No global `app.css` survives.

## Implementation Plan

### Phase 1: Tokens, themes, switcher state (~12%)

**Files:**
- `web/src/styles/tokens.css` — Create. Default `:root` =
  `abyss`. `[data-theme="glass"]` and `[data-theme="sunlit"]`
  rebinds.
- `web/src/styles/reset.css` — Create.
- `web/src/styles/base.css` — Create. `<body>` gradient
  (preserved verbatim from Sprint 011), font loading, focus
  outline.
- `web/src/styles/motion.css` — Create. `[data-motion="living"]`,
  `[data-motion="minimal"]`, `[data-motion="full"]` rules:
  caustics, transitions, count-up keyframes.
- `web/src/admin/useTopsideState.ts` — Create. URL+localStorage-
  backed hook; writes `data-*` on `<html>`; returns
  `{theme, layout, motion, set}`.
- `web/src/main.tsx` — Wire foundation stylesheets; mount the
  switcher (debug-flag gated); keep importing `app.css` until
  Phase 5.

**Tasks:**
- [ ] All three themes pass WCAG AA on body text against their
      surface (manually verified in the running app).
- [ ] `data-theme`/`data-layout`/`data-motion` reflect to
      `<html>` instantly.
- [ ] URL `?triptych=abyss,rail,living` deep-links a combo.
- [ ] `--gradient-sea` matches the Sprint 011 hex codes
      exactly under all three themes (sub-stop colors may
      shift, but the token name + role are preserved).

### Phase 2: Component library (~18%)

**Files:**
- `Page`, `Card`, `Stat`, `Chip`, `Empty`, `Section`,
  `DataTable`, `CommandBar`, `TopsideSwitcher` and their
  `.module.css` siblings.
- `web/src/admin/components/__tests__/*.tsx` — vitest props +
  snapshot tests per component.
- `web/src/admin/components/index.ts` — Barrel.

**Tasks:**
- [ ] No component accepts `style={...}` or `className`
      escape hatches in its public API — all visual variance
      is theme-driven.
- [ ] `Stat` count-up uses `requestAnimationFrame` only when
      `data-motion="full"`; otherwise renders the final value.
- [ ] `Chip` consumes semantic tokens, not raw hex.
- [ ] `CommandBar` opens on ⌘K / Ctrl+K; data source is a
      typed `Command[]` list registered by pages.
- [ ] Snapshot tests render each component under all 3 themes
      via `data-theme` on the test root.

### Phase 3: Three Shell variants + Today (~18%)

**Files:**
- `RailShell.tsx` + `.module.css`
- `SpacesShell.tsx` + `.module.css`
- `CanvasShell.tsx` + `.module.css`
- `TodayCanvas.tsx` + `.module.css` (the deck-plan landing)
- `web/src/admin/Shell.tsx` — Slim shim: reads layout, picks
  the variant.
- `web/src/admin/pages/Today.tsx` — Renamed from Overview;
  composition unchanged for `rail`/`spaces`, swapped for the
  canvas variant.
- `web/src/main.tsx` — Index route now mounts `Today`.

**Tasks:**
- [ ] Role gating works in all three layouts (Cruise Director
      sees Operations only; Org Admin sees everything).
- [ ] Sidebar/rail items reorder per the new IA (Inventory →
      Operations; Audit → Insights).
- [ ] `canvas` layout: SVG deck-plan renders the org's boats
      from the existing Overview API. Active trips are
      glowing arcs. Click a boat → trip dashboard.
- [ ] `rail` collapses gracefully at ≤ 700px; `spaces`
      collapses to a 56px rail at ≤ 900px; `canvas` collapses
      to a stacked card list at ≤ 900px.
- [ ] Active-state styling uses `--c-selection` on a subtle
      `--c-accent-subtle` surface, never a hard fill.

### Phase 4: Hero page migrations (~15%)

The four highest-traffic pages migrate first to validate every
theme × layout × motion combination is renderable:

**Files:**
- `web/src/admin/pages/Today.tsx`
- `web/src/admin/pages/Trips.tsx`
- `web/src/admin/pages/TripDashboard.tsx`
- `web/src/admin/pages/TripManifest.tsx`

**Tasks:**
- [ ] Each hero page renders 1:1 with previous behavior under
      every (theme × layout × motion) combo.
- [ ] Manual visual review of all 9 (theme × layout) combos at
      desktop (≥1280px), tablet (~900px), and phone (~390px).
- [ ] No use of any class from `app.css`.
- [ ] Existing functional tests still pass.

### Phase 5: Remaining 21 pages + `app.css` removal (~22%)

**Files:** the other 21 admin pages.

**Tasks:**
- [ ] Each page migrates to component library + a small
      page-local `.module.css` for layout-only concerns.
- [ ] Delete `web/src/styles/app.css`. Remove the import.
- [ ] Remove DM Sans / General Sans / Geist (display) loads
      from `index.html` — Inter (variable) + Geist Mono only.
- [ ] `grep -r "admin-card\|admin-page-header\|admin-nav\|chip--"
      web/src` returns nothing.

### Phase 6: Docs + ADR + Verification (~15%)

**Files:**
- `docs/decisions/0004-topside-triptych.md` — New ADR.
- `DESIGN.md` — Full rewrite. Captures the Triptych framework
  (semantic tokens + 3 themes + 3 layouts + 3 motion modes)
  and the decision log entries for each. Notes the future
  collapse to a single combo in Sprint 026.
- `CLAUDE.md` — One-line note: "Web changes go through
  `web/src/admin/components/` — page files should not declare
  bespoke CSS classes."
- `docs/sprints/SPRINT-025.md` — DoD ticked on merge.

**Tasks:**
- [ ] ADR documents: the "three competing visions, judged in
      the live app" framework, the semantic-token contract,
      and the criteria the user will use to choose.
- [ ] DESIGN.md decisions log gets one new row per axis
      (palette / layout / motion) and one for the
      framework itself.
- [ ] `make dev` boot: switcher visible, all 27 combinations
      reachable, screenshot a 3×3 palette × layout matrix into
      the ADR.
- [ ] `npm run build` clean. `go test ./...` clean (no Go
      changes expected, sanity check).

## API Endpoints

None changed. Web client + docs only.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `web/src/styles/tokens.css` | Create | Default `:root` = abyss; `[data-theme="glass"]` + `[data-theme="sunlit"]` rebinds. |
| `web/src/styles/reset.css` | Create | Minimal modern reset. |
| `web/src/styles/base.css` | Create | Body gradient (Sprint 011 preserved), font loading, focus styles. |
| `web/src/styles/motion.css` | Create | `[data-motion="living"]` / `minimal` / `full` rules. |
| `web/src/admin/useTopsideState.ts` | Create | URL + localStorage hook; writes `data-*` to `<html>`. |
| `web/src/admin/components/*.tsx` + `.module.css` | Create | Page, Card, Stat, Chip, Empty, Section, DataTable, CommandBar. |
| `web/src/admin/components/RailShell.tsx` + module | Create | `rail` layout. |
| `web/src/admin/components/SpacesShell.tsx` + module | Create | `spaces` layout. |
| `web/src/admin/components/CanvasShell.tsx` + module | Create | `canvas` layout. |
| `web/src/admin/components/TodayCanvas.tsx` + module | Create | Deck-plan landing for canvas. |
| `web/src/admin/components/TopsideSwitcher.tsx` + module | Create | Floating bottom-right dock. |
| `web/src/admin/components/index.ts` | Create | Barrel. |
| `web/src/admin/components/__tests__/*` | Create | vitest snapshot/variant tests. |
| `web/src/admin/Shell.tsx` | Modify | Slim shim that picks the Shell variant. |
| `web/src/admin/pages/Today.tsx` | Create (rename Overview) | New landing name. |
| `web/src/admin/pages/Overview.tsx` | Delete (after rename) | Replaced. |
| `web/src/admin/pages/*.tsx` (24 others) | Modify | Migrate to components. |
| `web/src/main.tsx` | Modify | Mount switcher; reorder CSS imports; rename index route. |
| `web/index.html` | Modify | Inter + Geist Mono only by end of sprint. |
| `web/src/styles/app.css` | Delete (Phase 5) | Replaced by per-component modules. |
| `DESIGN.md` | Rewrite | Triptych framework + decisions log. |
| `docs/decisions/0004-topside-triptych.md` | Create | ADR. |
| `CLAUDE.md` | Modify | One-line components pointer. |
| `docs/sprints/SPRINT-025.md` | Create | This sprint, after merge. |

## Definition of Done

- [ ] Body-level sea gradient is preserved verbatim under all
      three themes (some sub-stops may shift but the role +
      token name don't).
- [ ] `web/src/styles/app.css` no longer exists.
- [ ] `grep -r "admin-card\|admin-page-header\|admin-nav\|chip--"
      web/src` returns zero matches.
- [ ] Floating switcher visible at bottom-right of every
      admin page when `VITE_TOPSIDE_SWITCHER=true`; gated off
      otherwise.
- [ ] Switcher offers three segmented controls (palette /
      layout / motion) with 3 options each; selection persists
      across reloads.
- [ ] `?triptych=abyss,rail,living` URL deep-links a specific
      combination.
- [ ] All 27 (3 × 3 × 3) combinations render without console
      errors on at least the four hero pages.
- [ ] All 25 admin pages render under at least one (theme ×
      layout × motion) combo, and under all 3 themes when in
      the `spaces` layout (the lowest-risk baseline).
- [ ] Component library: Page, Card, Stat, Chip, Empty,
      Section, DataTable, CommandBar, three Shells,
      TodayCanvas, TopsideSwitcher — all in
      `web/src/admin/components/`, all with co-located CSS
      modules, all covered by vitest tests.
- [ ] Auth pages (Login, Signup, Verify, Forgot, Reset, Accept)
      visually align with the new chrome under `abyss` and
      `sunlit` (the two extremes).
- [ ] `DESIGN.md` rewritten end-to-end with the Triptych
      framework; decisions log preserved + appended.
- [ ] `docs/decisions/0004-topside-triptych.md` exists.
- [ ] `CLAUDE.md` mentions the component library.
- [ ] `npm run build` clean. `go test ./...` clean. `vitest`
      clean.
- [ ] Manual visual smoke: every (theme × layout) combo
      reviewed by the user in the running app at 1280px,
      900px, 390px.

## Documentation Manifest

The implementation sprint MUST land the following docs changes
alongside the code. Phase 6 verifies each file in this list was
modified before marking the sprint complete.

### New ADRs

- `docs/decisions/0004-topside-triptych.md` — Codifies the
  "ship three competing redesigns behind a runtime switcher,
  judge in the live app, collapse to one in 026" framework,
  the semantic-token contract, and the visual-decision
  criteria.

### Amended ADRs

- None. (`0001-auth-provider`, `0002-revert-clerk`,
  `0003-reporting-storage` are unrelated.)

### Cross-cutting docs

- `DESIGN.md` — Full rewrite. New sections: Triptych framework,
  three themes (abyss/glass/sunlit), three layouts
  (rail/spaces/canvas), three motion modes (living/minimal/
  full), semantic-token contract. Decisions log preserved +
  appended with one row per axis and one for the framework.
- `CLAUDE.md` — Add one-line pointer: "Web visual/layout
  changes go through `web/src/admin/components/`; page files
  shouldn't declare bespoke CSS classes."

### Skipped (with reasoning)

- `docs/product/*` — Personas and user stories don't change
  with a visual redesign; product surface is unchanged.
- `docs/sprints/README.md` — Sprint workflow doesn't change.
- `docs/dev/email-inbox.md` — Unrelated.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| 27-combo evaluation is overwhelming; the user can't decide | High | Indecision / sprint stalls into 026 | DoD requires the user to pick a winning combo before the sprint is marked complete. Provide a 3×3 palette × layout matrix screenshot in the ADR as a decision aid. |
| Sprint scope is huge (3 themes × 3 layouts × 3 motion + 25 pages) | High | Schedule slip | Phase 4 lands the 4 hero pages first so the switcher is demoable even if Phase 5 spills. Component library uses semantic tokens — adding a theme is editing one file, not 25. |
| `canvas` layout (deck-plan Today) is the heaviest single lift | Medium | Could push the whole sprint | Phase 3 owns `canvas`; if it stalls, ship `rail` + `spaces` only and document `canvas` as "future" in the ADR. The two-axis (palette × layout) judgment still works with 2 layouts. |
| Glass theme + low-contrast text fails accessibility | Medium | Some pages unreadable in `glass` | Phase 1 mandates WCAG AA verification per theme; if `glass` fails, raise body-text contrast to `#0E1726` and tighten card opacity (already in the token spec). |
| The bioluminescent `abyss` accent reads neon / kids-game | Medium | Looks wrong on a B2B tool | Use the color sparingly (CTAs + active states only), pair with soft outer glow rather than hard fills; the `--c-selection` coral provides a warm counterweight. |
| Caustic overlay tanks performance on low-end devices | Low | Janky scrolling | `[data-motion="living"]` overlay uses `will-change: transform` and a 90s cycle; auto-suppressed under `prefers-reduced-motion`. |
| Switcher ships to production by accident | Low | Confused real users | Gated behind `VITE_TOPSIDE_SWITCHER` env var (defaults false in production). Phase 1 task verifies it's off without the flag. |
| Sprint 026 cleanup is painful (deleting one theme touches everything) | Low | Slow follow-up | Token files are isolated; layouts are isolated React components; motion is isolated CSS. The "collapse" PR should be a few dozen file deletes + edits, not a rewrite. ADR documents the collapse procedure. |

## Security Considerations

- No new attack surface — presentation-layer sprint.
- Role-gating logic is preserved verbatim across all three
  layouts; the IA reshape changes labels/grouping, not
  who-sees-what.
- The switcher reads `?triptych=...` from the URL — no
  user-supplied content is rendered as HTML; values are
  validated against an allowlist of `theme/layout/motion`
  keys before reflecting to `<html>`.
- No new external CSS/JS dependencies beyond Inter (variable
  Google Font) and Geist Mono (existing CDN trust). Underwater
  video for `full` motion is a static asset bundled with the
  build, not a third-party load.

## Dependencies

- Sprint 011 (sea gradient) — depended on; preserved
  verbatim under every theme.
- Sprint 008 (admin chrome) — depended on; replaced.
- No external blockers.

## Open Questions

These should be raised proactively during merge if anything
turns up. The interview already locked every major axis.

1. **Underwater hero video source for `full` motion.** Public-
   domain CC0 clip vs. a bundled asset the user provides? If
   neither lands, `full` motion's hero falls back to a still
   image of the same composition.
2. **Storybook or `/dev/components` for visual review.**
   `/dev/components` is one extra route and matches the
   existing `/dev/inbox` pattern from Sprint 021. Pick the
   simpler one unless the user requests Storybook.
3. **Should the switcher's "Copy ?triptych=" button copy
   absolute URL or relative path?** Default: absolute (so the
   link survives being pasted into Slack / a note).
4. **CommandBar's command registry.** Static, or do pages
   register per-route? Default: static for this sprint;
   per-route registration is a 026+ enhancement.
