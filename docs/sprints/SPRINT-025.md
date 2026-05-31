# Sprint 025: Triptych — Three Bold UIs, Judged in the Live App

## Overview

Liveaboard's audience is adventurous — scuba operators running
charters in Raja Ampat, the Galapagos, and the Maldives. The
current admin chrome (warm slate + amber on white cards) looks
like an accounting tool. The user explicitly likes only the
body-level sea gradient introduced in Sprint 011; everything
else is disposable.

Rather than pre-commit to one direction from mockups, Sprint
025 ships **three complete redesigns side-by-side** behind a
runtime switcher. The user lives with them in the running app,
picks a winner, and Sprint 026 collapses to that combination.
Three axes, three options each, 27 combinations:

- **Palette:** `abyss` (dark navy + bioluminescent cyan-green),
  `glass` (translucent cards with backdrop-blur),
  `sunlit` (saturated coral + cyan duotone, light surfaces).
- **Layout:** `rail` (56px icon rail + ⌘K command bar),
  `spaces` (three labeled groups: Operations / Configuration /
  Insights), `canvas` (spatial "today's deck" landing).
- **Motion:** `living` (slow caustic overlay + ripple hover),
  `minimal` (no ambient motion), `full` (auth hero + animated
  counters + page transitions, all over `living`).

A floating dock at the bottom-right of every admin page exposes
three segmented controls, persists selection to `localStorage`,
and writes `data-palette`, `data-layout`, `data-motion` to the
admin root. The dock is gated by the existing `useDevFlags()`
pattern (a new `ui_redesign_switcher` flag, true in dev only),
so the production surface defaults to one configured
combination with no evaluation dock.

The **body-level sea gradient from Sprint 011 is preserved
verbatim** — same hex stops, same `background-attachment:
fixed`, same role. Themes rebind only the working-surface
tokens that sit above it.

## Use Cases

1. **Live design evaluation.** The user boots `make dev`, sees
   the switcher dock, flips between every (palette × layout ×
   motion) combination on the routes they care about (Today,
   Trips, Trip Manifest, Trip Dashboard, Trip Ledger, Reports,
   Payments, Pricing, Users) and decides.
2. **Charter-speed Cruise Director.** A Cruise Director lands
   on `/admin` under any layout, sees current-trip work
   prioritized, and reaches today's manifest in one hop.
3. **Org Admin reaches Reports + Configuration.** All three
   layouts surface Reports without nested expansion and group
   configuration screens coherently.
4. **Production silence.** With the `ui_redesign_switcher`
   flag false (production default), the dock disappears and
   the app renders one configured default combination
   (`abyss / spaces / living` for now).
5. **Sprint 026 makes the choice permanent.** The two losing
   themes / layouts / motion modes are deletable in one
   focused PR — token files are isolated, nav renderers are
   isolated React components, motion is isolated CSS — without
   touching any page.

## Architecture

### The layered system

```
┌──────────────────────────────────────────────────────────────────┐
│ FLOATING SWITCHER (bottom-right; gated by useDevFlags)           │
│ ┌────────────────────────────────────────────────────────────┐   │
│ │ Palette:  [abyss] [glass] [sunlit]                         │   │
│ │ Layout:   [rail]  [spaces] [canvas]                        │   │
│ │ Motion:   [living][minimal][full]                          │   │
│ │ [ Copy ?triptych= link ]                                   │   │
│ └────────────────────────────────────────────────────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│ <div class="admin-shell"                                         │
│      data-palette="abyss"                                        │
│      data-layout="rail"                                          │
│      data-motion="living">                                       │
│                                                                  │
│   ┌─────────────────┐  ┌────────────────────────────────────┐    │
│   │ NAV RENDERER    │  │ PAGE (Page, Card, Stat, Chip, …)   │    │
│   │ Rail|Spaces|    │  │ Components consume SEMANTIC TOKENS │    │
│   │ Canvas — picked │  │ only; never raw hex. Pages may use │    │
│   │ by data-layout  │  │ className for layout, not color.   │    │
│   └─────────────────┘  └────────────────────────────────────┘    │
│                                                                  │
├──────────────────────────────────────────────────────────────────┤
│ WATERLINE: --gradient-sea (Sprint 011, UNCHANGED)                │
│   linear-gradient(180deg,                                        │
│     var(--c-sea-50)  0%,                                         │
│     var(--c-sea-200) 22%,                                        │
│     var(--c-sea-400) 60%,                                        │
│     var(--c-sea-600) 100%)                                       │
│   body { background: var(--gradient-sea); background-attachment: fixed; }│
└──────────────────────────────────────────────────────────────────┘
```

The sea gradient lives on `<body>` and is not part of any
theme's rebind set. Themes change only `--surface-*`,
`--text-*`, `--accent-*`, `--border-*`, `--shadow-*`,
`--focus-ring`. The sea palette tokens (`--c-sea-50` ...
`--c-sea-700`) and `--gradient-sea` are untouched.

