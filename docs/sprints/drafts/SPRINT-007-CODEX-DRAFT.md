# Sprint 007 Codex Draft: Admin UX Design — 3 Candidate Experiences

## Overview

Sprint 007 should stay a design sprint, but it needs to do more than
pick a navigation pattern. The Admin surface spans seven domains with
different working rhythms: organization setup is occasional, fleet and
catalog maintenance are batch-oriented, trips are schedule-driven, and
reporting is scan-heavy. A good admin UX therefore needs a strong
primary mental model, clear escape hatches, and first-run behavior that
turns an empty org into a configured one without looking like a toy.

This draft proposes three genuinely different experiences:

1. **Control Tower**: a classic admin shell, but anchored around setup
   progress, exception queues, and dense list/detail work.
2. **Fleet Workbench**: the boat is the main object; most admin tasks
   are performed inside a selected boat workspace.
3. **Schedule Board**: the calendar is the primary surface; trips are
   the object that organizes everything else.

All three follow `DESIGN.md`: warm slate surfaces, amber accents, no
tourism styling, 8px spacing discipline, and a data-dense but
comfortable tone. All assume desktop-first behavior with a collapsible
sidebar under `768px`. All keep catalog pricing org-level while
inventory quantities are managed per boat.

## Shared Design Rules

- **Role gating**: only Org Admin sees this full surface. Future Site
  Director work should reuse components but not this IA.
- **First-run**: every option must work for a zero-data org and steer
  the first admin through org setup, fleet import/manual creation,
  catalog seeding, then trip creation.
- **Clerk fit**: account/profile actions live in an `Account` route or
  profile drawer that opens Clerk `<UserProfile>`; auth chrome is not
  reinvented.
- **Archive over delete**: boats, trips, users, and catalog items use
  archive/deactivate patterns with explicit status chips and
  confirmation states.
- **Quantities not price overrides**: catalog owns items + price once;
  each boat owns stocked quantity for each item.

## Option 1: Control Tower

### Mental Model

This is the strongest general-purpose admin UX. The left rail expresses
the seven domains clearly, while the home screen behaves like an
operations control tower: what is configured, what is missing, what
needs attention soon. From there, every domain lands in a dense
list/detail surface with bulk actions and fast scanning.

This is distinct from a generic "sidebar + tables" pitch because the
home screen is not a dead dashboard. It is a triage surface that
actively routes the admin into setup and maintenance work.

### Primary Wireframe

```text
+----------------------------------------------------------------------------------+
| Liveaboard                                                                       |
| Org: North Current Expeditions                             Search  Account       |
+----------------------+-----------------------------------------------------------+
| Overview             | Setup status                                              |
| Organization         | [85% configured]  Currency set  Fleet 3 boats  Catalog 42|
| Fleet                | Missing: 1 trip without director, 2 boats below min stock |
| Catalog              |                                                           |
| Trips                | Upcoming actions                                          |
| Users                | + Create trip      + Add boat      + Add item             |
| Reports              |                                                           |
| Archived             | Trips needing attention                                   |
|                      | --------------------------------------------------------- |
|                      | Komodo North    May 14-21   No director assigned   Fix   |
|                      | Gaia Love       Jun 02-09   Manifest 60% complete  Open  |
|                      |                                                           |
|                      | Recent changes                                            |
|                      | Low stock alerts | Pending invites | Revenue snapshot     |
+----------------------+-----------------------------------------------------------+
```

### Domain Screen Pattern

```text
+----------------------------------------------------------------------------------+
| Fleet                                                         + Add boat         |
| Search boats...  [Active v] [All trips v]                                         |
|----------------------------------------------------------------------------------|
| Name           Cabins   Berths   Upcoming trips   Stock alerts   Status          |
| Ambai II       8        16       4                2              Active          |
| Gaia Love      11       22       6                0              Active          |
| Seahorse       5        10       0                5              Active          |
|----------------------------------------------------------------------------------|
| Detail panel: Gaia Love                                                        > |
| Tabs: Overview | Cabins | Inventory | Trips | Notes                              |
| Inventory tab = quantity grid by item/category, inline editable                  |
+----------------------------------------------------------------------------------+
```

