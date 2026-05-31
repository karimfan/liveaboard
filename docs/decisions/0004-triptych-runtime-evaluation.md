# 0004 — Triptych Runtime UI Evaluation

**Status:** Accepted (Sprint 025) — partial implementation
**Date:** 2026-05-30

## Context

The admin app's chrome — warm-slate working surfaces + amber
accent + a sea-blue body gradient (Sprint 011) — has accreted
since Sprint 008 without a holistic visual review. Operators
running scuba liveaboard charters in Raja Ampat, the
Galapagos, and the Maldives are an adventurous audience, and
the current "accounting tool over a beach photo" pairing
underserves them.

The user wanted a redesign but didn't want to pre-commit to a
direction from mockups. Picking between dark-and-luminous,
glass-and-translucent, and saturated-and-light from a Figma
file is hard; picking between them in the live app is easy.

## Decision

Sprint 025 ships **three competing redesigns side-by-side**
behind a runtime switcher, so the choice happens in the live
app. Three axes, three options each:

| Axis | Options |
|---|---|
| **Palette** | `reef` (warm cream + hot coral + electric magenta + turquoise), `harbor` (sunset cream + coral + plum + gold), `midnight` (deep plum + magenta-pink + electric turquoise + ember) |
| **Layout** | `rail` (56-64px icon rail + ⌘K command bar), `spaces` (three labeled sidebar groups), `canvas` (spatial top-bar with TodayCanvas landing) |
| **Motion** | `living` (caustic overlay + ripple), `minimal`, `full` (everything + page-slide + count-up) |

**Palette round 1 was rejected** — abyss (navy + cyan-green),
glass (translucent + backdrop-blur), and sunlit (coral on
white) all read as safe SaaS rather than adventurous. Round 2
(above) drops navy and cyan-as-primary, drops translucent
glass entirely, and leans into the warm-saturated colors a
diver actually sees: tropical-fish-in-sunlight (reef),
sunset-on-the-boat-deck (harbor), and a night dive with
bioluminescent plankton against deep plum (midnight).

3 × 3 × 3 = 27 combinations. A floating bottom-right dock
(`TriptychSwitcher`) exposes three segmented controls.
Selection persists to `localStorage`, can be deep-linked via
`?triptych=palette,layout,motion`, and reflects to `<html>`
as `data-palette` / `data-layout` / `data-motion` attributes
that CSS variable rebinds consume.

The dock is gated behind a server-driven dev flag
(`useDevFlags().ui_redesign_switcher`, true only when the
backend reports `cfg.Mode == ModeDev`). Production builds see
the dock disabled and one configured default combination
(`abyss / spaces / living`).

## Semantic Token Contract

Components read **only** semantic tokens — never raw hex:

- Surfaces: `--surface-app`, `--surface-panel`,
  `--surface-panel-strong`, `--surface-raised`
- Text: `--text-primary`, `--text-muted`, `--text-inverse`
- Borders: `--border-subtle`, `--border-strong`
- Accents: `--accent-primary`, `--accent-primary-hover`,
  `--accent-primary-subtle`, `--accent-secondary`,
  `--accent-selection`
- Status: `--status-(success|warning|error|info)` + `-bg`
- Focus / shadow: `--focus-ring`, `--shadow-panel`,
  `--shadow-elevated`
- Glass-only opt-in: `--backdrop-blur`

`themes.css` rebinds every semantic token per `[data-palette]`.
Adding a fourth palette is one CSS block; removing one is
deleting one CSS block. The component library never changes.

## Body Sea Gradient — Verbatim

The Sprint 011 body composite (white wash + scuba-diver
photograph + sea gradient at the bottom, all
`background-attachment: fixed`) is preserved character-for-
character. No theme rebinds it. The sea palette tokens
(`--c-sea-*`) and `--gradient-sea` are untouched. Themes
change only the working-surface tokens that sit above the
gradient.

## Implementation Scope (Sprint 025)

**Shipped:**
- Backend `ui_redesign_switcher` flag on `/api/dev/flags`,
  sourced from `Server.IsDev` (= `cfg.Mode == ModeDev`).
- `web/src/admin/design/` runtime mode layer
  (`DesignModeProvider`, `useDesignMode`, typed allowlists,
  URL + localStorage backed).
- Semantic-token extension to `tokens.css`.
- `themes.css` — three palette rebinds.
- `motion.css` — three motion modes with
  `prefers-reduced-motion` always winning.
- 13-component library (`web/src/admin/components/`):
  Page, PageHeader, Card, Section, Stat, Chip, Empty,
  Button, Field, FormSection, Tabs, ActionBar, DataTable.
- 6 layout components: SpacesNav, RailNav, CanvasNav,
  CommandBar, TodayCanvas, TriptychSwitcher.
- Canonical `nav.ts` with role-aware filter + flatten.
- `Shell.tsx` refactored to ONE behavioral owner with three
  layout renderers underneath.
- `admin.css` — per-layout grid composition.

**Deferred (page-by-page migration):**
- All 25 admin pages still consume `app.css` legacy classes
  (`.admin-card`, `.admin-page-header`, `.chip--*`, etc.).
  This means the existing chrome on those pages looks the
  same regardless of selected palette — only the body, the
  shell sidebar/rail/canvas, and the canvas landing respond
  to palette flips. Motion modes (caustics) apply globally.
- `app.css` is NOT deleted. Removing it would break every
  unmigrated page.
- A follow-up sprint (provisionally Sprint 026 Phase A)
  performs the migration.

## Collapse Procedure (Sprint 026)

Once the user picks a winning combination:

1. Edit `themes.css` to remove the two non-winning palette
   rebinds (keep the winner in `:root` to make it the
   default instead of in a `[data-palette]` block).
2. Delete the two non-winning nav renderers from
   `web/src/admin/components/`.
3. Edit `Shell.tsx` to drop the dispatch — render the winning
   nav unconditionally.
4. Edit `motion.css` to remove the two non-winning motion
   modes (keep the winner under `:root`).
5. Delete `nav.ts` exports for the dropped layouts (glyphs,
   etc.).
6. Delete `TriptychSwitcher.tsx` + `useDesignMode` write
   methods (the read hook can stay for telemetry).
7. Delete the `ui_redesign_switcher` flag from
   `dev_flags_handlers.go` and `devFlags.ts`.
8. Amend this ADR with the winner.

Token files are isolated; layout components are isolated; the
collapse is a few dozen file edits, not a rewrite.

## Risks Accepted

- **Three layouts can drift on role gating.** Mitigated by
  centralizing role filter + behavior in `AdminShell`; each
  renderer is presentation-only.
- **`canvas` layout's landing is currently a spatial card
  grid, not a real deck plan.** Mitigated by scoping
  `TodayCanvas` to the existing Overview API only — no
  backend changes. A future sprint can grow boat-plan
  rendering when a Fleet endpoint exposes the geometry.
- **Two palettes look bad in production** if the dev-flag
  defaults flip incorrectly. Mitigated by sourcing
  `ui_redesign_switcher` from `cfg.Mode == ModeDev` on the
  backend — production cannot fall into dev mode.

## References

- `docs/sprints/SPRINT-025.md` — full sprint plan
- `docs/sprints/drafts/SPRINT-025-*.md` — planning artifacts
- `DESIGN.md` — design-system decisions log
- `web/src/admin/design/` — runtime mode layer
- `web/src/admin/components/` — component library + layouts
- `web/src/styles/{tokens,themes,motion,admin}.css` —
  the four foundation stylesheets that survive `app.css`
  teardown
