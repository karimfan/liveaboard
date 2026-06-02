import { useCallback, useEffect, useState } from "react";
import { Link, NavLink, Outlet, useParams } from "react-router-dom";

import { adminApi, type Boat, type Trip } from "../api";
import { Empty, Stat } from "../components";

import styles from "./BoatDetail.module.css";

export function BoatDetail() {
  const { id } = useParams();
  const [boat, setBoat] = useState<Boat | null>(null);
  const [trips, setTrips] = useState<Trip[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadBoatData = useCallback(async () => {
    if (!id) return;
    const [b, t] = await Promise.all([
      adminApi.getBoat(id),
      adminApi.listBoatTrips(id),
    ]);
    return { boat: b, trips: t.trips ?? [] };
  }, [id]);

  const refreshBoat = useCallback(async () => {
    const data = await loadBoatData();
    if (!data) return;
    setError(null);
    const { boat: b, trips: nextTrips } = data;
    setBoat(b);
    setTrips(nextTrips);
  }, [loadBoatData]);

  useEffect(() => {
    let cancelled = false;
    loadBoatData()
      .then((data) => {
        if (cancelled || !data) return;
        setBoat(data.boat);
        setTrips(data.trips);
      })
      .catch(
        (e) => !cancelled && setError(e?.message ?? "Failed to load boat."),
      );
    return () => {
      cancelled = true;
    };
  }, [loadBoatData]);

  if (error) {
    return (
      <Empty
        title="Boat not found"
        hint={<Link to="/admin/fleet">Back to fleet</Link>}
      />
    );
  }
  if (!boat) return <div className={styles.loading}>Loading…</div>;

  const tabClass = ({ isActive }: { isActive: boolean }) =>
    isActive ? `${styles.tab} ${styles.tabActive}` : styles.tab;

  return (
    <>
      <div className={styles.breadcrumb}>
        <Link to="/admin/fleet">Fleet</Link> · {boat.name}
      </div>

      <div className={styles.header}>
        {boat.image_url ? (
          <img className={styles.image} src={boat.image_url} alt={boat.name} />
        ) : (
          <div className={styles.image} />
        )}
        <div>
          <h1 className={styles.name}>{boat.name}</h1>
          <div className={styles.source}>
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
          <div className={styles.stats}>
            <Stat
              label="Upcoming trips"
              value={
                trips
                  ? trips.filter((t) => new Date(t.start_date) >= new Date())
                      .length
                  : "—"
              }
            />
            <Stat label="Total trips" value={trips?.length ?? "—"} />
            <Stat label="Source" value={boat.source_name} tabular={false} />
          </div>
        </div>
      </div>

      <nav className={styles.tabs}>
        <NavLink end to={`/admin/fleet/${boat.id}`} className={tabClass}>
          Trips
        </NavLink>
        <NavLink to={`/admin/fleet/${boat.id}/cabins`} className={tabClass}>
          Cabins
        </NavLink>
        <NavLink to={`/admin/fleet/${boat.id}/inventory`} className={tabClass}>
          Inventory
        </NavLink>
        <NavLink to={`/admin/fleet/${boat.id}/notes`} className={tabClass}>
          Notes
        </NavLink>
      </nav>

      <Outlet context={{ boat, trips: trips ?? [], refreshBoat }} />
    </>
  );
}
