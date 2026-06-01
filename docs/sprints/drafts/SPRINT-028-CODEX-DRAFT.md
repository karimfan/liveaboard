# Sprint 028: One UI — Finish the Admin Migration

## Overview

Sprint 028 makes the authenticated admin app feel like one product again.
Sprint 027 shipped the Voyage Cockpit and collapsed Triptych, but the
deeper admin routes still mix cockpit modules, shared primitives, legacy
global `app.css`, and one-off page styling. The result is exactly what
the user reported: different pages appear to use different fonts,
surfaces, and color palettes.

This sprint migrates **all admin pages** to the same tokenized design
system. Public auth, guest invitation, guest registration, guest tab, and
any future public funnel pages are explicitly out of scope. `app.css` is
not hard-deleted in Sprint 028; it becomes a shrinking compatibility shim
with an owner comment and a Sprint 029 retirement target.

The sprint also adds the guardrail that prevents the regression from
returning: Stylelint runs in CI with project rules for `no-raw-hex` and
font allowlisting. Palette source files may define colors; pages and
components consume semantic tokens.

## Use Cases

1. **Operator moves through the admin app without visual jumps.** Today,
   Trips, trip detail routes, Fleet, Inventory, Users, Organization,
   Reports, Audit, Import, and Account all share the same typography,
   chrome, surfaces, chips, tables, forms, and spacing language.
2. **Cruise Director sees the same system on allowed routes.** Director
   pages such as Trips, Trip Manifest, Trip Dashboard, Trip Ledger,
   Trip Guest Detail, Guest Folio, Account, and Audit use the same
   primitives as Org Admin routes without leaking admin-only navigation.
3. **A developer adding a page cannot bypass the palette by accident.**
   Stylelint fails when admin page/component CSS uses raw hex outside the
   token source or names a font outside the approved contract.
4. **The app actually loads the approved fonts.** Font packages or local
   `@font-face` definitions are wired through `main.tsx`, and computed
   admin typography resolves to the approved display, body, and mono
   families rather than an incidental system fallback.
5. **Legacy CSS shrinks without a risky cliff edge.** `app.css` remains
   imported for compatibility, but only contains temporary legacy
   selectors that are documented, token-only, and targeted for Sprint
   029 deletion.

## Architecture

### Scope Boundary

**In scope:**

- Every route mounted under `/admin`.
- Shared admin chrome in `web/src/admin/Shell.tsx`,
  `web/src/admin/components/`, and `web/src/admin/cockpit/`.
- Admin helpers rendered inside admin pages, including
  `AssignDirector`, `CurrencyPicker`, and `UserMenu`.
- Global style layers that affect admin rendering:
  `tokens.css`, `themes.css`, `base.css`, `admin.css`, `motion.css`,
  and the shrinking `app.css` shim.
- Font loading and package setup.
- Stylelint configuration, custom project rules if needed, npm scripts,
  Makefile/CI integration, and documentation.

**Out of scope:**

- Public auth routes: signup, login, verify email, password reset, and
  invitation acceptance.
- Guest/public routes: guest invitation, guest registration, guest tab,
  and public/guest funnel pages.
- New product features, route changes, IA changes, data-model changes,
  endpoint changes, or copy rewrites beyond tiny labels needed by shared
  components.
- Hard-deleting `web/src/styles/app.css` in Sprint 028.

### Typography Contract

The first implementation task is to reconcile `DESIGN.md`,
`tokens.css`, and loaded assets into one contract. The code currently
declares token font families but does not load webfont assets.

Sprint 028 should make one explicit decision and document it:

- Display font: `--font-display`.
- Body/UI font: `--font-body`.
- Mono/data font: `--font-mono`.
- No component or page CSS names an arbitrary family. It must use
  `var(--font-display)`, `var(--font-body)`, `var(--font-mono)`,
  `inherit`, or the approved fallback stack inside token definitions.

Use self-hosted fonts through `@fontsource` packages unless the
implementation finds a stronger local asset reason. `main.tsx` imports
the font CSS before app styles so all admin pages render consistently.

### Token Ownership

Color definitions live in token source files:

- `web/src/styles/tokens.css` owns base palette and semantic token names.
- `web/src/styles/themes.css` owns production cockpit/admin bindings.

Admin components and pages read semantic tokens only:

