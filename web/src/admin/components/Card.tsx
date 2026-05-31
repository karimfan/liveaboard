import type { ReactNode } from "react";

import styles from "./Card.module.css";

export type CardProps = {
  title?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string; // layout-only composition allowed; never color
  footer?: ReactNode;
};

export function Card({ title, actions, children, className, footer }: CardProps) {
  return (
    <section className={className ? `${styles.card} ${className}` : styles.card}>
      {(title || actions) && (
        <header className={styles.header}>
          {title && <h2 className={styles.title}>{title}</h2>}
          {actions && <div className={styles.actions}>{actions}</div>}
        </header>
      )}
      <div className={styles.body}>{children}</div>
      {footer && <footer className={styles.footer}>{footer}</footer>}
    </section>
  );
}
