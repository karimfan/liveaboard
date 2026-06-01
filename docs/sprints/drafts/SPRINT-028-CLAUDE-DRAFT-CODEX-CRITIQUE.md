# Codex Critique of Claude Sprint 028 Draft

## Valid Strengths Worth Preserving

- The draft correctly frames Sprint 028 as a behavior-preserving visual
  consistency sprint, not a feature or IA sprint.
- The foundation-first sequencing is right: make fonts load and clean the
  base/token layer before migrating pages.
- The per-page migration recipe is useful and should survive into the
  final plan: create modules, replace raw values with semantic tokens,
  apply font roles, delete dead rules, run QA.
- It calls out the core user-visible failure well: the cockpit can look
  finished while legacy pages still feel like a different app.
- It correctly chooses Stylelint over ad hoc manual review as the
  long-term guardrail.

## Major Concerns

1. **It violates the explicit `app.css` decision.**

   The human planner already decided that `app.css` is **not**
   hard-deleted in Sprint 028. Claude's target architecture says
   `app.css: DELETED`, Phase 3 deletes it, Files Summary deletes it, and
   Definition of Done requires deletion. This must be changed to a
   shrinking shim with an owner comment and Sprint 029 retirement target.

2. **It leaves settled scope as open questions.**

   The user explicitly locked scope to **all admin pages migrated this
   sprint** and public/guest funnel pages out of scope. Claude's Open
   Questions ask whether to migrate all pages or prioritize the top five,
   and whether to hard-delete `app.css`. These questions should be
   removed from the final plan and converted into requirements.

3. **It risks public/guest regressions by deleting global CSS.**

   Since public/guest/auth pages are out of scope, deleting `app.css`
   could accidentally restyle or break them even if admin pages no longer
   depend on it. The safer Sprint 028 posture is to remove migrated admin
   selectors from `app.css`, keep any unavoidable non-admin compatibility
   rules token-only, and defer hard deletion to Sprint 029 when public
   scope can be planned deliberately.

4. **It does not include a Documentation Manifest.**

   Recent sprint docs, especially Sprint 027, include a Documentation
   Manifest with specific docs that must be updated. The user explicitly
   requested this section. Claude's draft only mentions `DESIGN.md` in
   Files Summary and DoD. The final plan needs a proper manifest covering
   `DESIGN.md`, `CODEX.md`, `CLAUDE.md`, the final sprint doc, and any
   optional ADR or Sprint 029 follow-on note.

5. **The CI requirement is underspecified.**

   The human decision is "stylelint in CI with no-raw-hex +
   font-allowlist rules." Claude says "wire into existing lint/CI step,"
   but this repo currently has no `.github` workflow. The final plan
   should name the CI file or equivalent checked-in CI entry and require
   it to run stylelint. Local scripts alone do not satisfy the decision.

## Missing Implementation Details

- **Font contract reconciliation:** Claude assumes Space Grotesk, Inter,
  and JetBrains Mono because the intent says those are referenced, but
  the current `DESIGN.md` still names General Sans, DM Sans, Geist, and
  JetBrains Mono while `tokens.css` currently uses Inter stacks. The
  final sprint must explicitly reconcile `DESIGN.md`, `tokens.css`, and
  loaded font packages before implementation starts.
- **Stylelint mechanics:** The draft names built-in Stylelint options,
  but `color-no-hex` by itself cannot express all project exceptions
  cleanly. The final plan should allow custom local rules such as
  `liveaboard/no-raw-hex` and `liveaboard/font-allowlist`, with token
  source files exempted and admin/page/component CSS enforced.
- **Admin route inventory:** The draft omits some admin routes or names
  files imprecisely. `OrganizationProfile` and standalone `BoatNotes.tsx`
  do not appear in the current `web/src/admin/pages` file list; boat
  notes are exported from `BoatTabs.tsx`. The final plan should use the
  actual route/file inventory from `main.tsx`.
- **Shared primitives:** Claude's plan is heavily page-module oriented.
  That is workable, but it should first audit/extend existing primitives
  (`Page`, `PageHeader`, `Card`, `DataTable`, `Button`, `Chip`, `Field`,
  `FormSection`, `ActionBar`, `Tabs`, `Empty`, `Stat`) so pages do not
  each recreate the same styling in local modules.
- **Admin helper components:** `AssignDirector`, `CurrencyPicker`,
  `UserMenu`, `Shell`, and `SpacesNav` can also carry legacy/global
  styling. They should be in scope because they render inside admin.
- **Verification commands:** DoD should include `cd web && npm run
  lint:styles`, `cd web && npm run build`, `make lint`, and `make test`.
  Claude's `tsc`, `prettier`, and Go tooling language does not map as
  precisely to the repo's current scripts.

## Suggested Changes

- Replace every `app.css` deletion requirement with:
  - reduce `web/src/styles/app.css` to a documented Sprint 028 shim;
  - remove migrated admin selectors;
  - keep it imported;
  - enforce token-only CSS inside it;
  - defer hard deletion to Sprint 029.
- Remove the Open Questions section or reduce it to genuinely unresolved
  implementation details. Do not ask again about all-admin scope,
  public/guest scope, stylelint, or `app.css` deletion.
- Add a Documentation Manifest modeled after Sprint 027.
- Add a dedicated phase for shared primitives before page batches.
- Split migration batches by product surface:
  - admin run-loop pages;
  - org-admin configuration pages;
  - support pages and cockpit edges.
- Add actual CI work: create `.github/workflows/ci.yml` or document the
  equivalent checked-in CI entry if the project uses another runner.
- Update Files Summary and Definition of Done so they reflect all admin
  pages, a shimmed `app.css`, and CI-enforced Stylelint.

## Risks the Final Merge Should Address

| Risk | Why it matters | Final-plan mitigation |
|---|---|---|
| Hard-deleting `app.css` breaks out-of-scope public/guest pages | Public/guest pages are explicitly not being migrated | Keep `app.css` as a shrinking token-only shim until Sprint 029 |
| Page-by-page modules create another styling island | Repeated layout patterns may drift again | Prefer shared primitives first, page modules only for local layout |
| Stylelint blocks legitimate token files | Palette source files must define colors | Exempt `tokens.css` and `themes.css`; enforce pages/components/shims |
| Font names remain contradictory | Current docs and tokens do not fully agree | Make Phase 1 reconcile docs, tokens, and installed assets |
| Broad admin migration causes behavior regressions | Many pages contain forms and mutations | Keep API/state code unchanged; smoke both Org Admin and Cruise Director routes |

## Parts to Reject or Simplify

- Reject the target architecture line `app.css: DELETED`.
- Reject Phase 3's deletion of `app.css` and removal of its import from
  `main.tsx`.
- Reject DoD items requiring `app.css` deletion or "every admin page
  imports only a co-located module"; shared primitives are preferable to
  forcing every page to own a module.
- Reject Open Questions 1, 2, and 3 because the human planner already
  answered them.
- Simplify bundle-size discussion: font bundle size should be checked,
  but it should not distract from the main consistency work unless it is
  unexpectedly large.
