# Sprint 026: Open the Funnel, Portal, and Operations Spine

## Overview

Sprint 026 turns Liveaboard from "manage a charter that already
exists" into "fill the boat, serve the guest, audit the crew."
Three product anchors ship in one sprint, the Sprint 025 design
Triptych locks to production, and five hero pages migrate to
the component library:

- **Anchor 1 — Open the funnel.** A public trip catalog at
  `/charters/<org-slug>`, an inquiry form, operator-issued
  quotes, a guest acknowledgement flow, and an offline-deposit
  + offline-refund recording surface that holds a berth without
  any money moving through Liveaboard. No Stripe, no card data,
  no PCI surface — the platform records what the operator says
  happened, mirroring Sprint 015's folio-payment idiom.
- **Anchor 2 — Guest portal.** A token-authenticated guest
  surface at `/g/<token>` covering pre-trip (itinerary,
  briefing PDFs, day plan, registration handoff, equipment +
  dietary requests, certification upload) and on-trip (today's
  day plan, schedule, running folio).
- **Anchor 3 — Crew, equipment, certifications, readiness.**
  Crew roster with roles + certifications and expiry tracking;
  equipment serialized by asset with service history; a
  server-side readiness gate that blocks `POST /api/admin/
  trips/{id}/start` when required crew certs are expired or
  required equipment is out-of-service. Override is allowed
  with a reason and a structured audit row.

The Sprint 025 Triptych framework locks to `manta-night /
spaces / living` in production: `DEFAULT_MODE` is hardcoded,
the URL/localStorage design overrides are gated behind the
existing `useDevFlags().ui_redesign_switcher` dev flag, and
the floating switcher mounts only when the flag is true. The
four losing palettes, the rail/canvas nav renderers, and the
`full` motion mode stay in the tree as references for the
Sprint 027 collapse pass — they are not deleted now.

Five hero pages migrate to the component library this sprint
(Overview, Trips, TripDashboard, TripManifest, GuestFolio).
`web/src/styles/app.css` shrinks below 30 KB. The other 17
legacy pages stay on `[data-palette]`-themed legacy chrome
through Sprint 027.

This sprint is intentionally large. Posture per the interview
locks: all three anchors are required and stay in scope; if
mid-sprint pressure forces a cut, drop non-locked extras
(trip photos, surveys, dive log, post-trip rebook automation,
certification reminder emails) before touching anchor scope.

## Use Cases

1. **Public trip inquiry.** A prospective guest opens
   `/charters/coral-expeditions`, reviews published upcoming
   trips with itinerary, dates, deposit amount, cabin
   availability, and submits an inquiry (party size, date
   window, notes).
2. **Operator turns lead into quote.** Org Admin opens the
   Funnel, picks a lead, attaches it to a specific trip + cabin
   proposal, sets the deposit amount and currency, sends.
   Guest gets an email with a magic `/q/<token>` link.
3. **Guest acknowledges intent to send deposit.** Guest opens
   the link, reviews the quote, clicks "I'll send the
   deposit." Quote moves to `deposit_pending`. Operator gets
   notified.
4. **Operator records the offline deposit.** Bank transfer /
   cash / Wise lands externally. Operator opens the quote
   detail in the Funnel and records an `offline_payments`
   row with method, amount, currency, received date, and
   reference. Quote moves to `held` for the configured hold
   window (default 7 days). A `trip_guests` row is created
   so the guest can register.
5. **Guest opens the portal.** A magic-link email after deposit
   leads to `/g/<token>`. Portal shows itinerary, boat photo,
   dates, dive briefing PDFs uploaded by the operator, the
   pre-trip checklist (registration is a link to the existing
   Sprint 014 flow), and forms for rental equipment + dietary
   requests + certification uploads.
6. **Trip day arrives.** Guest sees today's day plan from the
   portal — dive sites, schedule, crew notes — alongside the
   running folio (read-only, same payload as Sprint 022's
   guest tab).
7. **Org Admin tries to start a trip with an expired cert.**
   System blocks `POST /api/admin/trips/{id}/start` with a
   structured `failures[]` array naming the crew member +
   expired cert. Operator either renews and re-attempts, or
   overrides with a written reason (≥ 20 chars) — the
   override + reason + failure list is logged to
   `audit_events`.
8. **Op Admin records a refund.** Booking cancellation in the
   refund window: operator pushes the money back through the
   original channel offline, then records an
   `offline_payments(direction='refund')` row in the quote.
   Berth is released; quote moves to `cancelled`.
9. **Production rendering is locked.** Production users see
   `manta-night / spaces / living` with no switcher and no
   way to flip themes via URL or localStorage. Dev users
   still see the switcher dock and can compare losing combos
   until Sprint 027 deletes them.
10. **Daily-driver pages now use the component library.**
    Overview, Trips, TripDashboard, TripManifest, GuestFolio
    render through `web/src/admin/components/` primitives and
    semantic tokens only — no legacy `.admin-card`,
    `.admin-page-header`, `.setup-list__*`, or `.chip--*`
    classes remain in those five files.

## Architecture

### Scope Boundaries

**In scope:**
- Public catalog, inquiry submission, quote issuance, quote
  acknowledgement, offline-deposit + offline-refund recording,
  berth-hold lifecycle.
- Guest portal pre-trip + on-trip + folio view + briefing
  documents + day plan + cert/equipment/dietary forms.
- Crew roster, crew certifications with expiry computation,
  serialized equipment with service history.
- Readiness gate on `POST /api/admin/trips/{id}/start`.
- Triptych production lock.
- Five hero page migrations.

**Out of scope (cut from earlier drafts; deferred to Sprint 027+):**
- Stripe / Stripe Connect, webhooks, checkout sessions,
  processor refunds, card storage, PCI flows.
- Adding a new `confirmed` trip status enum value (current
  `planned/active/completed/cancelled` is unchanged).
- Deleting the four losing palettes / rail+canvas nav
  renderers / `full` motion mode (stay in tree for Sprint 027).
- Migrating the other 17 legacy admin pages.
- Trip photos gallery, post-trip surveys, full dive log,
  post-trip "book next year" automation.
- Certification reminder emails (UI warnings only this sprint;
  reminder job is Sprint 027).
- New external Go dependencies. Rate limiting is a small
  token-bucket implementation built locally.
- Production deploy as a DoD step. Deploy via
  `deploy/deploy.sh` is an optional manual smoke after sprint
  close, per CLAUDE.md's "local development only for now."

### Data Model

