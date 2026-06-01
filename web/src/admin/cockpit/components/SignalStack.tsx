import { Link } from "react-router-dom";

import type { CockpitSignal } from "../CockpitApi";

import styles from "../Cockpit.module.css";

export function SignalStack({ signals }: { signals: CockpitSignal[] }) {
  return (
    <section className={styles.panel}>
      <div className={styles.panelHeader}>
        <span>Signals</span>
        <strong>{signals.length}</strong>
      </div>
      <div className={styles.signalList}>
        {signals.length === 0 ? (
          <p className={styles.quiet}>No blockers on the board.</p>
        ) : (
          signals.map((signal, i) => (
            <Link
              key={`${signal.kind}-${signal.route}-${i}`}
              to={signal.route}
              className={`${styles.signal} ${signal.severity === "blocker" ? styles.blocker : styles.warning}`}
            >
              <span>{signal.severity}</span>
              <strong>{signal.label}</strong>
              <small>{signal.detail}</small>
            </Link>
          ))
        )}
      </div>
    </section>
  );
}
