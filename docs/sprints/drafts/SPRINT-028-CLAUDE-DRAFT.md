# Sprint 028: One UI — Finish the Cockpit Migration

**Status:** planned
**Phase:** polish
**Depends on:** Sprint 027 (single cockpit shell, token contract),
Sprint 025 (token system origin)

---

## Overview

Sprint 027 closed the Triptych experiment and declared a single
production surface — the Voyage Cockpit — backed by a semantic token
contract in `DESIGN.md`. But it deliberately stopped short of migrating
the existing pages: `app.css` (21 KB, 137 raw hex values) was kept as
"legacy scaffolding for unmigrated pages, flagged for retirement." The
result is a UI that *looks* finished on the Today/cockpit view but
fractures the moment you click into Trips, Fleet, Inventory, or
Reports — each drawing from a different palette, and **none of them
rendering the branded fonts at all**, because the `@fontsource`
packages were never installed. Space Grotesk, Inter, and JetBrains Mono
are referenced in tokens but silently fall back to the OS system font
everywhere.

Sprint 028 finishes the job. It is a **consistency and consolidation
sprint**: wire the fonts so the typography contract becomes real,
migrate every legacy page off `app.css` and bespoke per-page
stylesheets onto semantic tokens + CSS modules, delete the dead CSS,
and install a guardrail so the inconsistency can't silently return.
There is no new feature work and no IA change — when the sprint lands,
the app should *do* exactly what it does today, but look like one
coherent product.

The approach is mechanical and low-risk per page, but broad. To keep it
reviewable we sequence it: (1) make the foundation real and correct
(fonts + base cleanup), (2) migrate pages in priority order behind the
already-correct token layer, (3) retire `app.css` and dead files once
no consumer remains, (4) lock it down with a lint/CI guardrail.

## Use Cases

1. **Consistent typography**: An operator navigating from the cockpit
   to Trips to Reports sees the same three fonts (Space Grotesk
   headings, Inter body, JetBrains Mono for amounts/timestamps)
   throughout — not the cockpit in one font and every other page in the
   OS default.
2. **Consistent palette**: Backgrounds, surfaces, text, accents, and
   signal colors are identical across every page because they all
   resolve from the same semantic tokens. No page "looks like a
   different app."
3. **No silent regression**: A developer adding a new page that hardcodes
   `#1a2b3c` or pulls in a fourth font is caught by lint/CI before
   merge, not by the user noticing drift three sprints later.
4. **Maintainable styling**: A designer changing the accent color edits
   one token and every page updates; there is no longer a 21 KB global
   stylesheet or a pile of one-off `.css` files to hunt through.

## Architecture

### Current (fractured)

```
tokens.css  (semantic tokens — correct, but fonts never loaded)
   │
   ├─ themes.css → binds tokens
   ├─ base.css   → body (font declared 2x, ~11 dup lines)
   │
   ├─ COCKPIT + Account + TripGuestDetail ──► tokens + CSS modules ✓
   ├─ Trips/Fleet/Inventory/Users/Org/… ────► app.css (137 raw hex) ✗
   └─ Reports/Onboarding/BoatNotes ─────────► bespoke per-page .css ✗

fonts: REFERENCED in tokens, NEVER loaded → system-ui fallback
```

### Target (unified)

```
@fontsource/{space-grotesk,inter,jetbrains-mono}  ← imported in main.tsx
   │
tokens.css  (single source of truth: type, color, spacing)
   │
   ├─ themes.css → binds tokens
   ├─ base.css   → body uses var(--font-body) once
   │
   └─ EVERY page ──► semantic tokens + co-located CSS module ✓

app.css: DELETED.  bespoke .css / .backup / .disabled: DELETED.
guardrail: stylelint (no raw hex in components, font allowlist) in CI.
```

### Migration pattern (per page)

Each legacy page follows the same recipe, mirroring the cockpit
reference implementation:

1. Create `PageName.module.css` co-located with the `.tsx`.
2. Port the page's visual rules from `app.css` / bespoke `.css`,
   replacing every raw hex with the nearest semantic token
   (`var(--hull)`, `var(--text-primary)`, `var(--accent-cyan)`, …).
3. Replace global className strings with `styles.*` references.
4. Apply font roles: headings → display, body/labels → body,
   amounts/timestamps/IDs → mono (per DESIGN.md type usage rule).