One migration:
`internal/store/migrations/0021_sprint_026_funnel_portal_readiness.sql`.

Every operational table carries `organization_id` and every
query filters by it.

```sql
-- ===== Public funnel =====

ALTER TABLE organizations
  ADD COLUMN public_slug text NULL;
CREATE UNIQUE INDEX organizations_public_slug_idx
  ON organizations(lower(public_slug))
  WHERE public_slug IS NOT NULL;

ALTER TABLE trips
  ADD COLUMN published_at timestamptz NULL,
  ADD COLUMN public_title text NULL,
  ADD COLUMN public_summary text NULL,
  ADD COLUMN deposit_amount_cents bigint NULL
    CHECK (deposit_amount_cents IS NULL OR deposit_amount_cents >= 0),
  ADD COLUMN deposit_currency char(3) NULL,
  ADD COLUMN berth_hold_days integer NOT NULL DEFAULT 7
    CHECK (berth_hold_days BETWEEN 1 AND 30);

CREATE TABLE leads (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  trip_id uuid NULL REFERENCES trips(id) ON DELETE SET NULL,
  email text NOT NULL,
  name text NOT NULL,
  party_size integer NOT NULL CHECK (party_size BETWEEN 1 AND 50),
  date_window text NULL,
  notes text NULL,
  source_ip text NULL,
  status text NOT NULL DEFAULT 'new'
    CHECK (status IN ('new','contacted','quoted','won','lost','archived')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX leads_org_status_idx
  ON leads(organization_id, status, created_at DESC);

CREATE TABLE booking_quotes (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  lead_id uuid NULL REFERENCES leads(id) ON DELETE SET NULL,
  trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  guest_name text NOT NULL,
  guest_email text NOT NULL,
  party_size integer NOT NULL CHECK (party_size > 0),
  quoted_total_cents bigint NOT NULL CHECK (quoted_total_cents >= 0),
  deposit_due_cents bigint NOT NULL CHECK (deposit_due_cents >= 0),
  currency char(3) NOT NULL,
  status text NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','sent','accepted','deposit_pending','held','cancelled','expired')),
  accepted_at timestamptz NULL,
  hold_expires_at timestamptz NULL,
  cancelled_reason text NULL,
  created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX booking_quotes_org_trip_idx
  ON booking_quotes(organization_id, trip_id, status);

CREATE TABLE offline_payments (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  quote_id uuid NULL REFERENCES booking_quotes(id) ON DELETE SET NULL,
  trip_guest_id uuid NULL REFERENCES trip_guests(id) ON DELETE SET NULL,
  direction text NOT NULL CHECK (direction IN ('deposit','refund')),
  amount_cents bigint NOT NULL CHECK (amount_cents > 0),
  currency char(3) NOT NULL,
  method text NOT NULL
    CHECK (method IN ('bank_transfer','cash','wise','card_external','other')),
  received_on date NULL,
  reference text NULL,
  notes text NULL,
  recorded_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX offline_payments_org_quote_idx
  ON offline_payments(organization_id, quote_id, recorded_at DESC);

-- ===== Guest portal =====

CREATE TABLE trip_briefing_documents (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  title text NOT NULL,
  file_path text NOT NULL,
  content_type text NOT NULL,
  uploaded_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trip_day_plans (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  day_date date NOT NULL,
  title text NOT NULL,
  schedule_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  dive_plan_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (trip_id, day_date)
);

CREATE TABLE guest_portal_requests (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  trip_guest_id uuid NOT NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
  request_type text NOT NULL CHECK (request_type IN ('equipment','dietary')),
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (trip_guest_id, request_type)
);

-- Guest certification metadata layered on top of existing
-- Sprint 017 document storage. The doc_id references the
-- existing guest_documents table; this row just adds the
-- structured metadata operators need.
CREATE TABLE guest_certifications (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  trip_guest_id uuid NOT NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
  guest_document_id uuid NULL REFERENCES guest_documents(id) ON DELETE SET NULL,
  certification_type text NOT NULL,
  issuer text NULL,
  certification_number text NULL,
  expires_on date NULL,
  verified_at timestamptz NULL,
  verified_by_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  verification_notes text NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- ===== Crew + equipment + readiness =====

CREATE TABLE crew_members (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  name text NOT NULL,
  email text NULL,
  role text NOT NULL
    CHECK (role IN ('captain','dive_master','chef','deckhand','engineer','cruise_director','other')),
  status text NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','inactive')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE crew_certifications (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  crew_member_id uuid NOT NULL REFERENCES crew_members(id) ON DELETE CASCADE,
  certification_type text NOT NULL,
  issuer text NULL,
  certification_number text NULL,
  expires_on date NULL,
  required_for_roles text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX crew_certifications_org_expiry_idx
  ON crew_certifications(organization_id, expires_on);

CREATE TABLE equipment_assets (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  boat_id uuid NULL REFERENCES boats(id) ON DELETE SET NULL,
  asset_type text NOT NULL
    CHECK (asset_type IN ('bcd','regulator','tank','dive_computer','o2_kit','other')),
  label text NOT NULL,
  serial_number text NULL,
  status text NOT NULL DEFAULT 'in_service'
    CHECK (status IN ('in_service','out_of_service','retired')),
  service_due_on date NULL,
  required_for_dive boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX equipment_assets_org_status_idx
  ON equipment_assets(organization_id, status, service_due_on);

CREATE TABLE equipment_service_events (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  equipment_asset_id uuid NOT NULL REFERENCES equipment_assets(id) ON DELETE CASCADE,
  event_type text NOT NULL
    CHECK (event_type IN ('annual_service','vip_inspection','repair','retirement')),
  event_on date NOT NULL,
  notes text NULL,
  recorded_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trip_crew_assignments (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  crew_member_id uuid NOT NULL REFERENCES crew_members(id) ON DELETE RESTRICT,
  role text NOT NULL,
  assigned_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (trip_id, crew_member_id)
);
```

### Public Funnel Flow

```
                              ┌─────────────────────┐
   Public web ─ inquiry ─────►│ leads               │
                              │  status: new        │
                              └──────────┬──────────┘
                                         │ Op Admin
                                         │ contacts + quotes
                                         ▼
                              ┌─────────────────────┐
                              │ booking_quotes      │
                              │  status: sent       │
                              │  token_hash         │
                              └──────────┬──────────┘
                                         │ guest clicks /q/<token>
                                         │ acknowledges
                                         ▼
                              ┌─────────────────────┐
                              │ booking_quotes      │
                              │  status:            │
                              │   deposit_pending   │
                              └──────────┬──────────┘
                                         │ Op Admin records
                                         │ offline_payments
                                         │  (direction=deposit)
                                         ▼
                              ┌─────────────────────┐
                              │ booking_quotes      │
                              │  status: held       │
                              │  hold_expires_at    │
                              │                     │
                              │ trip_guests (new    │
                              │  invite row)        │
                              └─────────────────────┘
```

