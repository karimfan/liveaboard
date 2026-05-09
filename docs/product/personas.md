# Personas

This document defines the user personas for the Liveaboard SaaS platform, what each persona owns, and the boundaries between them. It is the source of truth for persona-related scope decisions.

When a story or feature could plausibly belong to two personas, the boundary tables below decide. If a real case appears that the boundaries do not cover, update this document before adding the story.

## Persona Summary

| Persona | Scope | Primary use |
|---|---|---|
| Organization Admin | Org-wide | Configure the org and oversee operations: fleet, catalog, pricing, trip planning, user management, reporting and financial oversight. |
| Cruise Director | Single trip | Run one trip end-to-end: manifest, consumption, onboard ops. |
| Guest | Self only | Accept a trip invite and complete their own trip registration. Future scope includes tab, dive schedule, and trip details. |

---

## Organization Admin

**Scope:** Org-wide configuration, planning, and oversight.

**Owns:**
- Organization profile and defaults (name, currency).
- Payment settings: supported checkout currencies, offline payment
  methods, card transaction fee percentage, and folio email footer.
- Fleet: boats (name, image, source linkage) and reusable cabin layouts
  with cabin/berth slots.
- Catalog: items, categories, USD pricing, checkout currency defaults, and sellable services/fees.
- Per-boat inventory: how many counted catalog items each boat carries (quantity tracking).
- Trip planning: create trip shell, set dates, assign Cruise Director, cancel planned trips.
- Pre-departure manifest preparation: initial guest list before the trip starts.
- Guest registration readiness: invite guests, resend/revoke registration links, and review submitted registration details.
- Guest document readiness: review, upload on behalf of guests, download,
  and archive trip registration documents.
- Cabin and berth assignments for any organization trip.
- User management (MVP subset): invite Cruise Directors, deactivate users, assign trip leadership.
- Audit visibility across the organization for operational accountability.
- Reporting and oversight: setup completeness, operational trip status, revenue summaries, cross-trip analytics, financial reports.

**Does not own:**
- Starting or completing active trips (Cruise Director).
- Mid-trip manifest changes — adding/revoking/reassigning guests once a trip is `active` (Cruise Director).
- Recording guest consumption / ledger entries (Cruise Director).
- Billing and org-deletion controls — deferred (post-MVP).
- Advanced role administration (multi-admin, custom roles) — deferred.

**Boundary with Cruise Director:** Org Admin owns the trip until it transitions to `active`. At that point, manifest mutations and lifecycle transitions move to the Cruise Director. Org Admin retains read access throughout and may cancel only `planned` trips.

---

## Cruise Director

**Scope:** A single trip, while it is `planned` (post-assignment), `active`, or `completed` (read-only post-completion).

**Owns:**
- Their own profile: full name, contact phone (free-text). Editable from `/admin/account`.
- Mid-trip manifest operations: add/revoke/reassign guests once the trip is `active`.
- Guest registration readiness for assigned trips: invite guests, resend/revoke registration links, and review submitted registration details.
- Guest document readiness for assigned trips: review, upload, download,
  and archive documents from the guest profile.
- Cabin layout and berth assignments for boats/trips assigned to them.
- Assigned-trip guest checkout: review/correct purchased items, add an
  optional crew-tip line when requested, close the one end-of-trip
  folio as paid, and resend the folio email.
- Trip lifecycle transitions: start (`planned` → `active`), complete (`active` → `completed`).
- Guest consumption / ledger entries.
- Onboard operational coordination for the duration of the trip.

**Does not own:**
- Boat creation/source linkage (Org Admin).
- Catalog/pricing (Org Admin).
- Per-boat inventory configuration (Org Admin).
- Payment settings and card fee waivers (Org Admin; Directors cannot
  waive configured card fees).
- Cross-trip or org-wide reporting (Org Admin).
- User invitation or role management (Org Admin).

**Boundary with Org Admin:** Cruise Director takes the trip from a configured shell with an initial manifest and runs it. They cannot create trips, add boats, or change catalog items, but they can adjust cabin layouts and berth assignments for boats tied to their assigned trips.

---

## Guest

**Scope:** Self only.

**Owns:**
- Accepting a trip-specific registration invitation.
- Creating or reusing a guest account with email and password.
- Saving draft trip registration and returning later to complete it.
- Submitting their own identity, travel, emergency contact, diving, dietary/allergy, rental gear, and notes information.
- Uploading their own trip registration documents before the trip starts.
- Future: read access to their own tab, dive schedule, and trip details.

**Does not own:**
- Anything else. No org, trip, or other-guest visibility.

Guest access is not the admin chrome and does not expose organization data, other trips, or other guests.

---

## Cross-Persona Decision Table

For features that could belong to multiple personas, the table below records the resolution. Add a row when a real case forces a decision.

| Capability | Persona | Notes |
|---|---|---|
| Create trip shell | Org Admin | Date/boat/name. Trip starts in `planned`. |
| Assign Cruise Director to trip | Org Admin | Required before trip can be started. |
| Pre-departure manifest (planned trips) | Org Admin | Initial guest list. |
| Guest registration invite | Org Admin or assigned Cruise Director | Trip-scoped registration link; Cruise Directors only for assigned trips. |
| Cabin layout management | Org Admin or assigned Cruise Director | Admin org-wide; Cruise Director only for boats tied to assigned trips. |
| Guest berth assignment | Org Admin or assigned Cruise Director | Required when binding a guest to a trip; can be changed later. |
| Guest registration draft/submit | Guest | Guests can save and return later before final submission. |
| Guest document upload | Guest | Guests upload their own documents during trip registration. |
| Guest document review/manage | Org Admin or assigned Cruise Director | Staff manage documents from the per-guest profile. |
| Submitted registration review | Org Admin or assigned Cruise Director | Registration detail is fetched explicitly for the relevant trip. |
| Mid-trip manifest changes | Cruise Director | Add/revoke/reassign once trip is `active`; guest records are retained. |
| Start trip (`planned` → `active`) | Cruise Director; Org Admin emergency override | Admin override requires a reason and audit record. |
| Complete trip (`active` → `completed`) | Cruise Director; Org Admin emergency override | Open folios warn but do not block completion. |
| Cancel trip (`planned` only) | Org Admin | Cruise Director cannot. |
| Catalog item add/edit/deactivate | Org Admin | Prices are canonical in USD; checkout may quote another currency. |
| Per-boat stock setup and adjustment | Org Admin | Cruise Director folio entries can later decrement stock automatically. |
| Payment settings | Org Admin | Supported currencies, payment methods, card fee, folio email footer. |
| Record consumption / ledger entries | Cruise Director | |
| Guest checkout / close folio | Cruise Director | Assigned trips only; one end-of-trip folio per guest/trip. |
| Operational audit log | Org Admin or assigned Cruise Director | Admin sees org-wide events; Cruise Directors see assigned-trip events. |
| Invite Cruise Director | Org Admin | MVP user-mgmt subset. |
| Deactivate user | Org Admin | MVP user-mgmt subset. |
| Setup completeness dashboard | Org Admin | What is misconfigured. |
| Operational trip status view | Org Admin | Across trips. |
| Revenue summary per trip | Org Admin | `Should` priority. |
| Cross-trip analytics | Org Admin | Deferred (post-MVP). |

---

## Out of Scope (All Personas, MVP)

- Full guest self-service portal beyond trip registration.
- Billing and org-deletion controls (post-MVP).
- Multi-admin and custom-role administration.
- Cross-organization visibility of any kind.
- Offline / sync.
