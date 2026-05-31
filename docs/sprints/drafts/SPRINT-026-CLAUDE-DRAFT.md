# Sprint 026: Open the Funnel — Booking, Guest Portal, Crew & Equipment

## Overview

Liveaboard ships the back office; Sprint 026 adds the front of
the house. Three end-to-end anchors take the platform from
"manage a charter that already exists" to "fill the boat,
serve the guest, audit the crew, close the books":

1. **Open the funnel.** A public trip catalog per org, a real
   inquiry → quote → deposit → berth-hold flow, and a refund
   workflow. First time the platform meets untrusted traffic.
2. **Guest portal.** A token-authenticated guest surface at
   `/g/<token>` that owns the pre-trip, on-trip, and post-trip
   experience — itinerary, dive briefings, schedule, running
   folio, dive log, photo gallery, post-trip survey.
3. **Crew, equipment, certifications.** A real supply-side
   model: crew roster with roles, equipment by serial with
   service history, certifications with expiry tracking and
   trip-readiness gates that block confirming a trip when a
   required cert is expired or a required asset is out of
   service.

The Triptych design framework from Sprint 025 gets locked:
`manta-night / spaces / living` is the production default; the
switcher is removed from non-dev builds; the other palettes
and nav components stay in the tree as references for Sprint
027's collapse pass. As we build new pages for the three
anchors, we put them on the new component library; as a
stretch goal, the five highest-traffic existing pages
(Overview, Trips, TripDashboard, TripManifest, GuestFolio) get
migrated too. `app.css` shrinks measurably.

This sprint is large. Posture is "ship Anchor 1 fully, ship
Anchor 2 fully, ship Anchor 3 fully *or* fall back to its
read-only / data-model-only subset with the UI in Sprint 027."

## Use Cases

1. **Liveaboard.com competitor browses the platform.** Lands on
   `/charters/coral-expeditions`, sees a card per upcoming trip
   with photo, dates, itinerary, price-per-berth, cabin
   availability, an "Inquire" CTA.
2. **Prospective guest inquires.** Submits the inquiry form
   with party size, date window, and questions. Form goes to a
   `leads` table; Org Admin is notified.
3. **Org Admin sends a quote.** Picks a specific trip,
   confirms the cabin assignment, generates a quote with a
   deposit amount, and emails a one-click link.
4. **Guest acknowledges the quote + sends the deposit
   offline.** Clicks the magic link, reviews the quote, hits
   "I'll send the deposit." Operator gets notified. When
   the bank transfer / Wise / cash lands, the operator
   records the deposit in the admin UI. The guest's berth
   on the trip moves to `held` for N days while the guest
   completes the registration portal.
5. **Guest fills the portal pre-trip.** Lands at `/g/<token>`,
   sees the trip itinerary, fills out registration, uploads
   certs, requests rentals, completes dietary, reviews dive
   plan.
6. **Org Admin manages crew.** Adds Captain, Dive Master, Chef
   to the roster. Each crew member's profile lists
   certifications (CPR expires 2026-08-15, Dive Master expires
   2027-04-02), each tied to a renewal reminder.
7. **Org Admin sees a trip blocked.** Tries to mark next
   month's trip `confirmed`. System refuses: "Dive Master Anna's
   EFR cert expired 2026-04-30. Resolve or override."
8. **Trip starts; guest checks the portal.** Sees today's dive
   site, weather notes from the Captain, the running folio.
9. **Refund issued.** Guest cancels a held booking in the
   refund window. Org Admin pushes the money back through
   the original channel offline, then clicks "Record
   refund"; the held berth releases and the quote moves
   to `cancelled`.

## Architecture

### Anchor 1 — Funnel (offline-deposit flow, no Stripe)

