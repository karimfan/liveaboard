import type { ReactNode } from "react";

import styles from "./Page.module.css";
import { PageHeader } from "./PageHeader";

export type PageProps = {
  title: string;
  subtitle?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
};

export function Page({ title, subtitle, actions, children }: PageProps) {
  return (
    <div className={styles.page}>
      <PageHeader title={title} subtitle={subtitle} actions={actions} />
      <div className={styles.body}>{children}</div>
    </div>
  );
}
