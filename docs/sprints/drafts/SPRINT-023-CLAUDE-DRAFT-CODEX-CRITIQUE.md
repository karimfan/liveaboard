# Codex Critique of Claude Draft: Sprint 023

## Strengths to Preserve

- The draft correctly chooses a dedicated `/admin/onboarding` route instead of
  a modal. That fits deep-linking, refresh, browser history, and post-import
  re-entry.
- It centers the right four-step order: currency → boats → layouts →
  directors.
- It keeps a single persisted dismissal flag on `organizations`, which is the
  right amount of state for Sprint 023.
- It correctly extends `SetupCompleteness` rather than inventing a parallel
  setup model.
- It identifies the deferred import-to-layout cleanup as the same UX as the
  layouts step.
- It includes the necessary tests around role gating, cross-tenant isolation,
  dismissal idempotency, redirect loops, and layout detection.

## Major Concerns

### 1. The Draft Says "Presentation Layer" but Then Rebuilds Editors

The overview says there is no new business logic and each step deep-links into
existing editors. Later phases embed or add meaningful editor behavior:

- currency step embeds a save form;
- boats step adds a manual boat form and possibly a new `POST /admin/boats`;
- directors step embeds the invite form and lists pending/active directors;
- import progress is moved into the wizard.

That is more than a coordinator wizard. The repo already has working pages for
currency, import, cabin layout, and users. Rebuilding pieces of them increases
duplication and makes this sprint bigger than needed.

Recommended change:

- Default every step to deep links plus status/progress.
- Allow only a tiny inline currency quick-set if it reuses `CurrencyPicker` and
  the existing org patch API.
- Do not add manual boat creation in this sprint unless the final planner
  explicitly wants boat CRUD scope.
- Extract the invite modal only if the implementation proves it is a small,
  clean reuse; otherwise deep-link to `/admin/users`.

### 2. Manual Boat Creation Is a Scope Trap

The current app appears to create boats through import flows, not a standalone
manual-create endpoint. Claude flags this as an open question but still builds
the sprint around an inline manual-add form and lists `POST /api/admin/boats`
as an optional API endpoint.

This is likely to pull in validation, slug/source semantics, image/source URL
defaults, fleet UI states, tests, and follow-on edit behavior. That is a
separate fleet-management sprint, not a requirement for a wizard.

Recommended change:

- The boats step should offer "Import from liveaboard.com" and "Import
  spreadsheet" as the concrete working paths.
- If "Add manually" must appear, make it a placeholder/deep link to the Fleet
  page until a real manual boat story is planned.
- Remove `POST /api/admin/boats` from Sprint 023 unless the final merge
  explicitly expands scope.

### 3. `setup.pct < 100` Conflicts with the Trips Step Omission

The draft says `SetupCompleteness` grows from four to five steps by adding
`layouts`, while the existing `trips` step remains. It then says first-run
redirect should trigger when `dismissed_at IS NULL AND setup.pct < 100`.

That means an org that has currency, boats, layouts, and directors but no trips
would still be auto-redirected into a wizard that has no trips step. This
contradicts the open question that trips are intentionally out of wizard scope.

Recommended change:

- Compute onboarding completion from the four onboarding steps, not raw
  `SetupCompleteness.Percent`.
- Return both `setup_pct` and `onboarding_complete` if useful.
- Gate auto-show on `dismissed_at IS NULL && !onboarding_complete`.
- Keep trips in Overview/Reports setup if desired, but do not let it drive the
  first-run wizard redirect.

### 4. The Migration Backfill Is Both Unnecessary and Incomplete

Claude proposes backfilling `onboarding_dismissed_at` for orgs with currency,
boats, and active directors. That condition ignores layouts, which are the new
setup signal this sprint adds. It also risks hard-coding a one-time definition
that will age poorly.

Existing complete orgs do not need a backfill if auto-show is gated on derived
onboarding completeness. Existing incomplete orgs should probably be allowed to
see the wizard or dismiss it themselves.

Recommended change:

- Add the nullable column without broad backfill.
- Gate auto-show on current derived onboarding state.
- If a backfill remains, it must include layout coverage and should be clearly
  documented as a product decision, not just a migration convenience.

### 5. Post-Import `boat_filter=new` Is Not Supported by Current Import Data

The draft says the import page can link to
`/admin/onboarding?focus=layouts&boat_filter=new` or
`/admin/onboarding/layouts?boat_filter=new&job=<id>` and show only newly
imported boats.

Current import job data, as used by `ImportJobView`, exposes trip counts, not a
list of boat IDs created or updated by that job. The liveaboard flow appears to
start from one boat listing URL and import trips for that boat, so "5 newly
imported boats" is likely the wrong model.

Recommended change:

- Keep the post-import handoff simple: after success, link to
  `/admin/onboarding?step=layouts` or `/admin/onboarding/layouts`.