Public catalog endpoints (`GET /api/public/charters/:slug`,
`GET /api/public/charters/:slug/trips/:tripId`) enumerate
columns explicitly — no `SELECT *`. Pricing on the catalog is
the `deposit_amount_cents` + total — the operator chooses
which to publish.

Inquiry submission (`POST /api/public/charters/:slug/
inquiries`) goes through `internal/httpapi/rate_limit.go` —
a small in-memory token-bucket keyed on IP and on
`org_id`. Defaults: 5/hour/IP for inquiry; 100/day/org. Both
configurable via env (`LIVEABOARD_PUBLIC_INQUIRY_RATE_*`).

Quote acceptance (`POST /api/public/quotes/:token/
acknowledge-deposit`) sets `accepted_at` and moves status to
`deposit_pending`. It does NOT create a payment record. The
guest-facing copy is explicit that the deposit is sent
outside Liveaboard.

Recording a deposit (`POST /api/admin/booking-quotes/:id/
offline-payments` with `direction=deposit`) inserts an
`offline_payments` row, moves the quote to `held`, sets
`hold_expires_at = now() + trip.berth_hold_days`, and creates
a `trip_guests` invite row so the guest can register through
the existing Sprint 014 flow.

Recording a refund (`POST /api/admin/booking-quotes/:id/
offline-payments` with `direction=refund`) inserts a refund
row. When the sum of deposits minus refunds for a quote
reaches zero, the quote moves to `cancelled`, the
`trip_guests` invite is revoked, and the berth is released.

### Guest Portal Flow

The portal at `/g/:token` is a **new presentation surface**
that links into existing Sprint 014/017/022 flows. It does
NOT replace them:

- Registration is a link to the existing
  `/guest/trips/:tripGuestId/register` route.
- Document upload reuses Sprint 017 storage; the new
  `guest_certifications` table just adds structured metadata.
- Folio view reuses the existing `GuestTab` payload (Sprint
  022) without re-implementing money math.

Token resolution uses the existing `GuestSession` middleware
pattern. The token in the magic-link email expands to a
short-lived session cookie scoped to a single `trip_guest_id`.
TTL: trip end + 90 days.

Portal sections:

- **Overview**: trip, boat, dates, itinerary, registration
  status.
- **Pre-trip**: briefing PDFs (read-only download), packing
  notes, equipment rental request form, dietary form,
  certification upload form with structured metadata
  (`certification_type`, `issuer`, `certification_number`,
  `expires_on`).
- **On-trip**: today's `trip_day_plans` row (schedule + dive
  plan), the current folio snapshot.
- **Folio**: read-only view of the guest's running folio
  (same payload as the existing guest tab; no money math).
- **Post-trip**: placeholder section with a "Book another
  trip" CTA pointing back at `/charters/:slug`. No survey,
  no dive log, no rebook automation this sprint.

Invalid, expired, or cross-tenant tokens get opaque 404s.

### Readiness Gate

The readiness check fires on `POST /api/admin/trips/{id}/
start`. It runs server-side inside the transition
transaction; client readiness state from the dashboard is
advisory only.

The check computes:
- Every required `trip_crew_assignments` role is filled.
- For every assigned crew member, every cert in the role's
  `required_for_roles` set is present and unexpired on the
  trip start date.
- Every `equipment_assets` row with `required_for_dive=true`
  assigned to the trip's boat is `in_service` and not past
  `service_due_on` on the trip start date.

If any check fails, the transition returns `409 Conflict`
with a structured `failures[]` array:
```json
{
  "failures": [
    {"kind": "crew_cert_expired", "crew_member_id": "...",
     "crew_member_name": "Anna",
     "certification_type": "EFR",
     "expires_on": "2026-04-30"},
    {"kind": "equipment_out_of_service", "asset_id": "...",
     "asset_label": "BCD #4", "status": "out_of_service"}
  ]
}
```

Override path: the operator can re-submit the start
transition with an `override_reason` string (≥ 20 chars).
This writes a structured `audit_events` row with the failed
checks + the reason, then proceeds with the transition.
TripDashboard surfaces the override status (yellow chip)
until the trip completes.

Warnings (90/30/7 day cert expiry buckets) surface on
TripDashboard and the Trips list before the gate fires — so
operators know about issues during the planning window.

### Design Lock (Triptych production posture)

- `web/src/admin/design/types.ts` — `DEFAULT_MODE`
  hardcoded to `{ palette: "manta-night", layout: "spaces",
  motion: "living" }`. No change to the constant itself
  (already set); the lock is in the provider's behavior.
- `web/src/admin/design/DesignModeProvider.tsx` — URL +
  localStorage reads gated behind
  `useDevFlags().ui_redesign_switcher`. In production the
  provider always uses `DEFAULT_MODE`.
- `web/src/admin/components/TriptychSwitcher.tsx` — already
  gated; verify the dev-flag check is enforced in the
  production build (no `<aside>` rendered when flag is
  false).
- Losing palettes (`coral-surge`, `reef-carnival`,
  `abyss-flame`, `seaglass-electric`), nav renderers
  (`RailNav`, `CanvasNav`), and `full` motion mode stay in
  the tree. Sprint 027 collapses.

### Five Hero Page Migrations

Each migration:
1. Imports primitives from `web/src/admin/components/`
   (`Page`, `PageHeader`, `Card`, `Stat`, `Chip`, etc.).
2. Replaces page-local `<div className="admin-card">`
   markup with `<Card>` etc.
3. Co-locates any remaining page-specific CSS in a
   `PageName.module.css` consuming semantic tokens only.
4. After migration, `grep -E 'admin-card|admin-page-header|
   admin-page-title|setup-list__|chip--' web/src/admin/
   pages/<page>.tsx` returns nothing.

Migrated files:
- `web/src/admin/pages/Overview.tsx` (+ `.module.css`)
- `web/src/admin/pages/Trips.tsx` (+ `.module.css`)
- `web/src/admin/pages/TripDashboard.tsx` (+ `.module.css`)
- `web/src/admin/pages/TripManifest.tsx` (+ `.module.css`)
- `web/src/admin/pages/GuestFolio.tsx` (+ `.module.css`)