### First-Run Behavior

- `Overview` opens to a setup checklist, not empty charts.
- Primary CTA is `Add your first boat`, with secondary `Import from liveaboard.com`.
- Catalog starts with seeded categories and a prompt to add top-selling items.
- `Trips` is visually locked behind "add a boat first" until fleet exists.

### Seven-Domain Mapping

| Domain | Entry point | Core interaction |
|---|---|---|
| Org setup | `Organization` and `Overview` checklist | Edit org name, currency, defaults in a settings form |
| Fleet | `Fleet` | Dense table with row selection and detail tabs |
| Catalog | `Catalog` | Table by item/category; price edits inline or in drawer |
| Inventory quantities | `Fleet` → boat detail → `Inventory` tab | Matrix of items vs quantity for the selected boat |
| Trips | `Trips` | Table with status filters; open trip drawer/detail page |
| Users | `Users` | Invite/deactivate/resend in a membership table |
| Reporting | `Reports` | Summary cards plus sortable exception tables |

### Strengths

- Highest IA clarity: every domain has a named home.
- Best match for the current React shell and existing sidebar token.
- Easiest to scale to 50 boats and hundreds of trips because lists,
  filters, and archive states remain legible.
- Clearest role gating later because routes map cleanly to permissions.

### Risks

- Least opinionated about "where to start" unless the overview is kept
  strong and action-oriented.
- Can drift into a generic CRUD console if overview, drawers, and
  detail panes are weakly designed.

## Option 2: Fleet Workbench

### Mental Model

This experience treats the boat as the anchor object. An admin first
chooses a boat, then works inside a multi-tab workspace that combines
schedule, cabins, onboard inventory, assigned staff, and boat-specific
reporting. Organization-wide tasks still exist, but they are secondary.

This is best when operators think and speak in boats first: "show me
everything about Gaia Love" rather than "take me to trips" or "take me
to catalog."

### Primary Wireframe

```text
+----------------------------------------------------------------------------------+
| Liveaboard                                                   Boat: [Gaia Love v] |
| Fleet Workbench                                               Search  Account     |
+----------------------+-----------------------------------------------------------+
| Home                 | Gaia Love                                                 |
| Fleet Workbench      | Tabs: Schedule | Cabins | Inventory | Crew | Reports      |
| Organization         |                                                           |
| Catalog              | Schedule                                                 |
| Trip Queue           | --------------------------------------------------------- |
| Users                | May 14-21   Komodo North      Planned     Director: none |
| Reports              | Jun 02-09   Raja Ampat South  Planned     Director: Maya |
|                      | Jul 12-19   Banda Sea         Cancelled   Director: --   |
|                      |                                                           |
|                      | Right rail                                               |
|                      | Capacity 22  Active items 38  Low stock 4  Next trip 11d |
+----------------------+-----------------------------------------------------------+
```

### Organization/Fleet Split

```text
+----------------------------------------------------------------------------------+
| Organization                                                                     |
|----------------------------------------------------------------------------------|
| Org profile      Currency      Global catalog      User access      Revenue       |
|----------------------------------------------------------------------------------|
| This route handles the cross-boat concerns the workspace does not own.           |
+----------------------------------------------------------------------------------+
```

### First-Run Behavior

- Landing screen is a guided empty workspace with no boat selected.
- The first decision is `Create or import your first boat`.
- Once one boat exists, the workbench becomes the default landing page.
- Cross-org setup nags persist in an `Organization health` strip until
  currency and at least one catalog item exist.

### Seven-Domain Mapping

| Domain | Entry point | Core interaction |
|---|---|---|
| Org setup | `Organization` | Central settings page for org profile and defaults |
| Fleet | `Fleet Workbench` boat selector | Each boat is a workspace with overview + cabins |
| Catalog | `Catalog` | Global item and category management |
| Inventory quantities | `Fleet Workbench` → `Inventory` tab | Per-boat stock grid with quantity fields |
| Trips | `Fleet Workbench` → `Schedule` or `Trip Queue` | Trips are created and managed within a boat context |
| Users | `Users` and trip assignment modals | Invite and deactivate globally; assign from trip context |
| Reporting | `Reports` and boat `Reports` tab | Org summary plus per-boat operational reports |

