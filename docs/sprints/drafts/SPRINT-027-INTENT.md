# Sprint 027 Intent: Reboot the Product Into a Voyage Cockpit

## Seed

kplan I want you to review all the sprints and work done in this project and rethink it. I think the application is mediocre and needs a reboot. Be bold. Be aggressive with your design thinking and create something spectacular and beautiful including the UI too

## Context

The repository is a multi-tenant SaaS platform for scuba diving liveaboard operators. The backend has become materially capable: custom auth, organization setup, fleet/trips, imports, catalog/inventory, checkout currency, guest registration, cabin assignments, audit/documents, trip lifecycle gates, consumption ledger, reports, onboarding, and automated FX are all represented in code or sprint plans.

The app's weakness is now product shape and execution quality, not lack of domain primitives. The admin UI has grown feature-by-feature: many pages still depend on `web/src/styles/app.css` legacy classes, the Triptych switcher created several design options without collapsing them into a decisive product language, and Sprint 026 proposes adding public funnel, guest portal, crew/equipment/readiness, and partial page migrations before the main operating experience is excellent. That is risky: it expands surface area while the core app still feels like a themed CRUD dashboard.

The reboot should be a hard pivot from "admin dashboard with nautical styling" to a spectacular live operations cockpit for voyages: one primary command surface that makes the boat, trip, guests, cabins, ledger, readiness, alerts, and money feel spatial, immediate, and premium. It should keep the working backend foundations and replace the mediocre product composition around them.

There is no `AGENTS.md` in this repo despite the kplan template naming it. Project conventions live in `CODEX.md`, `CLAUDE.md`, `DESIGN.md`, sprint docs, and ADRs.

## Recent Sprint Context

### Sprint 024: Automated FX Rates via Frankfurter

Sprint 024 adds an `internal/fxauto` package, daily Frankfurter refresh, payment-settings freshness indicators, and on-demand refresh when a supported currency is added. This improves operational trust around checkout currencies, but it is still a supporting capability.

### Sprint 025: Triptych — Three Bold UIs, Judged in the Live App

Sprint 025 introduced runtime design modes, a component library, palettes, layout variants, motion modes, and a dev-only switcher. Its decision log shows several rejected visual rounds and the current user preference: `manta-night` with the original Sprint 011 background. It did not finish the migration: `app.css` remains large and many pages still use legacy `.admin-card`, `.admin-page-header`, `.chip--*`, and related classes.

### Sprint 026: Open the Funnel, Portal, and Operations Spine

Sprint 026 plans a very large expansion: public catalog/funnel, quote/deposit flow, guest portal, crew roster, equipment service history, readiness gates, Triptych production lock, and five page migrations. This is ambitious but risks worsening mediocrity by adding more pages before establishing a superior core experience. For a reboot, Sprint 026 should be paused or split; the product should first become excellent where operators live every day.

## Relevant Codebase Areas

- `web/src/main.tsx` — route map and CSS load order; still imports `app.css`.
- `web/src/admin/Shell.tsx` — current admin shell, layout dispatch, Triptych switcher mount.
- `web/src/admin/nav.ts` — canonical nav data grouped into Operations, Configuration, Insights.
- `web/src/admin/design/*` — runtime design mode, default `manta-night / spaces / living`.
- `web/src/admin/components/*` — component library from Sprint 025.
- `web/src/admin/pages/Overview.tsx` — current Today/Overview surface; still mostly legacy classes except canvas mode.
- `web/src/admin/components/TodayCanvas.tsx` — first spatial landing experiment, but constrained by existing overview data.
- `web/src/styles/{tokens,themes,motion,admin,base,app}.css` — current styling split; `app.css` is still 39 KB and legacy-heavy.
- `internal/httpapi/*` and `internal/store/*` — existing operational APIs and org-scoped store methods.
- `docs/decisions/0004-triptych-runtime-evaluation.md` — accepted Triptych ADR with partial implementation notes and collapse procedure.
- `DESIGN.md` — current design decisions log, including the user's repeated rejection of timid visual directions.

## Constraints

- Must follow project conventions in `CODEX.md` and `CLAUDE.md`.
- Must integrate with the existing Go + React/Vite architecture.
- Must preserve strict organization scoping and persona boundaries from `docs/product/personas.md`.
- Must follow sprint conventions in `docs/sprints/README.md`.
- Must include a `## Documentation Manifest`.
- Must not depend on adding a new design toolchain or cloud infrastructure.
- Must avoid expanding public/guest/funnel surface area until the core authenticated operating surface is rebooted.
- Must preserve the user's selected visual direction unless deliberately superseded: `manta-night` plus original Sprint 011 body background is the current baseline.

## Success Criteria

- The next sprint plan is a true reboot, not a continuation of incremental feature accretion.
- The plan identifies what to pause, delete, collapse, or defer from Sprints 025-026.
- The plan gives implementers a concrete architecture for a spectacular core experience: a spatial voyage cockpit, operational command palette, premium visual system, and focused workflow spine.
- The plan has testable acceptance criteria, verification strategy, and clear documentation updates.
- The scope is bold but implementable in one sprint by reusing existing backend primitives and constraining backend changes to focused aggregate endpoints.

## Open Questions

- Should Sprint 026 be formally superseded by Sprint 027, or should Sprint 027 be inserted before implementing Sprint 026?
- Should the reboot prioritize Org Admin, Cruise Director, or both in the first pass?
- Should the `canvas` layout become the permanent product direction, or should a new cockpit shell replace all Triptych layouts?
- How aggressive should deletion be: remove non-winning palettes/layouts now, or keep them behind dev tooling until the cockpit proves itself?
- Is the original body background sacred for the authenticated app, or can the cockpit use route-specific immersive media while preserving the original background on auth/public pages?
