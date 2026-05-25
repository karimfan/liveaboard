# Sprint 023 Intent: Initial Org Onboarding Wizard

## Seed

> Build a wizard that guides the initial onboarding of a new org to
> the app. Currency, setting up boats, layouts, adding directors.
> Offer the ability to skip that too, but we want to be prescriptive
> to facilitate good outcomes.

## Context

A fresh signup today lands the new Org Admin on the Overview page,
which surfaces a passive setup-completeness card with deep links to
each fix-it screen. The card is informative but not directive: a
brand-new admin sees a list of things to do but has to discover the
order and decide where to start. The seed asks for the opposite — a
prescriptive guided flow that walks the admin through the four
critical setup steps in a sensible order, with the option to step
off at any time.

Every editor the wizard needs already exists: the currency picker
on `/admin/organization` (Sprint 023 currency catalog), the
fleet/import surface on `/admin/fleet` and `/admin/import`, the
cabin layout editor on `/admin/fleet/:id/cabins` (Sprint 014), and
the director invite form on `/admin/users`. The setup-completeness
calculation in `store.SetupCompleteness` (Sprint 022) is already the
single source of truth for "what's missing"; this sprint extends it
to include "boats without layouts" and wraps the existing editors in
a guided stepper.

There is a related deferred chunk from yesterday's debugging — the
"import-then-cabin-layout" mini-wizard — that turns out to be a
subset of this onboarding wizard's "boats → layouts" stretch. The
intent here is to build one wizard that covers both first-run setup
AND post-import layout cleanup, rather than two near-identical
flows.

## Recent Sprint Context

- **Sprint 020 — Pricing Overrides and Currency Defaults**: USD/EUR
  default accepted currencies plus the catalog-pricing snapshots
  that make per-trip revenue reproducible.
- **Sprint 021 — Filesystem Email Transport**: dev/staging
  affordance for testing email flows end-to-end without SMTP.
- **Sprint 022 — Analytical Reports + Postgres-only Storage**:
  unified `SetupCompleteness` in `internal/store/reports.go` (this
  is the data the wizard will derive from), plus Admin / Director /
  Guest report surfaces.

## Relevant Codebase Areas

- `internal/store/reports.go` — `SetupCompleteness` + `SetupStep`
  DTOs. The wizard reuses/extends this data.
- `internal/store/organizations.go` — `UpdateOrganizationProfile`
  (currency). Already auto-adds the country currency to payment
  settings.
- `internal/store/boats.go` — `BoatsForOrg`, `BoatCountForOrg`.
- `internal/store/cabins.go` — `BoatCabinLayout` returns
  `ActiveCabinCount`. A boat with `ActiveCabinCount == 0` is "needs
  a layout". The wizard needs a new helper to enumerate these in
  one query.
- `internal/store/users.go` — `CountActiveUsersByRole`,
  `CreateInvitedUser`.
- `internal/auth/invitations.go` — invitation flow.
- `internal/httpapi/admin.go` — `HandleOverview` already calls
  `SetupCompleteness`. The wizard adds a new endpoint that returns
  the same data plus boats-without-layouts.
- `web/src/admin/pages/Overview.tsx` — current passive checklist.
  The wizard either lives at a new route (e.g. `/admin/onboarding`)
  and Overview links to it, or it inlines a fuller flow on first
  visit.
- `web/src/admin/pages/Organization.tsx`, `Fleet.tsx`,
  `BoatCabins.tsx`, `Users.tsx`, `Import.tsx` — the destination
  editors the wizard deep-links into.

## Constraints

- **Must follow CLAUDE.md conventions**: stdlib-heavy Go,
  strict tenant isolation, multi-tenant by `organization_id`,
  TS + React on the frontend.
- **No duplicate state**: the wizard reads from the existing
  `SetupCompleteness` source of truth. The wizard's job is
  *presentation*, not new state.
- **Skip-able everywhere**: per the seed. Skipping a step doesn't
  alter the underlying setup signal — the step is still "not done"
  in `SetupCompleteness`; it's just been dismissed from the
  wizard view.
