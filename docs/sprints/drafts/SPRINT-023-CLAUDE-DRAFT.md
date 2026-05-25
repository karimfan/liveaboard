# Sprint 023: Initial Org Onboarding Wizard

## Overview

Sprint 023 turns the passive setup-completeness card on the Overview
page into a prescriptive guided wizard that walks a brand-new Org
Admin through the four critical setup steps in a deliberate order:
**currency → boats → layouts → directors**. The wizard is opinionated
about sequence and copy, but every step is skippable, and the whole
wizard can be dismissed at any point — closing the loop on the
seed's "be prescriptive to facilitate good outcomes" without locking
operators out of the rest of the app.

The wizard is a *presentation layer over existing surfaces*. There
is no new business logic: each step deep-links into the editor that
already exists for that concern (currency picker, fleet/import, cabin
layout, invite director). The wizard's value is the ordering, the
"why this matters" copy, the sane defaults (browser-locale currency
guess), and a single dismissed-or-not state stored on the
organization. Setup-completeness in `internal/store/reports.go` is
extended to surface "boats without a cabin layout" as a fifth step,
making it the unified source of truth for what the wizard considers
done.

The deferred "import-then-cabin-layout" mini-wizard from the
previous debugging session is subsumed here: the post-import layout
cleanup IS the wizard's layouts step, just entered with a query
param that pins the focus to one trip's newly-imported boats.

## Use Cases

1. **First-run setup.** A new Org Admin signs up, verifies their
   email, lands on Overview, sees a prominent "Get your org set up
   in four steps" banner, clicks Start. The wizard auto-redirects
   to `/admin/onboarding` and walks them through currency, boats,
   layouts, directors. They land back on Overview with a
   meaningful setup state.
2. **Quick-skip operator.** An Org Admin clicks Start, sees the
   currency step, picks USD, clicks "Skip the rest" from the
   wizard header, and is back on Overview with setup partially
   complete and onboarding dismissed.
3. **Post-import layout cleanup.** An admin imports 5 boats from
   liveaboard.com. After the job finishes, the import-job page
   links into `/admin/onboarding?focus=layouts&boat_filter=new`.
   The wizard skips straight to the layouts step with only the 5
   newly-imported boats listed. The admin clicks each, sets up
   layouts, and exits.
4. **Re-entering by choice.** An admin who previously skipped sees
   a "Resume setup" link on the Overview's setup card. Clicking it
   reopens the wizard from the first incomplete step.
5. **Existing orgs unaffected.** An org that was already set up
   before this sprint shipped sees the new wizard exactly once
   (the migration backfills `onboarding_dismissed_at` for any org
   that already has a currency + boats + directors).

## Architecture

### State Model

One new nullable column on `organizations`:

```sql
ALTER TABLE organizations
  ADD COLUMN onboarding_dismissed_at timestamptz NULL;
```

Backfill: any org with `currency IS NOT NULL` AND `(SELECT count(*)
FROM boats WHERE organization_id = id) > 0` AND `(SELECT count(*)
FROM users WHERE organization_id = id AND role =
'cruise_director' AND is_active) > 0` gets
`onboarding_dismissed_at = now()` so existing orgs don't see the
wizard.

That's the only schema change. Everything else is presentation.

### Setup Completeness — One New Step

`SetupCompleteness` in `internal/store/reports.go` grows a fifth
step:

```go
{Key: "layouts", Label: "Lay out cabins for each boat",
 Done: boatsWithoutLayoutCount == 0, ...},
```

It uses a new helper:

```go
func (p *Pool) BoatsWithoutLayoutCount(ctx context.Context, orgID uuid.UUID) (int, error)
```

This is a single COUNT query over `boats` LEFT JOINing
`boat_cabins` and filtering `is_active`.

A sibling helper enumerates the boats for the wizard's layouts
step:

```go
func (p *Pool) BoatsWithoutLayout(ctx context.Context, orgID uuid.UUID) ([]BoatNeedsLayout, error)

type BoatNeedsLayout struct {
    ID          uuid.UUID
    DisplayName string
    SourceName  string
}
```

