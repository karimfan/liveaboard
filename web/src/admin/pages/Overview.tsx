import { Link } from "react-router-dom";

import {
  lowStock,
  recentActivity,
  setupCompletenessPct,
  setupSteps,
  trippsNeedingAttention,
} from "../mock";

export function Overview() {
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
          <div className="setup-pct">{setupCompletenessPct}%</div>
          <ul className="setup-list">
            {setupSteps.map((s) => (
              <li
                key={s.label}
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
          {trippsNeedingAttention.length === 0 ? (
            <div className="alert-row__sub">All planned trips are on track.</div>
          ) : (
            trippsNeedingAttention.map((t) => {
              const reason = t.director === null
                ? "No director assigned"
                : `Manifest ${Math.round((t.manifestFilled / t.manifestCapacity) * 100)}% — below 50%`;
              return (
                <div key={t.id} className="alert-row">
                  <div>
                    <div className="alert-row__title">
                      <Link to={`/admin/trips/${t.id}`}>
                        {t.boatName} · {t.itinerary}
                      </Link>
                    </div>
                    <div className="alert-row__sub">
                      {t.startDate} → {t.endDate} · {reason}
                    </div>
                  </div>
                  <Link to={`/admin/trips/${t.id}`}>
                    <button>Open</button>
                  </Link>
                </div>
              );
            })
          )}
        </div>

        <div className="admin-card">
          <h2 className="admin-card__title">Low-stock alerts</h2>
          {lowStock.length === 0 ? (
            <div className="alert-row__sub">All boats above their stock thresholds.</div>
          ) : (
            lowStock.map((row) => (
              <div key={row.boatId} className="alert-row">
                <div>
                  <div className="alert-row__title">{row.boatName}</div>
                  <div className="alert-row__sub">
                    {row.lowCount} {row.lowCount === 1 ? "item" : "items"} below threshold
                  </div>
                </div>
                <Link to={`/admin/fleet/${row.boatId === "boat_1" ? "gaia-love" : row.boatId === "boat_2" ? "blue-spirit" : "seahorse"}`}>
                  <button className="ghost">Open inventory</button>
                </Link>
              </div>
            ))
          )}
        </div>

        <div className="admin-card">
          <h2 className="admin-card__title">Recent activity</h2>
          {recentActivity.map((entry, i) => (
            <div key={i} className="alert-row">
              <div>
                <div className="alert-row__title">{entry.text}</div>
              </div>
              <div className="alert-row__sub">{entry.ts}</div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