### Strengths

- Best fit for the new inventory requirement because quantities live
  naturally inside the selected boat.
- Strong mental model for operators with a small-to-medium fleet.
- Boat detail becomes a rich workspace instead of a thin read-only page.

### Risks

- Trips across the whole org become less visible unless `Trip Queue` is
  excellent.
- User management and organization settings feel bolted on because they
  do not belong to any single boat.
- At larger scale, admins may spend too much time changing boat context.

## Option 3: Schedule Board

### Mental Model

This experience assumes the trip calendar is the operational heartbeat.
The admin lands on a multi-boat schedule board covering the next 6-12
months. Most actions start from a trip cell, a date range, or an empty
slot. Supporting entities like boats, users, and catalog remain
available, but the calendar is the main way into the system.

This is not just "Trips first." It inverts the IA so that planning
work, staffing gaps, manifest readiness, and revenue are all read from
time, not from entity lists.

### Primary Wireframe

```text
+----------------------------------------------------------------------------------+
| Liveaboard                                                    May 2026  [Month v]|
| Schedule Board                                                 + Trip  Account    |
+----------------------+-----------------------------------------------------------+
| Board                | Boats →      May          Jun          Jul                |
| Manage               | -------------------------------------------------------- |
| Organization         | Ambai II   [Trip]------  [Trip]--     [empty]            |
| Fleet                | Gaia Love  [Trip]----    [Trip]----   [Trip]----         |
| Catalog              | Seahorse   [empty]       [Trip]--     [cancelled]        |
| Users                |                                                           |
| Reports              | Right drawer                                              |
|                      | Selected: Gaia Love / Jun 02-09                           |
|                      | Status: Planned   Director: Maya                          |
|                      | Manifest: 12/22 assigned   Revenue: $4.8k planned        |
|                      | Actions: Edit | Assign director | Open manifest | Cancel  |
+----------------------+-----------------------------------------------------------+
```

### Manage Surface

```text
+----------------------------------------------------------------------------------+
| Manage                                                                            |
|----------------------------------------------------------------------------------|
| Fleet roster    Catalog    Users    Archived                                      |
| These are secondary maintenance screens used when the calendar is not enough.     |
+----------------------------------------------------------------------------------+
```

### First-Run Behavior

- Empty board shows a horizon grid with a prominent `Create first trip`
  CTA and a prerequisite callout: `You need a boat before you can
  schedule`.
- If no boats exist, the right drawer becomes a setup explainer with
  `Add boat` and `Import fleet`.
- A small persistent health banner reminds the admin about missing
  catalog or currency setup.

### Seven-Domain Mapping

| Domain | Entry point | Core interaction |
|---|---|---|
| Org setup | `Organization` | Form-based settings page outside the board |
| Fleet | `Manage` → `Fleet roster` | Supporting maintenance list; calendar uses boats as rows |
| Catalog | `Catalog` or `Manage` | Item/category management secondary to schedule |
| Inventory quantities | Trip drawer → linked boat stock, or `Fleet roster` | Quantities managed from boat context, referenced from trips |
| Trips | `Board` | Create, move, inspect, and filter trips from calendar rows |
| Users | Trip drawer and `Users` | Assign director from trip; invite/deactivate from list |
| Reporting | `Reports` and board overlays | Readiness, utilization, and revenue tied to date ranges |

### Strengths

- Best visual model for upcoming operations and trip gaps.
- Strongest support for spotting overlap, idle boats, and staffing holes.
- Feels like a true operations tool, not just a data-entry console.

### Risks

- Weakest fit for catalog and users, which become secondary screens.
- Inventory quantity management is awkward because stock is not
  inherently calendar-shaped.
- More expensive to implement well because the board interaction layer
  is custom and failure-prone.

## Decision Matrix

