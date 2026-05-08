# Sprint 016 Codex Draft: Cabin Layouts and Trip Berth Assignments

## Overview

Sprint 016 replaces the flat manifest-only model with a practical cabin
layout and berth assignment experience. Operators should not have to
manually enter `1A`, `1B`, `2A`, `2B`, and so on. Instead, each boat
gets a reusable cabin layout, generated from a small amount of structured
input, and each trip assigns guests to the generated berth slots.

The core model separates two concerns:

- **Boat cabin layout**: reusable operator-owned cabin and berth
  definitions for a boat.
- **Trip berth assignments**: trip-specific guest-to-berth assignments
  made from that boat layout.

Admins manage layout as part of adding/importing boats and from the
boat detail page. Assigned Cruise Directors can view and manage cabin
assignments for assigned trips; this draft recommends Cruise Directors
can add temporary trip notes and assignment changes, but reusable boat
layout edits should be admin-only unless the user confirms otherwise.

## Use Cases

1. **Create layout after boat import.** After importing trips from
   liveaboard.com or spreadsheet and creating a new boat, the Admin is
   prompted to configure that boat's cabin layout.
2. **Generate common layouts quickly.** Admin enters cabin ranges such
   as cabins `1-10` with berths `A,B` and cabins `11-12` with berths
   `A,B,C`; the app previews generated slots before save.
3. **Paste or upload layout rows.** Admin can paste text/CSV-like cabin
   rows such as `1, A B` or `Suite 4, A,B` and preview the generated
   cabins/berths.
4. **Manually adjust exceptions.** Admin edits individual cabins,
   berth labels, deck, sort order, and notes after bulk generation.
5. **View layout on boat detail.** Admin opens Fleet > Boat > Cabins to
   see cabin count, berth count, decks, and whether trips currently
   reference the layout.
6. **Assign guests on a trip.** Admin or assigned Cruise Director opens
   the trip manifest and assigns each active guest to an available berth.
7. **Support shared cabins.** Two guests can be assigned to Cabin `1`
   through distinct berths `1A` and `1B`; conflicts occur only when two
   active guests occupy the same berth on the same trip.
8. **Show occupancy.** Manifest and cabin board show available,
   occupied, unassigned, and overbooked/conflict states.
9. **Handle post-upload layout changes.** Admin can edit the boat layout
   later, but cannot delete or relabel berths that are assigned on any
   upcoming/active trip without first resolving dependent assignments.
10. **Director operational change.** Assigned Cruise Director can swap
    guest berth assignments for their trip without affecting the boat
    template.

## Architecture

### Core Rules

- Layout belongs to a boat and is operator-owned. Import refreshes must
  never overwrite layout rows.
- Assignment belongs to a trip and a `trip_guest`.
- Assignment target is a berth, not a cabin.
- One active, non-revoked guest can occupy one berth per trip.
- One active, non-revoked guest can have at most one cabin assignment
  per trip.
- Layout stores cabin and berth labels separately, with a generated
  `display_label` such as `1A`.
- Cabin labels are not forced numeric; examples: `1`, `Suite 2`,
  `Upper 4`, `Owner`.
- Berth labels are not forced single letters; examples: `A`, `B`,
  `Port`, `Starboard`, `Queen`.
- A berth can be marked inactive instead of physically deleted when
  historical or upcoming assignments reference it.
- Completed trips preserve readable assignment labels even if the boat
  layout changes later.
- Staff route authorization is not enough by itself; store helpers must
  scope every layout/assignment operation by `organization_id`.
- Guests do not receive cabin visibility in this sprint.

### Recommended Permission Model

| Capability | Org Admin | Assigned Cruise Director | Guest |
|---|---:|---:|---:|
| Configure reusable boat layout | Yes | No by default | No |
| View boat layout in fleet | Yes | No, unless exposed through assigned trip | No |
| View assigned trip cabin board | Yes | Yes | No |
| Assign/unassign trip guests to berths | Yes | Yes | No |
| Resolve assignment conflicts | Yes | Yes for assigned trip | No |
| Delete/relabel assigned berths | Admin only, guarded | No | No |

