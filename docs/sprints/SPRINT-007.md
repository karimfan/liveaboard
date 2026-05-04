# Sprint 007: Admin UX — Three Candidate Experiences

## Overview

Sprint 005 (Clerk auth) and Sprint 006 (boat + trip scraper) gave the
product its data foundation. The SPA's only authenticated screen is
still a placeholder dashboard with stat-card zeros — there is no admin
information architecture, no list views, no detail views, no forms.
Every Org Admin user story enumerated in
`docs/product/organization-admin-user-stories.md` is unrendered.

This is a **design-only sprint** (modeled after Sprint 002). The output
is three genuinely distinct candidate experiences for the Admin
surface, plus a recommendation. Sprint 008 begins the chosen direction.

The seven domains the Admin must serve:

1. **Org Setup** — name, currency/currencies, defaults (US-2.x)
2. **Fleet (Boats)** — CRUD; archive over delete (US-3.x). **Cabin
   layouts are dropped** (Phase 4 follow-up); a boat is just name +
   image + source linkage.
3. **Catalog** — top-level **org-level items + categories + prices**
   (US-5.x). The **per-boat inventory** (stock quantities for each
   item, e.g., "10 XL t-shirts on Gaia Love, 40 on Seahorse") is
   **not a top-level destination** — it lives on each boat as a tab.
   Top-level Catalog = items + prices; Boat → Inventory tab = how
   many of each item this specific boat carries.
4. **Trips** — create / edit / cancel-planned / monitor; assign
   director; pre-departure manifest (US-4.x)
5. **Users** — invite / deactivate / resend invitation (US-6.x)
6. **Reporting** — setup completeness, operational status, revenue
   per trip; cross-trip analytics deferred (US-7.x)

**Site Director UX is out of scope for Sprint 007** — it was a hint
toward a future sprint, not a deliverable here. Admin is the entire
focus.

## Phase 4 refinements (binding)

These were confirmed in the interview before Codex ran its compete
draft. They shape the entire sprint:

- **UX direction prior:** Sidebar + Tables family (Option A) is the
  planner's gut. Confirmed below as the primary recommendation.
- **Site Director scope:** **Out of scope.** Future sprint.
- **Inventory model:** **Per-boat quantity tracking.** "A boat might
  have 10 XL t-shirts, while another has 40." Pricing stays org-level
  flat (Sprint 002 product decision). Implies a future
  `boat_inventory(item_id, boat_id, qty, min_threshold)` table.
  **Inventory is not a top-level menu item** — it's a tab on each
  boat. The org-level Catalog (items + prices) is where new items are
  defined; Boat → Inventory is where each boat's quantities are
  managed.
- **Cabin layouts:** **Dropped.** Boats are not modeled with
  structured cabins. The manifest is a flat list of guests on a trip;
  any "cabin label" is a free-text field on the guest manifest entry,
  added later if/when needed. This affects US-3.3 (cabin part deferred),
  US-3.4 (cabin layout edit dropped), and US-4.5/4.6/4.7 (manifest
  cabin assignment becomes free-text or deferred).
- **Mobile:** Desktop-first. Sidebar collapses to a hamburger below
  768px. No tablet/phone primary design.

## DESIGN.md baseline (binding for all three options)

Every wireframe below is rendered against the existing tokens in
`web/src/styles/tokens.css`, in the spirit of `DESIGN.md`'s
"industrial / utilitarian" aesthetic:

| Concern | Token / value |
|---|---|
| Page background | `--c-50` (warm slate 50, `#f8f7f6`) |
| Card / surface | `white` with `1px solid --c-200` border, `--r-lg` (12px) radius |
| Body text | `--font-body` (DM Sans 400/500/600) |
| Display / headers | `--font-display` (General Sans 700) |
| Tabular numerics | `--font-mono` (Geist tabular-nums) for tables and metric values |
| Primary action | `--c-primary` (amber `#e5853b`) |
| Active nav state | `--c-primary` text on `--c-primary-subtle` background |
| Negative state | `--c-error` text on `--c-error-bg` background |
| Density | "comfortable" — table row height 48px, label-input gap 8px |
| Icons / glyphs | Text-only nav labels in MVP. Iconography lands later, after the IA is shipped. |

