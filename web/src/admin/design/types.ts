// Sprint 025 Triptych — runtime design mode types.
//
// The admin app supports three palettes, three IA layouts, and
// three motion modes, all switchable at runtime via a floating
// dock (TriptychSwitcher). Selection is reflected to <html>
// data-attributes (`data-palette`, `data-layout`, `data-motion`)
// which themes.css / motion.css / admin.css consume.
//
// String unions + allowlists are exported so DesignModeProvider
// can validate persisted/URL values before reflecting them into
// the DOM. Anything off the allowlist falls back to defaults.

export type PaletteMode = "abyss" | "glass" | "sunlit";
export type LayoutMode = "rail" | "spaces" | "canvas";
export type MotionMode = "living" | "minimal" | "full";

export type DesignMode = {
  palette: PaletteMode;
  layout: LayoutMode;
  motion: MotionMode;
};

export const PALETTES: readonly PaletteMode[] = ["abyss", "glass", "sunlit"] as const;
export const LAYOUTS: readonly LayoutMode[] = ["rail", "spaces", "canvas"] as const;
export const MOTIONS: readonly MotionMode[] = ["living", "minimal", "full"] as const;

// The default combination on first run when no localStorage + no
// ?triptych= URL. The user can flip from here at any time. Picked
// to be the bold (abyss) palette in the most familiar layout
// (spaces) with subtle ambient motion (living).
export const DEFAULT_MODE: DesignMode = {
  palette: "abyss",
  layout: "spaces",
  motion: "living",
};

export function isPalette(v: unknown): v is PaletteMode {
  return typeof v === "string" && (PALETTES as readonly string[]).includes(v);
}
export function isLayout(v: unknown): v is LayoutMode {
  return typeof v === "string" && (LAYOUTS as readonly string[]).includes(v);
}
export function isMotion(v: unknown): v is MotionMode {
  return typeof v === "string" && (MOTIONS as readonly string[]).includes(v);
}
