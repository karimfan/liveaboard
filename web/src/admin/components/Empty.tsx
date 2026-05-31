import type { ReactNode } from "react";

import styles from "./Empty.module.css";

export type EmptyProps = {
  title: ReactNode;
  hint?: ReactNode;
  action?: ReactNode;
};

export function Empty({ title, hint, action }: EmptyProps) {
  return (
    <div className={styles.empty}>
      <div className={styles.title}>{title}</div>
      {hint && <div className={styles.hint}>{hint}</div>}
      {action && <div className={styles.action}>{action}</div>}
    </div>
  );
}
