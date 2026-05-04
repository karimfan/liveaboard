export function Catalog() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Catalog</h1>
          <div className="admin-page-subtitle">
            Org-level items, categories, and prices.
          </div>
        </div>
      </div>

      <div className="empty-state">
        <h3>Catalog schema lands in Sprint 009</h3>
        <p>
          Items and categories are defined here at the org level (one
          item, one price). Per-boat <em>quantities</em> live on each
          boat's <strong>Inventory</strong> tab once both schemas land.
        </p>
        <p style={{ marginTop: "var(--sp-md)" }} className="muted">
          See <a href="/admin">Overview</a> for what's currently
          configured.
        </p>
      </div>
    </>
  );
}
