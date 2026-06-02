"use strict";

/*
 * Sprint 028 style guardrail.
 *
 * Enforces the design-token contract on component/page CSS:
 *   - no raw hex colors (colors must come from semantic tokens)
 *   - font-family must use the typography tokens or `inherit`
 *
 * The token SOURCE files define the actual palette + font stacks and
 * are excluded via `ignoreFiles`. app.css is the legacy sheet still
 * serving light auth/guest pages; it is exempted until those pages
 * migrate (Sprint 028 remaining / Sprint 029), then it gets deleted.
 */

module.exports = {
  plugins: ["./stylelint-rules/font-allowlist.cjs"],
  rules: {
    "color-no-hex": true,
    "liveaboard/font-allowlist": true,
  },
  ignoreFiles: [
    "src/styles/tokens.css",
    "src/styles/themes.css",
    "src/styles/app.css",
    "dist/**",
    "node_modules/**",
  ],
};
