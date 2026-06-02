"use strict";

/*
 * liveaboard/font-allowlist
 *
 * Sprint 028 guardrail. A `font-family` declaration in component/page
 * CSS must resolve through the typography tokens, not name a raw family.
 * Allowed values: a var() referencing one of the font tokens, or
 * `inherit`. The token source files (tokens.css) are where the actual
 * family stacks live and are excluded via the Stylelint `ignoreFiles`
 * config, not here.
 */

const stylelint = require("stylelint");

const ruleName = "liveaboard/font-allowlist";
const messages = stylelint.utils.ruleMessages(ruleName, {
  rejected: (value) =>
    `Unexpected font-family "${value}". Use var(--font-display|--font-body|--font-mono) or inherit.`,
});

const ALLOWED_TOKENS = ["--font-display", "--font-body", "--font-mono"];

/** @type {import('stylelint').Rule} */
const rule = (primary) => (root, result) => {
  const validOptions = stylelint.utils.validateOptions(result, ruleName, {
    actual: primary,
    possible: [true, false],
  });
  if (!validOptions || !primary) return;

  root.walkDecls(/^font-family$/i, (decl) => {
    const value = decl.value.trim();
    if (value.toLowerCase() === "inherit") return;
    // Accept only a single var() pointing at an approved token.
    const varMatch = value.match(/^var\(\s*(--font-[a-z-]+)\b/i);
    if (varMatch && ALLOWED_TOKENS.includes(varMatch[1])) return;

    stylelint.utils.report({
      result,
      ruleName,
      message: messages.rejected(value),
      node: decl,
    });
  });
};

rule.ruleName = ruleName;
rule.messages = messages;

module.exports = stylelint.createPlugin(ruleName, rule);
module.exports.ruleName = ruleName;
module.exports.messages = messages;
