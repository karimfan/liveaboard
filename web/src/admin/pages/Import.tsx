import { Link } from "react-router-dom";

// Sprint 012 — import hub. Two cards. The actual wizards live at
// /admin/import/liveaboard and /admin/import/spreadsheet.
export function Import() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Import trips</h1>
          <div className="admin-page-subtitle">
            Bring your fleet's schedule into Liveaboard.
          </div>
        </div>
      </div>

      <div className="admin-grid">
        <Link to="/admin/import/liveaboard" className="admin-card import-card">
          <h2 className="admin-card__title">From liveaboard.com</h2>
          <p>
            Paste a boat's URL and we'll fetch the next 18 months of
            trips. Re-running is safe — your operator-edited names and
            guest counts stay put.
          </p>
        </Link>

        <Link to="/admin/import/spreadsheet" className="admin-card import-card">
          <h2 className="admin-card__title">Upload a spreadsheet</h2>
          <p>
            <strong>.csv</strong> or <strong>.xlsx</strong> with columns for
            vessel name, start date, end date, itinerary. Number of
            guests is optional. Preview before commit.
          </p>
        </Link>
      </div>
    </>
  );
}
