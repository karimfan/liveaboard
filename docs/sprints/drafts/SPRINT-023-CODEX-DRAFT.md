# Sprint 023 Codex Draft: Initial Org Onboarding Wizard

## Overview

Sprint 023 replaces the passive first-run setup checklist with a
prescriptive Org Admin onboarding wizard. A newly verified Admin should not
have to infer the order of setup work from the Overview card; the app should
guide them through the minimum useful sequence: choose currency, add/import
boats, create cabin layouts, and invite Cruise Directors.

The sprint should stay thin around existing editor surfaces. Currency,
fleet/import, cabin layout, and user invitations already work. The wizard's job
is to sequence those surfaces, show progress from the existing setup source of
truth, and provide clear exit/re-entry points. It should not fork the
organization profile form, cabin layout editor, import flow, or invite modal
unless a tiny shared component is needed to reuse existing code.

## Use Cases

1. **Brand-new Admin first run.** After email verification, an Org Admin whose
   setup is incomplete and who has not dismissed onboarding lands on the
   onboarding wizard instead of being dropped directly into Overview.
2. **Guided minimum setup.** The wizard walks through currency, boats, cabin
   layouts, and directors in that order, explaining the next operational task
   with direct calls to the existing editor screens.
3. **Optional adoption.** The Admin can skip the current step, skip all
   onboarding, or leave for a linked editor without losing access to the app.
4. **Post-import layout cleanup.** After a liveaboard.com import completes, the
   Admin can continue into the wizard's layouts step and see every boat in the
   org that still has no active cabin layout.
5. **Overview re-entry.** If onboarding has been dismissed but setup remains
   incomplete, the Overview setup card offers a clear "Open onboarding" action.
6. **Existing org compatibility.** Existing organizations that are already
   complete do not see onboarding automatically, even though the new dismissal
   timestamp starts as null.

## Architecture

### Product Shape

Use a dedicated route:

- `/admin/onboarding`
- optional query parameter `?step=currency|boats|layouts|directors`

A route is better than a modal for this sprint because it supports deep links
from Overview and post-import, survives refresh, works with browser back, and
does not require embedding several existing editor workflows inside an overlay.

The wizard shell is a compact page inside the existing Admin chrome:

- left/top stepper with four fixed steps;
- current step body with status, next action, and skip/continue controls;
- persistent "Skip onboarding" action;
- "Back to Overview" link;
- no marketing hero or decorative onboarding page.

### Scope Boundary

The wizard is a guided coordinator, not a second implementation of setup forms.

- **Currency step** links to `/admin/organization`; it may include a small
  inline quick-set control only if it reuses `CurrencyPicker` and the existing
  `PATCH /api/admin/organization` contract.
- **Boats step** links to `/admin/import/liveaboard`, `/admin/import/spreadsheet`,
  and `/admin/fleet` for manual fleet work. Manual boat creation is not added
  in this sprint unless it already exists by implementation time.
- **Layouts step** lists boats without active cabin layouts and links each row
  to `/admin/fleet/:id/cabins`.
- **Directors step** links to `/admin/users`; if the existing invite modal is
  extracted into a small reusable component, it may be embedded here, but the
  underlying invite API stays unchanged.

Trip creation remains out of scope for the wizard even though
`SetupCompleteness` currently includes trips. This sprint can keep trip setup
on the Overview/Reports setup signal without putting it in the first-run
wizard.

### Data Model

Add a single nullable timestamp to `organizations`:

```sql
ALTER TABLE organizations
  ADD COLUMN onboarding_dismissed_at timestamptz NULL;
```

Semantics:

- null means the Admin has not explicitly closed onboarding;
- non-null means do not auto-redirect into onboarding;
- completing all wizard steps should set the timestamp;
- "Skip onboarding" should set the timestamp immediately;
- "Skip this step" should advance to the next wizard step without mutating the
  setup source of truth;
- reopening from Overview does not clear the timestamp.

Do not backfill the timestamp for all existing orgs. Auto-show is gated by both
`onboarding_dismissed_at IS NULL` and incomplete onboarding steps, so already
complete orgs are not interrupted.

### Setup Signal

Extend `internal/store/reports.go` rather than creating a new setup subsystem.
`SetupCompleteness` is already the shared source for Overview and Reports.
Sprint 023 should add onboarding-specific data beside it:

```go
type OnboardingState struct {
    DismissedAt *time.Time
    Complete    bool
    Steps       []OnboardingStep
    BoatsWithoutLayouts []BoatWithoutLayout
}

type OnboardingStep struct {
    Key   string
    Label string
    Done  bool
    Hint  string
    Href  string
}

type BoatWithoutLayout struct {
    BoatID uuid.UUID
    Name   string
    Href   string
}
```

`OnboardingState` should be derived from current org data on each read:

- currency done when `organizations.currency` is non-empty;
- boats done when `BoatCountForOrg > 0`;
- layouts done when there is at least one boat and zero active boats have no
  active cabin layout;
- directors done when `CountActiveUsersByRole(..., RoleCruiseDirector) > 0`.

The existing `SetupCompleteness` steps should be updated to include layout
coverage as a measurable blocker, but the final sprint should decide whether
the current "trips" step remains part of the percent. The safest path is to
add a new `layouts` setup step and keep `trips` in Overview/Reports, while the
wizard renders only the four onboarding steps.

### Boats Without Layouts Query

Add a store helper that enumerates missing layouts in one org-scoped query:

```go
func (p *Pool) BoatsWithoutCabinLayouts(ctx context.Context, orgID uuid.UUID) ([]BoatLayoutStatus, error)
```

The query should count active cabins, not merely the presence of historical
layout rows. A boat needs setup when it has no active cabins or no active
berths. Use `LEFT JOIN` against `boat_cabins` / `boat_cabin_berths` with
`is_active = true`, grouped by boat, and filter by `boats.organization_id = $1`.

### HTTP API

