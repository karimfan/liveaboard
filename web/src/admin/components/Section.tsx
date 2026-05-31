import type { ReactNode } from "react";

import styles from "./Section.module.css";

export type SectionProps = {
  title?: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
};

export function Section({ title, hint, children }: SectionProps) {
  return (
    <div className={styles.section}>
      {(title || hint) && (
        <div className={styles.head}>
          {title && <h3 className={styles.title}>{title}</h3>}
          {hint && <div className={styles.hint}>{hint}</div>}
        </div>
      )}
      <div className={styles.body}>{children}</div>
    </div>
  );
}
