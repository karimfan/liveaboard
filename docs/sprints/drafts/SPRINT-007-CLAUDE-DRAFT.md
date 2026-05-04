# Sprint 007: Admin UX — Three Candidate Experiences

## Overview

The product has its data foundations: orgs, users, boats, trips, and a
working scraper that lands real fleet data. What it does not have is a
single screen for an Org Admin to do anything beyond looking at the
auth-landing dashboard. This sprint produces three genuinely different
proposals for that admin surface — different mental models, different
information architectures, different first-run feels — and picks one
to take into Sprint 008.

The seven domains the Admin must serve (from the user-story backlog)
are: **Org Setup**, **Fleet (Boats)**, **Catalog (Inventory + pricing)**,
**Trips**, **Users (Site Directors)**, **Reporting**, and the **Site
Director's strict subset** (read-only on most, write on their assigned
trip's manifest + consumption ledger). All three options below cover
those seven domains; what differs is the navigation primitive, the
default view, and the operator's mental model.

This is a docs-only sprint (modeled after Sprint 002). The output is
ASCII wireframes + IA decisions + a decision matrix + a recommendation.
Implementation begins in Sprint 008 against the chosen direction.

## Use Cases

1. **First-run onboarding.** A brand-new org has zero of everything.
   The admin needs an obvious "next step" — typically: import or
   create the first boat.
2. **Trip planning at scale.** An admin running 8 boats with 20+ trips
   per year needs to see all upcoming trips, spot gaps, assign Site
   Directors before the trip starts.
3. **Boat configuration.** Admin adds a boat, defines its cabins,
   configures which catalog items are stocked onboard.
4. **Crew management.** Admin invites a Site Director by email,
   assigns them to a planned trip, eventually deactivates a leaver.
5. **Catalog maintenance.** Admin adds/edits/archives items; sets
   org-wide flat prices (per-boat overrides deferred per Sprint 002).
