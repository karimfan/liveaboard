# Sprint 026 Intent: Go Big — Lock the Design, Open the Funnel

## Seed

> take a look at what's been built and designed and redesign the
> UI if you think so and add whatever features you have in mind.
> Go big or go home. I want this to be the greatest liveaboard
> management tool there is.

## What "go big" means honestly

This seed is open-ended. To turn it into a sprint that ships
something real, we have to pick a posture. The three honest
options:

- **A. Polish path.** Finish the Sprint 025 Phase 5 migration
  (22 pages still on legacy markup), delete `app.css`, lock
  `manta-night / spaces / living` as production. Visible
  payoff: zero. Foundation payoff: huge — every future
  feature ships cleaner and faster.
- **B. Feature wave.** Skip the migration, pile on 4-5 hero
  features (guest portal, booking funnel, refunds, accounting
  export, equipment/cert tracking). Visible payoff: enormous.
  Foundation payoff: negative — we'd ship new pages on the
  same legacy chrome we already decided to retire.
- **C. Two-deck sprint.** Pick the **2-3 highest-leverage
  capabilities** that move the platform from "internal ops
  tool" to "tool the business runs on," and bundle each
  capability's pages onto the new component library. Use the
  feature work as the migration vehicle. Ship interim value
  per phase.

Sprint 026 takes posture **C**. The seed's "greatest" framing
implies the feature gap matters; the seed's "go big" framing
forbids "let me just clean up CSS." Posture C respects both.

## The chosen anchors

Three anchors. Together they take the platform from "I can run
a charter that was already booked" to "I can fill the boat,
serve the guest, and close the books." Each anchor is its own
end-to-end vertical (data → API → UI) and each ships its pages
on the new component library — that's how the migration gets
done as a side-effect.

### Anchor 1 — Open the funnel: Public booking + deposits

Today the platform has no way for a guest to *book* a trip.
Operators do their funnel in spreadsheets, Instagram DMs, or a
separate website, then manually create trips and add guests.
Sprint 026 ships:

- Public-facing **trip catalog** (`/charters/...`) listing each
  org's published upcoming trips with itinerary, dates, price,
  cabin availability.
- **Inquiry form** that captures a lead (email, party size,
  date window, notes) — flows into a new `leads` table.
- **Quote → deposit → berth hold** flow: an Org Admin sends a
  quote, the prospective guest pays a deposit via a payment
  link, the berth is held for N days while the rest of the
  registration completes.
- **Deposit refund/cancel** flow on the operator side.

This is the funnel. Without it, the platform is a back office;
with it, the platform is a business in a box.

### Anchor 2 — Guest experience: Pre-trip + on-trip portal

Today the guest gets a registration email, fills a form, then
disappears until the trip starts. Sprint 026 ships a real
**guest portal** at `/g/<trip-token>/...`:

- **Pre-trip**: their itinerary, dive plan (operator-uploaded
  briefing PDFs + day-by-day plan), packing list, equipment
  rental requests, dietary, certifications upload + verification.
- **On-trip**: today's dive sites, schedule, weather, the
  guest's running folio (no surprises at checkout).
- **Post-trip**: digital dive log (the operator can pre-fill,
  the guest can amend), a photo gallery (operator-shared), a
  one-click "book next year" CTA, post-trip survey.

This is the differentiator. Operators who feel competitive
pressure from luxury cruise lines need a guest experience that
matches.

### Anchor 3 — Crew, equipment, certs (the supply side)

Today the platform invites "Cruise Directors" but knows nothing
else about the crew, has no concept of equipment-by-serial, and
no concept of certification expiry. For a regulated industry
where a single expired CPR cert can void insurance, that's a
hole. Sprint 026 ships:

- **Crew roster** with roles (Captain, Dive Master, Chef,
  Deckhand, …). Org Admin invites; each crew member has a
  profile.
