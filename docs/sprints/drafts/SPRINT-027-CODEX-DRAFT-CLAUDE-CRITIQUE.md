# Sprint 027 — Claude Critique of Codex Draft

A real Claude pass on `SPRINT-027-CODEX-DRAFT.md`, grounded in the
current `main` (HEAD = `2911e4d sprint-026 phase 1: data
foundations + token-bucket rate limiter`). The previous file at this
path was a placeholder written by Codex itself after a Claude
connection error; this replaces it.

## Top-Level Verdict

The draft has the right strategic instinct (collapse Triptych,
build one decisive operating surface, stop adding pages) but treats
the repo as if it were still the pre-026 state, leans on adjective
strings ("spectacular", "luminous", "cinematic") in place of design
specifics, and bundles a critical-page migration onto an
already-large cockpit build that will almost certainly slip.

It should ship as the next sprint, but with three concrete changes:

1. **Acknowledge Sprint 026 Phase 1 already landed.** The cockpit
   should opportunistically light up the data that's already in the
   store, not pretend that work doesn't exist.
2. **Cut the five-page run-loop migration from Sprint 027.** Make
   it Sprint 028's whole sprint. The cockpit and the Triptych
   collapse are already a full sprint.
3. **Replace the visual-language section with a tight visual
   contract** — token assignments, tile dimensions, density rules,
   reference image, and one explicit anti-pattern. "Premium" is not
   a spec.

## What the Draft Gets Right

- **Aggregate endpoint is the correct architectural move.** Six
  parallel admin fetches on Overview today is a regression hazard;
  one `GET /api/admin/cockpit` aligned to a view-model keeps the
  cockpit cheap and lets the implementer reason about role scoping
  in one place. ✓
- **Pause-the-funnel posture.** Adding `/charters/:slug` + portal +
  crew CRUD pages on top of the current Overview-as-card-grid would
  bake mediocrity in. Doing the cockpit first is the right order. ✓
- **Triptych collapse is overdue.** ADR 0004 explicitly anticipates
  this; the only question is "delete vs dev-isolate," which the
  draft hedges on (see below). ✓
- **Role-aware first screen.** Org Admin and Cruise Director landing
  on the same Overview today is a long-standing wart. Sprint 027
  should fix it. ✓
- **Component-library-only for new code.** Reaffirming the
  no-raw-hex / `web/src/admin/components/` contract from ADR 0004 is
  load-bearing for the collapse. ✓
- **Mention of fixtures-first build.** The draft includes
  `web/src/admin/cockpit/fixtures.ts`. This is essential and
  underplayed — see below.

## Repo-State Facts the Draft Missed

These are not nitpicks; they change the scope and the file list.

### 1. Sprint 026 Phase 1 is already on `main`

The draft says "pause Sprint 026." Phase 1 of Sprint 026 has
already shipped:

```
internal/store/leads.go
internal/store/booking_quotes.go
internal/store/offline_payments.go
internal/store/crew.go
internal/store/equipment.go
internal/store/readiness.go
internal/store/guest_certifications.go
internal/store/guest_portal_requests.go
internal/httpapi/rate_limit.go
internal/httpapi/rate_limit_test.go
```

Implications the draft must address:

- The cockpit aggregate can read `leads`, `booking_quotes`,
  `offline_payments`, `crew_certifications.expires_on`, and
  `equipment_assets.service_due_on` **today**. The cockpit's
  "readiness" and "money" sections can light up real data
  immediately instead of waiting for hypothetical Sprint 028+
  endpoints.
- "Pausing Sprint 026" should be read as: do not ship any new
  *public* route, *guest portal* route, or *funnel UI* in Sprint
  027 — but the data already in the store is fair game for the
  cockpit to project.
- The migration `0021_sprint_026_funnel_portal_readiness.sql` is
  presumably applied. The draft should explicitly say what
  happens to it — it stays (cheap), and the cockpit reads from
  it where useful.

### 2. `app.css` is 39,872 bytes today

