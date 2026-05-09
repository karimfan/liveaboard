import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Overview as OverviewT } from "../api";
import { api, type ApiError, type CruiseDirectorOverview } from "../../lib/api";
import { useMe } from "../useMe";

export function Overview() {
  const me = useMe();

  if (!me.loaded) return null;

  // Org Admin sees the operational triage screen (Sprint 008).
  // Cruise Director sees their personal landing (Sprint 010).
  if (me.me?.role === "org_admin") {
    return <AdminOverview />;
  }
  return <CruiseDirectorLanding />;
}

function AdminOverview() {
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
            Per-boat inventory tracking arrives in a future sprint. Once
            stock levels are recorded, alerts will appear here.
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

function CruiseDirectorLanding() {
  const [data, setData] = useState<CruiseDirectorOverview | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .cruiseDirectorOverview()
      .then((d) => !cancelled && setData(d))
      .catch((e: ApiError) => !cancelled && setError(e?.message ?? "Failed to load."));
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) return <div className="error">{error}</div>;
  if (!data) return <div className="muted">Loading…</div>;

  const { profile, stats, trips } = data;
  const orderedTrips = [...trips].sort((a, b) => orderRank(a.status) - orderRank(b.status));

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">My trips</h1>
          <div className="admin-page-subtitle">
            Your assigned trips and contact details.
          </div>
        </div>
      </div>

      <div className="admin-grid">
        <div className="admin-card">
          <h2 className="admin-card__title">{profile.full_name}</h2>
          <ul className="contact-card">
            <li>{profile.email}</li>
            {profile.phone && <li>{profile.phone}</li>}
            <li className="muted">
              {profile.organization_name} · {profile.role.replace("_", " ")}
            </li>
          </ul>
          <p style={{ marginTop: "var(--sp-md)" }}>
            <Link to="/admin/account">Edit profile</Link>
          </p>
        </div>

        <div className="admin-card">
          <h2 className="admin-card__title">At a glance</h2>
          <ul className="counts">
            <li>
              <span className="counts__value">{stats.planned}</span>
              <span className="counts__label">Planned</span>
            </li>
            <li>
              <span className="counts__value">{stats.active}</span>
              <span className="counts__label">Active</span>
            </li>
            <li>
              <span className="counts__value">{stats.completed}</span>
              <span className="counts__label">Completed</span>
            </li>
          </ul>
        </div>
      </div>

      <h2 style={{ marginTop: "var(--sp-xl)" }}>My trips</h2>
      {orderedTrips.length === 0 ? (
        <div className="empty-state">
          <h3>No assigned trips yet</h3>
          <p className="muted">
            An admin will assign you to a trip when one is ready.
          </p>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Status</th>
              <th>Boat</th>
              <th>Itinerary</th>
              <th>Dates</th>
            </tr>
          </thead>
          <tbody>
            {orderedTrips.map((t) => (
              <tr key={t.id}>
                <td>
                  <span className={`chip chip--${t.status}`}>{t.status}</span>
                </td>
                <td>{t.boat_name}</td>
                <td>{t.itinerary}</td>
                <td>
                  {t.start_date} → {t.end_date}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

function orderRank(status: "planned" | "active" | "completed" | "cancelled"): number {
  switch (status) {
    case "active":
      return 0;
    case "planned":
      return 1;
    case "completed":
      return 2;
    case "cancelled":
      return 3;
  }
}