Rationale on glyphs: emoji and generic icons drift from the "serious,
competent, operational" tone in `DESIGN.md`. Text labels are honest
about what each route is.

---

# Option A — Control Tower

**Mental model.** "I'm operating a fleet. Show me what's configured,
what's missing, what needs my attention this week. Let me drill into
any domain to maintain it."

This is the sidebar-shell pattern, but with one key difference: the
home screen ("Overview") is an **active operational triage** screen —
setup completeness, exceptions, alerts — not a read-only dashboard
with placeholder charts.

## Information architecture

```
+----------------------------------------------------------------------+
|  Liveaboard                                                          |
|                                                                      |
|  Overview            <-- active triage (default landing)             |
|  Organization        <-- name, currency, defaults                    |
|  Fleet               <-- list of boats; click -> boat detail         |
|  Catalog             <-- org-level items + categories + prices       |
|  Trips               <-- list of trips; filters                      |
|  Users               <-- crew + pending invites                      |
|  Reports             <-- setup completeness, operational, revenue    |
|  ----                                                                |
|  Acme Diving         <-- org name (single org for now)               |
|  owner@acme.test     <-- user menu -> Account (Clerk UserProfile)    |
+----------------------------------------------------------------------+
```

Width 220px (`--sidebar-w`). Active item uses amber text on
`--c-primary-subtle`. Below 768px the sidebar collapses to a
hamburger; above 1024px it is permanently expanded.

Archive is **a per-domain filter**, not a top-level destination. Each
list view (Fleet, Trips, Catalog, Users) has a `Show: Active /
Archived / All` toggle in its header.

## Sample screen — Overview (the value prop)

```
+----------------------------------------------------------------------+
|  Overview                                                            |
|                                                                      |
|  +----------------------------+  +-------------------------------+   |
|  |  Setup completeness        |  |  Trips needing attention      |   |
|  |                            |  |                               |   |
|  |  85% configured            |  |  Komodo North     May 14-21   |   |
|  |  v Org name + currency     |  |     No director assigned      |   |
|  |  v Fleet (3 boats)         |  |                       [ Fix ] |   |
|  |  v Catalog (42 items)      |  |                               |   |
|  |  ! 2 boats below min stock |  |  Gaia Love        Jun 02-09   |   |
|  |  ! 1 trip without director |  |     Manifest 60% complete     |   |
|  |                            |  |                       [Open]  |   |
|  +----------------------------+  +-------------------------------+   |
|                                                                      |
|  +----------------------------+  +-------------------------------+   |
|  |  Low-stock alerts          |  |  Recent activity              |   |
|  |  Gaia Love     5 items low |  |  Boat "Seahorse" added        |   |
|  |  Seahorse      9 items low |  |  Item "T-shirt M" price set   |   |
|  |  Open inventory            |  |  Director "Maya" invited      |   |
|  +----------------------------+  +-------------------------------+   |
+----------------------------------------------------------------------+
```

Components:
- `Setup completeness` is computed (currency set, fleet ≥ 1, catalog
  ≥ 1 item, every active boat has stock for every active item, every
  active trip has a director). It is the first-run primary surface
  and slowly fades as the operator configures more.
- `Trips needing attention` is the operational triage primitive:
  trips in the next 30 days with a missing director or low manifest
  fill ratio. Each row is one click to fix the issue.
- `Low-stock alerts` aggregates per-boat inventory rows where
  `qty < min_threshold` (a future column on `boat_inventory`).

Typography: section headers in DM Sans 600 18px; body in DM Sans 400
14px; numerics (counts, percentages) in Geist tabular-nums.

## Sample screen — Boat detail

```
+----------------------------------------------------------------------+
|  Fleet > Gaia Love                                  [ ... Actions ]  |
|                                                                      |
|  +-----------+   Gaia Love                          [Re-sync source] |
|  |  [image]  |   liveaboard.com/diving/indonesia/gaia-love           |
|  |           |   Last synced  May 3, 2026 18:42                      |
|  +-----------+   Status  Active   v                                  |
|                                                                      |
|  Tabs:  Trips  |  Inventory  |  Notes                                |
|  ------------------------------------------------------------------  |
|  Inventory                                       [+ Add stock entry] |
|                                                                      |
|  Item                Category    On hand   Min   Low    Synced       |
|  T-shirt XL          Apparel        10      5     -     May 1        |
|  Mask defog 30ml     Consumables    27     20     -     May 1        |
|  Aluminum tank       Equipment       8     12     !     May 1        |
|  ...                                                                 |
+----------------------------------------------------------------------+
```

