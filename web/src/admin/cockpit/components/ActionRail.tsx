import { Link } from "react-router-dom";

import type { CockpitCommand } from "../CockpitApi";

import styles from "../Cockpit.module.css";

export function ActionRail({ commands }: { commands: CockpitCommand[] }) {
  return (
    <section className={styles.panel}>
      <div className={styles.panelHeader}>
        <span>Command rail</span>
        <strong>{commands.length}</strong>
      </div>
      <div className={styles.actionGrid}>
        {commands.slice(0, 8).map((cmd) => (
          <Link key={cmd.route} to={cmd.route} className={styles.action}>
            <span>{cmd.kind}</span>
            <strong>{cmd.label}</strong>
          </Link>
        ))}
      </div>
    </section>
  );
}
