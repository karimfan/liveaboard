# Design System — Liveaboard

## Product Context
- **What this is:** Operations platform for scuba diving liveaboard operators
- **Who it's for:** Organization owners, org admins, and site directors managing fleet operations, trips, guest manifests, and onboard consumption
- **Space/industry:** Marine hospitality / dive operations SaaS
- **Project type:** Web app / dashboard

## Aesthetic Direction
- **Direction:** Industrial/Utilitarian on the working surfaces; subtle marine-brand mood on the page-level background.
- **Decoration level:** Minimal where it counts. Working surfaces (sidebar, cards, tables, forms) lean on typography + whitespace; no ocean imagery, no illustrations. The body-level gradient (Sprint 011) is the one decorative element — it signals "scuba liveaboard product" without pushing into tourism-site territory because the working surfaces stay opaque slate.
- **Mood:** Serious, competent, operational with a faint nod to the sea. Linear/Stripe density on the data; brand recognition at the chrome edges.
- **Reference sites:** Competitors (DiveHQ, DiversDesk, Bloowatch, Liveaboard Manager) all converge on saturated ocean-blue tourism aesthetics. This design uses a sea gradient at the page level so the product feels at home in the dive industry, but holds the line on slate working surfaces so the operator-facing tool still looks like Linear, not Booking.com.

## Typography
- **Display/Hero:** General Sans 700 — geometric, confident, modern without being trendy
- **Body:** DM Sans 400/500/600 — highly readable, warm character, excellent for UI text
- **UI/Labels:** DM Sans (same as body)
- **Data/Tables:** Geist (tabular-nums) — precise, aligned columns for the ledger
- **Code:** JetBrains Mono
- **Loading:** Google Fonts (DM Sans, JetBrains Mono), Font Share (General Sans), CDN (Geist)
- **Scale:**
  - xs: 0.75rem (12px)
  - sm: 0.875rem (14px)
  - base: 1rem (16px)
  - lg: 1.125rem (18px)
  - xl: 1.25rem (20px)
  - 2xl: 1.5rem (24px)
  - 3xl: 2rem (32px)
  - 4xl: 2.5rem (40px)

## Color
- **Approach:** Two-layer palette. **Working surfaces** (sidebar, cards, tables, forms) use the warm slate scale + amber accent — the original "operations tool" tone, optimized for density and contrast. **Page-level mood** uses a turquoise sea gradient applied to `<body>` so the SPA feels at home for a scuba liveaboard product without compromising surface legibility.
- **Primary:** #E5853B (Amber/Warm Orange) — warm, urgent, operational. Used for CTAs, active states, accent highlights.
- **Primary hover:** #D0752F
- **Primary subtle:** #FDF3EB — light accent background for badges, highlights
- **Neutrals (working-surface scale):** Warm slate
  - 50: #F8F7F6
  - 100: #F0EEEC
  - 200: #E3E0DD
  - 300: #CCC8C3
  - 400: #A8A29E
  - 500: #78716C
  - 600: #57534E
  - 700: #44403C
  - 800: #292524
  - 900: #1C1917
- **Sea palette (page-level mood, Sprint 011):** Tropical Caribbean ocean — crystal lagoon at the top fading to open ocean blue at the bottom. Deliberately cyan-leaning, not teal.
  - 50: #F0FBFF — sun on white sand
  - 100: #D6F3FF — foam edge
  - 200: #A9E3F7 — knee-deep tropical water
  - 300: #6DCEF0 — lagoon
  - 400: #2EB6E5 — open shallow ocean
  - 500: #0E95CB — Caribbean noon
  - 600: #0A76A6 — drop-off
  - 700: #085680 — deep blue (still bright; not navy)
  - **`--gradient-sea`** = `linear-gradient(180deg, sea-50 0%, sea-200 22%, sea-400 60%, sea-600 100%)`. Applied to `body { background: var(--gradient-sea); background-attachment: fixed; }` so scrolling doesn't tile or stretch the gradient.
  - **Where to use it:** Page background only. Do NOT use sea tokens on cards, tables, or form surfaces — those stay opaque white over the gradient. Reasonable exceptions: the auth-page wordmark uses `--c-sea-700` for brand emphasis.
