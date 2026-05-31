# Codex Critique of Claude Sprint 026 Draft

Claude's draft gets the broad direction right: all three product
anchors are present, Stripe is excluded, offline deposits are explicit,
and the Triptych winner is named. The main problems are scope-control
and a few repo-contract mismatches that would make the merged sprint doc
ambiguous or misleading.

## Findings

1. **Overview makes locked scope optional.**
   The last Overview paragraph says Anchor 3 may fall back to a
   "read-only / data-model-only subset with the UI in Sprint 027." That
   conflicts with the intent's Interview Lock 1: "All three anchors, one
   sprint." Crew + equipment + certs cannot be framed as optional or
   read-only. If a safety valve is needed, it should trim non-locked
   extras like surveys/photos/reminder emails, not the Anchor 3
   management UI and readiness gate.

2. **Five hero page migrations are incorrectly treated as stretch/cuttable.**
   The Overview says the five highest-traffic migrations are "a stretch
   goal." Phase 6 calls them "migrate 5 hero pages," but the Risks table
   says to drop to 3 if the sprint is heavy. This conflicts with
   Interview Lock 4: Overview, Trips, TripDashboard, TripManifest, and
   GuestFolio migrate this sprint. The merged doc should make these five
   DoD items, not stretch work.

3. **Phase 5 / DoD assumes a `confirmed` trip status and endpoint that do
   not exist.**
   Phase 5 says readiness runs on `POST /api/admin/trips/:id/confirm`,
   and the DoD says "Trip cannot move to `confirmed`." Current
   `trips.status` is `planned | active | completed | cancelled` from
   `internal/store/migrations/0017_trip_lifecycle.sql`, and current
   routes are start/complete/cancel in `internal/httpapi/httpapi.go`.
   Either the sprint must explicitly add `confirmed` and update every
   lifecycle/report/status test, or it should gate the existing
   `POST /api/admin/trips/{id}/start` transition. Right now the draft
   silently invents an enum value and route.

4. **Anchor 1 data model assumes `trip_guests.status` and
   `hold_expires_at` without specifying the migration.**
   The diagram and Phase 2 tasks say `trip_guests` flips to `held` with
   `hold_expires_at`. Existing `trip_guests` has no `status` or
   `hold_expires_at` columns; it tracks invite/account/registration and
   `revoked_at`. This can be a valid design, but Phase 1 must name the
   `ALTER TABLE trip_guests ...` work and explain how `held` coexists
   with invited/revoked/registered states. Otherwise implementers will
   discover this mid-sprint.

5. **Guest portal scope is over-expanded beyond the locked success path.**
   Anchor 2 Architecture and Phase 4 add `trip_photos`, `dive_log`,
   `surveys`, `Rebook`, operator photo upload, and survey submission.
   The intent's resolved locks require the anchor to ship, but the
   success path emphasizes pre-trip flow, today's dive plan, and running
   folio. Full photo gallery and survey machinery are not necessary for
   that path and directly compete with the locked Funnel and Anchor 3
   work. These should be marked stretch/deferred unless already trivial
   through existing document storage.

6. **Reminder emails are added as required work without a scheduler
   design.**
   Phase 5 requires "Reminder emails: 90/30/7 days before cert expiry,"
   and the DoD says reminder emails fire. The repo has email transport,
   but this draft does not specify a scheduler/job runner, idempotency
   table, or how reminders avoid repeat sends. Given the sprint's size,
   the required deliverable should be expiry computation and UI warning;
   email reminders can be deferred or require a concrete job design.

7. **External dependency choice is too casual for this repo.**
   Phase 2 proposes `golang.org/x/time/rate`, and Dependencies lists it
   as the only new Go dependency. CLAUDE.md says Go stdlib + minimal
   dependencies, and public rate limiting can be small enough to build
   locally. A new dependency may be fine, but the sprint doc should not
   make it automatic without a short justification and fallback. The
   risk is not the package itself; it is making a dependency decision in
   a planning doc when this codebase has stayed deliberately lean.

