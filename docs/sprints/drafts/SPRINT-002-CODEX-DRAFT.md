# Sprint 002: Organization Admin User Stories

## Overview

This sprint defines the complete Organization Admin story map for the Liveaboard SaaS product. The repo is still greenfield from an application perspective, so the highest-value next step is not to start building screens or APIs prematurely, but to produce a precise, prioritized backlog that subsequent implementation sprints can execute against.

The Organization Admin is the operational backbone persona for the first version of the product. They configure the organization, fleet, cabin layouts, trips, catalog, pricing, and operational users that site directors and crew depend on during live trips. The sprint should turn that responsibility area into concrete, testable user stories with acceptance criteria, permissions assumptions, data entities, and sequencing.

The output of this sprint is documentation, not production application code. It should create a durable product artifact that explains what the Organization Admin can do, where this persona stops, and what should be handled by Site Directors, Organization Owners, or future Guest self-service features.

## Use Cases

1. **Organization setup and maintenance**: An Organization Admin can configure basic organization details and defaults that apply across the tenant.
2. **Fleet management**: An Organization Admin can add, update, archive, and review boats within their organization.
3. **Cabin layout configuration**: An Organization Admin can define each boat's cabins, capacities, and berth constraints so trips can inherit a usable layout.
4. **Trip planning**: An Organization Admin can create trips for a boat, set dates, assign operational leadership, and prepare the trip for manifest work.
5. **Catalog and pricing management**: An Organization Admin can define purchasable items and services, prices, categories, tax/service assumptions if applicable, and availability rules.
6. **Operational user management**: An Organization Admin can invite and manage users who need org-scoped or trip-scoped access.
7. **Admin oversight**: An Organization Admin can view high-level status across boats, upcoming trips, catalog completeness, and configuration gaps.
8. **Tenant-safe administration**: Every story assumes strict organization isolation, with no cross-tenant visibility or mutation.

## Architecture

This sprint defines a product backlog architecture, not a runtime architecture. The story map should preserve the domain boundaries that will later shape database tables, API resources, and UI navigation.

```
Organization
  |
  +-- Users / Roles
  |
  +-- Boats
  |     |
  |     +-- Cabins
  |     |
  |     +-- Trips
  |           |
  |           +-- Leadership assignments
  |           +-- Manifest handoff boundary
  |
  +-- Catalog
        |
        +-- Categories
        +-- Items / Services
        +-- Pricing / availability rules
```

### Story Map

```
Admin foundation
  -> org profile
  -> auth and roles
  -> dashboard/status

Fleet setup
  -> boats
  -> cabins
  -> capacity validation

Trip setup
  -> trip shell
  -> leadership assignment
  -> handoff to Site Director

Commercial setup
  -> catalog categories
  -> items/services
  -> pricing and availability

Operational governance
  -> archive/deactivate flows
  -> audit-sensitive changes
  -> reporting seed stories
```

### Story Format

Each user story should use a consistent format:

```markdown
### OA-XX: Story title

As an Organization Admin,
I want ...
so that ...

**Priority:** Must / Should / Could
**Area:** Auth / Organization / Fleet / Trips / Catalog / Users / Reporting

**Acceptance Criteria:**
- [ ] Criterion written as observable behavior
- [ ] Validation and error cases included
- [ ] Multi-tenant access expectation included where relevant

**Notes:**
- Implementation notes, exclusions, or persona boundaries
```

## Implementation Plan

### Phase 1: Product Boundaries and Taxonomy (~20%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Main story backlog artifact.
- `docs/product/personas.md` - Persona boundaries and responsibility notes, if this does not already exist.

**Tasks:**
- [ ] Create `docs/product/` if needed for durable product planning docs.
- [ ] Define the Organization Admin persona in relation to Organization Owner, Site Director, Crew, and Guest.
- [ ] Establish story ID convention (`OA-001`, `OA-002`, etc.) and priority labels (`Must`, `Should`, `Could`).
- [ ] Define functional areas: auth/access, organization settings, fleet, cabins, trips, catalog, users, reporting/oversight.
- [ ] Explicitly document that this sprint does not implement application code, database schema, APIs, or UI.

### Phase 2: Core Administration Stories (~35%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Add core setup and operational administration stories.

**Tasks:**
- [ ] Write auth and access stories: log in, accept invite, switch/view organization context if needed, sign out, handle unauthorized access.
- [ ] Write organization profile stories: view org details, edit org name/details, configure org-level defaults.
- [ ] Write fleet stories: create boat, edit boat, archive boat, list boats, view boat setup completeness.
- [ ] Write cabin stories: add cabin, edit cabin, deactivate cabin where safe, define capacity/type, validate total capacity.
- [ ] Write trip setup stories: create trip from boat, set date range, prevent invalid overlapping assumptions where needed, assign site director/crew leadership, mark trip ready for manifest setup.
- [ ] For each story, include acceptance criteria covering happy path, validation failure, authorization, and empty states where applicable.

### Phase 3: Catalog, Pricing, and User Management Stories (~25%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Add commercial setup and user administration stories.

