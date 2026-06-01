import { Link } from "react-router-dom";
import type { CSSProperties } from "react";

import type { CockpitVoyage } from "../CockpitApi";

import styles from "../Cockpit.module.css";

export function VoyageMap({ voyages }: { voyages: CockpitVoyage[] }) {
  if (voyages.length === 0) {
    return (
      <section className={styles.voyageMap}>
        <div className={styles.emptyPlane}>
          <span>No active voyage lanes yet</span>
          <Link to="/admin/trips">Create or import trips</Link>
        </div>
      </section>
    );
  }
  return (
    <section className={styles.voyageMap} aria-label="Voyage map">
      {voyages.map((v, index) => (
        <Link
          key={v.id}
          to={`/admin/trips/${v.id}/dashboard`}
          className={`${styles.voyageTile} ${v.status === "active" ? styles.activeVoyage : ""}`}
          style={{ "--lane": index } as CSSProperties}
        >
          <span className={styles.voyageStatus}>{v.status}</span>
          <strong>{v.boat_name}</strong>
          <span>{v.itinerary}</span>
          <div className={styles.voyageMeta}>
            <span>{v.guest_count}/{v.expected_guests ?? "?"} guests</span>
            <span>{v.cabin_assignments} berths</span>
            <span>{v.open_folio_count} folios</span>
          </div>
          {v.blocker_count > 0 && <b>{v.blocker_count} signal{v.blocker_count === 1 ? "" : "s"}</b>}
        </Link>
      ))}
    </section>
  );
}
