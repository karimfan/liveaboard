# Sprint 010 Claude Draft: Codex Critique

## Overall Assessment

Claude's draft is directionally strong. It keeps Sprint 010 scoped as
an extension of Sprint 008/009, makes sensible product choices on
invite-time metadata and the Cruise Director landing page, and does a
good job preserving the current auth/admin architecture instead of
inventing a parallel system.

The main weaknesses are not conceptual; they are in a few planning
details that would matter during implementation:

1. the migration plan does not fully cover every role constraint that
   must change,
2. the handling of pre-existing pending invitations is underspecified
   and can create blank-name users or broken greeting UX,
3. the new landing endpoint contract is a bit awkward relative to the
   existing admin namespace and role model.

## Strong Parts

- The draft correctly treats the rename as a full-stack cleanup, not a
  copy-only polish pass.
- The invitation redesign is coherent: admin captures identity, invitee
  sets password, accepted user inherits stored metadata.
- Adding self-serve profile editing under `/admin/account` is the right
  scope level for this sprint.
- The landing-page design is appropriately restrained and consistent
  with `DESIGN.md`.
- The file list and phase structure are mostly realistic and grounded in
  the existing codebase.

## Findings

### 1. Critical: the migration plan updates `users.role` values without updating `users_role_check`

The draft updates `users.role` from `site_director` to
`cruise_director`, but the migration sketch only drops/recreates the
`invitations_role_check`. The repo already has a `users_role_check`
constraint from earlier migrations (`0003_drop_crew_role.sql`), and it
still allows `('org_admin','site_director')`.

If Sprint 010 follows the draft literally, this statement will fail:

```sql
UPDATE users SET role = 'cruise_director' WHERE role = 'site_director';
```

because the old `users_role_check` would reject the new value.

What should change:

- explicitly drop and recreate `users_role_check`,
- sequence it alongside the data rewrite,
- call this out in the migration narrative and Definition of Done.

### 2. High: existing pending invitations are given `full_name = ''`, which conflicts with the new accept-flow contract

The draft uses:

```sql
ALTER TABLE invitations
  ADD COLUMN full_name text NOT NULL DEFAULT ''
```

to keep older rows valid. That is fine mechanically, but the plan then
changes acceptance so the invitee no longer enters a name and the email
/ lookup / accept flow assumes a real `full_name`.

That creates a bad edge case for any pre-Sprint-010 pending invitation:

- invitation email greeting can be blank,
- accept page greeting can be blank,
- accepted `users.full_name` can become `''`.

The draft should make a choice instead of leaving this implicit:

- revoke all pre-Sprint-010 pending invites, or
- require a non-empty fallback name in lookup/accept/email rendering, or
- backfill meaningful names from somewhere else if available.

Without that, the migration is technically valid but the new product
flow is not.

### 3. Medium: `/api/cruise-director/landing` is an awkward contract and weakens the role boundary

The new endpoint is reasonable in spirit, but the plan says:

> Org Admins can call it too — they just see their own contact info plus
> zero trips (or admin-side counts; pick the simpler shape).

That is too loose for a planning document. It mixes two different
personas into an endpoint that is supposed to exist specifically for the
Cruise Director landing page.

Problems with this choice:

- it makes the response semantics fuzzier than necessary,
- it adds a role branch the frontend does not need,
- it dilutes the security/ownership model for no real gain.

Cleaner alternatives:

- make it Cruise-Director-only, or
- keep the endpoint under the existing admin namespace and name it
  something like `/api/admin/cruise-director-overview`, while still
  enforcing `cruise_director` access only.

### 4. Medium: the landing-page data is split across two endpoints when one payload would likely be cleaner

Claude's draft proposes:

- `GET /api/cruise-director/landing` for user/org/counts
- `GET /api/admin/trips` for the actual trips table

That will work, but it increases coupling between two independent
requests for a single small screen and creates more room for drift
between counts and rows.

A single overview payload containing:

- profile/contact card,
- summary counts,
- sorted assigned trips

would likely be simpler to test and simpler to render. This is not a
blocking flaw, but it is a merge-phase design choice worth tightening.

### 5. Medium: the draft implies assignment UX exists more fully than it currently does

The use case says:

> Org Admin opens a trip row in `/admin/trips`, picks a Cruise Director
> from a dropdown.

That reads like an existing UI surface, but the shipped frontend is much
thinner: the backend assignment endpoint exists, while the visible Trips
page is still largely a read-only table. The draft later says the
assignment endpoint/UX is "unchanged in shape," which is not quite the
same thing.

This is mostly a wording issue, but for planning clarity the draft
should separate:

- what already exists in backend capability,
- what is already exposed in UI,
- what Sprint 010 is and is not changing.

## Recommendations For Merge

- Keep Claude's overall sprint framing, landing-page concept, and
  self-service profile decision.
- Fix the migration section so it explicitly handles both
  `users_role_check` and `invitations_role_check`.
- Decide how pre-existing pending invitations are treated after the new
  required `full_name` flow lands.
- Tighten the new landing endpoint contract so it is role-specific and
  does not need an "admins can call it too" fallback.
- Consider collapsing landing header + trip list into one overview
  endpoint unless there is a strong reuse reason to keep them split.
- Clarify the current state of trip-assignment UI so the sprint doc
  does not imply more shipped UX than Sprint 008 actually delivered.

## Bottom Line

Claude's draft is a good base and is close to merge-ready. The biggest
thing it needs is migration rigor: the rename must update every
constraint and account for legacy pending invitations, otherwise the
implementation will either fail at migrate-time or ship a broken blank-
name acceptance edge case.