- `--surface-*`
- `--text-*`
- `--accent-*`
- `--border-*`
- `--status-*`
- `--focus-ring`
- `--shadow-*`

Raw hex is allowed only in token source files and explicitly approved
asset/background definitions. It is forbidden in page CSS, component CSS,
and the surviving `app.css` shim.

### Migration Pattern

Pages should move off global selectors such as:

- `admin-page-header`
- `admin-page-title`
- `admin-page-subtitle`
- `admin-card`
- `admin-card__title`
- `setup-list__*`
- `chip--*`
- `rate-chip--*`

Use the existing shared primitives wherever they fit:

- `Page`
- `PageHeader`
- `Section`
- `Card`
- `DataTable`
- `Button`
- `Chip`
- `Field`
- `FormSection`
- `ActionBar`
- `Tabs`
- `Empty`
- `Stat`

When a page needs layout that is genuinely page-specific, add a colocated
CSS module and keep it token-only. Prefer small, boring layout modules to
large new styling systems.

### `app.css` Shim Policy

`web/src/styles/app.css` remains imported in Sprint 028. By the end of
the sprint it should:

- Start with a comment that names it as a Sprint 028 compatibility shim
  scheduled for Sprint 029 retirement.
- Contain no raw hex and no unapproved font stacks.
- Contain only selectors that are still needed by public/guest pages or
  unavoidable transition points.
- Contain no admin route styling that has been migrated to primitives or
  CSS modules.

`main.tsx` should keep the import order explicit and update the comment
from "temporary legacy chrome for unmigrated pages" to the narrower shim
purpose.

### Stylelint Guardrail

Add Stylelint as the frontend style gate:

- `web/package.json` scripts:
  - `lint:styles`
  - optionally `lint`
- `web/.stylelintrc.cjs` or `web/stylelint.config.cjs`.
- Custom local Stylelint rules under `web/stylelint-rules/` if built-in
  rules cannot express the project policy clearly.

Required rules:

- `liveaboard/no-raw-hex`: fail raw hex in admin/page/component CSS and
  compatibility CSS. Token source files are the only regular exception.
- `liveaboard/font-allowlist`: fail `font-family` declarations that do
  not use approved token variables, `inherit`, or the token-source
  fallback stacks.

CI must run the stylelint script. This repo currently has no `.github`
workflow, so Sprint 028 should add a minimal CI workflow or equivalent
project CI entry that runs:

- `make lint`
- `make test`
- `cd web && npm ci && npm run build && npm run lint:styles`

If the implementation chooses not to create GitHub Actions, it must land
an equivalent checked-in CI command path and document where it runs.

## Implementation Plan

### Phase 1: Contract Reconciliation and Font Loading (~10%)

**Files:**

- `DESIGN.md`
- `web/package.json`
- `web/package-lock.json`
- `web/src/main.tsx`
- `web/src/styles/tokens.css`
- `web/src/styles/base.css`
- `web/src/styles/themes.css`

**Tasks:**

- [ ] Reconcile `DESIGN.md` typography with `tokens.css` and the current
      production theme. Pick the three approved families and remove stale
      contradictory font names from the contract.
- [ ] Add self-hosted font dependencies or local font assets for the
      approved display/body/mono fonts.
- [ ] Import font CSS before app styles in `main.tsx`.
- [ ] Ensure `html, body` declare `font-family: var(--font-body)` once
      in the winning base layer.
- [ ] Ensure data/table primitives use the approved mono/data path where
      needed.
- [ ] Verify public/guest routes still render after font loading changes,
      while keeping their redesign out of scope.

### Phase 2: Shared Primitive Coverage (~15%)

**Files:**

- `web/src/admin/components/*.tsx`
- `web/src/admin/components/*.module.css`
- `web/src/admin/AssignDirector.tsx`
- `web/src/admin/CurrencyPicker.tsx`
- `web/src/admin/UserMenu.tsx`
- `web/src/admin/Shell.tsx`
- `web/src/styles/admin.css`

**Tasks:**

- [ ] Audit shared primitives for raw hex, hardcoded fonts, oversized
      weights, and inconsistent token use.
- [ ] Extend primitives only where repeated admin migrations need a real
      shared pattern, such as status chips, page actions, dense form rows,
      or responsive table wrappers.
- [ ] Move admin helper styling to primitives or CSS modules.
- [ ] Keep `SpacesNav` and cockpit chrome aligned with the same semantic
      token set used by migrated pages.
