# Organization Admin — User Story Backlog

This document is the canonical user story backlog for the Organization Admin persona. It is the source for subsequent implementation sprints.

For persona scope and boundaries see `personas.md`. For sprint synthesis context see `docs/sprints/SPRINT-002.md` and `docs/sprints/drafts/SPRINT-002-MERGE-NOTES.md`.

## Conventions

### Story format

```
### US-N.M: Title

> As an Organization Admin, I want ... so that ...

Priority: Must | Should | Could
Area:     Auth | Organization | Fleet | Trips | Catalog | Users | Oversight
Depends on: US-X.Y, ... (or None)

Acceptance Criteria:
- [ ] Behavior-level criterion (happy path)
- [ ] Validation / error case
- [ ] Authorization / org-scoped access where relevant

Notes: persona boundaries, deferred follow-ups, future-implementation hints.
```

### Priority

- **Must** — required for the first useful release.
- **Should** — important but a release without it is still useful.
- **Could** — nice-to-have. May be deferred indefinitely.

### Area

Stories are grouped by area in `## Group N` sections. The same areas are used in story headers for cross-reference.

### Acceptance criteria style

AC describe **observable behavior**, not implementation mechanism. Mechanism choices (JWT vs session, bcrypt vs argon2, exact session expiry windows) belong to the implementation sprint that builds the feature, not the story backlog.

---

## Product Decisions

These are the product-level decisions that frame the backlog. Confirmed unless marked otherwise.

| Decision | Choice | Rationale |
|---|---|---|
| MVP user management | Minimal subset only: invite Site Director, deactivate user, assign trip leadership. Advanced role admin deferred. | Site Director workflows depend on the ability to invite and assign — full deferral would block the next implementation sprints. |
| Catalog pricing | Org-level flat per-item pricing for MVP. Per-boat / per-trip overrides captured as `Could` follow-ups. | Simplest pricing model that supports a real first release. |
| Manifest ownership | Org Admin prepares the initial manifest pre-departure. Mid-trip manifest mutations belong to Site Director. | Matches `personas.md` and the README. |
| Cabin model | Cabins defined inline when adding a boat. Multi-berth cabins are represented as separately assignable units (e.g., Cabin 1A, Cabin 1B). | Each berth is the assignable unit for guest cabin assignment. |
| Trip lifecycle | Org Admin creates, configures, cancels (planned only), and monitors trips. Site Director performs `planned → active` and `active → completed` transitions. | Reflects who is on the boat at the moment of the transition. |
| Soft deletion | Boats, trips, catalog items, and users are deactivated/archived rather than hard-deleted. | Preserves historical trip and ledger integrity. |
| Reporting (Org Admin) | Setup completeness and operational status are `Must`. Revenue summaries are `Should`. Cross-trip analytics deferred (post-MVP). | Matches persona boundaries. |
| Inventory tracking | Deferred. Captured as `Could`. | Out of scope for first release. |
| Trip booking fees | Out of scope. Catalog covers onboard consumption only. | |

---

## Non-Goals

The following are explicitly out of scope for the Organization Admin backlog:

- Live ledger entry / mid-trip consumption flows (Site Director).
- Mid-trip manifest operations (Site Director).
- Trip start / complete lifecycle transitions (Site Director).
- Guest self-service portal.
- Cross-organization visibility of any kind.
- Deep analytics and reporting beyond setup + operational status + per-trip revenue.
- Billing and org-deletion controls (post-MVP).
- Advanced role administration (multi-admin, custom roles, granular permissions).
- Inventory tracking, per-boat/per-trip pricing overrides, trip booking fees.
- Offline / sync.
- Cloud deployment and infrastructure concerns.

---

## Security Considerations (Backlog-Level)

Apply to every Organization Admin story unless explicitly stated:

- All actions require an authenticated user with an Org Admin role on the requested organization.
- Every read and mutation is scoped to the user's organization. Cross-tenant access is rejected.
- Authentication flows must not enable email enumeration (login, signup, password reset).
- Passwords are stored hashed (mechanism chosen at implementation time).
- Sessions expire and can be invalidated server-side.
- Archival/deactivation preserves historical references; no destructive deletion of records that are referenced by trips or ledger entries.