**Tasks:**
- [ ] Write catalog category stories: create, edit, reorder/archive categories.
- [ ] Write catalog item/service stories: create item, edit item, archive item, set taxable/service metadata if relevant, define whether inventory tracking is required later.
- [ ] Write pricing stories: set base org price, define whether per-boat or per-trip overrides are in or out of initial scope, document follow-up stories for overrides if deferred.
- [ ] Write user management stories: invite Site Director, invite another admin if allowed, deactivate user, assign org-level role, assign trip leadership.
- [ ] Mark stories that are required before the first Site Director workflow can be built.

### Phase 4: Prioritization, Gaps, and Sprint Slicing (~15%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Add priority, dependencies, and follow-up sprint slices.
- `docs/sprints/drafts/SPRINT-002-MERGE-NOTES.md` - Optional planning notes if created during synthesis.

**Tasks:**
- [ ] Group stories into MVP, near-term, and later buckets.
- [ ] Identify dependencies between stories, such as boats before trips and catalog before ledger entry.
- [ ] Propose 3-5 follow-up implementation sprint candidates based on the story map.
- [ ] Answer the open questions from the intent directly or record explicit recommendations where final product decisions are still pending.
- [ ] Add non-goals for this persona, especially detailed manifest operation, live ledger entry, guest self-service, and deep analytics if deferred.

### Phase 5: Review and Consistency Pass (~5%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Final edits.
- `docs/sprints/drafts/SPRINT-002-CODEX-DRAFT.md` - This draft.

**Tasks:**
- [ ] Ensure every story has acceptance criteria.
- [ ] Ensure story language is implementation-ready but not over-prescriptive about UI or database structure.
- [ ] Ensure multi-tenant isolation is reflected in relevant stories.
- [ ] Ensure the resulting backlog is concise enough to guide development rather than becoming an unbounded product requirements document.
- [ ] Run a markdown formatting/readability pass.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/product/organization-admin-user-stories.md` | Create | Primary Organization Admin story backlog with priorities and acceptance criteria. |
| `docs/product/personas.md` | Create | Persona boundaries and responsibility definitions, if not already present. |
| `docs/sprints/drafts/SPRINT-002-MERGE-NOTES.md` | Optional Create | Notes from comparing draft approaches and final planning decisions. |
| `docs/sprints/SPRINT-002.md` | Future Create | Final merged sprint plan after draft review, outside this requested Codex draft step. |

## Definition of Done

- [ ] Organization Admin persona responsibilities are clearly defined.
- [ ] Story backlog covers auth, organization settings, fleet management, cabin layout, trip setup, catalog/pricing, user management, and admin oversight.
- [ ] Every story includes acceptance criteria.
- [ ] Stories are prioritized as Must / Should / Could.
- [ ] Stories identify dependencies and likely follow-up implementation sprint slices.
- [ ] Open questions from `SPRINT-002-INTENT.md` are answered or converted into explicit product decisions.
- [ ] Non-goals and persona boundaries are documented.
- [ ] Markdown docs are readable, consistently structured, and committed as documentation only.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Scope expands into full product requirements | Medium | Medium | Keep the artifact limited to Organization Admin and explicitly mark non-goals. |
| Stories are too vague for implementation | Medium | High | Require acceptance criteria and validation/error cases for each story. |
| Stories overfit a UI before architecture exists | Medium | Medium | Describe observable behavior and workflow outcomes, not component layouts. |
| User management is omitted despite being needed for trip leadership | Medium | High | Include user/role stories as first-class Organization Admin responsibilities. |
| Catalog pricing assumptions block ledger design later | Medium | High | Document pricing scope decisions and defer overrides only with explicit follow-up stories. |
| Site Director responsibilities leak into admin scope | High | Medium | Define handoff boundaries for manifest and live trip operations. |

## Security Considerations

- Every administration story should assume organization-scoped authorization.
- User invitation, role assignment, and deactivation stories must prevent privilege escalation.
- Archived/deactivated records should preserve historical trip and ledger integrity.
- Catalog and trip setup changes should account for auditability where they can affect guest charges.
- No story should imply cross-organization search, visibility, reporting, or mutation.

## Dependencies

- Sprint 001 is tooling-only and not functionally required for the product stories, but the sprint workflow should remain compatible with the Go tracker.
- `CLAUDE.md` project constraints: Go backend, TypeScript/React frontend, PostgreSQL row-level security, tests required for future implementation.
- `DESIGN.md` applies to future UI work, but this sprint should not design UI screens beyond workflow-level implications.

## Open Questions

1. Should Organization Admins be able to create other Organization Admins, or only invite Site Directors and crew?
2. Should pricing overrides be explicitly deferred to later, or included as `Should` stories for boat/trip-specific exceptions?
3. Should trip creation include initial guest manifest upload/setup, or should it stop at creating the trip shell and assigning a Site Director?
4. What minimum reporting belongs to Organization Admin for MVP: setup completeness only, operational trip status, revenue summaries, or all of these as separate priority tiers?
5. Do archived boats, cabins, catalog items, and users need explicit restore stories in MVP, or is archive-only sufficient initially?