### Runtime mode layer

```ts
// web/src/admin/design/types.ts
export type PaletteMode = "abyss" | "glass" | "sunlit";
export type LayoutMode  = "rail"  | "spaces" | "canvas";
export type MotionMode  = "living"| "minimal"| "full";

export type DesignMode = {
  palette: PaletteMode;
  layout:  LayoutMode;
  motion:  MotionMode;
};

// web/src/admin/design/DesignModeProvider.tsx
// - Reads `?triptych=palette,layout,motion` if present.
// - Else reads localStorage["triptych"] (validated against
//   allowlists; bad values fall back to defaults).
// - Else uses the default = { abyss, spaces, living }.
// - Writes data-palette / data-layout / data-motion on the
//   admin root on every change.
// - Exposes a useDesignMode() hook for the switcher.
```

Stored values are validated against typed allowlists before
being reflected to DOM attributes — a malformed
`?triptych=foo,bar,baz` URL falls back to defaults instead of
writing `data-palette="foo"`.

### Semantic tokens

Components read only semantic tokens. Each palette rebinds
them:

```css
/* tokens.css — semantic contract every component reads */
:root {
  --surface-app;
  --surface-panel;
  --surface-panel-strong;
  --surface-raised;
  --text-primary;
  --text-muted;
  --text-inverse;
  --border-subtle;
  --border-strong;
  --accent-primary;
  --accent-primary-hover;
  --accent-primary-subtle;
  --accent-secondary;
  --accent-selection;
  --status-success; --status-success-bg;
  --status-warning; --status-warning-bg;
  --status-error;   --status-error-bg;
  --status-info;    --status-info-bg;
  --focus-ring;
  --shadow-panel;
  --shadow-elevated;
}
```

```css
/* themes.css — palette rebinds */
[data-palette="abyss"] {
  --surface-panel:        #0A1428;
  --surface-panel-strong: #07101E;
  --border-subtle:        rgba(110,210,250,0.18);
  --text-primary:         #E8F4FA;
  --text-muted:            #7C94AB;
  --accent-primary:       #00E6D7;        /* bioluminescent */
  --accent-primary-hover: #4FFFEE;
  --accent-selection:     #FF6F61;        /* coral flash */
  --shadow-elevated:      0 0 40px -8px rgba(0,230,215,0.35);
}
[data-palette="glass"] {
  --surface-panel:        rgba(255,255,255,0.72);
  --surface-panel-strong: rgba(255,255,255,0.86);
  --border-subtle:        rgba(255,255,255,0.45);
  --text-primary:         #0E1726;
  --text-muted:           #475569;
  --accent-primary:       #FF6F61;
  --accent-primary-hover: #E55A4D;
  --shadow-elevated:      0 12px 40px -12px rgba(8,86,128,0.35);
  --backdrop-blur:        18px;
}
[data-palette="sunlit"] {
  --surface-panel:        #FFFFFF;
  --surface-panel-strong: #F4FBFF;
  --border-subtle:        #6DCEF0;        /* sea-300 — a real color */
  --text-primary:         #08294A;
  --text-muted:           #086998;
  --accent-primary:       #FF4D3D;        /* sun-coral */
  --accent-primary-hover: #E03525;
  --accent-secondary:     #00B4D8;        /* open-ocean cyan */
  --shadow-elevated:      0 8px 24px -8px rgba(8,41,74,0.18);
}
```

Pages and component CSS modules reference only
`var(--surface-panel)` etc. They never see raw hex.

### Single behavioral shell, three nav renderers

One `AdminShell` owns the behavioral surface (`useMe`, role
filter, sign-out, onboarding auto-show, `RequireAdmin`). Below
it, a `<NavRenderer>` component reads `data-layout` and renders
one of three presentations:

```tsx
// web/src/admin/Shell.tsx
function AdminShell() {
  // useMe + role filter + onboarding hook + signout — UNCHANGED
  const { layout } = useDesignMode();
  const nav = buildRoleAwareNav(me); // single source of truth
  return (
    <div className="admin-shell"
         data-palette={palette}
         data-layout={layout}
         data-motion={motion}>
      <NavRenderer nav={nav} layout={layout} />
      <main className="admin-main"><Outlet /></main>
    </div>
  );
}

// NavRenderer dispatches by layout:
function NavRenderer({ nav, layout }) {
  switch (layout) {
    case "rail":   return <RailNav   nav={nav} />;
    case "spaces": return <SpacesNav nav={nav} />;
    case "canvas": return <CanvasNav nav={nav} />;
  }
}
```

Role gating, sign-out, onboarding auto-show, and `RequireAdmin`
behavior are written **once** in `AdminShell`. The three nav
renderers only render markup; they cannot diverge on
authorization.

### IA structure (data, rendered three ways)

