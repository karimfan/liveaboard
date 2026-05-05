# Sprint 011 Intent: User Menu — Sign Out + Profile Entry From Sidebar

## Seed

> let's add signout/sign in buttons and also add a user profile
> management section. The user profile management appears when the
> user clicks their name on the bottom left nav

## Context

Sprint 009 shipped the self-hosted auth stack including
`POST /api/auth/logout`. Sprint 010 shipped a working profile editor
at `/admin/account` ("My profile" section above change-password and
change-email). Both pieces work end-to-end, but neither has a
discoverable chrome entry point:

- The sidebar footer in `web/src/admin/Shell.tsx` shows the user's
  name + email + role as plain text — there is no click affordance and
  no `Sign out` button anywhere in the running app. Today the only way
  to log out is by clearing the cookie manually or reaching for
  devtools.
- `/admin/account` is reachable by typing the URL but has no visible
  link in the chrome.
- The login page is fine as-is; "sign in" already exists at `/login`.
  The seed's wording was about surfacing the *sign out* affordance
  inside the authenticated app.

Sprint 011 closes the loop: clicking the user's name in the bottom of
the sidebar reveals a small menu (or routes directly) into profile
management, and a `Sign out` action invokes `api.logout()` and
returns to `/login`.

## Recent Sprint Context

- **Sprint 009** — Custom auth + Brevo SMTP. Logout endpoint shipped,
  no client surface for it.
- **Sprint 010** — Cruise Director rename + rich invitations + the
  Cruise Director landing page. Added the `My profile` section in
  `/admin/account` and the `useMe.refresh()` hook so chrome reflects
  profile saves live. Added a temporary `[ Edit profile ]` link from
  the Cruise Director landing card to `/admin/account` — that's the
  only entry point today.
- **Sprint 008** — Admin chrome IA: sidebar shell, role-gated nav,
  `useMe()` provider; the static footer block this sprint replaces.

## Relevant Codebase Areas

| Area | Files |
|---|---|
| Sidebar chrome | `web/src/admin/Shell.tsx` |
| Sidebar styling | `web/src/styles/app.css` (`.admin-sidebar*` rules) |
| Profile / security page | `web/src/admin/pages/Account.tsx` |
| Auth API client | `web/src/lib/api.ts` (`api.logout`) |
| Me context | `web/src/admin/useMe.tsx` |
| Login page | `web/src/pages/Login.tsx` (no change needed) |
| Routing | `web/src/main.tsx` (already has `/admin/account`) |

## Constraints

- Follow CLAUDE.md (Go stdlib + minimal deps; gofmt + vet + tests
  green per commit; work directly on `main`; focused commits).
- Match `DESIGN.md` tokens: warm slate background, amber CTA, General
  Sans display + DM Sans body, comfortable density, text-only nav
  labels (no decorative glyphs).
- No backend changes — `api.logout()` is already wired. No schema
  changes. No new dependencies.
- Preserve role-aware nav: the user menu must work for both
  `org_admin` and `cruise_director`.
- Keep accessibility decent: keyboard-reachable trigger, escape-to-
  close on any popover, focus management when the menu opens/closes.

## Success Criteria

- A signed-in user can click their name in the sidebar footer and see
  a clear path to (a) edit their profile and (b) sign out.
- Sign out clears the session cookie via `api.logout()`, navigates to
  `/login`, and the back button does not silently re-authenticate
  (subsequent `/api/me` returns 401).
- The profile management surface opens reliably from the click target
  the seed described (the user's name in the bottom-left nav).
- No regressions in the chrome on either role; `npm run build`,
  `tsc -b`, and `go test ./...` all stay green.
- Visual + interaction language matches DESIGN.md — token-driven
  colors, comfortable hit areas, no new icon set.

## Open Questions

1. **Click target affordance.** Does clicking the name (a) route
   directly to `/admin/account`, (b) open a small popover/menu with
   `Profile` + `Sign out` items, or (c) open a slide-over panel that
   embeds the profile editor inline?
2. **Sign-out placement.** Inside the same popover/menu as Profile,
   or as a second always-visible button in the sidebar footer
   (e.g. `[ user name ▾ ] [ Sign out ]`)?
3. **Profile page split.** Keep the single `/admin/account` page that
   already houses Profile + Security + Change email, or split into
   `/admin/account/profile` and `/admin/account/security` so the
   sidebar action lands on a cleaner Profile-only view?
4. **Mobile.** Sprint 007 declared desktop-first; do we still want to
   stamp the user menu mobile-friendly (hamburger reveal) now or
   defer that polish?

## Out-of-scope follow-ups

- Avatar / profile photo upload.
- "Sign out everywhere" / multi-session list.
- Theme switcher in the user menu.
- Help / Docs / Feedback links in the user menu.
- Account deletion from inside the profile page.
- Org-switch UI (single-org per user remains the rule).
