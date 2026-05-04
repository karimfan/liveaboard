import { useEffect, useState } from "react";
import { Link, NavLink, Outlet, useParams } from "react-router-dom";

import { adminApi, type Boat, type Trip } from "../api";

export function BoatDetail() {
  const { id } = useParams();
  const [boat, setBoat] = useState<Boat | null>(null);
  const [trips, setTrips] = useState<Trip[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    Promise.all([adminApi.getBoat(id), adminApi.listBoatTrips(id)])
      .then(([b, t]) => {
        if (cancelled) return;
        setBoat(b);
        setTrips(t.trips ?? []);
      })
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load boat."));
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (error) {
    return (
      <div className="empty-state">
        <h3>Boat not found</h3>
        <p>
          <Link to="/admin/fleet">Back to fleet</Link>
        </p>
      </div>
    );
  }
  if (!boat) return <div className="muted">Loading…</div>;

  return (
    <>
      <div className="admin-breadcrumb">
        <Link to="/admin/fleet">Fleet</Link> · {boat.name}
      </div>

      <div className="boat-detail-header">
        {boat.image_url ? (
          <img className="boat-detail-header__image" src={boat.image_url} alt={boat.name} />
        ) : (
          <div className="boat-detail-header__image" />
        )}
        <div>
          <h1 className="boat-detail-header__name">{boat.name}</h1>
          <div className="boat-detail-header__source">
            {boat.source_url ? (
              <a href={boat.source_url} target="_blank" rel="noreferrer">
                {boat.source_url.replace(/^https?:\/\//, "")}
              </a>
            ) : (
              "(no source linkage)"
            )}
            {" · last synced "}
            {new Date(boat.last_synced).toLocaleDateString()}
          </div>
          <div className="boat-detail-header__stats">
            <div>
              <div className="boat-stat__label">Upcoming trips</div>
              <div className="boat-stat__value">
                {trips ? trips.filter((t) => new Date(t.start_date) >= new Date()).length : "—"}
              </div>
            </div>
            <div>
              <div className="boat-stat__label">Total trips</div>
              <div className="boat-stat__value">{trips?.length ?? "—"}</div>
            </div>
            <div>
              <div className="boat-stat__label">Source</div>
              <div className="boat-stat__value" style={{ fontSize: "var(--fs-base)" }}>
                {boat.source_name}
              </div>
            </div>
          </div>
        </div>
      </div>

      <nav className="tabs">
        <NavLink
          end
          to={`/admin/fleet/${boat.id}`}
          className={({ isActive }) => "tabs__link" + (isActive ? " is-active" : "")}
        >
          Trips
        </NavLink>
        <NavLink
          to={`/admin/fleet/${boat.id}/inventory`}
          className={({ isActive }) => "tabs__link" + (isActive ? " is-active" : "")}
        >
          Inventory
        </NavLink>
        <NavLink
          to={`/admin/fleet/${boat.id}/notes`}
          className={({ isActive }) => "tabs__link" + (isActive ? " is-active" : "")}
        >
          Notes
        </NavLink>
      </nav>

      <Outlet context={{ boat, trips: trips ?? [] }} />
    </>
  );
}
