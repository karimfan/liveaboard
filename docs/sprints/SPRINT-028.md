# Sprint 028: One UI — One Palette, Sea Buttons, Enforced

**Status:** completed (2026-06-02)

## Outcome

Shipped on `main`:

- **Sea buttons** — `--btn-sea-*` family (solid aquamarine default, sea
  gradient, turquoise→mint) on the `Button` primitive; the old amber and
  cyan→violet button styling is gone.
- **One dark-sea palette across the admin surface** — all 24 admin pages
  migrated off `app.css` global classes onto component primitives +
  co-located CSS modules; a `.admin-main` token bridge covers the
  remaining shared helpers. Onboarding contrast fixed.
- **DESIGN.md trued-up** — Inter + JetBrains Mono typography, dark-sea
  color, sea buttons, and an Enforcement section.
- **Enforced** — Stylelint (`color-no-hex` + custom
  `liveaboard/font-allowlist`) wired into `make lint` and a new
  `.github/workflows/ci.yml`.
- **ADR 0006** created; ADR 0005 amended; CLAUDE.md / CODEX.md updated.

**Carried to Sprint 029:** `app.css` still serves the light public/guest
pages (`GuestTab`, `GuestRegistration`) and the `AssignDirector` /
`CurrencyPicker` helper classes (bridged for color); migrate those, then
delete `app.css`. Human click-through QA of the running admin app is the
recommended final check (this sprint was verified via build + lint +
structural review, not a logged-in pass).

## Overview

The seed: "make the UI consistent — same fonts throughout, same color
palette; some pages differ" plus "all buttons marine blue, like the sea
— turquoise / aquamarine."

Grounding on the actual code (not the first-draft assumptions) shows the
real problem is **two token worlds rendering at once**, not missing
fonts or raw hex:

- The **cockpit and Sprint-026-migrated pages** read the *semantic*
  tokens (`--surface-*`, `--text-*`, `--accent-*`), which `themes.css`
  rebinds to the **dark manta-night** palette (navy glass, cyan accent,
  light text).
- The **legacy pages** (Trips, Fleet, Inventory, Users, Organization,
  Reports, Onboarding, Trip*, Boat*) are styled by `web/src/styles/
  app.css`, which reads the raw **`--c-*`** palette directly
  (near-black text, amber links/buttons, white cards). `themes.css` does
  not rebind `--c-*`, so these pages stay in the original **light
  warm-slate + amber** world.

That split is exactly what the user sees: the Today/cockpit screen is
dark cyan glass; click into Trips and it becomes light slate with amber
links and near-black buttons. Buttons are doubly inconsistent — the
`Button` primitive uses a cyan→violet gradient while `app.css` buttons
are near-black/amber.

Sprint 028 makes the whole app one product: **collapse onto a single
semantic-token palette**, retire the divergent `--c-*` styling in
`app.css`, recolor **all buttons to the sea** (three turquoise/aquamarine
variants), make `DESIGN.md` tell the truth about the fonts and palette
that are actually shipping, and add a **Stylelint-in-CI guardrail** so
the split can't silently return. No new features, no IA changes, no data
or endpoint changes — behavior-preserving throughout.

Note on fonts: the three families already load via CDN links in
`index.html` (Inter, DM Sans, JetBrains Mono, General Sans); the font
problem is a *stale contract* (`DESIGN.md` documents General Sans / DM
Sans / Geist while `tokens.css` binds Inter), not a missing asset. This
sprint reconciles the contract rather than re-plumbing font loading.

## Use Cases

1. **One palette end to end.** Moving cockpit → Trips → Fleet → Reports →
   Organization shows the same surfaces, text colors, accents, chips,
   tables, and forms. No page reads like a different app.
2. **Sea buttons everywhere.** Every primary action is a
   turquoise/aquamarine sea button (three selectable variants), never the
   old amber or cyan→violet. Secondary/quiet buttons are consistent too.
3. **One typography contract.** `DESIGN.md`, `tokens.css`, and the loaded
   fonts agree; every surface uses the same display/body/mono roles.
4. **No silent regression.** A new page that hardcodes a hex or names an
   off-contract font fails Stylelint in CI before it can merge.
5. **Legacy CSS is gone, safely.** `app.css` is deleted once nothing
   imports the styles it provided — proven by grep, not assumed.

## Non-Goals

- New product features, routes, IA, copy, data model, or endpoints.
- Re-plumbing font *loading* (fonts already load; only the contract is
  reconciled). Self-hosting fonts is optional, not required.
- A full visual-regression / screenshot-diff harness (deferred to 029).
  This sprint relies on per-route QA smoke for both roles.

## Architecture

### Current (two worlds)