---

## Story Map

```
                       Organization Admin
                              |
   ┌─────┬─────────┬──────┬──────┬────────┬───────┬──────────┐
  Auth   Org Mgmt  Fleet  Trips  Catalog  Users   Oversight
  (1.x)  (2.x)     (3.x)  (4.x)  (5.x)    (6.x)   (7.x)
```

---

## Group 1: Authentication & Account

### US-1.1: Sign up as an Organization Admin

> As a new user, I want to create an account and register my organization so that I can start managing my liveaboard operations.

Priority: Must
Area: Auth
Depends on: None

Acceptance Criteria:
- [ ] User provides email, password, full name, and organization name.
- [ ] Password meets a documented minimum strength rule (length + complexity).
- [ ] Organization is created with the user as the first Org Admin.
- [ ] Email verification is required before the user can access org data.
- [ ] Duplicate email addresses are rejected with a non-enumerating error.
- [ ] After verification, the user lands on the organization dashboard.

Notes: Specific password rule and verification email mechanism are implementation choices.

### US-1.2: Log in

> As an Organization Admin, I want to log in with my email and password so that I can access my organization.

Priority: Must
Area: Auth
Depends on: US-1.1

Acceptance Criteria:
- [ ] User provides email and password.
- [ ] Invalid credentials show a generic error (no email enumeration).
- [ ] Successful login establishes a session that persists across browser refreshes.
- [ ] Sessions expire after an inactivity period.
- [ ] Login is rate-limited against brute force.

Notes: Session vs JWT, exact expiry windows, rate-limit thresholds: implementation-time decisions.

### US-1.3: Log out

> As an Organization Admin, I want to log out so that my session is terminated.

Priority: Must
Area: Auth
Depends on: US-1.2

Acceptance Criteria:
- [ ] Logout invalidates the current session server-side.
- [ ] User is redirected to the login page.
- [ ] Subsequent requests with the prior session are rejected.

### US-1.4: Reset password

> As an Organization Admin, I want to reset my password if I forget it so that I can regain access.

Priority: Should
Area: Auth
Depends on: US-1.1

Acceptance Criteria:
- [ ] User enters an email; a reset link is "sent" with no email-existence disclosure.
- [ ] Reset link expires after a bounded time window.
- [ ] Setting a new password invalidates all existing sessions for the user.
- [ ] Reused or expired reset links fail safely.

### US-1.5: Update my profile

> As an Organization Admin, I want to update my name, email, and password so that my account stays current.

Priority: Should
Area: Auth
Depends on: US-1.2

Acceptance Criteria:
- [ ] User can edit full name.
- [ ] Email change requires re-verification of the new address.
- [ ] Password change requires the current password.
- [ ] Changes are visible immediately on next request.

---

## Group 2: Organization Management

### US-2.1: View organization details

> As an Organization Admin, I want to view my organization so that I can see its current configuration.

Priority: Must
Area: Organization
Depends on: US-1.2

Acceptance Criteria:
- [ ] Dashboard shows organization name, creation date, and summary stats: number of boats, active trips, total guests on active trips.
- [ ] All data is scoped to the user's organization.

### US-2.2: Update organization details

> As an Organization Admin, I want to update my organization's name so that it stays accurate.

Priority: Should
Area: Organization
Depends on: US-2.1

Acceptance Criteria:
- [ ] Name is editable; non-empty validation enforced.
- [ ] Change is reflected immediately.

### US-2.3: Set currency for the organization

> As an Organization Admin, I want to set the org's default currency so that prices are displayed consistently.

Priority: Must
Area: Organization
Depends on: US-2.1

Acceptance Criteria:
- [ ] Admin selects from a list of common currencies (USD, EUR, GBP, IDR, THB, AUD, ...).
- [ ] Currency applies to all catalog prices and ledger entries org-wide.
- [ ] Changing currency does not retroactively rewrite historical prices; admin is warned and must confirm.

