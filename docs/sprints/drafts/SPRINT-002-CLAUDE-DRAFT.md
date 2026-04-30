# Sprint 002: Organization Admin — User Stories

## Overview

This sprint defines the complete set of user stories for the Organization Admin persona. The Org Admin is responsible for all org-wide operations: managing the fleet of boats (with cabins defined inline), creating and managing trips (including guest manifest and cabin assignment), and maintaining the catalog of items and services available for purchase during trips.

User management (inviting site directors, role management) is deferred — the system starts with a single admin per organization. Catalog pricing is flat per-item at the org level, with no per-boat or per-trip overrides for now.

These stories will drive subsequent implementation sprints. Each story includes acceptance criteria specific enough to be directly implementable and testable.

## Personas Reference

- **Organization Admin:** Has org-wide access. Can manage fleet, create/configure trips, manage guest manifests, and configure the item catalog and pricing.

## Interview Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| User management | Deferred | Single admin is fine to start |
| Catalog pricing | Org-level flat pricing | Start simple, override later if needed |
| Guest manifest | Both Org Admin and Site Director can manage | Either persona can add guests and assign cabins |
| Cabin model | Cabins defined inline when adding a boat | Two-bed cabins become 1A and 1B — each bed is an assignable unit |

---

## User Stories

### Group 1: Authentication & Account

**US-1.1: Sign up as an Organization Admin**
> As a new user, I want to create an account and register my organization so that I can start managing my liveaboard operations.

Acceptance Criteria:
- [ ] User provides email, password, full name, and organization name
- [ ] Password meets minimum security requirements (8+ chars, mixed case, number)
- [ ] Organization is created with the user as the first admin
- [ ] User receives email verification
- [ ] Duplicate email addresses are rejected
- [ ] After verification, user is redirected to the dashboard

**US-1.2: Log in**
> As an Organization Admin, I want to log in with my email and password so that I can access my organization's dashboard.

Acceptance Criteria:
- [ ] User provides email and password
- [ ] Invalid credentials show a generic error (no email enumeration)
- [ ] Successful login redirects to the organization dashboard
- [ ] Session persists across browser refreshes (JWT or session token)
- [ ] Session expires after a configurable inactivity period

**US-1.3: Log out**
> As an Organization Admin, I want to log out so that my session is terminated securely.

Acceptance Criteria:
- [ ] Logout invalidates the current session/token
- [ ] User is redirected to the login page
- [ ] Subsequent requests with the old token are rejected

**US-1.4: Reset password**
> As an Organization Admin, I want to reset my password if I forget it so that I can regain access to my account.

Acceptance Criteria:
- [ ] User enters their email address
- [ ] A password reset link is sent (regardless of whether the email exists — no enumeration)
- [ ] Reset link expires after 1 hour
- [ ] User can set a new password via the link
- [ ] Old sessions are invalidated after password reset

**US-1.5: Update my profile**
> As an Organization Admin, I want to update my name and email so that my account information stays current.

Acceptance Criteria:
- [ ] User can edit full name
- [ ] User can change email (requires re-verification)
- [ ] User can change password (requires current password)
- [ ] Changes are saved and reflected immediately

---

### Group 2: Organization Management

**US-2.1: View organization details**
> As an Organization Admin, I want to view my organization's details so that I can see the current configuration.

Acceptance Criteria:
- [ ] Dashboard shows organization name, creation date, and summary stats (number of boats, active trips, total guests)
- [ ] Data is scoped to the user's organization only

**US-2.2: Update organization details**
> As an Organization Admin, I want to update my organization's name and settings so that they reflect any changes.

Acceptance Criteria:
- [ ] Organization name can be edited
- [ ] Changes are saved and reflected immediately
- [ ] Organization name must be non-empty

**US-2.3: Set currency for the organization**
> As an Organization Admin, I want to set my organization's default currency so that all prices are displayed consistently.

Acceptance Criteria:
- [ ] Admin can select from a list of common currencies (USD, EUR, GBP, IDR, THB, AUD, etc.)
- [ ] Currency is set at the organization level and applies to all catalog prices and ledger entries
- [ ] Changing currency does not retroactively change existing prices — admin is warned and must confirm

---

### Group 3: Fleet Management — Boats & Cabins

**US-3.1: Add a boat to the fleet**
> As an Organization Admin, I want to add a new boat to my organization's fleet so that I can create trips on it.

Acceptance Criteria:
- [ ] Admin provides boat name (required), description (optional), and cabin layout
- [ ] Cabin layout: admin defines each cabin with a name and capacity (number of berths)
- [ ] Two-berth cabins are represented as two units (e.g., Cabin 1A and Cabin 1B)
- [ ] Boat name must be unique within the organization
- [ ] At least one cabin must be defined
- [ ] Boat appears in the fleet list with total berth capacity

**US-3.2: View fleet**
> As an Organization Admin, I want to see all boats in my fleet so that I can manage them.

Acceptance Criteria:
- [ ] List shows all boats with name, number of cabins, total berth capacity, and number of active/upcoming trips
- [ ] Boats are sorted alphabetically by name