After Phase 5 lands, `web/src/styles/app.css` size drops
below 30 KB by removing the now-unused legacy selectors
(`.setup-list__*`, `.admin-card`-specific descendant
selectors, etc. — only the rules superseded by the
migration are removed; the rest stays for the 17
unmigrated pages).

## Implementation Plan

### Phase 1: Data foundations + guardrails (~15%)

**Files:**
- `internal/store/migrations/0021_sprint_026_funnel_portal_readiness.sql` — Create. All schema above.
- `internal/store/{leads,booking_quotes,offline_payments}.go` — Create.
- `internal/store/{guest_portal,guest_certifications,trip_day_plans,trip_briefing_documents}.go` — Create.
- `internal/store/{crew,equipment,readiness}.go` — Create.
- `internal/store/trips.go` — Modify. Add `PublishedTripsForOrg`, public trip projections.
- `internal/httpapi/rate_limit.go` — Create. In-memory token-bucket keyed on (key, bucket); `Allow(key string) bool`.

**Tasks:**
- [ ] Migration applies cleanly forward; down path drops only Sprint 026 objects.
- [ ] Every new table has `organization_id` and a corresponding store helper that enforces it.
- [ ] `booking_quotes.token_hash` is a SHA-256 of the raw token; raw token only exposed at creation/send.
- [ ] Store transitions tested: `lead.new → contacted → quoted → won/lost/archived`; `quote.draft → sent → accepted → deposit_pending → held → cancelled/expired`.
- [ ] Cross-tenant tests prove org A cannot read or mutate org B's leads, quotes, payments, crew, equipment, day plans, briefing docs, or certifications.
- [ ] `internal/httpapi/rate_limit.go` token-bucket has unit tests covering burst, refill, and concurrent access.
- [ ] No new Go dependencies.

### Phase 2: Public catalog, leads, quotes, offline deposits (~20%)

**Files:**
- `internal/httpapi/public_charter_handlers.go` — Create. Catalog, trip detail, inquiry, quote lookup, acknowledgement.
- `internal/httpapi/booking_quote_handlers.go` — Create. Admin leads, quote CRUD, send, record-payment, refund, cancel.
- `internal/httpapi/httpapi.go` — Modify. Register `/api/public/*` (rate-limited) and `/api/admin/booking-quotes/*`, `/api/admin/leads/*`.
- `internal/email/templates/booking_quote_*.tmpl` — Create. Quote-sent and deposit-acknowledged operator-notification templates.
- `web/src/pages/CharterCatalog.tsx` — Create. Public catalog page.
- `web/src/pages/CharterTripDetail.tsx` — Create. Public single-trip page with inquiry form.
- `web/src/pages/BookingQuote.tsx` — Create. Guest quote review + acknowledgement.
- `web/src/admin/pages/Funnel.tsx` — Create. Leads + quotes + recorded payments dashboard.
- `web/src/admin/pages/QuoteDetail.tsx` — Create. Edit quote, send, record deposit, refund, cancel.
- `web/src/admin/nav.ts` — Modify. Add Funnel under Operations.
- `web/src/main.tsx` — Modify. Add `/charters/:slug`, `/charters/:slug/trips/:tripId`, `/q/:token`, `/admin/funnel`, `/admin/funnel/quotes/:id`.
- `web/src/lib/api.ts` — Modify. Public + admin DTOs and calls.

**Tasks:**
- [ ] `GET /api/public/charters/:slug` returns only trips with `published_at IS NOT NULL` and end dates in the future and `status='planned'`; explicit column enumeration.
- [ ] `POST /api/public/charters/:slug/inquiries` rate-limited per-IP (5/hour) and per-org (100/day); both configurable.
- [ ] `POST /api/admin/leads/:id/quotes` creates a `booking_quotes` row, generates token, sends email.
- [ ] `GET /api/public/quotes/:token` resolves token, returns quote details.
- [ ] `POST /api/public/quotes/:token/acknowledge-deposit` flips status to `deposit_pending`, sets `accepted_at`, notifies operator.
- [ ] `POST /api/admin/booking-quotes/:id/offline-payments` (direction=deposit) flips quote to `held`, sets `hold_expires_at`, creates `trip_guests` invite.
- [ ] `POST /api/admin/booking-quotes/:id/offline-payments` (direction=refund) when net == 0 cancels the quote and revokes the invite.
- [ ] `POST /api/admin/booking-quotes/:id/cancel` releases the hold without a refund record (no money was received).
- [ ] Admin endpoints require Org Admin role; Cruise Directors get 403.
- [ ] Audit events for: lead created, quote created, quote sent, quote acknowledged, deposit recorded, refund recorded, quote cancelled, hold expired.
- [ ] All new pages use component-library primitives and semantic tokens only.
- [ ] `npm run build` clean; TypeScript clean.

### Phase 3: Guest portal — pre-trip + on-trip + folio (~20%)

**Files:**
- `internal/httpapi/guest_portal_handlers.go` — Create. Portal aggregate + section endpoints.
- `internal/httpapi/httpapi.go` — Modify. Register `/api/guest/portal/*`.
- `internal/store/guest_portal.go` — Modify. Aggregate query helpers.
- `internal/email/templates/portal_invite_*.tmpl` — Create. Magic-link email after deposit.
- `web/src/pages/GuestPortal.tsx` — Create. Portal shell with token resolution.
- `web/src/pages/guestPortal/{Overview,PreTrip,OnTrip,Folio,PostTrip}.tsx` — Create.
- `web/src/main.tsx` — Modify. Add `/g/:token` route.
- `web/src/lib/api.ts` — Modify. Portal DTOs.
- `web/src/admin/pages/TripBriefing.tsx` — Create. Operator-side briefing doc upload + day plan editor (admin-side counterpart for the data the guest portal reads).
- `web/src/admin/nav.ts` — Modify. Briefing surface accessible from TripDashboard (no new sidebar entry).