```ts
type NavItem = { to: string; label: string; end?: boolean;
                 adminOnly?: boolean; children?: NavItem[] };
type NavSpace = { key: "operations" | "configuration" | "insights";
                  label: string; items: NavItem[] };

// One canonical nav structure — every layout consumes it.
const NAV: NavSpace[] = [
  { key: "operations", label: "Operations", items: [
    { to: "/admin",           label: "Today", end: true },     // file/route stay; sidebar label changes
    { to: "/admin/trips",     label: "Trips",
      children: [{ to: "/admin/import", label: "Import", adminOnly: true }] },
    { to: "/admin/inventory", label: "Inventory", adminOnly: true },
  ]},
  { key: "configuration", label: "Configuration", items: [
    { to: "/admin/organization", label: "Organization", adminOnly: true,
      children: [
        { to: "/admin/organization/payments", label: "Payments", adminOnly: true },
        { to: "/admin/organization/pricing",  label: "Pricing",  adminOnly: true },
      ] },
    { to: "/admin/fleet", label: "Fleet", adminOnly: true },
    { to: "/admin/users", label: "Users", adminOnly: true },
  ]},
  { key: "insights", label: "Insights", items: [
    { to: "/admin/reports", label: "Reports", adminOnly: true },
    { to: "/admin/audit",   label: "Audit" },
  ]},
];
```

| Layout | Renders the nav as | Notes |
|---|---|---|
| `rail` | 56px icon rail (text glyphs / initials — no new icon library), labels on hover; ⌘K command bar opens a route launcher fed by the same NAV data | Command bar is a route launcher, not a registry. |
| `spaces` | 220px sidebar with the three space labels as section headers; items underneath in order | Familiar sidebar shape; reshaped order. |
| `canvas` | `/admin` (Overview.tsx) renders an additional spatial "today's fleet" surface composed from existing Overview API data; minimal top-bar nav (a TopBar component) carries the rest of the routes | No backend changes; data source is the Overview API only. |

The Overview page (file: `Overview.tsx`, route: `/admin`) is
**not renamed**. Sidebar label is "Today"; file and route are
stable so the Sprint 023 onboarding auto-show
(`location.pathname === "/admin"`) doesn't ripple.

For `canvas`, `Overview.tsx` reads `useDesignMode().layout` and
mounts a `<TodayCanvas>` component instead of the existing
`<AdminOverview>` body when `layout === "canvas"`. Same fetch,
same data — different rendering.

### Motion modes

Per `[data-motion]`:

| Mode | Behavior |
|---|---|
| `minimal` | No ambient motion. Only color hover/focus transitions on interactive elements. Default under `prefers-reduced-motion`. |
| `living` | Body `::after` pseudo-element with a slow (90s) caustic-light overlay (`mix-blend-mode: soft-light`, `pointer-events: none`). Hover states have a 600ms ease-out ripple via `@keyframes`. Page transitions: 220ms cross-fade via React Router. |
| `full` | Everything from `living` plus: auth shell mounts an underwater video as hero background **if a video asset is bundled** — otherwise the same still image as `living`; `<Stat>` counters animate-on-mount with `requestAnimationFrame`; page transitions use a 280ms slide. |

All modes respect `@media (prefers-reduced-motion: reduce)` —
the OS preference forces effective `minimal` regardless of
selection, but the switcher selection is preserved (so when the
user disables reduced-motion at the OS level, `living` returns).

### Component library

```
web/src/admin/components/
  Page.tsx                — title/subtitle/actions shell
  PageHeader.tsx          — split out for grid usage
  Card.tsx                — semantic surface
  Section.tsx             — within a card
  Stat.tsx                — label + value, count-up under full
  Chip.tsx                — variant: success|warning|error|info|neutral
  Empty.tsx               — empty-state pattern
  DataTable.tsx           — typed Column<Row> wrapper
  Field.tsx               — form field row
  FormSection.tsx         — grouped form section
  Button.tsx              — accent CTA / secondary / quiet
  Tabs.tsx                — tabbed surface
  ActionBar.tsx           — sticky page-action toolbar
  RailNav.tsx             — rail layout's nav
  SpacesNav.tsx           — spaces layout's nav
  CanvasNav.tsx           — canvas layout's top-bar nav
  TodayCanvas.tsx         — canvas-mode landing surface
  CommandBar.tsx          — ⌘K route launcher (rail + canvas)
  TriptychSwitcher.tsx    — the floating dock
  index.ts                — barrel
```

- Public component props expose only design-token-mapped
  variants — never raw color literals.
- `className` IS allowed on components, but only for layout
  composition (grid placement, flex behavior, max-width). It
  is NEVER used to override colors / borders / typography.
- Lint rule (or grep CI check) ensures no `color:` or
  `background:` literal hex appears in any `*.module.css`
  outside `tokens.css` / `themes.css`.

### CSS file layout

