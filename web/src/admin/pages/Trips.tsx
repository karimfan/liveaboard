import { useEffect, useState } from "react";

import { adminApi, type Trip } from "../api";
import { useMe } from "../useMe";

export function Trips() {
  const me = useMe();
  const [trips, setTrips] = useState<Trip[] | null>(null);
  const [scope, setScope] = useState<"all" | "assigned_to_me">("all");
  const [error, setError] = useState<string | null>(null);

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

  const isAdmin = me.loaded && me.me?.role === "org_admin";

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
          <button className="primary" disabled title="Coming next sprint">
            + Trip
          </button>
        )}
      </div>

      {error && <div className="error">{error}</div>}
      {!trips ? (
        <div className="muted">Loading…</div>
      ) : trips.length === 0 ? (
        <div className="empty-state">
          <h3>No trips yet</h3>
          <p>
            {isAdmin
              ? "Use the boat scraper or click + Trip once that flow lands."
              : "You haven't been assigned to any trips."}
          </p>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Dates</th>
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
                <td>
                  {t.start_date} → {t.end_date}
                </td>
                <td>{t.boat_name}</td>
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
      )}
    </>
  );
}