- [ ] Avoid introducing new global admin selectors unless they are shell
      primitives with a clear owner.

### Phase 3: Migrate Admin Run-Loop Pages (~25%)

**Files:**

- `web/src/admin/pages/Trips.tsx`
- `web/src/admin/pages/TripDashboard.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/pages/TripConsumptionLedger.tsx`
- `web/src/admin/pages/TripCabins.tsx`
- `web/src/admin/pages/TripGuestDetail.tsx`
- `web/src/admin/pages/GuestFolio.tsx`
- Page-local CSS modules as needed.

**Tasks:**

- [ ] Replace global page headers with `Page` and `PageHeader`.
- [ ] Replace `admin-card` blocks with `Card`, `Section`, or page-local
      token modules.
- [ ] Replace `chip chip--*` status spans with the shared `Chip`
      component or a typed status-chip helper.
- [ ] Keep route behavior, API calls, forms, and mutations unchanged.
- [ ] Check dense trip tables and ledgers at mobile widths for no clipped
      controls or horizontal page overflow beyond intentional table
      scrolling.

### Phase 4: Migrate Org Admin Configuration Pages (~20%)

**Files:**

- `web/src/admin/pages/Fleet.tsx`
- `web/src/admin/pages/BoatDetail.tsx`
- `web/src/admin/pages/BoatTabs.tsx`
- `web/src/admin/pages/BoatCabins.tsx`
- `web/src/admin/pages/Inventory.tsx`
- `web/src/admin/pages/Users.tsx`
- `web/src/admin/pages/Organization.tsx`
- `web/src/admin/pages/OrganizationPayments.tsx`
- `web/src/admin/pages/OrganizationPricing.tsx`
- Page-local CSS modules as needed.

**Tasks:**

- [ ] Migrate fleet and boat detail surfaces to shared page/table/form
      primitives.
- [ ] Migrate inventory, catalog, FX, and pricing forms without changing
      mutation behavior.
- [ ] Migrate users and organization settings pages.
- [ ] Replace all remaining legacy chip and card classes in these routes.
- [ ] Preserve `RequireAdmin` route boundaries and API authorization
      behavior.

### Phase 5: Migrate Admin Support Pages and Cockpit Edges (~15%)

**Files:**

- `web/src/admin/pages/Overview.tsx`
- `web/src/admin/pages/AuditEvents.tsx`
- `web/src/admin/pages/Reports.tsx`
- `web/src/admin/pages/Onboarding.tsx`
- `web/src/admin/pages/Account.tsx`
- `web/src/admin/pages/Import.tsx`
- `web/src/admin/pages/ImportLiveaboard.tsx`
- `web/src/admin/pages/ImportSpreadsheet.tsx`
- `web/src/admin/pages/ImportJob.tsx`
- `web/src/admin/cockpit/*.tsx`
- `web/src/admin/cockpit/*.module.css`

**Tasks:**

- [ ] Confirm `/admin` cockpit still owns the index route and still passes
      Sprint 027's visual contract after shared primitive changes.
- [ ] Migrate audit, reports, onboarding, account, and import flows off
      global legacy selectors.
- [ ] Replace inline layout styles with module classes when repeated or
      when they obscure responsive behavior.
- [ ] Remove dead references to `setup-list__*`, `admin-card__*`,
      `admin-page-*`, `chip--*`, and `rate-chip--*` from admin code.
- [ ] Keep public guest/import data behavior untouched.

### Phase 6: Shrink `app.css` and Add Guardrails (~10%)

**Files:**

- `web/src/styles/app.css`
- `web/src/styles/admin.css`
- `web/src/main.tsx`
- `web/package.json`
- `web/package-lock.json`
- `web/stylelint.config.cjs`
- `web/stylelint-rules/no-raw-hex.cjs`
- `web/stylelint-rules/font-allowlist.cjs`
- `.github/workflows/ci.yml` or equivalent checked-in CI entry
- `Makefile`

**Tasks:**

- [ ] Remove migrated admin selectors from `app.css`.
- [ ] Leave `app.css` as a documented Sprint 028 shim, not a general
      admin styling surface.
- [ ] Add Stylelint dependencies and scripts.
- [ ] Add `no-raw-hex` and `font-allowlist` enforcement.
- [ ] Wire stylelint into CI and the local lint path.
- [ ] Ensure token source files are allowed to define colors while
      admin pages/components are not.