The user asked that "admins / site directors" can change the cabin
layout post boat upload. If interpreted literally, the implementation
can allow assigned Cruise Directors to perform boat layout edits through
assigned trips. This draft recommends tightening that to **trip
assignments for directors, reusable layout edits for admins**, because
layout is fleet-wide and changing it can affect future trips. This is an
interview question.

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
  updated_at timestamptz not null default now(),
  unique (organization_id, boat_id, label)
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
  updated_at timestamptz not null default now(),
  unique (organization_id, cabin_id, berth_label),
  unique (organization_id, boat_id, display_label)
)
```

```sql
trip_cabin_assignments (
  id uuid primary key default gen_random_uuid(),
  organization_id uuid not null references organizations(id) on delete cascade,
  trip_id uuid not null references trips(id) on delete cascade,
  trip_guest_id uuid not null references trip_guests(id) on delete cascade,
  boat_id uuid not null references boats(id) on delete cascade,
  cabin_id uuid null references boat_cabins(id) on delete set null,
  berth_id uuid null references boat_cabin_berths(id) on delete set null,
  cabin_label_snapshot text not null,
  berth_label_snapshot text not null,
  display_label_snapshot text not null,
  assigned_by_user_id uuid null references users(id) on delete set null,
  assigned_at timestamptz not null default now(),
  notes text null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
```

Indexes and constraints:

```sql
CREATE INDEX boat_cabins_org_boat_idx
  ON boat_cabins(organization_id, boat_id, sort_order);

CREATE INDEX boat_cabin_berths_org_boat_idx
  ON boat_cabin_berths(organization_id, boat_id, sort_order);

CREATE UNIQUE INDEX trip_cabin_assignments_one_guest_idx
  ON trip_cabin_assignments(trip_guest_id);

CREATE UNIQUE INDEX trip_cabin_assignments_one_berth_per_trip_idx
  ON trip_cabin_assignments(trip_id, berth_id)
  WHERE berth_id IS NOT NULL;

CREATE INDEX trip_cabin_assignments_org_trip_idx
  ON trip_cabin_assignments(organization_id, trip_id);
```

The `trip_cabin_assignments_one_berth_per_trip_idx` prevents two guests
from occupying the same berth. Cabin sharing works because each berth is
unique inside the cabin. The `trip_guest_id` unique index prevents a
guest from being assigned twice.

### Layout Input Formats

Implement two creation helpers first:

1. **Bulk range generator**

```json
{
  "ranges": [
    { "from": 1, "to": 10, "berths": ["A", "B"], "deck": "Lower" },
    { "from": 11, "to": 12, "berths": ["A", "B", "C"], "deck": "Main" }
  ]
}
```

2. **Paste parser**

Accepted rows:

```text
1 AB
2 AB
3 A
Suite 4 A,B
Upper 5 Port,Starboard
```

The parser should return a preview object and errors with row numbers.
CSV upload can reuse the same preview shape and can be a follow-up if
scope pressure is high.

Preview response:

```json
{
  "cabins": [
    {
      "label": "1",
      "deck": "Lower",
      "berths": [
        { "berth_label": "A", "display_label": "1A" },
        { "berth_label": "B", "display_label": "1B" }
      ]
    }
  ],
  "warnings": []
}
```

### Store Layer

Create `internal/store/cabins.go` with:

- `CabinLayout`
- `BoatCabin`
- `BoatCabinBerth`
- `TripCabinAssignment`
- `CabinLayoutInput`
- `CabinInput`
- `BerthInput`
- `TripCabinBoard`
- `TripCabinOccupancy`

Store methods:

- `BoatCabinLayout(ctx, orgID, boatID)`
- `ReplaceBoatCabinLayout(ctx, orgID, boatID, input)`
- `AddBoatCabin(ctx, orgID, boatID, input)`
- `UpdateBoatCabin(ctx, orgID, boatID, cabinID, input)`
- `UpdateBoatBerth(ctx, orgID, boatID, berthID, input)`
- `DeactivateBoatCabin(ctx, orgID, boatID, cabinID)`
- `DeactivateBoatBerth(ctx, orgID, boatID, berthID)`
- `TripCabinBoard(ctx, orgID, tripID, now)`
- `AssignTripGuestBerth(ctx, orgID, tripID, tripGuestID, berthID, actorID, notes)`
- `UnassignTripGuestBerth(ctx, orgID, tripID, tripGuestID)`

Important checks:

- Layout replacement only allowed when no upcoming/active assignment
  references any current berth, or when the replacement exactly preserves
  existing assigned berth IDs.
- Cabin and berth edits verify the boat belongs to org.
- Assignment verifies the trip belongs to org, the trip boat matches the
  berth boat, and the guest belongs to the trip.
- Assignment rejects revoked guests.
- Assignment rejects inactive berths.
- Assignment snapshots cabin/berth/display labels.
- Unassign is idempotent.

### HTTP API

Boat layout endpoints:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/boats/{id}/cabins` | GET | Org Admin | Read boat cabin layout. |
| `/api/admin/boats/{id}/cabins/preview` | POST | Org Admin | Preview generated/pasted layout. |
| `/api/admin/boats/{id}/cabins` | PUT | Org Admin | Replace layout from approved preview. |
| `/api/admin/boats/{id}/cabins` | POST | Org Admin | Add one cabin manually. |
| `/api/admin/boats/{id}/cabins/{cabin_id}` | PATCH | Org Admin | Edit cabin label/deck/notes/order. |
| `/api/admin/boats/{id}/cabins/{cabin_id}` | DELETE | Org Admin | Deactivate cabin if safe. |
| `/api/admin/boats/{id}/cabins/{cabin_id}/berths/{berth_id}` | PATCH | Org Admin | Edit berth label/notes/order. |
| `/api/admin/boats/{id}/cabins/{cabin_id}/berths/{berth_id}` | DELETE | Org Admin | Deactivate berth if safe. |

Trip assignment endpoints:

| Endpoint | Method | Role | Purpose |
|---|---|---|---|
| `/api/admin/trips/{id}/cabins` | GET | Org Admin or assigned CD | Read trip cabin board with guests and occupancy. |
| `/api/admin/trips/{id}/guests/{guest_id}/cabin-assignment` | PUT | Org Admin or assigned CD | Assign or move a guest to a berth. |
| `/api/admin/trips/{id}/guests/{guest_id}/cabin-assignment` | DELETE | Org Admin or assigned CD | Unassign guest berth. |

Use existing `authorizeManifestAccess` for trip assignment routes and
then store-level org/trip/guest/boat validation.

### Import and Boat Setup Flow

Current app does not have a first-class manual boat creation endpoint;
boats are created by liveaboard.com import or spreadsheet commit. This
sprint should improve the setup flow without inventing a full fleet CRUD
surface unless needed.

Liveaboard import:

- Job success response already includes inserted/updated counts.
- For newly inserted boats, return enough metadata for the UI to link to
  Fleet > Boat > Cabins.
- Boat detail should show an "Incomplete layout" state when a boat has
  zero active berths.

Spreadsheet import:

- Unknown vessel mapping already supports "create new".
- After commit, show created boat names and cabin layout setup links.
- Do not block trip import on missing layout.

Manual add:

- If implementing a manual "Add boat" flow in this sprint, route it
  through existing `UpsertBoat` with `source_provider='manual'` or add a
  clearer `CreateManualBoat` store method that writes a manual source
  slug. The create form should immediately continue to cabin layout
  setup.

### Frontend UX

Boat detail:

- Add Fleet > Boat > Cabins tab.
- Show summary: active cabins, active berths, unconfigured state.
- Include controls:
  - Generate by range.
  - Paste layout.
  - Edit cabin/berth rows.
  - Deactivate cabin/berth with clear dependency errors.
- Use preview-before-save for bulk operations.

Trip manifest:

- Add an Assignment column showing `Unassigned` or display label.
- Add a "Cabins" or "Assign cabins" action.
- Guest detail page should show cabin assignment and provide assign/edit
  controls if staff has permission.

Trip cabin board:

- New page or manifest sub-view at `/admin/trips/:id/cabins`.
- Group rows by cabin.
- Show berth slots and assigned guest names.
- Provide quick assignment controls:
  - Select unassigned guest for an empty berth.
  - Move guest between berths.
  - Unassign guest.
- Show unassigned guests list at top.

Cruise Director navigation:

- Directors currently cannot access Fleet due `RequireAdmin`, so their
  cabin access should live under assigned trips, not Fleet.
- Admins retain Fleet layout management.

### Product Docs

Update:

- `docs/product/personas.md`
  - Org Admin owns reusable boat layout.
  - Cruise Director owns assigned-trip cabin assignments.
  - Clarify whether Cruise Director can edit boat layout or only trip
    assignments after interview.
- `docs/product/organization-admin-user-stories.md`
  - Replace deferred cabin model decision with new product decision.
  - Update US-3 fleet stories with cabin layout setup.
  - Update US-4 manifest stories with berth assignment.

## Implementation Plan

### Phase 1: Schema, Store, and Parsers (~35%)

**Files:**

- `internal/store/migrations/0014_cabin_layouts.sql`
- `internal/store/cabins.go`
- `internal/store/cabins_test.go`
- `internal/testdb/testdb.go`

**Tasks:**

- [ ] Add cabin, berth, and trip assignment tables.
- [ ] Add uniqueness/index rules for one berth per trip and one
      assignment per guest.
- [ ] Add layout scan/view structs.
- [ ] Add layout preview/generation helpers for range and pasted text.
- [ ] Add layout replacement with dependency guards.
- [ ] Add cabin/berth edit and deactivation helpers.
- [ ] Add trip cabin board query.
- [ ] Add assign/unassign helpers with org/trip/guest/boat validation.
- [ ] Add tests for tenant isolation, duplicate berth labels,
      assignment conflicts, revoked guests, inactive berths, and
      destructive layout edit protection.

### Phase 2: HTTP API and Authorization (~20%)

**Files:**

- `internal/httpapi/cabin_handlers.go`
- `internal/httpapi/httpapi.go`
- `internal/httpapi/cabin_handlers_test.go`

**Tasks:**

- [ ] Add admin-only boat layout endpoints.
- [ ] Add assigned-trip cabin board endpoints.
- [ ] Reuse `authorizeManifestAccess` for trip assignment routes.
- [ ] Return consistent 400/403/404/409 errors for invalid input,
      forbidden access, missing rows, and assignment/layout conflicts.
- [ ] Add HTTP tests for Admin vs Cruise Director permissions.
- [ ] Add HTTP tests proving directors cannot operate unassigned trips.

### Phase 3: Fleet Layout UI and Import Prompts (~20%)

**Files:**

- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/admin/pages/BoatDetail.tsx`
- `web/src/admin/pages/BoatTabs.tsx`
- `web/src/admin/pages/BoatCabins.tsx` or split components
- `web/src/admin/pages/ImportJob.tsx`
- `web/src/admin/pages/ImportSpreadsheet.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [ ] Add cabin layout API types and client methods.
- [ ] Add Fleet > Boat > Cabins tab.
- [ ] Build range generator with preview.
- [ ] Build paste parser input with preview results and row warnings.
- [ ] Build cabin/berth edit/deactivate UI.
- [ ] Surface "layout not configured" state on boat detail/fleet list.
- [ ] Add post-import links for newly created/unconfigured boats.

### Phase 4: Trip Assignment UI (~20%)

**Files:**

- `web/src/admin/api.ts`
- `web/src/main.tsx`
- `web/src/admin/pages/TripManifest.tsx`
- `web/src/admin/pages/TripGuestDetail.tsx`
- `web/src/admin/pages/TripCabins.tsx`
- `web/src/styles/app.css`

**Tasks:**

- [ ] Add trip cabin board API types and client methods.
- [ ] Add manifest assignment column.
- [ ] Add route `/admin/trips/:id/cabins`.
- [ ] Build cabin board grouped by cabin and berth.
- [ ] Show unassigned guests list.
- [ ] Add assign, move, and unassign controls.
- [ ] Show assignment on guest detail page.
- [ ] Handle no-layout state with admin setup link and director-friendly
      message.

### Phase 5: Product Docs and Verification (~5%)

**Files:**

- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-016.md`

**Tasks:**

- [ ] Update product decision table to replace "Cabin layouts deferred".
- [ ] Update Fleet, Trip, Manifest, and Cruise Director user stories.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `npm run build` in `web`.
- [ ] Run `git diff --check`.

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/0014_cabin_layouts.sql` | Create | Cabin, berth, and assignment schema. |
| `internal/store/cabins.go` | Create | Cabin layout and trip assignment store helpers. |
| `internal/store/cabins_test.go` | Create | Store-level layout/assignment tests. |
| `internal/testdb/testdb.go` | Modify | Truncate cabin tables in dependency order. |
| `internal/httpapi/cabin_handlers.go` | Create | Boat layout and trip assignment endpoints. |
| `internal/httpapi/httpapi.go` | Modify | Route cabin endpoints. |
| `internal/httpapi/cabin_handlers_test.go` | Create | HTTP authorization and behavior tests. |
| `web/src/admin/api.ts` | Modify | Cabin API types and methods. |
| `web/src/main.tsx` | Modify | Fleet cabins and trip cabins routes. |
| `web/src/admin/pages/BoatDetail.tsx` | Modify | Add Cabins tab and summary. |
| `web/src/admin/pages/BoatTabs.tsx` | Modify | Split or link to cabins tab. |
| `web/src/admin/pages/BoatCabins.tsx` | Create | Boat layout management UI. |
| `web/src/admin/pages/TripManifest.tsx` | Modify | Show assignment and link to cabin board. |
| `web/src/admin/pages/TripGuestDetail.tsx` | Modify | Show/edit guest assignment. |
| `web/src/admin/pages/TripCabins.tsx` | Create | Trip-level cabin board. |
| `web/src/admin/pages/ImportJob.tsx` | Modify | Link new boats to layout setup. |
| `web/src/admin/pages/ImportSpreadsheet.tsx` | Modify | Link created boats to layout setup. |
| `web/src/styles/app.css` | Modify | Cabin board and layout editor styles. |
| `docs/product/personas.md` | Modify | Update responsibility boundaries. |
| `docs/product/organization-admin-user-stories.md` | Modify | Update cabin stories and decisions. |

## Definition of Done

- [ ] Boat cabin layouts can be generated by range and saved after
      preview.
- [ ] Boat cabin layouts can be pasted as text and saved after preview.
- [ ] Admins can view, edit, and deactivate cabins/berths from boat
      detail.
- [ ] Import success flows direct Admins to configure cabin layout for
      newly created/unconfigured boats.
- [ ] Admins and assigned Cruise Directors can view a trip cabin board.
- [ ] Admins and assigned Cruise Directors can assign, move, and
      unassign guests to berth slots.
- [ ] Manifest and guest detail show cabin/berth assignment.
- [ ] Store helpers enforce org isolation and trip/guest/boat ownership.
- [ ] Assignment conflicts are rejected with clear 409 responses.
- [ ] Layout edits that would orphan active/upcoming assignments are
      blocked or converted to inactive rows with preserved snapshots.
- [ ] Cruise Directors cannot access unassigned trip cabin boards.
- [ ] Guests cannot access cabin endpoints.
- [ ] Product docs reflect the new cabin model and role boundaries.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `npm run build` passes.
- [ ] `git diff --check` passes.

## Security Considerations

- All new tables include `organization_id`.
- Boat layout endpoints must verify boat ownership by org.
- Trip assignment endpoints must verify trip ownership, trip boat, and
  `trip_guest` membership.
- Cruise Directors must be scoped through assigned trips only.
- Guest sessions cannot call admin cabin endpoints.
- Cabin assignment labels can reveal rooming details; keep them out of
  guest-facing APIs for this sprint.
- Avoid trusting client-provided display labels; server derives and
  snapshots labels from canonical cabin/berth rows.

## Risks and Mitigations

- **Risk: reusable layout edits affect existing assignments.**
  Mitigation: store trip assignment snapshots and block destructive edits
  when upcoming/active assignments reference affected berths.
- **Risk: importer creates boats but layout remains empty.**
  Mitigation: visible unconfigured state and post-import setup links.
- **Risk: scope grows into full visual deck-plan editing.**
  Mitigation: build structured cabin/berth editor first; deck-plan image
  reference is deferred.
- **Risk: Cruise Director layout editing changes future trips.**
  Mitigation: default directors to trip assignment only unless the human
  planner confirms broader layout permissions.
- **Risk: labels vary widely across boats.**
  Mitigation: support text labels and explicit sort order, not numeric
  assumptions.

## Dependencies

- Sprint 014 trip guest manifest and registration.
- Sprint 015 guest detail/folio navigation.
- Existing fleet/import and trip assignment authorization.

## Open Questions

1. Should Cruise Directors be able to edit the reusable boat layout, or
   only trip-level assignments/notes?
2. Is CSV upload required in Sprint 016, or are range generation and
   pasted text enough for the first version?
3. Should cabin assignment be required before checkout/trip completion,
   or advisory for now?
4. Should completed trips be immutable for cabin assignments, or can
   admins correct historical assignment records?
