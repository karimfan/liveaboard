import { useOutletContext } from "react-router-dom";

import type { Boat, InventoryRow, Trip } from "../mock";

type Ctx = { boat: Boat; inventory: InventoryRow[]; trips: Trip[] };

function useBoatCtx(): Ctx {
  return useOutletContext<Ctx>();
}

export function BoatTrips() {
  const { trips, boat } = useBoatCtx();
  if (trips.length === 0) {
    return (
      <div className="empty-state">
        <h3>No trips yet</h3>
        <p>This boat has no scheduled trips.</p>
      </div>
    );
  }
  return (
    <table className="admin-table">
      <thead>
        <tr>
          <th>Dates</th>
          <th>Itinerary</th>
          <th>Director</th>
          <th>Manifest</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {trips.map((t) => (
          <tr key={t.id} className="is-clickable">
            <td>
              {t.startDate} → {t.endDate}
            </td>
            <td>{t.itinerary}</td>
            <td>{t.director ?? <span className="chip chip--warn">Unassigned</span>}</td>
            <td className="num">
              {t.manifestFilled} / {t.manifestCapacity}
            </td>
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
      {/* boat is captured for closure type-safety; render nothing extra */}
      <tfoot style={{ display: "none" }}>{boat.name}</tfoot>
    </table>
  );
}

export function BoatInventory() {
  const { inventory, boat } = useBoatCtx();
  if (inventory.length === 0) {
    return (
      <div className="empty-state">
        <h3>No stock entries yet</h3>
        <p>
          Add catalog items and set per-boat quantities to enable low-stock
          alerts.
        </p>
        <p style={{ marginTop: "var(--sp-md)" }}>
          <button className="primary">+ Add stock entry</button>
        </p>
      </div>
    );
  }
  return (
    <>
      <div className="filter-bar">
        <input type="search" placeholder={`Search ${boat.name} inventory...`} />
        <div className="filter-bar__spacer" />
        <button className="primary">+ Add stock entry</button>
      </div>
      <table className="admin-table">
        <thead>
          <tr>
            <th>Item</th>
            <th>Category</th>
            <th>On hand</th>
            <th>Min</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {inventory.map((row) => (
            <tr key={row.itemId}>
              <td>{row.itemName}</td>
              <td>{row.category}</td>
              <td className="num">
                <input className="inline-num" defaultValue={row.onHand} />
              </td>
              <td className="num">
                <input className="inline-num" defaultValue={row.minThreshold} />
              </td>
              <td>
                {row.onHand < row.minThreshold ? (
                  <span className="chip chip--low">low</span>
                ) : (
                  <span className="chip chip--ok">ok</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="muted" style={{ marginTop: "var(--sp-md)" }}>
        Edits in this mockup are not persisted.
      </p>
    </>
  );
}

export function BoatNotes() {
  return (
    <div className="empty-state">
      <h3>Notes (placeholder)</h3>
      <p>Free-form notes about this boat will live here.</p>
    </div>
  );
}
