import { Link } from "react-router-dom";

import { PageHeader } from "../components";

import styles from "./Import.module.css";

// Sprint 012 — import hub. Two cards. The actual wizards live at
// /admin/import/liveaboard and /admin/import/spreadsheet.
export function Import() {
  return (
    <>
      <PageHeader
        title="Import trips"
        subtitle="Bring your fleet's schedule into Liveaboard."
      />

      <div className={styles.grid}>
        <Link to="/admin/import/liveaboard" className={styles.card}>
          <h2 className={styles.cardTitle}>From liveaboard.com</h2>
          <p>
            Paste a boat's URL and we'll fetch every published trip on the
            listing. Re-running is safe — your operator-edited names and guest
            counts stay put.
          </p>
        </Link>

        <Link to="/admin/import/spreadsheet" className={styles.card}>
          <h2 className={styles.cardTitle}>Upload a spreadsheet</h2>
          <p>
            <strong>.csv</strong> or <strong>.xlsx</strong> with columns for
            vessel name, start date, end date, itinerary. Number of guests is
            optional. Preview before commit.
          </p>
        </Link>
      </div>
    </>
  );
}
