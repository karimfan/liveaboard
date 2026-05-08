# Sprint 016: Cabin Layouts and Guest Berth Assignments

## Overview

Sprint 016 adds reusable boat cabin layouts and mandatory trip guest
berth assignment. A cabin layout belongs to a boat and defines cabins
plus berth slots such as `1A`, `1B`, `Suite 2A`, or `Upper Port`.
Trip guests are assigned to those berth slots when they are added or
bound to a trip, and Admins or assigned Cruise Directors can change the
assignment later.

This sprint also makes layout setup part of the fleet/import workflow.
Operators can generate common layouts by range, paste structured rows,
or upload a CSV using an explicit schema. Imported or newly created
boats with no active berths are marked as needing layout setup.

## Use Cases

1. **Configure a new boat layout.** After importing or adding a boat,
   staff configures cabins and berths without manually typing every
   berth one by one.
2. **Generate common layouts.** Staff enters cabin ranges such as
   `1-10` with berths `A,B`, previews the generated layout, and saves.
3. **Paste layout rows.** Staff pastes structured cabin rows and sees
   row-level validation before saving.
4. **Upload CSV.** Staff uploads a CSV using the documented schema:
   `cabin_label,berth_label,deck,sort_order,notes`, one berth per row.
5. **Edit layout later.** Org Admins and assigned Cruise Directors can
   edit cabin labels, decks, berth labels, order, and notes after boat
   upload, with destructive changes guarded when assignments exist.
6. **Enroll a guest with a cabin.** Adding a guest to a trip requires
   selecting an active berth on that trip's boat.
7. **Change assignments later.** Org Admins and assigned Cruise
   Directors can move or unassign/reassign a guest berth later.
8. **See occupancy.** The manifest and cabin board show assigned
   berths, unassigned guests that need attention, occupied berths, and
   available berths.
9. **Free berths on revoke.** Revoking a guest invite also clears that
   guest's active berth assignment.

## Architecture

### Core Rules

- Cabin layout is stored on the boat and is never overwritten by import
  refreshes.
- Cabin labels and berth labels are separate fields; `display_label`
  is server-derived for UI display.
- Assignment targets a berth, not a cabin.
- A guest must have a berth assignment when added/bound to a trip.
- One active `trip_guest` can have one active berth assignment.
- One active berth can be occupied by one active `trip_guest` per trip.
- Multiple guests share a cabin through different berths.
- Unassign is soft-audited with `unassigned_at` and
  `unassigned_by_user_id`; it releases the berth for reassignment.
- Revoking a `trip_guest` automatically unassigns their active berth.
- Destructive layout replacement is allowed only when no active trip
  assignments reference the boat's layout.
- Once assignments exist, layout changes happen through per-cabin and
  per-berth patch/deactivate operations with dependency checks.
- Store helpers enforce `organization_id`, trip, guest, boat, and berth
  ownership even when HTTP handlers already authorize the route.
- Guests do not see cabin data in this sprint.

### Permissions

| Capability | Org Admin | Assigned Cruise Director | Guest |
|---|---:|---:|---:|
| View fleet boat layout | Yes | Through assigned-trip context only | No |
| Configure reusable boat layout | Yes | Yes, for boats on assigned trips | No |
| Bulk replace unassigned layout | Yes | Yes, for boats on assigned trips | No |
| View trip cabin board | Yes | Yes, assigned trips only | No |
| Assign/move/unassign guest berths | Yes | Yes, assigned trips only | No |
| Revoke guest and auto-free berth | Yes | Yes, assigned trips only | No |

### Schema

Add migration `0014_cabin_layouts.sql`.

