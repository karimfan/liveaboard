# Sprint 007 Intent: Admin UX Design — 3 Candidate Experiences

## Seed

> we need to design the UX for the Admin. An Admin should be able to
> CRUD Boats, Trips, Site Operators, Inventory (per boat) with pricing,
> Org Setup (name, currencies accepted..etc), and Reporting/Analytics.
> I need you to design 3 different experiences for this flow. This flow
> will only be visible for Admins. Site Directors will see a subset of
> this.

## Context

Sprints 005 (Clerk auth) and 006 (boat + trip scraper) shipped. The
SPA's only existing screen is a single placeholder dashboard with zero
stat cards. There is no admin IA, no list views, no detail views, no
forms — every Org Admin user-story enumerated in
`docs/product/organization-admin-user-stories.md` is still un-rendered.

This sprint is **design-only** (modeled after Sprint 002, which was
also a docs/design sprint). The output is three candidate UX
experiences for the Admin's full surface area, plus a recommendation
the user can pick from. Subsequent implementation sprints (008+) build
out the chosen direction.

## Recent Sprint Context

- **Sprint 005 — Clerk auth.** Replaced custom auth with Clerk; the
  SPA now has a working signup/login flow and a logged-in Dashboard
  shell. The dashboard is the only authenticated page.
- **Sprint 006 — Boat + trip scraper.** Added `boats` and `trips`
  tables (with the scraped/operator field split) and a CLI
  (`make scrape-boat`) that lands real fleet data. Data exists; the
  SPA does not yet display it.
- **Sprint 002 — Org Admin user-story backlog.** Defined 30+ user
  stories that this UX must serve. Notable product decisions: org-
  level flat catalog pricing for MVP (per-boat overrides are deferred
  US-5.6); soft-deletion (archive, not hard delete); manifest is split
  pre-departure (Org Admin) vs mid-trip (Site Director).

## Relevant Codebase Areas

| Area | Notes |
|---|---|
| `web/src/main.tsx` | Single-route SPA (`/signup`, `/login`, `/`). New routes will hang off here. |
| `web/src/pages/Dashboard.tsx` | The only signed-in page. Currently shows org name + 3 stat-card zeros. The starting point for whichever option lands. |
| `web/src/styles/tokens.css`, `app.css` | Existing design tokens + a small set of components (`auth-card`, `stat-card`, `dashboard-header`). Extend, don't replace. |
| `internal/store/boats.go`, `trips.go`, `users.go`, `organizations.go` | Repos exist for Boats, Trips, Users, Orgs. Ready to be exposed via HTTP. |
| `internal/auth/middleware.go`, `admin.go` | `RequireOrgAdmin` middleware already gates org-admin-only routes. Each new admin endpoint mounts behind it. |
| `internal/httpapi/httpapi.go` | Where the new admin routes get added. Currently only mounts `/me`, `/organization`, `/invitations`, `/users/{id}/deactivate`. |
| `docs/product/personas.md` | Source of truth for what each persona owns. Admin's surface is here; Site Director's strict subset is here. |
| `docs/product/organization-admin-user-stories.md` | Full backlog the UX must address. |
| `DESIGN.md` | Industrial/utilitarian aesthetic, warm slate + amber, "comfortable" density. Binding for all 3 options. |

## Surface Area to Design

The seven domains the Admin UX must cover:

1. **Org setup.** Name, currency/currencies, default settings. (US-2.x)
2. **Fleet (Boats).** CRUD + cabin layouts; archive vs hard delete.
   (US-3.x)
3. **Catalog (Inventory + Pricing).** Org-level items + per-boat
   *availability* (which items each boat stocks). Per-boat *pricing*
   is deferred per Sprint 002 product decision. (US-5.x)
4. **Trips.** Create / edit / cancel-planned / monitor; assign Site
   Director; pre-departure manifest. (US-4.x)
5. **Users.** Invite Site Director / deactivate / resend. (US-6.x)
6. **Reporting.** Setup completeness, operational status, revenue per
   trip; cross-trip analytics deferred. (US-7.x)
7. **Site Director's subset.** Read-only view of their assigned trips,
   write access to mid-trip manifest + consumption ledger. The chrome
   shows fewer nav items; behind it, the same RBAC.

## Constraints

- Must follow project conventions in CLAUDE.md (work on main, focused
  commits, tests, gofmt/go vet clean for any backend support added).
- DESIGN.md is binding for all three options. None of the candidates
  may propose ocean-tourism aesthetics; all are warm-slate + amber +
  industrial.
- Clerk's hosted UI components (`<UserProfile>`, the auth pages) are
  already styled per DESIGN.md. The admin chrome must compose with
  them, not replace them.
- Role gating is enforced server-side regardless of UI: the SPA's
  navigation hides admin-only sections from Site Directors, but
  `/api/admin/*` endpoints are also guarded by `RequireOrgAdmin`.
  The UX design must explicitly call out the Site Director chrome
  variant for each option.
- Deliverables for this sprint are *documents*, not code. Mockups are
  ASCII / markdown. Optional small Storybook-style HTML snippets are
  allowed but not required.

## Success Criteria

1. **Three genuinely distinct experiences.** Not three coats of paint
   on the same IA — three different mental models for how an Admin
   navigates the seven domains. Distinct enough that they would lead
   to different implementation sprints.
2. **Each experience covers the entire surface.** Every Org Admin user
   story in the backlog is reachable via the proposed IA. Where a
   story is hidden behind context, the doc says how the user finds it.