The draft says "shrink `app.css` materially." Without a number this
is unverifiable. Sprint 026's plan committed to `< 30,720 bytes`.
Sprint 027 should either keep that target (if the five-page
migration stays) or explicitly drop it (if the migration is cut to
Sprint 028, see scope cut below).

### 3. `DesignModeProvider` writes to `<html>`, not the admin shell

```ts
// DesignModeProvider.tsx — current behavior
html.setAttribute("data-palette", mode.palette);
html.setAttribute("data-layout", mode.layout);
html.setAttribute("data-motion", mode.motion);
```

That means `themes.css` `[data-palette="…"]` selectors target the
whole document, including auth pages, guest portal sketches, and
any future public surface. The draft's "collapse to `manta-night`"
plan needs to specify whether:

- the semantic tokens move into `:root` unconditionally
  (preferred — one less indirection); or
- `[data-palette="manta-night"]` stays on `<html>` and the four
  losing palette blocks are deleted from `themes.css`.

The collapse procedure section of ADR 0004 already prescribes
option (a). Sprint 027 should follow that, not invent a new path.

### 4. `Shell.tsx` already dispatches three nav renderers

```tsx
// Shell.tsx — current behavior
{mode.layout === "rail" ? <RailNav … />
 : mode.layout === "canvas" ? <CanvasNav … />
 : <SpacesNav … />}
```

The draft's "Phase 2: Collapse Triptych" should call out exactly
the deletions:

- `RailNav.tsx` + `.module.css` → delete
- `CanvasNav.tsx` + `.module.css` → delete
- `CommandBar.tsx` + `.module.css` → delete or repurpose into the
  cockpit's CommandPalette (preferred — it already handles ⌘K and
  the role-filtered NAV)
- `TodayCanvas.tsx` → delete or fold into the cockpit's
  `VoyageMap` (TodayCanvas already proves the spatial idea works
  off Overview API data — start from it, don't ignore it)
- `TriptychSwitcher.tsx` + dev flag `ui_redesign_switcher` →
  delete (not dev-isolate; see below)
- `web/src/admin/design/types.ts` → keep `PaletteMode` /
  `LayoutMode` / `MotionMode` exports? No. Delete them. The whole
  `design/` directory shrinks to a one-line file with a `motion`
  hook for `prefers-reduced-motion` if needed at all.
- `internal/httpapi/dev_flags_handlers.go` → drop the
  `ui_redesign_switcher` field
- `web/src/lib/devFlags.ts` → drop the type

### 5. `CommandBar` already exists

`web/src/admin/components/CommandBar.tsx` is already a ⌘K route
launcher fed by `NAV`. The draft proposes a new
`CommandPalette.tsx` under `cockpit/`. Either repurpose `CommandBar`
into the cockpit's palette or rename and move it. Don't ship two
overlapping components.

## Scope: Cut the Page Migration

The draft's Phase 5 migrates five pages (`Trips`, `TripDashboard`,
`TripManifest`, `TripConsumptionLedger`, `GuestFolio`) and the
Overview-becomes-cockpit. That is roughly Sprint 026 Phase 5 in
disguise, ported into Sprint 027 underneath a Cockpit build that
is itself larger than any single Sprint-025 phase.

A back-of-envelope shows this is 1.5 sprints:

- Cockpit aggregate (store + handler + tests + role scoping): ~3d
- Cockpit UI (six new components + states + fixtures + wire):
  ~4-5d
- Triptych collapse (deletes, CSS surgery, removing unused
  `app.css` `[data-palette]` blocks): ~1-2d
- Five-page migration (each page non-trivial — `TripManifest`
  and `GuestFolio` are the largest in the tree): ~5-7d
- Docs (ADR 0005, amend 0004, DESIGN/CLAUDE/CODEX, persona
  amend if cockpit changes role landings): ~1d

**Recommendation:** Sprint 027 ships Cockpit + Triptych collapse +
the *Overview-replaced-by-Cockpit* migration only. The other four
run-loop pages migrate in Sprint 028 (alongside the rest of the 17
unmigrated pages, on a single rhythm). This also avoids touching
`TripManifest` while the cockpit is still hardening — those two
surfaces share concerns and will fight each other for design
attention.

