# Sprint 023: Initial Org Onboarding Wizard

## Overview

Sprint 023 turns the passive setup-completeness card on Overview into
a prescriptive guided wizard that walks a brand-new Org Admin through
the four critical setup steps in a deliberate order:
**currency → boats → layouts → directors**. The wizard is opinionated
about sequence and copy, every step is skippable, and the whole
wizard can be dismissed at any point — matching the seed's
"prescriptive but escapable" framing.

The wizard is a thin coordinator over surfaces that already work.
Currency, fleet/import, cabin layout, and user invitations all have
existing editor pages; this sprint adds one extra setup signal
(boats without usable layouts), a dedicated route, a single
nullable dismissed-or-not timestamp on `organizations`, and step
views that mostly deep-link into those existing editors. Only the
currency step gets a small inline quick-set because it reuses the
existing `CurrencyPicker` and the existing org patch endpoint at no
duplication cost. No new editors, no new business logic.

The deferred "import-then-cabin-layout" mini-wizard from a previous
debugging session is subsumed here. The import job page's success
state links directly into this wizard's layouts step, so one
coordinator covers both first-run onboarding and post-import layout
cleanup.

## Use Cases

1. **First-run setup.** A new Org Admin signs up, verifies their
   email, and the very first visit to the admin chrome one-shot
   redirects them to `/admin/onboarding`. They walk through the
   four steps in order; each step explains why it matters and lets
   them either save (currency) or jump to the canonical editor
   (boats, layouts, directors).
2. **Quick-skip operator.** An admin clicks Start, sets currency,
   then clicks "Skip all" from the wizard header. They land on
   Overview with setup partially complete; the wizard does not
   auto-redirect again. A "Resume setup" link stays on Overview.
3. **Post-import layout cleanup.** An admin imports five boats via
   spreadsheet. The import job page's success state offers
   "Set up cabin layouts" → `/admin/onboarding?step=layouts`. The
   layouts step lists every boat in the org without a usable
   layout. The admin clicks each, sets layouts, returns.
4. **Re-entering by choice.** An admin who skipped earlier sees a
   "Resume setup" link on Overview's setup card while the four
   onboarding steps remain incomplete. Clicking it reopens the
   wizard at the first incomplete step.
5. **Existing orgs unaffected.** Migrations add the new column
   without backfill. Already-set-up orgs (currency, boats, usable
   layouts, ≥1 director) have all four onboarding steps "done"
   from day one, so the auto-show gate (`dismissed_at IS NULL &&
   !onboarding_complete`) doesn't fire for them.

## Architecture

### State Model

One new nullable column on `organizations`. **No backfill.**

```sql
ALTER TABLE organizations
  ADD COLUMN onboarding_dismissed_at timestamptz NULL;
```

Semantics:
- `NULL` — admin has not explicitly dismissed onboarding.
- non-null — do not auto-redirect into the wizard.
- "Skip all" sets the timestamp immediately.
- Completing all four wizard steps does **not** set the timestamp;
  it just makes `onboarding_complete = true`, which also stops the
  auto-redirect.

Why no backfill: the auto-redirect is gated by both
`dismissed_at IS NULL` AND `!onboarding_complete`. Already-set-up
orgs return `onboarding_complete = true` and never see the wizard.
Partially-set-up legacy orgs would benefit from seeing it once —
they can dismiss with one click if they don't want it.

### Setup Signal — One New Step

`SetupCompleteness` in `internal/store/reports.go` gains a fifth
step, ordered after `boats`:

```go
{Key: "layouts", Label: "Lay out cabins for each boat",
 Done: boatsWithoutLayoutCount == 0,
 Hint: pluralizeNoun(boatsWithoutLayoutCount, "boat needs cabins", "boats need cabins"),
 Href: "/admin/onboarding?step=layouts"},
```

The existing `trips` step stays so the Overview/Reports setup
percent remains a faithful operational signal. The wizard does NOT
include trips.

### Layout Completeness Definition

A boat has a usable layout iff it has **at least one active
cabin AND at least one active berth** (an active cabin with no
active berths is unusable — guests have nowhere to sleep). The
store helper:

```go
// BoatsWithoutCabinLayouts returns boats in the org that lack a
// usable cabin layout. A usable layout requires at least one
// active cabin with at least one active berth.
func (p *Pool) BoatsWithoutCabinLayouts(ctx context.Context, orgID uuid.UUID) ([]BoatLayoutStatus, error)

type BoatLayoutStatus struct {
    BoatID      uuid.UUID
    DisplayName string
    SourceName  string
}
```

And the counter:

```go
func (p *Pool) BoatsWithoutCabinLayoutCount(ctx context.Context, orgID uuid.UUID) (int, error)
```

Implementation: a single org-scoped query over `boats` LEFT JOIN
`boat_cabins` (active only) LEFT JOIN `boat_cabin_berths` (active
only), grouped by boat, where `count(berth.id) = 0` survives.

### Onboarding State

New store helper, separate from `SetupCompleteness` so onboarding
completion is decoupled from the trips signal:

```go
type OnboardingState struct {
    DismissedAt          *time.Time
    OnboardingComplete   bool
    Steps                []OnboardingStep        // exactly 4
    SetupPct             int                     // for the Overview banner
    BoatsWithoutLayouts  []BoatLayoutStatus
}

type OnboardingStep struct {
    Key   string  // "currency" | "boats" | "layouts" | "directors"
    Label string
    Done  bool
    Hint  string
}

func (p *Pool) OnboardingState(ctx context.Context, orgID uuid.UUID) (*OnboardingState, error)
```

Where:
- `currency_done = organizations.currency IS NOT NULL`
- `boats_done = BoatCountForOrg > 0`
- `layouts_done = boats_done && BoatsWithoutCabinLayoutCount == 0`
- `directors_done = CountActiveUsersByRole(RoleCruiseDirector) > 0`
- `OnboardingComplete = all four done`

### HTTP API

Two new endpoints, both Org Admin-only:

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/onboarding` | GET | Fetch `OnboardingState`. |
| `/api/admin/onboarding/dismiss` | POST | Set `onboarding_dismissed_at = now()` (idempotent). Records `organization.onboarding_dismissed` audit event. |

Response shape of GET:

```json
{
  "dismissed_at": null,
  "onboarding_complete": false,
  "setup_pct": 40,
  "steps": [
    {"key": "currency",  "label": "Set your country currency",       "done": true,  "hint": "USD"},
    {"key": "boats",     "label": "Add or import a boat",            "done": false, "hint": "0 boats"},
    {"key": "layouts",   "label": "Lay out cabins for each boat",    "done": false, "hint": ""},
    {"key": "directors", "label": "Invite a Cruise Director",        "done": false, "hint": "0 active"}
  ],
  "boats_without_layouts": [
    {"boat_id": "...", "display_name": "Sirius",  "source_name": "liveaboard.com"},
    {"boat_id": "...", "display_name": "Polaris", "source_name": "spreadsheet"}
  ]
}
```

### Route Shape

One frontend route: `/admin/onboarding?step=...`.

- `step` ∈ `{currency, boats, layouts, directors}`. Unknown values
  fall back to the first incomplete step.
- Bare `/admin/onboarding` (no `step` param) also defaults to the
  first incomplete step.
- The wizard component owns the routing — no four nested router
  entries, just one page that switches the body based on `step`.

### Auto-Show (First-Run Trigger)

Implemented entirely client-side in `AdminShell`:

```
After useMe confirms role = org_admin:
  fetch /api/admin/onboarding once on mount
  if dismissed_at == null AND !onboarding_complete
     AND sessionStorage.getItem("onboarding_auto_shown") != "1"
     AND current path is "/admin" (Overview)
     AND not already on /admin/onboarding
  → navigate("/admin/onboarding") and sessionStorage.setItem("onboarding_auto_shown", "1")
