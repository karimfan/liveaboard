# Sprint 002: Organization Admin User Story Backlog

## Overview

This is a documentation sprint. Its purpose is to produce a complete, prioritized, implementation-ready user story backlog for the Organization Admin persona, plus the persona-boundary doc that scope decisions hang off.

The repo is greenfield from an application perspective. Rather than starting screens or APIs immediately, this sprint defines the durable product artifacts that subsequent implementation sprints (003+) will execute against. No application code, schema, or APIs are produced here.

Inputs synthesized:
- `docs/sprints/drafts/SPRINT-002-INTENT.md` — seed and open questions.
- `docs/sprints/drafts/SPRINT-002-CLAUDE-DRAFT.md` — concrete story inventory.
- `docs/sprints/drafts/SPRINT-002-CODEX-DRAFT.md` — phased plan, story format, persona scaffolding.
- `docs/sprints/drafts/SPRINT-002-CLAUDE-DRAFT-CODEX-CRITIQUE.md` — structural critique.
- `docs/sprints/drafts/SPRINT-002-MERGE-NOTES.md` — final merge decisions.

## Use Cases

1. **Drive implementation sprints**: a downstream sprint can pull a story by ID and have enough acceptance criteria to build and verify it.
2. **Resolve persona scope disputes**: when a feature could plausibly belong to two personas, `personas.md` is the authority.
3. **Communicate non-goals**: explicitly enumerate what is deferred so the team does not silently re-scope mid-implementation.
4. **Capture product decisions**: pricing model, manifest ownership, lifecycle ownership, user-management subset are recorded once with rationale.

## Architecture

This sprint produces a product backlog architecture, not runtime architecture. The story map preserves the domain boundaries that will later shape database tables, API resources, and UI navigation:

```
Organization
  ├── Users / Roles
  ├── Boats
  │     ├── Cabins (with berth-level units, e.g., 1A, 1B)
  │     └── Trips
  │           ├── Site Director assignment
  │           ├── Pre-departure manifest (Org Admin)
  │           └── Mid-trip manifest + lifecycle (Site Director)
  └── Catalog
        ├── Categories
        └── Items / Pricing
```

### Story format

Each story uses:

```
### US-N.M: Title

> As an Organization Admin, I want ... so that ...

Priority: Must | Should | Could
Area:     Auth | Organization | Fleet | Trips | Catalog | Users | Oversight
Depends on: US-X.Y, ... (or None)

Acceptance Criteria:
- [ ] Behavior-level criteria with validation, authorization, error/empty states.

Notes: persona boundaries, deferred follow-ups, future-implementation hints.
```

Mechanism choices (JWT vs session, hashing algorithm, exact expiry windows, etc.) belong to the implementation sprint that builds the feature, not to the story backlog.

## Implementation Plan

### Phase 1: Persona doc + format scaffold (~15%)

**Files:**
- `docs/product/personas.md` - Persona definitions and boundaries.

**Tasks:**
- [ ] Create `docs/product/` directory.
- [ ] Write `personas.md` covering Organization Owner, Organization Admin, Site Director, Crew, and Guest.
- [ ] For each persona, document scope, what they own, what they do not own, and boundary notes.
- [ ] Add a cross-persona decision table for capabilities that could belong to multiple personas.

### Phase 2: Port and restructure existing stories (~40%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Story backlog.

**Tasks:**
- [ ] Create the story doc with conventions, product decisions, non-goals, security considerations, and story-map sections.
- [ ] Port US-1.x (Auth), US-2.x (Org), US-3.x (Fleet), US-4.x (Trips), US-5.x (Catalog) from the Claude draft into the new format.
- [ ] Add `Priority`, `Area`, and `Depends on` to every story.
- [ ] Remove implementation-mechanism details from acceptance criteria (move to `Notes` or implementation-sprint scope).

### Phase 3: Add missing groups (~20%)

**Files:**
- `docs/product/organization-admin-user-stories.md`

**Tasks:**
- [ ] Add Group 6 (Users): US-6.1 (invite Site Director), US-6.2 (deactivate user), US-6.3 (resend invitation).
- [ ] Add Group 7 (Oversight): US-7.1 (setup completeness), US-7.2 (operational status), US-7.3 (revenue summary), US-7.4 (cross-trip analytics — deferred).
- [ ] Add deferred catalog stories: US-5.6 (per-boat/per-trip pricing overrides), US-5.7 (inventory tracking).

### Phase 4: Rescope manifest + lifecycle stories (~15%)

**Files:**
- `docs/product/organization-admin-user-stories.md`

