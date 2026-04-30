# Sprint 002 Intent: Organization Admin User Stories

## Seed

We will work on building this SaaS application iteratively. We will first build all the functionality for the Organization Admin. Articulate all the user stories that should satisfy the use cases for this persona.

## Context

- **Project state:** Greenfield — no application code exists yet. The project has a README describing the domain, CLAUDE.md with tech stack decisions, and DESIGN.md with the visual design system.
- **Tech stack:** Go backend (stdlib + minimal deps), TypeScript/React frontend, PostgreSQL with row-level security, online-default.
- **Domain model:** Organizations own Boats, Boats have Cabins, Boats run Trips, Trips have guest Manifests, Guests consume from a Catalog, consumption is tracked in a Ledger.
- **Target persona:** Organization Admin — manages org-wide operations including creating trips, managing the fleet (adding/removing boats), and configuring the item catalog and pricing.

## Recent Sprint Context

- **Sprint 001 (planned):** Rewrite sprint tracking tooling from Python to Go. Infrastructure/tooling only — no application code. Not yet started.

This is the first application-level sprint.

## Relevant Codebase Areas

No application code exists. This sprint will define the foundational user stories that drive the first implementation sprints. Key entities to model:

- **Organization:** The tenant. Has a name, settings, and owns all boats/catalog/users.
- **Boat:** Belongs to an org. Has a name, description, and cabin layout.
- **Cabin:** Belongs to a boat. Has a name/number, type, and capacity.
- **Trip:** A bounded operational window on a specific boat. Has start/end dates, manifest, and crew assignment.
- **Catalog Item:** An item or service available for purchase (equipment rental, food, beverage, merch, services). Has pricing, category, and optional inventory tracking.
- **User/Auth:** The Org Admin needs to log in and be authorized at the org level.

## Constraints

- Must follow project conventions in CLAUDE.md (Go backend, tests required, code must compile, go vet must pass)
- Online-default — no offline/sync complexity
- Multi-tenant with strict data isolation between organizations
- Local development only for now (no cloud deployment considerations)
- This sprint is about defining user stories, not implementation — the stories will drive subsequent implementation sprints

## Success Criteria

- Complete set of user stories covering all Organization Admin responsibilities
- Stories are specific enough to be implementable in focused sprints
- Stories cover: auth, org management, fleet management (boats + cabins), trip management, catalog management
- Clear acceptance criteria per story
- Logical grouping and prioritization of stories

## Open Questions

1. Should the Org Admin also handle user management (inviting Site Directors, other admins)?
2. How granular should catalog pricing be — per-org only, or per-boat/per-trip overrides?
3. Should trip creation include guest manifest setup, or is that a Site Director concern?
4. What reporting/analytics capabilities should the Org Admin have at this stage?