### Phase 7: Verification and Close (~5%)

**Files:**

- `docs/sprints/SPRINT-028.md` when the final sprint is written.
- Documentation Manifest files listed below.
- Touched tests and config.

**Tasks:**

- [ ] Run `cd web && npm run lint:styles`.
- [ ] Run `cd web && npm run build`.
- [ ] Run `make lint`.
- [ ] Run `make test`, documenting any local PostgreSQL skips.
- [ ] Start the app and smoke all `/admin` routes for Org Admin.
- [ ] Smoke Cruise Director-allowed `/admin` routes.
- [ ] Verify no public/guest route was intentionally redesigned.
- [ ] Update sprint notes, tracker status, and Documentation Manifest.

## API Endpoints

No API endpoints are added or changed in Sprint 028.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `DESIGN.md` | Modify | Reconcile the authoritative typography and color contract with the implemented token system. |
| `CODEX.md`, `CLAUDE.md` | Modify | Add explicit admin-only migration, `app.css` shim, and Stylelint guardrail guidance. |
| `web/package.json`, `web/package-lock.json` | Modify | Add font and Stylelint dependencies plus scripts. |
| `web/src/main.tsx` | Modify | Load fonts and clarify style import order. |
| `web/src/styles/tokens.css` | Modify | Own approved font stacks, base palette, and semantic token names. |
| `web/src/styles/themes.css` | Modify | Keep production theme bindings as the only palette implementation layer. |
| `web/src/styles/base.css` | Modify | Keep global body typography/background/focus defaults consistent. |
| `web/src/styles/admin.css` | Modify | Retain only true admin shell overrides or delete if fully absorbed. |
| `web/src/styles/app.css` | Modify | Reduce to Sprint 028 compatibility shim; schedule deletion in Sprint 029. |
| `web/src/admin/components/*` | Modify | Ensure shared primitives enforce consistent admin UI. |
| `web/src/admin/cockpit/*` | Modify | Keep cockpit aligned after token/font changes. |
| `web/src/admin/pages/*.tsx` | Modify | Migrate all admin pages from legacy global styling to primitives/modules. |
| `web/src/admin/pages/*.module.css` | Create as needed | Page-specific token-only layout modules. |
| `web/stylelint.config.cjs` | Create | Stylelint entry point. |
| `web/stylelint-rules/*.cjs` | Create | Project-specific no-raw-hex and font allowlist rules if needed. |
| `.github/workflows/ci.yml` | Create | Run lint, tests, web build, and stylelint in CI if no equivalent exists. |
| `Makefile` | Modify | Include frontend style lint in the local lint path or add a documented target. |

## Definition of Done

- [ ] All routes mounted under `/admin` use the same approved font
      contract and render with loaded font assets.
- [ ] Public/guest/auth funnel pages are not redesigned as part of the
      sprint.
- [ ] Every admin page is migrated off legacy global classes:
      `admin-page-*`, `admin-card*`, `setup-list__*`, `chip--*`, and
      `rate-chip--*`.
- [ ] Admin page/component CSS contains no raw hex outside approved token
      source files.
- [ ] Admin page/component CSS contains no unapproved `font-family`
      declarations.
- [ ] `web/src/styles/app.css` remains present but is reduced to a
      documented compatibility shim scheduled for Sprint 029 retirement.
- [ ] No new page-level dependency on `app.css` is introduced.
- [ ] Stylelint runs locally and in CI with `no-raw-hex` and
      `font-allowlist` rules.
- [ ] `rg -n "admin-page-|admin-card|setup-list__|chip--|rate-chip--" web/src/admin` returns zero route/component dependencies, except comments explicitly documenting removed legacy names if needed.
- [ ] `cd web && npm run lint:styles` passes.
- [ ] `cd web && npm run build` passes.
- [ ] `make lint` passes.
- [ ] `make test` passes, or PostgreSQL-dependent skips are documented.
- [ ] Org Admin route smoke covers Today, Trips, trip detail routes,
      Fleet, Inventory, Users, Organization, Payments, Pricing, Reports,
      Audit, Onboarding, Import, and Account.
- [ ] Cruise Director route smoke covers Today, Trips, trip detail routes,
      Audit, and Account without exposing admin-only routes.
- [ ] Documentation Manifest items are complete.

## Security Considerations