If the user wants the five-page migration kept, the cockpit's
animation/visual ambition is what should be cut, not the migration.
Be explicit about the tradeoff.

## Visual Language: Not Yet a Spec

The Visual Language section is the weakest part of the draft.
Phrases like "luminous glass-metal workplane", "voyage tracks
feeling like a deck plan", "blockers as hot markers" describe a
mood, not a build target. Two implementers will produce two
different surfaces from this.

What the spec should look like:

```
Cockpit grid (desktop ≥ 1280px):
  Header strip          —  72px tall
  VoyageMap             — fills width; min-height 360px,
                          max-height 480px (fixed by content)
  3-column signal row   — 320px / 320px / 1fr; gap 16px
  MoneyStrip            — 96px tall
  ActivityStream        — 240px tall, scrollable

Tile dimensions (no shifting):
  small  — 220 × 96
  medium — 320 × 140
  large  — full-width × 200

Density:
  Body text 14px / 1.45
  Tabular data 13px / 1.4 tabular-nums
  Section labels 11px uppercase 0.06em tracking

Color (manta-night tokens, semantic only):
  Idle action       → --accent-primary (cyan)
  Selection / focus → --accent-secondary (violet)
  Blocker / risk    → --status-error (magenta)
  Money / warning   → --status-warning (ember)
  Surface base      → --surface-panel (deep navy)
  Surface elevated  → --surface-panel-strong

Anti-patterns:
  - No nested cards. The cockpit is one plane with regions.
  - No drop-shadow on tiles. Glow on hover only.
  - No animated counters on first paint. Stat values are
    immediate.
  - No filled buttons in the cockpit body. Quiet outlines
    plus accent ink; commands route to existing pages where
    filled CTAs live.
```

Sprint 027 doesn't need to land all of that verbatim, but the
draft should commit to *something* this concrete.

## Background Question

The draft locks in "manta-night + original Sprint 011 body
background" as the only authenticated baseline. This matches the
user's Round 4 decision (2026-05-31) and matches the stored
memory `feedback-bold-aesthetic`. But the intent doc explicitly
asks:

> Is the original body background sacred for the authenticated app,
> or can the cockpit use route-specific immersive media while
> preserving the original background on auth/public pages?

The draft answers "sacred" without engagement. That's a hedge — a
reboot is exactly the moment to reopen this. Two options worth
naming:

- **Keep original background under the cockpit too.** Coherent;
  preserves the user's stated preference; cockpit must earn
  premium feel through chrome and density alone. Lower risk.
- **Cockpit gets a dimmed underwater hero behind the workplane;
  auth and public pages keep the Sprint 011 composite.** Higher
  visual ambition; explicitly route-scoped (`<body>` background
  swap by `useLocation()` or a CSS class on the admin shell);
  reversible. Higher payoff if the imagery is right.

The merged sprint plan should pick one and say so. The current
"don't change the background" framing is safe but undersells the
reboot.

## Aggregate Endpoint — Specifics Missing

The DTO sketch is fine as far as it goes but doesn't answer
operational questions:

- **What's the caching posture?** None proposed, none needed at
  Sprint 027 scale. State this explicitly so the implementer
  doesn't add Redis. Same for ETags.
- **Concurrency.** Six store queries in parallel via
  `errgroup.WithContext` is the obvious shape. Worth naming so
  partial failures degrade gracefully (`failed_sections: []` in
  the response is a useful escape hatch — cockpit renders what
  it got).
- **Column projection.** CLAUDE.md as of Sprint 026 requires
  explicit column enumeration on public read paths; even though
  `/api/admin/cockpit` is authenticated, the same rule keeps PII
  footprint small. Enumerate.
- **Org Admin vs Cruise Director scoping.** Sketch says
  "assignment scoped" — make it concrete. Director sees:
  voyages where `trip_cruise_directors.user_id = me`, money
  totals for those voyages only, no fleet pulse, no inventory,
  no cross-trip audit. The endpoint returns a different
  *shape* per role (or omits sections rather than zeroing them).
