import { useState } from "react";
import { NavLink } from "react-router-dom";

import { flattenNav, type NavSpace } from "../nav";

import { CommandBar } from "./CommandBar";
import styles from "./RailNav.module.css";

export type RailNavProps = {
  nav: NavSpace[];
  brand: React.ReactNode;
  footer?: React.ReactNode;
};

export function RailNav({ nav, brand, footer }: RailNavProps) {
  const [open, setOpen] = useState(false);
  const items = flattenNav(nav);
  return (
    <>
      <aside className={styles.rail} aria-label="Primary navigation">
        <button
          type="button"
          className={styles.brand}
          onClick={() => setOpen(true)}
          aria-label="Open command bar"
          title={typeof brand === "string" ? brand : undefined}
        >
          {typeof brand === "string" ? brand.slice(0, 1).toUpperCase() : brand}
        </button>
        <nav className={styles.nav}>
          {nav.map((space) => (
            <div key={space.key} className={styles.space}>
              {space.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) =>
                    isActive ? `${styles.link} ${styles.active}` : styles.link
                  }
                  title={item.label}
                >
                  <span className={styles.glyph}>{item.glyph ?? item.label.slice(0, 1)}</span>
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        {footer && <div className={styles.footer}>{footer}</div>}
      </aside>
      <CommandBar items={items} open={open} onClose={() => setOpen(false)} />
    </>
  );
}
