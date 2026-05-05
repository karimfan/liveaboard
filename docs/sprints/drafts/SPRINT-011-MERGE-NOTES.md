# Sprint 011 Merge Notes

## Claude Draft Strengths

- Tight scope framing (frontend-only, no backend, no schema).
- Clear footer-trigger mockup that matches the user's interview
  preview verbatim.
- Captured the secondary-entry-point detail (the Sprint 010 "Edit
  profile" link on the Cruise Director landing remains harmless and
  stays).
- Full a11y scope including ARIA roles + click-outside.
- Sequencing matches recent sprints (component → styling → smoke).

## Codex Draft Strengths

- Sharper logout-failure semantics: only navigate on success, show
  inline error and allow retry on failure. Correctly diagnosed
  Claude's "navigate regardless" as inconsistent with the stated
  success criteria.
- Correctly flagged the file-path drift: `web/src/admin/Account.tsx`
  → `web/src/admin/pages/Account.tsx`.
- Correctly flagged tracker drift: `tracker.tsv` direct edits are not
  the documented convention; `tracker.go sync` is.
- Suggested `navigate("/login", { replace: true })` so the back
  button doesn't restore the authenticated chrome. Subtle but right.
- Argued for narrower keyboard scope (button + `Enter`/`Space` +
  `Escape` + focus return), avoiding a custom roving-focus
  implementation that doesn't pay back at this scale.
- Removed planning artifacts from the Files Summary.
- Added a `submitting` state to prevent double-submission of logout.

## Valid Critiques Accepted

1. **Logout failure must keep the user in the shell.** Final design:
   on success, `navigate("/login", { replace: true })`. On failure,
   render an inline error inside the menu and keep the menu open with
   the items re-enabled.
2. **`replace: true`** on the post-logout navigate so the browser back
   button doesn't restore `/admin/*` history.
3. **`submitting` state on Sign out** to prevent double-clicks during
   the network round-trip; both menu items are disabled while the
   request is in flight.
4. **File paths corrected** to `web/src/admin/pages/Account.tsx`.
5. **Tracker registration** uses `go run docs/sprints/tracker.go
   sync`, not direct `tracker.tsv` edits. Removed from the
   implementation Files Summary.
6. **Keyboard scope narrowed.** Interview chose "Standard" — that's
   button + `Enter`/`Space` to open, `Escape` to close, focus return
   to trigger. No arrow-key roving focus.
7. **Files Summary trimmed** to only the final sprint doc + the
   actual implementation files.

## Critiques Rejected (with reasoning)

- None. All five Codex points are valid.

## Interview Refinements Applied

1. **Popover from the name** with `Profile` and `Sign out`. (Interview
   answer #1.)
2. **Always-visible separate `Sign out` button** in the sidebar
   footer, in addition to the popover item. (Interview answer #2.)
   Two paths to sign out is intentional redundancy: the most-used
   action is one click away, and discoverability stays high.
3. **`/admin/account` stays a single page.** No tab split. (Interview
   answer #3.)
4. **Standard a11y, not full ARIA menu.** (Interview answer #4.)

## Final Decisions

### Footer layout

```
+----------------------+
| [Maya Sanchez      v]| ← clickable trigger; opens popover above
| maya@x.test · cruise |
| [ Sign out ]         | ← always-visible button; one-click logout
+----------------------+
```

When the popover is open it sits *above* the trigger:

```
+----------------------+
|   Profile            |
|   Sign out           |
+----------------------+
| [Maya Sanchez      v]|
| maya@x.test · cruise |
| [ Sign out ]         |
+----------------------+
```

### Logout state machine

```
idle  --click Sign out-->  submitting
                             |
                             +-- 200 -->  navigate("/login", { replace: true })
                             |
                             +-- error -> idle + inline error in menu
```

The `submitting` state disables every menu item and the standalone
`Sign out` button. On error, items re-enable; the menu stays open.

### Profile route

`Profile` (popover) → `useNavigate()` → `/admin/account`. No new
routes; no page split.

### Keyboard scope

- Trigger is a real `<button>`. `Tab` reaches it, `Enter`/`Space`
  opens.
- Open menu can be dismissed with `Escape`. On close, focus returns
  to the trigger.
- `Tab` traverses items inside the menu naturally — no custom
  roving-focus implementation.

### ARIA

- Trigger: `aria-haspopup="menu"`, `aria-expanded={open}`.
- Popover: `role="menu"`.
- Items: `role="menuitem"`.

### Files Summary (final)

| File | Action | Purpose |
|---|---|---|
| `docs/sprints/SPRINT-011.md` | Create | Final sprint doc. |
| `web/src/admin/UserMenu.tsx` | Create | Trigger + popover component encapsulating menu state, logout, click-outside. |
| `web/src/admin/Shell.tsx` | Modify | Replace static footer with `<UserMenu>` + always-visible `Sign out` button. |
| `web/src/styles/app.css` | Modify | New rules for trigger, popover, items, sign-out button. |

### Out of Files Summary

- Planning artifacts in `docs/sprints/drafts/` (per Codex).
- `docs/sprints/tracker.tsv` (handled by `tracker.go sync`).

## Phase Sequencing (final)

- Phase 1: `UserMenu` component + Shell wiring (~55%).
- Phase 2: Styling + always-visible sign-out button (~25%).
- Phase 3: Smoke + commit (~20%).
