import type { ReactNode } from "react";

import styles from "./ActionBar.module.css";

export type ActionBarProps = {
  primary?: ReactNode;
  secondary?: ReactNode;
  status?: ReactNode;
};

export function ActionBar({ primary, secondary, status }: ActionBarProps) {
  return (
    <div className={styles.bar}>
      <div className={styles.status}>{status}</div>
      <div className={styles.actions}>
        {secondary}
        {primary}
      </div>
    </div>
  );
}
