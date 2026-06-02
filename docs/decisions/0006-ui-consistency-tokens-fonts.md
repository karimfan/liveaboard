# 0006 — One UI: Semantic-Token Palette, Sea Buttons, Enforced

**Status:** Accepted (Sprint 028)
**Date:** 2026-06-02
**Builds on:** [0005](0005-voyage-cockpit-reboot.md) (cockpit reboot,
semantic-token contract)

## Context

Sprint 027 shipped the dark Voyage Cockpit and a semantic-token contract
but deferred page migration. The result was two visual worlds rendering
at once: the cockpit and a handful of pages read the dark **semantic
tokens** (`--surface-*`, `--text-*`, `--accent-*`), while every other
admin page was styled by `web/src/styles/app.css` against the raw
light **`--c-*`** warm-slate/amber palette. `themes.css` never rebinds
`--c-*`, so those pages stayed light inside the dark shell — the
"some pages differ" / low-contrast problem the user reported. The
typography contract in `DESIGN.md` was also stale (it named General
Sans / DM Sans / Geist while the app actually ships Inter + JetBrains
Mono), and primary buttons used a cyan→violet gradient rather than the
requested sea colors.

## Decision

1. **One palette for the authenticated app: dark sea (manta-night),
   consumed only through semantic tokens.** Admin pages and components
   never read the legacy `--c-*` scale directly. The unification
   direction is legacy → cockpit (pages adopt the dark palette), not the
   reverse — confirmed by the user.

2. **All buttons are sea.** A sea button family lives in `tokens.css`
   (`--btn-sea-*`) with a deep-navy label: solid aquamarine (`#2EB6E5`
   → hover `#0E95CB`, the default), sea gradient (`#6DCEF0 → #0A76A6`),
   and turquoise→mint (`#2EB6E5 → #00FFD1`). The `Button` primitive
   exposes them as `primary` / `primaryGradient` / `primaryElectric`.
   No amber or cyan→violet buttons remain.

3. **Components and pages read semantic tokens, not raw hex or raw font
   names.** Raw hex lives only in the token source files
   (`tokens.css`, `themes.css`). Every page composes the component
   library primitives plus a co-located `*.module.css` for page-local
   layout. `font-family` uses `var(--font-display|--font-body|
   --font-mono)` or `inherit`.

4. **The contract is machine-enforced.** Stylelint runs `color-no-hex`
   plus a custom `liveaboard/font-allowlist` rule over `src/**/*.css`,
   exempting only the token source files (and, for now, the legacy
   `app.css`). Wired into `make lint` and `.github/workflows/ci.yml`.

5. **`DESIGN.md` is the source of truth and was trued-up** to the
   shipped Inter typography, dark-sea color, and sea buttons, with an
   Enforcement section.

## Consequences

- All 24 admin pages were migrated off `app.css` global classes onto
  component primitives + co-located CSS modules during Sprint 028.
- A `.admin-main`-scoped bridge in `admin.css` repaints the few shared
  classes still in transition (and the `AssignDirector` /
  `CurrencyPicker` helpers) onto semantic tokens, so nothing on the
  admin surface reads the light palette.
- `app.css` survives **only** for the light public/guest pages
  (`GuestTab`, `GuestRegistration`, etc.) that still consume it. It is
  exempt from the no-raw-hex rule for now and is slated for deletion
  once those pages migrate (Sprint 029).
- New work: new admin UI must compose primitives + token-only module
  CSS; introducing a raw hex or a new `app.css` dependency fails CI.

## Alternatives considered

- **Unify toward a light sea look** (cockpit moves to pages) — rejected;
  it would reverse the Sprint 027 dark-cockpit direction. Reversible if
  ever wanted: it is one `themes.css` binding change, token names
  unchanged.
- **Lightweight CI grep instead of Stylelint** — rejected; Stylelint
  expresses the exemptions (token files) and the font allowlist cleanly
  and is standard tooling.
- **Delete `app.css` this sprint** — deferred; the public/guest pages
  still depend on it and are out of the admin-consistency scope.
