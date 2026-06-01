import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import type { NavItem } from "../nav";

import styles from "./CommandBar.module.css";

export type CommandBarProps = {
  items: NavItem[];
  open: boolean;
  onClose: () => void;
};

// CommandBar is a ⌘K route launcher fed by the role-filtered
// NAV (already flattened). It's used by the rail + canvas
// layouts; spaces hides it because the full sidebar already
// exposes everything.
//
// Global ⌘K/Ctrl+K listener lives in useGlobalCommandShortcut;
// pages that mount layouts wire the shortcut to setOpen(true).
export function CommandBar({ items, open, onClose }: CommandBarProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const returnFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (open) {
      returnFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      return;
    }
    setQuery("");
    returnFocusRef.current?.focus();
    returnFocusRef.current = null;
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  const q = query.trim().toLowerCase();
  const matches = q
    ? items.filter((it) => it.label.toLowerCase().includes(q) || it.to.includes(q))
    : items;

  return (
    <div className={styles.backdrop} onClick={onClose}>
      <div
        className={styles.palette}
        role="dialog"
        aria-modal="true"
        aria-label="Command bar"
        onClick={(e) => e.stopPropagation()}
      >
        <input
          className={styles.input}
          autoFocus
          placeholder="Jump to a page…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && matches[0]) {
              navigate(matches[0].to);
              onClose();
            }
          }}
        />
        <ul className={styles.list}>
          {matches.length === 0 ? (
            <li className={styles.empty}>No matches.</li>
          ) : (
            matches.map((it) => (
              <li key={it.to}>
                <button
                  type="button"
                  className={styles.item}
                  onClick={() => {
                    navigate(it.to);
                    onClose();
                  }}
                >
                  <span className={styles.label}>{it.label}</span>
                </button>
              </li>
            ))
          )}
        </ul>
      </div>
    </div>
  );
}

// useGlobalCommandShortcut binds ⌘K / Ctrl+K to open the
// command bar. Layouts call it from their effect tree.
export function useGlobalCommandShortcut(onOpen: () => void) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && (e.key === "k" || e.key === "K")) {
        e.preventDefault();
        onOpen();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onOpen]);
}
