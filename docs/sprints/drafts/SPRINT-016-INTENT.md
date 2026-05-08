# Sprint 016 Intent: Cabin Layouts and Trip Berth Assignments

## Seed

kplan build the cabin layout and assignment experience as per above. We will need to ensure that this layout is done during importing or adding a boat to the fleet. We should also allow admins / site directors to change the cabin layout post boat upload.

Context from the prior discussion:

- Cabin assignments should not require tedious manual entry of every cabin/bed slot.
- A boat should have a reusable cabin layout template.
- Cabins can have alphanumeric berth labels such as `1A`, `1B`, `2A`, `2B`.
- More than one guest can share a cabin, so assignments should target berth slots, not only cabin numbers.
- Useful creation methods include bulk range generation, pasted text/CSV import, and later optional deck-plan reference.

## Context

Liveaboard is a multi-tenant SaaS for scuba liveaboard operators. Recent work added:

- Trip guest manifest and guest registration.
- Guest-facing registration save/return/submit.
- Staff guest registration detail views.
- Per-guest checkout folios.
- Per-boat inventory and org payment settings.

The current product docs explicitly defer cabin layouts. Boats currently have scraper-owned source fields and an operator-owned display name, but no capacity or spatial layout. Trips currently refer to one boat and carry imported expected guest count, but no cabin inventory. The manifest is a flat guest list.

## Recent Sprint Context

### Sprint 013: Catalog, Inventory, and Checkout Currency Foundation

Created org-owned catalog, per-boat inventory, stock movement history, and FX quote foundations. Important pattern: boat-specific operational data is stored in separate org-scoped tables with store helpers enforcing org/boat ownership.

### Sprint 014: Guest Management and Trip Registration

Added guest accounts, guest sessions, trip guests, invitations, draft/submitted registration, and staff manifest views. Important pattern: staff access to trip guest data flows through manifest authorization, with Org Admin global org scope and Cruise Director assigned-trip scope.

### Sprint 015: Guest Folio Checkout and Payment Settings

Added one closed checkout folio per guest/trip, org payment settings, email folios, and atomic inventory decrement on close. Important pattern: per-guest trip operations are scoped by organization, trip, and trip_guest, not route authorization alone.

## Relevant Codebase Areas

- `internal/store/boats.go` and `internal/store/migrations/0005_boats_and_trips.sql` — current boat model and importer upsert behavior.
- `internal/httpapi/admin.go` — existing fleet and boat detail endpoints.
- `internal/httpapi/import_handlers.go` and `internal/imports/spreadsheet/*` — spreadsheet import flow where unknown vessels are mapped to existing boats or created.
- `internal/imports/liveaboard_runner.go` — liveaboard.com import job, which upserts boats and trips.
- `internal/store/trip_guests.go` — trip manifest rows and summaries.
- `internal/httpapi/guest_manifest_handlers.go` — manifest authorization and staff guest endpoints.
- `web/src/admin/pages/Fleet.tsx`, `BoatDetail.tsx`, `BoatTabs.tsx` — fleet and boat detail UI.
- `web/src/admin/pages/ImportLiveaboard.tsx`, `ImportSpreadsheet.tsx` — current import UI.
- `web/src/admin/pages/TripManifest.tsx` and `TripGuestDetail.tsx` — trip guest list and per-guest detail UI.
- `web/src/admin/api.ts` and `web/src/main.tsx` — admin API client and route wiring.
- `docs/product/personas.md` and `docs/product/organization-admin-user-stories.md` — persona boundaries and deferred cabin decision.

## Constraints

- Must follow project conventions in `CLAUDE.md`.
- Must integrate with existing architecture and tenant isolation.
- Must follow sprint conventions in `docs/sprints/README.md`.
- Work directly on `main` when implementing; no feature branches.
- Every tenant-owned table must carry `organization_id`.
- Org Admin can manage fleet-wide setup. Cruise Director can operate only assigned trips.
- Guests must not see other guests or cabin layout data unless a future guest portal explicitly exposes it.
- Do not rely on `trips.num_guests` as capacity; it is imported expected guest count.
- Boat importers must not clobber operator-owned layout edits.
- Cabin assignments must support multiple guests per cabin through berth-level assignment.
- Closed guest folios must not be affected by cabin assignment changes.

## Success Criteria

- Admins can define a boat cabin layout during manual/imported boat setup without manually entering every berth.
- Admins can edit a boat's reusable cabin layout from the boat detail page.
- Assigned Cruise Directors can view and adjust cabin layout/assignments for their assigned trips when operationally needed.
- Trip manifests show each guest's cabin/berth assignment and occupancy conflicts.
- Staff can assign/unassign guests to specific berths such as `1A` or `Suite 2B`.
- Layout edits after trips exist are safe: destructive changes are blocked or require explicit handling when trip assignments depend on those berths.
- Import flows surface layout setup as a required or strongly prompted post-import step for new boats.
- Tests cover tenant isolation, role scoping, assignment uniqueness, layout generation, and destructive layout edit protection.

## Open Questions

- Should Cruise Directors be allowed to change the reusable boat layout, or only trip-specific assignments and temporary trip overrides?
- Should the first sprint include CSV import, bulk range generation, and paste parsing, or should one of those be deferred?
- Should cabin assignment be required before guest checkout/completion, or remain advisory in this sprint?
- Should berth labels be stored as separate `cabin_label` + `berth_label` fields, or only as a generated display label?
- Should active/completed trips snapshot the layout at trip creation/start, or always reference the current boat layout with destructive edits blocked?