5. Delete the now-dead rules from `app.css` / the bespoke file.
6. Run the DESIGN.md QA checklist for the page.

## Implementation Plan

### Phase 1: Make the foundation real (~20%)

The token layer is correct; the problem is fonts don't load and
`base.css` contradicts itself. Fix this first so every subsequent page
migration inherits a correct baseline.

**Files:**
- `web/package.json` — add `@fontsource/space-grotesk`,
  `@fontsource/inter`, `@fontsource/jetbrains-mono` (self-hosted,
  no CDN).
- `web/src/main.tsx` — import the needed font weights at the top of the
  CSS load order (before `tokens.css`).
- `web/src/styles/base.css` — collapse the duplicated body
  `font-family` down to a single `var(--font-body)`; remove the
  hardcoded system stack and the ~11 repeated lines.
- `web/src/styles/tokens.css` — verify font tokens match the installed
  family names exactly; no change expected beyond confirmation.

**Tasks:**
- [ ] Install the three `@fontsource` packages, pinning only the
      weights DESIGN.md uses (display 500/600/700, body 400/500/600,
      mono 400/500).
- [ ] Import them in `main.tsx`; verify computed `font-family` on a
      heading resolves to Space Grotesk, body to Inter, a folio amount
      to JetBrains Mono.
- [ ] De-dupe `base.css` body font; confirm no system-stack override
      survives.
- [ ] Smoke-test bundle size delta; document it in the sprint doc.

### Phase 2: Migrate legacy pages to tokens + modules (~50%)

Migrate in priority order — highest-traffic admin pages first — so the
most-seen inconsistency disappears earliest and review batches are
coherent. Each page is an independent, behavior-preserving restyle.

**Batch A — global `app.css` admin pages (highest priority):**
- `Trips.tsx` → `Trips.module.css`
- `Fleet.tsx` → `Fleet.module.css`
- `Inventory.tsx` → `Inventory.module.css`
- `Users.tsx` → `Users.module.css`
- `Organization.tsx` (+ `OrganizationPayments`, `OrganizationPricing`,
  `OrganizationProfile`) → modules
- `TripDashboard.tsx`, `TripManifest.tsx`, `TripConsumptionLedger.tsx`,
  `TripCabins.tsx` → modules
- `BoatDetail.tsx`, `BoatTabs.tsx`, `BoatTrips.tsx`, `BoatCabins.tsx`
  → modules

**Batch B — bespoke per-page `.css` pages:**
- `Reports.tsx` (`Reports.css` → `Reports.module.css`)
- `Onboarding.tsx` (`Onboarding.css` → module)
- `BoatNotes.tsx` (`BoatNotes.css` → module)
- `AuditEvents.tsx`, `GuestFolio.tsx`, `Import*.tsx` → audit & migrate
  if they reference app.css/global classes.

**Tasks:**
- [ ] For each page: create module, port rules with tokens, swap
      classNames, apply font roles, delete dead rules, QA-checklist.
- [ ] Enforce the mono rule: every folio balance, ledger amount, and
      timestamp uses `var(--font-mono)` and right-aligns in tables.
- [ ] Keep a running "lines removed from app.css" tally per page to
      prove convergence.

### Phase 3: Retire dead CSS (~10%)

Once Phase 2 leaves `app.css` with no consumers, delete it and the
stale backups.

**Files:**
- `web/src/styles/app.css` — delete (and remove its import from
  `main.tsx`).
- `web/src/styles/motion.css.backup`, `motion.css.backup-fancy` —
  delete.
- `web/src/admin/pages/Reports.module.css.disabled`,
  `Onboarding.css.disabled`, `Onboarding.module.css.disabled`,
  `Onboarding.css.backup` — delete.

**Tasks:**
- [ ] `grep` the tree to prove no remaining import/reference to
      `app.css` or any global class it defined.
- [ ] Delete `app.css` + backups; remove the import; confirm clean
      build and no visual change on a click-through of every page.

### Phase 4: Guardrail against regression (~20%)

Make the consistency self-enforcing so Sprint 029+ can't silently
reintroduce drift.

**Files:**
- `web/.stylelintrc.json` (new) — `color-no-hex` (or
  `declaration-property-value-disallowed-list` for `color`/`background`)
  scoped to component/module CSS; a font-family allowlist.
- `web/package.json` — add `stylelint` + a `lint:css` script; wire into
  the existing lint/CI step.
