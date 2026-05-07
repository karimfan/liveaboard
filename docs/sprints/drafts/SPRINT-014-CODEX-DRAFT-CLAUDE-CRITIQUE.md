# Sprint 014 Codex Draft — Claude Critique

Critique of `SPRINT-014-CODEX-DRAFT.md` against:
- `SPRINT-014-INTENT.md` and its interview/default assumptions
- Existing codebase conventions in `internal/auth/`, `internal/store/`, `internal/httpapi/`
- Project rules in `CLAUDE.md`, `docs/sprints/README.md`

---

## Valid Strengths Worth Preserving

1. **Separate `guest_users` / `guest_sessions` from `users` / `sessions`.** This is the right architectural call. It keeps the existing `auth.Service`, `RequireOrgAdmin`, `UserFromContext`, and admin-chrome authorization paths completely unaffected by guest access. Every flow in the current codebase assumes `users` is staff-only; the separate table preserves that invariant cleanly.

2. **JSONB payload with server-side Go struct validation.** Balances schema evolution (no ALTER TABLE per new optional section) with type safety at the service boundary. The `custom` section extension path is forward-looking without being over-engineered. The API validates against typed Go structs; the database stores the server-approved document — this is the right split.

3. **Reuse of `internal/auth/tokens.go` for invite tokens and guest sessions.** `NewToken()` and `HashToken()` are already tested and follow the project's opaque-token-hashed-at-rest pattern. Using them for guest invitations avoids duplicating crypto primitives and makes both flows auditable under the same conventions.

4. **Double-scope (`organization_id` + `trip_id`) on every manifest table and query.** Matches the multi-tenant isolation pattern established in Sprints 009–013. The draft explicitly states this as a rule and reflects it in the schema for all six tables.

5. **Partial unique index on active invitations.** `(trip_guest_id) WHERE accepted_at IS NULL AND revoked_at IS NULL` allows historical invite rows after rotates/resends while preventing more than one live invite per guest manifest row. This is the correct tradeoff.

6. **Invite token rotation on resend.** Rotating `token_hash` and extending `expires_at` while preserving the same `trip_guest` row is the right approach. It invalidates stale links without creating orphan manifest entries or changing the guest's email-to-trip binding.

7. **Deferred file storage via `guest_document_metadata`.** Keeping a metadata table without binary storage avoids premature commitment to local storage or a cloud object store. The `storage_backend = 'deferred'` sentinel communicates intent to future implementers.

8. **Authorization helper `canManageTripManifest`.** Centralizing the Org Admin vs Cruise Director scope check into one helper prevents repetition across manifest endpoints and matches the pattern of `RequireOrgAdmin` as a reusable middleware.

9. **Phase ordering.** Schema → guest auth → API → admin UI → guest UI → docs is the correct sequencing. Each phase has a clear input/output boundary and Phase 1 unblocks all downstream phases.

10. **Definition of Done is specific and checkable.** Every DoD item maps to a concrete behavior or test assertion rather than vague "tests pass" criteria.

11. **Security Considerations section is thorough.** Log sanitization, cookie flag parity with staff sessions, minimal list responses, and the future retention-hook design (timestamps + separable metadata) are all worth preserving in the final doc.

---

## Major Concerns

### 1. Token Leaked Through GET Query Parameter

The draft proposes:

```
GET /api/guest/invitations/lookup?token=...
```

A secret token in a GET query parameter will appear in server access logs, browser history, and Referer headers on any subsequent navigation. The Security Considerations section says "Store only token hashes, never raw invite or session tokens" — the same principle applies to transport.

The existing staff invitation flow uses path parameters: `fmt.Sprintf("%s/invitations/%s/accept", ...)` in `auth/invitations.go`. The frontend route is already `/guest/invitations/:token` (path param). The API should match:

```
GET  /api/guest/invitations/{token}         (public lookup)
POST /api/guest/invitations/{token}/accept  (create/authenticate guest + set session)
```

This is a security issue that must be fixed before the route ships.

### 2. Password Verification Placed in the Store Layer

Phase 1 tasks include:

> Implement `FindOrCreateGuestUserForInvite`: create a guest user if email is new, or authenticate the existing guest user password if email already exists.

This name implies the store package would perform bcrypt comparison. In the existing codebase the separation is strict: `internal/store/` performs only data access; `internal/auth/` performs bcrypt. See `auth.go:Login()` — it fetches the user via `s.Store.UserByEmail`, then calls `bcrypt.CompareHashAndPassword` in the auth layer.