6. **Pre-departure manifest.** Admin enters guests against cabins
   for an upcoming trip. (Mid-trip changes are Site Director's.)
7. **Operational oversight.** Admin sees which trips are at risk
   (no Site Director assigned, missing manifest, etc.).
8. **Site Director's day.** A Site Director logs in, sees their
   single assigned active trip, opens the manifest, records
   consumption. Most admin chrome is invisible to them.

## DESIGN.md Compliance Baseline (binding for all three options)

Tokens used throughout — these mirror `web/src/styles/tokens.css`:

```
Colors:    primary  #E5853B (amber)
           neutrals warm slate, c-50 .. c-900
           semantic success/warning/error/info as defined
Type:      display  General Sans 700
           body     DM Sans 400/500/600
           data     Geist (tabular-nums) for all tables
Spacing:   8px base; xs(4) sm(8) md(16) lg(24) xl(32)
Radii:     sm(4) md(8) lg(12)
Density:   "comfortable" — crew scans fast, but not cramped
```

ASCII wireframes that follow are dimensionally honest: each character
roughly maps to a 12px column, each line to 24px row. They are
directional — not pixel-perfect — but they reflect actual DOM nesting.

---

# Option A — Sidebar + Tables

**Mental model:** "I'm in a back-office tool. Everything is a list of
things; I drill in to edit." Linear, Stripe Dashboard, Notion-database
analogue. Persistent left nav, table-by-default for every section.

## Information architecture

```
┌──────────────────┐
│ Liveaboard       │  ← wordmark
│                  │
│ ⌂ Overview       │  ← landing: "what needs your attention"
│ 🛥 Fleet          │  ← list of boats; click → boat detail
│ 🗓 Trips          │  ← list of trips; filters: status, boat, date
│ 👥 Crew           │  ← list of users + pending invites
│ 📦 Catalog        │  ← list of catalog items + categories
│ 📊 Reports        │  ← prebuilt reports + (future) custom views
│ ⚙ Settings       │  ← org name, currency, defaults
│                  │
│ ──────────────── │
│ Acme Diving      │  ← org switcher (single org for now)
│ owner@acme.test  │  ← user menu → /account (Clerk UserProfile)
└──────────────────┘
```

7 top-level destinations, all visible at all times. Nav width 220px
(matches DESIGN.md `--sidebar-w`). The Site Director chrome variant
hides Fleet / Crew / Catalog / Reports / Settings — leaving Overview +
Trips only — and Trips defaults to "my trips" instead of all-org.

## Sample screen — Trips list

```
┌─────────────────┬───────────────────────────────────────────────────────────┐
│ Liveaboard      │ Trips                                          [ + Trip ] │
│                 │                                                            │
│ ⌂ Overview      │ Status: Any ▾   Boat: All ▾   When: Next 90 days ▾  ⌘K   │
│ 🛥 Fleet         │ ────────────────────────────────────────────────────────── │
│ 🗓 Trips ●       │ │ Boat       │ Itinerary       │ Dates             │ Dir.│
│ 👥 Crew          │ │ Gaia Love  │ Raja Ampat N&S  │ Feb 6 - 16, 2027  │ —   │
│ 📦 Catalog       │ │ Gaia Love  │ Raja Ampat N&S  │ Feb 18 - 28, 2027 │ —   │
│ 📊 Reports       │ │ Gaia Love  │ Komodo          │ Mar 4 - 14, 2027  │ JM  │
│ ⚙ Settings      │ │ Blue Spirit│ Maldives Central│ Mar 9 - 16, 2027  │ KW  │
│                 │ │ ...        │                 │                   │     │
│ Acme Diving     │                                                  ‹ 1 2 ›  │
│ owner@acme.test │                                                            │
└─────────────────┴───────────────────────────────────────────────────────────┘
```

- **+ Trip** opens a slide-over panel from the right (preserves
  context). Inside: boat picker, dates, itinerary, departure/return
  ports, Site Director picker (Crew filtered to active SDs).
- **Row click** opens trip detail — full page or right-side panel
  (TBD Sprint 008; both are valid in this IA).
- **⌘K** opens a command palette: "go to boat...", "create trip", "invite
  crew". Useful but not load-bearing — discoverable as power feature.

## Sample screen — Boat detail (drill-in from Fleet)

```
┌─────────────────┬───────────────────────────────────────────────────────────┐
│ ⌂ Overview      │ Fleet › Gaia Love                            [ ⋯ Actions ] │
│ 🛥 Fleet ●       │                                                            │
│ 🗓 Trips         │ ┌───────────┐ Gaia Love                                    │
│ 👥 Crew          │ │  [image]  │ liveaboard.com/diving/indonesia/gaia-love    │
│ 📦 Catalog       │ │           │ Last synced: 2 hours ago        [ Re-sync ] │
│ 📊 Reports       │ └───────────┘                                              │
│ ⚙ Settings      │                                                            │
│                 │ Tabs:  Schedule | Cabins | Catalog (stocked items) | Notes │
│                 │ ────────────────────────────────────────────────────────── │
│                 │ [Schedule tab content - 18-month calendar of trips]        │
└─────────────────┴───────────────────────────────────────────────────────────┘
```

The boat detail page is itself tabbed because the alternative — a long
scrolling page — buries the Catalog (stocked-items) section that admins
want to edit frequently. Tabs surface "what about this boat".

## How the seven domains map

| Domain | Where in IA |
|---|---|
| Org setup | Settings (top-level) |
| Boats | Fleet → Boat detail (tabs: Schedule / Cabins / Catalog / Notes) |
| Catalog | Catalog (top-level, org items); Boat → Catalog tab (per-boat stocking) |
| Trips | Trips (top-level list); Trip detail page from row click |
| Users | Crew (top-level); invitation modal; user detail page |
| Reporting | Reports (top-level); each report a separate sub-page |
| Site Director chrome | Sidebar shows only Overview + Trips; Trips filtered to assigned |

## First-run state

Overview shows a "Getting Started" checklist:

```
[ ] Add or scrape your first boat       [Add boat] [Import from URL]
[ ] Create your first trip              (gated until ≥1 boat)
[ ] Invite a Site Director              [Invite]
[ ] Set up your catalog                 [Add items]
```

Each item is a discoverable nudge → opens the relevant slide-over.

## Pros / cons

**Pros**
- Familiar back-office mental model. Zero learning curve for anyone
  who's used Linear / Stripe / Notion / Salesforce.
- Highly information-dense. A 27" desk monitor shows 30+ trips in one
  table. Suits scaling to 10+ boats.
- Maps cleanly to existing repo helpers (each top-level is a single
  `BoatsForOrg` / `TripsByOrgInRange` / etc. query).
- Deep-linkable: every list / detail / sub-tab is a URL. Browser
  back/forward "just work".
- Easy keyboard navigation; pairs naturally with ⌘K.

**Cons**
- Doesn't reinforce the operator's actual unit of work, which is
  "one trip on one boat". Admins mentally pivot between two list views
  to find a trip — Trips list and Fleet → Boat → Schedule.
- Mobile fit is mediocre: tables compress poorly. Admin-on-tablet is
  fine; admin-on-phone is awkward.
- Reporting feels like an afterthought (a tab to itself with sparse
  content until US-7.x lands).
- Tables may feel "cold" to a small operator with 1 boat and 6 trips
  a year.

## Implementation roadmap stub (if A is picked)

- **Sprint 008.** Sidebar shell + Overview screen + Fleet list and
  detail (uses existing `boats` data). Real stat counts on Overview.
  Deep-linking, role-gated chrome.
- **Sprint 009.** Trips list + Trip detail + Trip create slide-over.
  Site Director assignment dropdown. Trip cancellation.
- **Sprint 010.** Catalog (org items + per-boat stocking tab). Crew
  list + invitations.
- **Sprint 011.** Manifest editor on Trip detail (pre-departure).
  Settings page (US-2.x). Reports MVP (US-7.1, US-7.2).

---

# Option B — Boat-Centric Workspaces

**Mental model:** "Each boat is its own world. I pick a boat, then I do
boat things to it." A top-level boat selector is the primary nav;
inside the selected boat is a tabbed workspace. Org-level concerns
(org settings, all-org reporting, all-org crew) live behind a separate
"Organization" route.

## Information architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Liveaboard   ⌃Boat: Gaia Love ▾   ⚲ Search   ⊘ Org   👤 owner@acme.test │  ← top bar
└──────────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────────┐
│ Tabs:  Schedule  Manifest  Crew  Catalog  Cabins  Reports  Settings      │
│ ──────────────────────────────────────────────────────────────────────── │
│                                                                          │
│  [active tab content]                                                    │
└──────────────────────────────────────────────────────────────────────────┘
```

The persistent boat selector at top-left is the compass. Click "⊘ Org"
to leave the boat context entirely; that route hosts:

```
Org root:
  Org Settings  (name, currency, defaults)
  All Crew      (every Site Director across boats)
  Org Reports   (cross-trip analytics, future)
  Manage Boats  (CRUD; the only way to add/remove a boat)
```

The Site Director chrome is severely reduced: they see no boat
selector. Their landing is `/my-trip` — a single-trip workspace with
just the Manifest, Crew (read-only), and Consumption tabs. They never
see the Org route.

## Sample screen — Boat workspace, Schedule tab

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Liveaboard   ⌃Boat: Gaia Love ▾                  ⊘ Org    owner@acme.test│
│ ──────────────────────────────────────────────────────────────────────── │
│ Schedule  Manifest  Crew  Catalog  Cabins  Reports  Settings             │
│ ──────────────────────────────────────────────────────────────────────── │
│ Gaia Love · Schedule                                            [+ Trip] │
│                                                                          │
│ [ ◂ Q1 2027 ▸ ]                                                          │
│                                                                          │
│   FEB 2027                                                               │
│     6 ─── 16   Raja Ampat North & South      JM   FULL                   │
│    18 ─── 28   Raja Ampat North & South      —    FULL                   │
│   MAR 2027                                                               │
│     4 ─── 14   Komodo                        JM   2 of 8 cabins booked   │
│    16 ─── 25   Komodo                        —    Available              │
│                                                                          │
│   APR 2027                                                               │
│     1 ─── 11   Raja Ampat                    KW   Departed               │
│   ...                                                                    │
└──────────────────────────────────────────────────────────────────────────┘
```

The schedule is a vertical timeline (one row per trip), grouped by
month. The status column at right shows the most-actionable info:
either a manifest fill ratio or "FULL" / "Available". The Site
Director column shows initials when assigned, "—" when not.

## How the seven domains map

| Domain | Where in IA |
|---|---|
| Org setup | Org route → Settings |
| Boats | Org route → Manage Boats; switch boat via the top-bar selector |
| Catalog | Boat workspace → Catalog tab (per-boat stocking + price view); Org route → Catalog Library (org items) |
| Trips | Boat workspace → Schedule tab; trip detail is a slide-over from a row |
| Users | Boat workspace → Crew tab (filtered to assigned); Org route → All Crew (full list, invite) |
| Reporting | Boat workspace → Reports tab (single-boat); Org route → Org Reports (cross-boat) |
| Site Director chrome | No boat selector; lands on `/my-trip` with just Manifest + Crew (RO) + Consumption tabs |

## First-run state

A brand-new org has no boats, so the boat selector says "(no boats yet)
— Add your first boat" as a CTA in the top bar. Clicking it routes to
Org → Manage Boats with the Add Boat slide-over open. After the first
boat is added, the user is auto-routed into that boat's Schedule tab
with an "Import upcoming trips from a URL" prompt (links to the
scrape-boat flow, future Sprint 010+).

## Pros / cons

**Pros**
- Mirrors how operators actually think. "Let me check on Gaia Love
  this morning" is the most common admin sentence.
- Boat-scoped tabs collapse the IA: Schedule + Manifest + Crew + Catalog
  for *this boat* are one click apart, never a top-nav round-trip.
- The Site Director variant is dramatically simpler — they never see
  the boat selector, never see the Org route. Easier role gating.
- Per-boat reporting (utilization, revenue, occupancy) is in the
  natural place — no separate "report builder" needed.

**Cons**
- "Where do I add a boat?" is non-obvious. Adding a boat requires
  leaving the boat-context to the Org route. Onboarding has a chicken-
  and-egg moment for first-time users.
- Cross-boat operations are awkward: "show all trips next week across
  the fleet" requires the Org route or a separate cross-boat view.
- Catalog has two homes (Org Library + per-Boat stocking); could
  confuse users about where to add a new item.
- The top-bar selector competes with browser-tab navigation patterns:
  bookmarks pin to a specific boat, not to the abstract "Schedule" view.
- More custom — less off-the-shelf component reuse than Option A.

## Implementation roadmap stub (if B is picked)

- **Sprint 008.** Top-bar shell + boat-selector + Schedule tab on the
  Boat workspace. Org route shell + Manage Boats list and Add Boat
  flow.
- **Sprint 009.** Manifest tab + Crew tab; Trip create slide-over.
- **Sprint 010.** Catalog tabs (Boat + Org Library); Org → All Crew +
  invitations.
- **Sprint 011.** Org Settings; Reports tabs (Boat + Org); first-run
  polish.

---

# Option C — Calendar-First with Contextual Side Panels

**Mental model:** "Time is the most important thing. Show me everything
on a calendar; let me click anything to act on it." Cal.com / Linear's
project view / Notion calendar analogue. The default landing is a
6–12 month timeline of all trips across all boats. CRUD happens via
side panels triggered by clicking calendar cells; secondary nav is
collapsed behind a `⋯ Manage` menu.

## Information architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Liveaboard    ⊞ Schedule  ⋯ Manage ▾   owner@acme.test                   │
└──────────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────────┐
│ ◂ Q1 2027 ▸                              [Boat: All ▾] [Status: Any ▾]   │
│ ──────────────────────────────────────────────────────────────────────── │
│           │ Jan │ Feb              │ Mar              │ Apr            │ │
│ Gaia Love │ ▮▮▮ │ ▮▮▮▮▮  ▮▮▮▮▮▮     │ ▮▮▮▮▮▮  ▮▮▮▮▮     │ ▮▮▮▮▮▮  ▮▮     │ │
│ Blue Spi. │ ▮▮  │   ▮▮▮ ▮▮▮▮       │ ▮▮▮▮▮  ▮▮▮▮▮      │ ▮▮▮▮▮          │ │
│ Coral Q.  │     │  ▮▮▮▮ ▮▮▮▮▮▮     │ ▮▮▮▮▮▮ ▮▮▮▮▮▮     │ ▮▮▮▮▮▮ ▮▮▮▮     │ │
│                                                                          │
│ Legend:  ▮ FULL    ▮ Booking    ▮ Available    ▮ Past                    │
└──────────────────────────────────────────────────────────────────────────┘
```

`⋯ Manage ▾` opens a popover menu:

```
Manage:
  Fleet
  Catalog
  Crew
  Reports
  Org Settings
```

These are the same entities as Options A/B, but they live one click
behind a menu — not always visible. The bet is that an admin's primary
job is *seeing the schedule and editing trips*. Everything else is
infrequent.

## Click behaviors

- **Click a trip cell** → right-side detail panel: dates, itinerary,
  ports, status, Site Director, manifest summary, "Open Manifest →"
  link. Edit in place.
- **Click an empty cell on a boat row** → slide-over: "Create trip on
  Gaia Love starting Feb 6". Pre-fills boat + start date.
- **Click a boat name on the left axis** → side panel: boat profile,
  cabins, stocked catalog items, settings. (No separate boat detail
  page.)
- **Click ⋯ Manage → Fleet** → routes to a list view (looks like
  Option A's Fleet, simpler).

## How the seven domains map

| Domain | Where in IA |
|---|---|
| Org setup | ⋯ Manage → Org Settings (full page) |
| Boats | Click boat name (side panel); ⋯ Manage → Fleet for list/CRUD |
| Catalog | ⋯ Manage → Catalog (full page) |
| Trips | The calendar IS trips. CRUD all happens on the calendar via panels |
| Users | ⋯ Manage → Crew (full page); also invitable from a trip's Site Director picker |
| Reporting | ⋯ Manage → Reports (full page) |
| Site Director chrome | The calendar shows only their assigned trip rows (one row); ⋯ Manage menu shrinks to "Crew (read-only)" + nothing else; or hide the menu entirely |

## First-run state

The empty calendar shows a single overlay:

```
┌──────────────────────────────────────────────┐
│  Welcome to Liveaboard.                      │
│  Add your first boat to get a schedule view. │
│                                              │
│      [ + Add Boat ]    [ Import from URL ]   │
└──────────────────────────────────────────────┘
```

After the first boat lands, the calendar shows that boat's row with
zero trips and a "+" prompt in the current month.

## Pros / cons

**Pros**
- Most aligned with what an operator actually does: look at when
  trips are happening and tweak them. The home screen is the value.
- Visual trip overview across boats is unmatched. Spotting an empty
  week or a clash is instant.
- Compact for many boats (one row each). Scales to 30+ boats with a
  vertical scroll.
- Site Director's chrome is dead simple: their calendar has one row.
- Mobile-friendly: a calendar can degrade to a single-boat day-view
  on phone naturally.
- Fewer top-level destinations means simpler IA to teach.

**Cons**
- Hides the secondary surfaces (Catalog, Settings, Reports) behind a
  menu. Admins who came to "set up the catalog" have to learn the
  shortcut.
- Calendar logic is the most complex implementation of the three.
  Date-grid math, virtualized scrolling for many boats, drag-to-edit
  (future).
- Empty state is awkward. Until there are trips, the calendar is
  blank. The "Manage" menu does the heavy lifting at first.
- Tabular reporting (US-7.x) doesn't live on the calendar — feels
  like a separate product surface.
- Less common pattern; users expect a sidebar or top tabs and may
  not discover ⋯ Manage. Could be mitigated with a permanent
  secondary nav, but then it converges toward Option A.
- Mobile *responsive* fit is OK; mobile *primary* would need a
  different layout entirely.

## Implementation roadmap stub (if C is picked)

- **Sprint 008.** Calendar grid component (read-only) using existing
  `trips` data. Trip detail side panel. Trip create slide-over from
  empty cell.
- **Sprint 009.** ⋯ Manage menu + Fleet page + Org Settings page.
- **Sprint 010.** Catalog page; Crew page (with invite from trip
  picker).
- **Sprint 011.** Reports MVP; Manifest editor inside trip panel;
  Site Director's single-row calendar variant.

---

# Decision Matrix

| Dimension | A: Sidebar+Tables | B: Boat Workspace | C: Calendar-First |
|---|---|---|---|
| Time to first useful screen | **Fast** (Fleet list = day-1) | Medium (workspace shell more custom) | Slow (calendar grid is involved) |
| First-run / empty-state UX | Strong (checklist) | Weak (chicken-and-egg on first boat) | Weak (blank calendar) |
| Information density | **Highest** (tables) | Medium (per-boat focus) | High but row-based |
| Operator mental-model fit | Generic | **Strongest** | Strong (time-first) |
| Cross-boat overview | Good (Trips list) | Weak (Org route only) | **Excellent** (calendar) |
| Mobile / tablet fit | Weak | Medium (works in single-boat) | **Best** (calendar collapses) |
| Site Director chrome simplicity | Strong (hide 5 nav) | **Strongest** (one route) | **Strongest** (one row) |
| Discoverability of secondary features | **Strongest** (always visible) | Medium (Org route) | Weakest (⋯ Manage menu) |
| Implementation cost (Sprint 008 alone) | **Lowest** | Medium | Highest |
| Scaling to 10+ boats | Strong | Medium (boat-switching gets old) | **Strongest** |
| Off-the-shelf component reuse | **Highest** (table libs etc.) | Medium | Medium |
| Keyboard / power-user fit | **Strongest** (⌘K + tables) | Medium | Medium |

# Recommendation

**Primary: Option A (Sidebar + Tables).**

Reasoning, in priority order:

1. **It's the cheapest first sprint.** Sprint 008 = a sidebar shell +
   one list view (Fleet) + one detail page. Three small commits land
   visible value. Options B and C both require non-trivial custom
   layout that delays first-screen-shipped by a sprint.
2. **Strongest first-run.** A new org has no boats, no trips, no
   crew — exactly the state where Options B/C visually fail. The
   Sidebar IA's Overview "Getting Started" checklist is the right
   answer for an empty org.
3. **Clearest role gating.** Hiding 5 of 7 sidebar items for Site
   Directors is a one-line render check. B and C both require more
   nuanced chrome variants.
4. **Compounds.** Once we have the sidebar, *all subsequent* admin
   features have an obvious home. Adding "Catalog" or "Reports" later
   is a CRUD page next to the existing ones, not an architectural
   decision.
5. **It's the boring choice.** That is the point. We are pre-customer.
   Picking the most familiar pattern means operator A and operator B
   both onboard quickly because they've used Linear/Stripe/Notion.

**Fall-forward 1: Option C (Calendar-First).** If the user research
later shows that operators spend most of their time on the schedule
view and rarely touch settings/catalog/reports, we restructure into
C in a future sprint. The data model doesn't change; only the chrome
does.

**Fall-forward 2: Option B (Boat Workspace).** The most distinctive
choice but also the riskiest because the Add Boat flow is
counter-intuitive. If we get to 5+ boats per org and operators are
clearly siloed per-boat in their workflow, restructure into B. Until
then, Option A's `Boat detail` page already provides a per-boat tabbed
mini-workspace inside the broader IA — a reasonable middle ground.

# Site Director Chrome (for the recommended Option A)

Site Directors are not Org Admins. They get a pruned variant of the
same shell:

```
┌──────────────────┐
│ Liveaboard       │
│                  │
│ ⌂ My trip        │  ← landing: detail page for their assigned active trip
│ 🗓 Trips          │  ← list filtered to their assigned trips (past + future)
│                  │
│ ──────────────── │
│ Acme Diving      │
│ jm@acme.test     │
└──────────────────┘
```

- No Fleet, Catalog, Crew, Reports, or Settings in the sidebar.
- Trips list filters server-side by `site_director_id = me`.
- The trip detail page they see has the **same** tabs an Admin sees
  (Schedule / Manifest / Crew / Notes), but:
  - Manifest is **writable** for them (mid-trip changes are theirs).
  - Crew is read-only.
  - The "Cancel trip" action is hidden.
  - The "Start trip → Active", "Complete trip → Completed" actions
    appear (Site Director owns trip lifecycle transitions per
    `personas.md`).
- Server-side: every endpoint they hit is also gated by both
  `RequireSession` and a per-trip role check. The chrome simplification
  is a UX nicety; the security boundary is in the API.

# Open Questions Surfaced for the Sprint 008 Author

1. **Multi-currency or single-currency-MVP?** Recommendation: ship
   single-currency for Sprint 008. `organizations.currency` stays a
   single text column. Add a "Currencies accepted" multi-select to
   Settings UI but disable / mark "(coming soon)" until the schema
   change.
2. **Per-boat catalog: stocking only, or stocking + price overrides?**
   Recommendation: stocking only for the MVP slice. The Boat detail's
   Catalog tab shows checkboxes against the org's catalog items.
   Per-boat / per-trip price overrides ship later (US-5.6).
3. **Where does Clerk's `<UserProfile>` open?** Recommendation: a
   `/account` modal/route launched from the user-menu footer of the
   sidebar.
4. **Mobile.** For Sprint 008 we ship desktop-first. The sidebar
   collapses to a hamburger on `<768px` viewports; that's enough for
   internal demos. A real mobile design comes when Site Directors
   actually use the app on the boat.

## Files Summary (this sprint)

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-007.md` | Create (after merge) | Final, single-page sprint doc with the picked option. |
| `docs/sprints/drafts/SPRINT-007-INTENT.md` | Create | Concentrated intent (already exists). |
| `docs/sprints/drafts/SPRINT-007-CLAUDE-DRAFT.md` | Create | This document. |
| `docs/sprints/drafts/SPRINT-007-CODEX-DRAFT.md` | Create | Codex's competing proposal. |
| `docs/sprints/drafts/SPRINT-007-CLAUDE-DRAFT-CODEX-CRITIQUE.md` | Create | Codex's critique of this doc. |
| `docs/sprints/drafts/SPRINT-007-MERGE-NOTES.md` | Create | Synthesis. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 007. |

No code changes this sprint.

## Definition of Done

- [ ] Three concretely different admin UX experiences exist as
      artifacts in `docs/sprints/SPRINT-007.md` (or referenced
      side-files), each with: IA diagram, 1-2 sample wireframes, the
      seven-domain mapping, the Site Director variant, first-run
      state, and pros/cons.
- [ ] Decision matrix scoring the three options across density,
      learning curve, mobile, time-to-first-useful-screen, scaling,
      role-gating clarity.
- [ ] A single primary recommendation with reasoning + named
      fall-forwards.
- [ ] An implementation roadmap stub (Sprints 008–011) for the
      recommended option.
- [ ] DESIGN.md compliance asserted: every wireframe references the
      relevant token names from `tokens.css`.
- [ ] Tracker shows Sprint 007 (`go run docs/sprints/tracker.go sync`).

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| User picks none of the three | Low | Medium | Each option is a self-contained design; if none fit, the docs themselves are reusable input for a follow-up "Option D" exploration. The kplan workflow (intent → drafts → critique → merge) is designed to surface this gap before code is written. |
| User picks one then changes mind during Sprint 008 | Medium | Medium | Sprint 008 is bounded to "shell + first list/detail" — pivoting between A/B/C costs at most one sprint. Schema (Sprints 003/005/006) is independent. |
| Three options feel too similar | Low | Medium | Codex's competing draft (Phase 5) provides external pressure to avoid convergence. Decision matrix forces explicit differentiation. |
| ASCII wireframes are too vague to commit to | Medium | Low | Sprint 008 author may need to re-draft a single screen as an HTML mockup before coding. That's a one-day spike, captured here as a follow-up. |

## Security Considerations

- **Role gating is server-side, not chrome.** The UX hides admin-only
  destinations from Site Directors, but every `/api/admin/*` endpoint
  is also `RequireOrgAdmin`. The Site Director chrome variant is a
  usability simplification, not a security boundary.
- **Cross-tenant isolation is preserved by repos.** Every list/detail
  query already takes an explicit `organization_id` (Sprint 006
  pattern). The UX surface adds no new tenant boundaries.
- **Clerk hosted UI integration.** `<UserProfile>` continues to handle
  email/password change inside its own surface; we do not re-implement
  account management in the admin chrome.

## Dependencies

- **Sprint 005** (Clerk auth + `RequireOrgAdmin` middleware) — the
  role-gating substrate.
- **Sprint 006** (boats + trips schema) — the data we'll surface.
- **Sprint 002** (user-story backlog + product decisions) — the
  source of every story this UX must serve.
- **DESIGN.md** — binding for every wireframe.
- No new tools, no new dependencies, no new infrastructure.

## Open Questions

1. Same as the seven open questions in `SPRINT-007-INTENT.md`. The
   merged sprint doc resolves the ones the user answered in the Phase 4
   interview and re-files the rest as Sprint 008's pre-flight.