### Wizard Endpoint

One new HTTP endpoint, admin-only:

```
GET  /api/admin/onboarding
POST /api/admin/onboarding/dismiss
```

Response of GET:
```json
{
  "dismissed_at": null,
  "current_step": "currency",
  "setup": {
    "pct": 25,
    "steps": [
      {"key": "currency", "label": "...", "done": true, ...},
      {"key": "boats",   "label": "...", "done": false, ...},
      {"key": "layouts", "label": "...", "done": false, ...},
      {"key": "directors","label": "...", "done": false, ...}
    ]
  },
  "boats_without_layout": [{"id": "...", "display_name": "Sirius"}, ...]
}
```

`current_step` is the first step where `done == false`. If the
admin uses `?focus=layouts`, the response forces
`current_step = "layouts"`.

POST `/dismiss` sets `onboarding_dismissed_at` to now and returns
the updated payload. Idempotent.

### Route Choice

Dedicated route at `/admin/onboarding` (not a modal). Reasons:
- Deep-linkable from email, from the post-import job page, from
  the Overview's "Resume setup" link.
- Survives browser reload.
- Easier to fit alongside the existing admin chrome.

The wizard page renders inside the admin shell (sidebar visible)
so the admin doesn't feel trapped. A "Skip all" button in the
wizard's own header exits to Overview and sets `dismissed_at`.

### First-Run Behavior

After login, the `MeProvider` (or a new lightweight provider)
hits `/api/admin/onboarding` once on mount for Org Admins. If
`dismissed_at IS NULL AND setup.pct < 100`, it inserts a one-time
auto-redirect to `/admin/onboarding` on the next route change.
This is gentle: no infinite redirect loops, and the admin can use
their browser back to escape if they want.

A simpler alternative: skip the auto-redirect entirely and just
make the Overview's setup banner more prominent with a "Start
setup" CTA. Pick auto-redirect for first-run only to satisfy the
"prescriptive" requirement.

### Step UX

Each step is its own route under `/admin/onboarding/<step-key>`:

```
/admin/onboarding              → redirects to current_step
/admin/onboarding/currency
/admin/onboarding/boats
/admin/onboarding/layouts
/admin/onboarding/directors
```

Each step has the same layout shell:
- Stepper at top showing all four steps + which is current + which
  are done.
- Big title + "why this matters" copy (2-3 sentences).
- The editor itself (see below).
- Footer with `Skip this step` (no-op except advancing) and
  `Continue` (advance after a successful save) buttons.

### Step Implementations

**Currency** — embeds the existing `CurrencyPicker` (Sprint 023
catalog) plus a pre-suggestion derived from `navigator.language`
via `Intl` lookup tables. The admin sees, e.g., "USD looks right
based on your locale" with a one-click confirm.

**Boats** — two CTAs:
- "Import from liveaboard.com" → links to `/admin/import/liveaboard`
  with `?return=onboarding/boats` so the existing import flow
  knows to return here.
- "Add manually" → opens a small inline form (display name + a
  link to the boat-detail page for further edits). Avoids
  re-implementing the full fleet page.

After at least one boat exists, "Continue" becomes enabled.

**Layouts** — lists every boat in
`onboarding.boats_without_layout`. Each row has the boat name and
a "Set up layout" button that links to
`/admin/fleet/:id/cabins?return=onboarding/layouts`. Returning to
the wizard re-fetches the list. "Continue" is enabled once the
list is empty OR the admin explicitly clicks "Skip remaining".

This is the same UX needed post-import.

**Directors** — embeds the existing invite form
(`POST /api/invitations` with role=cruise_director). Shows the
list of pending invites + active directors below. "Continue"
becomes enabled after the first invite is sent OR the admin
clicks "Skip for now".

### Boat-import return handling

For the existing `/admin/import/liveaboard` flow:
- If the URL has `?return=onboarding/boats`, after the job is
  kicked the page redirects to `/admin/onboarding/boats?job=<id>`.
