import { NavLink } from "react-router-dom";

import type { NavSpace } from "../nav";

import styles from "./SpacesNav.module.css";

export type SpacesNavProps = {
  nav: NavSpace[];
  brand: React.ReactNode;
  footer?: React.ReactNode;
};

export function SpacesNav({ nav, brand, footer }: SpacesNavProps) {
  return (
    <aside className={styles.sidebar} aria-label="Primary navigation">
      <div className={styles.brand}>{brand}</div>
      <nav className={styles.nav}>
        {nav.map((space) => (
          <div key={space.key} className={styles.space}>
            <div className={styles.spaceLabel}>{space.label}</div>
            {space.items.map((item) => (
              <div key={item.to} className={styles.group}>
                <NavLink
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) =>
                    isActive ? `${styles.link} ${styles.active}` : styles.link
                  }
                >
                  {item.label}
                </NavLink>
                {item.children?.map((child) => (
                  <NavLink
                    key={child.to}
                    to={child.to}
                    className={({ isActive }) =>
                      isActive
                        ? `${styles.link} ${styles.child} ${styles.active}`
                        : `${styles.link} ${styles.child}`
                    }
                  >
                    {child.label}
                  </NavLink>
                ))}
              </div>
            ))}
          </div>
        ))}
      </nav>
      {footer && <div className={styles.footer}>{footer}</div>}
    </aside>
  );
}
