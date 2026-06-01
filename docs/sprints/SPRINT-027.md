# Sprint 027: Voyage Cockpit Reboot

## Overview

Sprint 027 is the product reboot. The application has broad backend capability, but the daily experience still feels like a themed admin dashboard assembled one page at a time. The next sprint must make the core authenticated app feel chosen, operational, and beautiful before adding more external surfaces.

Earlier sprint plans are context, not binding direction. Sprint 026 Phase 1 has already landed data foundations and a token-bucket rate limiter on `main`; those tables and helpers may be read by the cockpit where useful. The remaining public funnel, guest portal, crew/equipment UI, and broad feature expansion stay paused until the cockpit spine is excellent.

The sprint ships three things: a role-specific `/admin` Voyage Cockpit for Org Admins and Cruise Directors, a hard Triptych collapse, and the Overview-to-Cockpit migration. The larger run-loop page migration moves to Sprint 028.

## Product Stance

Liveaboard becomes a premium dive-expedition operations cockpit: dark, spatial, dense, fast, and specific to liveaboard vessels. This is not a prettier Overview card grid. It is the app's command surface: voyage map, run sheet, blockers, money, readiness, activity, and route commands in one place.

The cockpit gets a route-scoped immersive visual treatment. Auth pages and non-cockpit public surfaces can keep the original Sprint 011 background until redesigned, but the authenticated cockpit is allowed to use stronger generated or curated imagery, lighting, overlays, and workplane composition when that makes the product more beautiful and clear.

## Use Cases

1. **Org Admin opens the app and reads the business in five seconds.** The cockpit shows fleet status, active/upcoming voyages, setup gaps, readiness blockers, guest/cabin fill, open folio exposure, payment/FX readiness, low-stock risk, lead/quote pressure from shipped Sprint 026 data, and latest activity.
2. **Cruise Director boards with a live run sheet.** The cockpit is scoped to assigned trips and centers start readiness, manifest gaps, berth gaps, guest registration/doc gaps, ledger entry, folio close, and trip completion.
3. **Operator acts without hunting.** A command palette and action rail launch common workflows: invite guest, assign cabin, record consumption, open ledger, close folio, import trips, fix FX, open blockers.
4. **The product finally feels ownable.** The UI looks like a dive expedition operations system, not generic SaaS chrome with ocean styling.
5. **Existing pages keep working during the reboot.** Existing routes remain reachable. Sprint 027 replaces the first screen; Sprint 028 migrates the deeper run-loop pages.

## Architecture

### Aggregate Endpoint

Add one role-aware endpoint:

| Endpoint | Method | Purpose |
|---|---:|---|
| `/api/admin/cockpit` | GET | Authenticated, role-scoped cockpit aggregate. |

Response sections:

- `identity` — user, role, organization, assignment summary.
- `admin_cockpit` — Org Admin fleet/business pulse; omitted for Cruise Directors.
- `director_cockpit` — assigned-trip run sheet; omitted for Org Admins unless explicitly useful.
- `voyage_lanes` — active and upcoming trips with status, dates, boat, director, manifest/cabin progress, lifecycle state.
- `blockers` — readiness, guest registration, cabin, FX/payment, inventory, folio, lead/quote, and lifecycle blockers.
- `money` — open folios, closed folios, settlement exposure, stale/missing FX, offline deposit/refund signals from shipped data.
- `inventory` — low and negative stock signals from existing inventory/ledger data.
- `activity` — bounded recent audit and ledger events.
- `commands` — role-filtered route commands for the command palette and action rail.
- `failed_sections` — optional list of degraded sections when a non-critical projection fails.

No cache, Redis, ETags, or derived persistent state in Sprint 027. Store projections should run bounded queries with explicit columns. Parallel independent queries may use a local `errgroup.WithContext`-style pattern if the repo already permits the dependency; otherwise keep it stdlib-simple and sequential.

Org Admins receive organization-scoped aggregate data. Cruise Directors receive only trips assigned to their user, money totals for those trips, no fleet-wide pulse, no org-wide inventory, and no cross-trip audit feed.

### Shipped Sprint 026 Data

The following already-shipped files are available as cockpit inputs where useful:

- `internal/store/leads.go`
- `internal/store/booking_quotes.go`
- `internal/store/offline_payments.go`
- `internal/store/crew.go`
- `internal/store/equipment.go`
- `internal/store/readiness.go`
- `internal/store/guest_certifications.go`
- `internal/store/guest_portal_requests.go`
- `internal/httpapi/rate_limit.go`

Sprint 027 does not expose new public routes or guest portal UI. It may project these existing rows into cockpit signals.

### Cockpit Shell

`/admin` renders `web/src/admin/cockpit/Cockpit.tsx`. Role differences live inside the Cockpit composition, not in separate routes.

The Sprint 023 onboarding auto-show in `Shell.tsx` must still fire for Org Admins landing on `/admin` when onboarding is incomplete.

### Visual Contract

Desktop cockpit grid (`>= 1280px`):

- Header strip: `72px` tall.
- VoyageMap: full width, `min-height: 360px`, `max-height: 480px`.
- Signal row: three columns, `320px / 320px / 1fr`, `16px` gap.
- MoneyStrip: `96px` tall.
- ActivityStream: `240px` tall, internal scroll.

Tile dimensions:

- Small: `220px x 96px`.
- Medium: `320px x 140px`.
- Large: full-width by `200px`.
- Below `768px`, tiles become `width: 100%`; no horizontal scroll.

Density:

- Body text: `14px / 1.45`.
- Tabular data: `13px / 1.4`, `tabular-nums`.
- Section labels: `11px`, uppercase, `0.06em` letter spacing.

Semantic token mapping:

- Idle action: `--accent-primary` (cyan).
- Selection/focus: `--accent-secondary` (violet).
- Blocker/risk: `--status-error` (magenta/coral).
- Money/warning: `--status-warning` (ember/gold).
- Surface base: `--surface-panel`.
- Surface elevated: `--surface-panel-strong`.
- Focus ring: `--focus-ring`.

Anti-patterns:

- No nested cards. The cockpit is one workplane with regions.
- No heavy drop-shadow stacks. Glow is reserved for hover/focus or live status.
- No animated counters on first paint. Values appear immediately.
- No filled buttons in the cockpit body except a single primary route action when the state clearly calls for it.
- No decorative imagery that reduces data contrast.

### Triptych Collapse

Delete rather than dev-isolate unless implementation finds a concrete blocker:

- `web/src/admin/components/TriptychSwitcher.tsx`
- `web/src/admin/components/TriptychSwitcher.module.css`
- `web/src/admin/components/RailNav.tsx`
- `web/src/admin/components/RailNav.module.css`
- `web/src/admin/components/CanvasNav.tsx`
- `web/src/admin/components/CanvasNav.module.css`
- `web/src/admin/components/TodayCanvas.tsx` or fold its spatial tile idea into `VoyageMap`
- `web/src/admin/design/types.ts`
- `web/src/admin/design/DesignModeProvider.tsx`
- production `data-palette`, `data-layout`, and `data-motion` switching
- `ui_redesign_switcher` in `internal/httpapi/dev_flags_handlers.go`
- matching `ui_redesign_switcher` type plumbing in `web/src/lib/devFlags.ts`
- four non-winning palette blocks in `web/src/styles/themes.css`
- `[data-palette]`, `[data-layout]`, and `[data-motion]` selectors that no longer serve production

`web/src/admin/components/CommandBar.tsx` already exists. Repurpose, rename, or move it into the cockpit command palette. Do not ship two overlapping command launchers.

### Fixture Strategy

Ship two rich fixtures in `web/src/admin/cockpit/fixtures.ts`:

- Org Admin fixture: three active voyages, two upcoming voyages, four blockers, two open folios over `$5k`, one low-stock signal, lead/quote pressure, and six activity events.
- Cruise Director fixture: two assigned trips, no fleet pulse, no org-wide inventory, restricted activity, and director-specific commands.

The cockpit may use fixtures when the local database is empty or behind an explicit development-only toggle. The visual review cannot depend on an empty seed state.

## Implementation Plan

### Phase 1: Reboot Lock and Docs Skeleton (~10%)