```
web/src/styles/
  base.css      — minimal reset, fonts (existing stack
                  preserved), <body> sea gradient (Sprint 011
                  preserved verbatim), focus-visible defaults
  tokens.css    — :root semantic token names + sea palette +
                  spacing/radius/typography scales
  themes.css    — [data-palette="abyss|glass|sunlit"] rebinds
  motion.css    — [data-motion="living|minimal|full"] rules
  admin.css    — shared admin-shell composition (grid, sidebar
                  rail, canvas surface). Page-component styles
                  stay in their own *.module.css.
```

`web/src/styles/app.css` is **deleted** by Phase 5.

### Switcher dock and gating

```tsx
// web/src/admin/components/TriptychSwitcher.tsx
// Only renders when useDevFlags().ui_redesign_switcher === true.

<aside className={styles.dock} aria-label="Triptych design switcher">
  <Segmented label="Palette" value={palette} options={["abyss","glass","sunlit"]}
             onChange={setPalette} />
  <Segmented label="Layout"  value={layout}  options={["rail","spaces","canvas"]}
             onChange={setLayout} />
  <Segmented label="Motion"  value={motion}  options={["living","minimal","full"]}
             onChange={setMotion} />
  <button onClick={copyShareLink}>Copy ?triptych= link</button>
</aside>
```

Backend dev-flags handler returns `ui_redesign_switcher: true`
when the server is running in dev mode. Production builds see
the flag false, the dock unrendered, and one configured default
combination active.

## Implementation Plan

### Phase 1: Design Mode Infrastructure (~12%)

**Files:**
- `web/src/admin/design/types.ts` — Create. Mode unions +
  `DesignMode` type + allowlists.
- `web/src/admin/design/DesignModeProvider.tsx` — Create.
  Context provider; reads URL + localStorage; validates; writes
  `data-*` attrs to admin root.
- `web/src/admin/design/useDesignMode.ts` — Create. Public
  hook.
- `web/src/admin/Shell.tsx` — Wrap in provider; apply
  `data-palette/layout/motion` to `.admin-shell`.
- `web/src/styles/base.css` — Create. Reset + fonts +
  preserved `<body>` sea gradient + focus styles.
- `web/src/styles/tokens.css` — Create. Semantic token names +
  sea palette (Sprint 011 hex codes copied exactly) +
  spacing/typography scale.
- `web/src/main.tsx` — Import base.css + tokens.css; keep
  importing app.css until Phase 5.
- `web/src/lib/devFlags.ts` — Add `ui_redesign_switcher:
  boolean` to `DevFlags` type.
- `internal/httpapi/dev_flags_handlers.go` — Add the new flag,
  true when `s.DevInboxDir != ""` (dev-only proxy) OR when
  `s.Mode == "dev"` (whichever is exposed). The handler is
  small; extend the JSON.

**Tasks:**
- [ ] `--gradient-sea` value, hex stops, and
      `background-attachment: fixed` are copied character-for-
      character from the existing `app.css` Sprint 011 setup
      into `tokens.css` + `base.css`. Diff to verify.
- [ ] URL `?triptych=abyss,rail,living` deep-links a combo
      after allowlist validation; an invalid combo falls back
      to the default with a console warning.
- [ ] localStorage persistence with validation.
- [ ] `useDevFlags().ui_redesign_switcher` flag plumbed end-
      to-end (Go handler → JSON → TS type → consumer).
- [ ] No production rebuild required to silence the dock — the
      flag does it at runtime.

### Phase 2: Themes + Motion (~10%)

**Files:**
- `web/src/styles/themes.css` — Create. The three
  `[data-palette]` rebinds.
- `web/src/styles/motion.css` — Create. The three
  `[data-motion]` rules.
- `DESIGN.md` — Begin rewrite (palette + framework sections).

**Tasks:**
- [ ] Each of the three palettes assigns every semantic token
      defined in `tokens.css` — no gaps that would leak the
      default.
- [ ] `prefers-reduced-motion: reduce` forces effective
      `minimal` regardless of selected mode.
- [ ] WCAG AA contrast verified for body text under each
      palette (manual: open the running app at every palette,
      verify text on `--surface-panel` and chip text on each
      `--status-*-bg`).
- [ ] Glass palette: backdrop-filter fallback (plain
      semi-opaque surface) when the browser doesn't support
      it, so cards are still legible on Firefox/Safari edge
      cases.

### Phase 3: Component Library (~18%)

**Files:**
- `web/src/admin/components/{Page,PageHeader,Card,Section,Stat,Chip,Empty,DataTable,Field,FormSection,Button,Tabs,ActionBar}.tsx`
  + co-located `.module.css`.
- `web/src/admin/components/index.ts` — Barrel.

**Tasks:**
- [ ] Every component CSS module references only semantic
      tokens — `grep -E 'color:|background:|border-color:'
      web/src/admin/components/*.module.css | grep -v 'var('`
      returns nothing.
- [ ] `Chip` variants: `success | warning | error | info |
      neutral`, mapped to `--status-*-bg` and `--status-*`.
