import { useEffect, useState, type ReactNode } from "react";

import { useDesignMode } from "../design";

import styles from "./Stat.module.css";

export type StatProps = {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
  tabular?: boolean;
};

export function Stat({ label, value, hint, tabular = true }: StatProps) {
  const display = useCountUp(value);
  return (
    <div className={styles.stat}>
      <div className={styles.label}>{label}</div>
      <div className={tabular ? `${styles.value} ${styles.tabular}` : styles.value}>
        {display}
      </div>
      {hint && <div className={styles.hint}>{hint}</div>}
    </div>
  );
}

// useCountUp animates numeric values on mount when motion === "full".
// Non-numeric values, or any other motion mode, render the value
// unchanged. Uses requestAnimationFrame; no easing library.
function useCountUp(value: ReactNode): ReactNode {
  const { mode } = useDesignMode();
  const numeric = typeof value === "number" ? value : null;
  const [shown, setShown] = useState<number | null>(numeric);

  useEffect(() => {
    if (numeric == null) return;
    if (mode.motion !== "full") {
      setShown(numeric);
      return;
    }
    const target = numeric;
    const start = performance.now();
    const duration = 600;
    let raf = 0;
    const tick = (t: number) => {
      const k = Math.min(1, (t - start) / duration);
      const eased = 1 - Math.pow(1 - k, 3);
      setShown(Math.round(target * eased));
      if (k < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [numeric, mode.motion]);

  if (numeric == null) return value;
  return shown;
}
