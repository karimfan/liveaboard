import type { ReactNode } from "react";

import styles from "./Stat.module.css";

export type StatProps = {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
  tabular?: boolean;
};

export function Stat({ label, value, hint, tabular = true }: StatProps) {
  return (
    <div className={styles.stat}>
      <div className={styles.label}>{label}</div>
      <div className={tabular ? `${styles.value} ${styles.tabular}` : styles.value}>
        {value}
      </div>
      {hint && <div className={styles.hint}>{hint}</div>}
    </div>
  );
}
