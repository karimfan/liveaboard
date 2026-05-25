# Sprint 023 Merge Notes

## Claude Draft Strengths

- Dedicated `/admin/onboarding` route over a modal — supports deep
  links, refresh, browser back, and post-import handoff.
- Single source-of-truth state model: one nullable timestamp on
  `organizations`, no parallel setup subsystem.
- Wizard derives setup state from `SetupCompleteness` instead of
  inventing a new computation.
- Explicit subsumption of yesterday's deferred import-then-layout
  mini-wizard.
- Risk table called out the auto-redirect trap and the
  backfill-mismatch risk up front.

## Codex Draft Strengths

- **Honest about scope creep.** Codex spotted that my "presentation
  layer" overview didn't match my later phases (inline manual-add
  boat form, currency embed, possible new endpoint). Same surface
  area, leaner footprint.
- **Auto-show gate must use onboarding-specific completeness, not
  raw setup percent.** Without this, a fully-onboarded org with no
  trips yet would loop into the wizard forever.
- **Layout completeness needs active berths, not just active
  cabins.** A boat with one cabin and zero berths isn't usable; my
  definition would mark it "done".
- **Drop the backfill.** It misses the new layouts signal. Since
  auto-show is gated by derived state, no backfill is needed at
  all.
- **Drop manual boat creation.** No `POST /admin/boats` exists
  today, and adding it drags in slug/source semantics, fleet UI,
  tests. Out of scope for a wizard sprint.
- **Drop `boat_filter=new`.** Import job records don't expose
  newly-created boat IDs. The layouts step lists all org boats
  without layouts; that's sufficient.
- **`?step=` query param > nested routes.** Simpler routing
  surface; same UX.

## Valid Critiques Accepted

1. **Default every step to deep-link, not embed.** Only the currency
   step keeps a small inline quick-set (reuses `CurrencyPicker` and
   the existing `PATCH /api/admin/organization`).
2. **No manual-add boat in this sprint.** Boats step offers:
   "Import from liveaboard.com", "Import spreadsheet", or
   "Add via Fleet" (deep link to `/admin/fleet?return=onboarding`).
   No new endpoint.
3. **Auto-show condition uses `!onboarding_complete`, not
   `setup.pct < 100`.** Onboarding completeness is derived from the
   four wizard steps only (currency, boats, layouts, directors).
   Trips stays in Overview/Reports setup signal but does not drive
   the wizard.
4. **No migration backfill.** Single column add; gate on derived
   state.
5. **Drop `boat_filter=new` and any job-specific filtering.**
   Post-import deep-links to `/admin/onboarding?step=layouts`
   without further filtering. Layouts step lists every layout-less
   boat in the org.
6. **Use `?step=` query param**, one route, one page component.
7. **Layout completeness = ≥1 active cabin AND ≥1 active berth per
   boat.** Test all the edges (inactive cabin, active cabin/zero
   berths, inactive berths, two-org isolation).

## Critiques Rejected (with reasoning)

1. **Codex flagged `local_setup.md` as missing.** It exists at the
   repo root (created in an earlier session) and is the standard
   place for dev-onboarding notes. Keep the doc update in the plan.
2. **Codex questioned the audit event for dismissal.** I'll keep
   it. Dismissal is a meaningful product action and the existing
   `recordStaffAudit` helper makes it nearly free.

## Interview Refinements Applied

| Interview answer | Final-doc impact |
|---|---|
| One-shot auto-redirect on first authenticated visit per session | Implement via `sessionStorage` flag in `AdminShell`. Gate on `dismissed_at IS NULL && !onboarding_complete`. |
| Manual-add boat → deep-link to Fleet | No `POST /admin/boats`. Boats step has three CTAs: Import liveaboard, Import spreadsheet, Add via Fleet (deep link with return). |
| One wizard subsumes import-then-layout | ImportJob success page links to `/admin/onboarding?step=layouts`. No second wizard ships. |
| Done = all four steps OR explicit dismiss | Compute `onboarding_complete = currency_done && boats_done && layouts_done && directors_done`. Auto-show ceases when complete OR dismissed; Overview's "Resume setup" CTA stays visible while either condition holds and the other doesn't. |

## Final Decisions

- **Schema**: one column, no backfill.
  `ALTER TABLE organizations ADD COLUMN onboarding_dismissed_at timestamptz NULL;`
- **Setup signal extension**: `SetupCompleteness` grows a `layouts`
  step (≥1 active cabin AND ≥1 active berth per boat). `trips`
  stays.
- **Onboarding completion**: derived from currency + boats +
  layouts + directors only. Separate from `SetupCompleteness.Percent`.
- **Two endpoints**:
  - `GET /api/admin/onboarding` — returns
    `{dismissed_at, onboarding_complete, setup_pct, steps[],
     boats_without_layouts[]}`. Honors `?step=` only for client
    convenience; client computes the right one if omitted.
  - `POST /api/admin/onboarding/dismiss` — sets the timestamp,
    idempotent, writes audit event.
- **Route**: `/admin/onboarding?step=currency|boats|layouts|directors`.
  One page component, four step views inside.
- **Auto-show**: client-side in `AdminShell` after `useMe` confirms
  Org Admin. One-shot per session via sessionStorage. Never
  redirect Cruise Directors, never redirect when already on
  `/admin/onboarding`, never redirect from non-admin routes.
- **Step UX**:
  - Currency: small inline `CurrencyPicker` + Save (reuses existing
    org patch endpoint).
  - Boats: three CTAs (import liveaboard, import spreadsheet, add
    via Fleet). All deep-link with `?return=onboarding/boats`.
  - Layouts: list `boats_without_layouts` with deep-link to
    `/admin/fleet/:id/cabins?return=onboarding/layouts`. Refresh
    on window focus.
  - Directors: deep-link to `/admin/users?return=onboarding/directors`.
    The wizard does NOT embed the invite form.
- **Locale currency guess**: best-effort, not in DoD. Top ~20
  countries → currency map; default USD otherwise.
- **Post-import handoff**: `ImportJob.tsx` adds a "Set up cabin
  layouts" link to `/admin/onboarding?step=layouts` on success.
  No filtering, no job-id propagation.
- **Audit event**: `organization.onboarding_dismissed` on dismiss,
  recorded via the existing `recordStaffAudit` helper.

## Phasing for the Final Doc

1. Phase 1 — Schema + store helpers (column, BoatsWithoutCabinLayouts, OnboardingState) — ~25%
2. Phase 2 — HTTP endpoints + tests — ~15%
3. Phase 3 — Wizard shell + stepper + step routing — ~20%
4. Phase 4 — Step views (deep-link first, currency inline) — ~20%
5. Phase 5 — Auto-show guard + Overview CTA + ImportJob handoff — ~15%
6. Phase 6 — Docs + verification — ~5%
