import { catalogItems } from "../mock";

export function Catalog() {
  const categories = Array.from(new Set(catalogItems.map((i) => i.category))).sort();
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Catalog</h1>
          <div className="admin-page-subtitle">
            Org-level items + categories + prices. Per-boat quantities live on
            each boat under <em>Fleet → Boat → Inventory</em>.
          </div>
        </div>
        <button className="primary">+ Add item</button>
      </div>

      <div className="filter-bar">
        <select defaultValue="all">
          <option value="all">All categories</option>
          {categories.map((c) => (
            <option key={c} value={c}>{c}</option>
          ))}
        </select>
        <select defaultValue="active">
          <option value="active">Active</option>
          <option value="archived">Archived</option>
          <option value="all">All</option>
        </select>
        <input type="search" placeholder="Search items..." />
        <div className="filter-bar__spacer" />
      </div>

      <table className="admin-table">
        <thead>
          <tr>
            <th>Item</th>
            <th>Category</th>
            <th>Price</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {catalogItems.map((it) => (
            <tr key={it.id} className="is-clickable">
              <td>{it.name}</td>
              <td>{it.category}</td>
              <td className="num">{it.priceText}</td>
              <td>
                {it.active ? (
                  <span className="chip chip--active">Active</span>
                ) : (
                  <span className="chip chip--archived">Archived</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
