import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type TripDashboardResponse } from "../api";

export function TripDashboard() {
  const { id = "" } = useParams<{ id: string }>();
  const [data, setData] = useState<TripDashboardResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    adminApi
      .tripDashboard(id)
      .then((d) => !cancelled && setData(d))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load."));
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (error) return <div className="error">{error}</div>;
  if (!data) return <div className="muted">Loading…</div>;

  const t = data.trip;

  return (
    <>
      <div className="admin-breadcrumb">
        <Link to="/admin/trips">Trips</Link>
        {" / "}
        <Link to={`/admin/trips/${id}/manifest`}>{t.boat_name}</Link>
      </div>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Dashboard</h1>
          <div className="admin-page-subtitle">
            {t.boat_name} — {t.start_date} to {t.end_date} — {t.itinerary}
          </div>
        </div>
        <div className="header-actions">
          <span className={`chip chip--${t.status}`}>{t.status}</span>
          <Link className="secondary" to={`/admin/trips/${id}/manifest`}>
            Manifest
          </Link>
          {t.status === "active" && (
            <Link className="primary" to={`/admin/trips/${id}/ledger`}>
              Ledger
            </Link>
          )}
        </div>
      </div>

      <div className="admin-grid">
        <OccupancyCard data={data} />
        <RegistrationCard data={data} />
        <DocumentsCard data={data} />
        <FolioCard data={data} />
      </div>

      <h2 style={{ marginTop: "var(--sp-xl)" }}>Top consumed items</h2>
      <TopItemsTable data={data} />

      <h2 style={{ marginTop: "var(--sp-xl)" }}>Low stock on this boat</h2>
      <p className="muted">
        Items at or below the configured reorder level, plus any item with
        zero or negative stock on hand.
      </p>
      <LowStockTable data={data} />
    </>
  );
}

function OccupancyCard({ data }: { data: TripDashboardResponse }) {
  const o = data.occupancy;
  const pct = o.berths_total > 0 ? Math.round((o.cabin_assignments / o.berths_total) * 100) : null;
  return (
    <div className="admin-card">
      <h2 className="admin-card__title">Occupancy</h2>
      <ul className="counts">
        <li>
          <span className="counts__value">{o.guest_count}</span>
          <span className="counts__label">Guests</span>
        </li>
        <li>
          <span className="counts__value">{o.cabin_assignments}</span>
          <span className="counts__label">Cabin assigned</span>
        </li>
        <li>
          <span className="counts__value">{o.berths_total}</span>
          <span className="counts__label">Berths total</span>
        </li>
      </ul>
      {pct !== null && (
        <div className="muted" style={{ marginTop: "var(--sp-sm)" }}>
          {pct}% of berths assigned
        </div>
      )}
      {o.num_guests != null && (
        <div className="muted">Expected count: {o.num_guests}</div>
      )}
    </div>
  );
}

function RegistrationCard({ data }: { data: TripDashboardResponse }) {
  const r = data.registration_ready;
  return (
    <div className="admin-card">
      <h2 className="admin-card__title">Registration readiness</h2>
      <ul className="counts">
        <li>
          <span className="counts__value">{r.submitted_count}</span>
          <span className="counts__label">Submitted</span>
        </li>
        <li>
          <span className="counts__value">{r.pending_count}</span>
          <span className="counts__label">Pending</span>
        </li>
        <li>
          <span className="counts__value">{r.guest_count}</span>
          <span className="counts__label">Guests</span>
        </li>
      </ul>
    </div>
  );
}

function DocumentsCard({ data }: { data: TripDashboardResponse }) {
  const d = data.document_ready;
  return (
    <div className="admin-card">
      <h2 className="admin-card__title">Document readiness</h2>
      <ul className="counts">
        <li>
          <span className="counts__value">{d.uploaded_count}</span>
          <span className="counts__label">Uploaded</span>
        </li>
        <li>
          <span className="counts__value">{d.guests_with_docs_count}</span>
          <span className="counts__label">Guests w/ docs</span>
        </li>
        <li>
          <span className="counts__value">{d.guest_count}</span>
          <span className="counts__label">Guests</span>
        </li>
      </ul>
    </div>
  );
}

function FolioCard({ data }: { data: TripDashboardResponse }) {
  const f = data.folio_totals;
  return (
    <div className="admin-card">
      <h2 className="admin-card__title">Folios</h2>
      <ul className="counts">
        <li>
          <span className="counts__value">{f.open_count}</span>
          <span className="counts__label">Open</span>
        </li>
        <li>
          <span className="counts__value">{f.closed_count}</span>
          <span className="counts__label">Closed</span>
        </li>
      </ul>
      <table className="admin-table" style={{ marginTop: "var(--sp-sm)" }}>
        <tbody>
          <tr>
            <td>Charges</td>
            <td className="num">{usd(f.charges_usd_cents)}</td>
          </tr>
          <tr>
            <td>Settled</td>
            <td className="num">{usd(f.settled_usd_cents)}</td>
          </tr>
          <tr>
            <td>Outstanding</td>
            <td className="num">{usd(f.outstanding_usd_cents)}</td>
          </tr>
          <tr>
            <td>Crew tips</td>
            <td className="num">{usd(f.crew_tip_usd_cents)}</td>
          </tr>
          <tr>
            <td>Card fees</td>
            <td className="num">{usd(f.card_fee_usd_cents)}</td>
          </tr>
          {f.voided_line_count > 0 && (
            <tr>
              <td>Voided</td>
              <td className="num">
                {f.voided_line_count} · {usd(f.voided_usd_cents)}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

function TopItemsTable({ data }: { data: TripDashboardResponse }) {
  if (data.top_items.length === 0) {
    return <div className="muted">No catalog lines on this trip yet.</div>;
  }
  return (
    <table className="admin-table">
      <thead>
        <tr>
          <th>Item</th>
          <th className="num">Quantity</th>
          <th className="num">USD</th>
        </tr>
      </thead>
      <tbody>
        {data.top_items.map((t) => (
          <tr key={t.catalog_item_id}>
            <td>{t.item_name}</td>
            <td className="num">{t.quantity}</td>
            <td className="num">{usd(t.usd_cents)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function LowStockTable({ data }: { data: TripDashboardResponse }) {
  if (data.low_stock.length === 0) {
    return <div className="muted">No low-stock alerts for this boat.</div>;
  }
  return (
    <table className="admin-table">
      <thead>
        <tr>
          <th>Category</th>
          <th>Item</th>
          <th className="num">On hand</th>
          <th className="num">Reorder</th>
          <th className="num">Par</th>
        </tr>
      </thead>
      <tbody>
        {data.low_stock.map((r) => (
          <tr key={r.catalog_item_id}>
            <td>{r.category_name}</td>
            <td>{r.item_name}</td>
            <td className={"num" + (r.quantity_on_hand <= 0 ? " low" : "")}>
              {r.quantity_on_hand}
            </td>
            <td className="num">{r.reorder_level ?? "—"}</td>
            <td className="num">{r.par_level ?? "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function usd(cents: number): string {
  const sign = cents < 0 ? "-" : "";
  const n = Math.abs(cents);
  return `${sign}$${(n / 100).toFixed(2)}`;
}