- [ ] `Stat` accepts a `tabular` prop that applies
      `font-variant-numeric: tabular-nums`; under `[data-
      motion="full"]`, mounts with a `requestAnimationFrame`
      count-up.
- [ ] `DataTable<Row>` types `Column<Row>` and supports
      `sortable`, `align`, `header`, `cell` per column.
- [ ] `Button` variants: `primary | secondary | quiet` plus a
      `loading` state.

### Phase 4: Three Nav Renderers + Canvas Landing (~15%)

**Files:**
- `web/src/admin/components/{RailNav,SpacesNav,CanvasNav,CommandBar,TodayCanvas,TriptychSwitcher}.tsx`
  + co-located `.module.css`.
- `web/src/admin/Shell.tsx` — Refactor to dispatch via
  `NavRenderer`. Behavior (useMe / role filter / signout /
  onboarding auto-show / RequireAdmin) stays here.
- `web/src/admin/pages/Overview.tsx` — Adds a layout-aware
  branch: under `canvas`, renders `<TodayCanvas>`. Under
  `rail` or `spaces`, renders the existing
  `AdminOverview` / `CruiseDirectorLanding` body.
- `web/src/styles/admin.css` — Shell grid composition for each
  layout.

**Tasks:**
- [ ] Sidebar label "Today" is set in the NAV constant; file
      and route remain `Overview.tsx` / `/admin`. Sprint 023's
      onboarding auto-show is verified to still fire.
- [ ] Role gating works in all three layouts. Cruise Director
      hitting `/admin/users` (an admin-only route) under any
      layout still gets bounced to `/admin` by `RequireAdmin`.
- [ ] `rail` collapses gracefully at ≤ 700px (rail rotates to
      top bar with overflow menu); `spaces` collapses to a
      56px rail at ≤ 900px; `canvas` collapses to a stacked
      card list at ≤ 900px.
- [ ] `TodayCanvas` reads only the existing Overview API
      payload (setup, exceptions, trip lists). It renders a
      spatial "today" surface — explicitly NOT a deck-plan
      requiring boat positions or geometry. If a richer
      spatial render is wanted in 026, the data layer can
      grow then.
- [ ] `CommandBar` opens on ⌘K / Ctrl+K, lists every
      `NavItem` from the role-filtered NAV, navigates on
      enter. Closes on Esc. Active in `rail` and `canvas`;
      hidden under `spaces`.
- [ ] `TriptychSwitcher` only mounts when
      `useDevFlags().ui_redesign_switcher`. Three segmented
      controls. Copy-link button puts an absolute URL with
      the current `?triptych=` query on the clipboard.

### Phase 5: Page Migrations + `app.css` Removal (~30%)

The 25 admin pages migrate to the component library. Page-
local CSS modules cover layout-only composition; no page
introduces color literals.

**The 25 pages:**

```
Account.tsx          AuditEvents.tsx       BoatCabins.tsx
BoatDetail.tsx       BoatTabs.tsx          Fleet.tsx
GuestFolio.tsx       Import.tsx            ImportJob.tsx
ImportLiveaboard.tsx ImportSpreadsheet.tsx Inventory.tsx
Onboarding.tsx       Organization.tsx      OrganizationPayments.tsx
OrganizationPricing.tsx Overview.tsx       Reports.tsx
TripCabins.tsx       TripConsumptionLedger.tsx
TripDashboard.tsx    TripGuestDetail.tsx   TripManifest.tsx
Trips.tsx            Users.tsx
```

Plus three admin-root primitives that should move into
`components/` or be wrapped:

```
AssignDirector.tsx   CurrencyPicker.tsx    UserMenu.tsx
```

**Files:**
- All 25 admin pages + the three primitives.
- `web/src/admin/components/*.module.css` — additions as
  pages reveal real shared patterns.
- `web/src/styles/app.css` — DELETED at the end of this phase.
- `web/src/main.tsx` — Remove the `app.css` import.

**Tasks:**
- [ ] Each page renders correctly under **every (palette ×
      layout) combination** — 9 combos per page. Motion modes
      are CSS-only and verified once across the suite, not
      per page.
- [ ] After the migration:
      `grep -rE 'admin-card|admin-page-header|admin-nav|chip--'
      web/src` returns zero matches.
- [ ] After the migration:
      `grep -rE '#[0-9a-fA-F]{3,6}' web/src/admin/components
      web/src/admin/pages` returns only matches that are in
      comments or are SVG asset literals (not CSS color
      values).
- [ ] `app.css` is deleted; its import removed from
      `main.tsx`.
- [ ] Existing route paths and data fetches unchanged.

### Phase 6: ADR + DESIGN.md + Verification (~15%)

**Files:**
- `docs/decisions/0004-triptych-runtime-evaluation.md` —
  New ADR.
- `DESIGN.md` — Rewrite end-to-end to describe the Triptych
  framework. Preserve and append the decisions log.