```
                                  ┌─────────────────────┐
  Public web ─── inquiry ────────►│ leads               │
                                  │  (org_id, contact,  │
                                  │   party, dates,     │
                                  │   notes, status)    │
                                  └──────────┬──────────┘
                                             │ Org Admin picks
                                             │ a trip + cabin
                                             ▼
                                  ┌─────────────────────┐
                                  │ quotes              │
                                  │  (trip_id, guest_id,│
                                  │   amount_usd_cents, │
                                  │   deposit_cents,    │
                                  │   deposit_method,   │
                                  │   instructions,     │
                                  │   expires_at, …)    │
                                  └──────────┬──────────┘
                                             │ email link to guest
                                             ▼
                                  ┌─────────────────────┐
                                  │ /q/<token>          │
                                  │  Guest reviews;     │
                                  │  acknowledges       │
                                  │  ("I'll send the    │
                                  │   deposit"). Quote  │
                                  │  status: accepted   │
                                  └──────────┬──────────┘
                                             │ Org Admin gets notified;
                                             │ confirms when money arrives
                                             ▼
                                  ┌─────────────────────┐
                                  │ payments            │
                                  │  (kind: deposit,    │
                                  │   method: bank/cash │
                                  │     /wise/other,    │
                                  │   amount_cents,     │
                                  │   confirmed_by,     │
                                  │   confirmed_at,     │
                                  │   reference)        │
                                  └──────────┬──────────┘
                                             │ on confirmation
                                             ▼
                                  ┌─────────────────────┐
                                  │ trip_guests         │
                                  │  status: held,      │
                                  │  hold_expires_at    │
                                  └─────────────────────┘
```

**No money flows through the platform.** Operators receive
deposits via their existing channels (bank transfer, cash,
Wise, etc. — same model as the existing folio payment
methods) and **manually record receipt** in the admin UI.
This drops Stripe entirely, removes any PCI surface, and
keeps Anchor 1 squarely in the project's existing
"offline-confirmed payments" idiom (Sprint 015).

**New entities:** `leads`, `quotes`, `payments` (offline-
confirmation records — same shape as the existing folio
payments, but tied to a quote, not a folio).

**API:**
- `GET  /api/public/orgs/:slug/trips`                 — public catalog
- `POST /api/public/orgs/:slug/inquiries`             — rate-limited
- `POST /api/admin/quotes`                            — Org Admin creates
- `POST /api/admin/quotes/:id/send`                   — emails the guest a `/q/<token>` link
- `GET  /api/public/quotes/:token`                    — guest reviews
- `POST /api/public/quotes/:token/accept`             — guest acknowledges intent to pay
- `POST /api/admin/quotes/:id/payments`               — Org Admin records deposit received
- `POST /api/admin/quotes/:id/payments/:pid/refund`   — Org Admin records deposit refunded
- `POST /api/admin/quotes/:id/cancel`                 — release the hold

Refund is **also** offline: the operator pushes the money
back through whatever channel they received it on; the
admin marks "refunded" in the UI; the platform records the
event and releases the berth. Like an offline credit-note.

This shrinks Anchor 1 by ~40% vs the Stripe version. No
webhook, no SDK, no signature verification, no Stripe
Connect onboarding. The funnel still ships end-to-end —
public catalog, inquiry, quote, acceptance, deposit
recording, berth hold, refund — just with the
"money-moving" step delegated to the operator's existing
banking workflow.

### Anchor 2 — Guest portal

```
                     Email link with magic token
                                │
                                ▼
                  ┌─────────────────────────┐
                  │ /g/<token>              │
                  │   token resolves to     │
                  │   trip_guest_id;        │
                  │   server issues short   │
                  │   guest session cookie  │
                  │   (existing pattern)    │
                  └────────────┬────────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        ▼                      ▼                      ▼
    Pre-trip               On-trip                Post-trip
   ─────────              ─────────              ─────────
   - itinerary            - today's plan         - dive log
   - registration         - schedule              - photo gallery
   - cert upload          - weather               - survey
   - rentals              - running folio         - "rebook" CTA
   - dietary              - emergency contacts
   - briefing PDFs
```

**New entities:** `dive_plans` (per trip, day-by-day plan +
sites + briefing PDF), `trip_photos` (object-storage URLs +
captions), `dive_log_entries` (per trip-guest), `surveys`
(post-trip).

Guest auth: reuse the existing `GuestSession` pattern that
`/guest/trips/:tripGuestId/register` already uses. Magic
token in the email expands to a short-lived session cookie;
session lives for the duration of the trip + 90 days post.

### Anchor 3 — Crew, equipment, certifications

