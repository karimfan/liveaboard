export function Inventory() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Inventory</h1>
          <div className="admin-page-subtitle">
            Items, categories, prices, and per-boat stock levels.
          </div>
        </div>
      </div>

      <div className="empty-state">
        <h3>Inventory schema is on the roadmap</h3>
        <p>
          This page will host org-level items + prices and per-boat
          quantities once the schema and CRUD ship in a future sprint.
        </p>
        <p style={{ marginTop: "var(--sp-md)" }} className="muted">
          See <a href="/admin">Overview</a> for what's currently
          configured.
        </p>
      </div>
    </>
  );
}