- `CLAUDE.md` — Add one line:
  > Web visual/layout changes go through
  > `web/src/admin/components/`; page files reference only
  > semantic tokens (`var(--surface-*)`, `var(--accent-*)`,
  > etc.), never raw hex.
- This file (`docs/sprints/SPRINT-025.md`) — DoD ticked.

**Tasks:**
- [ ] ADR 0004 captures: why three competing redesigns instead
      of one chosen direction; the semantic-token contract;
      the production-silent dev-flag gate; the explicit plan
      to collapse in Sprint 026.
- [ ] DESIGN.md decisions log: one new row for the bold-
      aesthetic reversal, one for the Triptych framework, one
      for the semantic-token contract.
- [ ] `make dev` boot: switcher visible at bottom-right, every
      combination reachable, role gating intact.
- [ ] `npm run build` clean. TypeScript clean.
- [ ] `go test ./...` clean (sanity check; only Go change is
      the dev-flag JSON addition).
- [ ] Implementer smoke matrix: every page rendered under
      `spaces × abyss × living` (the default), plus
      `rail × abyss` and `canvas × sunlit` on the
      Cruise-Director path (Today → Trips → Trip Dashboard →
      Trip Manifest → Trip Ledger).

## API Endpoints

| Endpoint | Method | Sprint 025 |
|---|---|---|
| `/api/dev/flags` | GET | Response gains `ui_redesign_switcher: bool`. Other flags unchanged. |

No other endpoint or response shape changes.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `web/src/admin/design/types.ts` | Create | Palette/Layout/Motion unions + allowlists. |
| `web/src/admin/design/DesignModeProvider.tsx` | Create | Context provider; URL + localStorage + DOM attr writes. |
| `web/src/admin/design/useDesignMode.ts` | Create | Public hook. |
| `web/src/admin/components/Page.tsx` + `.module.css` | Create | Page shell primitive. |
| `web/src/admin/components/PageHeader.tsx` + `.module.css` | Create | Title/subtitle/actions row. |
| `web/src/admin/components/Card.tsx` + `.module.css` | Create | Surface primitive. |
| `web/src/admin/components/Section.tsx` + `.module.css` | Create | Within-card grouping. |
| `web/src/admin/components/Stat.tsx` + `.module.css` | Create | Label/value with count-up under `full`. |
| `web/src/admin/components/Chip.tsx` + `.module.css` | Create | Semantic chip. |
| `web/src/admin/components/Empty.tsx` + `.module.css` | Create | Empty-state. |
| `web/src/admin/components/DataTable.tsx` + `.module.css` | Create | Typed-column table wrapper. |
| `web/src/admin/components/Field.tsx` + `.module.css` | Create | Form field row. |
| `web/src/admin/components/FormSection.tsx` + `.module.css` | Create | Grouped form section. |
| `web/src/admin/components/Button.tsx` + `.module.css` | Create | Accent / secondary / quiet. |
| `web/src/admin/components/Tabs.tsx` + `.module.css` | Create | Tabbed surface. |
| `web/src/admin/components/ActionBar.tsx` + `.module.css` | Create | Sticky action toolbar. |
| `web/src/admin/components/RailNav.tsx` + `.module.css` | Create | Rail layout nav renderer. |
| `web/src/admin/components/SpacesNav.tsx` + `.module.css` | Create | Spaces layout nav renderer. |
| `web/src/admin/components/CanvasNav.tsx` + `.module.css` | Create | Canvas layout top-bar nav. |
| `web/src/admin/components/CommandBar.tsx` + `.module.css` | Create | ⌘K route launcher (rail + canvas). |
| `web/src/admin/components/TodayCanvas.tsx` + `.module.css` | Create | Spatial landing for canvas layout. |
| `web/src/admin/components/TriptychSwitcher.tsx` + `.module.css` | Create | Floating bottom-right dock. |
| `web/src/admin/components/index.ts` | Create | Barrel. |
| `web/src/admin/Shell.tsx` | Modify | Wrap in DesignModeProvider; dispatch nav renderer; keep all behavioral hooks. |
| `web/src/admin/pages/Overview.tsx` | Modify | Layout-aware branch for canvas. File + route unchanged. |
| `web/src/admin/pages/*.tsx` (24 others) | Modify | Migrate to component library + page-local module CSS. |
| `web/src/admin/{AssignDirector,CurrencyPicker,UserMenu}.tsx` | Modify | Align with new component primitives. |
| `web/src/pages/*.tsx` | Modify | Auth shell uses palette tokens; hero treatment under `full` motion (still image, with optional video). |
| `web/src/lib/devFlags.ts` | Modify | Add `ui_redesign_switcher: bool`. |
| `web/src/main.tsx` | Modify | Import base/tokens/themes/motion/admin.css; remove app.css at end of Phase 5. |
| `web/index.html` | (Unchanged) | Existing font stack preserved. |
| `web/src/styles/base.css` | Create | Reset + fonts + preserved Sprint 011 sea gradient. |
| `web/src/styles/tokens.css` | Create | Semantic tokens + sea palette + scales. |
| `web/src/styles/themes.css` | Create | The three palette rebinds. |
| `web/src/styles/motion.css` | Create | The three motion modes. |
| `web/src/styles/admin.css` | Create | Shell composition. |
| `web/src/styles/app.css` | Delete (Phase 5) | Replaced by the above. |
| `internal/httpapi/dev_flags_handlers.go` | Modify | Add `ui_redesign_switcher` in the JSON. |
| `DESIGN.md` | Rewrite | Triptych framework + decisions log. |
| `docs/decisions/0004-triptych-runtime-evaluation.md` | Create | ADR. |
| `CLAUDE.md` | Modify | One-line "components-first + semantic tokens" pointer. |

