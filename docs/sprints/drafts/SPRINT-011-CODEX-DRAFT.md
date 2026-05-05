# Sprint 011: User Menu in Sidebar Footer

## Overview

Sprint 009 and Sprint 010 already shipped the hard parts of account
management: the backend can terminate sessions with
`POST /api/auth/logout`, and `/admin/account` already exposes self-serve
profile, password, and email management. The missing piece is chrome.
Today the signed-in user sees their name and email in the sidebar footer,
but that block is inert. There is no visible sign-out affordance, and the
profile page is effectively hidden unless someone knows the URL or
happens to discover the temporary "Edit profile" link on the Cruise
Director landing page.

Sprint 011 should close that gap without inventing new backend surface.
The sidebar footer becomes a proper account entry point: clicking the
user’s name opens a compact menu with two actions, `Profile` and
`Sign out`. `Profile` routes to `/admin/account`. `Sign out` calls the
existing logout API, then replaces history to `/login` so re-entering
the authenticated shell requires a fresh `/api/me` success. This is a
small sprint, but it matters because it turns already-shipped auth and
profile features into a complete, discoverable user journey.

The key engineering work is interaction correctness, not data modeling:
keyboard-reachable trigger, menu dismissal on outside click or `Escape`,
reasonable focus return, disabled state while logout is in flight, and
styling that feels native to the existing amber-and-slate admin chrome.
No schema changes, no new routes, no new dependencies.

## Use Cases

1. **Open account menu from the sidebar footer**: A signed-in Org Admin or Cruise Director clicks their name in the bottom-left sidebar and sees a small menu anchored to that footer block.
2. **Navigate to profile management**: The user chooses `Profile` from the menu and lands on `/admin/account`, where Sprint 010’s existing self-service sections remain the source of truth.
3. **Sign out cleanly**: The user chooses `Sign out`, the app calls `api.logout()`, the session cookie is invalidated, and the browser lands on `/login`.
4. **Fail safely on logout errors**: If logout returns an error, the user stays in place, sees an inline error, and can retry without the menu getting stuck.
5. **Use the menu with keyboard only**: The footer trigger can be focused and activated from the keyboard; `Escape` closes the menu and returns focus to the trigger.

## Architecture

### Chosen interaction model

Use a compact popover menu in the sidebar footer, not a direct route on
click and not an embedded slide-over profile editor.

- A direct route from the footer name to `/admin/account` solves only
  half the problem; sign-out would still need a second affordance.
- A slide-over panel is too heavy for the current shell and duplicates
  the account page we already have.
- A two-item menu keeps the actions grouped under the user identity,
  matches the intent language, and requires only local React state.

### Component shape

`AdminShell` remains the chrome owner. The static footer block becomes a
small interactive account-menu component:

```text
+-----------------------------------+
| Liveaboard                        |
|                                   |
| Overview                          |
| Organization                      |
| Fleet                             |
| ...                               |
|                                   |
| -------------------------------   |
| Maya Sanchez                  v   |  <- button trigger
| maya@example.com · org admin      |
|                                   |
|   +---------------------------+   |
|   | Profile                   |   |
|   | Sign out                  |   |
|   +---------------------------+   |
+-----------------------------------+
```

### State + event flow

```text
Sidebar footer button
    |
    +-- click / Enter / Space --> open menu
    |                             set aria-expanded=true
    |                             focus first item
    |
    +-- Escape / outside click --> close menu
    |                             return focus to trigger
    |
    +-- "Profile" --------------> navigate("/admin/account")
    |
    +-- "Sign out" -------------> api.logout()
                                  disable menu items
                                  on success: navigate("/login", { replace: true })
                                  on error: show inline error, keep user in shell
```

### Routing and auth implications

- No new routes are required. `/admin/account` already exists and is the
  correct destination.
- No backend handlers change. This sprint consumes existing
  `/api/auth/logout` and `/api/me`.
- The back-button guarantee comes from the existing auth guard:
  navigating back to `/admin/*` after logout re-runs `RequireSession`,
  which calls `/api/me` and redirects to `/login` on `401`.

### Accessibility notes

- Trigger must be a real `<button>`, not a clickable `<div>`.
- Use `aria-haspopup="menu"` and `aria-expanded`.
- Support dismissal via `Escape` and outside click.
- When the menu opens, focus should move into it; when it closes, focus
  should return to the trigger.
- Keep hit targets comfortable enough for the existing density system.

## Implementation Plan

### Phase 1: Footer Menu Component in Admin Shell (~55%)

**Files:**
- `web/src/admin/Shell.tsx` - Replace the inert footer text block with an interactive account-menu trigger and popover.

**Tasks:**
- [ ] Extract the footer block into a local `UserMenu` component inside `Shell.tsx` or an adjacent small helper.
- [ ] Render the current user name, email, and role inside a trigger button that still looks like the existing footer summary.
- [ ] Add local state for `open`, `submitting`, and transient logout error.
- [ ] Implement open/close behavior for pointer and keyboard activation.
- [ ] Close the menu on route changes, outside clicks, and `Escape`.
- [ ] Wire the `Profile` item to `/admin/account`.
- [ ] Wire the `Sign out` item to `api.logout()` and redirect to `/login` with `replace: true`.
- [ ] Prevent duplicate logout submissions and keep the menu usable after a failure.