- `DESIGN.md` — add a short "Enforcement" note pointing at the lint
  rule so the contract and the check stay linked.

**Tasks:**
- [ ] Add stylelint with rules: no raw hex in `*.module.css`; font
      stack must be one of the three token vars.
- [ ] Run it across the migrated tree; fix any stragglers until clean.
- [ ] Add `lint:css` to CI / the documented pre-commit gate.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `web/package.json` | Modify | Add `@fontsource/*` + `stylelint` deps & scripts |
| `web/src/main.tsx` | Modify | Import fonts; drop `app.css` import |
| `web/src/styles/base.css` | Modify | Single correct body font; de-dupe |
| `web/src/styles/tokens.css` | Verify | Confirm font token names |
| `web/src/styles/app.css` | Delete | Retire legacy global sheet |
| `web/src/styles/*.backup*` | Delete | Remove dead backups |
| `web/src/admin/pages/*.tsx` | Modify | Swap to `styles.*` references |
| `web/src/admin/pages/*.module.css` | Create | Per-page token-based styles |
| `web/src/admin/pages/{Reports,Onboarding,BoatNotes}.css` | Delete | Replaced by modules |
| `web/src/admin/pages/*.disabled` | Delete | Dead files |
| `web/.stylelintrc.json` | Create | Hex/font guardrail |
| `DESIGN.md` | Modify | Add enforcement note |

## Definition of Done

- [ ] `@fontsource` packages installed; computed font-family on
      headings/body/amounts resolves to Space Grotesk / Inter /
      JetBrains Mono on every admin page (not system-ui).
- [ ] `base.css` body font declared once via `var(--font-body)`; no
      system-stack override, no duplicate lines.
- [ ] Every admin page imports only a co-located `*.module.css`; no
      page references `app.css` or a bespoke global `.css`.
- [ ] `app.css` and all `.backup`/`.disabled` stylesheets deleted; no
      remaining imports or class references.
- [ ] No raw hex in any `*.module.css` (stylelint passes).
- [ ] Stylelint `lint:css` added and wired into the lint/CI gate.
- [ ] DESIGN.md QA checklist passes for every migrated page.
- [ ] `tsc`, `prettier`, and `go vet`/`gofmt` (if any Go touched) clean.
- [ ] Manual click-through confirms no behavioral or layout regression.
- [ ] DESIGN.md updated with enforcement note; sprint doc finalized.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Per-page restyle introduces subtle layout regressions | High | Medium | Migrate one page per commit; click-through QA; keep DOM/classNames structurally equivalent |
| Scope creep — "while I'm here" feature tweaks | Medium | Medium | Hard rule: behavior-preserving only; defer any feature idea to a backlog note |
| Fonts bloat the bundle | Medium | Low | Pin only required weights; subset; measure & report delta |
| Hidden `app.css` consumers (public/guest pages) break on delete | Medium | High | grep-prove zero references before delete; stage delete as its own commit |
| Tables lose mono/right-align nuance in migration | Medium | Low | Codify the table/amount pattern once, reuse across pages |
| Token gaps — a legacy color has no semantic token | Medium | Medium | Add the missing semantic token to tokens.css rather than inlining hex |

## Security Considerations

- Self-hosted fonts via `@fontsource` (no external CDN) — avoids a
  third-party network dependency and the privacy/availability concerns
  of a CDN font fetch, consistent with DESIGN.md.
- No data, auth, or API surface changes; this is a presentation-layer
  sprint. RLS / tenant isolation untouched.

## Dependencies

- Sprint 027 (token contract, single shell) — required, complete.
- No external dependencies; all work is within `web/`.

## Open Questions

1. Migrate **all** legacy pages this sprint, or prioritize the top 5
   admin pages and stage the long tail into 029? (Draft assumes all
   admin pages, with guest/public funnel pages explicitly out of scope
   unless they import `app.css`.)
2. `app.css` — hard-delete at end of Phase 3, or leave a shrinking shim
   for one more sprint as insurance? (Draft assumes hard-delete once
   grep-proven dead.)
3. Guardrail depth: full stylelint config vs. a lightweight CI grep?
   (Draft assumes stylelint — more robust, standard tooling.)
4. Is there an existing visual-regression harness, or do we rely on
   manual click-through QA this sprint? (Draft assumes manual; flags a
   screenshot harness as a possible 029 follow-on.)
