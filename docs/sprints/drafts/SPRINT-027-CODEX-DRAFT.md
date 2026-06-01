# Sprint 027 Codex Draft: Voyage Cockpit Reboot

## Overview

Sprint 027 is a product reboot, not another feature sprint. The application has enough backend muscle to be useful, but the experience still feels like a themed admin dashboard assembled one page at a time. This sprint pauses the Sprint 026 expansion into public funnel, guest portal, crew/equipment, and quote flows, and instead turns the authenticated app into a spectacular live operations cockpit.

The new center of gravity is `/admin`: a full-bleed **Voyage Cockpit** that shows the operator what matters now: active and upcoming voyages, boat readiness, manifest pressure, cabin fill, folio exposure, ledger activity, trip lifecycle blockers, FX/payment readiness, and urgent actions. It uses the current `manta-night` direction and original background as the brand atmosphere, but collapses the UI into one decisive product language: dark luminous chrome, map/deck-like spatial composition, sharp data surfaces, large status typography, tactile controls, and no remaining Triptych indecision in production.

This sprint deliberately rejects a "more pages" answer. The goal is to make the core daily surface so good that every future funnel, portal, crew, and equipment feature has a design system and workflow spine to plug into.

## Product Stance

### What We Keep

- Existing Go backend, PostgreSQL store, org scoping, sessions, auth, RBAC, and audit posture.
- Existing domain foundations: boats, trips, cabin assignments, guest manifest, registration status, folios, ledger, inventory, reports, onboarding, FX/payment settings.
- Current user-selected visual baseline: `manta-night` over the original Sprint 011 background composite.
- The Sprint 025 component-library idea, but not the indecision of multiple production layouts.

### What We Pause

- Sprint 026 public catalog/funnel.
- Sprint 026 guest portal.
- Sprint 026 crew/equipment/readiness expansion.
- New public routes and quote/deposit state machines.
- Any new major backend table family unless required for the cockpit aggregate.

### What We Delete or Collapse

- Production Triptych switching.
- Production layout dispatch between `rail`, `spaces`, and `canvas`.
- Production palette switching between five palettes.
- The idea that the app's primary screen is a generic Overview card grid.
- The practice of page files depending on broad legacy classes from `app.css`.

## Use Cases

1. **Owner opens the app and understands the fleet in five seconds.** They land on `/admin` and see a cinematic but dense command surface: active voyage, next departures, readiness blockers, guest/cabin fill, open folios, low-stock risks, and money exposure.
2. **Cruise Director boards with a clear run sheet.** Their `/admin` cockpit is scoped to assigned trips and puts Start Trip, manifest readiness, berth gaps, guest registration/doc gaps, and ledger actions within one click.
3. **Operator acts without hunting.** A command palette and cockpit action rail route directly to "Invite guest", "Assign cabin", "Record consumption", "Close folio", "Add FX fallback", "Import trips", and "Open readiness blockers".
4. **The app looks premium and ownable.** The UI reads as a dive expedition operations system, not a generic SaaS template with ocean colors.
5. **Legacy pages still work while being visually contained.** Existing pages render inside the new shell and receive global token updates, but the sprint only fully migrates the cockpit and the highest-frequency trip pages.
6. **Future Sprint 026 features have a home.** Funnel, portal, crew, and equipment work can later enter through cockpit panels and action feeds instead of adding disconnected pages.

## Architecture

### New Product Model: Operations Cockpit

The cockpit is an authenticated command surface backed by a single aggregate endpoint.

```
GET /api/admin/cockpit
┌──────────────────────────────────────────────────────────────┐
│ identity + role + org                                         │
│ fleet pulse: boats, trips, active voyages, next departure     │
│ voyage lanes: active/planned trips with status + progress     │
│ readiness: director, manifest, cabins, registration, FX       │
│ revenue: open folios, closed folios, unpaid exposure          │
│ inventory: low-stock and negative-stock warnings              │
│ activity: latest audit events and ledger entries              │
│ actions: role-aware primary commands                          │
└──────────────────────────────────────────────────────────────┘
```

The endpoint composes existing store queries where possible. It does not create new business concepts; it packages existing state into a product-grade view model.

### Frontend Shell

`AdminShell` becomes a single cockpit shell:

- Permanent layout: `cockpit`.
- Palette: `manta-night` mapped into `:root` semantic tokens.
- Motion: `living`, reduced-motion respected.
- Navigation: a compact command rail plus command palette. Spaces-style grouped navigation remains as a secondary drawer or compact menu, not the primary product metaphor.
- `TriptychSwitcher` is removed from production and, if retained, moved behind a dev-only route or deleted.

### Visual Language