```

Browser back works (the navigate is to a real route). The
sessionStorage flag prevents re-redirecting if the admin clicks
back. "Skip all" persists the dismissal so future sessions also
don't auto-redirect.

Cruise Directors never trigger this. The fetch is gated on role.

### Step Views

All in `web/src/admin/pages/Onboarding.tsx` (one file, four small
section components). Shell layout:

- Top: a horizontal stepper showing all four steps with done/
  pending state and the current one highlighted.
- Body: the active step's content.
- Header has: "Skip all" → POST dismiss + navigate to `/admin`.
- Each step footer has: "Skip this step" → advance to next step
  (client-side, no API call); "Continue" → only enabled when the
  step is done (or after a save in the currency step).

**Currency step**: inline `CurrencyPicker` (reuses the Sprint 020
catalog + searchable picker). Best-effort locale guess pre-selects
a currency from `navigator.language` via a small static
country→currency map (top ~20 currencies; defaults to USD
otherwise). Save calls `PATCH /api/admin/organization`.

**Boats step**: three CTAs:
- "Import from liveaboard.com" → `/admin/import/liveaboard?return=onboarding/boats`
- "Import spreadsheet" → `/admin/import/spreadsheet?return=onboarding/boats`
- "Add via Fleet" → `/admin/fleet?return=onboarding/boats`

No manual-add endpoint in this sprint. The wizard reads
`onboarding.steps` to know when the boats step is done.

**Layouts step**: lists `onboarding.boats_without_layouts`. Each
row is a small card with the boat name and a "Set up layout"
button linking to `/admin/fleet/:id/cabins?return=onboarding/layouts`.
The wizard re-fetches `OnboardingState` when the window regains
focus, so returning from the cabin editor refreshes the list.

**Directors step**: a single CTA "Open Users to invite a director"
→ `/admin/users?return=onboarding/directors`. The wizard does NOT
embed the invite form — that would duplicate logic in `Users.tsx`.
The wizard shows the current active-director count below the CTA
so the admin sees progress.

### Post-Import Handoff

`ImportJob.tsx` (the existing import-job status page) adds a
secondary action on success: "Set up cabin layouts" linking to
`/admin/onboarding?step=layouts`. No `boat_filter`, no `job_id`
propagation — the layouts step lists every layout-less boat in the
org, which is the right behavior (other boats may also need
layouts, and the import data doesn't reliably attribute newly-
created boats to a job anyway).

### Overview Integration

`Overview.tsx` setup card replaces its "Open <step>" hyperlinks
with a single primary CTA:

- If `dismissed_at IS NULL && !onboarding_complete`:
  "Start setup" → `/admin/onboarding`.
- If `dismissed_at IS NOT NULL && !onboarding_complete`:
  "Resume setup" → `/admin/onboarding`.
- If `onboarding_complete`: no banner. The existing per-step list
  remains as informational items below the percent.

### What Does NOT Change

- All existing editors (Organization, Fleet, BoatDetail/BoatCabins,
  Users, Import) keep their current standalone behavior.
- `SetupCompleteness` keeps its current shape; the new `layouts`
  step is additive.
- HandleOverview keeps showing the setup card.
- Cruise Director chrome is untouched.
- No changes to auth, billing, or trip flows.

## Implementation Plan

### Phase 1: Schema + Store Helpers (~25%)

**Files:**
- `internal/store/migrations/0021_org_onboarding.sql` — add
  `onboarding_dismissed_at`. Reversible Up/Down. No backfill.
- `internal/store/store.go` or `organizations.go` — scan the new
  nullable timestamp into `Organization`.
- `internal/store/organizations.go` — `DismissOrganizationOnboarding`.
- `internal/store/cabins.go` — `BoatsWithoutCabinLayouts` +
  `BoatsWithoutCabinLayoutCount`.
- `internal/store/reports.go` — extend `SetupCompleteness` with the
  `layouts` step; add `OnboardingState`.
- `internal/store/onboarding_test.go` — DB-backed tests.

**Tasks:**
- [x] Migration adds the column. Down drops it. No `UPDATE` clause.
- [x] `Organization.OnboardingDismissedAt *time.Time` field
      threaded through every SELECT that scans `Organization`.
- [x] `BoatsWithoutCabinLayouts` + count helper using one
      org-scoped query joining boat_cabins/berths on `is_active`.
- [x] `OnboardingState` derives the four-step completion from
      currency, boats, layouts (active cabins + berths), and
      directors. Returns the boats-without-layouts list inline.
- [x] `SetupCompleteness` adds `layouts` step ordered after
      `boats`; `trips` unchanged.
- [x] Tests cover: org with no currency, org with boats but zero
      layouts, org with active cabin but no berths (still
      incomplete), fully set up org, two-org isolation, dismissal
      persistence.

### Phase 2: HTTP API + Authz Tests (~15%)

**Files:**
- `internal/httpapi/onboarding_handlers.go`.
- `internal/httpapi/onboarding_handlers_test.go`.
- `internal/httpapi/httpapi.go` — mount routes inside
  `RequireOrgAdmin` group.

**Tasks:**
- [x] `handleGetOnboarding` returns the unified `OnboardingState`
      payload (see Response shape above).
- [x] `handleDismissOnboarding` updates the timestamp idempotently
      and records `organization.onboarding_dismissed` audit event
      via the existing `recordStaffAudit` helper.
- [x] Tests:
  - Cruise Director → 403 on both endpoints.
  - Unauthenticated → 401.
  - Cross-tenant: org A cannot dismiss or read org B's state.
  - Dismiss is idempotent (second POST returns same state, no
    duplicate audit event needed — record once on first dismiss).

### Phase 3: Wizard Shell + Stepper + Routing (~20%)

**Files:**
- `web/src/admin/pages/Onboarding.tsx` — one page with header,
  stepper, body, footer.
- `web/src/admin/api.ts` — typed `onboarding()` and
  `dismissOnboarding()`.
- `web/src/main.tsx` — register `/admin/onboarding` behind
  `RequireAdmin`.
- `web/src/styles/app.css` — stepper + step-shell styles.

**Tasks:**
- [x] Fetch `OnboardingState` on mount. Compute current step from
      `?step=` (validated against the four keys) or fall back to
      first incomplete.
- [x] Render stepper with done/active/pending visual states.
- [x] Wizard header: "Skip all" button → POST dismiss → navigate
      to `/admin`.
- [x] Step footer: "Skip this step" advances; "Continue" is
      disabled until the step is done (or successfully saved for
      currency).
- [x] Re-fetch state on window focus.
- [x] Route gated by `RequireAdmin`.

### Phase 4: Step Views (~20%)

**Files:** (all sections inside `Onboarding.tsx`)

**Tasks:**
- [x] Currency: inline `CurrencyPicker` + locale guess + Save
      (PATCH /api/admin/organization). Reuses the existing org
      patch shape. After successful save, advance to boats.
- [x] Boats: three CTAs (import liveaboard, import spreadsheet,
      add via Fleet) all with `?return=onboarding/boats` query
      param so the destination knows to come back.
- [x] Layouts: list each `boats_without_layouts` row with name +
      source_name + "Set up layout" button linking to
      `/admin/fleet/:id/cabins?return=onboarding/layouts`.
- [x] Directors: single CTA to `/admin/users?return=onboarding/directors`
      with the active-director count shown for context.

### Phase 5: Auto-Show + Overview CTA + Import Handoff (~15%)

**Files:**
- `web/src/admin/Shell.tsx` (or a small `OnboardingGate.tsx`) —
  client-side first-run auto-show.
- `web/src/admin/pages/Overview.tsx` — "Start/Resume setup" CTA.
- `web/src/admin/pages/ImportJob.tsx` — "Set up cabin layouts"
  link on success.

**Tasks:**
- [x] On Org Admin mount, fetch onboarding once. If
      `dismissed_at IS NULL && !onboarding_complete` and the
      session-storage flag isn't set and the current path is
      `/admin`, navigate to `/admin/onboarding` and set the flag.
- [x] Never auto-redirect for Cruise Directors, never from
      `/admin/onboarding` itself, never from non-`/admin` routes.
- [x] Overview's setup card: primary CTA is "Start setup" /
      "Resume setup" / hidden, based on the onboarding state.
- [x] ImportJob success state gains "Set up cabin layouts" →
      `/admin/onboarding?step=layouts`.

### Phase 6: Docs + Verification (~5%)

**Files:**
- `docs/product/personas.md` — note onboarding ownership.
- `docs/product/organization-admin-user-stories.md` — link
  US-7.1/7.2 to the wizard.
- `local_setup.md` — mention the wizard.

**Tasks:**
- [x] Update product docs.
- [x] `go test ./...`, `go vet ./...`, `npm run build` all pass.

## API Endpoints

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/onboarding` | GET | Org Admin | Wizard state. |
| `/api/admin/onboarding/dismiss` | POST | Org Admin | Mark dismissed. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0021_org_onboarding.sql` | Create | Add `onboarding_dismissed_at`. |
| `internal/store/organizations.go` | Modify | Scan + `DismissOrganizationOnboarding`. |
| `internal/store/cabins.go` | Modify | `BoatsWithoutCabinLayouts(+Count)`. |
| `internal/store/reports.go` | Modify | `layouts` step; `OnboardingState`. |
| `internal/store/onboarding_test.go` | Create | DB-backed tests. |
| `internal/httpapi/onboarding_handlers.go` | Create | GET + POST. |
| `internal/httpapi/onboarding_handlers_test.go` | Create | Authz + isolation. |
| `internal/httpapi/httpapi.go` | Modify | Mount routes. |
| `web/src/admin/pages/Onboarding.tsx` | Create | Wizard page (shell + four sections). |
| `web/src/admin/pages/Overview.tsx` | Modify | Start/Resume CTA. |
| `web/src/admin/pages/ImportJob.tsx` | Modify | Post-import handoff. |
| `web/src/admin/Shell.tsx` | Modify | First-run auto-show. |
| `web/src/admin/api.ts` | Modify | Typed fetchers. |
| `web/src/main.tsx` | Modify | Route + RequireAdmin. |
| `web/src/styles/app.css` | Modify | Stepper + shell styles. |
| `docs/product/personas.md` | Modify | Onboarding ownership note. |
| `docs/product/organization-admin-user-stories.md` | Modify | Link US-7.x to wizard. |
| `local_setup.md` | Modify | Mention wizard. |

## Definition of Done

- [x] Migration applies cleanly on a fresh DB; Down works.
- [x] No data backfill; existing orgs see the wizard only if their
      four onboarding steps aren't all done.
- [x] `BoatsWithoutCabinLayouts` treats a boat with 0 active berths
      as missing a layout, even if it has active cabins; tests
      cover this and two-org isolation.
- [x] `SetupCompleteness` returns five steps (currency, boats,
      layouts, directors, trips). Existing Overview and Reports
      consumers render correctly with the new step.
- [x] `OnboardingState.onboarding_complete` is computed from the
      four wizard steps only; trips does not affect it.
- [x] `GET /api/admin/onboarding` returns the unified payload and
      refuses non-admin callers with 403.
- [x] `POST /api/admin/onboarding/dismiss` is idempotent and
      writes one audit event on first dismiss.
- [x] Cross-tenant test confirms org A cannot read or dismiss org
      B's state.
- [x] Wizard auto-redirects only when
      `dismissed_at IS NULL && !onboarding_complete` AND
      sessionStorage flag is unset AND path is `/admin`.
- [x] "Skip all" sets the timestamp and navigates to `/admin`.
- [x] "Skip this step" advances without an API call.
- [x] Currency step saves through the existing org patch endpoint;
      best-effort locale guess works for at least USD/EUR/GBP
      without ever blocking the picker.
- [x] Layouts step refreshes on window focus.
- [x] Overview's setup card shows the correct "Start" / "Resume"
      CTA according to onboarding state.
- [x] ImportJob success page links to
      `/admin/onboarding?step=layouts`.
- [x] `go test ./...`, `go vet ./...`, `npm run build` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Auto-redirect traps the admin in a loop. | Low | High | One-shot per session via sessionStorage; "Skip all" always visible; bare `/admin/onboarding` redirects, but the wizard route never redirects back. |
| Layout detection wrong (e.g., counts inactive berths). | Medium | High | Single org-scoped query with `is_active = true` on both cabins and berths. Tests cover inactive cabin, active cabin / zero berths, two-org isolation. |
| `setup.pct` from `SetupCompleteness` includes trips and would loop the wizard forever. | Was: High | High | `OnboardingComplete` is derived from the four wizard steps only; the auto-redirect gate uses it, not `setup_pct`. |
| Currency picker locale guess misfires. | Medium | Low | Best-effort static map of common countries; falls back to USD; admin can always pick freely from the searchable list. |
| Post-import handoff links to a stale layouts step. | Low | Low | Wizard re-fetches state on window focus; layouts step always shows the current set. |
| Manual-add boat is missing and admins ask for it. | Medium | Low | Out of scope by design; "Add via Fleet" deep link covers this. If demand appears, build a dedicated fleet sprint. |
| Existing partially-set-up orgs get surprised by the wizard. | Medium | Low | "Skip all" is one click; no destructive effects from auto-show. |

## Security Considerations

- All onboarding endpoints sit inside the existing Org Admin route
  group; Cruise Directors return 403.
- Every store query takes `organization_id` and filters by it in
  SQL; the `boats_without_layouts` payload contains only boat id,
  display name, and source name (no operational data).
- Dismiss writes one audit event scoped to the org; no PII.
- Frontend route guards are UX only; the API role check remains
  the security boundary.
- The wizard never exposes invitation tokens, import internals,
  guest data, or session cookies.

## Dependencies

- Sprint 008 — admin chrome + Overview.
- Sprint 010 — user invitation flow (linked from the directors
  step).
- Sprint 012 — liveaboard.com + spreadsheet import (linked from
  the boats step and the post-import handoff).
- Sprint 014/016 — cabin layouts and berth model (used by the
  layouts step + the new "usable layout" definition).
- Sprint 020 — currency picker + defaults (the currency step
  reuses `CurrencyPicker` from this work).
- Sprint 022 — `SetupCompleteness` in `internal/store/reports.go`
  (extended here).
- No new external dependencies.

## References

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md` — US-7.1
  (setup completeness), US-7.2 (operational status).
- `docs/sprints/SPRINT-014.md` — cabin layouts.
- `docs/sprints/SPRINT-020.md` — currency catalog.
- `docs/sprints/SPRINT-022.md` — `SetupCompleteness` source of
  truth.
- `docs/sprints/drafts/SPRINT-023-INTENT.md`
- `docs/sprints/drafts/SPRINT-023-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-023-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-023-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-023-MERGE-NOTES.md`