- This is a presentation sprint. It should not alter API authorization,
  tenant scoping, session behavior, role filtering, or route guards.
- `RequireAdmin` must continue protecting org-admin-only routes; the
  visual migration cannot make hidden routes discoverable to Cruise
  Directors.
- Stylelint custom rules should inspect CSS source only and must not
  execute untrusted project code.
- Self-hosted fonts avoid external font CDN requests and keep the app's
  privacy/network posture unchanged.
- CI should not require secrets for frontend style checks.
- Visual smoke should include role boundaries because broad page edits can
  accidentally expose links or controls even without backend changes.

## Dependencies

- Sprint 027 cockpit, token, and shell collapse are the baseline.
- Existing admin component primitives from `web/src/admin/components/`.
- Font packages selected during Phase 1.
- Stylelint and any minimal local rule helpers.
- No new backend dependencies.
- No database migrations.

## Documentation Manifest

The implementation sprint MUST land the following docs changes alongside
the code. The `sprint` skill verifies each file in this list was modified
before marking the sprint complete.

### Cross-cutting docs

- `DESIGN.md` — Reconcile the typography contract with loaded fonts and
  current production tokens; document that admin pages consume semantic
  tokens only and that public/guest pages are out of this sprint's visual
  migration.
- `CODEX.md` — Add Sprint 028 guidance: all new admin UI must use shared
  primitives or token-only CSS modules; no raw hex, no arbitrary fonts,
  no new `app.css` dependencies.
- `CLAUDE.md` — Mirror the same admin UI guidance, including the
  `app.css` shim and Sprint 029 retirement target.

### Sprint docs

- `docs/sprints/SPRINT-028.md` — Final sprint document with completed
  implementation notes and DoD status.
- `docs/sprints/SPRINT-029.md` or `docs/sprints/drafts/SPRINT-029-INTENT.md`
  — Create a follow-on note for hard-deleting the remaining `app.css`
  shim after public/guest scope is planned, unless the tracker process
  prefers adding this as a deferred section in Sprint 028 instead.

### Optional ADR

- `docs/decisions/0006-admin-design-token-enforcement.md` — Create only
  if implementation introduces custom Stylelint rules or a new CI policy
  that should be durable beyond Sprint 028. Skip if the final change is
  straightforward config and adequately documented in `DESIGN.md`,
  `CODEX.md`, and `CLAUDE.md`.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Migrating every admin page is broad enough to cause behavior regressions | High | Keep changes presentation-only, preserve component state/API code, migrate in route groups, and smoke both roles. |
| Stylelint blocks token source files that legitimately define palette values | Medium | Scope the raw-hex rule to pages/components/shims and explicitly exempt token source files. |
| `app.css` keeps enough selectors that visual drift remains | Medium | Require zero admin dependencies on legacy selectors and document every remaining shim selector with its owner/scope. |
| Typography contract conflicts between `DESIGN.md`, `tokens.css`, and current implementation | Medium | Resolve in Phase 1 before page migration; make `DESIGN.md` the source of truth and update tokens to match. |
| Shared primitives are missing variants needed by legacy pages | Medium | Add small variants to existing primitives when repeated; use page modules for one-off layout only. |
| CI setup expands the sprint unexpectedly | Low | Add the smallest workflow or checked-in CI path that runs existing gates plus stylelint; do not build a larger CI platform. |
| Public/guest pages still use old styles and confuse validation | Low | State the admin-only boundary in docs and DoD; smoke them only for non-regression, not visual parity. |

## Deferred to Sprint 029

- Hard-delete `web/src/styles/app.css`.
- Bring public auth, guest registration, guest tab, and future public
  funnel pages onto the same post-Sprint-028 contract if the product
  wants full-app visual parity.
- Consider visual regression screenshots once admin styling is stable and
  the route set is no longer in active migration.

## References

- `docs/sprints/drafts/SPRINT-028-INTENT.md`
- `docs/sprints/README.md`
- `docs/sprints/SPRINT-025.md`
- `docs/sprints/SPRINT-026.md`
- `docs/sprints/SPRINT-027.md`
- `DESIGN.md`
- `CLAUDE.md`
- `CODEX.md`
- `web/src/styles/tokens.css`
- `web/src/styles/themes.css`
- `web/src/styles/app.css`
- `web/src/admin/components/`
- `web/src/admin/pages/`
