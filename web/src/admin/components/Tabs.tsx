import type { ReactNode } from "react";

import styles from "./Tabs.module.css";

export type TabItem = {
  key: string;
  label: ReactNode;
  badge?: ReactNode;
};

export type TabsProps = {
  items: TabItem[];
  active: string;
  onSelect: (key: string) => void;
};

export function Tabs({ items, active, onSelect }: TabsProps) {
  return (
    <div role="tablist" className={styles.tabs}>
      {items.map((it) => {
        const isActive = it.key === active;
        return (
          <button
            type="button"
            key={it.key}
            role="tab"
            aria-selected={isActive}
            tabIndex={isActive ? 0 : -1}
            onClick={() => onSelect(it.key)}
            className={isActive ? `${styles.tab} ${styles.active}` : styles.tab}
          >
            <span>{it.label}</span>
            {it.badge != null && <span className={styles.badge}>{it.badge}</span>}
          </button>
        );
      })}
    </div>
  );
}