8. **Deploy to GCP VM is out of step with project context and sprint
   close.**
   Phase 7 and DoD require deploy to the GCP VM and tagging the deploy
   in an ADR. CLAUDE.md says local development only for now and cloud
   deployment comes later; the intent says cloud deploy is for
   evaluation only. A deploy can be a manual optional smoke, but making
   it Definition of Done adds operational scope unrelated to the locked
   product/design anchors.

9. **New ADR count is too heavy and partly misplaced.**
   Phase 6 requires ADRs 0005, 0006, and 0007, plus ADR 0004 changes.
   The offline-deposit decision is important enough for an ADR or an
   ADR 0004-adjacent product decision, but token auth and readiness gates
   may be adequately captured in the sprint doc unless they introduce
   durable architectural constraints. Requiring three new ADRs inflates
   documentation work and the DoD without clear payoff.

10. **"Magic-token email replaces the current trip-guest registration link"
    risks breaking existing guest registration semantics.**
    Phase 3 says the portal magic-token email replaces the current
    registration link template. Existing routes and flows
    (`/guest/invitations/:token`, `/guest/trips/:tripGuestId/register`,
    guest sessions) already support registration. Replacing rather than
    linking/integrating may create regressions in Sprint 014/017 guest
    registration/document behavior. The plan should say the portal wraps
    or links to the existing registration path unless the migration
    explicitly preserves all current invitation/session behavior.

11. **Admin UI file paths are inconsistent with the current app shape.**
    The draft introduces `web/src/guest/Portal.tsx` and
    `web/src/guest/pages/*`, but the current app keeps guest-facing
    React pages under `web/src/pages/` (`GuestInvitation`,
    `GuestRegistration`, `GuestTab`). A new `web/src/guest/` area may be
    reasonable, but it should be a deliberate architecture change, not a
    casual file split. Likewise, route examples mix `/q/<token>`,
    `/quotes/:token`, `/api/public/orgs/:slug/trips`, and
    `/charters/:slug`; the merged doc should choose one URL scheme.

12. **`app.css` byte target conflicts with the intent's specific
    threshold.**
    Phase 6 asks for "reduced by at least 15 KB"; the DoD repeats that.
    The intent's success criterion is `app.css` smaller than 30 KB. The
    current file is about 39 KB, so a 15 KB reduction is stricter than
    required and may pull time away from product work. Use the explicit
    `< 30 KB` target unless the final merge intentionally raises the
    bar.

13. **Override path for readiness weakens the anchor unless constrained.**
    Anchor 3 architecture and Phase 5 allow Org Admin override with a
    reason. The intent says "a trip cannot move from `planned` to
    `confirmed` if any required crew cert is expired or equipment is out
    of service." If the merged doc keeps an override, it should be
    explicitly out of the blocking path or reserved for warnings only.
    Otherwise the core readiness gate can be bypassed on day one.

14. **Payment table naming should avoid colliding with existing/payment
    processor language.**
    Anchor 1 names the table `payments`, while Phase 1 names
    `quote_payments`. The intent says a `payments` table still exists,
    but "mirrors the existing folio payment-method pattern" and records
    operator-confirmed offline payments. Because Sprint 015 already has
    `organization_payment_settings` and `guest_folios.payment_method`,
    the merged doc should choose one name, preferably explicit
    (`offline_payments` or `quote_payments`), and state there is no
    processor/payment-intent model.

## Suggested Merge Direction

Keep Claude's end-to-end structure, but tighten the merged sprint around
the resolved locks:

- Make all three anchors required and remove the Anchor 3 read-only
  fallback.
- Make the five page migrations required DoD with the exact file list.
- Gate readiness through the existing lifecycle unless the doc
  explicitly adds and tests a `confirmed` status.
- Defer or downgrade photos, surveys, reminder emails, deploy, and extra
  ADRs if scope pressure appears.
- Specify the offline deposit schema and semantics in a way that cannot
  be mistaken for Stripe/payment processing.