| Criteria | Control Tower | Fleet Workbench | Schedule Board |
|---|---|---|---|
| Information density | High | Medium-high | Medium |
| Learning curve | Low | Medium | Medium-high |
| First-run clarity | High | Medium-high | Medium |
| Fit for per-boat quantity tracking | High | Very high | Medium |
| Fit for org-wide user management | High | Medium | Medium |
| Fit for reporting/analytics | High | Medium-high | High for operational, lower for admin |
| Mobile degradation | Good | Good | Fair |
| Accessibility risk | Low-medium | Low-medium | Medium-high |
| Route/RBAC clarity | Very high | High | Medium |
| Implementation speed for Sprint 008 | Fastest | Medium | Slowest |
| Scalability to 50 boats | High | Medium | Medium-high visually, lower operationally |
| Distinctiveness | Medium | High | Very high |

## Recommendation

Recommend **Control Tower**.

It is not the most novel option, but it is the best fit for this
product's actual backlog and current codebase. The seven domains are
equally important at MVP, and only Control Tower gives each a clear
home without forcing org-wide tasks into a boat or trip abstraction.
It also handles the newly confirmed per-boat quantity model well: keep
global pricing in `Catalog`, expose per-boat quantities in a boat
detail inventory tab, and use overview alerts for low-stock conditions.

It is also the least likely to create avoidable implementation drag in
Sprint 008. The current SPA already has a dashboard shell, the design
system already defines a sidebar width, and the backend routes will map
cleanly to sectioned resources. That matters because the first
implementation sprint should ship useful admin surfaces quickly, not
burn time on a complex calendar interaction model.

If priorities change:

- Pick **Fleet Workbench** if the product identity becomes "fleet
  operations by boat" and inventory depth becomes more important than
  org-wide oversight.
- Pick **Schedule Board** if visual trip planning and capacity
  management become the dominant workflow and the team is willing to
  spend more implementation effort on custom interactions.

## Recommended Option: Experience Notes

### Navigation

- Left rail sections: `Overview`, `Organization`, `Fleet`, `Catalog`,
  `Trips`, `Users`, `Reports`, `Archived`.
- Global header: org switcher placeholder, search, account/profile.
- Detail work opens in split panes first; full-page drill-in only for
  heavier forms and trip manifests.

### Interaction Pattern

- Lists default to dense tables with sticky filters and search.
- Row click opens detail pane; primary actions stay visible in page
  headers.
- Cross-domain actions are contextual:
  `Assign director` from trip detail, `Adjust stock` from boat
  inventory, `Deactivate item` from catalog detail.

### First-Run Sequence

1. Set org name and currency.
2. Import or add first boat.
3. Seed catalog items and categories.
4. Enter per-boat stock quantities.
5. Invite Site Director.
6. Create first trip and assign director.

### DESIGN.md Compliance

- Use warm slate page background with slightly raised slate-50/slate-0
  cards; amber reserved for CTA, active nav, and warnings that need
  action.
- Typography split: General Sans for page titles, DM Sans for labels
  and forms, Geist tabular numbers in tables and metric cards.
- Density target: comfortable tables with 48px rows, clear dividers,
  and no decorative hero sections.

## Roadmap Stub for the Recommended Option

### Sprint 008: Admin Shell + Organization/Fleet Foundation

- Extend `web/src/main.tsx` with routed admin sections.
- Replace the placeholder dashboard with the `Control Tower` overview.
- Ship `Organization` settings, `Fleet` list, boat detail, and archive
  flow.
- Backend/API slice: org update, boat CRUD, cabin CRUD, archive guards.

### Sprint 009: Catalog + Boat Inventory Quantities

- Ship global catalog items/categories and pricing management.
- Add boat inventory quantity tab and low-stock indicators on overview.
- Backend/API slice: catalog CRUD, category CRUD, per-boat quantity
  records, archive/deactivate semantics.

### Sprint 010: Trips + Users + Basic Reports

- Ship trip list/detail/create/edit/cancel and Site Director
  assignment.
- Ship invite/deactivate/resend for users.
- Ship overview/reporting blocks for setup completeness, operational
  status, and per-trip revenue summary.

## Open Questions

1. Should `Reports` be a first-class left-nav item at MVP, or should
   the overview own setup completeness and operational status until
   revenue reporting exists?
2. Should `Archived` be a dedicated section, or should each domain own
   its own archived filter to reduce nav weight?
3. Should the first implementation sprint include global search, or is
   section-local search enough until the data set grows?
