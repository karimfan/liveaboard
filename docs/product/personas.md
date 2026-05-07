# Personas

This document defines the user personas for the Liveaboard SaaS platform, what each persona owns, and the boundaries between them. It is the source of truth for persona-related scope decisions.

When a story or feature could plausibly belong to two personas, the boundary tables below decide. If a real case appears that the boundaries do not cover, update this document before adding the story.

## Persona Summary

| Persona | Scope | Primary use |
|---|---|---|
| Organization Admin | Org-wide | Configure the org and oversee operations: fleet, catalog, pricing, trip planning, user management, reporting and financial oversight. |
| Cruise Director | Single trip | Run one trip end-to-end: manifest, consumption, onboard ops. |
| Guest | Self only | View own tab, dive schedule, trip details. (Future.) |

---

## Organization Admin

**Scope:** Org-wide configuration, planning, and oversight.

**Owns:**
- Organization profile and defaults (name, currency).
- Fleet: boats (name, image, source linkage). Cabin layouts are not modeled; capacity is a single number per boat.
- Catalog: items, categories, USD pricing, checkout currency defaults, and sellable services/fees.
- Per-boat inventory: how many counted catalog items each boat carries (quantity tracking).
- Trip planning: create trip shell, set dates, assign Cruise Director, cancel planned trips.
- Pre-departure manifest preparation: initial guest list before the trip starts.
- User management (MVP subset): invite Cruise Directors, deactivate users, assign trip leadership.
- Reporting and oversight: setup completeness, operational trip status, revenue summaries, cross-trip analytics, financial reports.

**Does not own:**
- Starting or completing active trips (Cruise Director).
- Mid-trip manifest changes — adding/removing/reassigning guests once a trip is `active` (Cruise Director).
- Recording guest consumption / ledger entries (Cruise Director).
- Billing and org-deletion controls — deferred (post-MVP).
- Advanced role administration (multi-admin, custom roles) — deferred.

**Boundary with Cruise Director:** Org Admin owns the trip until it transitions to `active`. At that point, manifest mutations and lifecycle transitions move to the Cruise Director. Org Admin retains read access throughout and may cancel only `planned` trips.

---

## Cruise Director

**Scope:** A single trip, while it is `planned` (post-assignment), `active`, or `completed` (read-only post-completion).

**Owns:**
- Their own profile: full name, contact phone (free-text). Editable from `/admin/account`.
- Mid-trip manifest operations: add/remove/reassign guests once the trip is `active`.
- Trip lifecycle transitions: start (`planned` → `active`), complete (`active` → `completed`).
- Guest consumption / ledger entries.
- Onboard operational coordination for the duration of the trip.

**Does not own:**
- Boat configuration (Org Admin).
- Catalog/pricing (Org Admin).
- Per-boat inventory configuration (Org Admin).
- Cross-trip or org-wide reporting (Org Admin).
- User invitation or role management (Org Admin).

**Boundary with Org Admin:** Cruise Director takes the trip from a configured shell with an initial manifest and runs it. They cannot create trips, add boats, or change catalog items.

---

## Guest (Future)

**Scope:** Self only.

**Owns:**
- Read access to their own tab, dive schedule, and trip details.

**Does not own:**
- Anything else. No org, trip, or other-guest visibility.

Out of scope for the initial product. Captured here so the data model accounts for it.

---

## Cross-Persona Decision Table

For features that could belong to multiple personas, the table below records the resolution. Add a row when a real case forces a decision.

| Capability | Persona | Notes |
|---|---|---|
| Create trip shell | Org Admin | Date/boat/name. Trip starts in `planned`. |
| Assign Cruise Director to trip | Org Admin | Required before trip can be started. |
| Pre-departure manifest (planned trips) | Org Admin | Initial guest list. |
| Mid-trip manifest changes | Cruise Director | Add/remove/reassign once trip is `active`. |
| Start trip (`planned` → `active`) | Cruise Director | Org Admin cannot. |
| Complete trip (`active` → `completed`) | Cruise Director | Org Admin cannot. |
| Cancel trip (`planned` only) | Org Admin | Cruise Director cannot. |
| Catalog item add/edit/deactivate | Org Admin | Prices are canonical in USD; checkout may quote another currency. |
| Per-boat stock setup and adjustment | Org Admin | Cruise Director folio entries can later decrement stock automatically. |
| Record consumption / ledger entries | Cruise Director | |
| Invite Cruise Director | Org Admin | MVP user-mgmt subset. |
| Deactivate user | Org Admin | MVP user-mgmt subset. |
| Setup completeness dashboard | Org Admin | What is misconfigured. |
| Operational trip status view | Org Admin | Across trips. |
| Revenue summary per trip | Org Admin | `Should` priority. |
| Cross-trip analytics | Org Admin | Deferred (post-MVP). |

---

## Out of Scope (All Personas, MVP)

- Guest self-service portal.
- Billing and org-deletion controls (post-MVP).
- Multi-admin and custom-role administration.
- Cross-organization visibility of any kind.
- Offline / sync.