**Files:**
- `docs/decisions/0005-voyage-cockpit-reboot.md`
- `docs/decisions/0004-triptych-runtime-evaluation.md`
- `DESIGN.md`
- `CODEX.md`
- `CLAUDE.md`

**Tasks:**
- [ ] Create ADR 0005 for the cockpit reboot, visual direction, and deletion posture.
- [ ] Amend ADR 0004 status to superseded by ADR 0005.
- [ ] Update `DESIGN.md` with the cockpit visual contract.
- [ ] Update `CODEX.md` and `CLAUDE.md` to prohibit new page-level `app.css` dependencies and require cockpit-first frontend patterns.

### Phase 2: Triptych Collapse (~20%)

**Files:**
- `web/src/admin/Shell.tsx`
- `web/src/admin/nav.ts`
- `web/src/admin/design/*`
- `web/src/admin/components/{TriptychSwitcher,RailNav,CanvasNav,TodayCanvas,CommandBar}.*`
- `internal/httpapi/dev_flags_handlers.go`
- `web/src/lib/devFlags.ts`
- `web/src/styles/{tokens,themes,motion,admin,base,app}.css`
- `web/src/main.tsx`

**Tasks:**
- [ ] Collapse winning semantic tokens into `:root`; remove production `[data-palette]` blocks.
- [ ] Remove production URL/localStorage design switching.
- [ ] Delete Triptych switcher and dev flag plumbing.
- [ ] Replace shell layout dispatch with one production cockpit shell.
- [ ] Delete losing nav renderers and fold any useful spatial idea into `VoyageMap`.
- [ ] Grep `admin.css` for `[data-` and remove dead selectors that can still win specificity.
- [ ] Keep auth and existing guest pages rendering after token collapse.

### Phase 3: Cockpit Aggregate API (~20%)

**Files:**
- `internal/httpapi/cockpit_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/store/cockpit.go`
- `internal/store/cockpit_test.go`
- `internal/httpapi/cockpit_handlers_test.go`
- `web/src/admin/api.ts`

**Tasks:**
- [ ] Add `GET /api/admin/cockpit`.
- [ ] Compose bounded projections from existing trips, guests, cabins, folios, inventory, FX, audit, lifecycle, and shipped Sprint 026 Phase 1 data.
- [ ] Add Org Admin response shape.
- [ ] Add Cruise Director assigned-trip response shape.
- [ ] Add graceful empty state for a fresh organization.
- [ ] Add store tests for cross-org isolation and director assignment scoping.
- [ ] Add HTTP tests for auth, role filtering, and no cross-tenant leakage.

### Phase 4: Cockpit UI (~35%)

**Files:**
- `web/src/admin/cockpit/Cockpit.tsx`
- `web/src/admin/cockpit/Cockpit.module.css`
- `web/src/admin/cockpit/CockpitApi.ts`
- `web/src/admin/cockpit/fixtures.ts`
- `web/src/admin/cockpit/CommandPalette.tsx`
- `web/src/admin/cockpit/components/*`
- `web/src/admin/pages/Overview.tsx`
- optional generated or curated assets in `web/public/`

**Tasks:**
- [ ] Build cockpit against the two fixtures first.
- [ ] Build `VoyageMap`, `SignalStack`, `ActionRail`, `MoneyStrip`, `ActivityStream`, and `ReadinessMatrix`.
- [ ] Repurpose or move existing `CommandBar` into `CommandPalette`.
- [ ] Wire real API data after fixture pass.
- [ ] Add loading, empty, partial-error, and unauthorized states.
- [ ] Every cockpit tile resolves to an existing route or mutation workflow.
- [ ] Route-scoped immersive visual treatment renders with correct contrast.
- [ ] Mobile layout works at `375 x 667` and `414 x 896` without clipped controls or horizontal scroll.

### Phase 5: Overview to Cockpit (~10%)

**Files:**
- `web/src/admin/pages/Overview.tsx`
- `web/src/admin/components/index.ts`
- `web/src/styles/app.css`

**Tasks:**
- [ ] `/admin` renders `Cockpit`.
- [ ] Delete the old role/layout branching in `Overview.tsx`.
- [ ] Delete `CruiseDirectorLanding`; director landing is the director cockpit.
- [ ] Ensure onboarding auto-show still works.
- [ ] Do not migrate Trips, TripDashboard, TripManifest, TripConsumptionLedger, or GuestFolio in Sprint 027; make Sprint 028 own that migration.

