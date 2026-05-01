# Design System — Liveaboard

## Product Context
- **What this is:** Operations platform for scuba diving liveaboard operators
- **Who it's for:** Organization owners, org admins, and site directors managing fleet operations, trips, guest manifests, and onboard consumption
- **Space/industry:** Marine hospitality / dive operations SaaS
- **Project type:** Web app / dashboard

## Aesthetic Direction
- **Direction:** Industrial/Utilitarian — function-first, data-dense where needed, clean where not
- **Decoration level:** Minimal — typography and whitespace do the work. No gradients, illustrations, or ocean imagery in the UI.
- **Mood:** Serious, competent, operational. This should feel like a professional business tool — think Linear or Stripe Dashboard — not a dive tourism site.
- **Reference sites:** Competitors (DiveHQ, DiversDesk, Bloowatch, Liveaboard Manager) all converge on ocean-blue tourism aesthetics. This design deliberately breaks from that convention to signal "operations tool."

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
- **Approach:** Restrained — warm slate neutrals + single bold accent. Deliberately no blue.
- **Primary:** #E5853B (Amber/Warm Orange) — warm, urgent, operational. Used for CTAs, active states, accent highlights.
- **Primary hover:** #D0752F
- **Primary subtle:** #FDF3EB — light accent background for badges, highlights
- **Neutrals:** Warm slate scale
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
- **Semantic:**
  - Success: #2D9D5C (bg: #ECFDF3)
  - Warning: #D4930C (bg: #FFF9EB)
  - Error: #DC3545 (bg: #FEF2F2)
  - Info: #3B7CE5 (bg: #EFF6FF)
- **Dark mode:** Invert surfaces, reduce accent saturation ~15%. Accent becomes #D4792F. Background becomes #0F0E0D. Surface becomes #1A1918.

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
