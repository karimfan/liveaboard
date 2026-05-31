import { useState } from "react";
import { NavLink } from "react-router-dom";

import { flattenNav, type NavSpace } from "../nav";

import { CommandBar } from "./CommandBar";
import styles from "./CanvasNav.module.css";

export type CanvasNavProps = {
  nav: NavSpace[];
  brand: React.ReactNode;
  footer?: React.ReactNode;
};

export function CanvasNav({ nav, brand, footer }: CanvasNavProps) {
  const [open, setOpen] = useState(false);
  const items = flattenNav(nav);
  return (
    <>
      <header className={styles.topbar} aria-label="Primary navigation">
        <div className={styles.brand}>{brand}</div>
        <nav className={styles.nav}>
          {items
            .filter((it) => !it.to.includes("/import") && !it.to.includes("/organization/"))
            .map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  isActive ? `${styles.link} ${styles.active}` : styles.link
                }
              >
                {item.label}
              </NavLink>
            ))}
        </nav>
        <button
          type="button"
          className={styles.search}
          onClick={() => setOpen(true)}
          aria-label="Open command bar"
        >
          <span aria-hidden>⌘K</span>
          <span className={styles.searchHint}>Find&hellip;</span>
        </button>
        {footer && <div className={styles.footer}>{footer}</div>}
      </header>
      <CommandBar items={items} open={open} onClose={() => setOpen(false)} />
    </>
  );
}
