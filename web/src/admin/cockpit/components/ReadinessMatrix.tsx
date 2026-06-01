import { Link } from "react-router-dom";

import type { CockpitInventory, CockpitVoyage } from "../CockpitApi";

import styles from "../Cockpit.module.css";

export function ReadinessMatrix({
  voyages,
  inventory,
}: {
  voyages: CockpitVoyage[];
  inventory: CockpitInventory[];
}) {
  const guests = voyages.reduce((n, v) => n + v.guest_count, 0);
  const cabins = voyages.reduce((n, v) => n + v.cabin_assignments, 0);
  const submitted = voyages.reduce((n, v) => n + v.submitted_count, 0);
  const directors = voyages.filter((v) => v.director_count > 0).length;
  return (
    <section className={styles.panel}>
      <div className={styles.panelHeader}>
        <span>Readiness</span>
        <strong>
          {Math.round(readinessPct(voyages, submitted, cabins, directors))}%
        </strong>
      </div>
      <div className={styles.readinessGrid}>
        <Cell
          label="Registration"
          value={`${submitted}/${guests}`}
          to="/admin/trips"
        />
        <Cell label="Berths" value={`${cabins}/${guests}`} to="/admin/trips" />
        <Cell
          label="Directors"
          value={`${directors}/${voyages.length}`}
          to="/admin/users"
        />
        <Cell
          label="Inventory"
          value={inventory.length === 0 ? "clear" : `${inventory.length} low`}
          to="/admin/inventory"
        />
      </div>
    </section>
  );
}

function Cell({
  label,
  value,
  to,
}: {
  label: string;
  value: string;
  to: string;
}) {
  return (
    <Link to={to} className={styles.readinessCell}>
      <span>{label}</span>
      <strong>{value}</strong>
    </Link>
  );
}

function readinessPct(
  voyages: CockpitVoyage[],
  submitted: number,
  cabins: number,
  directors: number,
) {
  if (voyages.length === 0) return 100;
  const guests = voyages.reduce((n, v) => n + v.guest_count, 0);
  const guestScore = guests === 0 ? 1 : (submitted + cabins) / (guests * 2);
  const directorScore = directors / voyages.length;
  return Math.max(0, Math.min(100, ((guestScore + directorScore) / 2) * 100));
}