- The wizard's boats step polls the job status (reuses
  `adminApi.getImportJob`) and shows progress inline.
- When the job succeeds, the wizard advances the admin to the
  layouts step automatically (with `?boat_filter=new&job=<id>` so
  the layouts step can optionally pin the focus to just those
  boats).

### What Does NOT Change

- All existing editor pages (Organization, Fleet, BoatCabins,
  Users, Import) stay exactly where they are and keep their
  current standalone usefulness.
- `SetupCompleteness` keeps backwards-compatible shape; the new
  step is additive.
- HandleOverview keeps showing the setup card (now with 5 steps).
- No changes to auth, billing, or trip flows.

## Implementation Plan

### Phase 1: Schema + Setup Completeness Extension (~20%)

**Files:**
- `internal/store/migrations/0021_onboarding_dismissed_at.sql` — new
  migration. Adds the column + backfill for already-set-up orgs.
- `internal/store/organizations.go` — extend `Organization` struct
  + scan + update helper for `DismissOnboarding`.
- `internal/store/cabins.go` — `BoatsWithoutLayoutCount`,
  `BoatsWithoutLayout`.
- `internal/store/reports.go` — extend `SetupCompleteness` with the
  new layouts step.
- `internal/store/onboarding_test.go` — store-level tests.

**Tasks:**
- [ ] Migration with reversible Up/Down and a guarded backfill.
- [ ] `Organization.OnboardingDismissedAt *time.Time` field +
      column added to selects.
- [ ] `BoatsWithoutLayoutCount` and `BoatsWithoutLayout` helpers,
      scoped by org and ignoring boats with at least one active
      cabin.
- [ ] `SetupCompleteness` includes a `layouts` step ordered after
      `boats`.
- [ ] Tests: completeness order, layouts step done iff every boat
      has ≥1 active cabin, backfill picks up only fully-set-up
      orgs.

### Phase 2: Onboarding Endpoint + Dismiss (~15%)

**Files:**
- `internal/httpapi/onboarding_handlers.go` — GET + POST handlers.
- `internal/httpapi/httpapi.go` — admin-only route mounts.
- `internal/httpapi/onboarding_handlers_test.go` — authz tests
  (Org Admin only; cross-tenant isolation).

**Tasks:**
- [ ] Define a small `OnboardingState` DTO assembled from
      `SetupCompleteness`, the boats-without-layout list, the
      dismissed timestamp, and the computed `current_step`.
- [ ] GET handler. Honors optional `?focus=` to pin current_step.
- [ ] POST `/dismiss` handler. Idempotent. Records a staff audit
      event (`organization.onboarding_dismissed`).
- [ ] Tests: directors are rejected with 403; cross-org callers
      can't read another org's state; dismiss is idempotent.

### Phase 3: Wizard Shell + Stepper (~15%)

**Files:**
- `web/src/admin/pages/Onboarding.tsx` — wizard shell with router
  for the four steps.
- `web/src/admin/api.ts` — `onboarding()` and `dismissOnboarding()`.
- `web/src/main.tsx` — register
  `/admin/onboarding` (and `/admin/onboarding/:step`).
- `web/src/styles/app.css` — stepper + step-shell styles.

**Tasks:**
- [ ] `Onboarding.tsx` fetches state, computes current step, and
      renders the stepper + the active step component.
- [ ] Header has "Skip all" button → POST dismiss + navigate to
      Overview.
- [ ] Bare `/admin/onboarding` redirects to `current_step`.
- [ ] Route gated by `RequireAdmin`.

### Phase 4: Step Components (~25%)

**Files:**
- `web/src/admin/pages/OnboardingCurrency.tsx`
- `web/src/admin/pages/OnboardingBoats.tsx`
- `web/src/admin/pages/OnboardingLayouts.tsx`
- `web/src/admin/pages/OnboardingDirectors.tsx`

**Tasks:**
- [ ] Currency step: embeds `CurrencyPicker`, suggests a default
      from `navigator.language` via a small static
      country→currency map. Save calls `updateOrganization`. On
      success, advance.