**Tasks:**
- [ ] Rescope US-4.5, US-4.6, US-4.7 to **pre-departure manifest preparation only** (status `planned`).
- [ ] Replace Org Admin start/complete stories with US-4.8 (monitor lifecycle, read-only) — Site Director owns the transitions.
- [ ] Add US-4.10 (assign Site Director to trip) as Must, depends on US-6.1.
- [ ] Keep US-4.9 (cancel `planned` trip) with Org Admin.

### Phase 5: Sequencing + slicing (~10%)

**Files:**
- `docs/product/organization-admin-user-stories.md`
- `docs/sprints/SPRINT-002.md` (this file)

**Tasks:**
- [ ] Group stories into Must / Should / Could buckets.
- [ ] Identify dependencies between stories (boats before trips, catalog before per-trip ledger, invite before assign).
- [ ] Propose 5–6 follow-up implementation sprints (003–008).
- [ ] Resolve open questions from the intent in the story doc's Product Decisions section.
- [ ] Sync `tracker.tsv` with this sprint via `go run docs/sprints/tracker.go sync`.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/product/personas.md` | Create | Persona scope and boundaries; source of truth for cross-persona scope decisions. |
| `docs/product/organization-admin-user-stories.md` | Create | Canonical Org Admin backlog with priority, area, dependencies, and acceptance criteria. |
| `docs/sprints/SPRINT-002.md` | Create | This sprint document. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 002 via `tracker.go sync`. |

## Definition of Done

- [ ] `docs/product/personas.md` exists and defines Org Owner, Org Admin, Site Director, Crew, and Guest with handoff boundaries.
- [ ] `docs/product/organization-admin-user-stories.md` exists with all stories in the documented format.
- [ ] Every story has `Priority`, `Area`, `Depends on`, and behavior-level acceptance criteria.
- [ ] User management MVP stories (US-6.1, US-6.2) and Site Director assignment (US-4.10) are present and marked Must.
- [ ] Oversight stories (US-7.1, US-7.2) are present and marked Must.
- [ ] Manifest stories US-4.5–4.7 are explicitly rescoped to pre-departure (`planned` status only).
- [ ] Trip lifecycle ownership reflects Site Director boundary (US-4.8 monitor-only, no Org Admin start/complete).
- [ ] Non-Goals section enumerates everything explicitly deferred.
- [ ] Open questions from the intent are resolved or recorded as explicit recommendations.
- [ ] `tracker.tsv` shows Sprint 002.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Scope expands into a full PRD | Medium | Medium | Restrict to Org Admin scope; enforce non-goals section. |
| Stories are too vague for implementation | Medium | High | Require behavior-level AC including validation/authorization. |
| Stories overfit a UI before architecture exists | Medium | Medium | Describe behavior and outcomes, not component layouts. |
| User management omitted, blocking Site Director work | Low | High | Group 6 is Must, with US-4.10 marking the explicit dependency. |
| Site Director scope leaks into Org Admin | Medium | Medium | `personas.md` boundary table; manifest stories pinned to `planned` status. |
| Pricing assumptions block ledger design | Medium | Medium | Document flat-pricing decision; capture overrides as deferred Could. |

## Security Considerations

- Every Org Admin action requires authentication and an Org Admin role on the requested organization.
- All reads and mutations are org-scoped; cross-tenant access is rejected.
- Auth flows do not enable email enumeration in login, signup, or password reset.
- Sessions expire and can be invalidated server-side. Passwords are stored hashed.
- Soft-deletion preserves trip and ledger references; nothing is hard-deleted that is referenced by historical data.

## Dependencies

- Sprint 001 (Go tracker tooling) — useful for registering this sprint via `tracker.go sync`, but not functionally required to produce the docs.
- `CLAUDE.md` constraints (Go backend, TypeScript/React frontend, PostgreSQL with row-level security, tests required) apply to the downstream implementation sprints.
- `DESIGN.md` applies to future UI work; this sprint produces no UI.

## Open Questions (from Intent) — Resolved

| Question | Resolution |
|---|---|
| Should Org Admin handle user management? | Yes — minimum subset: invite Site Director (US-6.1), deactivate user (US-6.2), assign trip leadership (US-4.10). Advanced role admin deferred. |
| Catalog pricing granularity? | Org-level flat pricing for MVP. Per-boat/per-trip overrides captured as US-5.6 (`Could`). |
| Trip creation includes manifest setup? | Org Admin prepares the initial manifest pre-departure (US-4.5–4.7, `planned` only). Mid-trip manifest ops are Site Director. |
| Reporting/analytics? | MVP: setup completeness (US-7.1) + operational status (US-7.2). Revenue summary (US-7.3) is `Should`. Cross-trip analytics (US-7.4) deferred to Owner persona. |
| Inventory tracking? | Deferred — US-5.7 captured as `Could`. |
| Trip booking fees vs onboard catalog? | Out of scope. Catalog covers onboard consumption only. |
