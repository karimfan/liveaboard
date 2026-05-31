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

export type PaletteMode =
  | "coral-surge"
  | "manta-night"
  | "reef-carnival"
  | "abyss-flame"
  | "seaglass-electric";

export type LayoutMode = "rail" | "spaces" | "canvas";
export type MotionMode = "living" | "minimal" | "full";

export type DesignMode = {
  palette: PaletteMode;
  layout: LayoutMode;
  motion: MotionMode;
};

export const PALETTES: readonly PaletteMode[] = [
  "coral-surge",
  "manta-night",
  "reef-carnival",
  "abyss-flame",
  "seaglass-electric",
] as const;
export const LAYOUTS: readonly LayoutMode[] = ["rail", "spaces", "canvas"] as const;
export const MOTIONS: readonly MotionMode[] = ["living", "minimal", "full"] as const;

// Round 3 default (user-supplied theme bundle): the bundle's
// README recommends Abyss & Flame "for the admin cockpit," so
// that's what first-run visitors see. Layout stays `spaces`
// (the most familiar shell); motion stays `living` (caustic
// ambience without page-slide).
export const DEFAULT_MODE: DesignMode = {
  palette: "abyss-flame",
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