### Phase 6: Verification and Close (~5%)

**Files:**
- `docs/sprints/SPRINT-027.md`
- touched test files

**Tasks:**
- [ ] Run `make test`.
- [ ] Run `npm run build`.
- [ ] Start the dev server and inspect Org Admin and Cruise Director cockpits.
- [ ] Smoke auth pages after token collapse.
- [ ] Verify reduced-motion behavior.
- [ ] Capture implementation notes and sync sprint tracker.

## API Endpoints

| Endpoint | Method | Auth | Purpose |
|---|---:|---|---|
| `/api/admin/cockpit` | GET | Session | Role-aware cockpit aggregate. |

No public endpoints are added in Sprint 027.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `docs/decisions/0005-voyage-cockpit-reboot.md` | Create | Reboot ADR. |
| `docs/decisions/0004-triptych-runtime-evaluation.md` | Modify | Mark Triptych superseded. |
| `DESIGN.md` | Modify | Cockpit visual contract. |
| `CODEX.md`, `CLAUDE.md` | Modify | Cockpit-first implementation guidance. |
| `internal/httpapi/cockpit_handlers.go` | Create | Cockpit endpoint. |
| `internal/store/cockpit.go` | Create | Aggregate store projections. |
| `web/src/admin/cockpit/*` | Create | Cockpit UI and fixtures. |
| `web/src/admin/pages/Overview.tsx` | Modify | Replace old Overview with Cockpit. |
| `web/src/admin/Shell.tsx` | Modify | Single production shell. |
| `web/src/admin/design/*` | Delete/Modify | Remove production Triptych switching. |
| `web/src/admin/components/{TriptychSwitcher,RailNav,CanvasNav,TodayCanvas}.*` | Delete/Modify | Remove evaluation scaffolding. |
| `web/src/admin/components/CommandBar.*` | Move/Modify | Become Cockpit command palette or shared primitive. |
| `internal/httpapi/dev_flags_handlers.go` | Modify | Remove redesign switcher flag. |
| `web/src/lib/devFlags.ts` | Modify | Remove redesign switcher type. |
| `web/src/styles/{tokens,themes,motion,admin,base,app}.css` | Modify | Collapse tokens and remove dead data-attribute selectors. |

## Definition of Done

- [ ] `/admin` renders the Voyage Cockpit, not the old Overview card grid.
- [ ] Org Admin and Cruise Director both have first-class cockpit variants.
- [ ] `GET /api/admin/cockpit` returns real role-scoped data.
- [ ] Cockpit reads shipped Sprint 026 Phase 1 data where useful without exposing new public/funnel/portal UI.
- [ ] Org Admin responses are org-scoped; Cruise Director responses are assigned-trip scoped.
- [ ] A fresh org gets a polished empty cockpit, not a blank canvas.
- [ ] Rich Org Admin and Cruise Director fixtures exist and cover non-empty states.
- [ ] Command palette supports keyboard open, search, route selection, escape close, focus return, and visible focus rings.
- [ ] Production Triptych switching is gone; `?triptych=` and `localStorage["triptych"]` cannot alter production rendering.
- [ ] `themes.css` has zero production `[data-palette]` blocks; semantic tokens live in `:root`.
- [ ] `web/src/admin/design/` is deleted or reduced to a tiny reduced-motion helper.
- [ ] `grep -rE 'admin-card|admin-page-header|admin-page-title|setup-list__|chip--' web/src/admin/cockpit web/src/admin/pages/Overview.tsx` returns zero matches.
- [ ] Cockpit renders without clipped controls at `375 x 667` and `414 x 896`; no horizontal scroll.
- [ ] Reduced-motion users do not get ambient or page-transition motion.
- [ ] Auth pages still render after token collapse.
- [ ] `make test` passes, or skips only for documented local PostgreSQL availability.
- [ ] `npm run build` passes.
- [ ] Documentation Manifest items are complete.

## Security Considerations

