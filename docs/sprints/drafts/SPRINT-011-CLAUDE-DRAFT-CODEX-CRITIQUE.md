# Codex Critique of `SPRINT-011-CLAUDE-DRAFT.md`

## Overall Assessment

Claude's draft is directionally correct. It picks the right product
shape for this sprint: a compact sidebar-footer user menu with
`Profile` and `Sign out`, no backend work, and `/admin/account` as the
profile destination. The draft is also grounded in the recent sprint
history and keeps the scope appropriately chrome-focused.

The main problems are not with the core product decision. They are in
implementation semantics and repo-fit details:

1. the logout failure path is too loose and conflicts with the sprint's
   own success criteria,
2. a few file/process references are inaccurate against the current
   repo,
3. the keyboard spec is slightly over-scoped relative to the rest of
   the app and the value of this sprint.

## Findings

### 1. Logout-on-failure is underspecified and internally inconsistent

The draft says:

- the sign-out handler should `await api.logout(); navigate("/login");`
- but also that on network failure the app should navigate anyway and
  let the next auth check sort it out
- and in Security Considerations it frames that as acceptable because
  the cookie will expire naturally later

That is the weakest part of the plan.

Why it matters:

- The intent explicitly says success means sign out clears the session
  via `api.logout()`, navigates to `/login`, and does not silently
  re-authenticate.
- If the network request fails, immediately navigating to `/login`
  while leaving a valid session cookie in place means the user has not
  actually signed out.
- In the current app, a still-valid cookie would allow the user to hit
  `/admin` again and pass `RequireSession`, so "navigate regardless" is
  not a real logout guarantee.

What I would change:

- Treat logout failure as a failure, not as a success-shaped redirect.
- Keep the user in the shell, show an inline error in the menu, and let
  them retry.
- Only navigate to `/login` after the logout request succeeds.

## 2. `tracker.tsv` should not be an implementation step here

The draft lists `docs/sprints/tracker.tsv` as an update target "to
register Sprint 011."

That does not match the repo's sprint process in `docs/sprints/README.md`.
The documented flow is:

- create the sprint document
- run `go run docs/sprints/tracker.go sync`

Direct `tracker.tsv` editing is not the convention. For a planning
draft, it is also premature noise. The draft should describe the final
sprint doc and implementation files, not prescribe manual edits to the
tracker database.

What I would change:

- Remove `docs/sprints/tracker.tsv` from Files Summary.
- If tracker registration needs to be mentioned at all, refer to
  `tracker.go sync`, not direct TSV editing.

## 3. One file path is wrong against the current tree

The Architecture table references:

- `web/src/admin/Account.tsx`

The actual file in the repo is:

- `web/src/admin/pages/Account.tsx`

This is small, but sprint docs work best when they point at the exact
implementation surface. The rest of the draft is concrete enough that
this kind of mismatch stands out.

## 4. Arrow-key menu navigation is probably more complexity than this sprint needs

The draft adds full menu-style keyboard behavior:

- `Enter` / `Space` opens
- arrows move focus
- `Enter` activates
- `Escape` closes

I would narrow that.

Why:

- The intent requires accessibility, keyboard reachability, and
  escape-to-close. It does not require a fully modeled ARIA menu with
  roving focus.
- The existing app has no shared popover/menu primitive. Adding arrow
  navigation from scratch increases implementation detail for limited
  user value.
- A simpler and still acceptable model is: trigger is a button, focus
  moves to the first action on open, `Tab` moves through actions,
  `Escape` closes, focus returns to the trigger.

What I would change:

- Keep keyboard accessibility, but avoid overcommitting the sprint to
  custom arrow-key behavior unless the implementation falls out
  naturally.

## 5. Planning artifacts in Files Summary add noise

The Files Summary includes:

- `docs/sprints/drafts/SPRINT-011-*.md`

That is not useful in the actual sprint implementation plan. The sprint
doc should primarily describe product and code files that change when
the sprint is executed. Planning artifacts belong to the planning
workflow, not the implementation inventory.

What I would change:

- Keep the Files Summary focused on implementation files plus the final
  sprint doc.

## What Claude Draft Gets Right

- Correctly identifies this as a frontend-only sprint.
- Chooses the popover menu pattern, which is the cleanest answer to the
  intent's "click the name for profile + sign out" requirement.
- Preserves `/admin/account` as the canonical profile surface instead of
  duplicating profile UI elsewhere.
- Keeps the existing Cruise Director landing-page profile link as a
  harmless secondary shortcut.
- Grounds the work in the right files: `Shell.tsx`, `app.css`,
  existing routing, and `api.logout()`.

## Recommended Adjustments Before Merge

1. Change logout semantics so navigation happens only after successful
   logout, with inline retryable error on failure.
2. Remove `tracker.tsv` from the plan and follow the documented
   `tracker.go sync` convention later when the final sprint doc exists.
3. Fix `web/src/admin/Account.tsx` to
   `web/src/admin/pages/Account.tsx`.
4. Reduce keyboard requirements from full arrow-key menu behavior to a
   simpler button + focus-management pattern unless stricter behavior is
   explicitly desired.
5. Trim planning-artifact files out of the Files Summary.