### Phase 2: Chrome Styling and Interaction Polish (~25%)

**Files:**
- `web/src/styles/app.css` - Add menu-trigger, popover, item, hover, focus, and error states for the sidebar footer.

**Tasks:**
- [ ] Add sidebar-footer menu styles using existing tokens only.
- [ ] Preserve the current footer information hierarchy while making the trigger obviously clickable.
- [ ] Style the menu as a compact, elevated panel that feels native to the existing admin chrome.
- [ ] Add visible focus treatment for both the trigger and menu actions.
- [ ] Ensure the menu does not visually collide with the sidebar border or content edge.
- [ ] Keep narrow-width behavior sane without inventing a full mobile nav system.

### Phase 3: Verification and Cleanup (~20%)

**Files:**
- `web/src/admin/Shell.tsx` - Small cleanup as needed after QA.
- `docs/sprints/SPRINT-011.md` - Final sprint document once this draft is merged and adopted.

**Tasks:**
- [ ] Manually verify both roles (`org_admin`, `cruise_director`) see the same footer menu behavior.
- [ ] Verify `Profile` reaches `/admin/account` from the click target described in the sprint intent.
- [ ] Verify `Sign out` lands on `/login` and a subsequent `/api/me`-guarded navigation returns to login rather than silently restoring shell access.
- [ ] Run `npm run build`, `tsc -b`, and `go test ./...`.
- [ ] Confirm no visual regressions in the existing sidebar nav and no DESIGN.md violations.

## API Endpoints

No new endpoints in this sprint. Existing surfaces used by the frontend:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/auth/logout` | `POST` | Invalidate current session and clear cookie. |
| `/api/me` | `GET` | Re-check authenticated state when navigating back into `/admin/*` after logout. |
| `/admin/account` | route | Existing profile/security page reached from the new menu. |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-011.md` | Create | Final sprint document synthesized from the planning drafts. |
| `web/src/admin/Shell.tsx` | Modify | Add the sidebar-footer user menu, logout flow, and keyboard/outside-click behavior. |
| `web/src/styles/app.css` | Modify | Style the new trigger and popover in the existing admin chrome language. |

## Definition of Done

- [ ] Clicking the user name in the sidebar footer reveals a clear account menu.
- [ ] The menu exposes `Profile` and `Sign out` for both Org Admin and Cruise Director.
- [ ] `Profile` routes to `/admin/account` without breaking existing route guards.
- [ ] `Sign out` calls the existing logout API, lands on `/login`, and does not silently restore `/admin` access via the back button.
- [ ] Trigger and menu are keyboard reachable; `Escape` closes the menu and returns focus sensibly.
- [ ] Logout-in-flight state prevents double submission and shows an actionable error if the request fails.
- [ ] No backend, schema, or dependency changes were required.
- [ ] `npm run build`, `tsc -b`, and `go test ./...` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Menu interaction becomes flaky due to ad hoc document listeners | Medium | Medium | Keep the implementation local and simple: one trigger ref, one menu ref, one outside-click path, one `Escape` path. |
| Logout appears successful in the UI but leaves stale authenticated state visible | Low | High | Redirect with `replace: true` after successful `api.logout()` and rely on `RequireSession` + `/api/me` when re-entering `/admin`. |
| Footer trigger loses the current visual hierarchy and looks like a random button | Medium | Medium | Style the trigger to preserve the current summary text layout and use subtle affordances rather than a heavy CTA treatment. |
| Mobile expectations creep into scope | Medium | Low | Keep the sprint desktop-first; ensure the menu does not break narrow layouts, but defer larger responsive shell changes. |
| Frontend interaction coverage remains lighter than backend coverage | High | Medium | Keep the change surface narrow, preserve existing backend logout regression coverage, and use a short explicit manual QA checklist for menu behavior. |

## Security Considerations

- The menu is a convenience surface only; the security boundary remains the backend session checks.
- Sign-out must continue to invalidate the current session server-side rather than only clearing client state.
- No user data beyond the already-visible name/email/role summary should be exposed in the menu.
- Role gating for admin-only routes remains unchanged; this sprint must not weaken existing `RequireAdmin` behavior.

## Dependencies

- Sprint 009: provides the working logout endpoint and cookie-session model.
- Sprint 010: provides `/admin/account`, `useMe.refresh()`, and the current sidebar footer identity block.
- No external services or new packages.

## Open Questions

1. Should the menu label say `Profile` or `My profile`? `Profile` is cleaner in the compact menu, but `My profile` would match the page section label.
2. Do we want a signed-out flash message on `/login`, or is silent redirect sufficient for this sprint?
3. Should the temporary `Edit profile` link on the Cruise Director landing page remain as a secondary shortcut, or should the new footer menu become the sole chrome-level entry point?

## References

- `docs/sprints/SPRINT-008.md`
- `docs/sprints/SPRINT-009.md`
- `docs/sprints/SPRINT-010.md`
- `docs/sprints/drafts/SPRINT-011-INTENT.md`
- `docs/product/personas.md`
- `docs/product/organization-admin-user-stories.md`
- `CLAUDE.md`
- `DESIGN.md`
