# Sprint 011: User Menu + Sea Palette — Sign Out, Profile Entry, Brand Refresh

## Overview

Two related chrome changes in one sprint. First, close the session
loop in the running app: the auth stack has been complete since
Sprint 010 (login, signup, invitations, profile editor,
change-password, change-email), but the chrome has no visible way to
sign out and no affordance pointing at the profile editor — both
rely on the user typing URLs or reaching for devtools.

Second, give the SPA a brand identity. Today the entire app sits on a
flat warm-slate background. For a scuba liveaboard product, leaning
into a turquoise "sea" gradient is on-brand; the existing slate
palette stays in play as the working surface for nav, cards, and
tables so density doesn't suffer. This is an explicit reversal of the
2026-04-29 "no blue in palette" decision in DESIGN.md, captured as a
new entry in the Decisions Log.

Sprint 011 turns the static sidebar footer ("Maya Sanchez —
maya@example.com — cruise director") into an interactive user menu.
Clicking the name opens a small popover anchored to the footer with
two items — `Profile` and `Sign out`. A second always-visible
`Sign out` button sits below the trigger so the most common
session-ending action is reachable in a single click. `Profile` routes
to `/admin/account`; `Sign out` invokes the existing `api.logout()`
and, **only on success**, navigates to `/login` with `replace: true`.
Failure keeps the user in the shell with an inline retryable error.

This is a frontend-only sprint. No backend, no schema, no new
dependencies. The risk surface is small: the React component for the
popover, plus a token + body-background change.

## Use Cases

1. **Discover sign out.** A signed-in user wants to log out. They
   either click the always-visible `Sign out` button in the footer,
   or click their name → menu → `Sign out`. Either path returns them
   to `/login` with no live session.
2. **Edit own profile.** A signed-in user clicks their name in the
   footer, picks `Profile` from the menu, edits + saves on
   `/admin/account`, and sees the change reflect in the footer
   immediately (already wired via `useMe.refresh()` from Sprint 010).
3. **Cruise Director self-service.** Same flow as above for a Cruise
   Director — the menu and target route are role-agnostic.
4. **Keyboard usage.** `Tab` reaches the trigger; `Enter`/`Space`
   opens the menu; `Escape` closes it and returns focus to the
   trigger. `Tab` traverses items inside the open menu.
5. **Click-outside dismiss.** Opening the menu and then clicking
   anywhere else in the app closes it without taking action.
6. **Logout failure stays graceful.** If the network call fails, the
   user is *not* navigated away. The menu stays open, both `Sign
   out` buttons re-enable, and an inline error appears below the
   menu items.

## Architecture

### What changes

| Area | Change |
|---|---|
| `web/src/admin/UserMenu.tsx` | **NEW.** Encapsulates the trigger, popover, click-outside, `Escape` handling, logout state machine, and inline error. |
| `web/src/admin/Shell.tsx` | Replace the static `<div className="admin-sidebar__footer">…</div>` with `<UserMenu />` + an always-visible `Sign out` button. |
| `web/src/styles/app.css` | Append rules for `.admin-sidebar__trigger`, `.user-menu`, `.user-menu__item`, `.admin-sidebar__signout`, and the inline error treatment. |
| `web/src/admin/pages/Account.tsx` | No changes. Still the canonical destination for `Profile`. |
| Existing `[ Edit profile ]` link on the Cruise Director landing | Keep. The menu is an additional entry point, not a replacement. |
| `web/src/lib/api.ts` | No changes. `api.logout()` already exists. |
| Routing | No changes. `/admin/account` and `/login` already mounted. |

### Sidebar footer layout

Today (Sprint 010 chrome):

```
+----------------------+
| Maya Sanchez         |
| maya@x.test · cruise |
+----------------------+
```

After Sprint 011, the entire identity block is a button trigger and
a second always-visible `Sign out` button sits below it:

```
+----------------------+
| [Maya Sanchez      v]|  ← trigger (button); opens popover above
| maya@x.test · cruise |
| [ Sign out ]         |  ← always-visible footer button
+----------------------+
```

Clicking the trigger reveals an anchored popover above the footer:

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

The chevron (`▾`) signals "click to expand." The popover renders
above the trigger so it stays inside the viewport at the bottom edge
of the sidebar. Both `Sign out` paths (popover item and standalone
button) trigger the same handler.

### Popover semantics

- The popover is local React state inside `UserMenu.tsx`:
  `const [open, setOpen] = useState(false)` plus
  `const [submitting, setSubmitting] = useState(false)` plus
  `const [error, setError] = useState<string | null>(null)`.
- `setOpen(false)` runs on:
  - Click of `Profile` (after `navigate`).
  - Successful `Sign out` (the navigate replaces the route entirely).
  - Outside click (`mousedown` listener on `document` while open).
  - `Escape` keypress while open.
  - Route change (a `useEffect` keyed on `location.pathname`).
- The popover uses `position: absolute; bottom: 100%; left: 0`
  relative to the trigger. No portal — the sidebar's stacking
  context is enough at this scale.
- ARIA: trigger gets `aria-haspopup="menu"` and `aria-expanded={open}`;
  popover gets `role="menu"`; items get `role="menuitem"`.
- Keyboard: `Enter`/`Space` on the trigger toggles `open`. `Escape`
  while open closes and returns focus to the trigger. Items are
  natural `<button>` elements; `Tab` traverses them. No custom
  roving-focus.

### Sign-out state machine

```
   idle ──── click ────►  submitting
    ▲                     │
    │  error              │  api.logout()
    │                     ▼
    └────────────  ┌──── success ────► navigate("/login", { replace: true })
                   │
                   └──── failure ────► back to idle, error="…", menu stays open
```

While `submitting === true`:

- Both `Sign out` paths (popover item + standalone footer button)
  are disabled.
- The popover stays open so the user can see the spinner / pending
  state.
- The `Profile` item is also disabled — switching routes mid-logout
  would be confusing.

On success: `navigate("/login", { replace: true })`. The `replace`
ensures the back button doesn't restore the authenticated chrome
from history. The next request in the SPA hits `/api/me` and
follows the existing 401 → `/login` redirect path.

On failure: `setError(message)`, `setSubmitting(false)`. The user
can click `Sign out` again. Failure is rare (cookie revocation is
idempotent server-side and never returns an error in normal
operation) but we treat it as a real failure rather than as a
success-shaped redirect — leaving the user with a still-valid
cookie while they think they've signed out is worse than asking
them to retry.

### What we explicitly don't do

- No new sidebar nav item for "Account" / "Profile". The user menu
  is the entry point.
- No avatar circle. Text-only matches DESIGN.md and the rest of the
  chrome.
- No "switch organization" — there's one org per user.
- No animation beyond a subtle CSS opacity / transform on the
  popover. Token-driven colors only.
- No mobile redesign. Sprint 007 declared desktop-first; the menu
  must not break narrow layouts but a hamburger reveal is out of
  scope.
- No splitting `/admin/account` into `Profile` vs `Security` tabs.
  The single-page layout from Sprint 010 stays.

## Implementation Plan

### Phase 1: `UserMenu` component + Shell wiring (~55%)

**Files:**
- `web/src/admin/UserMenu.tsx` — Create. Owns trigger button,
  popover, click-outside listener, `Escape` handler, logout state,
  inline error.
- `web/src/admin/Shell.tsx` — Modify. Replace static footer with
  `<UserMenu />` followed by an always-visible `Sign out` button
  that calls the same handler.

**Tasks:**
- [ ] Create `UserMenu.tsx`. Props: `me: Me`. Internal state: `open`,
      `submitting`, `error`.
- [ ] Trigger: `<button>` showing name + email + role + chevron.
- [ ] Popover: render only when `open === true`; contains `Profile`
      and `Sign out` items.
- [ ] Wire `Profile` item → `useNavigate()` to `/admin/account`,
      then close the menu.
- [ ] Wire `Sign out` item → handler that runs `setSubmitting(true)`,
      calls `api.logout()`, on success calls
      `navigate("/login", { replace: true })`, on failure calls
      `setSubmitting(false)` and `setError(...)`.
- [ ] Close on click-outside (mousedown listener attached only while
      `open`), `Escape`, and route change (`useLocation` effect).
- [ ] ARIA: `aria-haspopup`, `aria-expanded` on trigger; `role="menu"`
      on popover; `role="menuitem"` on items.
- [ ] Keyboard: native `<button>` semantics for trigger and items;
      `Escape` closes; focus returns to the trigger after close.
- [ ] Export the logout handler from `UserMenu` so the standalone
      footer button can reuse it (or have `Shell.tsx` own the handler
      and pass it down).
- [ ] Modify `Shell.tsx` to render `<UserMenu />` plus the
      always-visible `Sign out` button. Both routes share the same
      logout handler so `submitting` disables both consistently.

### Phase 2: Styling (~25%)

**Files:**
- `web/src/styles/app.css` — Append rules.

**Tasks:**
- [ ] `.admin-sidebar__trigger` — full-width button styled to match
      the existing footer block. Subtle hover (`--c-100`
      background). Trailing chevron (unicode `▾`, no icon library).
- [ ] `.user-menu` — absolute popover. White surface, `--c-200`
      border, `--r-md` radius, `--shadow-sm` (or equivalent), padding
      `--sp-xs`. Positioned `bottom: 100%; left: 0`.
- [ ] `.user-menu__item` — comfortable hit area (~40px tall),
      hover/focus uses `--c-primary-subtle`, keyboard focus ring uses
      the existing focus token.
- [ ] `.user-menu__error` — small inline error styled like the
      existing `.error` class but compact, sitting below the items.
- [ ] `.admin-sidebar__signout` — text-only button with a subtle
      `--c-500` color so it doesn't visually compete with the
      primary nav. No destructive red — sign-out is reversible by
      signing back in.
- [ ] Disabled state for trigger + items + standalone button while
      `submitting === true`: reduced opacity and `cursor: not-allowed`.

### Phase 3: Sea palette + DESIGN.md (~20%)

**Files:**
- `web/src/styles/tokens.css` — Append `--c-sea-50` through
  `--c-sea-700` plus `--gradient-sea`.
- `web/src/styles/app.css` — Apply `--gradient-sea` to `body`. Update
  `.auth-shell` to be transparent (so the body gradient shows
  through). Keep `.admin-sidebar`, `.admin-card`, `.admin-table`
  white. Tighten the auth wordmark centering.
- `DESIGN.md` — New "Sea palette" section under Color, plus a
  Decisions Log entry reversing the no-blue rule.

**Tasks:**
- [ ] Add the eight sea tokens (50, 100, 200, 300, 400, 500, 600,
      700).
- [ ] Add `--gradient-sea` as a `linear-gradient(180deg, ...)` from a
      light foam at the top to a deeper teal at the bottom.
- [ ] Apply the gradient to `body { background: var(--gradient-sea); }`.
- [ ] `.auth-shell` background becomes transparent. Wordmark
      centered for visual balance.
- [ ] Verify the admin sidebar (white) and admin-main content
      (transparent over the gradient) don't lose contrast.
- [ ] Update DESIGN.md: add the sea palette block under Color;
      explain that slate stays as the working-surface palette while
      sea is the page-level mood; new Decisions Log entry dated
      today.

### Phase 4: Smoke + commit (~20%)

**Tasks:**
- [ ] `npm --prefix web run build` clean.
- [ ] Visual smoke: trigger looks right at both roles; menu opens
      above and stays inside the viewport.
- [ ] Functional smoke: sign-out hits 200; the SPA reaches `/login`;
      back button does **not** restore `/admin/*` (replace works);
      `/api/me` from devtools returns 401 after navigation.
- [ ] Failure smoke: temporarily point `/api/auth/logout` at a
      no-op 500 (or unplug Postgres) to confirm the inline error
      surfaces and the user stays in `/admin`.
- [ ] Profile path: click `Profile` → land on `/admin/account` →
      verify the page is the existing Sprint 010 surface.
- [ ] Keyboard smoke: `Tab` reaches the trigger; `Enter` opens;
      `Escape` closes; focus returns to the trigger.
- [ ] Sprint ledger: `go run docs/sprints/tracker.go sync` then
      `go run docs/sprints/tracker.go complete 011`.
- [ ] Commit + push to `main`.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-011.md` | Create | This sprint doc. |
| `web/src/admin/UserMenu.tsx` | Create | Trigger + popover + logout state machine + inline error. |
| `web/src/admin/Shell.tsx` | Modify | Replace static footer with `<UserMenu />` + always-visible `Sign out` button. |
| `web/src/styles/tokens.css` | Modify | Add the sea palette + gradient token. |
| `web/src/styles/app.css` | Modify | Trigger, popover, items, standalone `Sign out`, error styling, body gradient. |
| `DESIGN.md` | Modify | Add Sea palette section + Decisions Log entry reversing the no-blue rule. |

## Definition of Done

- [ ] Sidebar footer renders the user identity block as a clickable
      trigger; clicking opens a popover with `Profile` and `Sign
      out`.
- [ ] An additional always-visible `Sign out` button sits below the
      trigger; both `Sign out` paths share the same handler.
- [ ] `Profile` navigates to `/admin/account`.
- [ ] **On successful `Sign out`** (HTTP 200): the SPA navigates to
      `/login` with `replace: true`. After navigation, the back
      button does not restore `/admin/*`, and any direct
      `/admin/*` URL bounces back to `/login` (cookie cleared,
      `/api/me` returns 401).
- [ ] **On failed `Sign out`** (network error or non-2xx): the user
      stays in the shell. The menu shows an inline error and both
      `Sign out` paths re-enable so the user can retry.
- [ ] While `submitting === true`: both `Sign out` paths and
      `Profile` are disabled to prevent double-submission.
- [ ] Popover closes on outside click, `Escape`, and route change.
- [ ] Trigger and items use real `<button>` elements; `Tab` reaches
      them; `Enter`/`Space` activate; `Escape` closes the menu and
      returns focus to the trigger.
- [ ] `aria-haspopup="menu"`, `aria-expanded`, `role="menu"`, and
      `role="menuitem"` are set.
- [ ] Both `org_admin` and `cruise_director` see the menu and the
      standalone button.
- [ ] DESIGN.md tokens only — no new colors, no icon library.
- [ ] `npm --prefix web run build` clean; `tsc -b` clean.
- [ ] Backend untouched: `go test ./...`, `go vet ./...`, `gofmt
      -l .` all clean.
- [ ] Sprint ledger updated via `go run docs/sprints/tracker.go
      complete 011`.
- [ ] Manual smoke through the running app: invite → accept →
      sign out from each path (button, popover) → re-login.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Popover positioning escapes the viewport at the bottom of the sidebar | Medium | Low | Anchor with `bottom: 100%`; verify at 720p height during smoke. |
| Click-outside listener leaks if not detached | Low | Low | `useEffect` with cleanup; only attach when `open === true`. |
| Sign-out network failure leaves the user "stuck" but believing they signed out | Medium | High | Only navigate on 2xx. Failure surfaces an inline error and re-enables the button so the user can retry. |
| Back button after sign-out restores authenticated chrome from history | Medium | Medium | `navigate("/login", { replace: true })` so the post-logout entry replaces the previous one. The next nav also runs `RequireSession`. |
| Double-submission of logout (rapid clicks) | Medium | Low | `submitting` state disables both `Sign out` paths and the `Profile` item until the request resolves. |
| Two `Sign out` affordances confuse users | Low | Low | Both call the same handler and share the same disabled state; the popover item adds discoverability while the standalone button is the one-click path. |
| Mobile / narrow viewport positioning | Low | Low | Sprint 007 declared desktop-first; revisit if it becomes a real issue. |

## Security Considerations

- **Sign-out is server-authoritative.** The session cookie is
  invalidated by the server on success; the SPA navigates only after
  a 2xx. On failure the user keeps a still-valid cookie *and* knows
  the action failed.
- **`replace: true`** prevents the back button from restoring an
  authenticated history entry. It does not prevent a user from
  manually retyping `/admin/*` and (in the failure case) reaching
  the chrome via the still-valid cookie — that's the right behavior:
  if logout truly failed, the session genuinely is still valid.
- **No persistent client state of secrets.** The popover only
  renders data already in `useMe()`.
- **No new endpoints, no schema, no new RBAC paths.**

## Dependencies

- **Sprint 008** (admin shell + `useMe`) — built upon.
- **Sprint 009** (`api.logout` endpoint) — built upon.
- **Sprint 010** (`/admin/account` profile editor) — the target route.
- No new Go module deps; no new npm deps.

## Out of Scope (captured as follow-ups)

- Avatar / profile photo upload.
- "Sign out everywhere" / session list with revoke.
- Theme switcher in the user menu.
- Help / Docs / Feedback links in the user menu.
- Account deletion from inside the profile page.
- Org-switch UI.
- Mobile chrome redesign / hamburger nav.
- Splitting `/admin/account` into `Profile` vs `Security` tabs.

## References

- Sprint 008 — `docs/sprints/SPRINT-008.md` (admin shell + sidebar IA).
- Sprint 009 — `docs/sprints/SPRINT-009.md` (auth + logout endpoint).
- Sprint 010 — `docs/sprints/SPRINT-010.md` (profile editor + `useMe.refresh`).
- Personas — `docs/product/personas.md`.
- Design — `DESIGN.md`.
- Codex critique — `docs/sprints/drafts/SPRINT-011-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-011-MERGE-NOTES.md`.