The reboot uses one visual language:

- Full-bleed authenticated app surface with original background visible as atmosphere at the edges.
- Main cockpit surface as a single luminous glass-metal workplane, not separate floating cards nested in cards.
- Spatial trip lanes that feel like voyage tracks: active trip center, upcoming trips as radial/linear lanes, blockers as hot markers.
- High-contrast data tiles with fixed dimensions and no layout shift.
- Accent hierarchy:
  - cyan: primary operational action/readiness
  - violet: navigation/selection
  - magenta/coral: urgent blockers and destructive risk
  - ember/gold: money and warnings
- Tables remain dense and utilitarian inside detail pages.

### Backend Aggregate Types

Add focused DTOs in `internal/httpapi/cockpit_handlers.go` and store helpers as needed:

```go
type CockpitResponse struct {
    Me          CockpitIdentity      `json:"me"`
    Fleet       FleetPulse           `json:"fleet"`
    Voyages     []VoyagePulse        `json:"voyages"`
    Readiness   []ReadinessSignal    `json:"readiness"`
    Money       MoneyPulse           `json:"money"`
    Inventory   []InventorySignal    `json:"inventory"`
    Activity    []ActivitySignal     `json:"activity"`
    Actions     []CockpitAction      `json:"actions"`
}
```

Every query is scoped by `organization_id`. Cruise Directors receive only assigned trips and role-allowed actions.

### UI Structure

```
web/src/admin/cockpit/
├── Cockpit.tsx
├── Cockpit.module.css
├── CockpitApi.ts
├── CommandPalette.tsx
├── CommandPalette.module.css
├── components/
│   ├── VoyageMap.tsx
│   ├── VoyageMap.module.css
│   ├── SignalStack.tsx
│   ├── ActionRail.tsx
│   ├── MoneyStrip.tsx
│   ├── ActivityStream.tsx
│   └── ReadinessMatrix.tsx
└── fixtures.ts
```

Use static fixtures first for visual build-out, then wire to the aggregate endpoint. This keeps the design iteration fast without waiting on every backend query.

### Route Strategy

- `/admin` renders `Cockpit`.
- Existing detail pages remain:
  - `/admin/trips`
  - `/admin/trips/:id/dashboard`
  - `/admin/trips/:id/manifest`
  - `/admin/trips/:id/ledger`
  - `/admin/trips/:id/guests/:guestId/folio`
  - configuration routes
- The cockpit links into these pages through explicit command actions and voyage tiles.

### Page Migration Scope

Fully migrate only the cockpit and the critical trip run-loop:

- `Overview.tsx` is replaced by or delegates to `Cockpit`.
- `Trips.tsx`
- `TripDashboard.tsx`
- `TripManifest.tsx`
- `TripConsumptionLedger.tsx`
- `GuestFolio.tsx`

All five should use component primitives or page-local modules, not `app.css` legacy classes.

## Implementation Plan

### Phase 1: Reboot Lock and Dead-Scope Control (~10%)

**Files:**
- `docs/sprints/SPRINT-026.md`
- `docs/decisions/0004-triptych-runtime-evaluation.md`
- `DESIGN.md`
- `CODEX.md`
- `CLAUDE.md`

**Tasks:**
- [ ] Mark Sprint 026 as superseded/deferred by Sprint 027 in its overview, without deleting the plan.
- [ ] Amend ADR 0004: final production direction is no longer Triptych evaluation; it is the Voyage Cockpit.
- [ ] Update `DESIGN.md` to describe the cockpit visual language.
- [ ] Update agent guidance to prefer the cockpit component system and reject broad `app.css` additions.

### Phase 2: Collapse Triptych Into One Production Shell (~15%)

**Files:**
- `web/src/admin/Shell.tsx`
- `web/src/admin/design/types.ts`
- `web/src/admin/design/DesignModeProvider.tsx`
- `web/src/admin/components/TriptychSwitcher.tsx`
- `web/src/admin/components/{RailNav,CanvasNav,SpacesNav}.tsx`
- `web/src/styles/themes.css`
- `web/src/styles/motion.css`
- `web/src/styles/admin.css`
- `web/src/styles/base.css`
- `web/src/main.tsx`

**Tasks:**
- [ ] Make `manta-night` the root semantic token set instead of a switchable palette block.
- [ ] Remove production URL/localStorage design override behavior.
- [ ] Delete or dev-isolate `TriptychSwitcher`.
- [ ] Replace layout dispatch in `AdminShell` with the cockpit shell.
- [ ] Keep role filtering centralized.
- [ ] Preserve reduced-motion behavior.
- [ ] Verify `?triptych=` cannot change production rendering.

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
- [ ] Build role-aware aggregate response for Org Admin and Cruise Director.
- [ ] Include active and upcoming trips, assignment gaps, guest registration gaps, cabin gaps, open folio counts, low/negative inventory signals, latest audit events, and FX readiness.
- [ ] Add store tests for organization scoping and Cruise Director assignment scoping.
- [ ] Add HTTP tests for auth, role filtering, and response shape.