- **Test plan.** `internal/store/cockpit_test.go` and
  `internal/httpapi/cockpit_handlers_test.go` should explicitly
  cover: (a) cross-org isolation, (b) director seeing zero
  voyages when unassigned, (c) graceful empty state on a
  fresh-seeded org. Skipped-when-no-Postgres caveat applies as
  for all other store tests.

## TriptychSwitcher: Delete, Don't Dev-Isolate

The draft says "delete or dev-isolate" the switcher. Pick delete.
The reasons to dev-isolate (debug a future palette, A/B compare,
visual regression) all assume the *abandoned* palettes will be
revived; if that ever happens, restoring code from git is one
command, and the kept code rots in the meantime. ADR 0004 also
specifies the collapse as deletion, not dev-isolation.

Delete: `TriptychSwitcher.tsx` + module CSS + `useDevFlags`
field + backend dev-flag field + the four non-winning palette
blocks in `themes.css` + the `RailNav`/`CanvasNav`/`TodayCanvas`
files (folding `TodayCanvas`'s spatial idea into `VoyageMap`) +
the `[data-palette]` / `[data-layout]` selectors in `admin.css`.

## Fixture Strategy Is Underspecified

The draft mentions `web/src/admin/cockpit/fixtures.ts` once and
moves on. A reboot judged "in the live app" (per ADR 0004
philosophy) needs real fixtures or it's judged against an empty
dev DB and looks like vaporware.

Sprint 027 should ship two fixtures:

1. **Org Admin fixture** — a synthetic `CockpitResponse` covering
   3 active voyages, 2 upcoming, 4 readiness blockers (mix of
   crew cert, equipment service, manifest gap), 2 open folios
   over $5k, one low-stock signal, six activity events.
2. **Cruise Director fixture** — same shape, scoped to two
   assigned trips, no fleet pulse, no inventory, restricted
   activity feed.

These power Storybook-style review (already feasible via the
component library) and the development cockpit when the local DB
is empty. Without them the visual judgment loop is dead.

## Migration of `Overview.tsx` Specifically

Today's `Overview.tsx` branches by role and by layout
(`canvas` → `TodayCanvas`). When the cockpit replaces it:

- Default route `/admin` renders `Cockpit`.
- Role branch moves into Cockpit (it's the shape that varies, not
  the route).
- `TodayCanvas` is folded into `VoyageMap` (start point: the
  flagged-trip tile grid already implemented at
  `TodayCanvas.tsx:71-83`).
- `CruiseDirectorLanding` is deleted; the Director view is the
  cockpit with reduced sections.
- The Sprint 023 onboarding auto-show at `Shell.tsx:20-44` must
  still fire on `/admin` for org admins whose onboarding isn't
  complete. The cockpit must not break this.

## Documentation Manifest Quibbles

- `docs/decisions/0005-voyage-cockpit-reboot.md` (new) — agreed.
- `docs/decisions/0004-triptych-runtime-evaluation.md` (amend) —
  agreed; mark Status: Superseded by 0005, keep the body for
  archaeology.
- `docs/sprints/SPRINT-026.md` — agreed it should be marked
  superseded/deferred, but the wording must acknowledge that
  Phase 1 *already shipped*. The right note is: "Phase 1 (data
  foundations + rate limiter) shipped at commit 2911e4d. Phases
  2-6 deferred to Sprint 028+; the cockpit (Sprint 027) reads
  Phase 1 data opportunistically without lighting up the public
  funnel UI."
- `docs/product/personas.md` — only amend if the cockpit changes
  what the persona's first screen shows. It does (CD landing is
  no longer the trip list — it's a Director-scoped cockpit), so
  yes, amend.
- `docs/product/organization-admin-user-stories.md` — the draft
  doesn't list this; the cockpit collapses several user stories
  into one screen and that should be acknowledged.
