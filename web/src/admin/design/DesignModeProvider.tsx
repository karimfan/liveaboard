import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import {
  DEFAULT_MODE,
  type DesignMode,
  isLayout,
  isMotion,
  isPalette,
  type LayoutMode,
  type MotionMode,
  type PaletteMode,
} from "./types";

const STORAGE_KEY = "triptych";
const URL_PARAM = "triptych";

type DesignModeContextValue = {
  mode: DesignMode;
  setPalette: (p: PaletteMode) => void;
  setLayout: (l: LayoutMode) => void;
  setMotion: (m: MotionMode) => void;
  setMode: (m: DesignMode) => void;
};

const DesignModeContext = createContext<DesignModeContextValue | null>(null);

// loadInitialMode resolves the active combination at mount time:
//   1) ?triptych=palette,layout,motion query string (validated)
//   2) localStorage["triptych"] (JSON; validated)
//   3) DEFAULT_MODE
// Any invalid value falls back to the default; we never write
// untrusted strings into DOM attrs.
function loadInitialMode(): DesignMode {
  if (typeof window === "undefined") return DEFAULT_MODE;
  const fromUrl = readFromUrl();
  if (fromUrl) return fromUrl;
  const fromStorage = readFromStorage();
  if (fromStorage) return fromStorage;
  return DEFAULT_MODE;
}

function readFromUrl(): DesignMode | null {
  try {
    const params = new URLSearchParams(window.location.search);
    const raw = params.get(URL_PARAM);
    if (!raw) return null;
    const parts = raw.split(",").map((s) => s.trim());
    if (parts.length !== 3) return null;
    const [p, l, m] = parts;
    if (!isPalette(p) || !isLayout(l) || !isMotion(m)) return null;
    return { palette: p, layout: l, motion: m };
  } catch {
    return null;
  }
}

function readFromStorage(): DesignMode | null {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") return null;
    const { palette, layout, motion } = parsed as Record<string, unknown>;
    if (!isPalette(palette) || !isLayout(layout) || !isMotion(motion)) return null;
    return { palette, layout, motion };
  } catch {
    return null;
  }
}

function persist(mode: DesignMode) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(mode));
  } catch {
    // localStorage can throw in private modes — non-fatal.
  }
}

function reflectToDom(mode: DesignMode) {
  if (typeof document === "undefined") return;
  const html = document.documentElement;
  html.setAttribute("data-palette", mode.palette);
  html.setAttribute("data-layout", mode.layout);
  html.setAttribute("data-motion", mode.motion);
}

export function DesignModeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<DesignMode>(loadInitialMode);

  // Reflect to DOM + persist on every change. Runs on mount too,
  // so the initial mode resolved from URL/localStorage immediately
  // surfaces in <html data-*=...>.
  useEffect(() => {
    reflectToDom(mode);
    persist(mode);
  }, [mode]);

  const setPalette = useCallback((palette: PaletteMode) => {
    setModeState((m) => ({ ...m, palette }));
  }, []);
  const setLayout = useCallback((layout: LayoutMode) => {
    setModeState((m) => ({ ...m, layout }));
  }, []);
  const setMotion = useCallback((motion: MotionMode) => {
    setModeState((m) => ({ ...m, motion }));
  }, []);
  const setMode = useCallback((next: DesignMode) => {
    setModeState(next);
  }, []);

  const value = useMemo<DesignModeContextValue>(
    () => ({ mode, setPalette, setLayout, setMotion, setMode }),
    [mode, setPalette, setLayout, setMotion, setMode],
  );

  return <DesignModeContext.Provider value={value}>{children}</DesignModeContext.Provider>;
}

export function useDesignMode(): DesignModeContextValue {
  const ctx = useContext(DesignModeContext);
  if (!ctx) {
    throw new Error("useDesignMode must be used inside <DesignModeProvider>");
  }
  return ctx;
}
