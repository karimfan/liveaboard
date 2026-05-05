import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Trip } from "../api";
import { useMe } from "../useMe";
import { AssignDirector, useCruiseDirectors } from "../AssignDirector";

export function Trips() {
  const me = useMe();
  const [trips, setTrips] = useState<Trip[] | null>(null);
  const [scope, setScope] = useState<"all" | "assigned_to_me">("all");
  const [error, setError] = useState<string | null>(null);

  const isAdmin = me.loaded && me.me?.role === "org_admin";
  const { directors } = useCruiseDirectors(isAdmin);

  useEffect(() => {
    let cancelled = false;
    adminApi
      .listTrips()
      .then((res) => {
        if (cancelled) return;
        setTrips(res.trips ?? []);
        setScope(res.scope);
      })
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load trips."));
    return () => {
      cancelled = true;
    };
  }, []);

  // After a successful add/remove, patch the row in place with the
  // server-returned director list so the chips re-render.
  function onChanged(tripId: string, ids: string[], names: string[]) {
    setTrips((prev) =>
      prev
        ? prev.map((t) =>
            t.id === tripId
              ? { ...t, cruise_director_user_ids: ids, cruise_director_names: names }
              : t,
          )
        : prev,
    );
  }

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Trips</h1>
          <div className="admin-page-subtitle">
            {scope === "assigned_to_me"
              ? "Trips assigned to you, in chronological order."
              : "All upcoming trips across the fleet, in chronological order."}
          </div>
        </div>
        {isAdmin && (
          <Link to="/admin/import" className="primary" style={{ display: "inline-block" }}>
            + Import trips
          </Link>
        )}
      </div>

      {error && <div className="error">{error}</div>}
      {!trips ? (
        <div className="muted">Loading…</div>
      ) : trips.length === 0 ? (
        <div className="empty-state">
          <h3>No trips yet</h3>
          <p>
            {isAdmin ? (
              <>
                <Link to="/admin/import">Import trips</Link> from
                liveaboard.com or upload a spreadsheet.
              </>
            ) : (
              "You haven't been assigned to any trips."
            )}
          </p>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th className="col-dates">Dates</th>
              <th>Boat</th>
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
                <td>
                  <Link to={`/admin/fleet/${t.boat_id}`}>{t.boat_name}</Link>
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
      )}
    </>
  );
}