- **Certifications** per crew member (CPR, EFR, Dive Master,
  Captain's License, etc.) with expiry dates + reminder
  triggers (90/30/7 days).
- **Equipment** by serial: BCDs, regulators, tanks (with VIP
  expiry), dive computers, O2 kits. Each asset has a service-
  history log (annual service, repairs, retirement).
- **Trip readiness** check: a trip cannot move from `planned`
  to `confirmed` if any required crew cert is expired or any
  required equipment is out of service.

This is the operations spine. Without it, the platform can't
claim to be the "greatest" — it's just a checkout terminal.

## Plus: Phase 5 migration done as we go

Every new page in the three anchors ships on the component
library (`web/src/admin/components/`) and uses semantic tokens
only. As a stretch goal, we migrate the **5 highest-traffic
existing pages** (Overview, Trips, TripDashboard, TripManifest,
GuestFolio) onto the same primitives. The rest stay on legacy
chrome with `[data-palette]` overrides through Sprint 027.

When the dust settles, `app.css` still exists but is
measurably smaller, and the new chrome is the obvious template
for the rest of the migration.

## Locking the Triptych

Per ADR 0004, the Triptych switcher is a temporary evaluation
affordance. The user has picked: `manta-night / spaces / living`.
Sprint 026 hardcodes the default and removes the switcher from
production builds, but keeps the framework in place so a future
season can re-explore. Specifically:

- `DEFAULT_MODE` remains in `DesignModeProvider`, but the
  provider stops listening to URL/localStorage in production
  (gated by the same dev-flag).
- `TriptychSwitcher` only mounts in dev mode (already the
  case; tighten the gate).
- The other four palettes + the rail/canvas nav components +
  the `full` motion mode stay in the codebase as references.
  Deleting them is Sprint 027's lap.

## Context (Phase 1 orientation in 6 lines)

1. Backend ships ~80% of charter ops; missing booking
   funnel, deposits/refunds, equipment/serial, certs/expiry,
   SOP/incident, accounting export, comms, marketing CRM.
2. Sprint 025 landed the Triptych framework + component
   library; Phase 5 (page migration) was deferred. 22/25
   pages still legacy. `app.css` is 39 KB on disk.
3. `manta-night` palette chosen with the original Sprint 011
   body composite. Bold typography + gradient pill buttons +
   backdrop-blur cards already styled in `admin.css`.
4. Personas (`docs/product/personas.md`) + Org Admin stories
   (`docs/product/organization-admin-user-stories.md`) frame
   the product backlog.
5. CLAUDE.md mandates tests/gofmt/vet/prettier per commit;
   focused commits to `main`; multi-tenant org isolation;
   semantic-token-only chrome.
6. Cloud deploy is for evaluation only; the platform itself
   targets local dev / single-org. The public booking funnel
   in Anchor 1 is the first surface that meets the outside
   world and needs a real auth/rate-limit posture.

## Constraints

- All Anchor 1 (public funnel) endpoints must rate-limit and
  validate input — they receive untrusted traffic for the
  first time in the project.
- Multi-tenant isolation on every new table (`organization_id`
  + RLS or equivalent at the query layer).
- Every new page uses the component library + semantic tokens.
  No new raw hex. No new legacy class strings.
- Deposit handling: real money, even in evaluation. Treat as
  a security-sensitive surface. Idempotent webhook handlers.
- Tests required per CLAUDE.md. Backend integration tests for
  every new endpoint; frontend smoke for every new page.

## Success Criteria

A guest can land on `/charters/<org-slug>`, submit an inquiry,
receive a quote, pay a deposit, get a berth held, complete the
guest portal pre-trip flow, and at trip start see today's dive
plan in their portal. The Org Admin can manage the crew
roster, see which trip is at-risk due to an expired
certification, and refund a deposit.

The 5 highest-traffic existing pages render on the new
component library; `app.css` is smaller than 30 KB; the
TriptychSwitcher does not appear in a production build.

## Interview Locks (resolved 2026-05-31)

1. **All three anchors, one sprint.** Posture C confirmed.
2. **Skip Stripe / payments processor entirely.** Anchor 1
   reshapes: public catalog → inquiry → operator-issued quote
   → guest acknowledges via a link → **operator manually
   records deposit confirmation** (offline — bank transfer,
   cash, Wise, whatever) → berth held. Refunds are operator-
   recorded too. No money flows through the platform; no
   webhook, no Stripe SDK, no PCI surface. Quote acceptance
   semantics need to be simple and clear: the guest clicks
   "I will send deposit," the operator gets notified, the
   operator marks "received" when it lands, the berth flips
   to `held`. A `payments` table still exists — it records
   operator-confirmed offline payments, mirroring the
   existing folio payment-method pattern. This shrinks
   Anchor 1's scope by ~40% and removes the project's first
   real money/PCI risk.
3. **Lock Triptych now; delete losers in Sprint 027.**
   `manta-night / spaces / living` is the hardcoded production
   default. Switcher is dev-only. Other palettes / nav
   renderers / `full` motion stay in the tree as references
   for the Sprint 027 collapse pass.
4. **Migrate 5 hero pages this sprint.** Overview, Trips,
   TripDashboard, TripManifest, GuestFolio. The other 17
   legacy pages stay for Sprint 027.

## Open Questions

1. **Payment processor.** Stripe is the obvious pick (link
   support, webhooks, refunds, multi-currency). Does the
   user agree, or pick a different processor? Stripe Connect
   for multi-tenant payouts vs single Stripe account?
2. **Deposit semantics.** Fixed % of trip price? Fixed
   minimum? Operator-chosen per trip? Refund window policy
   (refundable vs non-refundable vs sliding scale)?
3. **Public trip catalog visibility.** Every org gets a public
   page at `/charters/<slug>`? Or operator opt-in per trip
   (a `published` boolean on the trip)?
4. **Guest portal auth.** Magic-link via email is simplest
   and matches the existing guest registration pattern. OK,
   or do we need passwords? OAuth? Per-trip token URL only?
5. **Equipment scope.** Track every fin, or just high-value
   gear (BCDs, regs, tanks, computers, O2)? Per-serial vs
   per-asset-type with quantity?
6. **Certification authorities.** PADI, SSI, NAUI, RAID,
   etc. — pick a canonical list to seed, vs let the operator
   define their own?
7. **Stretch scope: which 5 legacy pages migrate first?**
   Recommend Overview, Trips, TripDashboard, TripManifest,
   GuestFolio — the daily-driver pages. Confirm?
8. **Scope safety valve.** If at mid-sprint we discover the
   booking-funnel + Stripe integration is too large alone,
   does the user want to drop Anchor 2 or Anchor 3 to ship
   Anchor 1 cleanly, or split the sprint into 026A/026B?
