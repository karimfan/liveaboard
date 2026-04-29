# Sprint 002 Merge Notes

## Inputs Compared

- `SPRINT-002-INTENT.md` — seed and open questions.
- `SPRINT-002-CLAUDE-DRAFT.md` — concrete story inventory with acceptance criteria.
- `SPRINT-002-CODEX-DRAFT.md` — phased documentation-sprint plan with story format and persona scaffolding.
- `SPRINT-002-CLAUDE-DRAFT-CODEX-CRITIQUE.md` — structural critique of the Claude draft.

## Claude Draft Strengths

- Concrete, testable acceptance criteria for 25+ stories grouped by domain (auth, org, fleet, trips, catalog).
- Sensible MVP product decisions: flat org-level pricing, soft deletion, no overlapping trips, inline cabin definition with berth-level units (1A/1B).
- Pragmatic implementation ordering across 5 future sprints.
- Strong security instincts (no email enumeration, generic auth errors, password reset expiry, org-scoped queries).
- Surfaced cabin/berth modeling early — important domain decision that would otherwise leak into trip work.

## Codex Critique Strengths

- Caught that the draft lacks a canonical artifact location and Files Summary required by the repo sprint template.
- Identified that deferring user management entirely conflicts with the need for Site Director assignments on trips.
- Flagged manifest ownership leak: README says Site Director owns manifest, draft assigns it to Org Admin.
- Noted that stories lack Must/Should/Could priorities, which blocks MVP slicing.
- Pointed out that some acceptance criteria over-specify implementation (JWT, bcrypt, password rules) for a story-definition sprint.
- Surfaced gaps: admin oversight/reporting stories, inventory and availability decisions, trip lifecycle role boundaries.

## Codex Draft Strengths

- Treats this as a documentation sprint with phased plan and explicit non-goals.
- Defines a consistent story format (ID, Priority, Area, Acceptance Criteria, Notes).
- Names the durable artifact: `docs/product/organization-admin-user-stories.md` plus `docs/product/personas.md`.
- Calls out persona boundaries as a first-class deliverable.
- Risks section explicitly lists "Site Director responsibilities leak into admin scope" — matches the critique.

## Critiques Accepted

1. **Add canonical artifact path.** Stories live in `docs/product/organization-admin-user-stories.md`. Persona boundaries live in `docs/product/personas.md`.
2. **Add Files Summary section** matching `docs/sprints/README.md` template.
3. **Add Must/Should/Could priorities** to every story, plus Area and dependency metadata.
4. **Restore minimum user management for MVP**: invite Site Director, assign trip leadership, deactivate user. Defer advanced role admin (multi-admin, custom roles, granular permissions).
5. **Split manifest stories.** Org Admin can create the trip shell and prepare an initial manifest pre-departure. Live manifest operations (mid-trip add/remove/reassign, ledger entry) are Site Director-owned and out of scope here.
6. **Soften over-specified security AC.** Keep behavior-level requirements (no enumeration, sessions expire, passwords hashed) but move mechanism choices (JWT vs session, bcrypt vs argon2, exact expiry windows) to implementation notes for the future implementation sprints.
7. **Add admin oversight stories** for setup completeness and operational trip status as MVP. Defer revenue/analytics to a later tier.
8. **Add explicit deferred stories** for inventory tracking, per-boat/per-trip pricing overrides, deeper analytics, guest self-service — so they are not lost.
9. **Decide trip lifecycle ownership.** Org Admin can plan/cancel/configure trips and monitor active/completed ones. Site Director performs start/complete transitions during operations. Reflect this in US-4.8 and US-4.9.
10. **Add a non-goals section**: live ledger entry, guest self-service, deep analytics, offline/sync, cloud deployment, advanced role admin, manifest mid-trip operations.
11. **Reframe "Interview Decisions" as "Product Decisions"** with explicit confirmed/recommendation status, since no live interview transcript exists in this draft cycle.

## Critiques Rejected (with reasoning)

1. **Renaming story IDs to `OA-XX`.** Codex draft proposed `OA-001` etc. Keep Claude's `US-N.M` grouping — it makes the area visible in the ID and matches the existing Group structure. Lower disruption.
2. **Creating a separate Phase 5 review pass.** Folded into Definition of Done checks. The sprint is small enough that a dedicated review phase is overhead.
3. **Writing implementation notes naming JWT/bcrypt/argon2 in the story doc.** Even moved out of AC, mechanism choices belong in the implementation sprint that builds auth, not in the story backlog. Keep a short "Security Considerations" section at story-doc top with behavior requirements only.

## Open Questions Resolved

| Question (from intent) | Resolution |
|---|---|
| Should Org Admin handle user management? | Yes — minimum: invite Site Director, assign trip leadership, deactivate user. Advanced role admin deferred. |
| Catalog pricing granularity? | Org-level flat pricing for MVP. Per-boat/per-trip overrides captured as deferred Could stories. |
| Trip creation includes manifest setup? | Org Admin creates trip shell and may prepare initial manifest pre-departure. Live manifest ops are Site Director scope. |
| Reporting/analytics? | MVP: setup completeness + operational trip status. Revenue summaries Should. Deep analytics Could/deferred. |
| Inventory tracking? | Deferred. Captured as explicit Could story so it is not lost. |
| Trip pricing (booking fees) vs onboard catalog? | Out of scope for Sprint 002. Catalog covers onboard consumption only. |

---

## Final Sprint 002 Proposal

### Title

`Sprint 002: Organization Admin User Story Backlog`

### Type

Documentation sprint. No application code, schema, or APIs produced.

### Deliverables

