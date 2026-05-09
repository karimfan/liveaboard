import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Trip } from "../api";
import { useMe } from "../useMe";
import { AssignDirector, useCruiseDirectors } from "../AssignDirector";

export function Trips() {
  const me = useMe();
  const [trips, setTrips] = useState<Trip[] | null>(null);
  const [scope, setScope] = useState<"all" | "assigned_to_me">("all");
  const [statusFilter, setStatusFilter] = useState("operational");
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
        <>
        <div className="admin-toolbar">
          <label>
            Status
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
              <option value="operational">Operational</option>
              <option value="planned">Planned</option>
              <option value="active">Active</option>
              <option value="completed">Completed</option>
              <option value="cancelled">Cancelled</option>
              <option value="removed">Removed from source</option>
              <option value="all">All</option>
            </select>
          </label>
        </div>
        <table className="admin-table">
          <thead>
            <tr>
              <th className="col-dates">Dates</th>
              <th>Status</th>
              <th>Boat</th>
              <th>Itinerary</th>
              <th>Director</th>
              <th>Guests</th>
              <th>Price</th>
              <th>Availability</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filterTrips(trips, statusFilter).map((t) => (
              <tr key={t.id}>
                <td className="col-dates">
                  {t.start_date} → {t.end_date}
                </td>
                <td><span className={`chip chip--${t.removed_from_source_at ? "removed" : t.status}`}>{t.removed_from_source_at ? "removed" : t.status}</span></td>
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
                <td>
                  <ManifestSummary trip={t} />
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
                <td className="actions-cell">
                  <Link to={`/admin/trips/${t.id}/manifest`}>Manifest</Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </>
      )}
    </>
  );
}

function filterTrips(trips: Trip[], filter: string): Trip[] {
  if (filter === "all") return trips;
  if (filter === "removed") return trips.filter((t) => t.removed_from_source_at);
  if (filter === "operational") return trips.filter((t) => !t.removed_from_source_at && t.status !== "cancelled");
  return trips.filter((t) => !t.removed_from_source_at && t.status === filter);
}

function ManifestSummary({ trip }: { trip: Trip }) {
  const summary = trip.manifest_summary;
  if (!summary) return <span className="muted">0 guests</span>;
  return (
    <div className="manifest-summary">
      <span>{summary.guest_count} guests</span>
      {summary.submitted_count > 0 && <span>{summary.submitted_count} submitted</span>}
      {summary.expected_count != null && <span>{summary.expected_count} expected</span>}
      {summary.has_warning && <span className="error-inline">over expected</span>}
    </div>
  );
}
