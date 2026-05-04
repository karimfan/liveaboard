export function Reports() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Reports</h1>
          <div className="admin-page-subtitle">
            Setup completeness, operational status, and revenue.
          </div>
        </div>
      </div>

      <div className="admin-grid">
        <div className="admin-card">
          <h2 className="admin-card__title">Setup completeness</h2>
          <p className="muted">
            See <a href="/admin">Overview</a> for the live setup checklist
            and per-step completion. A trend over time will land here as
            data accumulates.
          </p>
        </div>
        <div className="admin-card">
          <h2 className="admin-card__title">Operational status</h2>
          <p className="muted">
            Trips by status across the fleet. Coming with US-7.2.
          </p>
        </div>
        <div className="admin-card">
          <h2 className="admin-card__title">Revenue per trip</h2>
          <p className="muted">
            Per-trip charges, settled, and outstanding. Coming with US-7.3.
          </p>
        </div>
        <div className="admin-card">
          <h2 className="admin-card__title">Cross-trip analytics</h2>
          <p className="muted">
            Trends across boats and seasons. Deferred (post-MVP).
          </p>
        </div>
      </div>
    </>
  );
}