- [ ] Boats step: two CTAs (import vs add manually). Manual form
      uses an existing or new minimal "create boat" endpoint
      (NB: today boats are only created via import — we may need
      a small POST /admin/boats handler; flag in open questions).
- [ ] Layouts step: lists `boats_without_layout`, deep links to
      `/admin/fleet/:id/cabins?return=onboarding/layouts`,
      refreshes on window focus.
- [ ] Directors step: invite form (reuses existing invite POST),
      lists pending + active directors below.
- [ ] Per-step "Skip" advances without saving; "Continue" advances
      after a save.

### Phase 5: First-Run Trigger + Re-entry (~10%)

**Files:**
- `web/src/admin/useMe.tsx` or a new `OnboardingGate.tsx` — gentle
  one-shot redirect on first visit.
- `web/src/admin/pages/Overview.tsx` — replace/augment the setup
  card with a "Resume setup" CTA when
  `dismissed_at && pct < 100`.
- `web/src/admin/Shell.tsx` — optional "Setup" nav item visible
  to admins when setup is incomplete.

**Tasks:**
- [ ] One-time auto-redirect: on first Overview visit per session,
      if `dismissed_at IS NULL AND pct < 100`, navigate to
      `/admin/onboarding`. Use a session-storage flag so the
      browser back button works.
- [ ] Overview's setup card gains a primary CTA: "Resume setup" if
      dismissed but incomplete, "Start setup" if not yet
      dismissed.
