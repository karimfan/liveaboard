import type { ReactNode } from "react";

import styles from "./Chip.module.css";

export type ChipVariant = "neutral" | "success" | "warning" | "error" | "info";

export type ChipProps = {
  variant?: ChipVariant;
  children: ReactNode;
};

export function Chip({ variant = "neutral", children }: ChipProps) {
  return <span className={`${styles.chip} ${styles[variant]}`}>{children}</span>;
}