Notes: Multi-currency support and FX conversion are out of scope.

---

## Group 3: Fleet Management — Boats & Cabins

### US-3.1: Add a boat to the fleet

> As an Organization Admin, I want to add a new boat with its cabin layout so that I can run trips on it.

Priority: Must
Area: Fleet
Depends on: US-2.1

Acceptance Criteria:
- [ ] Admin provides boat name (required) and description (optional).
- [ ] Admin defines at least one cabin; each cabin has a name and a berth count.
- [ ] Multi-berth cabins are persisted as separately assignable units (e.g., 1A, 1B).
- [ ] Boat name is unique within the organization.
- [ ] Boat appears in the fleet list with total berth capacity.

### US-3.2: View fleet

> As an Organization Admin, I want to see all boats in my fleet so that I can manage them.

Priority: Must
Area: Fleet
Depends on: US-3.1

Acceptance Criteria:
- [ ] List shows boat name, number of cabins, total berth capacity, and number of active/upcoming trips.
- [ ] Sorted alphabetically by name; only active (non-archived) boats by default.

### US-3.3: View boat details and cabins

> As an Organization Admin, I want to see a specific boat's details including cabin layout.

Priority: Must
Area: Fleet
Depends on: US-3.2

Acceptance Criteria:
- [ ] Detail view shows name, description, full cabin/berth list, and total capacity.
- [ ] Lists upcoming and past trips for this boat.

### US-3.4: Update boat details

> As an Organization Admin, I want to update a boat's name, description, or cabin layout so that information stays accurate.

Priority: Must
Area: Fleet
Depends on: US-3.3

Acceptance Criteria:
- [ ] Admin can edit name and description.
- [ ] Admin can add cabins/berths.
- [ ] Admin can rename cabins/berths.
- [ ] Admin can remove cabins/berths only if they are not referenced by any active or upcoming trip's manifest.
- [ ] Updated boat name remains unique within the organization.

### US-3.5: Archive a boat

> As an Organization Admin, I want to archive a boat so that it is no longer available for new trips while preserving history.

Priority: Must
Area: Fleet
Depends on: US-3.3

Acceptance Criteria:
- [ ] Boat cannot be archived if it has active or upcoming trips.
- [ ] Archived boat is hidden from default fleet views and trip-creation pickers.
- [ ] Historical trips and ledger entries remain intact and queryable.
- [ ] Confirmation is required.

Notes: Soft deletion only.

---

## Group 4: Trip Management

### US-4.1: Create a trip

> As an Organization Admin, I want to create a trip on a boat so that it can be planned and staffed.

Priority: Must
Area: Trips
Depends on: US-3.1

Acceptance Criteria:
- [ ] Admin selects a boat and provides trip name, start date, and end date.
- [ ] Start date must be in the future; end date must be after start date.
- [ ] Trips on the same boat cannot have overlapping date ranges.
- [ ] Trip is created in `planned` status.
- [ ] Trip snapshots the boat's current cabin layout as its available cabins.

### US-4.2: View all trips

> As an Organization Admin, I want to see all trips across my fleet so that I can plan and monitor operations.

Priority: Must
Area: Trips
Depends on: US-4.1

Acceptance Criteria:
- [ ] List shows trip name, boat, dates, status, guest count, occupancy %.
- [ ] Filter by status (`planned`, `active`, `completed`, `cancelled`) and by boat.
- [ ] Default sort: upcoming first by start date.

### US-4.3: View trip details

> As an Organization Admin, I want to see a trip's details so that I can review its current state.

Priority: Must
Area: Trips
Depends on: US-4.2

Acceptance Criteria:
- [ ] Shows name, boat, dates, status, assigned Site Director.
- [ ] Shows cabin occupancy: which cabins/berths are assigned, which are empty.
- [ ] Shows the manifest with cabin assignments.
- [ ] Shows revenue summary (charges, settled, outstanding) — read-only.

### US-4.4: Update trip details

> As an Organization Admin, I want to update a trip's name or dates so that I can adjust plans.