```
  organizations
       │
       │
       ▼
  crew_members (org-scoped)
       │
       ├─► crew_certifications (with expiry_date, reminder)
       │
       └─► trip_crew_assignments (which crew on which trip,
                                  with role)

  boats
       │
       │
       ▼
  equipment_assets (per-boat, by serial)
       │
       ├─► equipment_service_log (date, service type, notes,
       │                          performed_by)
       │
       └─► equipment_status: in_service | out_of_service |
                              retired

  trips
       │
       ├─► readiness check on confirm:
       │     - required crew roles all assigned
       │     - all assigned crew's required certs in date
       │     - required equipment all in_service
       │
       └─► (override path: Org Admin can confirm despite
           failure with a reason + audit row)
```

**New entities:** `crew_members`, `crew_certifications`,
`trip_crew_assignments`, `equipment_assets`,
`equipment_service_log`. Plus `cert_kinds` and `equipment_kinds`
as small lookup tables (operator-extensible).

### UI — chrome and IA

The locked production combo is `manta-night / spaces / living`.
The Triptych framework stays in the tree; `TriptychSwitcher`
mounts only when `useDevFlags().ui_redesign_switcher === true`
(already the case — we tighten the production gate). The
provider's URL + localStorage reads are gated behind the same
flag in production so production users can't accidentally land
on `/admin?triptych=coral-surge,canvas,full`.

New pages — every Anchor's pages — ship on the component
library (`web/src/admin/components/`) with co-located `*.module.css`
and semantic tokens only.

Migrated legacy pages (stretch): Overview, Trips,
TripDashboard, TripManifest, GuestFolio.