- The layouts step should list every boat in the org without a layout.
- Defer job-specific boat filtering until import jobs persist enough result
  metadata to support it.

### 6. Route Shape May Be Overbuilt

Claude proposes both `/admin/onboarding` and four nested routes:

- `/admin/onboarding/currency`
- `/admin/onboarding/boats`
- `/admin/onboarding/layouts`
- `/admin/onboarding/directors`

That is workable, but it creates more router surface and more files than the
wizard needs. A query-param step (`/admin/onboarding?step=layouts`) is enough
for deep links and easier to keep inside one page.

Recommended change:

- Prefer one route plus `?step=` unless implementation ergonomics strongly
  favor nested routes.
- If nested routes stay, ensure `main.tsx` and `Onboarding.tsx` handle unknown
  step keys and back/forward navigation cleanly.

### 7. Layout Detection Is Under-Specified

The draft says a layout is done when every boat has at least one active cabin.
The intent and cabin model care about usable layouts, which means active berths
matter too. A boat with one active cabin and zero active berths should still be
considered missing a usable layout.

Recommended change:

- Define layout completion as at least one active cabin and at least one active
  berth.
- Test inactive cabins, inactive berths, active cabin with zero active berths,
  and two-org isolation.
- Consider naming helpers `BoatsWithoutCabinLayouts` or
  `BoatLayoutStatuses` to avoid ambiguous "layout" semantics.

## Missing or Under-Specified Details

- **Organization struct location:** The draft names
  `internal/store/organizations.go`, but the `Organization` type may live in
  `internal/store/store.go`; final file list should include it.
- **Audit event scope:** The draft requires an audit event for dismissal. That
  may be fine, but it is not in the intent. If retained, specify event name,
  actor, and org metadata, and add the test to the phase.
- **Locale currency guess:** `navigator.language` does not reliably map to a
  currency, and JavaScript `Intl` does not provide a direct locale-to-currency
  API. This should be a best-effort suggestion, not a Definition of Done item
  for several currencies unless a static map is deliberately added.
- **`current_step` on the API:** Step focus is mostly presentation state. The
  server can return steps and completion; the client can choose first
  incomplete or query-selected step. If the server honors `?focus=`, keep the
  behavior simple and validate allowed step keys.
- **Docs file list:** `local_setup.md` is listed, but that file does not appear
  in the repo file listing. Use an existing docs file or omit it.
- **Dependency wording:** "Sprint 023 (this session's earlier work)" is
  confusing inside a Sprint 023 draft. Reference the existing currency picker
  or Sprint 020 currency/default work instead.

## Suggested Scope Changes

- Keep the dedicated onboarding route, Org Admin-only API, dismissal timestamp,
  Overview re-entry, and post-import layouts link.
- Replace route-level auto-show condition with
  `dismissed_at IS NULL && !onboarding_complete`.
- Remove backfill unless the merge phase intentionally wants existing
  incomplete orgs suppressed.
- Remove manual boat creation from the sprint.
- Remove job-specific `boat_filter=new` handling from the sprint.
- Use existing editor routes as the primary step actions.
- Treat locale currency guess as a nice-to-have, not a core acceptance
  criterion.
- Define layout completeness as active cabins plus active berths.

## Risks the Final Merge Should Address

- **Over-scoping:** Manual boat creation, embedded forms, and import polling
  inside the wizard can consume the sprint and delay the core guided flow.
- **Redirect mismatch:** Using overall setup percent can keep redirecting users
  for a trips step that the wizard intentionally does not include.
- **Silent data drift:** Backfilling dismissal based on an incomplete
  definition can hide onboarding from orgs that still need layout work.
- **Unsupported import filtering:** Job-specific layout filtering needs data the
  current import job response does not expose.
- **Duplicated UI logic:** Embedded currency/director forms can drift from
  their canonical pages unless components are intentionally shared.

## Parts to Reject or Simplify

- Reject `POST /api/admin/boats` from this sprint unless manual fleet creation
  is explicitly promoted into scope.
- Reject `boat_filter=new` and automatic import-job-to-layout filtering for now.
- Reject `setup.pct < 100` as the auto-show condition; use onboarding-specific
  completion.
- Simplify per-step routes to one route with `?step=` unless nested routes are
  clearly worth the added surface.
- Simplify the migration to a nullable timestamp without broad backfill.
- Soften the locale currency guess from a required behavior to a best-effort
  enhancement.

## Bottom Line

Claude's draft has the right product direction and several useful guardrails,
but it should be merged into a leaner implementation. Sprint 023 should ship a
reliable coordinator wizard over existing admin pages, not new fleet CRUD,
job-specific import metadata, or duplicate editor forms. The final plan should
gate first-run onboarding on the four wizard steps, keep setup completeness as
the shared signal, and preserve the escape hatch with one persisted dismissal
timestamp.
