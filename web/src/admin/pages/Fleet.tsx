import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Boat } from "../api";

export function Fleet() {
  const [boats, setBoats] = useState<Boat[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    adminApi
      .listBoats()
      .then((res) => !cancelled && setBoats(res.boats ?? []))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load fleet."));
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Fleet</h1>
          <div className="admin-page-subtitle">
            All boats in your organization.
          </div>
        </div>
        <button className="primary" disabled title="Coming next sprint">
          + Add boat
        </button>
      </div>

      <div className="filter-bar">
        <select defaultValue="active">
          <option value="active">Active</option>
          <option value="archived">Archived</option>
          <option value="all">All</option>
        </select>
        <input type="search" placeholder="Search boats..." />
        <div className="filter-bar__spacer" />
      </div>

      {error && <div className="error">{error}</div>}

      {!boats ? (
        <div className="muted">Loading…</div>
      ) : boats.length === 0 ? (
        <div className="empty-state">
          <h3>No boats yet</h3>
          <p>
            Use <code>make scrape-boat</code> to import one from
            liveaboard.com, or click "+ Add boat" once that flow lands.
          </p>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Source</th>
              <th>Last synced</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {boats.map((b) => (
              <tr key={b.id}>
                <td>
                  <Link to={`/admin/fleet/${b.id}`}>{b.name}</Link>
                </td>
                <td>
                  {b.source_url ? (
                    <a href={b.source_url} target="_blank" rel="noreferrer">
                      {b.source_url.replace(/^https?:\/\//, "")}
                    </a>
                  ) : (
                    <span className="muted">—</span>
                  )}
                </td>
                <td>{relativeTime(b.last_synced)}</td>
                <td>
                  <span className="chip chip--active">Active</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

function relativeTime(iso: string): string {
  const t = new Date(iso);
  const diff = Date.now() - t.getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return "just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}d ago`;
  return t.toISOString().slice(0, 10);
}