The store should expose `CreateGuestUser` and `GuestUserByEmail`; the auth service in `guest_accounts.go` handles credential checking. If bcrypt ends up in the store layer, test performance will degrade (bcrypt is intentionally slow) and the layering that every other flow relies on will be broken.

### 3. `trip_guests.status` Has a Dual-Write Problem

`trip_guests.status` stores a text enum (`invited`, `account_created`, `registration_draft`, `submitted`, `revoked`, `expired`), but this state is also derivable from related table columns: `guest_trip_invitations.accepted_at`, `guest_trip_invitations.revoked_at`, `guest_trip_invitations.expires_at`, and `guest_trip_registrations.status`. Keeping the denormalized status field in sync requires every state-changing path to atomically update both the originating row and `trip_guests.status`.

The draft must resolve this before implementation. **Option A**: Status is fully derived at read time via joins — simpler to keep consistent, slightly more query complexity. **Option B**: Status in `trip_guests` is authoritative and every state-changing operation updates it in the same transaction — the draft must then enumerate every transition and prove each path is atomic. The current draft leaves this implicit, which means status drift is a silent correctness risk.

### 4. `expired` Status: Stored vs Computed — Must Be Resolved Before Implementation

The Open Questions section asks: "Should expired guest invitations automatically set `trip_guests.status` to expired, or should expiry be computed dynamically at read time?" This is not a question to leave open — any test that asserts on `expired` status will be wrong or missing until someone decides.

Recommendation: compute expiry dynamically at read time by comparing `guest_trip_invitations.expires_at` against `now()`. This matches how the existing staff invitation flow works (`LookupInvitation` checks `inv.ExpiresAt.Before(s.now())` at lookup time rather than writing back a status) and avoids needing a background job.

### 5. Re-Invite After Revoke Is Undefined

`trip_guests` has `UNIQUE (organization_id, trip_id, email)`. If an admin revokes a guest's invite and then wants to re-invite the same email to the same trip, the unique constraint blocks a new INSERT. The draft doesn't address whether:

- The revoked row should be reactivated (status reset, new invitation row inserted), or
- Revoke is permanent and the same email cannot be re-added to the same trip.

This needs a clear decision in the DoD. The admin UI resend/revoke actions depend on it.

### 6. Guest Logout Not in API Table

The API endpoint table lists 10 endpoints; none delete a guest session. The `guest_sessions` table has `revoked_at` so revocation is designed for. A 7-day session cookie on a shared device — common in a dive resort where guests may use lobby computers — without a logout endpoint is a real operational risk. Even a minimal `DELETE /api/guest/session` that marks `guest_sessions.revoked_at` should be in scope for this sprint. Omitting it should be an explicit decision, not an oversight.

### 7. Admin Reading Draft Registrations Raises Privacy Concerns

The endpoint `GET /api/admin/trips/{id}/guests/{guest_id}/registration` is documented as "View submitted or draft registration details." Allowing operators to read draft data means incomplete, exploratory guest work — partial medical notes, a tentative travel document — is visible before the guest intentionally submits.

The final sprint doc should resolve this: either restrict the admin endpoint to `submitted` status only, or explicitly document that operators can see draft data and surface this in the UI as "registration in progress — not submitted." Both choices are defensible but neither is acceptable left implicit.

---

## Missing Implementation Details

### A. Guest Session TTL Not Specified

The auth service has named duration knobs (`SessionDuration`, `InvitationDuration`, etc.) on `auth.Service`. The draft says to add a guest invitation duration knob (7 days) but never specifies the guest session duration. Guests may need to return over several weeks to complete registration — a 14-day session matching staff is probably too short; 30–90 days is more appropriate. This should be a named `GuestSessionDuration` knob with a documented default.

### B. No `internal/store/guest_users_test.go`

The files summary lists test files for `trip_guests` and `guest_registrations` but not for `guest_users.go`. Guest account creation, duplicate-email detection (unique constraint on `guest_users.email`), and guest session create/validate/revoke need store-layer tests. This is especially important because the unique-email behavior under concurrent INSERT is subtle.

### C. `guest_document_metadata` Has No API Surface

The schema includes six tables; `guest_document_metadata` is one of them. But no API endpoint in the table creates, reads, or deletes its rows. The DoD says "File binary upload is either not exposed or implemented only as metadata placeholders." If no endpoint uses this table in this sprint, including it in the migration adds schema complexity with zero test coverage and no verifiable behavior. See the rejection recommendation below.

### D. Dietary Acknowledgement Field Missing from Payload Contract