### Phase 4: Build the Voyage Cockpit UI (~25%)

**Files:**
- `web/src/admin/cockpit/*`
- `web/src/admin/pages/Overview.tsx`
- `web/src/admin/components/index.ts`
- `web/src/styles/tokens.css`
- `web/src/styles/base.css`

**Tasks:**
- [ ] Build the cockpit against local fixtures first.
- [ ] Create `VoyageMap`, `SignalStack`, `ActionRail`, `MoneyStrip`, `ActivityStream`, and `ReadinessMatrix`.
- [ ] Wire the real `GET /api/admin/cockpit` endpoint.
- [ ] Render Org Admin and Cruise Director variants from the same components.
- [ ] Add loading, empty, partial-error, and unauthorized states.
- [ ] Ensure all tiles and action controls have stable responsive dimensions.
- [ ] Use icons for action controls where the app has an icon dependency; if not, add one deliberately or use compact text only where necessary.

### Phase 5: Critical Run-Loop Page Migration (~20%)

**Files:**
- `web/src/admin/pages/Trips.tsx`
- `web/src/admin/pages/TripDashboard.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/pages/TripConsumptionLedger.tsx`
- `web/src/admin/pages/GuestFolio.tsx`
- Corresponding `*.module.css` files
- `web/src/styles/app.css`

**Tasks:**
- [ ] Move the five pages off `.admin-card`, `.admin-page-header`, `.chip--*`, and broad legacy layout classes.
- [ ] Use semantic tokens and component primitives.
- [ ] Preserve all existing behavior.
- [ ] Add cockpit return links and command affordances where appropriate.
- [ ] Shrink `app.css` by removing rules no longer referenced by migrated pages.

### Phase 6: Visual QA, Interaction QA, and Guardrails (~10%)

**Files:**
- `web/package.json` if visual tooling scripts already exist or are needed.
- `web/src/admin/cockpit/*.test.*` only if a test framework exists; otherwise keep verification manual and documented.
- `docs/sprints/SPRINT-027.md`