## Definition of Done

- [ ] `<body>` sea gradient is byte-identical to Sprint 011:
      same hex stops, same `background-attachment: fixed`, no
      theme rebinds it.
- [ ] `web/src/styles/app.css` is deleted and not imported.
- [ ] `grep -rE 'admin-card|admin-page-header|admin-nav|chip--'
      web/src` returns zero matches.
- [ ] `grep -rE '#[0-9a-fA-F]{3,6}' web/src/admin/components
      web/src/admin/pages` returns no CSS color literals
      (matches in SVG / comments allowed).
- [ ] `web/src/admin/components/` contains every component
      listed in the Files Summary, with co-located CSS
      modules, and is exported via `index.ts`.
- [ ] `useDevFlags().ui_redesign_switcher` is plumbed end-to-
      end. Set to true in `dev` mode and false in `test` /
      `production`.
- [ ] Floating switcher renders at bottom-right of every
      admin page when the flag is true; absent when false.
      Three segmented controls (palette / layout / motion),
      each with 3 options.
- [ ] Selection persists across reloads via `localStorage`.
      Invalid persisted values fall back to defaults.
- [ ] `?triptych=palette,layout,motion` URL deep-links a combo
      after allowlist validation; malformed values fall back
      to defaults.
- [ ] All three palettes (`abyss`, `glass`, `sunlit`) rebind
      every semantic token in `tokens.css`; no gaps.
- [ ] All three layouts (`rail`, `spaces`, `canvas`) render
      every admin page without console errors under every
      palette.
- [ ] All three motion modes (`living`, `minimal`, `full`)
      render without console errors and respect
      `prefers-reduced-motion`.
- [ ] One behavioral `AdminShell`. Role gating, sign-out,
      onboarding auto-show, and `RequireAdmin` work
      identically across all three layouts.
- [ ] Sprint 023 onboarding auto-show (route check on
      `/admin`) still fires correctly under all three
      layouts.
- [ ] Auth pages (`Login`, `Signup`, `Verify`, `ForgotPassword`,
      `ResetPassword`, `AcceptInvitation`, `GuestInvitation`,
      `GuestRegistration`, `GuestTab`) use the new tokens
      and look coherent under at least `abyss` and `sunlit`.
- [ ] `DESIGN.md` is rewritten with the Triptych framework;
      decisions log preserved + appended.
- [ ] `docs/decisions/0004-triptych-runtime-evaluation.md`
      exists and documents the Sprint 026 collapse plan.
- [ ] `CLAUDE.md` mentions the component library and the
      semantic-token rule.
- [ ] `npm run build` from `web/` is clean. TypeScript
      passes. `go test ./...` and `go vet ./...` are clean.
- [ ] Implementer smoke matrix:
      - Every admin page rendered under `spaces × abyss ×
        living` (the default).
      - Cruise-Director path (Today → Trips → Trip Dashboard
        → Trip Manifest → Trip Ledger) rendered under
        `rail × abyss` and `canvas × sunlit`.
      - Reports + Audit + Payments + Pricing rendered under
        `spaces × glass` (glass contrast spot-check).

## Documentation Manifest

The implementation sprint MUST land the following docs
changes alongside the code. Phase 6 verifies each file in
this list was modified before marking the sprint complete.

### New ADRs

- `docs/decisions/0004-triptych-runtime-evaluation.md` —
  Codifies the "ship three competing redesigns behind a
  runtime switcher, judge in the live app, collapse to one in
  Sprint 026" framework, the semantic-token contract, the
  production-silent dev-flag gate, and the collapse
  procedure.

### Amended ADRs

- None. (`0001-auth-provider`, `0002-revert-clerk`,
  `0003-reporting-storage` are unrelated.)

### Cross-cutting docs

- `DESIGN.md` — Full rewrite. New sections: Triptych
  framework, three palettes (abyss/glass/sunlit) with token
  tables, three layouts (rail/spaces/canvas), three motion
  modes (living/minimal/full), semantic-token contract,
  component-library policy. Existing decisions log preserved
  and appended with rows for the bold-aesthetic reversal, the
  Triptych framework, and the semantic-token contract.
