# Sprint 026 Merge Notes

## Claude Draft Strengths (worth preserving)

- Concrete posture C narrative ("polish vs feature wave vs
  two-deck") and the three-anchor framing.
- Use cases enumerate the user journey end-to-end with
  realistic actors.
- Documentation Manifest section already structured; can be
  imported wholesale into the final doc.
- "Sprint 027 preview" at the bottom names what falls off if
  scope tightens.

## Codex Draft Strengths (worth adopting)

- **Concrete migration SQL** for every new entity, with
  CHECK constraints, indexes, and FK ON DELETE behavior
  spelled out — far more actionable than my prose tables.
- **Hold state on `booking_quotes`, not `trip_guests`** —
  cleaner separation. Sidesteps the "we need to ALTER
  trip_guests" problem I waved at.
- **`offline_payments` with `direction` enum** is a clearer
  table name than my `quote_payments` and uses the same
  shape for deposit/refund.
- **Scope Boundaries / Out of scope** section sets a hard
  fence — my draft has the same content scattered across
  three paragraphs.
- **Explicit acknowledgement that `confirmed` status doesn't
  exist** — Codex's recommendation to gate readiness on the
  existing `start` transition (rather than invent a new
  enum value) is the right pragmatic call.
- **Hashed tokens** in `booking_quotes.token_hash` — better
  than storing tokens plain.
- **Phase split** (15/20/20/20/15/10) is more even than mine.

## Valid Critiques Accepted

1. **Drop the Anchor 3 fallback hedge.** Per Lock 1, all
   three anchors ship. Overview rewritten without the
   "or fall back to read-only" escape clause. If scope
   pressure appears, drop non-locked extras (surveys,
   photos, dive log, post-trip rebook automation), not
   anchor scope.

2. **5 hero migrations are DoD, not stretch.** Per Lock 4.
   Overview, Trips, TripDashboard, TripManifest, GuestFolio
   are required deliverables, listed in the DoD by exact
   filename.

3. **No invented `confirmed` status.** Current trip status
   is `planned | active | completed | cancelled` (`internal/
   store/migrations/0017_trip_lifecycle.sql`). Readiness
   gates on the existing `POST /api/admin/trips/{id}/start`
   transition. Adding a `confirmed` pre-start state is a
   future product enhancement, not Sprint 026.

4. **`booking_quotes` carries hold state, not `trip_guests`.**
   Adopt Codex's schema. `hold_expires_at` on the quote;
   `trip_guests` gets a row only when registration starts.
   Avoids touching the Sprint 014 `trip_guests` schema.

5. **Trim guest portal to the success path.** Pre-trip
   (itinerary, briefing PDFs, registration link, cert
   upload, equipment+dietary requests) + on-trip (today's
   day plan, schedule, running folio) are required.
   **Cut from Sprint 026:** trip photos, dive log,
   surveys, "rebook next year" automation. The post-trip
   tab exists with a placeholder CTA but no implementation.

6. **No certification reminder emails.** The repo has no
   scheduler; adding one is its own concern. Sprint 026
   ships:
   - Server-side expiry computation (90/30/7 day buckets).
   - UI warnings on `Crew` and `TripDashboard` for expiring/
     expired certs.
   - Email reminders move to Sprint 027.

7. **No new Go dependencies.** Per CLAUDE.md's "stdlib +
   minimal dependencies" rule. Rate limiting is a small
   token-bucket implementation in
   `internal/httpapi/rate_limit.go` (~50 LoC). No
   `golang.org/x/time/rate` import.

8. **Deploy is not DoD.** Per CLAUDE.md "local development
   only for now." Production deploy via `deploy/deploy.sh`
   is an optional manual smoke after the sprint closes,
   not a checkbox on the DoD.

9. **Only one new ADR.** Drop the planned ADRs 0006 (guest
   portal token auth — reuses existing `GuestSession`) and
   0007 (readiness gate — pure feature work captured in the
   sprint doc). Keep:
   - **ADR 0005**: `0005-offline-deposit-funnel.md`. Why no
     Stripe; why offline-confirmed; how it mirrors Sprint 015.

10. **Portal links to existing registration; doesn't replace
    it.** Sprint 014's `/guest/invitations/:token` and
    `/guest/trips/:tripGuestId/register` keep working
    unchanged. The portal embeds a link to the existing
    registration flow as a checklist item.

11. **Frontend file paths follow the existing repo shape.**
    Guest pages live at `web/src/pages/` (matching
    `GuestRegistration`, `GuestTab`). Portal:
    `web/src/pages/GuestPortal.tsx` + a small subdir for
    portal sections. URL scheme:
    - `/charters/:slug` — public catalog
    - `/charters/:slug/trips/:tripId` — public trip detail
    - `/q/:token` — guest quote acknowledgement
    - `/g/:token` — guest portal

12. **`app.css` target follows the intent: < 30 KB.** Current
    is ~39 KB. Target is reduction of ~9 KB, not 15.

13. **Readiness override is constrained.** Override exists
    (operators need it for last-minute cert renewals, paper
    docs, insurance edge cases) BUT:
    - Override requires a free-text reason ≥ 20 chars.
    - Override writes a structured audit row with the failed
      checks + the reason.
    - TripDashboard shows the override status until the trip
      completes; post-trip review is on the user.

14. **Adopt `offline_payments` as the table name.** Not
    `payments`, not `quote_payments`. Self-documenting,
    no Stripe-adjacent semantics implied.

## Critiques Rejected (with reasoning)

- None outright. Every Codex finding is valid or partially
  valid. Where Codex pushed toward "cut anchor scope," the
  locks override (Locks 1+4); I keep the anchors and cut
  non-locked extras instead.

## Interview Locks Applied

- **Lock 1 (all three anchors).** Overview, DoD, and Risks
  table all reflect this — no "or fall back" hedge.
- **Lock 2 (no Stripe / offline-confirmed deposits).** Schema
  + endpoints + ADR 0005 all reflect offline-only.
- **Lock 3 (Triptych lock now, delete in 027).** Phase 5
  hardcodes default, tightens dev-flag gate, leaves losing
  palettes/nav/motion in tree.
- **Lock 4 (5 hero migrations).** Listed by file in DoD;
  `app.css < 30 KB` after migration.

## Final Decisions

| Decision | Resolution |
|---|---|
| Anchor 1 storage shape | Adopt Codex's `leads`, `booking_quotes`, `offline_payments` schema; `booking_quotes` carries hold state. |
| Anchor 2 portal scope | Pre-trip + on-trip + folio view + cert upload + briefing PDFs + day plan. Photos, dive log, surveys, post-trip rebook = Sprint 027. |
| Anchor 3 readiness gate | Server-side check on `POST /api/admin/trips/{id}/start`. Blocks with structured `failures[]`. Override allowed with reason ≥ 20 chars + audit row. Warnings surface before the gate fires (TripDashboard + Trips list). |
| Trip status enum | Unchanged. No `confirmed` value. |
| Certification reminder emails | Out of scope. UI warnings only. Sprint 027 adds email job. |
| Rate limiting | Local token-bucket implementation. No new Go deps. |
| Deploy | Not DoD. Optional post-sprint manual smoke. |
| New ADRs | Only ADR 0005 (offline-deposit-funnel). |
| Portal URLs | `/charters/:slug`, `/charters/:slug/trips/:tripId`, `/q/:token`, `/g/:token`. |
| Portal file location | `web/src/pages/GuestPortal.tsx` + `web/src/pages/guestPortal/*` for sections. |
| `app.css` target | < 30 KB after the 5 migrations. |
| Triptych production lock | DEFAULT_MODE hardcoded; URL/localStorage reads gated by `useDevFlags().ui_redesign_switcher`; switcher mounts only when flag true. Loser code stays in tree. |
