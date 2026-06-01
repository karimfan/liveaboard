# Sprint 027 Merge Notes

## Codex Draft Strengths

- Correctly diagnosed the failure mode: broad backend capability, mediocre product composition.
- Correctly pivoted from feature accretion to a core authenticated operations cockpit.
- Proposed a single role-aware aggregate endpoint instead of scattered client orchestration.
- Preserved tenant isolation, RBAC, and existing mutation endpoints as security boundaries.
- Established a deletion posture for Triptych and legacy CSS.

## Claude Code Critique Strengths

- Grounded the plan in current `main`: Sprint 026 Phase 1 has already shipped data foundations and rate limiting.
- Identified over-scope: cockpit + Triptych collapse + five run-loop page migrations is too much for one sprint.
- Replaced vague visual language with a concrete contract: dimensions, density, tokens, and anti-patterns.
- Called out exact Triptych deletion targets in `Shell.tsx`, `themes.css`, `admin.css`, `dev_flags_handlers.go`, and `devFlags.ts`.
- Clarified command palette reuse: repurpose or move `CommandBar`; do not ship two overlapping command launchers.
- Strengthened fixture requirements and DoD checks.

## Valid Critiques Accepted

- Sprint 026 Phase 1 is already in the codebase. Sprint 027 pauses public/guest/funnel UI expansion, but may read shipped data such as leads, booking quotes, offline payments, crew certs, equipment, readiness, guest certs, and portal requests.
- Five extra page migrations are deferred to Sprint 028. Sprint 027 migrates `/admin` by replacing Overview with Cockpit; it does not also migrate Trips, TripDashboard, TripManifest, TripConsumptionLedger, and GuestFolio.
- Triptych switcher and losing options should be deleted, not dev-isolated.
- `themes.css` should collapse semantic tokens into `:root`; no production `[data-palette]` blocks.
- `DesignModeProvider` and `web/src/admin/design/` should be deleted or shrunk to a tiny reduced-motion helper.
- Visual language must be specified as grid, tile sizes, typography, token mapping, and anti-patterns.
- Fixture strategy is mandatory for judging the reboot.
- The Documentation Manifest should not reference nonexistent `current_status.md` or `local_setup.md`.

## Critiques Rejected or Modified

- Claude recommended amending `docs/sprints/SPRINT-026.md` in the manifest. The user said to go with 27 and ignore anything before; the final plan acknowledges Phase 1 landed but does not require editing Sprint 026 during Sprint 027.
- Claude suggested keeping the original background as a named option. The final plan makes the decision: Cockpit gets a route-scoped immersive background/workplane treatment; auth and non-cockpit public surfaces can keep the original Sprint 011 composite until redesigned.

## Interview Refinements Applied

- Sprint 027 is the reboot direction.
- Both Org Admin and Cruise Director cockpits are in scope.
- Weak UI/code may be removed at implementer discretion.
- Background and immersive visual direction is decided in the sprint plan.

## Final Decisions

- Sprint title: `Voyage Cockpit Reboot`.
- `/admin` becomes a role-specific Cockpit, replacing Overview.
- Sprint 027 scope is Cockpit + Triptych collapse + Overview migration + docs/verification.
- Daily run-loop page migrations move to Sprint 028.
- `GET /api/admin/cockpit` powers both cockpit variants with strict role scoping.
- Existing Sprint 026 Phase 1 store data is fair game for cockpit projection; public funnel/guest portal UI remains paused.
- Triptych production switching, losing palettes, losing layout renderers, `TriptychSwitcher`, and related dev flag plumbing are deleted.
- Cockpit gets a concrete visual contract: grid dimensions, tile dimensions, density rules, semantic token mapping, and anti-patterns.
- Cockpit may use a route-scoped immersive visual treatment; the original Sprint 011 background is not sacred for the authenticated cockpit.