The Inventory tab is the **per-boat quantity grid**. Inline-editable
numeric inputs; a `!` chip in the `Low` column when below threshold.
Org-level prices live on the catalog item, not here — this view is
about *quantity stocked on this boat*.

## Seven-domain mapping

| Domain | Where in IA |
|---|---|
| Org setup | `Organization` (form for name, currency, defaults) |
| Boats | `Fleet` (list) → boat detail (Trips / Inventory / Notes tabs) |
| Catalog (org items + prices) | `Catalog` (top-level) — org-level items + categories + prices |
| Inventory (per-boat quantities) | **Not top-level.** `Fleet > Boat > Inventory` tab — quantity grid for that boat |
| Trips (cross-fleet) | `Trips` (top-level) — every upcoming trip across all boats, chronological; filters: boat, status, date |
| Trips (this boat) | `Fleet > Boat > Trips` tab — same trip data, scoped to one boat |
| Users | `Users` (crew + pending invites); invite = modal; assign-to-trip happens on Trip detail |
| Reporting | `Reports` (setup completeness, operational status, revenue per trip) |
| Account / profile | Sidebar footer → opens Clerk `<UserProfile>` |

## First-run state

Empty Overview shows a "Get started" sequence as the only content,
with Setup completeness at 0%:

```
+----------------------------------------------------------------------+
|  Welcome to Liveaboard.                                              |
|                                                                      |
|  Get your organization ready for trips.                              |
|                                                                      |
|  1.  Set your organization currency       [ Set currency ]           |
|  2.  Add or import your first boat        [ Add boat ]  [ Import ]   |
|  3.  Seed your catalog                    (gated until step 2)       |
|  4.  Set per-boat inventory               (gated until step 3)       |
|  5.  Invite a Site Director               [ Invite ]                 |
|  6.  Create your first trip               (gated until step 2)       |
+----------------------------------------------------------------------+
```

Each unlocked step is a primary CTA in amber; each locked step is
muted with a "needs N first" hint.

## Pros / cons

**Pros**

- Clean seven-domain mapping — every story has one obvious home.
- Per-boat **quantity grids** fit naturally inside boat detail's
  Inventory tab; low-stock alerts roll up to Overview.
- Active Overview is the right product surface for a configuration-
  heavy MVP. Every empty/at-risk state is one click from a fix.
- Reuses the existing sidebar shell + tokens. Sprint 008 ships
  visible value the first day.
- Deep-linkable: every list / detail / sub-tab has a URL.
- Lowest implementation cost of the three options.

**Cons**

- Most familiar / least distinctive of the three. Doesn't reinforce
  any product-specific mental model (boat-first or schedule-first).
