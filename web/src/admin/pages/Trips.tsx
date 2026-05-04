import { trips } from "../mock";

export function Trips() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Trips</h1>
          <div className="admin-page-subtitle">
            All upcoming trips across the fleet, in chronological order.
          </div>
        </div>
        <button className="primary">+ Trip</button>
      </div>

      <div className="filter-bar">
        <select defaultValue="any">
          <option value="any">Any status</option>
          <option value="planned">Planned</option>
          <option value="active">Active</option>
          <option value="completed">Completed</option>
          <option value="cancelled">Cancelled</option>
        </select>
        <select defaultValue="any-boat">
          <option value="any-boat">All boats</option>
          <option value="gaia-love">Gaia Love</option>
          <option value="blue-spirit">Blue Spirit</option>
        </select>
        <select defaultValue="90">
          <option value="30">Next 30 days</option>
          <option value="90">Next 90 days</option>
          <option value="365">Next 12 months</option>
          <option value="all">All upcoming</option>
        </select>
        <input type="search" placeholder="Search itinerary or boat..." />
        <div className="filter-bar__spacer" />
      </div>

      <table className="admin-table">
        <thead>
          <tr>
            <th>Dates</th>
            <th>Boat</th>
            <th>Itinerary</th>
            <th>Director</th>
            <th>Manifest</th>
            <th>Price</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {trips.map((t) => (
            <tr key={t.id} className="is-clickable">
              <td>
                {t.startDate} → {t.endDate}
              </td>
              <td>{t.boatName}</td>
              <td>{t.itinerary}</td>
              <td>
                {t.director ?? (
                  <span className="chip chip--warn">Unassigned</span>
                )}
              </td>
              <td className="num">
                {t.manifestFilled} / {t.manifestCapacity}
              </td>
              <td className="num">{t.priceText}</td>
              <td>
                <span
                  className={
                    "chip " +
                    (t.availability === "FULL"
                      ? "chip--full"
                      : "chip--available")
                  }
                >
                  {t.availability}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