- Every cockpit query is scoped by organization.
- Cruise Director cockpit data is limited to assigned trips and role-allowed signals.
- Guest PII exposure must not exceed existing manifest/folio pages for the same role.
- Cockpit commands are navigation shortcuts first; mutations continue through existing protected endpoints.
- Aggregate response projections must enumerate fields explicitly. No `SELECT *`.
- Recent activity feed must respect existing audit visibility boundaries.
- Removing production URL-driven design attributes reduces UI state injection risk.

## Dependencies

- Existing shipped foundations from auth, trips, guests, cabins, folios, inventory, FX, audit, lifecycle, and Sprint 026 Phase 1 data foundations.
- Sprint 025 component-library concepts, but not production runtime switching.
- No new backend dependencies.
- Frontend visual/icon dependencies may be added only if they materially improve cockpit quality and are documented.

## Documentation Manifest

The implementation sprint MUST land the following docs changes alongside the code. The `sprint` skill verifies each file in this list was modified before marking the sprint complete.

### New ADRs

- `docs/decisions/0005-voyage-cockpit-reboot.md` — Codifies Sprint 027 as the product reboot, the authenticated cockpit as the app spine, the route-scoped immersive visual direction, the Triptych deletion posture, and the decision to defer public/guest expansion until the cockpit is excellent.

### Amended ADRs

- `docs/decisions/0004-triptych-runtime-evaluation.md` — Mark status as superseded by ADR 0005; record which Triptych pieces were deleted or folded into Cockpit.

### Cross-cutting docs

- `DESIGN.md` — Rewrite or substantially amend with the cockpit visual contract, motion rules, asset/background policy, command surface rules, and legacy CSS deletion policy.
- `CODEX.md` — Update frontend notes to require cockpit-first patterns and prohibit new page-level `app.css` dependencies.
- `CLAUDE.md` — Mirror the cockpit-first guidance and deletion posture.
- `docs/product/personas.md` — Amend role landing expectations: Org Admin and Cruise Director now land on role-specific cockpit variants.
- `docs/product/organization-admin-user-stories.md` — Note that cockpit consolidates existing operational oversight stories into the first screen; do not tick unshipped public funnel/portal stories.

### Skipped (with reasoning)

- `docs/sprints/SPRINT-026.md` — Not amended in Sprint 027. Phase 1 already shipped; remaining phases are non-binding context and future work can be replanned from the cockpit spine.
- New ADR for the cockpit aggregate endpoint — skipped because the aggregate is an implementation detail under the reboot ADR unless persistent derived state or caching is introduced.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Cockpit becomes decorative instead of operational | High | Fixtures and DoD require real blockers, money, activity, commands, and every tile resolving to an existing workflow. |
| Both role cockpits over-scope the sprint | High | Share endpoint, components, command registry, and visual primitives; role-specific composition only. |
| Deleting Triptych breaks auth or legacy pages | Medium | Smoke auth pages; keep a small compatibility layer only where needed; remove dead selectors deliberately. |
| Aggregate endpoint becomes slow or leaky | High | Bounded projections, explicit fields, org/assignment tests, no unbounded event history. |
| Visual ambition hurts usability | Medium | Fixed dimensions, mobile QA, reduced-motion support, and dense detail pages remain utilitarian until Sprint 028 migration. |
| Existing Sprint 026 Phase 1 files confuse direction | Medium | Cockpit may read shipped data, but public/funnel/portal UI remains paused. |
| Onboarding auto-show regresses | Medium | Preserve and smoke the `/admin` onboarding redirect behavior in `Shell.tsx`. |

## Deferred to Sprint 028

- Trips page migration.
- TripDashboard migration.
- TripManifest migration.
- TripConsumptionLedger migration.
- GuestFolio migration.
- Any public catalog/funnel UI.
- Guest portal UI.
- Crew/equipment UI expansion beyond cockpit projections.

## References

- `docs/sprints/drafts/SPRINT-027-INTENT.md`
- `docs/sprints/drafts/SPRINT-027-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-027-CODEX-DRAFT-CLAUDE-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-027-MERGE-NOTES.md`
- `CODEX.md`
- `CLAUDE.md`
- `DESIGN.md`
- `docs/decisions/0004-triptych-runtime-evaluation.md`
