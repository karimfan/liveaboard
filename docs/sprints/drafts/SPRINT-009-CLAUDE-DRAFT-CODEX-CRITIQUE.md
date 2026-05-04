# Critique of `SPRINT-009-CLAUDE-DRAFT.md`

## Findings

1. **The draft conflicts with the binding planner decision on change email.**  
   The Phase 4 refinements in the intent make change email mandatory for
   this sprint and explicitly call for a two-phase flow backed by a new
   `email_change_requests` table. Claude's draft omits that table from
   the schema entirely ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:61)), never adds change-email endpoints to the public HTTP surface ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:186)), never includes a frontend page or account UX for it ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:308)), and then explicitly defers it as an open question ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:632)). That is not just a gap; it directly disagrees with the binding decisions.

2. **The migration section recommends the destructive wipe, but the actual 0007 schema block does not perform it or enforce the final invariant.**  
   The schema snippet starts by dropping Clerk tables and columns, then re-adds `password_hash` as nullable ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:66)). The required `DELETE FROM users; DELETE FROM organizations;` is only mentioned later in prose as an alternative/recommendation ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:134)). For this sprint, the migration needs the delete at the top and `users.password_hash` landing `NOT NULL` in the same migration. Leaving the actual SQL sketch in a half-restored state makes the unwind ordering too ambiguous for the final sprint doc.

3. **Invitation constraints are too loose and partly wrong for the intended feature.**  
   The draft allows invitation roles of both `org_admin` and `site_director` ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:103)), but the intent for this sprint is specifically admin-to-site-director invitation. Expanding the role set adds unneeded behavior and review surface. The uniqueness rule is also underspecified: a plain `UNIQUE (organization_id, email)` constraint on the table ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:115)) blocks future reinvites even after acceptance or revocation unless rows are deleted or mutated in a specific way. What the sprint actually needs is a uniqueness rule for **pending** invitations only, typically via a partial unique index keyed to `accepted_at IS NULL AND revoked_at IS NULL`.

4. **The brute-force defense regresses from the intent’s recommendation by keeping state only in memory.**  
   The intent calls out per-email login attempt counters with exponential cooldown as the minimum viable defense now that Clerk is gone. Claude's draft pushes this into "a small in-memory map" in both risks and security sections ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:577), [SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:592)). That is weaker than the stated requirement because it disappears on restart, is awkward to test deterministically, and leaves no persistent lockout state during local multi-process or repeated manual testing. The sprint doc should either specify a small DB-backed `login_attempts` table or be explicit that the app is accepting a materially weaker defense than the planner asked for.

5. **Email coverage is incomplete for the sprint’s required flows.**  
   The email package only defines templates for verification, invitation, and password reset ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:248), [SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:416)). Once change email is in scope, the sprint also needs a dedicated change-email confirmation template and copy. Without that, the draft does not actually cover every outbound email the sprint is binding itself to ship.

6. **The SMTP test strategy is decent for wire capture, but it does not fully connect to the required "no real Brevo in tests" guarantee for the auth flows themselves.**  
   The draft uses `MockSender` for service tests and a local `net.Listener` for wire-format tests ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:416)). That covers rendering and formatting, but it does not say how token-bearing auth flows will assert against real rendered message contents without coupling every test to an in-memory-only sender. The final sprint doc should be more explicit that service tests inspect captured outbound messages for the expected links/tokens while transport tests separately validate MIME structure against a dummy SMTP listener. Otherwise the no-Brevo strategy is implied rather than fully specified.

7. **The frontend plan under-specifies how the two-phase verification-required login response works.**  
   The draft correctly says login returns generic bad-credentials behavior plus a verification-required state ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:188)), but the frontend section only says "clear 'email not verified — resend?' prompt" without defining how the SPA distinguishes that case from a truly bad credential path ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:55)). Because the binding decision is "generic invalid credentials" outwardly with a separate verification hint after a clean credential check, the sprint doc should pin down a machine-readable response code or field so the client behavior is implementable without violating the enumeration constraint.

8. **The Definition of Done is missing the newly binding change-email flow.**  
   The DoD lists "all seven new auth flows" and invitation coverage, but it never names change email or resend verification separately ([SPRINT-009-CLAUDE-DRAFT.md](/Users/karim/src/liveaboard/docs/sprints/drafts/SPRINT-009-CLAUDE-DRAFT.md:551)). Given the planner’s refinement, change email is no longer optional polish; it should appear explicitly in both automated-test expectations and manual smoke.

## Summary

Claude's draft is directionally sound on the main Clerk unwind, cookie
restoration, and SMTP introduction, but it is not yet aligned with the
binding Phase 4 decisions. The most important corrections are:

- make `0007` explicitly destructive in the SQL plan, not just in prose,
- add the full two-phase change-email design and `email_change_requests`
  schema,
- tighten invitation constraints around pending-only uniqueness and
  site-director-only scope,
- persist brute-force state rather than treating it as process-local,
- include the missing change-email email/template/test/frontend work in
  the planned scope and DoD.
