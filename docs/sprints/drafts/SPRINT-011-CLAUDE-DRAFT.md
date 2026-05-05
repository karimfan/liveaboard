# Sprint 011: User Menu — Sign Out + Profile Entry From Sidebar

## Overview

A small chrome-only sprint that closes the session loop in the running
app. After Sprint 010 the auth stack is feature-complete on both ends
(login, signup, invitations, profile editor, change-password,
change-email), but the chrome has no visible way to sign out and no
affordance pointing at the profile editor — both rely on the user
typing URLs or reaching for devtools.

This sprint converts the static sidebar footer ("Maya Sanchez —
maya@example.com — cruise director") into an interactive user menu.
Clicking the name opens a small popover anchored to the footer with
two items — `Profile` and `Sign out`. Profile routes to
`/admin/account`; Sign out invokes the existing `api.logout()` and
navigates to `/login`. The login page itself needs no change (sign-in
already lives there); the seed's "sign in / sign out buttons" wording
was about getting the sign-out action into the authenticated chrome.

This is a frontend-only sprint. No backend, no schema, no new
dependencies, no new tests at the Go layer. The risk surface is small
and concentrated in the React component for the popover.

## Use Cases

1. **Discover sign out.** A signed-in user wants to log out. They
   click their name in the sidebar footer, the user menu opens, they
   click `Sign out`, and they're returned to `/login` with no live
   session.
2. **Edit own profile.** A signed-in user wants to update their phone
   number. They click their name in the footer, click `Profile` from
   the menu, edit + save on `/admin/account`, see the change reflect
   in the footer immediately (already wired via `useMe.refresh()`
   from Sprint 010).
3. **Cruise Director self-service.** Same flow as above for a Cruise
   Director — the menu and target route are role-agnostic.
4. **Keyboard usage.** A keyboard user tabs to the footer trigger,
   presses `Enter` to open the menu, arrows down through items,
   presses `Enter` to select. `Escape` closes the menu.
5. **Click-outside dismiss.** Opening the menu and then clicking
   anywhere else in the app closes it without taking action.

## Architecture

### What changes

| Area | Change |
|---|---|
| `web/src/admin/Shell.tsx` | Sidebar footer becomes an interactive trigger that toggles a popover; popover lists `Profile` + `Sign out`. |
| `web/src/styles/app.css` | New rules: `.admin-sidebar__trigger`, `.user-menu`, `.user-menu__item`. |
| `web/src/admin/Account.tsx` | No changes — already the right target for `Profile`. |
| Existing `[ Edit profile ]` link on the Cruise Director landing | Keep — it remains a direct link from the contact card; the menu is an additional entry point, not a replacement. |
| `web/src/lib/api.ts` | No changes — `api.logout()` already exists. |
| Routing | No changes — `/admin/account` and `/login` already mounted. |

### Sidebar footer layout

Today:

```
+----------------------+
| Maya Sanchez         |
| maya@x.test · cruise |
+----------------------+
```

After Sprint 011, the entire footer block becomes a button:

```
+----------------------+
| [Maya Sanchez      v]|
| maya@x.test · cruise |
+----------------------+
```

Clicking it opens an anchored popover above the footer:

```
+----------------------+
|   Profile            |
|   Sign out           |
+----------------------+
| [Maya Sanchez      v]|
| maya@x.test · cruise |
+----------------------+
```

The chevron (`▾`) indicates "click to expand". The popover renders
above the trigger so it stays inside the viewport even at the bottom
edge of the sidebar.

### Popover semantics

- The popover is a small uncontrolled component. State lives in
  `Shell.tsx` (`const [open, setOpen] = useState(false)`).
- `setOpen(false)` runs on:
  - Click of any menu item (after the action fires).
  - Click outside the trigger / popover (use a `mousedown` listener
    on `document` while open).
  - `Escape` keypress while open.
  - Route change (the menu should not survive navigation).
- The popover uses `position: absolute` with `bottom: 100%` and
  `left: 0` relative to the footer. No portal — staying inside the
  sidebar's stacking context is fine for this scale.
- ARIA: the trigger gets `aria-haspopup="menu"`, `aria-expanded={open}`;
  the popover gets `role="menu"`; items get `role="menuitem"`.

### Sign-out flow

```
[Sign out] → api.logout() → navigate("/login")
```

The handler is `async () => { await api.logout(); navigate("/login"); }`.
We don't depend on the response — even if the network call fails the
UI takes the user back to `/login`, where `RequireSession` would have
sent them anyway after the cookie expires. We do `await` so the
`Set-Cookie: lb_session=; Max-Age=-1` round-trip lands before
navigation, which avoids a flash where the SPA briefly thinks it's
still authed.

### What we explicitly don't do

- No new sidebar nav item for "Account" / "Profile". The menu is the
  entry point.
- No avatar circle. Text-only matches DESIGN.md and the rest of the
  chrome.
- No "switch organization" — there's one org per user.
- No animation beyond a subtle CSS transition on the popover's
  visibility. Token-driven colors only.

## Implementation Plan

### Phase 1: User menu component (~60%)

**Files:**
- `web/src/admin/UserMenu.tsx` — Create. Encapsulates the trigger +
  popover. Takes `me` as a prop.
- `web/src/admin/Shell.tsx` — Modify. Replace the static footer with
  `<UserMenu me={me.me} />`.
- `web/src/styles/app.css` — Append `.admin-sidebar__trigger`,
  `.user-menu`, `.user-menu__item` rules.

**Tasks:**
- [ ] Create `UserMenu.tsx` with a button trigger + popover.
- [ ] Wire `Profile` item → `useNavigate` to `/admin/account`.
- [ ] Wire `Sign out` item → `api.logout()` then `navigate("/login")`.
- [ ] Close on click-outside, `Escape`, and route change.
- [ ] Keyboard: `Enter`/`Space` opens, arrows move focus, `Enter`
      activates, `Escape` closes.
- [ ] ARIA roles + `aria-expanded` on the trigger.
- [ ] Wire into `Shell.tsx` so both roles see the menu.

### Phase 2: Styling (~25%)

**Files:**
- `web/src/styles/app.css` — Append rules.

**Tasks:**
- [ ] `.admin-sidebar__trigger` — full-width button styled to match
      the existing footer block; subtle hover state; chevron glyph
      via inline SVG or unicode (text-only).
- [ ] `.user-menu` — absolute-positioned card with token border,
      shadow, padding; appears above the trigger.
- [ ] `.user-menu__item` — comfortable hit area (~40px tall),
      hover/focus states using `--c-primary-subtle`.
- [ ] Sign out item gets a slightly muted treatment (no destructive
      red — the action is reversible by signing back in).

### Phase 3: Smoke + commit (~15%)

**Tasks:**
- [ ] `npm --prefix web run build` clean.
- [ ] Visual smoke: sidebar trigger looks right at both roles; menu
      opens above and stays inside the viewport.
- [ ] Functional smoke: sign-out hits 200 then `/api/me` returns 401
      after navigation; profile action lands on `/admin/account`.
- [ ] Commit + push to `main`.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-011.md` | Create | Final sprint doc. |
| `docs/sprints/drafts/SPRINT-011-*.md` | Create | Planning artifacts. |
| `web/src/admin/UserMenu.tsx` | Create | Trigger + popover component. |
| `web/src/admin/Shell.tsx` | Modify | Replace static footer with `<UserMenu>`. |
| `web/src/styles/app.css` | Modify | Trigger / popover rules. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 011. |

## Definition of Done

- [ ] Sidebar footer renders as a clickable trigger; clicking opens a
      popover with `Profile` and `Sign out`.
- [ ] `Profile` navigates to `/admin/account`.
- [ ] `Sign out` calls `api.logout()` and navigates to `/login`. After
      navigation, hitting any authenticated route bounces back to
      `/login` (cookie cleared).
- [ ] Popover closes on outside click, `Escape`, route change.
- [ ] `Enter`/`Space` opens the menu; arrows move focus; `Enter`
      activates; `Escape` closes.
- [ ] `aria-haspopup`, `aria-expanded`, and `role="menu"`/`menuitem`
      are set.
- [ ] Both `org_admin` and `cruise_director` see the menu.
- [ ] `npm --prefix web run build` and `tsc -b` clean.
- [ ] No backend changes; existing Go tests still green
      (`go test ./...`).
- [ ] Manual smoke through the running app: invite → accept → sign
      out from the menu → re-login.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Popover positioning escapes the sidebar (overflows viewport at the bottom) | Medium | Low | Anchor with `bottom: 100%`; pin `left: 0` to the trigger; verify at 720p height during smoke. |
| Click-outside listener leaks if not detached | Low | Low | Use `useEffect` with cleanup; only attach when `open === true`. |
| Sign-out network failure leaves the user "stuck" on `/admin/*` | Low | Low | Best-effort `await api.logout()`, then navigate regardless. The next request will 401 and the SPA will redirect to `/login` automatically via `RequireSession`. |
| Keyboard shortcut conflicts | Low | Low | The keys we use (`Enter`, `Space`, arrows, `Escape`) are scoped to the menu when open. |
| Mobile / narrow viewport positioning | Low | Low | Sprint 007 declared desktop-first; revisit if it becomes a real issue. |

## Security Considerations

- **Logout is best-effort but always navigates.** Even if the network
  call fails, the SPA leaves the authenticated chrome. The cookie is
  cleared by the server when the call succeeds; if it doesn't, the
  cookie expires naturally (TTL = `LIVEABOARD_SESSION_DURATION`).
- **No new endpoints.** No surface area added to the backend.
- **No persistent client state of secrets.** The popover only renders
  data already in `useMe()`.

## Dependencies

- **Sprint 008** (admin shell + `useMe`) — built upon.
- **Sprint 009** (`api.logout` endpoint) — built upon.
- **Sprint 010** (`/admin/account` profile editor) — the target route.
- No new Go module deps; no new npm deps.

## Out of Scope (captured as follow-ups)

- Avatar / profile photo upload.
- "Sign out everywhere" / session list with revoke.
- Theme switcher.
- Help / Docs / Feedback links.
- Account deletion from inside the profile page.
- Org-switch UI.

## Open Questions

1. **Menu vs direct route.** I default to a small popover with two
   items because the seed implies a discoverable menu, and the
   sign-out button needs *somewhere* to live; routing the click
   directly to `/admin/account` would leave sign-out homeless. The
   interview will validate this.
2. **Sign-out as a separate footer button.** Plausible alternative is
   to keep the popover for `Profile` only and put a dedicated `Sign
   out` button next to the name. Probably less clean than the menu;
   confirm with the interview.
3. **Splitting the Account page.** Could split into Profile vs
   Security tabs; defer unless the user asks.

## References

- Sprint 008 — `docs/sprints/SPRINT-008.md` (admin shell + sidebar IA).
- Sprint 009 — `docs/sprints/SPRINT-009.md` (auth + logout endpoint).
- Sprint 010 — `docs/sprints/SPRINT-010.md` (profile editor + useMe.refresh).
- Personas — `docs/product/personas.md`.
- Design — `DESIGN.md`.
- Codex critique — `docs/sprints/drafts/SPRINT-011-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-011-MERGE-NOTES.md`.