The validation section says "Required on final submit: ... explicit dietary/allergy acknowledgement even when empty." But the payload contract has no acknowledgement field in the `dietary` section:

```json
"dietary": {
  "dietary_requirements": "Vegetarian",
  "allergies": "Peanuts",
  "medical_notes": ""
}
```

There is nothing to validate as "acknowledged." Either add `"dietary_acknowledged": true` to the `dietary` section (a boolean the guest must explicitly set), or drop the acknowledgement requirement from the validation spec. As written the requirement cannot be implemented.

### E. Rate Limiting on Guest Token Validation

The existing auth system applies `auth.Throttle` to login to prevent brute-force attacks. The guest accept endpoint (`POST /api/guest/invitations/{token}/accept`) accepts a password from an actor who already has a valid token and a known email. Without rate limiting, an attacker with a valid invite token can brute-force an existing guest's password. The draft should specify whether `auth.Throttle` is applied here or an alternative per-token attempt counter is used.

### F. Chi Router Group Structure Not Specified for `httpapi.go`

The draft says to "mount staff, public, and guest routes" in `httpapi.go` but doesn't specify the chi router grouping or middleware wrapping. In the existing code, staff routes are wrapped with `SessionMiddleware`. Guest routes need `GuestSessionMiddleware`. Public token routes need no middleware. The final sprint doc should specify the chi group structure to prevent accidentally applying the wrong middleware — for example, applying `RequireOrgAdmin` to a public guest route is a security misconfiguration that would go undetected until a specific negative test catches it.

### G. Email Send Failure Handling — Transaction Semantics Unresolved

The risks section says "create manifest/invite transaction first, attempt send second, and mark status/invite_sent_at only after successful send." But the backend flow shows INSERT + SEND together without stating the fallback. The Open Questions section lists this too, meaning it's explicitly unresolved.

The existing staff invitation flow in `auth/invitations.go:Invite()` does not roll back the invitation row if email fails — it creates the row, attempts send, and returns an error while leaving the row. Guest invitations should follow the same pattern for consistency: keep the manifest row, keep the invitation row, surface a retriable "email not sent" state that the resend action can recover. The migration or service should add a field to track this (e.g., `invite_sent_at` remains NULL until send succeeds; the manifest shows "invite pending" until then).

### H. Capacity Check Race Condition Not Addressed

The draft mentions "lock existing manifest rows for count" but doesn't specify the locking mechanism. Without a `SELECT ... FOR UPDATE` on the trip row or a serializable transaction, two concurrent POST requests to add guests to the same trip can both pass the capacity check and both INSERT, exceeding the cap. The implementation plan should specify the concurrency control explicitly.

---

## Suggested Changes

1. **Change GET token lookup to path parameter**: `GET /api/guest/invitations/{token}` and `POST /api/guest/invitations/{token}/accept`. Update the email link format and frontend route to match. This is a required security fix.

2. **Split `FindOrCreateGuestUserForInvite`**: expose `GuestUserByEmail` and `CreateGuestUser` in the store; move bcrypt comparison to `auth/guest_accounts.go:AcceptGuestInvite()`.

3. **Resolve `status` computation strategy** before implementation. Document the choice in the sprint doc. Add a DoD item: "All `trip_guests.status` transitions are atomic and covered by a test."

4. **Resolve `expired` computation** — prefer dynamic at read time via join on `guest_trip_invitations.expires_at`. Remove `expired` from the `status` CHECK constraint if status is computed dynamically.

5. **Define re-invite-after-revoke behavior** and add a test case. Most natural: allow reactivating a revoked manifest row by creating a new invitation row, resetting `status` to `invited`.

6. **Add a guest logout endpoint** (`DELETE /api/guest/session`) to the API table. Implement as a single `RevokeGuestSession` call in the store.

7. **Restrict admin registration view to `submitted` only** for MVP, or explicitly document and display "draft visible" status. Add a DoD item for whichever is chosen.

8. **Add `GuestSessionDuration` knob** to the guest service struct with a documented default (suggest 30 days).

9. **Add `internal/store/guest_users_test.go`** to the files summary.

10. **Defer `guest_document_metadata`** entirely from the migration if no API surface uses it in this sprint. Keep a note in the sprint doc that a storage sprint will add this table when an upload endpoint exists.

11. **Add `dietary_acknowledged bool` to the `dietary` section** of the payload contract, or remove the explicit-acknowledgement requirement from the validation spec.

12. **Specify throttling on guest accept endpoint** — apply existing `auth.Throttle` pattern or note a per-token attempt counter.