**Tasks:**
- [ ] `GET /api/guest/portal` resolves token to `trip_guest_id`, returns the aggregate (trip, boat, dates, registration status, briefing docs, day plans for active trips, folio snapshot).
- [ ] Invalid/expired/cross-tenant tokens return opaque 404.
- [ ] `PUT /api/guest/portal/requests/equipment` and `PUT /api/guest/portal/requests/dietary` upsert `guest_portal_requests` rows.
- [ ] `POST /api/guest/portal/certifications` creates a `guest_certifications` row tied to an upload via Sprint 017 document handler.
- [ ] On-trip section returns today's `trip_day_plans` row (by `now() between trip.start_date and trip.end_date`); falls back cleanly when no plan exists.
- [ ] Folio section reuses Sprint 022's `GuestTab` payload via store helper; no duplication of money math.
- [ ] Post-trip section renders a placeholder "Book another trip" CTA only.
- [ ] Pages use component library; portal links to existing `/guest/trips/:tripGuestId/register` for registration.
- [ ] Frontend smoke: `/g/<valid-token>` happy path renders; `/g/<bad-token>` returns the 404 surface.

### Phase 4: Crew, equipment, certifications, readiness gate (~20%)

**Files:**
- `internal/httpapi/crew_handlers.go` — Create. Crew member + cert CRUD.
- `internal/httpapi/equipment_handlers.go` — Create. Equipment + service event CRUD.
- `internal/httpapi/readiness_handlers.go` — Create. `GET /api/admin/trips/:id/readiness`.
- `internal/httpapi/trip_lifecycle_handlers.go` — Modify. Inject readiness check into start transition with override path.
- `internal/store/readiness.go` — Modify. The check function returns `Failures []ReadinessFailure`.
- `web/src/admin/pages/Crew.tsx` + `CrewMember.tsx` — Create.
- `web/src/admin/pages/Equipment.tsx` + `EquipmentAsset.tsx` — Create.
- `web/src/admin/components/ReadinessPanel.tsx` — Create. Embedded in TripDashboard.
- `web/src/admin/nav.ts` — Modify. Add Crew + Equipment under Operations.

**Tasks:**
- [ ] Org Admin can CRUD crew members; each has role + status.
- [ ] Crew certifications: `certification_type`, `issuer`, `certification_number`, `expires_on`, `required_for_roles` (text array of role names).
- [ ] Equipment by serial; `required_for_dive` boolean drives the gate.
- [ ] Equipment service events log; `service_due_on` advances based on the last event.
- [ ] `POST /api/admin/trips/:id/start` runs the readiness check inside the transaction; returns 409 + `failures[]` if blocked.
- [ ] Override: same endpoint with `override_reason` (≥ 20 chars, validated server-side) writes the `audit_events` row + the failure list + the reason, then proceeds.
- [ ] Cruise Directors see readiness for assigned trips (read-only); cannot mutate crew/equipment.
- [ ] TripDashboard surfaces readiness warnings before the gate fires (90/30/7 day buckets).
- [ ] Trips list shows a status pill ("ready" / "warnings" / "blocked") per trip.

### Phase 5: Triptych lock + five hero page migrations (~15%)

**Files:**
- `web/src/admin/design/DesignModeProvider.tsx` — Modify. URL/localStorage reads only when `useDevFlags().ui_redesign_switcher === true`; otherwise force `DEFAULT_MODE`.
- `web/src/admin/components/TriptychSwitcher.tsx` — Verify production mount is empty when flag false.
- `web/src/admin/pages/Overview.tsx` + `Overview.module.css` — Migrate.
- `web/src/admin/pages/Trips.tsx` + `Trips.module.css` — Migrate.
- `web/src/admin/pages/TripDashboard.tsx` + `TripDashboard.module.css` — Migrate.
- `web/src/admin/pages/TripManifest.tsx` + `TripManifest.module.css` — Migrate.
- `web/src/admin/pages/GuestFolio.tsx` + `GuestFolio.module.css` — Migrate.
- `web/src/styles/app.css` — Modify. Remove rules superseded by the five migrations; verify final size < 30 KB.
- `web/src/styles/admin.css` — Modify. Remove `[data-palette]` overrides specifically targeting the five migrated pages' legacy classes.

**Tasks:**
- [ ] `grep -E 'admin-card|admin-page-header|admin-page-title|setup-list__|chip--' web/src/admin/pages/{Overview,Trips,TripDashboard,TripManifest,GuestFolio}.tsx` returns nothing.
- [ ] No raw hex in the five migrated pages or their module CSS files.
- [ ] `wc -c web/src/styles/app.css` < 30 720.
- [ ] Production build: `npm run build` then verify the bundled CSS contains no `TriptychSwitcher` rules (tree-shaking).
- [ ] Production build: `?triptych=` query string and `localStorage["triptych"]` are ignored — render is always `manta-night/spaces/living`.
- [ ] Dev mode: switcher still mounts and selection still persists.

### Phase 6: Verification + docs + sprint close (~10%)

**Files:**
- `docs/decisions/0005-offline-deposit-funnel.md` — Create. Codifies why no Stripe; the offline-confirmation flow; the deposit/refund/cancel state machine; the audit + reconciliation expectations.
- `docs/decisions/0004-triptych-runtime-evaluation.md` — Modify. Amend with the production lock decision and the Sprint 027 collapse plan.
- `DESIGN.md` — Modify. Decisions log appended with: production lock, funnel pattern, readiness gate pattern.
- `CLAUDE.md` — Modify. New rule: "Public-facing endpoints (`/api/public/*`) MUST rate-limit and validate input. Read paths MUST enumerate columns explicitly."
- `docs/product/organization-admin-user-stories.md` — Modify. Tick the now-shipped stories (booking funnel, crew roster, equipment tracking).
- `docs/sprints/SPRINT-026.md` — This file. DoD ticked at sprint close.

**Tasks:**
- [ ] `go test ./...` clean.
- [ ] `go vet ./...` clean.
- [ ] `gofmt -l .` empty.
- [ ] `npm run build` clean.
- [ ] Prettier on touched frontend files.
- [ ] Manual smoke matrix:
  - Public catalog → inquiry → admin sees lead → quote → guest acknowledges → operator records deposit → quote held → trip_guest invite created.
  - Operator records refund (full); quote cancelled; berth released.
  - Guest opens portal → sees itinerary, briefing, day plan, requests, certs, folio.
  - Operator tries to start a trip with expired cert; gate blocks with `failures[]`.
  - Operator overrides with `override_reason`; audit row written; transition succeeds.
  - Crew/Equipment CRUD as Org Admin; readonly as Cruise Director.
  - Production build: switcher absent; URL `?triptych=...` ignored.
  - All five migrated pages render correctly under `manta-night / spaces / living`.
- [ ] Tracker synced.

## API Endpoints