- Cross-boat *visual* overview (e.g., "where are my boats this
  week?") is weaker than Option C's calendar.
- Tables compress poorly on narrow screens; mobile fit is acceptable
  but not delightful.

---

# Option B — Fleet Workbench

**Mental model.** "Each boat is a workspace I open up. Most things I
do are scoped to a single boat."

The primary nav is a **boat selector** in the header; inside the
selected boat is a tabbed workspace. Org-wide concerns (settings,
catalog library, all-org users, all-org reports) live behind a
separate `Organization` route.

## Information architecture

```
+----------------------------------------------------------------------+
|  Liveaboard           Boat: [ Gaia Love     v ]    Organization   ▾  |
|                                                          owner@acme  |
+----------------------------------------------------------------------+
|  Trips | Inventory | Reports | Notes                                 |
|  -------------------------------------------------------------       |
|                                                                      |
|  [active tab content]                                                |
|                                                                      |
+----------------------------------------------------------------------+
```

The tabs are strictly **boat-scoped**. Trip-scoped work (Manifest,
Crew assignment) opens as a slide-over from the `Trips` tab, not as
a separate tab — because those are trip workflows, not boat
workflows. The boat's `Trips` tab is the same data as top-level
`Trips`, scoped to one boat.

## The Organization route

```
+----------------------------------------------------------------------+
|  Organization                                                        |
|                                                                      |
|  Settings   |   Fleet roster   |   Catalog library   |   Users   |   |
|  Reports                                                              |
|                                                                      |
|  [active sub-page]                                                   |
+----------------------------------------------------------------------+
```

`Fleet roster` is where boats are added or archived. `Catalog library`
holds org-level items + prices. `Users` is the cross-boat crew
list. `Reports` is org-level reporting.

## Sample screen — Boat workspace, Schedule tab

```
+----------------------------------------------------------------------+
|  Boat: Gaia Love     v                          Organization ▾       |
|  Trips | Inventory | Reports | Notes                                 |
|  ------------------------------------------------------------------  |
|  Trips                                                   [ + Trip ]  |
|                                                                      |
|  [ Q1 2027  ◂ ▸ ]                                                    |
|                                                                      |
|  FEB 2027                                                            |
|  06 - 16   Raja Ampat North & South     Maya       FULL              |
|  18 - 28   Raja Ampat North & South     —          FULL              |
|                                                                      |
|  MAR 2027                                                            |
|  04 - 14   Komodo                       Maya       2 of 8 booked     |
|  16 - 25   Komodo                       —          Available         |
+----------------------------------------------------------------------+
```

Click a row → trip detail slide-over. Click a director → assignment
modal. Manifest editing opens from a "Manifest" button inside the
slide-over.

## Seven-domain mapping

| Domain | Where in IA |
|---|---|
| Org setup | `Organization > Settings` |
| Boats | `Organization > Fleet roster` (CRUD); switch boat via top selector |
| Catalog (org items + prices) | `Organization > Catalog library` |
| Inventory (per-boat quantities) | `Boat > Inventory` tab |
| Trips (this boat) | `Boat > Trips` tab |
| Trips (cross-fleet) | `Organization > Trips` (chronological list across all boats) |
| Users | `Organization > Users`; assign director from trip slide-over |
| Reporting | `Boat > Reports` (single-boat); `Organization > Reports` (org-wide) |
| Account / profile | Header → opens Clerk `<UserProfile>` |

## First-run state

A brand-new org has no boats, so the boat selector reads
`(no boats yet)` and prompts `Add your first boat → Organization >
Fleet roster`. After the first boat lands, the user is auto-routed
into that boat's Schedule tab.

A persistent "Org health" strip across the top reminds the admin to
set currency and seed at least one catalog item, fading once
configured.

## Pros / cons

**Pros**

- Mirrors how operators actually think: most morning sentences
  start with "Let me check on Gaia Love."
- Per-boat tabs collapse the IA — Trips, Inventory, Reports for
  *this boat* are one click apart.
- Per-boat reporting (utilization, revenue, inventory turnover) is
  in the natural place.
- Inventory quantity grids live exactly where operators want them.

**Cons**

- "Where do I add a boat?" is non-obvious — adding a boat requires
  leaving the boat context to `Organization > Fleet roster`.
  Chicken-and-egg problem at first run.
- Cross-boat overviews ("show me all trips next week") require the
  Organization route — two clicks deep.
- Catalog has two homes (Org Library + per-Boat Inventory). Easy
  for users to wonder "where do I add a new item?"
- The boat selector competes with browser-tab navigation — a
  bookmark pins a specific boat, not the abstract "Schedule" view.
- Org-wide concerns feel demoted ("buried in another route"). At
  MVP, those are still load-bearing.

---

# Option C — Schedule Board

**Mental model.** "Time is the most important thing. Show me a
calendar of every trip across every boat; let me click anywhere to
act."

The default landing is a 6–12 month multi-boat trip calendar (rows =
boats, columns = months). CRUD happens via side panels triggered by
clicking cells. Secondary domains (Fleet, Catalog, Users, Reports,
Settings) live behind a `Manage ▾` menu.

## Information architecture

```
+----------------------------------------------------------------------+
|  Liveaboard      Schedule    Manage ▾                  owner@acme    |
|                                                                      |
|  [ Q1 2027  ◂ ▸ ]                Boat: All v   Status: Any v         |
|  ------------------------------------------------------------------  |
|              JAN       FEB             MAR             APR           |
|  Ambai II    [trip]    [trip][trip]    [trip][trip]    [trip]        |
|  Gaia Love   [trip]    [trip][trip]    [trip][trip]    [trip][trip]  |
|  Seahorse              [trip]          [trip]          (cancelled)   |
|                                                                      |
|  Legend:  filled=FULL   striped=Booking   open=Available   gray=Past |
+----------------------------------------------------------------------+
```

The `Manage ▾` popover hosts:

```
Manage:
  Fleet roster
  Catalog
  Users
  Reports
  Org settings
```

## Click behaviors

- Click a trip cell → right-side drawer with status, director,
  manifest fill ratio, "Open Manifest →" link.
- Click an empty cell on a boat row → slide-over: "Create trip on
  Gaia Love starting Feb 6" pre-filled.
- Click the boat name on the left axis → side panel: boat profile +
  inventory + settings (no separate boat detail page).
- `Manage ▾` items are full-page routes.

## Seven-domain mapping

| Domain | Where in IA |
|---|---|
| Org setup | `Manage > Org settings` |
| Boats | Click boat name (side panel); `Manage > Fleet roster` for list/CRUD |
| Catalog | `Manage > Catalog` |
| Inventory quantities | Boat side panel → Inventory section; or `Manage > Fleet roster > Boat > Inventory` |
| Trips | The calendar IS trips |
| Users | `Manage > Users`; invite from trip drawer's director picker |
| Reporting | `Manage > Reports`; some operational signals overlay the calendar (low-stock dot on a boat row, etc.) |
| Account / profile | Header → opens Clerk `<UserProfile>` |

## First-run state

The empty calendar shows a single overlay:

```
+--------------------------------------------------+
|  Welcome to Liveaboard.                          |
|  Add your first boat to start scheduling.        |
|                                                  |
|     [ Add boat ]      [ Import from URL ]        |
+--------------------------------------------------+
```

After the first boat lands, the calendar shows that boat's row with
zero trips and a `+` prompt in the current month. A persistent banner
nags about currency + catalog + first-trip until each is configured.

## Pros / cons

**Pros**

- Strongest visual fit for operations: "where are my boats this
  week" is the entire home screen.
- Spotting empty weeks, clashes, or staffing holes is instantaneous.
- Compact for many boats (one row each); scales to 30+ boats.
- Mobile responsive degrades naturally to a single-boat day-view.

**Cons**

- **Wrong shape for an MVP that is config-heavy.** At launch,
  operators are setting up org, boats, catalog, users, *and*
  inventory — exactly the surfaces hidden behind `Manage ▾`. Putting
  four of the seven domains behind a popover is the most expensive
  IA decision possible right now.
- Calendar logic is the heaviest implementation of the three —
  date-grid math, virtualized scrolling, drag-to-edit (future).
- Empty state is awkward — until trips exist, the calendar is blank.
- Tabular reporting (US-7.x) doesn't live on the calendar; it's a
  separate route that feels disconnected.
- Inventory quantity management is awkward to surface — stock
  doesn't have a calendar shape; goes through a boat side panel.
- Less common pattern; users may not discover `Manage ▾`.

---

# Decision Matrix

| Dimension | Control Tower (A) | Fleet Workbench (B) | Schedule Board (C) |
|---|---|---|---|
| Time to first useful screen | **Fastest** | Medium | Slowest |
| First-run / empty-state UX | **Strongest** | Weak (chicken-and-egg) | Weak (blank calendar) |
| Information density | **Highest** | High (within boat) | Medium |
| Operator mental-model fit | Generic / familiar | **Strongest** (boat-first) | Strong (time-first) |
| Cross-boat overview | Strong (Trips list) | Weak (Org route only) | **Excellent** (calendar) |
| Fit for per-boat **quantity** inventory | **Very strong** | **Very strong** | OK (via side panel) |
| Fit for org-wide users + catalog | **Strong** (top-level) | Medium (Org route) | Weak (Manage menu) |
| Mobile / tablet fit | Acceptable | Medium | **Best** (calendar collapses) |
| Discoverability of secondary features | **Highest** (always visible) | Medium (Org route) | Lowest (popover) |
| Implementation cost (Sprint 008 alone) | **Lowest** | Medium | Highest |
| Off-the-shelf component reuse | **Highest** | Medium | Medium |
| Scaling to 10+ boats | Strong | Medium | **Strongest** visually |
| Distinctiveness | Medium | High | **Very high** |

# Recommendation

**Primary: Option A — Control Tower.**

Reasoning, in priority order:

1. **Best fit for an MVP that's still configuration-heavy.** At
   launch, every admin spends most of their time setting up org,
   boats, catalog, users, *and* per-boat inventory. Five of the seven
   domains are configuration. Only Option A keeps all of them as
   first-class navigation; B demotes most to a separate route, C
   buries them behind a menu.
2. **Best fit for the per-boat quantity grid.** Both A and B handle
   this well; A handles it inside the boat detail's Inventory tab
   without the boat-context-switching tax B imposes.
3. **Active Overview as setup-triage screen.** The home screen is
   not a dead dashboard; it computes setup completeness, surfaces
   trips needing attention, and aggregates low-stock alerts. The
   value lands the moment the operator has anything configured.
4. **Cheapest first implementation sprint.** The existing Vite SPA
   already has a Dashboard shell + sidebar tokens; Sprint 008 = a
   sidebar + Overview + Fleet list + boat detail. Three small
   commits.
5. **Easiest to evolve.** Both fall-forwards (B and C) can be
   layered onto A's data model later without rewriting.

**Fall-forward 1: Option C (Schedule Board).** If, after we ship A
and accumulate operator feedback, the dominant complaint is "I just
want to see the calendar," restructure into C. The data model
doesn't change; only the chrome does.

**Fall-forward 2: Option B (Fleet Workbench).** If operators with
multiple boats are clearly siloed per-boat in their workflow,
restructure into B. Until then, A's "Boat detail" page already gives
each boat a tabbed mini-workspace inside the broader IA.

# Implementation Roadmap (for the recommended option)

## Sprint 008: Admin shell + Organization + Fleet foundation

- Replace the placeholder dashboard with the Control Tower **Overview**
  (real setup completeness, trips-needing-attention, low-stock).
- Sidebar shell + routing scaffolding (`/`, `/organization`, `/fleet`,
  `/catalog`, `/trips`, `/users`, `/reports`).
- **Organization** — settings page: name, currency.
- **Fleet** — list view + boat detail (Trips / Inventory / Notes
  tabs; Inventory tab is read-only this sprint).
- Backend: `GET /api/admin/boats`, `GET /api/admin/boats/{id}`,
  `PATCH /api/organization`, all behind `RequireOrgAdmin`.

## Sprint 009: Catalog + per-boat Inventory

- **Catalog** — list view + item create/edit + categories.
- **Boat > Inventory** — quantity grid (numeric inputs, low-stock
  threshold, save-on-blur).
- Schema: new `boat_inventory(boat_id, item_id, qty, min_threshold,
  updated_at, PK(boat_id, item_id))` table + repo.
- Overview's low-stock alerts become live.
- Backend: catalog CRUD + boat-inventory upsert/list endpoints.

## Sprint 010: Trips + Users + Reports MVP

- **Trips** — list + detail + create/edit/cancel-planned.
- Site Director assignment dropdown (uses existing `Users` data).
- **Users** — list + invite + deactivate + resend (UI on top of
  Sprint 005's `/api/invitations` endpoints).
- **Reports** — setup completeness summary, operational status table,
  per-trip revenue summary (US-7.1, 7.2, 7.3).
- Manifest editor on Trip detail (pre-departure only — Site Director
  gets mid-trip in their future sprint).

# Files (this sprint)

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-007.md` | Create | This document. |
| `docs/sprints/drafts/SPRINT-007-INTENT.md` | Create | Concentrated intent (already exists). |
| `docs/sprints/drafts/SPRINT-007-CLAUDE-DRAFT.md` | Create | Claude's initial draft. |
| `docs/sprints/drafts/SPRINT-007-CODEX-DRAFT.md` | Create | Codex's competing draft. |
| `docs/sprints/drafts/SPRINT-007-CLAUDE-DRAFT-CODEX-CRITIQUE.md` | Create | Codex's critique. |
| `docs/sprints/drafts/SPRINT-007-MERGE-NOTES.md` | Create | Synthesis. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 007. |

No code changes this sprint.

# Definition of Done

- [ ] Three concretely different admin UX experiences exist as
      sections in this doc, each with: IA diagram, ≥1 sample
      wireframe, the seven-domain mapping, first-run behavior, and
      pros/cons.
- [ ] Decision matrix scores all three across density, learning
      curve, mobile fit, time-to-first-useful-screen, scaling, and
      cost.
- [ ] A single primary recommendation with reasoning + named
      fall-forwards.
- [ ] Implementation roadmap stub for Sprints 008–010 against the
      recommendation.
- [ ] Site Director UX is **not** part of this doc (out of scope per
      Phase 4).
- [ ] Per-boat inventory is documented as **quantity tracking**, not
      checkbox availability, in every option.
- [ ] DESIGN.md compliance demonstrated (token references in
      wireframes; text-only nav labels; no decorative glyphs).
- [ ] Tracker shows Sprint 007 (`go run docs/sprints/tracker.go sync`).

# Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Picked option does not survive first contact with Sprint 008 implementation | Medium | Medium | Sprint 008 is bounded to "shell + Overview + Fleet"; pivoting between A/B/C costs at most one sprint. The data model doesn't change. |
| Operators want the schedule board long-term | Medium | Low | Option C is named as a fall-forward; A's data model + components are reusable inside a calendar shell. |
| Per-boat quantity grid feels cramped for catalogs of 100+ items | Low | Medium | Inventory tab gets pagination + filter + search at Sprint 009. |
| Admins can't find Settings / Catalog because the recommendation has 7 sidebar items | Low | Low | Density is fine at 7; common back-office tools (Linear, Stripe) carry similar breadth. |
| Mobile demos are awkward | Medium | Low | Hamburger collapse below 768px; mobile primary design is explicitly out of scope and captured as future work. |

# Security Considerations

- **Server-side role gating.** Every `/api/admin/*` endpoint mounted in
  Sprint 008+ stays behind `RequireOrgAdmin` (Sprint 005). The chrome
  hides admin-only destinations from non-admin users, but the security
  boundary is in the API.
- **Cross-tenant isolation preserved.** Every list/detail query takes
  an explicit `organization_id` (Sprint 006 pattern). The UX adds no
  new tenant boundaries.
- **Clerk hosted UI integration.** Account / profile / password change
  continue to live in Clerk's `<UserProfile>`; we do not re-implement
  account management in the admin chrome.

# Dependencies

- **Sprint 005** — Clerk auth + `RequireOrgAdmin` middleware.
- **Sprint 006** — boats + trips schema and repos.
- **Sprint 002** — Org Admin user-story backlog.
- **DESIGN.md** — binding for every wireframe.
- No new tools, libraries, or infrastructure.

# Open Questions for the Sprint 008 author

1. **`Reports` as a top-level nav at MVP?** The Overview already
   surfaces setup completeness and operational status. Recommendation:
   keep `Reports` as a top-level item but populate it sparingly until
   US-7.3 (per-trip revenue) lands; then it has real content.
2. **Multi-currency.** `organizations.currency` is a single text
   column today. The Settings UI shows a single-select for now;
   "Currencies accepted" (plural) is a schema change captured for a
   future sprint.
3. **`Account` route shape.** Sidebar footer link → modal with embedded
   `<UserProfile>`, or a full `/account` route. Recommendation:
   modal — feels lighter and matches Clerk's hosted pattern.
4. **Search at the chrome level.** A global search box (`Find
   anything`) in the sidebar header is on the wishlist but not
   required for Sprint 008. Each list view ships its own filter +
   text-search.
5. **Boat-inventory schema details.** `boat_inventory(boat_id,
   item_id, qty, min_threshold, updated_at)` — does it also carry
   `archived_at` for "this boat used to stock this item but no longer
   does"? Recommendation: yes; the column lets the inventory grid
   hide rows by default while preserving history.

# References

- Sprint 002 — `docs/sprints/SPRINT-002.md` (Org Admin user-story
  backlog).
- Sprint 005 — `docs/sprints/SPRINT-005.md` (auth + role middleware).
- Sprint 006 — `docs/sprints/SPRINT-006.md` (boats + trips data).
- Personas — `docs/product/personas.md` (Org Admin owns the seven
  domains; Site Director is a strict subset deferred to a future
  sprint).
- Stories — `docs/product/organization-admin-user-stories.md`.
- Design — `DESIGN.md`.
- Codex critique — `docs/sprints/drafts/SPRINT-007-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-007-MERGE-NOTES.md`.