```sql
boat_cabins (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  boat_id uuid not null references boats(id) on delete cascade,
  label text not null,
  deck text null,
  sort_order int not null default 0,
  notes text null,
  is_active boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

```sql
boat_cabin_berths (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  boat_id uuid not null references boats(id) on delete cascade,
  cabin_id uuid not null references boat_cabins(id) on delete cascade,
  berth_label text not null,
  display_label text not null,
  sort_order int not null default 0,
  notes text null,
  is_active boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

```sql
trip_cabin_assignments (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  boat_id uuid not null references boats(id) on delete cascade,
  berth_id uuid not null references boat_cabin_berths(id) on delete restrict,
  cabin_label_snapshot text not null,
  berth_label_snapshot text not null,
  display_label_snapshot text not null,
  assigned_by_user_id uuid null references users(id) on delete set null,
  assigned_at timestamptz not null default now(),
  unassigned_by_user_id uuid null references users(id) on delete set null,
  unassigned_at timestamptz null,
  notes text null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

Indexes:

```sql
CREATE INDEX boat_cabins_org_boat_idx
  ON boat_cabins(organization_id, boat_id, sort_order);

CREATE UNIQUE INDEX boat_cabins_org_boat_label_active_idx
  ON boat_cabins(organization_id, boat_id, label)
  WHERE is_active;

CREATE INDEX boat_cabin_berths_org_boat_idx
  ON boat_cabin_berths(organization_id, boat_id, sort_order);

CREATE UNIQUE INDEX boat_cabin_berths_cabin_label_active_idx
  ON boat_cabin_berths(organization_id, cabin_id, berth_label)
  WHERE is_active;

CREATE UNIQUE INDEX boat_cabin_berths_display_label_active_idx
  ON boat_cabin_berths(organization_id, boat_id, display_label)
  WHERE is_active;

CREATE UNIQUE INDEX trip_cabin_assignments_one_active_guest_idx
  ON trip_cabin_assignments(trip_guest_id)
  WHERE unassigned_at IS NULL;

CREATE UNIQUE INDEX trip_cabin_assignments_one_active_berth_per_trip_idx
  ON trip_cabin_assignments(trip_id, berth_id)
  WHERE unassigned_at IS NULL;

CREATE INDEX trip_cabin_assignments_org_trip_idx
  ON trip_cabin_assignments(organization_id, trip_id);

CREATE INDEX trip_cabin_assignments_berth_active_idx
  ON trip_cabin_assignments(berth_id)
  WHERE unassigned_at IS NULL;
```

### Layout Creation Formats

#### Range Generator

```json
{
  "ranges": [
    { "from": 1, "to": 10, "berths": ["A", "B"], "deck": "Lower" },
    { "from": 11, "to": 12, "berths": ["A", "B", "C"], "deck": "Main" }
  ]
}
```

The server previews generated cabins and berths before save.

#### Paste Parser

Use one cabin per row. Accepted grammar:

```text
<cabin_label>,<berth_label>[,<berth_label>...]
```

Examples:

```text
1,A,B
2,A,B
3,A
Suite 4,A,B
Upper 5,Port,Starboard
```

Whitespace around tokens is trimmed. Blank rows are ignored. Ambiguous
space-only berth lists like `1 AB` are rejected with row numbers so
staff can fix the input.

#### CSV Upload

CSV is one berth per row with this schema:

```csv
cabin_label,berth_label,deck,sort_order,notes
1,A,Lower,10,
1,B,Lower,11,
2,A,Lower,20,
2,B,Lower,21,
Suite 4,A,Main,40,Convertible twin
Suite 4,B,Main,41,Convertible twin
```

Required columns:

- `cabin_label`
- `berth_label`

Optional columns:

- `deck`
- `sort_order`
- `notes`

The UI must show this schema hint and a sample before upload. The
server validates and previews the parsed result before saving.

### Store Layer

Create `internal/store/cabins.go` with:

- `BoatCabin`
- `BoatCabinBerth`
- `CabinLayout`
- `CabinLayoutPreview`
- `CabinLayoutInput`
- `TripCabinAssignment`
- `TripCabinBoard`
- `TripCabinOccupancy`

Store methods:

- `BoatCabinLayout(ctx, orgID, boatID)`
- `PreviewCabinLayout(ctx, orgID, boatID, input)`
- `ReplaceBoatCabinLayout(ctx, orgID, boatID, actorID, input)`
- `AddBoatCabin(ctx, orgID, boatID, actorID, input)`
- `UpdateBoatCabin(ctx, orgID, boatID, cabinID, actorID, input)`
- `DeactivateBoatCabin(ctx, orgID, boatID, cabinID, actorID)`
- `UpdateBoatBerth(ctx, orgID, boatID, berthID, actorID, input)`
- `DeactivateBoatBerth(ctx, orgID, boatID, berthID, actorID)`
- `TripCabinBoard(ctx, orgID, tripID, now)`
- `AssignTripGuestBerth(ctx, orgID, tripID, tripGuestID, berthID, actorID, notes)`
- `UnassignTripGuestBerth(ctx, orgID, tripID, tripGuestID, actorID)`
- `UnassignTripGuestBerthForRevoke(ctx, orgID, tripID, tripGuestID, actorID)`

Important checks:

- `ReplaceBoatCabinLayout` rejects with conflict if active assignments
  reference any berth on the boat.
- Assignment verifies the trip belongs to org, the trip's boat matches
  the berth's boat, and the `trip_guest` belongs to the trip.
- Assignment rejects revoked guests and inactive berths.
- Assignment snapshots cabin, berth, and display labels from server
  state.
- `AddTripGuest` enrollment must validate `berth_id` and create the
  assignment in the same logical flow as the guest invite.
- Existing guests without assignments are visible as `needs_cabin` until
  assigned.
- `internal/testdb/testdb.go` truncates `trip_cabin_assignments`,
  `boat_cabin_berths`, and `boat_cabins` before trip, guest, and boat
  tables.

### Import Result Payload

Extend import job result storage/read model so the UI can link newly
created or unconfigured boats to layout setup.

Recommended shape:

```json
{
  "created_boats": [
    { "id": "uuid", "name": "Gaia Love", "active_berth_count": 0 }
  ],
  "unconfigured_boats": [
    { "id": "uuid", "name": "Gaia Love", "active_berth_count": 0 }
  ]
}
```

This can be implemented as a JSONB `result_payload` column on
`import_jobs` or as a computed response that joins imported boat IDs to
the cabin layout summary. The sprint implementation should choose the
simplest durable path and cover it with tests.

### HTTP API

Boat layout endpoints:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/boats/{id}/cabins` | GET | Admin or assigned CD for boat | Read boat cabin layout. |
| `/api/admin/boats/{id}/cabins/preview` | POST | Admin or assigned CD for boat | Preview range, paste, or CSV-derived layout. |
| `/api/admin/boats/{id}/cabins` | PUT | Admin or assigned CD for boat | Replace unassigned layout from approved input. |
| `/api/admin/boats/{id}/cabins` | POST | Admin or assigned CD for boat | Add one cabin manually. |
| `/api/admin/boats/{id}/cabins/{cabin_id}` | PATCH | Admin or assigned CD for boat | Edit cabin label/deck/notes/order. |
| `/api/admin/boats/{id}/cabins/{cabin_id}` | DELETE | Admin or assigned CD for boat | Deactivate cabin if safe. |
| `/api/admin/boats/{id}/cabins/{cabin_id}/berths/{berth_id}` | PATCH | Admin or assigned CD for boat | Edit berth label/notes/order. |
| `/api/admin/boats/{id}/cabins/{cabin_id}/berths/{berth_id}` | DELETE | Admin or assigned CD for boat | Deactivate berth if safe. |

Trip assignment endpoints:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/cabins` | GET | Admin or assigned CD | Read trip cabin board. |
| `/api/admin/trips/{id}/guests` | POST | Admin or assigned CD | Add guest and required `berth_id`. |
| `/api/admin/trips/{id}/guests/{guest_id}/cabin-assignment` | PUT | Admin or assigned CD | Assign or move guest to a berth. |
| `/api/admin/trips/{id}/guests/{guest_id}/cabin-assignment` | DELETE | Admin or assigned CD | Unassign guest berth. |

In these routes, `{guest_id}` means `trip_guest_id`, consistent with
Sprint 014 and Sprint 015.

Assigned Cruise Director access to boat layout endpoints is allowed if
the boat has at least one trip assigned to that director. Admin access
remains org-wide.

## Implementation Plan

### Phase 1: Schema and Store (~30%)

**Files:**

- `internal/store/migrations/0014_cabin_layouts.sql`
- `internal/store/cabins.go`
- `internal/store/cabins_test.go`
- `internal/testdb/testdb.go`
- `internal/store/import_jobs.go`

**Tasks:**

- [x] Add cabin, berth, and soft-audited assignment tables.
- [x] Add active-only uniqueness for cabin labels, berth labels, and
      display labels.
- [x] Add active assignment uniqueness for `trip_guest_id` and
      `(trip_id, berth_id)`.
- [x] Add range, paste, and CSV parsing/preview helpers.
- [x] Add layout replacement and per-row edit/deactivate helpers.
- [x] Add trip cabin board and assignment helpers.
- [x] Add auto-unassign helper for guest revoke.
- [x] Extend import job result payload/read model for created and
      unconfigured boat links.
- [x] Add store tests for parser validation, tenant isolation,
      duplicate labels, assignment conflicts, revoke auto-unassign,
      inactive berths, and destructive layout guards.

### Phase 2: HTTP API and Authorization (~20%)

**Files:**

- `internal/httpapi/cabin_handlers.go`
- `internal/httpapi/guest_manifest_handlers.go`
- `internal/httpapi/import_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/cabin_handlers_test.go`
- `internal/httpapi/guest_registration_test.go`

**Tasks:**

- [x] Add boat layout endpoints with Admin and assigned-CD access.
- [x] Add trip cabin board endpoint.
- [x] Require `berth_id` when adding a guest to a trip.
- [x] Wire guest revoke to auto-unassign active berth.
- [x] Return clear 400/403/404/409 errors for invalid input, forbidden
      access, missing rows, and conflicts.
- [x] Add tests for Admin access, assigned Director access, unassigned
      Director denial, guest session denial, required berth enrollment,
      and revoke freeing the berth.

### Phase 3: Fleet Layout UI and Import Setup (~20%)

**Files:**

- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/admin/pages/BoatDetail.tsx`
- `web/src/admin/pages/BoatCabins.tsx`
- `web/src/admin/pages/Fleet.tsx`
- `web/src/admin/pages/ImportJob.tsx`
- `web/src/admin/pages/ImportSpreadsheet.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [x] Add cabin layout API types and methods.
- [x] Add Fleet > Boat > Cabins tab.
- [x] Build range generator with preview.
- [x] Build paste input with row-level validation output.
- [x] Build CSV upload with visible schema hint and sample.
- [x] Build cabin/berth edit/deactivate UI.
- [x] Surface "layout not configured" state on fleet and boat detail.
- [x] Show post-import setup links for created/unconfigured boats.
- [x] Apply `DESIGN.md` typography, spacing, color, and density rules.

### Phase 4: Trip Enrollment and Cabin Board UI (~20%)

**Files:**

- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/pages/TripGuestDetail.tsx`
- `web/src/admin/pages/TripCabins.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [x] Add trip cabin board API types and methods.
- [x] Require berth selection in the Add Guest form.
- [x] Show no-layout state before guest enrollment with a setup link
      for users allowed to edit layout.
- [x] Add manifest assignment column and `needs_cabin` state for legacy
      guests.
- [x] Add route `/admin/trips/:id/cabins`.
- [x] Build cabin board grouped by cabin and berth.
- [x] Add assign, move, unassign, and quick-swap controls.
- [x] Show and edit assignment on guest detail page.
- [x] Apply `DESIGN.md` typography, spacing, color, and density rules.

### Phase 5: Product Docs and Verification (~10%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-016.md`

**Tasks:**

- [x] Update personas: Admin and Cruise Director cabin layout/assignment
      responsibilities.
- [x] Replace the deferred cabin decision with the Sprint 016 model.
- [x] Update Fleet and Manifest user stories to require berth assignment
      at guest enrollment.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `npm run build` in `web`.
- [x] Run `git diff --check`.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0014_cabin_layouts.sql` | Create | Cabin, berth, and assignment schema. |
| `internal/store/cabins.go` | Create | Store helpers for layout and assignments. |
| `internal/store/cabins_test.go` | Create | Store-level tests. |
| `internal/testdb/testdb.go` | Modify | Test truncation order. |
| `internal/store/import_jobs.go` | Modify | Import result payload for setup links. |
| `internal/httpapi/cabin_handlers.go` | Create | Cabin layout and trip board handlers. |
| `internal/httpapi/guest_manifest_handlers.go` | Modify | Required berth on add guest and revoke unassign. |
| `internal/httpapi/import_handlers.go` | Modify | Return created/unconfigured boat layout links. |
| `internal/httpapi/httpapi.go` | Modify | Route cabin endpoints. |
| `internal/httpapi/cabin_handlers_test.go` | Create | HTTP behavior and authorization tests. |
| `web/src/admin/api.ts` | Modify | Cabin API types and client methods. |
| `web/src/main.tsx` | Modify | Fleet cabins and trip cabins routes. |
| `web/src/admin/pages/BoatDetail.tsx` | Modify | Add Cabins tab. |
| `web/src/admin/pages/BoatCabins.tsx` | Create | Boat layout management UI. |
| `web/src/admin/pages/Fleet.tsx` | Modify | Layout configured/unconfigured state. |
| `web/src/admin/pages/ImportJob.tsx` | Modify | Post-import cabin setup links. |
| `web/src/admin/pages/ImportSpreadsheet.tsx` | Modify | Spreadsheet import cabin setup links. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Required berth on add guest and assignment column. |
| `web/src/admin/pages/TripGuestDetail.tsx` | Modify | Show/edit guest assignment. |
| `web/src/admin/pages/TripCabins.tsx` | Create | Trip cabin board. |
| `web/src/styles/app.css` | Modify | Cabin editor and board styling. |
| `docs/product/personas.md` | Modify | Role boundary update. |
| `docs/product/organization-admin-user-stories.md` | Modify | Cabin model and stories. |

## Definition of Done

- [x] Boat layouts can be generated by range and saved after preview.
- [x] Boat layouts can be pasted and saved after preview.
- [x] Boat layouts can be uploaded by CSV with visible schema guidance.
- [x] Admins and assigned Cruise Directors can view/edit boat layouts
      within their permitted scope.
- [x] Import success flows link created/unconfigured boats to cabin
      setup.
- [x] Adding a guest to a trip requires an active berth assignment.
- [x] Admins and assigned Cruise Directors can view the trip cabin
      board.
- [x] Admins and assigned Cruise Directors can move and unassign guest
      berths later.
- [x] Manifest and guest detail show cabin/berth assignment.
- [x] Revoking a guest frees their active berth assignment.
- [x] Store helpers enforce org isolation and trip/guest/boat ownership.
- [x] Assignment conflicts return clear 409 responses.
- [x] Destructive layout edits with dependent assignments are blocked.
- [x] Cruise Directors cannot access unassigned trip cabin boards or
      unrelated boat layouts.
- [x] Guest sessions cannot access cabin endpoints.
- [x] Product docs reflect the new cabin model and role boundaries.
- [x] `go test ./...` passes.
- [x] `go vet ./...` passes.
- [x] `npm run build` passes.
- [x] `git diff --check` passes.

## Security Considerations

- Every new tenant-owned table includes `organization_id`.
- Boat layout handlers verify boat ownership and director assignment
  scope.
- Trip assignment handlers verify trip ownership, trip boat, berth boat,
  and `trip_guest` membership.
- Server derives and snapshots display labels; client-provided labels
  are never trusted for assignment history.
- Cabin assignments expose rooming details and remain staff-only in this
  sprint.
- CSV upload is parsed server-side with strict size, header, and row
  validation.

## Dependencies

- Sprint 014 guest manifest and guest registration.
- Sprint 015 per-guest detail/folio navigation.
- Existing fleet/import and Cruise Director trip-assignment
  authorization.
- `DESIGN.md` for all new UI decisions.

## Risks and Mitigations

- **Layout edits can affect multiple trips.** Guard destructive bulk
  replacement once assignments exist; use per-row edits with dependency
  checks.
- **Directors can edit reusable layout.** Scope Director layout edits to
  boats connected to assigned trips and audit the actor on assignment
  changes.
- **CSV/paste input can be messy.** Use strict preview validation with
  row numbers and a visible CSV schema hint.
- **Existing guests may lack assignments.** Surface `needs_cabin` and
  require assignment before future enrollment-critical actions.
- **Importer source identity can change.** Preserve layouts on existing
  boat rows and surface any new/unconfigured boat as needing setup.

## References

- `CLAUDE.md`
- `DESIGN.md`
- `docs/sprints/SPRINT-014.md`
- `docs/sprints/SPRINT-015.md`
- `docs/sprints/drafts/SPRINT-016-INTENT.md`
- `docs/sprints/drafts/SPRINT-016-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-016-CODEX-DRAFT-CLAUDE-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-016-MERGE-NOTES.md`