```
tokens.css :root
  ├─ --c-*  (warm slate + amber, LIGHT)         ← never rebound
  └─ --surface-*/--text-*/--accent-* (defaults, light)
themes.css :root  ── rebinds semantic tokens → manta-night (DARK sea/cyan)

cockpit + 5 migrated pages ─► semantic tokens ─► DARK  ✓
legacy app.css pages       ─► raw --c-* + app.css ─► LIGHT amber  ✗
buttons: Button primitive = cyan→violet gradient ; app.css = black/amber
```

### Target (one world)

```
tokens.css   = base palette + semantic NAMES + sea button tokens
themes.css   = the ONE production binding (dark sea / manta-night)
  → every admin + funnel/guest page reads semantic tokens only
  → --c-* legacy palette no longer styles any page

buttons: ONE sea family (3 variants) used by Button primitive AND
         anywhere a raw <button> survives:
  • solid aquamarine   #2EB6E5 → hover #0E95CB   (default primary)
  • sea gradient       #6DCEF0 → #0A76A6
  • turquoise → mint   #2EB6E5 → #00FFD1
  navy (#020617) label on all three.
--sidebar-active-bg, --accent-primary, --focus-ring → sea.

app.css: DELETED once grep proves zero consumers.
guardrail: stylelint (custom no-raw-hex + font-allowlist) in CI.
```

### Unification direction (CONFIRMED)

Unification collapses onto the **cockpit's dark sea palette** — legacy
pages move *toward* the cockpit, not the cockpit toward them. This
follows Sprint 027 / ADR 0005 and the CLAUDE.md rule that new admin UI
follows the Voyage Cockpit contract. **Confirmed by the user
(2026-05-31).** The light-sea alternative was considered and rejected;
should it ever be revisited, it is only a different `themes.css` binding
(semantic token names do not change).

### Sea button family

The `Button` primitive (`web/src/admin/components/Button.tsx` +
`.module.css`) gains three primary-weight sea variants (the user kept all
three as selectable). The legacy `app.css` `<button>` / `button.primary`
rules are folded into the same family during migration, then removed.
`--sidebar-active-bg` is rebound from cyan→violet to a sea gradient so
the active nav item, primary buttons, links, and focus ring all read sea.

### Token ownership & Stylelint

- Raw hex lives **only** in `tokens.css` (palette) and `themes.css`
  (bindings) and explicitly-approved asset/background rules.
- Pages and components read semantic tokens (`--surface-*`, `--text-*`,
  `--accent-*`, `--border-*`, `--status-*`, `--focus-ring`,
  `--shadow-*`) and `var(--font-*)` only.
- Custom rules `liveaboard/no-raw-hex` and `liveaboard/font-allowlist`
  enforce this on `web/src/**` CSS, exempting the two token files.

### Scope of migration

All admin pages **and** the public funnel + guest pages (per the
interview), plus any **auth** page that still imports `app.css` — because
`app.css` can only be deleted once it has no consumers. Route/file
inventory is taken from `web/src/main.tsx`, not memory.

## Implementation Plan

### Phase 1: Palette unify + contract truth-up (~15%)

**Files:** `web/src/styles/tokens.css`, `web/src/styles/themes.css`,
`web/src/styles/base.css`, `DESIGN.md`

**Tasks:**
- [ ] Decide and document the one production palette (dark sea /
      manta-night) as the binding in `themes.css`; ensure `--c-*` is no
      longer relied on for page styling.
- [ ] Add sea button tokens; rebind `--sidebar-active-bg`,
      `--accent-primary`, `--accent-primary-hover`, `--focus-ring` to sea.
- [ ] Reconcile `DESIGN.md` Typography (Inter display/body, JetBrains
      Mono data) and Color (manta-night dark sea + sea buttons) with the
      shipped tokens; correct the stale General Sans/DM Sans/Geist + amber
      sections; append a decisions-log row.
- [ ] Confirm `base.css` body font is `var(--font-body)` once; keep the
      background composite intact.

### Phase 2: Sea button family + primitives audit (~15%)

**Files:** `web/src/admin/components/Button.tsx` + `.module.css`,
other `web/src/admin/components/*` + modules, `web/src/styles/admin.css`

**Tasks:**
- [ ] Add three sea primary variants to `Button` (solid /
      gradient / electric); navy label; sea glow shadow.
- [ ] Audit every primitive (Page, PageHeader, Card, DataTable, Chip,
      Field, FormSection, ActionBar, Tabs, Empty, Stat) for raw hex,
      hardcoded fonts, and correct semantic-token use; extend variants
      where page migration will need them.