Priority: Must
Area: Trips
Depends on: US-4.1

Acceptance Criteria:
- [ ] Name is editable at any status except `cancelled`.
- [ ] Dates are editable only while status is `planned`.
- [ ] Updated dates must not overlap with other trips on the same boat.
- [ ] Start date must remain in the future.

### US-4.5: Prepare initial manifest — add a guest (pre-departure)

> As an Organization Admin, I want to add guests to a planned trip and assign them to cabins so that the manifest is ready before departure.

Priority: Must
Area: Trips
Depends on: US-4.1

Acceptance Criteria:
- [ ] Available only while trip status is `planned`.
- [ ] Admin provides guest name (required) and email (optional).
- [ ] Admin assigns the guest to an available berth (e.g., Cabin 1A).
- [ ] A berth cannot be assigned to more than one guest at a time.
- [ ] Guest count and occupancy % update on the trip view.

Notes: Mid-trip add/remove/reassign once the trip is `active` is Site Director scope.

### US-4.6: Prepare initial manifest — remove a guest (pre-departure)

> As an Organization Admin, I want to remove a guest from a planned trip so that their berth is freed up.

Priority: Must
Area: Trips
Depends on: US-4.5

Acceptance Criteria:
- [ ] Available only while trip status is `planned`.
- [ ] Guest is removed from the manifest; their berth is released.
- [ ] Confirmation is required.

### US-4.7: Prepare initial manifest — reassign cabin (pre-departure)

> As an Organization Admin, I want to move a guest to a different cabin/berth so that I can adjust the planned manifest.

Priority: Should
Area: Trips
Depends on: US-4.5

Acceptance Criteria:
- [ ] Available only while trip status is `planned`.
- [ ] Admin picks a new available berth.
- [ ] Previous berth is released.

### US-4.8: Monitor trip lifecycle (read-only for active/completed)

> As an Organization Admin, I want to see when trips start, are active, and complete so that I can track operations across the fleet.

Priority: Must
Area: Trips
Depends on: US-4.3

Acceptance Criteria:
- [ ] Trip detail view reflects status changes initiated by the Site Director (`planned → active`, `active → completed`).
- [ ] Org Admin cannot perform `start` or `complete` actions themselves.
- [ ] Trip history view shows lifecycle timestamps.

Notes: Lifecycle transitions are owned by Site Director (see `personas.md`).

### US-4.9: Cancel a planned trip

> As an Organization Admin, I want to cancel a planned trip so that it no longer appears as upcoming.

Priority: Must
Area: Trips
Depends on: US-4.1

Acceptance Criteria:
- [ ] Available only while status is `planned`.
- [ ] Confirmation is required.
- [ ] Status becomes `cancelled`; trip is hidden from default views but kept for records.

### US-4.10: Assign a Site Director to a trip

> As an Organization Admin, I want to assign a Site Director to a trip so that it can be operated.

Priority: Must
Area: Trips
Depends on: US-4.1, US-6.1

Acceptance Criteria:
- [ ] Admin selects from active users who have accepted a Site Director invitation.
- [ ] At most one Site Director is assigned per trip at a time.
- [ ] Reassignment is allowed while status is `planned` or `active`.
- [ ] A trip cannot be transitioned to `active` (by the Site Director) without an assigned Site Director.

Notes: Required to unblock Site Director workflows in subsequent sprints.

---

## Group 5: Catalog Management

### US-5.1: Add a catalog item

> As an Organization Admin, I want to add items and services to the catalog so that they can be sold to guests during trips.

Priority: Must
Area: Catalog
Depends on: US-2.3

Acceptance Criteria:
- [ ] Admin provides item name (required), category (required), description (optional), and price (required, positive decimal).
- [ ] Item name is unique within the organization.
- [ ] Price is in the org's default currency.
- [ ] Item is active by default.

### US-5.2: View catalog

> As an Organization Admin, I want to see all catalog items so that I can manage what is available.

Priority: Must
Area: Catalog
Depends on: US-5.1

