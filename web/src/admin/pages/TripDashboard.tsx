import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type TripDashboardResponse } from "../api";
import {
  Button,
  Card,
  Chip,
  type ChipVariant,
  type Column,
  DataTable,
  Empty,
  PageHeader,
  Stat,
} from "../components";

import styles from "./TripDashboard.module.css";

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

  if (error) return <div className={styles.error}>{error}</div>;
  if (!data) return <div className={styles.muted}>Loading…</div>;

  const t = data.trip;

  return (
    <>
      <div className={styles.breadcrumb}>
        <Link to="/admin/trips">Trips</Link>
        {" / "}
        <Link to={`/admin/trips/${id}/manifest`}>{t.boat_name}</Link>
      </div>
      <PageHeader
        title="Dashboard"
        subtitle={`${t.boat_name} — ${t.start_date} to ${t.end_date} — ${t.itinerary}`}
        actions={
          <>
            <Chip variant={statusVariant(t.status)}>{t.status}</Chip>
            <Link to={`/admin/trips/${id}/manifest`}>
              <Button variant="secondary">Manifest</Button>
            </Link>
            {t.status === "active" && (
              <Link to={`/admin/trips/${id}/ledger`}>
                <Button variant="primary">Ledger</Button>
              </Link>
            )}
          </>
        }
      />

      <div className={styles.grid}>
        <OccupancyCard data={data} />
        <RegistrationCard data={data} />
        <DocumentsCard data={data} />
        <FolioCard data={data} />
      </div>

      <h2 className={styles.sectionHeading}>Top consumed items</h2>
      <TopItemsTable data={data} />

      <h2 className={styles.sectionHeading}>Low stock on this boat</h2>
      <p className={styles.muted}>
        Items at or below the configured reorder level, plus any item with zero
        or negative stock on hand.
      </p>
      <LowStockTable data={data} />
    </>
  );
}

function statusVariant(status: string): ChipVariant {
  switch (status) {
    case "active":
      return "success";
    case "completed":
      return "info";
    case "cancelled":
      return "error";
    default:
      return "neutral";
  }
}

function OccupancyCard({ data }: { data: TripDashboardResponse }) {
  const o = data.occupancy;
  const pct =
    o.berths_total > 0
      ? Math.round((o.cabin_assignments / o.berths_total) * 100)
      : null;
  return (
    <Card title="Occupancy">
      <div className={styles.counts}>
        <Stat label="Guests" value={o.guest_count} />
        <Stat label="Cabin assigned" value={o.cabin_assignments} />
        <Stat label="Berths total" value={o.berths_total} />
      </div>
      {pct !== null && (
        <div className={styles.cardNote}>{pct}% of berths assigned</div>
      )}
      {o.num_guests != null && (
        <div className={styles.muted}>Expected count: {o.num_guests}</div>
      )}
    </Card>
  );
}

function RegistrationCard({ data }: { data: TripDashboardResponse }) {
  const r = data.registration_ready;
  return (
    <Card title="Registration readiness">
      <div className={styles.counts}>
        <Stat label="Submitted" value={r.submitted_count} />
        <Stat label="Pending" value={r.pending_count} />
        <Stat label="Guests" value={r.guest_count} />
      </div>
    </Card>
  );
}

function DocumentsCard({ data }: { data: TripDashboardResponse }) {
  const d = data.document_ready;
  return (
    <Card title="Document readiness">
      <div className={styles.counts}>
        <Stat label="Uploaded" value={d.uploaded_count} />
        <Stat label="Guests w/ docs" value={d.guests_with_docs_count} />
        <Stat label="Guests" value={d.guest_count} />
      </div>
    </Card>
  );
}

function FolioCard({ data }: { data: TripDashboardResponse }) {
  const f = data.folio_totals;
  const rows: { label: string; value: string }[] = [
    { label: "Charges", value: usd(f.charges_usd_cents) },
    { label: "Settled", value: usd(f.settled_usd_cents) },
    { label: "Outstanding", value: usd(f.outstanding_usd_cents) },
    { label: "Crew tips", value: usd(f.crew_tip_usd_cents) },
    { label: "Card fees", value: usd(f.card_fee_usd_cents) },
  ];
  if (f.voided_line_count > 0) {
    rows.push({
      label: "Voided",
      value: `${f.voided_line_count} · ${usd(f.voided_usd_cents)}`,
    });
  }
  return (
    <Card title="Folios">
      <div className={styles.counts}>
        <Stat label="Open" value={f.open_count} />
        <Stat label="Closed" value={f.closed_count} />
      </div>
      <table className={styles.miniTable}>
        <tbody>
          {rows.map((r) => (
            <tr key={r.label}>
              <td>{r.label}</td>
              <td className={styles.num}>{r.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}

function TopItemsTable({ data }: { data: TripDashboardResponse }) {
  const columns: Column<(typeof data.top_items)[number]>[] = [
    { key: "item", header: "Item", cell: (t) => t.item_name },
    {
      key: "quantity",
      header: "Quantity",
      align: "right",
      tabular: true,
      cell: (t) => t.quantity,
    },
    {
      key: "usd",
      header: "USD",
      align: "right",
      tabular: true,
      cell: (t) => usd(t.usd_cents),
    },
  ];
  return (
    <DataTable
      columns={columns}
      rows={data.top_items}
      rowKey={(t) => t.catalog_item_id}
      empty={<Empty title="No catalog lines on this trip yet." />}
    />
  );
}

function LowStockTable({ data }: { data: TripDashboardResponse }) {
  const columns: Column<(typeof data.low_stock)[number]>[] = [
    { key: "category", header: "Category", cell: (r) => r.category_name },
    { key: "item", header: "Item", cell: (r) => r.item_name },
    {
      key: "onhand",
      header: "On hand",
      align: "right",
      tabular: true,
      cell: (r) => (
        <span className={r.quantity_on_hand <= 0 ? styles.low : undefined}>
          {r.quantity_on_hand}
        </span>
      ),
    },
    {
      key: "reorder",
      header: "Reorder",
      align: "right",
      tabular: true,
      cell: (r) => r.reorder_level ?? "—",
    },
    {
      key: "par",
      header: "Par",
      align: "right",
      tabular: true,
      cell: (r) => r.par_level ?? "—",
    },
  ];
  return (
    <DataTable
      columns={columns}
      rows={data.low_stock}
      rowKey={(r) => r.catalog_item_id}
      empty={<Empty title="No low-stock alerts for this boat." />}
    />
  );
}

function usd(cents: number): string {
  const sign = cents < 0 ? "-" : "";
  const n = Math.abs(cents);
  return `${sign}$${(n / 100).toFixed(2)}`;
}
