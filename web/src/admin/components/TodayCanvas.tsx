import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Overview as OverviewT } from "../api";

import { Card } from "./Card";
import { Empty } from "./Empty";
import { PageHeader } from "./PageHeader";
import { Stat } from "./Stat";
import styles from "./TodayCanvas.module.css";

// TodayCanvas is the spatial landing surface for the canvas
// layout. It reads only the existing Overview API (no backend
// changes) and renders today's operational state — setup %,
// active trip count, and a tile per active trip — laid out on
// a deck-style grid that the rail/spaces layouts don't use.
//
// This is intentionally lighter than a real fleet deck-plan
// (which would need boat geometry / positions the API doesn't
// expose). A future sprint can grow it; for evaluation, the
// spatial idiom is conveyed by the grid + glow.
export function TodayCanvas() {
  const [data, setData] = useState<OverviewT | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    adminApi
      .overview()
      .then((d) => !cancelled && setData(d))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load."));
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) return <Card><div className={styles.error}>{error}</div></Card>;
  if (!data) return <Card><div className={styles.muted}>Loading…</div></Card>;

  const flagged = data.trips_needing_attention ?? [];

  return (
    <div className={styles.canvas}>
      <PageHeader
        title="Today"
        subtitle="Setup, exceptions, and what needs your attention."
      />
      <div className={styles.statRow}>
        <Card>
          <Stat label="Setup" value={`${data.setup?.pct ?? 0}%`} hint="Onboarding completeness" />
        </Card>
        <Card>
          <Stat label="Boats" value={data.counts?.boats ?? 0} />
        </Card>
        <Card>
          <Stat label="Trips" value={data.counts?.trips ?? 0} />
        </Card>
        <Card>
          <Stat label="Cruise directors" value={data.counts?.cruise_directors ?? 0} />
        </Card>
      </div>
      <h2 className={styles.heading}>Live deck</h2>
      {flagged.length === 0 ? (
        <Card>
          <Empty
            title="Smooth seas — no trips need your attention"
            hint="Flagged trips will surface here as glowing tiles you can tap into."
          />
        </Card>
      ) : (
        <div className={styles.deck}>
          {flagged.map((t) => (
            <Link key={t.id} to={`/admin/trips/${t.id}/dashboard`} className={styles.tile}>
              <div className={styles.tileBoat}>{t.boat_name}</div>
              <div className={styles.tileTrip}>{t.itinerary}</div>
              <div className={styles.tileMeta}>
                {t.start_date} → {t.end_date}
              </div>
              <div className={styles.tileReason}>{t.reason}</div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