3. **Each experience documents its Site Director variant.** What the
   chrome looks like for a non-admin user. What's hidden, what
   degrades to read-only.
4. **DESIGN.md compliance.** Every wireframe uses warm slate +
   amber tokens; spacing follows the 8px scale; typography uses the
   declared families.
5. **Decision matrix.** A side-by-side comparison covering at least:
   information density, learning curve, mobile fit, time-to-first-
   useful-screen for an implementation sprint, scaling beyond MVP
   (e.g., 50 boats), accessibility, role-gating clarity.
6. **Clear recommendation.** A primary recommendation with reasoning,
   plus the other two named as fall-forwards if priorities change.
7. **Implementation roadmap stub.** For the recommended option:
   sketches the next 2-3 sprints needed to ship the MVP slice.

## Open Questions

The drafts must answer or surface:

1. **"Per-boat pricing" vs "per-boat availability".** Sprint 002
   committed to org-level flat pricing for MVP. The seed asks for
   "per boat with pricing" — does the user mean per-boat *which items
   are stocked* (a reasonable MVP feature) or per-boat *price overrides*
   (deferred US-5.6)? Recommendation: design both modes but ship
   availability now, prices later.
2. **"Currencies accepted" plural.** Today's `organizations.currency`
   is a single text column. Plural implies a multi-currency org with
   per-trip currency selection. Schema change. Each design must show
   how it handles either single-currency-MVP or multi-currency-future.
3. **Mobile fit.** Site Directors are on a boat with intermittent
   connectivity, often on a tablet or phone. Org Admins are at a desk.
   Does each design degrade gracefully to mobile, and how?
4. **Chrome shape for Site Directors.** Each design needs a paragraph
   on what a Site Director sees. Single-trip workspace? A subset of
   the admin chrome? A separate top-level route?
5. **Time horizon.** Are we designing for MVP (1 boat, 1 admin, 5–20
   trips/year) or for a successful operator (10+ boats, multiple
   admins, 100+ trips/year)? Recommendation: design for the
   successful operator but say which corners we cut for MVP.
6. **Where Clerk's hosted UI fits.** Profile updates, password
   change, and (future) MFA all live in `<UserProfile>`. Each design
   must say where that opens (modal? side route? `/account`?).
7. **First-run / empty states.** A brand-new org has zero boats, zero
   trips, zero crew, zero catalog. Each design must show its first-run
   state and the recommended onboarding nudge (CTA, scrape-import
   prompt, etc.).

## Non-Goals

- Implementing any of the three designs. This sprint produces docs.
- Producing fully-rendered Figma mockups. ASCII / markdown wireframes
  are sufficient (and faster to iterate on).
- Pricing the implementation cost of each option in points/days.
  Sprint 008+ will scope its chosen design.
- Designing Site Director's full UX. Only the chrome variant + which
  admin features degrade to read-only. Site Director's mid-trip
  workflows (manifest, consumption ledger) are a future sprint of
  their own.
- Designing Owner / Guest experiences (Owner role was removed in
  Sprint 005's follow-up; Guest is a future-only persona).
- Mobile-app design. Mobile-web responsive is in scope; native is not.

## Phase 4 Refinements (planner answers — binding)

These were confirmed in the Phase 4 interview before Codex's draft. The
final sprint doc resolves the "Open Questions" above accordingly.

1. **UX direction prior:** Sidebar + Tables (Option A) is the
   planner's gut; Codex is invited to push back. Treat it as a strong
   prior, not a lock-in.
2. **Site Director scope:** **Out of scope for Sprint 007.** "Site
   Operators" in the seed was a future hint; Sprint 007 designs the
   Admin UX only. Each option's Site Director chrome variant section
   is dropped. (A separate future sprint will design Site Director's
   workflows.)
3. **Inventory model:** **Per-boat with quantity tracking.** "A boat
   might have 10 XL t-shirts, while another has 40." This is closer
   to US-5.7 (inventory tracking, previously deferred) than to
   per-boat pricing. The design must accommodate per-boat *stock
   quantities*. Pricing remains org-level flat per Sprint 002 (item
   has one price; quantities are per boat). The Boat detail's Catalog
   tab is a quantity grid, not a checkbox grid; an org-level Catalog
   page still owns the items + prices.
4. **Mobile:** Desktop-first. Sidebar collapses to a hamburger on
   `<768px` viewports for graceful degradation. No tablet/phone
   primary design in Sprint 007.

## Recommended Shortlist of Approaches (drafts must evaluate at least these)

The drafts should each propose three distinct experiences and pick a
primary. Strong candidates worth evaluating:

- **A. Sidebar + Tables.** Persistent left nav with seven sections;
  each is a list-by-default with drill-in detail. Linear / Stripe
  Dashboard analog. Dense, keyboard-driven.
- **B. Boat-centric workspace.** Top selector picks "which boat";
  inside is a tabbed workspace (Schedule / Inventory / Crew /
  Reports). Org-level concerns are a separate "Organization" route.
  Mirrors operator mental model.
- **C. Calendar/timeline-first.** Home is a multi-boat trip calendar
  across the next 6–12 months. CRUD is contextual: click a trip cell
  → side panel; "+ Trip" button drops a blank. Boats / crew /
  catalog are a secondary panel or `/manage`-style nav.
- **D. Command-palette-driven** (provocation). Spotlight-style
  ⌘K palette as the primary entry: "create trip", "find boat", "add
  crew" all filter into actions. Visual nav is minimal.

The drafts may choose any three; D is offered as a useful provocation
to compare against more conventional IAs.