A new sidebar space appears: **Funnel** (under Operations) for
leads + quotes + payments. **Crew** moves from Configuration to
Operations (it's now a daily-use surface). **Settings** absorbs
Organization / Payments / Pricing.

## Implementation Plan

### Phase 1: Funnel data model (offline-deposit, no Stripe) (~10%)

**Files:**
- `internal/store/migrations/00NN_leads_quotes_payments.sql` — three new tables + multi-tenant scoping.
- `internal/store/leads.go`, `quotes.go`, `quote_payments.go` — CRUD + transitions.

**Tasks:**
- [ ] Migration adds `leads`, `quotes`, `quote_payments`.
- [ ] All three tables have `organization_id` + tenant scoping enforced at the store layer.
- [ ] `quote_payments` mirrors the existing folio-payment shape: `kind` (`deposit`/`refund`), `method` (`bank`/`cash`/`wise`/`other`), `amount_minor_cents`, `currency`, `confirmed_by` (user id), `confirmed_at`, `reference` (free text).
- [ ] Store transitions tested: `lead.new → contacted → quoted → won/lost`; `quote.draft → sent → accepted → deposit_received → expired/cancelled`.
- [ ] No external dependencies added.

### Phase 2: Public funnel endpoints + admin quote/payment flow (~12%)

**Files:**
- `internal/httpapi/public_handlers.go` — public catalog + inquiry POST (rate-limited via `golang.org/x/time/rate`).
- `internal/httpapi/quote_handlers.go` — admin quote CRUD + send + accept (public token) + record-payment + record-refund + cancel.
- `web/src/pages/charters/*` — public catalog + trip detail + inquiry form (auth not required).
- `web/src/pages/QuoteAccept.tsx` — guest-facing quote review + acknowledge.
- `web/src/admin/pages/Funnel.tsx` — leads + quotes + payments dashboard.
- `web/src/admin/pages/QuoteDetail.tsx` — single quote: edit, send, record deposit, refund, cancel.

**Tasks:**
- [ ] `GET /api/public/orgs/:slug/trips` returns published upcoming trips with cabin availability snapshot (explicit columns only — no `SELECT *`).
- [ ] `POST /api/public/orgs/:slug/inquiries` rate-limited per IP (5/hour) and per org (100/day); both configurable.
- [ ] `POST /api/admin/quotes` creates a quote tied to a trip + cabin assignment proposal; `POST /api/admin/quotes/:id/send` emails the guest a magic `/q/<token>` link.
- [ ] `GET /api/public/quotes/:token` renders the quote for the guest; `POST /api/public/quotes/:token/accept` flips status to `accepted` and notifies the operator.
- [ ] `POST /api/admin/quotes/:id/payments` records an offline deposit confirmation; on first `deposit` record, the corresponding `trip_guest` flips to `held` with `hold_expires_at = now() + hold_window`.
- [ ] `POST /api/admin/quotes/:id/payments/:pid/refund` records the refund (operator pushes money back through their channel); when the deposit-refund pair fully zeros, the trip_guest hold is released and the quote moves to `cancelled`.
- [ ] `POST /api/admin/quotes/:id/cancel` releases the hold without a refund record (no money was received).
- [ ] `web/src/pages/charters/*`, `QuoteAccept.tsx`, `Funnel.tsx`, `QuoteDetail.tsx` use ONLY component library primitives + semantic tokens.
- [ ] Frontend smoke: render-check via TypeScript + `npm run build`. No Playwright in scope.

### Phase 3: Guest portal — pre-trip (~15%)

**Files:**
- `internal/store/migrations/00NN_dive_plans_trip_photos.sql` — schema additions.
- `internal/store/dive_plans.go`, `trip_photos.go`.
- `internal/httpapi/guest_portal_handlers.go` — `/api/guest/portal/...` endpoints behind GuestSession.
- `web/src/guest/Portal.tsx` — guest portal shell.
- `web/src/guest/pages/{Itinerary,Registration,Briefing,Rentals}.tsx`.

**Tasks:**
- [ ] Dive plan per trip: day-by-day sites + briefing PDF reference + crew notes.
- [ ] Guest sees their itinerary + dive plan + rental request form.
- [ ] Guest can upload certs (existing pattern in registration; extended to a managed list).
- [ ] All guest portal pages use component library.
- [ ] Magic-token email replaces the current trip-guest registration link template (one path, more content).

### Phase 4: Guest portal — on-trip + post-trip (~10%)

**Files:**
- `web/src/guest/pages/{Today,Folio,DiveLog,Photos,Survey,Rebook}.tsx`.
- `internal/store/dive_log.go`, `surveys.go`.
- `internal/httpapi/guest_portal_handlers.go` — additions.

**Tasks:**
- [ ] Guest sees today's dive site + plan from the dive plan on the relevant trip day.
- [ ] Guest sees their running folio (read-only — checkout still happens at trip end via Org Admin).
- [ ] Guest can submit a dive log entry; operator can pre-fill (depth, time, conditions).
- [ ] Operator uploads trip photos (server-side: just metadata + URL to whatever object storage; for evaluation we can use the existing `DOCUMENTS_DIR`).
- [ ] Post-trip survey: 5-question template (will customize later).
- [ ] "Book next year" CTA opens a pre-filled inquiry form.

### Phase 5: Crew + equipment + certifications (~15%)

**Files:**
- `internal/store/migrations/00NN_crew_equipment.sql`.
- `internal/store/crew.go`, `equipment.go`, `cert_kinds.go`.
- `internal/httpapi/crew_handlers.go`, `equipment_handlers.go`.
- `web/src/admin/pages/Crew.tsx`, `CrewMember.tsx`, `Equipment.tsx`, `EquipmentAsset.tsx`.

**Tasks:**
- [ ] Crew member entity with role enum + profile.
- [ ] Crew certifications with `kind`, `issuer`, `issued_at`, `expires_at`, `evidence_doc_id`.
- [ ] Equipment by serial with kind, assigned-boat, status, and service log.
- [ ] Trip readiness check runs on `POST /api/admin/trips/:id/confirm`: returns 409 + structured `blockers[]` array unless override flag is passed (with audit reason).
- [ ] Reminder emails: 90/30/7 days before cert expiry (via the existing email transport).
- [ ] Pages use component library.

### Phase 6: Lock Triptych production + migrate 5 hero pages + docs (~10%)

**Files:**
- `web/src/admin/design/DesignModeProvider.tsx` — tighten production gate.
- `web/src/admin/components/TriptychSwitcher.tsx` — verify dev-only mount.
- `web/src/admin/pages/{Overview,Trips,TripDashboard,TripManifest}.tsx` + `GuestFolio.tsx` — migrate to components.
- `web/src/admin/pages/*.module.css` — co-located CSS for each migrated page.
- `docs/decisions/0004-triptych-runtime-evaluation.md` — amend: locked combo, deferred collapse to Sprint 027.
- `docs/decisions/0005-offline-deposit-funnel.md` — NEW.
- `docs/decisions/0006-guest-portal-token-auth.md` — NEW.
- `docs/decisions/0007-trip-readiness-gates.md` — NEW.
- `DESIGN.md` — append decisions log rows for the locked combo + funnel/guest/crew patterns.
- `CLAUDE.md` — append: public endpoints get rate-limit + structured input validation by default.
- `docs/sprints/SPRINT-026.md` — DoD ticked.

**Tasks:**
- [ ] Provider checks the dev-flag before reading URL/localStorage in production.
- [ ] Switcher does not appear in `npm run build` artifacts when flag is false.
- [ ] Five hero pages render correctly under `manta-night / spaces / living` with no use of `.admin-card`, `.admin-page-header`, `.admin-page-title`, `.setup-list__*`, etc.
- [ ] `app.css` total bytes reduced by at least 15 KB.
- [ ] ADRs 0005, 0006, 0007 land.
- [ ] All migrations applied, all tests green, full build clean.

### Phase 7: Verification + deploy (~10% — overlaps with Phase 6)

**Tasks:**
- [ ] `go test ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `gofmt -l .` empty.
- [ ] `npm run build` clean.
- [ ] Manual smoke matrix: public catalog → inquiry → quote → guest acknowledges → operator records deposit → berth held → guest portal pre-trip flow → trip readiness check passes/fails → refund recorded.
- [ ] Deploy to GCP VM via `deploy/deploy.sh` (with `manta-night` as the locked default).
- [ ] Tag the deploy in the ADR.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/00NN_*.sql` | Create | Three new migrations: funnel; dive plans + photos + dive log + surveys; crew + equipment + certs. |
| `internal/store/{leads,quotes,quote_payments}.go` | Create | Funnel entities (offline-deposit model). |
| `internal/store/{dive_plans,trip_photos,dive_log,surveys}.go` | Create | Guest portal entities. |
| `internal/store/{crew,equipment,cert_kinds}.go` | Create | Supply-side entities. |
| `internal/httpapi/{public,quote}_handlers.go` | Create | Funnel HTTP. |
| `internal/httpapi/guest_portal_handlers.go` | Create | Guest portal HTTP. |
| `internal/httpapi/{crew,equipment}_handlers.go` | Create | Crew + equipment HTTP. |
| `internal/httpapi/trip_lifecycle_handlers.go` | Modify | Readiness check on confirm transition. |
| `internal/config/config.go` | Modify | New per-org `hold_window_days` default + inquiry rate-limit knobs. |
| `cmd/server/main.go` | Modify | Wire the rate-limit middleware + readiness-check route. |
| `web/src/pages/charters/*` | Create | Public catalog + inquiry. |
| `web/src/admin/pages/Funnel.tsx` | Create | Leads + quotes + payments dashboard. |
| `web/src/admin/pages/{Crew,CrewMember,Equipment,EquipmentAsset}.tsx` | Create | Crew + equipment management. |
| `web/src/guest/Portal.tsx` + `web/src/guest/pages/*` | Create | Guest portal. |
| `web/src/admin/pages/{Overview,Trips,TripDashboard,TripManifest,GuestFolio}.tsx` | Modify | Migrate to component library (Phase 6). |
| `web/src/admin/pages/*.module.css` | Create | Co-located CSS for migrated pages. |
| `web/src/admin/Shell.tsx` | Modify | Add Funnel + Crew nav items; reshape sidebar groups. |
| `web/src/admin/nav.ts` | Modify | NAV constant gets Funnel + Crew + Settings restructure. |
| `web/src/admin/design/DesignModeProvider.tsx` | Modify | Production gate on URL/localStorage reads. |
| `web/src/styles/app.css` | Modify | Delete the styles superseded by Phase 6 migration. |
| `docs/decisions/0005-offline-deposit-funnel.md` | Create | Funnel ADR — why no Stripe; offline-deposit-confirmation flow. |
| `docs/decisions/0006-guest-portal-token-auth.md` | Create | Guest portal ADR. |
| `docs/decisions/0007-trip-readiness-gates.md` | Create | Readiness gate ADR. |
| `docs/decisions/0004-triptych-runtime-evaluation.md` | Modify | Locked-combo amendment. |
| `DESIGN.md` | Modify | Decisions log rows for the locked combo + the funnel/guest/crew patterns. |
| `CLAUDE.md` | Modify | Public endpoint rate-limit/validate rule. |
| `docs/sprints/SPRINT-026.md` | Create | This sprint (after merge). |

## Definition of Done

- [ ] Public trip catalog renders at `/charters/:slug` for any org with at least one published trip.
- [ ] Inquiry form on the public catalog rate-limits per-IP and per-org.
- [ ] Org Admin can create a quote tied to a trip + cabin, send it, and see its status.
- [ ] Guest can acknowledge a quote via the magic link (`POST /api/public/quotes/:token/accept`); operator is notified.
- [ ] Org Admin can record an offline deposit confirmation; the corresponding trip_guest flips to `held` for the hold window.
- [ ] Berth auto-releases after hold window if registration doesn't complete.
- [ ] Org Admin can record a deposit refund (offline-pushed back); berth releases and quote moves to `cancelled`.
- [ ] Guest portal at `/g/<token>` is reachable via the magic token in the existing registration email.
- [ ] Pre-trip portal: itinerary, registration, certs upload, rentals, briefing PDFs, dietary all work.
- [ ] On-trip portal: today's plan, schedule, weather notes, running folio.
- [ ] Post-trip portal: dive log, photos, survey, rebook CTA.
- [ ] Crew roster with role + profile; org-scoped.
- [ ] Crew certifications with expiry; reminder emails fire at 90/30/7 days.
- [ ] Equipment by serial with kind + service log; status drives readiness.
- [ ] Trip cannot move to `confirmed` if a required crew cert is expired or required equipment is out_of_service, unless overridden with a reason.
- [ ] `manta-night / spaces / living` is the default; switcher is hidden in production.
- [ ] Overview, Trips, TripDashboard, TripManifest, GuestFolio migrated to the component library; no `.admin-card`/`.admin-page-header` markup remains on those five files.
- [ ] `app.css` is ≥15 KB smaller than at the start of the sprint.
- [ ] All new endpoints have integration tests; all new pages have at minimum a render smoke.
- [ ] ADRs 0005, 0006, 0007 exist; ADR 0004 amended; DESIGN.md decisions log appended; CLAUDE.md updated.
- [ ] `go test ./...`, `go vet ./...`, `gofmt -l .` clean; `npm run build` clean.
- [ ] Deployed to the GCP VM and smoke-tested end-to-end.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Sprint is too large for one go | High | Schedule slip | Phases are ordered by dependency; each phase ships interim value. If Phase 5 (crew + equipment) is at risk, cut the readiness-gate UI for Sprint 027 — keep the data model and the cert reminder email. |
| Operators forget to record a deposit, berth never holds | Medium | Lost booking; guest confusion | The "I'll send deposit" event sends an immediate operator notification; quote lands in a Funnel inbox view marked `awaiting_deposit`; auto-reminder email at 48h if still unrecorded. |
| Public endpoints introduce abuse vectors | High | Real ops cost / spam | Rate-limit per IP + per org; rotate inquiry tokens; CAPTCHA optional and gated behind a config flag. |
| Magic-link guest auth is misissued | Medium | Cross-trip data leak | Tokens are 32-byte URL-safe random; scoped to a single `trip_guest_id`; short TTL; logged in `audit_events`. Reuse the existing `GuestSession` middleware. |
| Trip readiness gate blocks legitimate ops | Medium | Operator frustration | Override is one click + a required reason field; gate fires only on `confirmed` transition, not earlier; warning surfaces in the trip dashboard from the moment a cert enters the 90-day window. |
| Migrations conflict on multi-table foreign keys | Low | Migration failure mid-run | One migration per phase; each phase's migration is independent (later anchors don't reference earlier-anchor tables); goose's transaction wrap covers per-file atomicity. |
| 5-page UI migration drags the sprint out | Medium | Stretch goal slips | Phase 6 explicitly lists the five files; if Anchor 1 or Anchor 3 is heavier than estimated, drop migrations down to 3 (Overview + TripDashboard + GuestFolio). |
| Off-platform deposit reconciliation drifts (operator records payment that didn't land) | Medium | Operator ledger out of sync | The `quote_payments` record captures `reference` (transfer ID / cheque number); audit log entry per record + per refund; periodic operator-facing reconciliation report. |
| 39 KB app.css removal touches everything | Medium | Visual regressions | Phase 6 only removes the rules now-unused after the five migrations (everything else stays); regressions caught by manual smoke matrix. |
| Public catalog leaks data we don't want public | High | Privacy violation | Public catalog query explicitly enumerates the columns it returns (no `SELECT *`); pricing private by default per trip; `published` boolean defaults `false`. |

## Security Considerations

- **First untrusted-traffic surface in the project.** Public
  endpoints (`/api/public/orgs/:slug/trips`,
  `/api/public/orgs/:slug/inquiries`,
  `/api/public/quotes/:token`,
  `/api/public/quotes/:token/accept`) get explicit rate
  limits, structured input validation, and tight column
  enumeration on read paths. Add this rule to CLAUDE.md.
- **Magic tokens.** 32-byte URL-safe random, single-use for
  state transitions (quote → paid), reusable for read-only
  guest portal (scoped to one trip_guest). TTL configurable;
  default 7 days post-trip-end.
- **Deposit handling.** No money flows through the platform;
  every payment is an operator-confirmed offline record.
  Audit log entries for every `quote_payments` insert + every
  refund. Operator override of the readiness gate is
  separately audited.
- **PII in dive logs + photos.** Dive logs may include medical
  observations (DCS, accident). Treat as sensitive: serve only
  to authenticated guest or Org Admin; logged access.
- **Multi-tenant isolation.** Every new table has
  `organization_id`; queries that span tenants go through
  store helpers that enforce it. Public endpoints route via
  org slug → org id; no cross-tenant queries possible.
- **Override audit.** Trip readiness override requires a free-
  text reason; written to `audit_events` with `actor_id` +
  the failed checks.
- **Rate-limit knobs.** Per-IP + per-org limits configurable;
  defaults conservative (inquiry: 5/hour/IP, 100/day/org).

## Dependencies

- **Sprint 015** (folio + payment settings) — funnel deposits
  layer on top of existing `payment_settings` (currencies,
  methods).
- **Sprint 017** (audit log) — override + refund + readiness
  events all emit audit rows.
- **Sprint 023** (onboarding wizard) — the public catalog
  visibility depends on the org completing the wizard
  (specifically the `published` setting).
- **Sprint 025** (Triptych) — every new page uses the
  component library + semantic tokens.
- **External: `golang.org/x/time/rate`** — rate-limit middleware. The only new Go dependency added by this sprint.

## Open Questions

These should be raised in the Phase 4 interview.

1. **Deposit policy.** Fixed % of trip price? Fixed minimum?
   Operator-chosen per trip? Default refund window? My
   default: operator-chosen per quote (a `deposit_cents`
   field on the quote), default refund window 30 days
   pre-trip.
2. **Guest portal scope.** Ship all three phases (pre, on,
   post) in this sprint, or split (pre + on now; dive log +
   photos + survey in Sprint 027)? My default: ship all three
   per the user's locked "all three anchors" answer; cut to
   "pre + on, post deferred" only if Anchor 3 (crew/equipment)
   is at risk.
3. **Crew certifications.** Seed the table with a canonical
   list (PADI EFR, PADI Rescue, etc.) or operator-defined
   kinds only? My default: ship operator-defined `cert_kinds`
   with a small canonical seed (PADI OWSI, PADI Rescue, EFR,
   Captain's License, STCW Basic Safety).
4. **Equipment serial vs. type-quantity.** Track every BCD by
   serial, but tanks by quantity per boat? Hybrid model? My
   default: every asset is a row with a serial, EXCEPT
   `kind='tank'` which keeps a count-tracked inventory item
   per boat (Sprint 015's existing model).
5. **Photo storage in evaluation.** Reuse `DOCUMENTS_DIR`
   pattern (filesystem) or stand up a real object store
   (S3 / GCS)? My default: filesystem under `DOCUMENTS_DIR/
   trip_photos/` mirroring the existing guest-document
   pattern. Object storage is Sprint 028+.
6. **Sprint 027 preview.** Falls off into 027 by design: the
   Triptych collapse (delete losing palettes/nav/motion),
   the remaining 17 legacy-page migrations, and any Anchor
   3 scope dropped at mid-sprint.