| Endpoint | Method | Auth | Purpose |
|---|---:|---|---|
| `/api/public/charters/:slug` | GET | none + rate limit | Org profile + published upcoming trips |
| `/api/public/charters/:slug/trips/:tripId` | GET | none + rate limit | Single published trip |
| `/api/public/charters/:slug/inquiries` | POST | none + rate limit | Lead capture |
| `/api/public/quotes/:token` | GET | token | Guest quote review |
| `/api/public/quotes/:token/acknowledge-deposit` | POST | token | Guest acknowledges intent |
| `/api/admin/leads` | GET | Org Admin | List leads |
| `/api/admin/leads/:id` | GET/PATCH | Org Admin | Lead detail + status |
| `/api/admin/leads/:id/quotes` | POST | Org Admin | Create + send quote |
| `/api/admin/booking-quotes/:id` | GET/PATCH | Org Admin | Quote detail + edits |
| `/api/admin/booking-quotes/:id/offline-payments` | POST | Org Admin | Record deposit or refund |
| `/api/admin/booking-quotes/:id/cancel` | POST | Org Admin | Release hold w/o refund record |
| `/api/guest/portal` | GET | GuestSession | Portal aggregate |
| `/api/guest/portal/requests/equipment` | PUT | GuestSession | Upsert equipment request |
| `/api/guest/portal/requests/dietary` | PUT | GuestSession | Upsert dietary request |
| `/api/guest/portal/certifications` | POST | GuestSession | Upload + metadata |
| `/api/admin/crew` | GET/POST | Org Admin | Crew roster |
| `/api/admin/crew/:id` | GET/PATCH/DELETE | Org Admin | Crew member |
| `/api/admin/crew/:id/certifications` | GET/POST | Org Admin | Crew certs |
| `/api/admin/equipment` | GET/POST | Org Admin | Equipment list |
| `/api/admin/equipment/:id` | GET/PATCH | Org Admin | Equipment asset |
| `/api/admin/equipment/:id/service-events` | GET/POST | Org Admin | Service history |
| `/api/admin/trips/:id/readiness` | GET | Org Admin / CD | Readiness snapshot |
| `/api/admin/trips/:id/crew-assignments` | GET/POST/DELETE | Org Admin | Trip crew |
| `/api/admin/trips/:id/briefing-documents` | GET/POST | Org Admin | Briefing PDFs |
| `/api/admin/trips/:id/day-plans` | GET/PUT | Org Admin | Day plan editor |
| `/api/admin/trips/:id/start` | POST | Org Admin | Existing endpoint; now runs readiness gate with optional `override_reason` |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0021_sprint_026_funnel_portal_readiness.sql` | Create | All new tables + ALTER TABLE columns. |
| `internal/store/{leads,booking_quotes,offline_payments}.go` | Create | Funnel entities. |
| `internal/store/{guest_portal,guest_certifications,trip_day_plans,trip_briefing_documents}.go` | Create | Portal entities. |
| `internal/store/{crew,equipment,readiness}.go` | Create | Supply-side entities + readiness check. |
| `internal/store/trips.go` | Modify | Public trip projections + readiness helpers. |
| `internal/httpapi/public_charter_handlers.go` | Create | Public catalog/inquiry/quote. |
| `internal/httpapi/booking_quote_handlers.go` | Create | Admin leads/quotes/payments. |
| `internal/httpapi/guest_portal_handlers.go` | Create | Portal endpoints. |
| `internal/httpapi/{crew,equipment,readiness}_handlers.go` | Create | Crew/equipment/readiness HTTP. |
| `internal/httpapi/trip_lifecycle_handlers.go` | Modify | Readiness gate on start. |
| `internal/httpapi/rate_limit.go` | Create | Local token-bucket middleware. |
| `internal/httpapi/httpapi.go` | Modify | Register new routes. |
| `internal/email/templates/booking_quote_*.tmpl` | Create | Quote send + acknowledgement templates. |
| `internal/email/templates/portal_invite_*.tmpl` | Create | Post-deposit magic-link email. |
| `web/src/pages/CharterCatalog.tsx` | Create | Public catalog. |
| `web/src/pages/CharterTripDetail.tsx` | Create | Public trip detail + inquiry form. |
| `web/src/pages/BookingQuote.tsx` | Create | Guest quote acknowledgement. |
| `web/src/pages/GuestPortal.tsx` + `guestPortal/*` | Create | Guest portal shell + sections. |
| `web/src/admin/pages/Funnel.tsx` | Create | Leads + quotes + payments dashboard. |
| `web/src/admin/pages/QuoteDetail.tsx` | Create | Quote management. |
| `web/src/admin/pages/Crew.tsx` + `CrewMember.tsx` | Create | Crew management. |
| `web/src/admin/pages/Equipment.tsx` + `EquipmentAsset.tsx` | Create | Equipment management. |
| `web/src/admin/pages/TripBriefing.tsx` | Create | Briefing docs + day-plan editor. |
| `web/src/admin/components/ReadinessPanel.tsx` | Create | Embedded readiness UI. |
| `web/src/admin/pages/Overview.tsx` + `.module.css` | Modify | Migrate to component library. |
| `web/src/admin/pages/Trips.tsx` + `.module.css` | Modify | Migrate + readiness/lead affordances. |
| `web/src/admin/pages/TripDashboard.tsx` + `.module.css` | Modify | Migrate + readiness panel + day-plan link. |
| `web/src/admin/pages/TripManifest.tsx` + `.module.css` | Modify | Migrate + portal/cert signals. |
| `web/src/admin/pages/GuestFolio.tsx` + `.module.css` | Modify | Migrate. |
| `web/src/admin/design/DesignModeProvider.tsx` | Modify | Production gate on URL/localStorage. |
| `web/src/admin/components/TriptychSwitcher.tsx` | Modify | Verify dev-only mount. |
| `web/src/styles/app.css` | Modify | Remove superseded rules; final size < 30 KB. |
| `web/src/styles/admin.css` | Modify | Remove `[data-palette]` overrides for migrated pages. |
| `web/src/main.tsx` | Modify | New routes. |
| `web/src/lib/api.ts` | Modify | DTOs and calls for all new surfaces. |
| `web/src/admin/nav.ts` | Modify | Add Funnel, Crew, Equipment to Operations. |
| `docs/decisions/0005-offline-deposit-funnel.md` | Create | ADR. |
| `docs/decisions/0004-triptych-runtime-evaluation.md` | Modify | Production-lock amendment. |
| `DESIGN.md` | Modify | Decisions log appended. |
| `CLAUDE.md` | Modify | Public-endpoint rate-limit + validate rule. |
| `docs/product/organization-admin-user-stories.md` | Modify | Tick shipped stories. |
| `docs/sprints/SPRINT-026.md` | Create | This sprint. |

## Definition of Done

- [ ] A prospective guest can open `/charters/:slug`, see only published upcoming trips, and submit an inquiry.
- [ ] Org Admin can convert a lead to a quote and send a `/q/:token` magic-link email.
- [ ] Guest can review a quote via the magic link and acknowledge intent to send the deposit.
- [ ] Org Admin can record an offline deposit; the quote moves to `held` for the trip's `berth_hold_days`; a `trip_guests` invite is created.
- [ ] Org Admin can record an offline refund; when net == 0 the quote moves to `cancelled` and the invite is revoked.
- [ ] Hold auto-expires (server check on read) after `hold_expires_at`.
- [ ] No Stripe SDK, webhook handler, processor checkout-session endpoint, processor payment link, card token, or PCI-sensitive field is introduced.
- [ ] Guest portal at `/g/:token` shows itinerary, briefing docs, day plan (when active), equipment/dietary requests, certification upload + status, and running folio view (read-only).
- [ ] Operator can upload briefing PDFs and edit day plans on the admin side.
- [ ] Crew roster + crew certifications with `expires_on` and `required_for_roles` are CRUDable by Org Admin.
- [ ] Equipment assets with `serial_number`, `status`, `service_due_on`, and `required_for_dive` are CRUDable by Org Admin.
- [ ] Equipment service events log every annual service, VIP, repair, or retirement.
- [ ] `POST /api/admin/trips/:id/start` runs the readiness check inside the transaction; returns 409 + `failures[]` when blocked.
- [ ] Override with `override_reason` (≥ 20 chars) writes a structured `audit_events` row and proceeds.
- [ ] TripDashboard surfaces readiness warnings at 90/30/7-day cert-expiry buckets and out-of-service equipment alerts before the gate fires.
- [ ] Production build: `DesignModeProvider` ignores `?triptych=` query and `localStorage["triptych"]`; `TriptychSwitcher` does not render.
- [ ] Dev build: switcher still mounts; selection still persists.
- [ ] Overview, Trips, TripDashboard, TripManifest, GuestFolio render through `web/src/admin/components/` primitives; no legacy `.admin-card`/`.admin-page-header`/`.admin-page-title`/`.setup-list__*`/`.chip--*` markup remains in those five files; no raw hex in their `.module.css`.
- [ ] `wc -c web/src/styles/app.css` returns less than 30 720.
- [ ] Public endpoints rate-limit per IP and per org; admin endpoints require Org Admin role; cross-tenant access tests prove org A cannot read or mutate org B's rows.
- [ ] Audit events written for every state change in the funnel + every readiness override.
- [ ] ADR 0005 lands. ADR 0004 amended. DESIGN.md decisions log appended with three rows. CLAUDE.md gains the public-endpoint rule. Org admin user stories doc ticks shipped stories.
- [ ] `go test ./...`, `go vet ./...`, `gofmt -l .` clean. `npm run build` clean. Prettier clean on touched frontend files.
- [ ] No new external Go dependencies; no new external JS dependencies beyond what Sprint 025 introduced.

## Documentation Manifest

The implementation sprint MUST land the following docs changes
alongside the code. The `sprint` skill verifies each file in
this list was modified before marking the sprint complete.

### New ADRs

- `docs/decisions/0005-offline-deposit-funnel.md` — Codifies
  the offline-confirmation funnel: why no Stripe, the
  lead/quote/payment/hold state machine, the
  audit/reconciliation expectations, the rate-limit posture
  for public endpoints, and the Sprint 027 follow-up
  candidates (Stripe Connect, reminder emails, dive log,
  surveys).

### Amended ADRs

- `docs/decisions/0004-triptych-runtime-evaluation.md` — Add
  the production-lock decision: `manta-night / spaces /
  living` is the production default; URL + localStorage
  reads gated by the dev-flag; switcher dev-only;
  losers stay in the tree for Sprint 027 collapse.

### Cross-cutting docs

- `DESIGN.md` — Decisions log appended with: production
  Triptych lock; offline-deposit funnel design language;
  readiness-gate UI pattern (warnings → block → override).
- `CLAUDE.md` — Append under Development Rules: "Public-
  facing endpoints (`/api/public/*`) MUST rate-limit per IP
  and per org, validate all untrusted input, and enumerate
  columns explicitly on read paths."
- `docs/product/organization-admin-user-stories.md` — Tick
  the now-shipped stories: public catalog + inquiry, quote
  funnel, deposit/refund recording, crew roster, equipment
  tracking, certification expiry tracking, trip readiness
  gate.

### Skipped (with reasoning)

- New ADR for guest portal token auth — skipped because the
  portal reuses the existing `GuestSession` middleware from
  Sprint 014 unchanged; no new architectural decision.
- New ADR for the readiness gate — skipped because the gate
  is feature work fully captured in this sprint doc; no
  cross-sprint architectural decision.
- `docs/CONFIG.md` — no new config keys land this sprint
  beyond the two `LIVEABOARD_PUBLIC_INQUIRY_RATE_*` knobs,
  which are documented inline at the rate-limit handler;
  CONFIG.md will catch up in Sprint 027 if more knobs land.
- `docs/sprints/README.md` — sprint workflow itself
  unchanged.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Three-anchor scope overflows | High | Schedule slip / cut anchor | Locks forbid cutting anchors. Cut order if pressure appears: post-trip rebook automation (already placeholder) → guest cert verification UI → briefing-docs upload UI → day-plan rich editor → trip photos (already deferred). Anchor scope itself is non-negotiable. |
| Operator forgets to record a deposit; berth never holds | Medium | Lost booking; guest confused | Quote acknowledgement triggers an operator notification email; Funnel dashboard surfaces `awaiting_deposit` quotes prominently; 48h reminder email to operator. |
| Off-platform deposit reconciliation drifts | Medium | Operator ledger out of sync | `offline_payments.reference` captures the transfer ID / cheque number; audit log entry per record + per refund; per-quote `recorded` vs `received` line items visible on Funnel. |
| Readiness gate blocks legitimate ops | Medium | Operator frustration | Override is one click + a required reason ≥ 20 chars; gate fires only on `start`, not `planned`; warnings surface from 90 days out so operators have time to resolve. |
| Override is overused, becomes a rubber stamp | Medium | Erodes the gate's safety value | Override writes structured audit row with failure list + reason; TripDashboard surfaces override status (yellow chip) until trip completes; Reports view aggregates override usage per org. |
| Public endpoints introduce abuse | High | Real ops cost | Rate-limit per IP + per org; configurable; tight column enumeration on read paths; no `SELECT *`; structured input validation. |
| Magic-token cross-tenant leak | Medium | Privacy violation | Tokens are 32-byte URL-safe random, stored hashed (SHA-256), single-scoped to a `trip_guest_id`; opaque 404 for foreign tokens; reuses `GuestSession` pattern that already does this. |
| Migration touches an existing schema we didn't anticipate | Low | Migration failure mid-run | The migration only `ALTER`s `organizations` (one nullable column) and `trips` (five nullable columns + one defaulted); every other change is `CREATE TABLE`; goose wraps in a transaction. |
| 5-page UI migration drags out | Medium | Stretch goal slips | Page list is exactly 5; if needed, reduce to 3 (Overview + TripDashboard + GuestFolio) and document the cut. DoD requires the full 5, so the cut is a sprint failure mode acknowledged here. |
| `app.css` size target missed | Low | DoD failure | Phase 5 removes rules only for the five migrated pages' legacy classes; the remaining 17 pages' rules stay. Target < 30 KB is achievable from current 39 KB by removing the ~12 KB of setup-list / admin-card descendant rules tied to the migrated pages. |
| Triptych production-lock gate is incorrectly placed | Medium | Production users see switcher | Phase 5 task explicitly verifies the production build via `grep -c TriptychSwitcher dist/assets/*.js` returns 0 (tree-shaken); CSS bundle does not include `.dock` class. |
| Operators expect Stripe-style instant confirmation | Low | UX confusion | Quote acknowledgement copy is explicit: "Your operator will confirm by email when they receive the deposit." Hold timer visible in the portal. |

## Security Considerations

- **First untrusted-traffic surface.** Public endpoints
  (`/api/public/charters/*`, `/api/public/quotes/*`) get
  explicit per-IP and per-org rate limits, structured input
  validation, and tight column enumeration on read paths.
  CLAUDE.md updated with this rule.
- **No money flows through the platform.** Every payment is
  an operator-confirmed offline record. No card data, no
  processor tokens, no bank credentials, no PCI surface.
  `offline_payments.reference` is free text (transfer ID
  number, cheque #) — no structured banking data.
- **Audit log entries** for every state change in the
  funnel (lead created, quote sent, quote acknowledged,
  deposit recorded, refund recorded, quote cancelled, hold
  expired) and every readiness override (with failure list +
  reason).
- **Magic tokens** are 32-byte URL-safe random, stored
  hashed (SHA-256) in `booking_quotes.token_hash`; raw
  token only returned at creation/send; single-scoped to a
  `booking_quotes.id`. Portal tokens reuse the same scheme,
  scoped to `trip_guests.id`.
- **Multi-tenant isolation** on every new table via
  `organization_id` + store-layer enforcement. Cross-tenant
  tests prove org A cannot read or mutate org B's leads,
  quotes, payments, crew, equipment, certs, day plans,
  briefing docs, or portal data.
- **PII in briefing docs + certifications.** Briefing docs
  are operator-uploaded and may include guest manifest
  info; access scoped to the trip's guests + Org Admin +
  assigned Cruise Director. Certifications can include
  certificate numbers; treated as sensitive, served only to
  the authenticated guest or Org Admin; access logged.
- **Override authorization.** The readiness override path is
  Org-Admin-only; Cruise Directors cannot bypass the gate.
- **Rate-limit middleware.** Local in-memory token bucket;
  resets on server restart (acceptable for evaluation; a
  Redis-backed limiter is Sprint 028+ if multi-instance
  serving is needed).

## Dependencies

- **Sprint 014** (guest invitations + registration) — the
  portal links into the existing registration flow; tokens
  and sessions reuse `GuestSession`.
- **Sprint 015** (folio + payment settings) — `offline_
  payments` mirrors the existing folio-payment idiom; no
  reuse, but the conceptual pattern matches.
- **Sprint 017** (audit log + document storage) — readiness
  overrides + funnel events emit audit rows; guest cert
  uploads reuse `guest_documents`.
- **Sprint 018** (trip lifecycle) — readiness gate hooks
  into the existing `start` transition.
- **Sprint 022** (reports + guest tab) — portal folio view
  reuses the guest tab payload.
- **Sprint 025** (Triptych framework + component library) —
  every new page uses the library; production lock retires
  the switcher.

No new external Go dependencies. No new external JS
dependencies.

## Open Questions

These can be resolved during implementation without changing
the sprint's commitments:

1. **Deposit policy default.** Operator chooses
   `deposit_amount_cents` per trip (the schema supports this
   via `trips.deposit_amount_cents`). If null, the quote
   captures the deposit amount per-quote. No global default.
2. **Briefing doc storage path.** Reuse
   `DOCUMENTS_DIR/<org>/briefings/<trip>/<doc>` mirroring
   the Sprint 017 guest-documents pattern. Object storage
   is Sprint 028+.
3. **Day plan editor surface.** Sprint 026 ships a minimal
   form (title + JSON-backed schedule + dive-plan list);
   rich editor (drag-and-drop sites, time-slot blocks) is
   Sprint 027+.
4. **Crew certification seed list.** Operator-defined kinds
   only this sprint; a small canonical seed (PADI OWSI,
   PADI Rescue, EFR, Captain's License, STCW Basic Safety)
   ships if Phase 4 has bandwidth.
5. **Operator notification channels.** Email this sprint
   (lead created, quote acknowledged, deposit awaiting
   confirmation); SMS / Slack / Discord is Sprint 028+.

## References

- `docs/sprints/drafts/SPRINT-026-INTENT.md`
- `docs/sprints/drafts/SPRINT-026-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-026-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-026-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-026-MERGE-NOTES.md`
- `docs/sprints/SPRINT-014.md` (guest invitations)
- `docs/sprints/SPRINT-015.md` (folio + payments)
- `docs/sprints/SPRINT-017.md` (documents + audit)
- `docs/sprints/SPRINT-018.md` (trip lifecycle)
- `docs/sprints/SPRINT-022.md` (reports + guest tab)
- `docs/sprints/SPRINT-025.md` (Triptych framework)
- `docs/decisions/0004-triptych-runtime-evaluation.md`
- `DESIGN.md`
- `CLAUDE.md`
