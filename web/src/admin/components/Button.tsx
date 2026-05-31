import type { ButtonHTMLAttributes, ReactNode } from "react";

import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "quiet";

export type ButtonProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className"> & {
  variant?: ButtonVariant;
  loading?: boolean;
  children: ReactNode;
  className?: string; // layout-only composition; never color
};

export function Button({
  variant = "secondary",
  loading = false,
  disabled,
  children,
  className,
  ...rest
}: ButtonProps) {
  const cls = [styles.btn, styles[variant], className].filter(Boolean).join(" ");
  return (
    <button
      type="button"
      {...rest}
      disabled={disabled || loading}
      className={cls}
      aria-busy={loading || undefined}
    >
      {loading ? <span className={styles.spinner} aria-hidden /> : null}
      <span className={styles.label}>{children}</span>
    </button>
  );
}
