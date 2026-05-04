import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Overview as OverviewT } from "../api";
import { useMe } from "../useMe";

export function Overview() {
  const me = useMe();
  const [data, setData] = useState<OverviewT | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Site Directors don't have access to the admin /overview endpoint.
  // Their landing page renders a smaller variant.
  const isAdmin = me.loaded && me.me?.role === "org_admin";

  useEffect(() => {
    if (!isAdmin) return;
    let cancelled = false;
    adminApi
      .overview()
      .then((d) => !cancelled && setData(d))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load."));
    return () => {
      cancelled = true;
    };
  }, [isAdmin]);

  if (!me.loaded) return null;

  if (!isAdmin) {
    return <SiteDirectorOverview />;
  }

  if (error) return <div className="error">{error}</div>;
  if (!data) return <div className="muted">Loading…</div>;

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Overview</h1>
          <div className="admin-page-subtitle">
            Setup, exceptions, and what needs your attention.
          </div>
        </div>
      </div>

      <div className="admin-grid">
        <div className="admin-card">
          <h2 className="admin-card__title">Setup completeness</h2>
          <div className="setup-pct">{data.setup.pct}%</div>
          <ul className="setup-list">
            {data.setup.steps.map((s) => (
              <li
                key={s.key}
                className={"setup-list__item" + (s.done ? "" : " is-pending")}
              >
                <span
                  className={
                    "setup-list__check" + (s.done ? "" : " is-pending")
                  }
                  aria-hidden
                >
                  {s.done ? "✓" : "·"}
                </span>
                <span className="setup-list__label">
                  {s.href ? <Link to={s.href}>{s.label}</Link> : s.label}
                </span>
                <span className="setup-list__hint">{s.hint}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="admin-card">
          <h2 className="admin-card__title">Trips needing attention</h2>
          {data.trips_needing_attention.length === 0 ? (
            <div className="alert-row__sub">
              All planned trips in the next 90 days have a director assigned.
            </div>
          ) : (
            data.trips_needing_attention.map((t) => (
              <div key={t.id} className="alert-row">
                <div>
                  <div className="alert-row__title">
                    {t.boat_name} · {t.itinerary}
                  </div>
                  <div className="alert-row__sub">
                    {t.start_date} → {t.end_date} · {t.reason}
                  </div>
                </div>
                <Link to="/admin/trips">
                  <button>Open</button>
                </Link>
              </div>
            ))
          )}
        </div>

        <div className="admin-card">
          <h2 className="admin-card__title">Low-stock alerts</h2>
          <div className="alert-row__sub">
            Per-boat inventory tracking arrives in Sprint 009. Once stock
            levels are recorded, alerts will appear here.
          </div>
        </div>

        <div className="admin-card">
          <h2 className="admin-card__title">Recent activity</h2>
          <div className="alert-row__sub">
            An activity log will land alongside reporting (US-7.x).
          </div>
        </div>
      </div>
    </>
  );
}

function SiteDirectorOverview() {
  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">My trips</h1>
          <div className="admin-page-subtitle">
            Your assigned trips, current and upcoming.
          </div>
        </div>
      </div>
      <div className="admin-card">
        <p className="muted">
          Site Director views are coming together. Open <Link to="/admin/trips">Trips</Link> for
          the trips you're assigned to.
        </p>
      </div>
    </>
  );
}