- [ ] Align `SpacesNav` / Shell chrome and admin helpers
      (`AssignDirector`, `CurrencyPicker`, `UserMenu`) to the same tokens.

### Phase 3: Migrate admin run-loop pages (~20%)

**Files:** `Trips`, `TripDashboard`, `TripManifest`,
`TripConsumptionLedger`, `TripCabins`, `TripGuestDetail`, `GuestFolio`
(+ page-local `*.module.css` as needed)

**Tasks:**
- [ ] Replace `app.css` global classes (`admin-page-*`, `admin-card*`,
      `chip--*`, `rate-chip--*`, `admin-table`, `admin-toolbar`,
      `setup-list__*`) with primitives or token-only modules.
- [ ] Amounts/timestamps/IDs → `var(--font-mono)`, right-aligned.
- [ ] Behavior, API calls, forms, mutations unchanged; one page/commit;
      remove the dead `app.css` rules as each page lands.

### Phase 4: Migrate config + support + funnel/guest pages (~30%)

**Files:** config — `Fleet`, `BoatDetail`, `BoatTabs`, `BoatCabins`,
`Inventory`, `Users`, `Organization`, `OrganizationPayments`,
`OrganizationPricing`; support — `Overview`, `AuditEvents`, `Reports`,
`Onboarding`, `Account`, `Import`, `ImportLiveaboard`,
`ImportSpreadsheet`, `ImportJob`; public/guest — funnel/catalog, guest
portal, guest registration/tab, and any **auth** page importing
`app.css` (inventory from `main.tsx`).

**Tasks:**
- [ ] Migrate each off `app.css`/`--c-*` onto semantic tokens +
      primitives; preserve `RequireAdmin` and guest-session boundaries.
- [ ] Bring public funnel/guest pages onto the same palette (in scope per
      interview); keep their layouts, just unify the design language.
- [ ] Maintain a running "rules removed from app.css" tally toward zero
      consumers.

### Phase 5: Delete app.css + Stylelint/CI guardrail (~15%)

**Files:** `web/src/styles/app.css` (delete), `web/src/main.tsx`,
`web/src/styles/*.backup*` / `*.disabled` (delete), `web/package.json`,
`web/stylelint.config.cjs`, `web/stylelint-rules/{no-raw-hex,font-allowlist}.cjs`,
`.github/workflows/ci.yml` (or checked-in equivalent), `Makefile`

**Tasks:**
- [ ] grep-prove no import/class consumer of `app.css` remains; delete it
      and its `main.tsx` import; delete dead `.backup`/`.disabled` sheets.
- [ ] Add Stylelint + custom rules + `lint:styles` script; exempt
      `tokens.css`/`themes.css`; fix any stragglers.
- [ ] Add a CI workflow running `npm ci && npm run build && npm run
      lint:styles` plus `make lint && make test`.

### Phase 6: Verification + close (~5%)

**Tasks:**
- [ ] `cd web && npm run build`, `npm run lint:styles`; `make lint`,
      `make test` (document any local PostgreSQL skips).
- [ ] Click-through smoke of every `/admin` route for Org Admin and the
      Cruise-Director-allowed subset; confirm no behavior/layout
      regression and no admin-only route leaks to directors.
- [ ] Smoke funnel/guest/auth pages for the unified look.
- [ ] Update tracker + Documentation Manifest.

## API Endpoints

None added or changed.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `web/src/styles/tokens.css` | Modify | Sea button tokens; semantic names; palette source |
| `web/src/styles/themes.css` | Modify | One production binding (dark sea); sea accents |
| `web/src/styles/base.css` | Modify | Body font once; keep background composite |
| `web/src/styles/admin.css` | Modify | Keep only true shell overrides |
| `web/src/styles/app.css` | Delete | Retired once no consumer remains |
| `web/src/styles/*.backup*`, `*.disabled` | Delete | Dead sheets |
| `web/src/admin/components/Button.*` | Modify | Three sea primary variants |
| `web/src/admin/components/*` | Modify | Token/variant audit |
| `web/src/admin/pages/*.tsx` (+ `*.module.css`) | Modify/Create | Migrate to tokens/primitives |
| `web/src/pages/*` (funnel/guest/auth as needed) | Modify | Unify palette; drop `app.css` |
| `web/src/main.tsx` | Modify | Drop `app.css` import; load order |
| `web/package.json` | Modify | Stylelint dep + `lint:styles` |
| `web/stylelint.config.cjs`, `web/stylelint-rules/*.cjs` | Create | no-raw-hex + font-allowlist |
| `.github/workflows/ci.yml` | Create | Run build + lint:styles + make test |
| `Makefile` | Modify | Wire style lint into lint path |
| `DESIGN.md` | Modify | Truth-up typography + color; sea buttons; enforcement |
| `CLAUDE.md`, `CODEX.md` | Modify | Token-only + sea-button + no-`app.css` guidance |
| `docs/decisions/0006-ui-consistency-tokens-fonts.md` | Create | ADR (see Manifest) |
| `docs/decisions/0005-voyage-cockpit-reboot.md` | Modify | Note migration completed |