Acceptance Criteria:
- [ ] List shows name, category, price, active/inactive status.
- [ ] Filter by category, search by name.
- [ ] Sort by category, then name.

### US-5.3: Update a catalog item

> As an Organization Admin, I want to update an item's details or price so that the catalog stays current.

Priority: Must
Area: Catalog
Depends on: US-5.2

Acceptance Criteria:
- [ ] Admin can edit name, description, category, and price.
- [ ] Price changes apply only to future ledger entries; historical entries retain their original price.
- [ ] Updated name remains unique within the organization.

### US-5.4: Deactivate a catalog item

> As an Organization Admin, I want to deactivate an item so that it stops appearing for sale without losing history.

Priority: Must
Area: Catalog
Depends on: US-5.2

Acceptance Criteria:
- [ ] Inactive items are hidden from in-trip sale interfaces.
- [ ] Inactive items remain visible in catalog management with a clear indicator.
- [ ] Historical ledger references are preserved.
- [ ] Item can be reactivated.

### US-5.5: Manage catalog categories

> As an Organization Admin, I want to create and manage categories so that the catalog stays organized.

Priority: Must
Area: Catalog
Depends on: US-5.1

Acceptance Criteria:
- [ ] Default categories are seeded (Equipment Rental, Food & Beverage, Merchandise, Service).
- [ ] Admin can add custom categories (e.g., Nitrox, Bar, Gift Shop).
- [ ] Categories can be renamed.
- [ ] Empty categories can be deleted; categories with items cannot.

### US-5.6: Per-boat or per-trip pricing overrides (deferred)

> As an Organization Admin, I want to override item prices for specific boats or trips so that pricing can vary by route or boat tier.

Priority: Could
Area: Catalog
Depends on: US-5.3

Acceptance Criteria: Deferred — captured so it is not lost. Will be expanded if/when prioritized.

### US-5.7: Inventory tracking for catalog items (deferred)

> As an Organization Admin, I want to track stock for items so that the system can warn or block sales when stock is exhausted.

Priority: Could
Area: Catalog
Depends on: US-5.1

Acceptance Criteria: Deferred — captured so it is not lost. Will be expanded if/when prioritized.

---

## Group 6: User Management (MVP Subset)

### US-6.1: Invite a Site Director

> As an Organization Admin, I want to invite a user to be a Site Director so that I can assign them to trips.

Priority: Must
Area: Users
Depends on: US-2.1

Acceptance Criteria:
- [ ] Admin provides invitee email and full name.
- [ ] An invitation is sent to the email; on acceptance the invitee creates a password and joins the organization with the Site Director role.
- [ ] Pending invitations are listed in a users view with status (pending, accepted, expired).
- [ ] Invitations expire after a bounded time window.
- [ ] Duplicate active invitations to the same email are rejected.

### US-6.2: Deactivate a user

> As an Organization Admin, I want to deactivate a user so that they can no longer access the organization.

Priority: Must
Area: Users
Depends on: US-6.1

Acceptance Criteria:
- [ ] Deactivating a user invalidates all of their active sessions.
- [ ] A deactivated user cannot be assigned to new trips.
- [ ] Existing trip assignments to the deactivated user are flagged as needing reassignment.
- [ ] Deactivation is reversible (reactivate).
- [ ] An Org Admin cannot deactivate the only remaining Org Admin.

### US-6.3: Re-send a pending invitation

> As an Organization Admin, I want to resend a pending invitation so that an invitee who lost or missed the original can join.

Priority: Should
Area: Users
Depends on: US-6.1

Acceptance Criteria:
- [ ] Resend is allowed only for invitations in `pending` status.
- [ ] Resend issues a new link and resets the expiry window.
- [ ] Previous link is invalidated.

Notes: Multi-admin, custom-role administration, and granular permissions are explicitly deferred.

---

## Group 7: Admin Oversight

### US-7.1: Setup completeness dashboard

> As an Organization Admin, I want to see what is misconfigured across the org so that I can fix it before trips start.

Priority: Must
Area: Oversight
Depends on: US-2.1

