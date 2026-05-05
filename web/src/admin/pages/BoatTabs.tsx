import { useState } from "react";
import { useOutletContext } from "react-router-dom";

import type { Boat, Trip } from "../api";
import { useMe } from "../useMe";
import { AssignDirector, useCruiseDirectors } from "../AssignDirector";

type Ctx = { boat: Boat; trips: Trip[] };

function useBoatCtx(): Ctx {
  return useOutletContext<Ctx>();
}

export function BoatTrips() {
  const me = useMe();
  const ctx = useBoatCtx();

  // Local copy so director assignments patch in place without a
  // refetch of the boat detail page.
  const [trips, setTrips] = useState<Trip[]>(ctx.trips);

  const isAdmin = me.loaded && me.me?.role === "org_admin";
  const { directors } = useCruiseDirectors(isAdmin);

  function onChanged(tripId: string, ids: string[], names: string[]) {
    setTrips((prev) =>
      prev.map((t) =>
        t.id === tripId
          ? { ...t, cruise_director_user_ids: ids, cruise_director_names: names }
          : t,
      ),
    );
  }

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
          <th className="col-dates">Dates</th>
          <th>Itinerary</th>
          <th>Director</th>
          <th>Price</th>
          <th>Availability</th>
        </tr>
      </thead>
      <tbody>
        {trips.map((t) => (
          <tr key={t.id}>
            <td className="col-dates">
              {t.start_date} → {t.end_date}
            </td>
            <td>{t.itinerary}</td>
            <td>
              <AssignDirector
                trip={t}
                directors={directors}
                canEdit={isAdmin}
                onChanged={onChanged}
              />
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
        Schema and CRUD ship in a future sprint.
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