**US-3.3: View boat details and cabins**
> As an Organization Admin, I want to see a specific boat's details including its cabin layout so that I can review it.

Acceptance Criteria:
- [ ] Detail view shows boat name, description, and cabin list with names and capacity
- [ ] Cabins are displayed grouped by deck or position (if specified)
- [ ] Shows total berth capacity
- [ ] Shows upcoming and past trips for this boat

**US-3.4: Update boat details**
> As an Organization Admin, I want to update a boat's name, description, or cabin layout so that the information stays accurate.

Acceptance Criteria:
- [ ] Admin can edit boat name and description
- [ ] Admin can add new cabins to the boat
- [ ] Admin can remove cabins that are not assigned to any active or upcoming trip
- [ ] Admin can rename cabins
- [ ] Updated boat name must still be unique within the organization
- [ ] Changes are saved immediately

**US-3.5: Remove a boat from the fleet**
> As an Organization Admin, I want to remove a boat from my fleet so that it's no longer available for new trips.

Acceptance Criteria:
- [ ] Boat cannot be removed if it has active or upcoming trips
- [ ] Removing a boat preserves historical trip data associated with it
- [ ] Boat is soft-deleted (marked inactive, not physically deleted)
- [ ] Confirmation is required before removal

---

### Group 4: Trip Management

**US-4.1: Create a trip**
> As an Organization Admin, I want to create a new trip on a boat so that I can plan an upcoming voyage.

Acceptance Criteria:
- [ ] Admin selects a boat, provides trip name, start date, and end date
- [ ] Start date must be in the future
- [ ] End date must be after start date
- [ ] Trips on the same boat cannot have overlapping dates
- [ ] Trip is created in "planned" status
- [ ] Trip inherits the boat's current cabin layout as its available cabins

**US-4.2: View all trips**
> As an Organization Admin, I want to see all trips across my fleet so that I can plan and monitor operations.

Acceptance Criteria:
- [ ] List shows all trips with name, boat name, dates, status, guest count, and occupancy percentage
- [ ] Trips can be filtered by status (planned, active, completed, cancelled)
- [ ] Trips can be filtered by boat
- [ ] Default sort is by start date (upcoming first)

**US-4.3: View trip details**
> As an Organization Admin, I want to see a specific trip's details so that I can review its status.

Acceptance Criteria:
- [ ] Shows trip name, boat, dates, status
- [ ] Shows cabin occupancy — which cabins are assigned, which are empty
- [ ] Shows guest list with cabin assignments
- [ ] Shows revenue summary (total charges, total settled, total outstanding)

**US-4.4: Update trip details**
> As an Organization Admin, I want to update a trip's name or dates so that I can adjust plans.

Acceptance Criteria:
- [ ] Trip name can be edited at any time
- [ ] Dates can only be changed while trip is in "planned" status
- [ ] Updated dates must not overlap with other trips on the same boat
- [ ] Start date must remain in the future (for planned trips)

**US-4.5: Add a guest to a trip**
> As an Organization Admin, I want to add guests to a trip and assign them to cabins so that the manifest is ready before departure.

Acceptance Criteria:
- [ ] Admin provides guest name (required) and email (optional)
- [ ] Admin assigns the guest to an available cabin (e.g., Cabin 1A)
- [ ] A cabin cannot be assigned to more than one guest at a time
- [ ] Guest appears in the trip manifest
- [ ] Total guest count and occupancy percentage update accordingly

**US-4.6: Remove a guest from a trip**
> As an Organization Admin, I want to remove a guest from a trip so that their cabin is freed up.

Acceptance Criteria:
- [ ] Guest is removed from the manifest
- [ ] Their cabin assignment is released
- [ ] If the guest has open ledger entries, a warning is shown
- [ ] Removal requires confirmation if there are charges
- [ ] Occupancy percentage updates accordingly

**US-4.7: Reassign a guest's cabin**
> As an Organization Admin, I want to move a guest to a different cabin so that I can adjust the manifest.

Acceptance Criteria:
- [ ] Admin selects a new cabin from available (unoccupied) cabins
- [ ] Previous cabin is released
- [ ] Guest's ledger history is unaffected by the move

**US-4.8: Start a trip**
> As an Organization Admin, I want to mark a trip as active so that onboard operations can begin.

Acceptance Criteria:
- [ ] Only trips in "planned" status can be started
- [ ] Trip status changes to "active"
- [ ] No minimum guest count required to start

**US-4.9: Complete a trip**
> As an Organization Admin, I want to mark an active trip as completed so that it's archived.

Acceptance Criteria:
- [ ] Only active trips can be completed
- [ ] Trip status changes to "completed"
- [ ] A warning is shown if there are unsettled guest tabs
- [ ] Completed trips remain visible in trip history

**US-4.10: Cancel a trip**
> As an Organization Admin, I want to cancel a planned trip so that it no longer appears as upcoming.

Acceptance Criteria:
- [ ] Only trips in "planned" status can be cancelled
- [ ] Cancellation requires confirmation
- [ ] Cancelled trips are soft-deleted (kept for records but hidden from default views)