1. `docs/product/organization-admin-user-stories.md` — primary backlog using Claude's story content as base, with structural changes below.
2. `docs/product/personas.md` — persona boundary doc covering Organization Owner, Organization Admin, Site Director, Crew, Guest. Defines what each persona owns and where handoffs occur.
3. `docs/sprints/SPRINT-002.md` — the sprint plan itself (this merge synthesized into the standard sprint template).

### Story Doc Structure

Each story uses this format:

```markdown
### US-N.M: Title

> As an Organization Admin, I want ... so that ...

**Priority:** Must | Should | Could
**Area:** Auth | Organization | Fleet | Trips | Catalog | Users | Oversight
**Depends on:** US-X.Y (or None)

**Acceptance Criteria:**
- [ ] Behavior-level criteria, validation, authorization, empty/error states.
- [ ] Org-scoped data access where relevant.

**Notes:** Persona boundaries, deferred follow-ups, future-implementation hints.
```

### Story Coverage

Take Claude's existing US-1.x through US-5.x as the base, with these edits:

- **US-1.x Auth:** Keep as-is. Drop JWT/bcrypt/argon2 from AC; move to Notes.
- **US-2.x Organization:** Keep. Add persona note that owner-level operations (billing, deletion) are out of scope.
- **US-3.x Fleet:** Keep including berth-level cabin model.
- **US-4.x Trips:**
  - US-4.1 (create), US-4.2 (list), US-4.3 (view), US-4.4 (edit), US-4.10 (cancel): keep with Org Admin.
  - US-4.5 (add guest), US-4.6 (remove guest), US-4.7 (reassign): rescope to **pre-departure manifest preparation only**. Add Notes that mid-trip operations are Site Director scope.
  - US-4.8 (start), US-4.9 (complete): change owner to Site Director with a cross-reference; Org Admin role is monitoring only.
  - **Add US-4.11 (Must):** Assign a Site Director to a trip — required to unblock Site Director workflows.
- **US-5.x Catalog:** Keep. Add deferred Could stories for inventory tracking, per-boat/per-trip pricing overrides.
- **New Group 6: Users (Must subset only).**
  - US-6.1 (Must): Invite Site Director by email.
  - US-6.2 (Must): Deactivate a user.
  - US-6.3 (Should): Re-send pending invitation.
  - Note: multi-admin, custom roles, granular permissions explicitly deferred.
- **New Group 7: Oversight (Must subset only).**
  - US-7.1 (Must): Setup completeness dashboard (which boats lack cabins, which trips lack a Site Director, etc.).
  - US-7.2 (Must): Operational trip status view (planned/active/completed counts, occupancy at a glance).
  - US-7.3 (Should): Revenue summary per trip.
  - US-7.4 (Could): Cross-trip analytics — deferred.

### Non-Goals

- Live ledger entry / mid-trip catalog consumption flows
- Guest self-service
- Deep analytics and reporting beyond setup + status
- Offline / sync
- Cloud deployment concerns
- Advanced role administration (multi-admin, custom roles, granular permissions)
- Mid-trip manifest operations (Site Director scope)
- Trip booking fees / external pricing

### Implementation Phases (sprint-internal)

1. **Persona doc + story format scaffold** (~15%) — write `personas.md`, agree story format.
2. **Port + restructure existing stories** (~40%) — move Claude's US-1.x..US-5.x into the new format, add priorities, areas, dependencies; soften over-specified AC.
3. **Add new groups** (~20%) — Users (Group 6) and Oversight (Group 7) stories.
4. **Rescope manifest + lifecycle stories** (~15%) — split US-4.5–4.7, change ownership of US-4.8–4.9, add US-4.11.
5. **Sequencing + sprint slicing** (~10%) — group into MVP / near-term / later, propose 3–5 follow-up implementation sprints, write Definition of Done.

### Suggested Follow-Up Implementation Sprints

1. **Sprint 003 — Auth + Org foundation:** US-1.1, 1.2, 1.3, 2.1.
2. **Sprint 004 — Fleet:** US-3.1–3.5.
3. **Sprint 005 — Catalog + currency:** US-5.1–5.5, US-2.3.
4. **Sprint 006 — Trips + Site Director invite:** US-4.1–4.4, US-4.10, US-4.11, US-6.1, US-6.2.
5. **Sprint 007 — Pre-departure manifest + oversight:** US-4.5–4.7 (rescoped), US-7.1, US-7.2.
6. **Sprint 008 — Polish:** US-1.4, US-1.5, US-2.2, US-6.3, US-7.3.

### Definition of Done

- [ ] `docs/product/organization-admin-user-stories.md` exists with all stories in the new format.
- [ ] `docs/product/personas.md` exists and defines Org Owner, Org Admin, Site Director, Crew, Guest with handoff boundaries.
- [ ] Every story has Priority, Area, Depends on, and behavior-level AC.
- [ ] User management MVP stories (US-6.1, 6.2) and Site Director assignment (US-4.11) are present.
- [ ] Oversight stories (US-7.1, 7.2) are present.
- [ ] Manifest stories rescoped to pre-departure; trip lifecycle ownership reflects Site Director boundary.
- [ ] Non-goals section enumerates everything explicitly deferred.
- [ ] Open questions from the intent are resolved or recorded as explicit recommendations.
- [ ] `docs/sprints/SPRINT-002.md` summarizes this work and the tracker is synced.

### Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/product/organization-admin-user-stories.md` | Create | Canonical Org Admin story backlog with priorities, areas, dependencies, AC. |
| `docs/product/personas.md` | Create | Persona boundaries: Org Owner, Org Admin, Site Director, Crew, Guest. |
| `docs/sprints/SPRINT-002.md` | Create | Sprint document referencing the artifacts above. |
| `docs/sprints/tracker.tsv` | Update | Add Sprint 002 entry via `tracker.go sync`. |