## Definition of Done

- [ ] No admin/funnel/guest page reads the legacy `--c-*` palette or
      `app.css` classes; all use semantic tokens + primitives/modules.
- [ ] `web/src/styles/app.css` is deleted; no import/class consumer
      remains (`rg -n "app.css|admin-page-|admin-card|setup-list__|chip--|rate-chip--" web/src` clean of real deps).
- [ ] Every button is a sea variant; no amber (`--c-primary`) or
      cyan→violet button remains; the three sea variants are selectable
      via `Button`.
- [ ] `DESIGN.md` typography + color sections match the shipped tokens
      (Inter; dark sea palette; sea buttons); decisions log updated.
- [ ] No raw hex outside `tokens.css`/`themes.css`; no off-contract
      `font-family`; `lint:styles` passes.
- [ ] Stylelint runs in CI alongside build + `make test`.
- [ ] `cd web && npm run build` passes; `make lint`/`make test` pass (or
      documented PostgreSQL skips).
- [ ] Both-role click-through smoke shows one consistent UI and no route
      leakage.
- [ ] Documentation Manifest complete.

## Security Considerations

- Presentation-only: no change to auth, tenant scoping, sessions, role
  filtering, or route guards. `RequireAdmin` and guest-session scoping
  preserved; broad page edits smoke-tested for both roles so no hidden
  control leaks.
- Stylelint custom rules inspect CSS source only; no untrusted execution.
- CI style checks need no secrets.

## Dependencies

- Sprint 027 (cockpit, token collapse, single shell) — complete.
- Existing component library; Stylelint (new dev dep). No backend deps,
  no migrations.

## Documentation Manifest

The implementation sprint MUST land the following docs changes alongside
the code. The `sprint` skill verifies each file in this list was modified
before marking the sprint complete.

### New ADRs

- `docs/decisions/0006-ui-consistency-tokens-fonts.md` — Codifies: one
  production semantic-token palette (dark sea) for the whole app; the
  `--c-*` legacy palette retired from page styling; the three sea button
  variants and navy label; components/pages read semantic tokens only
  (no raw hex); Stylelint (`no-raw-hex` + `font-allowlist`) enforced in
  CI; `app.css` deleted.

### Amended ADRs

- `docs/decisions/0005-voyage-cockpit-reboot.md` — Note the deferred page
  migration is complete: all admin + funnel/guest pages now share the
  cockpit's semantic palette, buttons are sea, and `app.css` is deleted.

### Cross-cutting docs

- `DESIGN.md` — Reconcile Typography (Inter roles) and Color (dark sea
  palette + sea buttons) with shipped tokens; add an Enforcement section
  (Stylelint rules); decisions-log row.
- `CLAUDE.md` — Tighten the design guardrail: all pages (not just new
  admin UI) read semantic tokens + primitives; sea button family; no
  `app.css`; raw hex only in token files.
- `CODEX.md` — Mirror the same frontend guidance.

### Skipped (with reasoning)

- `current_status.md` — no entity/capability change (pure presentation).
- `local_setup.md` — no new setup step; fonts still load via the existing
  `index.html` links.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Broad migration causes layout/behavior regressions | High | Medium | One page/commit; preserve DOM/state/API; both-role smoke |
| Deleting `app.css` breaks a missed consumer (incl. auth) | Medium | High | Pull every `app.css` consumer into scope; grep-prove zero before delete; delete as its own commit |
| Unifying to dark sea is the wrong direction for some pages | Low | High | Direction confirmed by user; if ever reversed, swap the `themes.css` binding (token names unchanged) |
| Three button variants drift in use | Low | Low | All defined in `Button`; stylelint blocks ad-hoc button CSS |
| Stylelint blocks legitimate token files | Medium | Low | Exempt `tokens.css`/`themes.css`; enforce only `web/src/**` pages/components |
| "All pages incl. funnel" is large | Medium | Medium | Archetype batching + primitives reuse keep per-page cost low |

## References

- `docs/sprints/drafts/SPRINT-028-INTENT.md`
- `docs/sprints/drafts/SPRINT-028-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-028-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-028-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-028-MERGE-NOTES.md`
- `DESIGN.md`, `CLAUDE.md`, `CODEX.md`
- `docs/decisions/0005-voyage-cockpit-reboot.md`