- `CLAUDE.md` — Add one line under Development Rules:
  "Web visual/layout changes go through
  `web/src/admin/components/`; page files reference only
  semantic tokens (`var(--surface-*)`, `var(--accent-*)`),
  never raw hex."

### Skipped (with reasoning)

- `docs/product/*` — Personas and user stories don't change
  with a visual redesign; product surface is unchanged.
- `docs/sprints/README.md` — Sprint workflow doesn't change.
- `docs/dev/email-inbox.md` — Unrelated.
- `docs/CONFIG.md` — No new config keys. The dev-flag JSON
  addition is documented in the ADR + DESIGN.md, not here.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| 27-combo evaluation is overwhelming; user can't pick a winner | Medium | Sprint 026 delays | ADR ships a 3×3 palette × layout matrix screenshot as a decision aid; the switcher is debug-gated, so the loop "ship, look, refine" can run as long as needed without production fallout. |
| 25-page migration is large | High | Schedule slip | One canonical NAV + semantic tokens + thin component primitives mean adding a theme is one file edit, not 25. Page migrations are repetitive and reviewable in batches. |
| `canvas` layout is the heaviest single lift | High | Could push the sprint | Scope `TodayCanvas` to the existing Overview API only; spatial idiom uses CSS grid + simple SVG, not a real deck-plan with boat geometry. If still too heavy, drop animated arcs but keep the layout. |
| Glass palette + low contrast fails accessibility | Medium | Forms unreadable in glass | Phase 2 verifies WCAG AA per palette; if glass fails, raise body text to `#0E1726` and tighten card opacity (already in spec); browser without backdrop-filter falls back to plain semi-opaque surface. |
| Bioluminescent accent in `abyss` reads as neon / kids-game | Medium | "Looks wrong" for B2B | Use the cyan accent for CTAs + active states only, paired with a soft glow rather than hard fills; warm coral `--accent-selection` provides a counterweight. |
| Three nav renderers diverge on role-gating | Medium | Authorization regression | Avoided by design — role filter runs ONCE in `AdminShell`; nav renderers receive the filtered list and only render markup. Backend route guards remain the security boundary regardless. |
| Caustics overlay tanks performance on low-end devices | Low | Janky scrolling | `[data-motion="living"]` overlay is a pseudo-element on body with `will-change: transform` and a 90s cycle; auto-suppressed under `prefers-reduced-motion`. |
| Switcher accidentally ships to production | Low | Confused real users | Gated by `useDevFlags().ui_redesign_switcher`, sourced from the backend. The backend handler returns false in `test` and `production` mode. Verified in Phase 1 task. |
| Sprint 026 cleanup is painful | Low | Slow follow-up | Token files are isolated; nav renderers are isolated React components; motion is isolated CSS. ADR documents the collapse procedure file-by-file. |

## Security Considerations

- Presentation-layer sprint. No new attack surface.
- Role gating (`useMe` + `RequireAdmin` + backend route guards)
  is **unchanged** and lives in `AdminShell`. Nav renderers
  receive only the already-filtered NAV.
- `localStorage` values are untrusted: palette / layout /
  motion strings are validated against typed allowlists before
  reflecting into DOM `data-*` attributes.
- `?triptych=` URL query is parsed and validated against the
  same allowlists — no user-supplied content is reflected as
  HTML.
- The dev-flag gate (`useDevFlags().ui_redesign_switcher`)
  uses the same path as the existing `filesystem_email`
  affordance; backend ensures it's false in production.
- No new external CSS / JS dependencies. The existing font
  stack is preserved. If a hero video for `full` motion is
  later added, it ships as a bundled static asset, not a
  third-party load.

## Dependencies

- **Sprint 011** (body sea gradient) — preserved verbatim.
- **Sprint 008** (admin chrome) — replaced.
- **Sprint 023** (onboarding wizard) — its `/admin` auto-show
  hook is preserved by keeping the route stable.
- **Sprint 021** (dev/inbox + dev flags pattern) — extended.
- No external blockers.

## Open Questions

These can be resolved during implementation without changing
the sprint's commitments:

1. **Underwater hero asset for `full` motion.** If the user
   provides a CC0 underwater clip, it bundles as a static
   asset and `full` mounts it; otherwise `full` uses the
   same still hero image as `living`. No DoD requirement.
2. **Default combination on first run** when no localStorage
   and no `?triptych=`. Plan defaults to
   `abyss / spaces / living` — the bold pick that still
   sits in the most familiar layout. Reversible.
3. **`/dev/components` route** for visual review. Optional
   in this sprint — pages alone are enough to evaluate.
   Adding it later is one tsx file + one route line.

## References

- `docs/sprints/drafts/SPRINT-025-INTENT.md`
- `docs/sprints/drafts/SPRINT-025-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-025-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-025-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-025-MERGE-NOTES.md`
- `DESIGN.md`
- `CLAUDE.md`
- `web/src/admin/Shell.tsx`
- `web/src/lib/devFlags.ts`
- `internal/httpapi/dev_flags_handlers.go`