Add Org Admin-only endpoints:

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/onboarding` | GET | Return `OnboardingState` plus current setup percent |
| `/api/admin/onboarding/dismiss` | POST | Set `organizations.onboarding_dismissed_at = now()` |

Potential response shape:

```json
{
  "dismissed_at": null,
  "complete": false,
  "setup_pct": 40,
  "steps": [
    {"key":"currency","label":"Set currency","done":false,"hint":"Required for local defaults","href":"/admin/organization"}
  ],
  "boats_without_layouts": [
    {"id":"...","name":"Gaia Love","href":"/admin/fleet/.../cabins"}
  ]
}
```

No per-step dismissal endpoint is needed for Sprint 023. Per-step "Skip" is
client navigation to the next step; whole-wizard dismissal is the persisted
choice.

### First-Run Routing

Implement the auto-show in the frontend, not with an API redirect:

- `AdminShell` or a small child guard fetches `/api/admin/onboarding` after
  `useMe` confirms `org_admin`;
- if the user is on `/admin`, state is not dismissed, and wizard steps are
  incomplete, navigate to `/admin/onboarding`;
- never redirect Cruise Directors;
- never redirect when already on `/admin/onboarding`;
- never redirect from non-admin routes such as auth, invitation, or guest
  surfaces.

Keeping this client-side preserves the existing JSON API behavior and avoids
surprising redirects for fetch callers.

### Import-to-Layouts Flow

When `ImportJobView` observes a succeeded liveaboard.com import, add a secondary
action:

- "Set up cabin layouts" linking to `/admin/onboarding?step=layouts`;
- keep the existing "View trips" link.

The layouts step reads fresh onboarding state when opened, so it catches all
boats without layouts, including imported boats and manually created boats.
The wizard should not wait for import jobs. The import screen already polls
until terminal state; the handoff happens after success.

### Frontend Design

Follow `DESIGN.md`: dense, utilitarian, opaque work surfaces over the existing
admin background. Use small cards only for repeated step/status rows or the
wizard work panel. Do not introduce a landing page, hero, illustration, or
tourism-style onboarding treatment.

Recommended UI structure:

- `web/src/admin/pages/Onboarding.tsx` owns route state and step rendering;
- `web/src/admin/onboarding.ts` or `web/src/admin/onboardingSteps.ts` can hold
  step metadata if it keeps the page readable;
- shared helper components should be extracted only when they prevent copying
  existing form logic, such as the Users invite modal.

## Implementation Plan

### Phase 1: Schema and Store State (~30%)

**Files:**
- `internal/store/migrations/0021_org_onboarding.sql` - add dismissal timestamp
- `internal/store/store.go` - include any new scan field if `Organization` lives there
- `internal/store/organizations.go` - read/update dismissal timestamp
- `internal/store/reports.go` - extend setup/onboarding DTOs and derived state
- `internal/store/reports_test.go` - DB-backed onboarding and layout tests
- `internal/store/cabins.go` - boat layout status helper if not kept in reports.go

**Tasks:**
- [ ] Add `organizations.onboarding_dismissed_at`.
- [ ] Update `Organization` scanning to include the new nullable timestamp.
- [ ] Add `DismissOrganizationOnboarding(ctx, orgID)` store method.
- [ ] Add `BoatsWithoutCabinLayouts` with strict org scoping.
- [ ] Add `OnboardingState(ctx, orgID)` derived from currency, boats, layouts,
      directors, and dismissal timestamp.
- [ ] Extend setup completeness to include layout coverage without removing the
      existing trips setup signal.
- [ ] Test incomplete/new org state, complete org state, dismissal persistence,
      layout detection with inactive cabins/berths, and two-org isolation.

### Phase 2: HTTP API and Authz (~20%)

**Files:**
- `internal/httpapi/onboarding_handlers.go` - new handlers and serializers
- `internal/httpapi/httpapi.go` - mount Org Admin-only routes
- `internal/httpapi/onboarding_handlers_test.go` - API/auth tests
- `internal/httpapi/admin.go` - only if Overview response needs a re-entry flag

**Tasks:**
- [ ] Mount `GET /api/admin/onboarding` behind `RequireOrgAdmin`.
- [ ] Mount `POST /api/admin/onboarding/dismiss` behind `RequireOrgAdmin`.
- [ ] Return setup percent, onboarding steps, dismissal state, and boats without
      layouts.
- [ ] Keep handlers thin; all setup decisions come from store methods.
- [ ] Test Cruise Directors receive 403, unauthenticated callers receive the
      existing session failure, and cross-org data cannot be surfaced.

### Phase 3: Wizard UI and Routing (~30%)

**Files:**
- `web/src/admin/pages/Onboarding.tsx` - new wizard page
- `web/src/admin/api.ts` - onboarding types and calls
- `web/src/admin/Shell.tsx` - admin-only auto-open guard and optional nav/re-entry
- `web/src/admin/pages/Overview.tsx` - prominent onboarding CTA when incomplete
- `web/src/main.tsx` - route registration
- `web/src/styles/app.css` - wizard/stepper styles

**Tasks:**
- [ ] Add typed `adminApi.onboarding()` and `adminApi.dismissOnboarding()`.
- [ ] Register `/admin/onboarding` behind `RequireAdmin`.
- [ ] Build the four-step page with stable route/query step selection.
- [ ] Implement Continue, Skip this step, Skip onboarding, and Back to Overview.
- [ ] Link each step to the existing editor surface and preserve a return path
      through the onboarding route.
- [ ] Add first-run navigation for incomplete, non-dismissed Org Admins.
- [ ] Add Overview re-entry when setup is incomplete, including dismissed orgs.
- [ ] Ensure Cruise Director navigation and Overview remain unchanged.

### Phase 4: Import Handoff and Editor Reuse (~10%)

**Files:**
- `web/src/admin/pages/ImportJob.tsx` - add layout setup handoff on success
- `web/src/admin/pages/Users.tsx` - optional invite modal extraction
- `web/src/admin/components/InviteCruiseDirectorModal.tsx` - optional shared modal
- `web/src/admin/pages/Onboarding.tsx` - consume extracted invite component if used

**Tasks:**
- [ ] Add "Set up cabin layouts" after successful import.
- [ ] If embedding director invite in the wizard, extract the existing invite
      modal without changing invitation API behavior.
- [ ] Avoid copying the cabin layout editor into the wizard; deep-link to the
      boat cabin page.
- [ ] Refresh onboarding state when returning from linked editors.

### Phase 5: Verification and Polish (~10%)

**Files:**
- `internal/store/reports_test.go`
- `internal/httpapi/onboarding_handlers_test.go`
- relevant frontend files from prior phases

**Tasks:**
- [ ] Add tests for onboarding state and dismissal behavior.
- [ ] Add focused handler tests for role gating and response shape.
- [ ] Manually test a fresh signup/verify path with filesystem email transport
      if practical.
- [ ] Manually test dismissed onboarding and Overview re-entry.
- [ ] Verify layout list updates after saving a boat cabin layout.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `npm run build`.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/onboarding` | GET | Fetch current onboarding state for the org |
| `/api/admin/onboarding/dismiss` | POST | Persist "do not auto-show onboarding again" |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0021_org_onboarding.sql` | Create | Add onboarding dismissal timestamp |
| `internal/store/store.go` | Modify | Add nullable organization field if needed |
| `internal/store/organizations.go` | Modify | Read and update onboarding dismissal state |
| `internal/store/reports.go` | Modify | Add onboarding state and layout setup signal |
| `internal/store/cabins.go` | Modify | Add org-scoped boats-without-layout helper if kept near cabin logic |
| `internal/store/reports_test.go` | Modify | Test derived onboarding/setup completeness behavior |
| `internal/httpapi/onboarding_handlers.go` | Create | Onboarding API handlers |
| `internal/httpapi/onboarding_handlers_test.go` | Create | Role and response tests |
| `internal/httpapi/httpapi.go` | Modify | Mount onboarding endpoints |
| `web/src/admin/api.ts` | Modify | Add onboarding DTOs and client calls |
| `web/src/admin/pages/Onboarding.tsx` | Create | Guided onboarding wizard page |
| `web/src/admin/Shell.tsx` | Modify | Admin-only first-run redirect guard |
| `web/src/admin/pages/Overview.tsx` | Modify | Add onboarding re-entry CTA |
| `web/src/admin/pages/ImportJob.tsx` | Modify | Add post-import layout handoff |
| `web/src/main.tsx` | Modify | Register onboarding route |
| `web/src/styles/app.css` | Modify | Wizard stepper and layout styles |

## Definition of Done

- [ ] Org Admins with incomplete, non-dismissed onboarding are guided to
      `/admin/onboarding` on first admin landing after verification/login.
- [ ] Cruise Directors never see or can access onboarding.
- [ ] Wizard steps are fixed in the order currency → boats → layouts →
      directors.
- [ ] Each wizard step has a clear Continue action and a Skip-this-step action.
- [ ] A persistent Skip onboarding action sets `onboarding_dismissed_at`.
- [ ] Dismissed onboarding does not auto-open again.
- [ ] Overview offers onboarding re-entry when setup remains incomplete.
- [ ] Boats without active cabin layouts are listed accurately and scoped to
      the current organization.
- [ ] Successful liveaboard.com import offers a direct handoff to the layouts
      step.
- [ ] `SetupCompleteness` remains the source of truth and includes layout
      coverage.
- [ ] No duplicated currency, cabin layout, import, or invitation business logic.
- [ ] Store and handler tests cover org isolation and role gates.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `npm run build` passes.

## Risks and Mitigations

- **Risk: The wizard becomes a second admin app.** Mitigation: link to existing
  editor pages by default; extract shared components only when reuse is small
  and obvious.
- **Risk: Setup completeness and onboarding drift.** Mitigation: derive wizard
  state from store-level setup queries and test it in `reports_test.go`.
- **Risk: Auto-redirect traps the Admin.** Mitigation: persist whole-wizard
  dismissal, never redirect while already on onboarding, and keep Overview
  accessible.
- **Risk: Existing orgs get interrupted.** Mitigation: gate auto-show on
  incomplete onboarding steps, not just null dismissal timestamp.
- **Risk: Layout detection is expensive or wrong.** Mitigation: one grouped,
  org-scoped query over boats/cabins/berths with active filters and DB-backed
  tests.
- **Risk: Per-step skip expectations are ambiguous.** Mitigation: per-step skip
  is client-only advance; only whole-wizard skip is persisted.

## Security Considerations

- All onboarding endpoints are Org Admin-only.
- Every store query accepts `organization_id` and filters by it in SQL.
- The boats-without-layouts helper must not expose boat names or IDs from other
  organizations.
- Dismissal mutation updates only the current user's organization.
- Frontend route guards are UX only; API role checks remain the security
  boundary.
- The wizard must not expose invitation tokens, import internals, or guest data.

## Dependencies

- Sprint 008 admin chrome and Overview.
- Sprint 010 user invitation flow.
- Sprint 012 liveaboard.com/spreadsheet import.
- Sprint 014/016 cabin layouts and berth assignment model.
- Sprint 020 currency catalog/default behavior.
- Sprint 022 `SetupCompleteness` in `internal/store/reports.go`.
- No new external dependencies.

## Open Questions

1. Should the currency step include a quick inline `CurrencyPicker`, or should
   it only link to `/admin/organization` for maximum reuse?
2. Should the Users invite modal be extracted and embedded in onboarding, or
   should the directors step simply deep-link to `/admin/users`?
3. Should `SetupCompleteness.Percent` include both `layouts` and `trips`, or
   should the wizard have its own four-step percent while Overview keeps the
   broader setup percent?
4. Should onboarding appear in the sidebar permanently for Admins, or only as a
   CTA from Overview/setup cards?
5. Is whole-wizard dismissal enough persistence, or do we eventually need
   per-step dismissal state after seeing how operators use it?
