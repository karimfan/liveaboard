# Sprint 007 Merge Notes

## Claude Draft Strengths

- Clean, opinionated scoring matrix and a clear primary recommendation
  (Option A) with named fall-forwards.
- Concrete first-run checklist for Option A's Overview screen — kept.
- Useful contrast across three IAs (sidebar / boat-centric / calendar)
  that map onto recognizable industry patterns.
- Roadmap stub for the recommended option grounded in real existing
  code paths (sidebar shell, existing repo helpers).

## Codex Draft Strengths

- **"Control Tower" framing** for Option A is materially sharper than
  "Sidebar + Tables." The home screen is an *exception queue + setup
  checklist*, not a static dashboard. This is the right
  product-shaped frame for an admin tool. Adopt verbatim.
- Codex's Option 2 (Fleet Workbench) cleanly factors org-wide concerns
  onto a separate Organization route, eliminating the "where is X
  managed?" muddiness Claude's Option B had.
- 3-sprint implementation stub (008/009/010) is leaner than Claude's
  4-sprint stub (008–011). Tighter cuts.
- Codex bakes in the per-boat **quantity grid** correctly throughout —
  Claude's draft missed this almost entirely.
- Codex's "Trips needing attention" pattern on the Overview is the
  right operational triage primitive (no director assigned, manifest
  fill ratio, low stock, etc.).

## Valid Critiques Accepted

1. **Site Director leakage.** Phase 4 was binding: SD UX is out of
   scope for Sprint 007. Claude's draft kept SD as a domain, scored
   options on "SD chrome simplicity," and added a full SD section.
   **Fix:** strip SD from the surface-area list, the decision matrix,
   and the recommendation criteria. The merged doc covers Admin only.

2. **Inventory model is outdated.** Phase 4 confirmed: "10 XL t-shirts
   on one boat, 40 on another." That's per-boat **quantity tracking**,
   not checkbox availability. Claude's draft kept "stocked items" /
   checkbox language. **Fix:** every option's per-boat inventory
   surface is a numeric quantity grid; Overview surfaces low-stock
   alerts; the data model implies a `boat_inventory` table (item ×
   boat × quantity).

3. **Option B's IA muddiness.** Manifest and Crew are trip-scoped, not
   boat-scoped, but Claude's draft put them as persistent boat-workspace
   tabs. **Fix:** in Option B, Manifest moves to trip detail (slide-
   over from the Schedule tab); the boat workspace's Crew tab
   becomes "directors who have run this boat" (read-only, with
   assignment happening on Trip detail).

4. **Option C's `⋯ Manage` menu hides too much for an MVP.** This
   product is configuration-heavy right now (org settings, boats,
   catalog, users, inventory all need to be set up), so a 4-section
   menu behind a popover is wrong-shaped for first-run. **Fix:**
   sharpen Option C's "weak first-run" critique; recommendation does
   not change (still Option A) but the cons are more honest.

5. **Stale recommendation criteria.** With SD chrome and "stocked
   items" gone, the original Option A reasoning was leaning on the
   wrong supports. **Fix:** reframe the recommendation around (a)
   clean seven-domain mapping, (b) quantity-grid fit, (c) first-run
   checklist as an active triage screen, (d) reuse of the existing
   sidebar shell — not on role gating.

6. **DESIGN.md compliance asserted more than demonstrated.** Token
   recitation is fine; what's missing is what the typography hierarchy
   and density actually look like in the quantity grid, the setup
   checklist, and split-pane detail work. Also, Claude's nav glyphs
   (🛥, 🗓, ⚙, etc) drift from the "serious, competent, operational"
   tone DESIGN.md requires. **Fix:** wireframes use text-only nav
   labels; each component description names the typography family +
   row height + spacing token used.

## Critiques Adjusted (not fully accepted)

- **"Archived" as top-level nav** (Codex's Option 1 has it; Codex
  itself flagged in Open Questions). Decision: **per-domain archived
  filter**, not a top-level destination. Each list view (Boats, Trips,
  Catalog, Users) gets a "Show: Active / Archived / All" toggle.
  Reduces nav weight; matches operator expectation.

## Critiques Rejected (with reasoning)

None of Codex's critiques were rejected outright. Each was either
accepted or adjusted (per above).

## Interview Refinements Applied (already in intent doc)

| Question | Answer | Effect |
|---|---|---|
| UX direction prior | Sidebar + Tables (Option A) | Locked in as recommendation; renamed "Control Tower" per Codex's framing. |
| Site Director scope | Out of scope for Sprint 007 | All SD content removed; SD is a future sprint. |
| Inventory model | Per-boat quantities ("10 vs 40 t-shirts") | Quantity grid in every option; Overview surfaces low-stock. |
| Mobile | Desktop-first | Hamburger collapse <768px; no tablet/phone primary design. |

## Final Decisions

- **Three options preserved**, renamed and sharpened:
  - **Option A — Control Tower** (sidebar shell + active Overview that
    triages exceptions and surfaces setup completeness)
  - **Option B — Fleet Workbench** (boat is the anchor; tabbed
    workspace; trip-scoped work via Trip detail slide-over from the
    Schedule tab)
  - **Option C — Schedule Board** (calendar-first; Manage menu for
    secondary surfaces)
- **Recommendation: Option A (Control Tower).** Reframed reasoning:
  clean seven-domain mapping; best fit for quantity-heavy inventory;
  active Overview as setup-triage screen; reuses the existing sidebar
  shell.
- **Site Director is dropped** from this sprint's surface, decision
  matrix, and roadmap. A future sprint designs SD UX separately.
- **Inventory** = per-boat quantity grid (numeric inputs). Pricing
  stays org-level flat per Sprint 002. Implies a future
  `boat_inventory` schema (item × boat × qty + min/max thresholds).
- **Archive** is a per-domain filter, not a top-level nav.
- **Mobile** = desktop-first; hamburger below 768px.
- **DESIGN.md compliance** demonstrated in wireframes: text-only nav
  labels (no emoji), explicit typography family + row heights +
  spacing tokens called out per screen.
- **Roadmap**: 3 sprints (008/009/010) for the MVP slice, per Codex's
  leaner cut.