- **Semantic:**
  - Success: #2D9D5C (bg: #ECFDF3)
  - Warning: #D4930C (bg: #FFF9EB)
  - Error: #DC3545 (bg: #FEF2F2)
  - Info: #3B7CE5 (bg: #EFF6FF)
- **Dark mode:** Invert surfaces, reduce accent saturation ~15%. Accent becomes #D4792F. Background becomes #0F0E0D. Surface becomes #1A1918. Sea gradient is suppressed in dark mode (deep teal would compete with chrome contrast).

## Spacing
- **Base unit:** 8px
- **Density:** Comfortable — crew needs to scan fast but not feel cramped
- **Scale:** 2xs(2px) xs(4px) sm(8px) md(16px) lg(24px) xl(32px) 2xl(48px) 3xl(64px)

## Layout
- **Approach:** Grid-disciplined — strict columns, predictable alignment
- **Grid:** Sidebar (220px) + fluid content area on desktop; single column on mobile
- **Max content width:** 1200px
- **Border radius:** sm(4px) md(8px) lg(12px) full(9999px)

## Motion
- **Approach:** Minimal-functional — only transitions that aid comprehension
- **Easing:** enter(ease-out) exit(ease-in) move(ease-in-out)
- **Duration:** micro(50-100ms) short(150-250ms) medium(250-400ms) long(400-700ms)

