import { useOutletContext } from "react-router-dom";

import type { Boat, Trip } from "../api";

type Ctx = { boat: Boat; trips: Trip[] };

function useBoatCtx(): Ctx {
  return useOutletContext<Ctx>();
}

export function BoatTrips() {
  const { trips } = useBoatCtx();
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
          <th>Price</th>
          <th>Availability</th>
        </tr>
      </thead>
      <tbody>
        {trips.map((t) => (
          <tr key={t.id}>
            <td>
              {t.start_date} → {t.end_date}
            </td>
            <td>{t.itinerary}</td>
            <td>
              {t.site_director_name ?? (
                <span className="chip chip--warn">Unassigned</span>
              )}
            </td>
            <td className="num">{t.price_text ?? "—"}</td>
            <td>
              {t.availability_text ? (
                <span
                  className={
                    "chip " +
                    (t.availability_text.toUpperCase().includes("FULL")
                      ? "chip--full"
                      : "chip--available")
                  }
                >
                  {t.availability_text}
                </span>
              ) : (
                <span className="muted">—</span>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function BoatInventory() {
  return (
    <div className="empty-state">
      <h3>Inventory tracking coming next sprint</h3>
      <p>
        Per-boat stock quantities for each catalog item will live here.
        Schema and CRUD ship in Sprint 009.
      </p>
    </div>
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
