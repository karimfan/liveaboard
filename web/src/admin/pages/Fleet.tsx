import { Link } from "react-router-dom";

import { boats, inventoryByBoat } from "../mock";

export function Fleet() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Fleet</h1>
          <div className="admin-page-subtitle">
            All boats in your organization.
          </div>
        </div>
        <button className="primary">+ Add boat</button>
      </div>

      <div className="filter-bar">
        <select defaultValue="active">
          <option value="active">Active</option>
          <option value="archived">Archived</option>
          <option value="all">All</option>
        </select>
        <input type="search" placeholder="Search boats..." />
        <div className="filter-bar__spacer" />
      </div>

      <table className="admin-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Capacity</th>
            <th>Upcoming trips</th>
            <th>Stock alerts</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {boats.map((b) => {
            const lowCount =
              (inventoryByBoat[b.id] ?? []).filter((r) => r.onHand < r.minThreshold).length;
            return (
              <tr key={b.id}>
                <td>
                  <Link to={`/admin/fleet/${b.slug}`}>{b.name}</Link>
                </td>
                <td className="num">{b.capacity}</td>
                <td className="num">{b.upcomingTrips}</td>
                <td>
                  {lowCount === 0 ? (
                    <span className="chip chip--ok">none</span>
                  ) : (
                    <span className="chip chip--low">{lowCount} low</span>
                  )}
                </td>
                <td>
                  {b.status === "active" ? (
                    <span className="chip chip--active">Active</span>
                  ) : (
                    <span className="chip chip--archived">Archived</span>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </>
  );
}
