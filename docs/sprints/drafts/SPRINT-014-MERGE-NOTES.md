# Sprint 014 Merge Notes

## Codex Draft Strengths

- Correctly split guest accounts/sessions from staff `users` and staff
  sessions.
- Kept the sprint focused on invite-driven registration rather than a
  broad guest portal.
- Modeled registration sections from the Gaia reference in generic
  operator-neutral terms.
- Included resend/revoke, draft save, status tracking, and
  admin/director access.
- Called out sensitive-data and tenant-isolation risks explicitly.

## Critique Strengths

- Flagged query-string invite tokens as a security issue; final route
  uses path tokens to match existing staff invitation links and requires
  request-log redaction for tokenized guest invite paths.
- Flagged auth/store layering around bcrypt; final plan keeps password
  verification in `internal/auth`.
- Identified that `trips.num_guests` is expected/imported guest count,
  not enforceable capacity.
- Identified that trip lifecycle status is described in product docs but
  not present in the current schema.
- Flagged file/document metadata as premature without an upload
  endpoint.
- Flagged duplicated status state across manifest, invitation, and
  registration tables.
- Strengthened privacy and email-send failure handling.

## Valid Critiques Accepted

- No hard capacity enforcement in Sprint 014. The manifest shows expected
  occupancy from `trips.num_guests` and warns when count exceeds it.
  A real capacity model remains a follow-up.
- No dependency on planned/active/completed/cancelled trip status in
  implementation rules. Admins can manage org trip manifests; Cruise
  Directors can manage assigned trip manifests. Lifecycle gating waits
  for a trip-status sprint.
- No binary upload or document metadata table in this sprint. The
  registration payload can include document-related "will provide later"
  acknowledgements and notes only.
- Manifest readiness status is computed from durable invitation/account/
  registration timestamps where possible instead of stored as a
  duplicate status field.
- Staff can see manifest status immediately, but sensitive registration
  detail is exposed to staff only after guest submission.
- Add invite email send state so failed sends create a retryable state
  rather than an ambiguous invited row.
- Add guest-session TTL, guest logout, accept-flow throttling, and
  explicit chi route grouping.
- Allow re-invite after revoke by reusing the existing `trip_guests` row
  and creating a fresh invitation row.

## Interview Defaults Applied

The interactive interview phase was not paused for user input. These
defaults were applied based on the seed prompt and repo context:

- Use separate guest accounts rather than staff users.
- Keep registration data operator-neutral and destination-neutral.
- Defer binary document upload.
- Allow Org Admins and assigned Cruise Directors to add/manage guests.
- Include resend and revoke.
- Include save-draft before submit.
- Avoid folios, checkout, payments, inventory depletion, and dive
  scheduling.

## Final Merge Decisions

- Sprint 014 ships a guest-management foundation: manifest rows,
  invite email, guest account creation, guest session, registration
  draft/save/submit, and staff manifest status.
- Generic guest login, password reset, document upload, custom
  per-operator fields, registration review/locking, and capacity model
  are follow-ups.
- The final sprint document should be `docs/sprints/SPRINT-014.md` and
  tracker status should remain `planned` unless the user explicitly
  starts it while Sprint 013 is still in progress.