- **Prescriptive ordering**: the wizard fixes the order
  (currency → boats → layouts → directors). The Overview checklist
  stays orderable-by-the-user; the wizard is the prescriptive view.
- **Subsumes the deferred import wizard**: the "boats → layouts"
  step in this onboarding flow IS the same UX as
  post-import-layout-cleanup. We design one flow; the post-import
  entrypoint navigates into the layouts step of this wizard.
- **Admin-only**: the wizard is for Org Admins. Cruise Directors
  don't see it.
- **Persistence**: a single nullable timestamp on `organizations`
  (e.g. `onboarding_dismissed_at`) records whether the admin has
  explicitly closed the wizard. The wizard shows when
  `onboarding_dismissed_at IS NULL` AND setup is incomplete.
- **No new external dependencies**.

## Success Criteria

- A brand-new Org Admin who signs up sees the wizard immediately
  after email verification, with currency as the first step,
  pre-suggested from their browser locale if possible.
- Each step embeds (or links to) the existing editor without
  reimplementing the form.
- Each step has both "Continue" (advance to next) and "Skip" (mark
  dismissed, exit to Overview).
- A "Skip onboarding" affordance is also visible from the wizard
  shell at any point.
- Once the admin completes or skips the wizard,
  `onboarding_dismissed_at` is set and the wizard does not
  auto-show again. Admin can re-enter via an "Onboarding" link in
  the Overview if setup is incomplete.
- The boats step offers two paths: "Import from liveaboard.com" or
  "Add manually" — funnels into the existing flows.
- The layouts step lists every boat in the org without a layout
  (not just newly-imported ones), with deep links into the cabin
  layout editor. Returning to the wizard refreshes the list.
- The directors step uses the existing invite form, with a clear
  "Skip for now" option.
- All existing tests pass; new store helpers have DB-backed tests;
  no migration regressions.
- `go test`, `go vet`, `npm run build` all pass.

## Open Questions

The drafts should each take a position and the interview will
resolve disagreement.

1. **Wizard route shape**: dedicated route at `/admin/onboarding`
   the admin can navigate to and from, or a modal/overlay anchored
   on Overview? Dedicated route is friendlier to deep-linking +
   browser back; overlay feels more "wizard-y" but is harder to
   reload.
2. **First-run trigger**: auto-redirect on first authenticated
   admin visit when `onboarding_dismissed_at IS NULL` AND setup is
   incomplete? Or show a banner on Overview with a "Start setup"
   CTA?
3. **Step embed vs link**: should each step embed the editor inline
   (single-page flow) or link out to the existing editor and back
   (multi-page flow)? Inline is cleaner but means duplicating each
   editor's UI logic; linking reuses everything but breaks the
   wizard's continuity.
4. **Skip granularity**: skip-this-step (advance to next) AND
   skip-whole-wizard, or just skip-whole-wizard? Per-step skip
   matches the prescriptive-but-optional framing better.
5. **Boat import wait**: liveaboard.com import is async (returns a
   job id). Does the wizard's boats step wait for the job before
   advancing to layouts, or advance immediately and tell the admin
   "we'll surface the new boats in the layouts step when ready"?
6. **Currency locale guess**: pre-suggest USD always (today's
   default), pre-suggest based on `navigator.language`, or leave
   blank and require explicit selection?
7. **Re-entry**: once dismissed, where's the "open onboarding" link
   shown? On the Overview's setup card? In the user menu? Both?
8. **First director invite**: should the wizard insist on at least
   one director invite before completion, or just suggest it?
9. **Bypassed-old-orgs**: existing orgs (created before this
   sprint) have `onboarding_dismissed_at IS NULL` but are already
   set up — they shouldn't see the wizard. Set the timestamp via
   the migration's backfill for any org with > 0 boats AND > 0
   directors AND a currency? Or just let `SetupCompleteness == 100`
   gate the wizard's auto-show entirely?
10. **Director-only orgs**: an org might be just the admin for a
    while (single-person operation). Should "no other directors"
    block the wizard from "complete"? Probably no — skip is fine.
11. **Trips step**: setup completeness includes trips, but the seed
    omits it. Confirm scope: wizard is currency/boats/layouts/
    directors only; trip creation stays out of the wizard.