---

### Group 5: Catalog Management

**US-5.1: Add an item to the catalog**
> As an Organization Admin, I want to add items and services to my organization's catalog so that they can be sold to guests during trips.

Acceptance Criteria:
- [ ] Admin provides item name (required), category (required), description (optional), and price (required, positive decimal)
- [ ] Item name must be unique within the organization
- [ ] Price is in the organization's default currency
- [ ] Item is active by default
- [ ] Item appears in the catalog

**US-5.2: View catalog**
> As an Organization Admin, I want to see all items in my catalog so that I can manage what's available.

Acceptance Criteria:
- [ ] List shows all items with name, category, price, and active/inactive status
- [ ] Items can be filtered by category
- [ ] Items can be searched by name
- [ ] Items are sorted by category, then name

**US-5.3: Update a catalog item**
> As an Organization Admin, I want to update an item's details or price so that the catalog stays current.

Acceptance Criteria:
- [ ] Admin can edit name, description, category, and price
- [ ] Price changes apply to future transactions only (historical ledger entries retain original price)
- [ ] Updated name must remain unique within the organization

**US-5.4: Deactivate a catalog item**
> As an Organization Admin, I want to deactivate a catalog item so that it's no longer available for sale without deleting historical data.

Acceptance Criteria:
- [ ] Inactive items are hidden from the sale interface during trips
- [ ] Inactive items remain visible in catalog management (with clear inactive indicator)
- [ ] Historical ledger entries referencing the item are preserved
- [ ] Item can be reactivated at any time

**US-5.5: Manage catalog categories**
> As an Organization Admin, I want to create and manage categories for catalog items so that they are organized logically.

Acceptance Criteria:
- [ ] Admin can create custom categories (e.g., "Nitrox", "Bar", "Gift Shop")
- [ ] Default categories are provided (Equipment Rental, Food & Beverage, Merchandise, Service)
- [ ] Categories can be renamed
- [ ] Categories with items assigned cannot be deleted
- [ ] Empty categories can be deleted

---

## Story Map

```
                        Organization Admin
                              |
    ┌─────────┬──────────┬────┴────┬──────────┐
    │         │          │         │          │
   Auth    Org Mgmt    Fleet     Trips    Catalog
  (1.x)    (2.x)      (3.x)    (4.x)     (5.x)
    │         │          │         │          │
  Sign up   View      Add boat  Create    Add item
  Log in    Update    View fleet View all  View
  Log out   Currency  Details   Details   Update
  Reset PW           Update    Update    Deactivate
  Profile            Remove    Add guest Categories
                               Remove
                               Reassign
                               Start
                               Complete
                               Cancel
```

## Suggested Implementation Order

1. **Foundation (Sprint 003):** US-1.1, US-1.2, US-1.3, US-2.1 — Auth + basic org dashboard. Unblocks everything.
2. **Fleet (Sprint 004):** US-3.1–3.5 — Boats with inline cabin layout. Required before trips.
3. **Catalog (Sprint 005):** US-5.1–5.5, US-2.3 — Items, categories, and currency. Required before trips can track consumption.
4. **Trips (Sprint 006):** US-4.1–4.10 — Full trip lifecycle including guest manifest and cabin assignment.
5. **Polish (Sprint 007):** US-1.4, US-1.5, US-2.2 — Password reset, profile updates, org settings.

## Definition of Done

- [ ] All user stories have clear, testable acceptance criteria
- [ ] Stories cover the full scope of Organization Admin responsibilities
- [ ] Stories are grouped logically by domain area
- [ ] Implementation ordering accounts for dependencies between groups
- [ ] No stories assume offline/sync capabilities
- [ ] Stories respect multi-tenant data isolation
- [ ] User management is explicitly deferred
- [ ] Pricing model is org-level flat pricing only

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Scope creep into Site Director responsibilities | Medium | Medium | Clearly delineate: Org Admin manages structure and configuration; manifest is shared |
| Cabin naming convention confusion | Low | Medium | Document the 1A/1B convention clearly; validate on input |
| Catalog pricing model too simple | Medium | Medium | Start with flat per-item pricing; note that overrides may come later |
| Missing edge cases in multi-tenant isolation | Medium | High | Every story implicitly requires org-scoped data access |

## Security Considerations

- All API endpoints must verify the user's organization membership and role
- Password handling must use bcrypt or argon2 — never store plaintext
- Session tokens must be cryptographically random, stored securely, and expire
- Email enumeration must be prevented in login, signup, and password reset flows
- Multi-tenant isolation: every database query must be scoped to the user's organization

## Dependencies

- Sprint 001 (sprint tooling) — should be completed first for workflow tooling
- No external dependencies — all stories are self-contained within the platform

## Open Questions

1. Should catalog support inventory tracking (stock quantities) at this stage, or defer to a later sprint?
2. Should trip pricing (what guests pay to book the trip itself) be managed here, or is this purely about onboard consumption catalog?
3. Should the Org Admin see a basic revenue/activity dashboard, or is that an Organization Owner concern?