Acceptance Criteria:
- [ ] Dashboard surfaces: boats with no cabins, trips in `planned` with no Site Director assigned, trips in `planned` with empty manifests inside a configurable time-to-departure window, catalog items with no category, organization with no currency set.
- [ ] Each item links to the screen where it can be fixed.

### US-7.2: Operational trip status view

> As an Organization Admin, I want a single view of trip status across the fleet so that I know what is happening at a glance.

Priority: Must
Area: Oversight
Depends on: US-4.2

Acceptance Criteria:
- [ ] Counts of trips in each status (`planned`, `active`, `completed`, `cancelled`) for a configurable time window.
- [ ] Per-trip occupancy % for upcoming and active trips.
- [ ] No drill-down required for the headline numbers; details linked through to trip view.

### US-7.3: Revenue summary per trip

> As an Organization Admin, I want to see a per-trip revenue summary so that I can monitor financial activity.

Priority: Should
Area: Oversight
Depends on: US-4.3

Acceptance Criteria:
- [ ] For each trip: total charges, total settled, total outstanding.
- [ ] Aggregations are reproducible from the underlying ledger.

### US-7.4: Cross-trip analytics (deferred)

> As an Organization Admin, I want cross-trip and cross-boat analytics so that I can identify trends.

Priority: Could
Area: Oversight
Depends on: US-7.3

Acceptance Criteria: Deferred (post-MVP). Captured here so it is not lost.

---

## Sprint Slicing — Suggested Implementation Order

The story IDs below feed proposed implementation sprints (003+). Final sprint scoping decisions belong in those sprint docs.

1. **Sprint 003 — Auth + Org foundation:** US-1.1, US-1.2, US-1.3, US-2.1.
2. **Sprint 004 — Fleet:** US-3.1, US-3.2, US-3.3, US-3.4, US-3.5.
3. **Sprint 005 — Catalog + currency:** US-2.3, US-5.1, US-5.2, US-5.3, US-5.4, US-5.5.
4. **Sprint 006 — Trips + Site Director invitation:** US-4.1, US-4.2, US-4.3, US-4.4, US-4.9, US-4.10, US-6.1, US-6.2.
5. **Sprint 007 — Pre-departure manifest + oversight:** US-4.5, US-4.6, US-4.7, US-4.8, US-7.1, US-7.2.
6. **Sprint 008 — Polish:** US-1.4, US-1.5, US-2.2, US-6.3, US-7.3.

Deferred (no sprint assignment): US-5.6, US-5.7, US-7.4.

---

## Dependencies Summary

| Story | Depends on |
|---|---|
| US-1.1 | None |
| US-1.2 | US-1.1 |
| US-1.3 | US-1.2 |
| US-1.4 | US-1.1 |
| US-1.5 | US-1.2 |
| US-2.1 | US-1.2 |
| US-2.2 | US-2.1 |
| US-2.3 | US-2.1 |
| US-3.1 | US-2.1 |
| US-3.2 | US-3.1 |
| US-3.3 | US-3.2 |
| US-3.4 | US-3.3 |
| US-3.5 | US-3.3 |
| US-4.1 | US-3.1 |
| US-4.2 | US-4.1 |
| US-4.3 | US-4.2 |
| US-4.4 | US-4.1 |
| US-4.5 | US-4.1 |
| US-4.6 | US-4.5 |
| US-4.7 | US-4.5 |
| US-4.8 | US-4.3 |
| US-4.9 | US-4.1 |
| US-4.10 | US-4.1, US-6.1 |
| US-5.1 | US-2.3 |
| US-5.2 | US-5.1 |
| US-5.3 | US-5.2 |
| US-5.4 | US-5.2 |
| US-5.5 | US-5.1 |
| US-5.6 | US-5.3 |
| US-5.7 | US-5.1 |
| US-6.1 | US-2.1 |
| US-6.2 | US-6.1 |
| US-6.3 | US-6.1 |
| US-7.1 | US-2.1 |
| US-7.2 | US-4.2 |
| US-7.3 | US-4.3 |
| US-7.4 | US-7.3 |