- `current_status.md` — file does not exist in repo. Drop from
  the manifest or stop hedging with "if absent."
- `local_setup.md` — file does not exist in repo. Drop.

## Definition of Done — Concrete Holes

Most DoD items are fine. These need sharper criteria:

- *"`app.css` shrinks materially."* → "`wc -c web/src/styles/
  app.css` returns less than 30,720" (matching the cut Sprint
  026 had committed to), or, if the five-page migration is cut,
  drop this DoD item.
- *"No text overlap at mobile width."* → "Cockpit renders without
  clipped controls at 375 × 667 and 414 × 896; tile widths fall
  back to 100% below 768px; no horizontal scroll."
- *"Command palette supports keyboard open, search, route
  selection, escape close, and focus return."* → keep; add "focus
  ring visible on every option per `--focus-ring` token."
- *"Documentation Manifest items are completed."* → keep; the
  `sprint` skill enforces this anyway.

Add:

- "`grep -rE 'admin-card|admin-page-header|admin-page-title|
  setup-list__|chip--' web/src/admin/cockpit web/src/admin/
  pages/Overview.tsx` returns zero matches."
- "`themes.css` has zero `[data-palette]` blocks (only `:root`)."
- "`web/src/admin/design/` is deleted or shrunk to a one-line
  motion hook."

## Risks the Draft Underestimates

- **Cockpit-as-decoration risk** is named but not mitigated. The
  mitigation is the fixture strategy above plus a DoD that every
  cockpit tile resolves to an existing mutation endpoint. The
  draft should say so.
- **Triptych collapse rippling into auth and guest pages.** The
  `data-palette` attr is on `<html>`, so auth pages currently
  consume `manta-night` tokens via inheritance. After collapse,
  auth pages must still render. Worth one explicit smoke step.
- **CSS specificity from `[data-palette]` selectors in
  `admin.css`.** Removing the `[data-layout]` selectors without
  cleaning the cascade can leave dead rules that still win
  specificity battles. Phase 2 should grep `admin.css` for
  `[data-` and clean every match.
- **Persona/Onboarding auto-show regression.** Cockpit must not
  break `Shell.tsx:20-44`. Add a manual smoke step.

## Parts to Keep Verbatim

These sections of the Codex draft don't need rewriting; they
should ship as-is into the final sprint:

- Use Cases 1-6 (good shape).
- The aggregate response sketch (sections — names need
  tightening but the shape is right).
- The "What We Keep / Pause / Delete" list (sharp, useful).
- API Endpoints table (one row is correct).
- Security Considerations (accurate; PII concern is real).
- Risks/Mitigations table (keep, with the two additions above).

## Suggested Final Structure for the Merged Plan

```
# Sprint 027: Voyage Cockpit Reboot

## Overview                              (lightly rewritten)
## Product Stance                        (keep)
## Use Cases                             (keep)
## Architecture
  ### Aggregate endpoint                 (tighten projection)
  ### Cockpit shell                      (concrete grid + tokens)
  ### Triptych collapse procedure        (concrete deletes)
  ### Fixture strategy                   (NEW)
  ### Route + role scoping               (Director shape)
## Implementation Plan
  Phase 1: Reboot lock + ADR             (~10%)
  Phase 2: Triptych collapse             (~20%)
  Phase 3: Aggregate endpoint            (~20%)
  Phase 4: Cockpit UI                    (~35%)
  Phase 5: Overview→Cockpit migration    (~10%)
  Phase 6: Verification + docs           (~5%)
  (Five-page migration deferred to S028.)
## API Endpoints                         (one row, agreed)
## Files Summary                         (rewrite with deletes)
## Definition of Done                    (sharper, see above)
## Security                              (keep)
## Documentation Manifest                (drop nonexistent files)
## Risks + Mitigations                   (add 2 rows)
```

## One-Line Summary

Ship the cockpit, collapse Triptych, defer the four extra page
migrations to Sprint 028, replace the visual mood-board with a
concrete token-and-dimension spec, and acknowledge that
Sprint 026 Phase 1 is already in the codebase.
