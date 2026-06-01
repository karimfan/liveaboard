# Sprint 028 Intent: One UI — Finish the Cockpit Migration

## Seed

> make the UI consistent. Same fonts throughout. Same color palette. I
> see some pages differ

## Context (Orientation Summary)

- **Fonts are declared but never loaded.** `web/src/styles/tokens.css`
  defines `--font-display` (Space Grotesk), `--font-body` (Inter),
  `--font-mono` (JetBrains Mono), but **no `@fontsource` package is
  installed** and there is no `@font-face` / Google Fonts link anywhere.
  Every branded font silently falls back to `system-ui`. DESIGN.md's
  typography contract is currently fiction — the app renders in the OS
  default sans everywhere.
- **Three parallel styling worlds coexist:**
  1. **Token + CSS-module pages** (the target): the cockpit
     (`web/src/admin/cockpit/`), `Account`, `TripGuestDetail`.
  2. **Global `app.css` pages**: `Trips`, `Fleet`, `Inventory`,
     `Users`, `Organization`, `TripDashboard`, `TripManifest`, and more
     — styled against a 21 KB global stylesheet with **137 raw hex
     values** that predate the token contract.
  3. **One-off plain-CSS pages**: `Reports.css`, `Onboarding.css`,
     `BoatNotes.css` — each ships its own bespoke palette/spacing.
  This is exactly the "some pages differ" the user observes.
- **Base layer has cruft/contradictions.** `base.css` sets the body
  `font-family` twice (a hardcoded system stack overriding
  `var(--font-body)`), then repeats `font-family: var(--font-body)`
  ~11 times. Stale `.disabled` / `.backup` / `.backup-fancy`
  stylesheets linger in `src/styles/` and `src/admin/pages/`.
- **The contract already exists and is good.** DESIGN.md (rewritten in
  Sprint 027) is a complete, authoritative spec for type, color,
  spacing, and components. The gap is **implementation**, not design:
  pages were never migrated onto the tokens, and fonts were never
  wired.

## Recent Sprint Context

- **Sprint 025 (Triptych):** built a runtime-switchable design system
  (rail/spaces/canvas) to explore IA philosophies — the origin of the
  token contract.
- **Sprint 026 (Funnel):** opened the public funnel, guest portal, and
  ops spine. Added several pages, some with bespoke CSS.
- **Sprint 027 (Cockpit Reboot):** closed Triptych, deleted the
  switcher and losing renderers, collapsed to a single cockpit shell.
  **Explicitly deferred page migration** — kept `app.css` as "legacy
  scaffolding for unmigrated pages (Trips, Fleet, Inventory, etc.),
  flagged for retirement." Sprint 028 is the direct follow-on: finish
  what 027 started.

## Relevant Codebase Areas

- `web/package.json` — add `@fontsource` deps (self-hosted fonts).
- `web/src/main.tsx` — font `@fontsource` imports + CSS load order.
- `web/src/styles/tokens.css` — semantic token source of truth.
- `web/src/styles/themes.css` — token bindings.
- `web/src/styles/base.css` — body defaults (needs de-duping).
- `web/src/styles/app.css` — 21 KB legacy global sheet to retire.
- `web/src/styles/*.backup*`, `*.disabled` — dead files to delete.
- `web/src/admin/pages/*.tsx` + bespoke `.css` — migration targets.
- `web/src/admin/cockpit/` — reference implementation to mirror.
- `DESIGN.md` — the binding contract; QA checklist lives here.

## Constraints

- Must follow `DESIGN.md` exactly — no new fonts beyond the three named
  families; semantic tokens only, no raw hex in components; no runtime
  theme/palette switching; retire `app.css` rather than extend it.
- Must follow `CLAUDE.md` Development Rules: compile, tests written and
  passing, `go vet` / `gofmt` / `prettier` / `tsc` clean, focused
  commits, work directly on `main`, no PRs unless asked.
- Self-hosted fonts only (no external CDN) per DESIGN.md typography
  note.
- Behavior-preserving: this is a visual-consistency sprint, not a
  feature or IA change. No route, data, or copy changes beyond what a
  restyle requires.

## Success Criteria

1. The three branded fonts actually load and render across every admin
   page (verifiable: computed `font-family` resolves to Space Grotesk /
   Inter / JetBrains Mono, not the system fallback).
2. Every admin page draws from the same semantic token palette — no
   page visibly diverges in background, surface, text, accent, or
   signal colors.
3. `app.css` and all bespoke per-page `.css` files are either deleted
   or reduced to token-referencing CSS modules; no raw hex remains in
   component/page styles.
4. `base.css` body font is declared once, correctly, via
   `var(--font-body)`; dead `.backup`/`.disabled` files removed.
5. A repeatable guardrail exists (lint rule, CI check, or documented QA
   step) so new pages can't silently reintroduce raw hex or off-palette
   fonts.
6. All quality gates pass; DESIGN.md QA checklist passes for every
   migrated page.

## Open Questions

1. **Font delivery:** `@fontsource` self-hosted packages (matches
   DESIGN.md) vs. variable-font subset — assume `@fontsource` unless
   told otherwise. Confirm bundle-size tolerance.
2. **Scope of migration:** migrate *all* legacy pages this sprint, or
   prioritize the highest-traffic admin pages (Trips, Fleet, Inventory,
   Users, Organization) and stage the rest? Guest/public funnel pages
   (Sprint 026) — in scope or separate?
3. **`app.css` retirement:** hard-delete after migration, or keep a
   shrinking shim until every consumer is gone?
4. **Enforcement mechanism:** Stylelint (raw-hex / font ban) vs. a
   lightweight CI grep vs. manual QA checklist — how much tooling does
   the user want?
5. **Visual regression safety:** is there an existing screenshot/visual
   test harness, or should the plan add a minimal one to catch drift?
