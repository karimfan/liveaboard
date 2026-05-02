# Personas

This document defines the user personas for the Liveaboard SaaS platform, what each persona owns, and the boundaries between them. It is the source of truth for persona-related scope decisions.

When a story or feature could plausibly belong to two personas, the boundary tables below decide. If a real case appears that the boundaries do not cover, update this document before adding the story.

## Persona Summary

| Persona | Scope | Primary use |
|---|---|---|
| Organization Owner | Org-wide | Read-only oversight: financial, operational, reporting. |
| Organization Admin | Org-wide | Configure the org: fleet, catalog, pricing, trip planning, user management. |
| Site Director | Single trip | Run one trip end-to-end: manifest, consumption, onboard ops. |
| Guest | Self only | View own tab, dive schedule, trip details. (Future.) |

---

## Organization Owner

**Scope:** All boats, trips, users, catalog, ledger, and reports within their organization.

**Owns:**
- Full read access to financial reporting and analytics across the org.
- Org-level configuration that requires owner-tier authority (billing, org deletion — out of scope for MVP).

**Does not own:**
- Day-to-day fleet/trip/catalog configuration (Org Admin).
- Trip operations or guest manifest (Site Director).

**Boundary with Org Admin:** Owner is read-only at the operational layer. Owner-tier mutations (billing, org deletion) are deferred beyond MVP.

---

## Organization Admin

**Scope:** Org-wide configuration and planning.

**Owns:**
- Organization profile and defaults (name, currency).
- Fleet: boats and cabin layouts.
- Catalog: items, categories, pricing.
- Trip planning: create trip shell, set dates, assign Site Director, cancel planned trips.
- Pre-departure manifest preparation: initial guest list and cabin assignments before the trip starts.
- User management (MVP subset): invite Site Directors, deactivate users, assign trip leadership.
- Oversight: setup completeness, operational trip status, revenue summaries.

**Does not own:**
- Starting or completing active trips (Site Director).
- Mid-trip manifest changes — adding/removing/reassigning guests once a trip is `active` (Site Director).
- Recording guest consumption / ledger entries (Site Director).
- Owner-tier financial controls (Owner).
- Advanced role administration (multi-admin, custom roles) — deferred.

**Boundary with Site Director:** Org Admin owns the trip until it transitions to `active`. At that point, manifest mutations and lifecycle transitions move to the Site Director. Org Admin retains read access throughout and may cancel only `planned` trips.

---

## Site Director

**Scope:** A single trip, while it is `planned` (post-assignment), `active`, or `completed` (read-only post-completion).

**Owns:**
- Mid-trip manifest operations: add/remove/reassign guests once the trip is `active`.
- Trip lifecycle transitions: start (`planned` → `active`), complete (`active` → `completed`).
- Guest consumption / ledger entries.
- Onboard operational coordination for the duration of the trip.

**Does not own:**
- Boat/cabin configuration (Org Admin).
- Catalog/pricing (Org Admin).
- Cross-trip or org-wide reporting (Owner, Org Admin).
- User invitation or role management (Org Admin).

**Boundary with Org Admin:** Site Director takes the trip from a configured shell with an initial manifest and runs it. They cannot create trips, add boats, or change catalog items.

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

| Capability | Owner persona | Notes |
|---|---|---|
| Create trip shell | Org Admin | Date/boat/name. Trip starts in `planned`. |
| Assign Site Director to trip | Org Admin | Required before trip can be started. |
| Pre-departure manifest (planned trips) | Org Admin | Initial guest list and cabin assignments. |
| Mid-trip manifest changes | Site Director | Add/remove/reassign once trip is `active`. |
| Start trip (`planned` → `active`) | Site Director | Org Admin cannot. |
| Complete trip (`active` → `completed`) | Site Director | Org Admin cannot. |
| Cancel trip (`planned` only) | Org Admin | Site Director cannot. |
| Catalog item add/edit/deactivate | Org Admin | |
| Record consumption / ledger entries | Site Director | |
| Invite Site Director | Org Admin | MVP user-mgmt subset. |
| Deactivate user | Org Admin | MVP user-mgmt subset. |
| Setup completeness dashboard | Org Admin | What is misconfigured. |
| Operational trip status view | Org Admin | Across trips. |
| Revenue summary per trip | Org Admin | `Should` priority. |
| Cross-trip analytics | Owner | Deferred. |

---

## Out of Scope (All Personas, MVP)

- Guest self-service portal.
- Owner-tier billing and org deletion controls.
- Multi-admin and custom-role administration.
- Cross-organization visibility of any kind.
- Offline / sync.