**Tasks:**
- [ ] Run `npm run build`.
- [ ] Run `make test`.
- [ ] Start dev server and inspect desktop and mobile viewports.
- [ ] Verify cockpit has no blank states with seeded/dev data.
- [ ] Verify no text overlap at mobile width.
- [ ] Verify keyboard command palette navigation.
- [ ] Verify production build ignores Triptych URL/localStorage.
- [ ] Document screenshots or QA notes in the sprint doc implementation notes.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/cockpit` | GET | Role-aware cockpit aggregate for the authenticated user's organization and assignment scope. |

No public endpoints are added in this sprint.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `docs/sprints/SPRINT-026.md` | Modify | Mark as deferred/superseded by cockpit reboot. |
| `docs/decisions/0004-triptych-runtime-evaluation.md` | Modify | Close Triptych evaluation and record cockpit decision. |
| `docs/decisions/0005-voyage-cockpit-reboot.md` | Create | Codify reboot posture and product shell decision. |
| `DESIGN.md` | Rewrite/Modify | Replace timid dashboard framing with cockpit visual system. |
| `CODEX.md`, `CLAUDE.md` | Modify | Update frontend guidance and cockpit-first workflow. |
| `internal/httpapi/cockpit_handlers.go` | Create | Cockpit aggregate HTTP handler. |
| `internal/store/cockpit.go` | Create | Store queries for aggregate signals. |
| `web/src/admin/cockpit/*` | Create | Cockpit UI and components. |
| `web/src/admin/Shell.tsx` | Modify | Single production cockpit shell. |
| `web/src/admin/design/*` | Modify/Delete | Remove production design mode switching. |
| `web/src/admin/components/TriptychSwitcher.*` | Delete or dev-isolate | Remove production evaluation dock. |
| `web/src/styles/{tokens,themes,motion,admin,base,app}.css` | Modify | Collapse tokens and remove superseded legacy CSS. |
| `web/src/admin/pages/{Overview,Trips,TripDashboard,TripManifest,TripConsumptionLedger,GuestFolio}.tsx` | Modify | Cockpit and critical run-loop migration. |

## Definition of Done

- [ ] `/admin` is the Voyage Cockpit for Org Admins and Cruise Directors.
- [ ] The cockpit renders real role-scoped aggregate data from `GET /api/admin/cockpit`.
- [ ] Org Admins never see another organization's data; Cruise Directors see only assigned trip operational data.
- [ ] Triptych production switching is removed or fully dev-isolated.
- [ ] `?triptych=` and `localStorage["triptych"]` do not alter production rendering.
- [ ] `manta-night` plus the original Sprint 011 background is the only production authenticated visual baseline.
- [ ] Cockpit and five critical run-loop pages no longer depend on broad legacy `app.css` classes.
- [ ] `app.css` shrinks materially and no new page depends on it.
- [ ] Desktop and mobile layouts have no text overlap, clipped buttons, or incoherent stacking.
- [ ] Command palette supports keyboard open, search, route selection, escape close, and focus return.
- [ ] `make test` passes or skips only for documented local Postgres availability.
- [ ] `npm run build` passes.
- [ ] Documentation Manifest items are completed.

## Security Considerations

- Cockpit aggregate must be organization-scoped on every query.
- Cruise Director data must be assignment-scoped; detail links must still rely on backend authorization.
- The cockpit must not leak guest PII beyond what the role already sees on existing manifest/folio pages.
- Command actions are navigation shortcuts only; mutations still occur through existing protected endpoints.
- Removing runtime design URL reflection reduces DOM attribute injection risk from untrusted query strings.
- Activity feed must respect existing audit visibility boundaries.

## Dependencies

- Depends on completed foundations from Sprints 009-024.
- Intentionally supersedes or precedes Sprint 026.
- Uses Sprint 025 component library concepts but collapses the runtime evaluation model.
- No new external backend dependencies.
- If an icon set is added for command/action controls, prefer a small frontend-only dependency and document it.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Cockpit becomes another pretty dashboard without operational depth | High | Aggregate must surface actionable blockers and route to concrete workflows; DoD requires role-scoped real data. |
| Scope balloons into rebuilding every page | High | Only cockpit plus five run-loop pages migrate; all other pages remain functional legacy surfaces. |
| Visual ambition hurts usability | Medium | Dense data tables remain utilitarian; cockpit uses fixed dimensions, responsive checks, and reduced-motion support. |
| Removing Triptych loses useful experiments | Low | Preserve a screenshot or ADR record; keep dev-only references only if they do not affect production code. |
| Aggregate endpoint duplicates business logic | Medium | Store helpers compute summaries from existing tables; mutations and canonical rules stay in existing services. |
| Sprint 026 plans become stale | Medium | Explicitly amend Sprint 026 as deferred and later rebase funnel/portal/crew work onto cockpit patterns. |

## Documentation Manifest

The implementation sprint MUST land the following docs changes alongside the code. The `sprint` skill verifies each file in this list was modified before marking the sprint complete.

### New ADRs

- `docs/decisions/0005-voyage-cockpit-reboot.md` — Codifies the reboot decision: authenticated app becomes a single Voyage Cockpit shell, Triptych evaluation is closed, and public/guest expansion waits until the cockpit spine is excellent.

### Amended ADRs

- `docs/decisions/0004-triptych-runtime-evaluation.md` — Amend status from partial evaluation to superseded/closed; record why the runtime switcher is removed or dev-isolated.

### Cross-cutting docs

- `DESIGN.md` — Replace the old dashboard framing with the Voyage Cockpit visual system, token rules, motion rules, and page-migration rules.
- `CODEX.md` — Update frontend notes: cockpit-first shell, no new `app.css` page dependencies, Triptych closed.
- `CLAUDE.md` — Mirror cockpit and design guidance for Claude Code.
- `docs/sprints/SPRINT-026.md` — Mark Sprint 026 as deferred/superseded by Sprint 027 and note how funnel/portal/crew work should resume later.
- `docs/product/personas.md` — Amend Cruise Director/Admin landing expectations if cockpit changes the role-specific first screen.
- `current_status.md` — If present, update the product status and known gaps. If absent, note absence in implementation notes.
- `local_setup.md` — Update only if new frontend scripts, fixtures, or icon dependencies are added.

### Skipped (with reasoning)

- New ADR for cockpit aggregate endpoint — skipped because it is an implementation shape under the cockpit reboot ADR, not a separate architectural decision unless it introduces caching or persistent derived state.

## Open Questions

- Should Sprint 026 be formally marked `skipped`, or remain `planned` but deferred?
- Should the non-winning Triptych code be deleted completely or moved behind a dev-only archive route for reference?
- Should the cockpit use a new icon dependency such as `lucide-react`, or stay dependency-free?
- Should the cockpit introduce visual screenshots as checked-in QA artifacts, or keep screenshot verification manual?