## Decisions Log
| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-29 | Initial design system created | Created by /design-consultation based on competitive research of dive operator SaaS products |
| 2026-04-29 | No blue in palette | Every competitor uses ocean-blue. Amber accent signals "business tool, not tourism site" |
| 2026-04-29 | Industrial/utilitarian aesthetic | Operators managing money and manifests need a tool that looks serious and competent |
| 2026-04-29 | Warm slate neutrals over cool grays | Pairs with amber accent; feels physical and grounded rather than clinical |
| 2026-05-05 | **Reverse the no-blue rule — introduce a tropical-ocean gradient** at the page level (Sprint 011). | The original rule held "no blue at all"; in practice the SPA felt anonymous next to a product whose customers run dive boats. Compromise: keep slate as the working-surface palette so tables/forms stay neutral and dense, but apply a cyan-leaning ocean gradient to `<body>` so the chrome edges hint at the Caribbean shallows. Cards and the sidebar stay white over the gradient. The original intent (don't look like a tourism site) is preserved by the surface palette, not by avoiding blue everywhere. |
| 2026-05-05 | Auth pages re-centered, wordmark recolored | Sprint 011 — the unauthenticated pages were left-skewed in some viewports and the wordmark used `--c-900` which got lost against the lighter card. Centered the auth-shell with flex and recolored the wordmark to `--c-sea-700` for brand emphasis. |
| 2026-05-05 | First sea palette was teal; revised to true ocean blue | Sprint 011 — initial pass used greenish hex codes (`#34a092` etc.) which read as teal/aquamarine, not the bright Caribbean tropical blue requested. Swapped to a cyan-leaning palette (`#2eb6e5`, `#0e95cb`, `#085680`) that evokes Bahamas/Maldives postcards. Same role in the system; just the right hue. |
| 2026-05-05 | Admin chrome centered as a block | Sprint 011 — the admin grid (`sidebar + main`) now caps at `--content-max` (1200px) and `margin: 0 auto`. On viewports wider than 1200px, the body gradient peeks through on either side and a soft tinted shadow makes the chrome feel like a centered island floating on the sea. On narrower viewports it fills the width naturally. Sidebar stays white; `.admin-main` gains `--c-50` background so cards still pop against it. |
| 2026-05-30 | **Bold-for-divers reversal** of the "industrial/utilitarian" posture | Sprint 025 — the original "Linear/Stripe density, accounting-tool calm" framing produced generic results. The audience is adventurous (Raja Ampat / Galapagos / Maldives operators), so the design system should look like it. Future visual decisions should lead with the distinctive option, not the safe one. |
| 2026-05-30 | **Triptych: ship three competing redesigns behind a runtime switcher** instead of picking one direction up front | Sprint 025 — three palettes, three layouts (rail / spaces / canvas), three motion modes (living / minimal / full). 27 combinations live-switchable from a floating bottom-right dock gated by `useDevFlags().ui_redesign_switcher`. See [ADR 0004](docs/decisions/0004-triptych-runtime-evaluation.md). Picking a winning combo collapses into Sprint 026. |
| 2026-05-30 | **Palette round 2: reef / harbor / midnight** replaces abyss / glass / sunlit | Sprint 025 — round 1 was "weak, tepid, cold, boring." Navy + cyan + translucent white is the default for every B2B dashboard. Round 2 lands warm-saturated palettes drawn from what divers actually see: tropical fish in sunlight (reef — cream + hot coral + magenta + turquoise), sunset on the boat deck (harbor — cream + sunset coral + plum + gold), and bioluminescent plankton on a night dive (midnight — deep plum + magenta-pink + electric turquoise + ember). No navy. No cyan as primary. No translucent glass. |
| 2026-05-31 | **Palette round 3: ship the user's 5-theme bundle verbatim** — `coral-surge` / `manta-night` / `reef-carnival` / `abyss-flame` / `seaglass-electric` | Round 2 was also rejected — sidebar floated disconnected from the main body, and even the bolder warm-tropical execution still felt timid. The user supplied a hand-crafted theme bundle (`~/Downloads/liveaboard-themes.zip`) with paired sidebar-and-content surface designs, per-theme `--background-overlay` gradients over a derived underwater photo (`/background-derived.jpg`), gradient active-state pills, and translucent content areas with backdrop-blur. `themes.css` is the bundle's five `[data-palette]` blocks verbatim plus one shared translation layer that maps the bundle's tokens (`--primary`, `--sidebar-bg`, `--content-bg`, …) onto the existing semantic tokens so every component module picks up the theme without per-component edits. Default = `abyss-flame` (the bundle's recommended admin-cockpit pick). |
| 2026-05-31 | **Body composite is now per-theme**, no longer Sprint 011 verbatim | Round 3 — each theme drives `--background-overlay` on `<body>` over the new derived underwater photo with a `--page-bg` fallback color, all `background-attachment: fixed`. Sprint 011's white-wash + sea-gradient composite is no longer applied; `--gradient-sea` survives in tokens.css for any direct consumer (e.g. the auth wordmark color). |
| 2026-05-31 | **Round 4: pick `manta-night`, restore the Sprint 011 body composite verbatim** | User decision after seeing the five themes in the running app: "manta night BUT with the original background theme. Do not change the background at all." `base.css` reverts to the Sprint 011 composite character-for-character — white-wash gradient over `/background.jpg` over `--gradient-sea`, three layers `background-attachment: fixed`. The original photo (still in `web/public/background.jpg`) is what the user actually liked. The bundle's `--background-overlay` tokens stay defined per theme but are no longer applied to `<body>`. Other manta-night tokens (sidebar, surfaces, accents) remain in effect, so the dark navy chrome with cyan/violet/magenta accents floats on the original photo. DEFAULT_MODE = `manta-night`; the other four palettes stay switchable. |
| 2026-05-30 | **Semantic-token contract** — components read only `--surface-*` / `--text-*` / `--accent-*` / `--border-*` / `--status-*` | Sprint 025 — each palette rebinds the semantic layer in `themes.css`. Components never reference raw hex. Adding/removing a palette is one CSS block, not a 25-page rewrite. Body sea gradient stays in `:root` and is NOT part of the semantic rebind set — preserved verbatim across every palette. |