13. **Specify chi router group structure** for public, guest-session, and staff routes in the `httpapi.go` tasks section.

14. **Fix phase percentage sum**: Phases sum to 110% (30+20+20+15+15+10). Adjust so they total 100%.

15. **Resolve email send failure semantics** inline: adopt the existing invitation pattern (keep the row, surface send failure as a retriable state via `invite_sent_at = NULL`).

---

## Risks the Final Merge Should Address

### Token Log Leakage
If `GET /api/guest/invitations/lookup?token=...` ships as written, the raw invite token will appear in every server access log line for that request. This must be fixed to a path parameter before the route ships.

### Store-Layer Auth Creep
If `FindOrCreateGuestUserForInvite` includes bcrypt in the store package, it violates the auth/store layering that every other flow respects, and it makes the store test suite slow (bcrypt cost 12 = ~250ms per hash). This becomes hard to untangle later.

### Status Drift Under Concurrent Operations
If `trip_guests.status` is stored and two concurrent operations (guest submitting + admin viewing) read-modify-write without serialization, the status can silently drift. The implementation must use row locking (`SELECT ... FOR UPDATE` on the `trip_guest` row) for all status-changing paths, or derive status at read time.

### Guest Session Cookie Scope Overlap
If a staff user opens their own registration link in the same browser session, both `lb_session` and `lb_guest_session` cookies will be set. Guest-session middleware must ignore `lb_session` and vice versa. If any middleware mistakenly accepts either cookie for either route type, a staff session could be used to access guest routes or a guest session could escalate to staff routes. This is a correctness risk requiring explicit tests.

### Capacity Check Race Condition
Concurrent manifest additions can both pass a non-locking capacity check and both INSERT. The `CreateTripGuestWithInvite` transaction must use a locking read (`SELECT ... FOR UPDATE` or equivalent) on the trip row to serialize concurrent adds.

### SPRINT-014.md Already Exists
The git status shows `docs/sprints/SPRINT-014.md` as an untracked new file. Confirm whether this is an authoritative partial draft or a placeholder before writing the final sprint doc.

### Trip Status Column May Not Exist
The draft references blocking mutations for `completed/cancelled` trips. Confirm that a `status` column (or equivalent state) exists on the `trips` table in Sprint 013's migration before writing logic that depends on it. If it doesn't exist, the mutation guards cannot be implemented as described.

---

## Parts That Should Be Rejected or Simplified

### Reject: `guest_document_metadata` Without Upload API
A migration table with no application code, no API surface, and no tests in this sprint adds schema complexity with zero verifiable behavior. Defer the entire table to the storage sprint when an upload endpoint will exist and be tested. The DoD already says binary upload is deferred; the metadata table should follow.

### Reject: Hard Capacity Enforcement on `trips.num_guests`
`trips.num_guests` was imported in Sprint 012 as expected/booking count from liveaboard.com — it represents anticipated occupancy, not vessel capacity. Using it as a hard cap will block legitimate manifest additions when the imported value is stale, missing, or only a booking estimate. Either:
- Add an explicit operator-owned `trips.guest_capacity` column in this sprint, or
- Treat `num_guests` as a display hint with a soft warning on overage (not a hard block).
The draft acknowledges this risk in the risks section but still specifies hard-cap enforcement in the DoD. These two statements contradict each other and must be reconciled.

### Simplify: Registration UI — Scrollable Form Over Stepper for MVP
Phase 5 calls for "tabs or stepper: Identity, Travel, Emergency, Diving, Dietary, Gear, Notes." A stepper requires each section to independently handle draft-save, back-navigation state preservation, and session-recovery state. For a first-sprint feature tested primarily by operator staff, a single scrollable form with section headers is sufficient, easier to test, and easier to extend later. Defer the stepper UX to when the guest portal grows.

### Simplify: Resolve All Six Open Questions Inline
Per the planner's interview defaults, six of the seven open questions are already answered:

| Open Question | Answer (from interview defaults) |
|---|---|
| Separate `guest_users` or extend `users`? | Separate `guest_users` — stated in the draft architecture |
| Implement file uploads or defer? | Defer to metadata placeholders only |
| Who can add guests in which trip states? | Both Org Admin and assigned CD: planned and active trips only |
| Resend/revoke in first sprint? | Yes, include both |
| Draft-save before submit? | Yes, include draft-save |
| Fixed schema or configurable per-org fields? | Fixed schema for MVP |

The only genuinely open question is guest logout. The final sprint doc should close all six answered questions inline and record guest logout as an explicit known gap with a note that it is a follow-up.
