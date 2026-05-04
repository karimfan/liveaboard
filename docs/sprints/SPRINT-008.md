# Sprint 008: Admin Chrome — Real Data + RBAC

## Overview

Sprint 007 picked Option A ("Control Tower") and shipped a clickable
mockup at `/admin/*` driven by hardcoded data. This sprint replaces the
mock with live data for the parts of the model that already exist
(orgs, boats, trips, users), wires Site Director RBAC into the existing
chrome (subset of sidebar items + server-scoped data), and lays the
director-assignment column the Site Director scoping depends on. Two
domains stay as stubs because their schema does not exist yet
(Catalog, Inventory).

This is a tighter, more focused implementation sprint than 005/006:
the IA is locked, the mockup compiles. We swap mock arrays for `fetch`
calls and add a thin admin endpoint surface behind the existing
`RequireOrgAdmin` middleware.

## Scope (live in this sprint)

- **Overview** — real setup completeness counts (currency, boats,
  trips, users) sourced from a single `GET /api/admin/overview`.
  "Trips needing attention" lists planned trips with no Site Director
  assigned. Low-stock alerts and recent activity are stubbed (no
  schema yet).
- **Fleet** — live list and detail. Boat detail's `Trips` tab uses
  real trip data; `Inventory` tab shows an empty state ("Coming next
  sprint"); `Notes` tab stays a placeholder.
- **Trips** — live cross-fleet list with date-range filter and a
  status filter. Director column shows assigned name or "Unassigned".
- **Users** — live list of org members (admin + site directors)
  including pending Clerk invitations.
- **Organization** — live read of name + currency, plus
  `PATCH /api/organization` to save changes.

## Scope (stubs; reach in Sprint 009+)

- **Catalog** — page renders a "schema lands in Sprint 009" callout
  plus the design language so the stub looks intentional. No backing
  endpoint.
- **Reports** — same shape as today (cards naming what each report
  will be). No new aggregations.
- **Boat → Inventory tab** — empty state, same reason.

## RBAC

- **Org Admin** sees the full chrome (7 sidebar items).
- **Site Director** sees a subset: `Overview` + `Trips` only. Every
  admin-only route (`/admin/organization`, `/admin/fleet`,
  `/admin/catalog`, `/admin/users`, `/admin/reports`) returns 403 from
  the API and redirects to `/admin` on the client.
- The Site Director's `/admin/trips` is filtered server-side to only
  the trips they are assigned to. They see no other org's trips.
- `useMe()` React context provides `{ user, role, isLoaded }`; the
  sidebar reads `role` to decide which items to render. The chrome
  hides nav items, but the security boundary is the API.

## Schema

Migration `0006_trip_site_director.sql`:

```sql
ALTER TABLE trips
    ADD COLUMN site_director_user_id uuid NULL
    REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX trips_site_director_user_id_idx
    ON trips(site_director_user_id);
```

Nullable because an unassigned trip is the default state and the
"Trips needing attention" Overview card depends on it.

## API surface

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/admin/overview` | RequireOrgAdmin | Setup completeness counts + trips-needing-attention list. |
| GET | `/api/admin/boats` | RequireOrgAdmin | List boats for org. |
| GET | `/api/admin/boats/{id}` | RequireOrgAdmin | Single boat. |
| GET | `/api/admin/boats/{id}/trips` | RequireOrgAdmin | Trips for that boat. |
| GET | `/api/admin/trips` | RequireSession | All trips for org if admin; trips assigned to me if site_director. |
| PATCH | `/api/admin/trips/{id}` | RequireOrgAdmin | Update site_director_user_id (more fields later). |
| GET | `/api/admin/users` | RequireOrgAdmin | List org members. |
| PATCH | `/api/organization` | RequireOrgAdmin | Update org name + currency. |

The trip-list endpoint splits the policy: a Site Director hits the
same URL but the handler scopes the query by their user id. Server-
side scoping, not client-side filtering.

## Definition of Done

- [ ] Migration `0006_trip_site_director.sql` applies cleanly.
- [ ] All 8 endpoints land with handler tests.
- [ ] `useMe()` context exposes role; sidebar adjusts; admin-only
      routes redirect Site Directors to `/admin`.
- [ ] Overview, Fleet (list + detail with Trips tab), Trips (cross-
      fleet), Users, Organization use live data.
- [ ] Catalog, Reports, Boat → Inventory show stub messaging that's
      visually consistent with DESIGN.md.
- [ ] Backend tests cover RBAC: an admin endpoint hit with a
      site_director session returns 403; trips list scoping is
      verified for both roles.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm run build`
      all clean.

## Out of Scope

- Manifest editing (US-4.5/4.6). Schema doesn't exist.
- Catalog CRUD. Sprint 009.
- Per-boat inventory tracking. Sprint 009.
- Trip detail page with "Cancel trip" / "Open Manifest". Future.
- Site Director's mid-trip workflows (consumption ledger). Their own
  future sprint.
- Clerk's `<UserProfile>` mounted as `/account`. Future polish.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-008.md` | Create | This document. |
| `internal/store/migrations/0006_trip_site_director.sql` | Create | Add trips.site_director_user_id. |
| `internal/store/trips.go` | Modify | Scan + queries pick up site_director_user_id; add `TripsForUser`, `AssignSiteDirector`. |
| `internal/store/boats.go` | Modify (small) | `BoatByID` for the admin endpoint URL shape. |
| `internal/store/users.go` | Modify | `UsersForOrg` helper. |
| `internal/store/organizations.go` | Modify | `UpdateOrganizationProfile(name, currency)` helper. |
| `internal/httpapi/admin.go` | Create | All `/api/admin/*` handlers. |
| `internal/httpapi/httpapi.go` | Modify | Mount the admin route group. |
| `internal/httpapi/admin_test.go` | Create | Endpoint tests covering RBAC. |
| `web/src/admin/api.ts` | Create | Typed fetch wrappers (replaces mock arrays). |
| `web/src/admin/useMe.tsx` | Create | React context for current user. |
| `web/src/admin/Shell.tsx` | Modify | Read role; hide admin-only nav items. |
| `web/src/admin/pages/*.tsx` | Modify | Swap mock → API; add loading/empty/error. |
| `web/src/main.tsx` | Modify | Mount MeProvider; route guards for SD. |

## References

- `docs/sprints/SPRINT-007.md` — IA + decision matrix + roadmap.
- `docs/product/personas.md` — what each role owns.
- `docs/product/organization-admin-user-stories.md` — story coverage.