- [ ] Update the import job page to deep-link into
      `/admin/onboarding/layouts?boat_filter=new&job=<id>` on
      success (only when accessed via the wizard's return URL).

### Phase 6: Docs + Verification (~15%)

**Files:**
- `docs/product/personas.md` — note that Org Admin owns onboarding.
- `docs/product/organization-admin-user-stories.md` — link the
  setup-completeness stories (US-7.1 etc.) to the wizard.
- `local_setup.md` — note the wizard.

**Tasks:**
- [ ] Update docs.
- [ ] `go test ./...`, `go vet ./...`, `npm run build` pass.

## API Endpoints

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/onboarding` | GET | Org Admin | Wizard state. |
| `/api/admin/onboarding/dismiss` | POST | Org Admin | Mark dismissed. |
| `/api/admin/boats` | POST | Org Admin | (Optional) Manual boat create. See open question. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0021_onboarding_dismissed_at.sql` | Create | Column + backfill. |
| `internal/store/organizations.go` | Modify | OnboardingDismissedAt + DismissOnboarding. |
| `internal/store/cabins.go` | Modify | BoatsWithoutLayout helpers. |
| `internal/store/reports.go` | Modify | Layouts step in SetupCompleteness. |
| `internal/store/onboarding_test.go` | Create | Store tests. |
| `internal/httpapi/onboarding_handlers.go` | Create | GET + POST. |
| `internal/httpapi/httpapi.go` | Modify | Routes. |
| `internal/httpapi/onboarding_handlers_test.go` | Create | Authz tests. |
| `web/src/admin/pages/Onboarding.tsx` | Create | Shell + stepper. |
| `web/src/admin/pages/OnboardingCurrency.tsx` | Create | Step 1. |
| `web/src/admin/pages/OnboardingBoats.tsx` | Create | Step 2. |
| `web/src/admin/pages/OnboardingLayouts.tsx` | Create | Step 3. |
| `web/src/admin/pages/OnboardingDirectors.tsx` | Create | Step 4. |
| `web/src/admin/pages/Overview.tsx` | Modify | "Start/Resume setup" CTA. |
| `web/src/admin/Shell.tsx` | Modify | Optional Setup nav entry. |
| `web/src/admin/api.ts` | Modify | onboarding + dismiss fetchers. |
| `web/src/main.tsx` | Modify | Routes. |
| `web/src/styles/app.css` | Modify | Stepper + step-shell styles. |
| `docs/product/personas.md` | Modify | Onboarding ownership. |
| `docs/product/organization-admin-user-stories.md` | Modify | Link stories. |
| `local_setup.md` | Modify | Note wizard. |

## Definition of Done

- [ ] Migration applies cleanly to a fresh DB and the existing
      dev DB; reversible Down works.
- [ ] Backfill marks already-set-up orgs as dismissed; partially-
      set-up orgs remain non-dismissed.
- [ ] `SetupCompleteness` returns five steps in the documented
      order; layouts step is done iff every boat in the org has at
      least one active cabin.
- [ ] `GET /api/admin/onboarding` returns the unified state
      and refuses non-admin callers.
- [ ] `POST /api/admin/onboarding/dismiss` is idempotent and writes
      an audit event.
- [ ] Cross-tenant test confirms one org can never read or dismiss
      another's state.
- [ ] Wizard auto-redirects new admins on their first authenticated
      visit when `dismissed_at IS NULL AND pct < 100`. No infinite
      loops.
- [ ] Per-step "Skip" advances without mutating org state;
      per-wizard "Skip all" sets `dismissed_at`.
- [ ] Currency step's locale guess works for at least USD, EUR,
      GBP, AUD, JPY; falls back to USD gracefully.
- [ ] Boats step funnels into the existing import + a small
      manual-add form.
- [ ] Layouts step refreshes when the admin returns from the
      cabin editor (window focus or a back-button refetch).
- [ ] Directors step uses the existing invite endpoint and shows
      pending + active directors below.
- [ ] Overview's setup card now reads "Resume setup" / "Start
      setup" appropriately.
- [ ] `go test ./...`, `go vet ./...`, `npm run build` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Auto-redirect loops trap the admin. | Low | High | One-shot per session via sessionStorage; explicit "Skip all" always present in the wizard header. |
| Backfill misclassifies an org. | Medium | Medium | Migration condition is conservative (currency AND boats AND ≥1 active director); falsely-dismissed orgs can re-open from Overview. |
| Manual-add boat path requires a new mutation we don't have. | High | Medium | If we don't want a new endpoint, the manual path becomes a deep-link to the existing fleet page with `?return=onboarding/boats`. |
| Post-import return loop hides the import progress. | Medium | Low | The wizard's boats step polls the job and displays inline progress until success/failure. |
| Existing orgs see the wizard once because backfill missed them. | Low | Low | Wizard shell explicitly shows a "Skip all" button at the top — escape is one click. |
| Two surfaces (Overview card + wizard) drift. | Medium | Low | Both render from the same `SetupCompleteness` response. |

## Security Considerations

- All onboarding endpoints sit inside the existing Org Admin route
  group (`RequireOrgAdmin`).
- The `boats_without_layout` payload returns only `id`,
  `display_name`, `source_name` — no extra PII or operational
  data.
- Dismiss writes a single audit event with no sensitive metadata
  beyond the actor + org id.
- The first-run auto-redirect happens client-side and uses
  sessionStorage; no server-side cookies or counters.

## Dependencies

- Sprint 014 — cabin layout editor (target of the layouts step).
- Sprint 020 — currency picker + defaults.
- Sprint 022 — `SetupCompleteness` source of truth.
- Sprint 023 (this session's earlier work) — full ISO 4217
  catalog + searchable picker (the currency step's UI).

## Open Questions

- **Manual-add boat endpoint?** Today boats are only created via
  import. A wizard manual-add either needs a new
  `POST /admin/boats` or just deep-links to the existing fleet
  page. Recommend the latter for this sprint to avoid scope
  creep; revisit if operators report friction.
- **Locale guess fidelity**: map every ISO country → currency or
  just the top ~30? Top 30 covers >99% of likely users; default
  USD for the rest.
- **Auto-redirect cadence**: one-shot per session is the
  recommendation; alternatives are "every Overview visit until
  dismissed" or "never auto-redirect". Picking one in the
  interview.
- **Trips step omission**: confirm "trips" stays out of the
  wizard per the seed (trip creation is operational, not setup).
- **Director invite required?**: confirm whether "Skip" on the
  directors step counts as setup-complete or leaves a yellow
  "Resume setup" hint on Overview.
