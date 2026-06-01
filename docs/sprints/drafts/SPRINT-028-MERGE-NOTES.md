# Sprint 028 Merge Notes

> Supersedes the first-pass merge notes. The first pass was written
> against a wrong premise ("fonts never load", "137 raw hex in app.css")
> and wrong interview assumptions. Both Claude and Codex re-grounded on
> the real code and converged; this version is authoritative.

## Factual corrections (both drafts initially wrong, then fixed)

The INTENT and first CLAUDE-DRAFT claimed the fonts never load and that
`app.css` was full of raw hex. Reading the actual code disproved both:

- **Fonts already load** via CDN links in `web/index.html` (Inter, DM
  Sans, JetBrains Mono, General Sans). They are not falling back to
  system-ui. The real font issue is a *stale contract*: `DESIGN.md`
  still documents General Sans / DM Sans / Geist, while `tokens.css`
  actually binds `--font-display`/`--font-body` to **Inter**.
- **`app.css` has 0 raw hex.** It already consumes `var(--c-*)` and
  `var(--font-*)`. The 1902-line / ~39 KB sheet is token-based, not
  hex-littered.
- **The component-library already exists** (`web/src/admin/components/`:
  Page, PageHeader, Card, DataTable, Button, Chip, Field, FormSection,
  ActionBar, Tabs, Empty, Stat) and Sprint 026 migrated 5 hero pages
  onto it.

## The real root cause (the actual "some pages differ")

There are **two token worlds** rendering simultaneously:

- **Cockpit + migrated pages** read the *semantic* tokens
  (`--surface-*`, `--text-*`, `--accent-*`). `themes.css` rebinds those
  to the **dark manta-night** set (navy glass surfaces, cyan accent,
  light text).
- **Legacy pages** (Trips, Fleet, Inventory, Users, Organization,
  Reports, Onboarding, Trip*, Boat*) are styled by `app.css`, which
  reads the **raw `--c-*` palette directly** (`color: var(--c-900)`
  near-black text, `a { color: var(--c-primary) }` amber, `button {
  background: var(--c-900) }`). `themes.css` does **not** rebind `--c-*`,
  so those pages stay in the original **light warm-slate + amber** world.

Result: the Today/cockpit screen is dark cyan glass; click into Trips
and it's light slate cards with amber links and near-black buttons. Same
app, two visual languages. Buttons are doubly split: the `Button`
primitive uses the cyan→violet `--sidebar-active-bg` gradient, while
`app.css` `<button>` is near-black/amber.

## Claude draft strengths (kept)

- Behavior-preserving framing; foundation-first sequencing.
- Per-page migration recipe; Stylelint over manual review.

## Codex draft strengths (adopted)

- **Primitives-audit phase before page migration** — extend the existing
  primitives rather than spawn 20 page modules.
- **Archetype batching** (run-loop / config / support pages).
- **Custom stylelint rules** (`liveaboard/no-raw-hex`,
  `liveaboard/font-allowlist`) with token files exempted — built-in
  `color-no-hex` can't express the exceptions.
- **Concrete CI**: repo has no `.github/workflows`; add one (or a
  checked-in equivalent) running build + lint:styles + make test.
- **Real verification commands** (`cd web && npm run build`,
  `npm run lint:styles`, `make lint`, `make test`) over generic tsc/Go.
- **Accurate file inventory** (no `OrganizationProfile`/`BoatNotes.tsx`;
  boat notes live in `BoatTabs.tsx`).

## Interview answers applied (the TRUE answers)

The AskUserQuestion results were: **all admin pages**, **include the
public/guest funnel**, **stylelint in CI**, **hard-delete app.css when
dead**. (My Codex invocation mistakenly told Codex the opposite — funnel
out / shim. The final doc follows the user's real answers, not Codex's
constrained draft.)

- Scope = **all admin pages + public funnel/guest pages**, plus any auth
  page that still imports `app.css` (required so `app.css` can actually
  become dead).
- `app.css` is **hard-deleted** once grep proves zero consumers — not
  kept as a shim. Codex's shim caution is mooted because the funnel/guest
  pages are now in scope, so few/no consumers remain.
- **Stylelint in CI** with custom no-raw-hex + font-allowlist rules.

## New requirement (post-interview): sea buttons

User: "all buttons marine blue, like the sea — turquoise / aquamarine."
Rendered three sea treatments side by side in the real app; user chose
**keep all three as selectable variants**:

1. **Solid aquamarine** — `#2EB6E5` → hover `#0E95CB`, navy text.
2. **Sea gradient** — `#6DCEF0 → #0A76A6` (lagoon→drop-off), navy text.
3. **Turquoise → mint** — `#2EB6E5 → #00FFD1`, navy text.

All three replace the cyan→violet gradient. The `Button` primitive gains
three primary-weight sea variants; the legacy `app.css` `<button>` rules
are folded into the same family during migration so every button is sea.

## Final decisions

1. **Unify onto the cockpit's semantic-token world.** Legacy pages stop
   reading raw `--c-*` and adopt the same `--surface-*/--text-*/
   --accent-*` tokens the cockpit uses → one palette everywhere.
   **Direction (dark sea, toward the cockpit) confirmed by the user
   2026-05-31.**
2. **Sea button family** = three variants above; rebind
   `--sidebar-active-bg` and `--accent-primary` to sea so links, active
   nav, and focus rings read sea too.
3. **Truth-up `DESIGN.md`** typography + color sections to match the
   implemented tokens (Inter family; manta-night dark sea palette; sea
   buttons). The stale General Sans/DM Sans/Geist + amber sections are a
   real inconsistency source.
4. Phases: token-unify + font/contract truth-up → primitives audit →
   migrate admin pages (by archetype) → migrate funnel/guest/auth pages
   off `app.css` → delete `app.css` + stylelint/CI guardrail.
5. New ADR `0006-ui-consistency-tokens-fonts.md`; amend ADR `0005`.
