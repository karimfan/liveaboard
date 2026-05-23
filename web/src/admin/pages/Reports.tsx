import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import {
  adminApi,
  type AdminReportsResponse,
  type SettlementCurrencyRow,
} from "../api";

export function Reports() {
  const [data, setData] = useState<AdminReportsResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [from, setFrom] = useState<string>("");
  const [to, setTo] = useState<string>("");
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    adminApi
      .adminReports({ from: from || undefined, to: to || undefined })
      .then((d) => !cancelled && setData(d))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load."));
    return () => {
      cancelled = true;
    };
  }, [refreshKey, from, to]);

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Reports</h1>
          <div className="admin-page-subtitle">
            Setup completeness, operational status, and revenue.
          </div>
        </div>
        <div className="header-actions">
          <label className="muted">
            From{" "}
            <input
              type="date"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
            />
          </label>
          <label className="muted">
            To{" "}
            <input
              type="date"
              value={to}
              onChange={(e) => setTo(e.target.value)}
            />
          </label>
          <button onClick={() => setRefreshKey((k) => k + 1)}>Refresh</button>
        </div>
      </div>

      {error && <div className="error">{error}</div>}
      {!data && !error && <div className="muted">Loading…</div>}

      {data && (
        <>
          <div className="admin-grid">
            <SetupCard data={data} />
            <StatusCard data={data} />
          </div>

          <h2 style={{ marginTop: "var(--sp-xl)" }}>Operational status</h2>
          <OperationalTable data={data} />

          <h2 style={{ marginTop: "var(--sp-xl)" }}>Revenue per trip</h2>
          <p className="muted">
            Headline numbers in USD (canonical price snapshots). Settlement
            currency totals appear per-currency, never summed across mixed
            currencies. Voided lines are excluded from revenue and reported
            as correction metadata.
          </p>
          <RevenueTable data={data} />

          <div className="admin-card" style={{ marginTop: "var(--sp-xl)" }}>
            <h2 className="admin-card__title">Cross-trip analytics</h2>
            <p className="muted">
              Trends across boats and seasons remain deferred (post-MVP). See
              ADR-0003 for the escalation path.
            </p>
          </div>
        </>
      )}
    </>
  );
}

function SetupCard({ data }: { data: AdminReportsResponse }) {
  return (
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
              className={"setup-list__check" + (s.done ? "" : " is-pending")}
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
  );
}

function StatusCard({ data }: { data: AdminReportsResponse }) {
  const c = data.trip_status_counts;
  return (
    <div className="admin-card">
      <h2 className="admin-card__title">Trips by status</h2>
      <ul className="counts">
        <li>
          <span className="counts__value">{c.planned}</span>
          <span className="counts__label">Planned</span>
        </li>
        <li>
          <span className="counts__value">{c.active}</span>
          <span className="counts__label">Active</span>
        </li>
        <li>
          <span className="counts__value">{c.completed}</span>
          <span className="counts__label">Completed</span>
        </li>
        <li>
          <span className="counts__value">{c.cancelled}</span>
          <span className="counts__label">Cancelled</span>
        </li>
      </ul>
      <div className="muted" style={{ marginTop: "var(--sp-sm)" }}>
        Window: {data.window.from} → {data.window.to}
      </div>
    </div>
  );
}

function OperationalTable({ data }: { data: AdminReportsResponse }) {
  const rows = data.trip_operational;
  if (rows.length === 0) {
    return <div className="muted">No trips in this window.</div>;
  }
  return (
    <table className="admin-table">
      <thead>
        <tr>
          <th>Status</th>
          <th>Boat</th>
          <th>Dates</th>
          <th>Guests</th>
          <th>Submitted</th>
          <th>Docs</th>
          <th>Cabins</th>
          <th>Directors</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.trip_id}>
            <td>
              <span className={`chip chip--${r.status}`}>{r.status}</span>
            </td>
            <td>{r.boat_name}</td>
            <td>
              {r.start_date} → {r.end_date}
            </td>
            <td>
              {r.guest_count}
              {r.num_guests != null && ` / ${r.num_guests}`}
            </td>
            <td>{r.submitted_count}</td>
            <td>{r.document_count}</td>
            <td>{r.cabin_assignments}</td>
            <td>{r.director_count}</td>
            <td>
              <Link to={`/admin/trips/${r.trip_id}/dashboard`}>Dashboard</Link>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function RevenueTable({ data }: { data: AdminReportsResponse }) {
  const rows = data.trip_revenue;
  if (rows.length === 0) {
    return <div className="muted">No revenue rows for this window.</div>;
  }
  return (
    <table className="admin-table">
      <thead>
        <tr>
          <th>Boat</th>
          <th>Dates</th>
          <th>Status</th>
          <th>Charges</th>
          <th>Settled</th>
          <th>Outstanding</th>
          <th>Card fees</th>
          <th>Tips</th>
          <th>Voided</th>
          <th>Settlement breakdown</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.trip_id}>
            <td>{r.boat_name}</td>
            <td>
              {r.start_date} → {r.end_date}
            </td>
            <td>
              <span className={`chip chip--${r.status}`}>{r.status}</span>
            </td>
            <td>{usd(r.charges_usd_cents)}</td>
            <td>{usd(r.settled_usd_cents)}</td>
            <td>{usd(r.outstanding_usd_cents)}</td>
            <td>{usd(r.card_fee_usd_cents)}</td>
            <td>{usd(r.crew_tip_usd_cents)}</td>
            <td>
              {r.voided_line_count > 0 ? (
                <span title={`${r.voided_line_count} voided line(s)`}>
                  {r.voided_line_count} · {usd(r.voided_usd_cents)}
                </span>
              ) : (
                <span className="muted">—</span>
              )}
            </td>
            <td>
              <SettlementBreakdown rows={r.settlement_by_currency} />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function SettlementBreakdown({ rows }: { rows: SettlementCurrencyRow[] }) {
  if (rows.length === 0) {
    return <span className="muted">—</span>;
  }
  return (
    <ul className="settlement-list">
      {rows.map((s) => (
        <li key={s.currency}>
          {s.currency}: {formatMinor(s.total_minor, s.currency)}{" "}
          <span className="muted">({s.folio_count})</span>
        </li>
      ))}
    </ul>
  );
}

function usd(cents: number): string {
  const sign = cents < 0 ? "-" : "";
  const n = Math.abs(cents);
  return `${sign}$${(n / 100).toFixed(2)}`;
}

// formatMinor renders a settlement-currency total. For now we assume an
// exponent of 2 (cents-equivalent) for common currencies. The exact
// `currency_exponent` from the ledger is exposed by the guest tab payload
// but the admin breakdown groups across folios; the UI takes the
// pragmatic display path here and notes the count alongside.
function formatMinor(minor: number, ccy: string): string {
  return `${ccy} ${(minor / 100).toFixed(2)}`;
}
