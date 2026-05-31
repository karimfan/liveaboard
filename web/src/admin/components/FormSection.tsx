import type { ReactNode } from "react";

import styles from "./FormSection.module.css";

export type FormSectionProps = {
  title?: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
};

export function FormSection({ title, hint, children }: FormSectionProps) {
  return (
    <fieldset className={styles.section}>
      {title && <legend className={styles.title}>{title}</legend>}
      {hint && <div className={styles.hint}>{hint}</div>}
      <div className={styles.body}>{children}</div>
    </fieldset>
  );
}
