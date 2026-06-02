import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import {
  adminApi,
  type AdminReportsResponse,
  type SettlementCurrencyRow,
} from "../api";
import {
  Button,
  Card,
  Chip,
  type ChipVariant,
  type Column,
  DataTable,
  PageHeader,
  Section,
} from "../components";

import styles from "./Reports.module.css";

type OperationalRow = AdminReportsResponse["trip_operational"][number];
type RevenueRow = AdminReportsResponse["trip_revenue"][number];

function statusVariant(status: string): ChipVariant {
  switch (status) {
    case "active":
      return "success";
    case "cancelled":
      return "error";
    case "completed":
      return "info";
    default:
      return "neutral";
  }
}

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
      <PageHeader
        title="Reports"
        subtitle="Setup completeness, operational status, and revenue."
        actions={
          <div className={styles.headerActions}>
            <label>
              From{" "}
              <input
                type="date"
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
            </label>
            <label>
              To{" "}
              <input
                type="date"
                value={to}
                onChange={(e) => setTo(e.target.value)}
              />
            </label>
            <Button
              variant="secondary"
              onClick={() => setRefreshKey((k) => k + 1)}
            >
              Refresh
            </Button>
          </div>
        }
      />

      {error && <div className={styles.error}>{error}</div>}
      {!data && !error && <div className={styles.loading}>Loading…</div>}

      {data && (
        <>
          <div className={styles.grid}>
            <SetupCard data={data} />
            <StatusCard data={data} />
          </div>

          <Section title="Operational status">
            <OperationalTable data={data} />
          </Section>

          <Section
            title="Revenue per trip"
            hint="Headline numbers in USD (canonical price snapshots). Settlement currency totals appear per-currency, never summed across mixed currencies. Voided lines are excluded from revenue and reported as correction metadata."
          >
            <RevenueTable data={data} />
          </Section>

          <Card title="Cross-trip analytics">
            <p className={styles.muted}>
              Trends across boats and seasons remain deferred (post-MVP). See
              ADR-0003 for the escalation path.
            </p>
          </Card>
        </>
      )}
    </>
  );
}

function SetupCard({ data }: { data: AdminReportsResponse }) {
  return (
    <Card title="Setup completeness">
      <div className={styles.setupPct}>{data.setup.pct}%</div>
      <ul className={styles.setupList}>
        {data.setup.steps.map((s) => (
          <li key={s.key} className={styles.setupItem}>
            <span
              className={
                s.done
                  ? styles.setupCheck
                  : `${styles.setupCheck} ${styles.setupCheckPending}`
              }
              aria-hidden
            >
              {s.done ? "✓" : "·"}
            </span>
            <span>{s.href ? <Link to={s.href}>{s.label}</Link> : s.label}</span>
            <span className={styles.setupHint}>{s.hint}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

function StatusCard({ data }: { data: AdminReportsResponse }) {
  const c = data.trip_status_counts;
  return (
    <Card title="Trips by status">
      <ul className={styles.counts}>
        <li>
          <span className={styles.countValue}>{c.planned}</span>
          <span className={styles.countLabel}>Planned</span>
        </li>
        <li>
          <span className={styles.countValue}>{c.active}</span>
          <span className={styles.countLabel}>Active</span>
        </li>
        <li>
          <span className={styles.countValue}>{c.completed}</span>
          <span className={styles.countLabel}>Completed</span>
        </li>
        <li>
          <span className={styles.countValue}>{c.cancelled}</span>
          <span className={styles.countLabel}>Cancelled</span>
        </li>
      </ul>
      <div className={styles.window}>
        Window: {data.window.from} → {data.window.to}
      </div>
    </Card>
  );
}

function OperationalTable({ data }: { data: AdminReportsResponse }) {
  const columns: Column<OperationalRow>[] = [
    {
      key: "status",
      header: "Status",
      cell: (r) => <Chip variant={statusVariant(r.status)}>{r.status}</Chip>,
    },
    { key: "boat", header: "Boat", cell: (r) => r.boat_name },
    {
      key: "dates",
      header: "Dates",
      cell: (r) => (
        <>
          {r.start_date} → {r.end_date}
        </>
      ),
    },
    {
      key: "guests",
      header: "Guests",
      cell: (r) => (
        <>
          {r.guest_count}
          {r.num_guests != null && ` / ${r.num_guests}`}
        </>
      ),
    },
    { key: "submitted", header: "Submitted", cell: (r) => r.submitted_count },
    { key: "docs", header: "Docs", cell: (r) => r.document_count },
    { key: "cabins", header: "Cabins", cell: (r) => r.cabin_assignments },
    { key: "directors", header: "Directors", cell: (r) => r.director_count },
    {
      key: "actions",
      header: "",
      cell: (r) => (
        <Link to={`/admin/trips/${r.trip_id}/dashboard`}>Dashboard</Link>
      ),
    },
  ];
  return (
    <DataTable
      columns={columns}
      rows={data.trip_operational}
      rowKey={(r) => r.trip_id}
      empty={<span className={styles.muted}>No trips in this window.</span>}
    />
  );
}

function RevenueTable({ data }: { data: AdminReportsResponse }) {
  const columns: Column<RevenueRow>[] = [
    { key: "boat", header: "Boat", cell: (r) => r.boat_name },
    {
      key: "dates",
      header: "Dates",
      cell: (r) => (
        <>
          {r.start_date} → {r.end_date}
        </>
      ),
    },
    {
      key: "status",
      header: "Status",
      cell: (r) => <Chip variant={statusVariant(r.status)}>{r.status}</Chip>,
    },
    {
      key: "charges",
      header: "Charges",
      tabular: true,
      cell: (r) => usd(r.charges_usd_cents),
    },
    {
      key: "settled",
      header: "Settled",
      tabular: true,
      cell: (r) => usd(r.settled_usd_cents),
    },
    {
      key: "outstanding",
      header: "Outstanding",
      tabular: true,
      cell: (r) => usd(r.outstanding_usd_cents),
    },
    {
      key: "cardFees",
      header: "Card fees",
      tabular: true,
      cell: (r) => usd(r.card_fee_usd_cents),
    },
    {
      key: "tips",
      header: "Tips",
      tabular: true,
      cell: (r) => usd(r.crew_tip_usd_cents),
    },
    {
      key: "voided",
      header: "Voided",
      cell: (r) =>
        r.voided_line_count > 0 ? (
          <span title={`${r.voided_line_count} voided line(s)`}>
            {r.voided_line_count} · {usd(r.voided_usd_cents)}
          </span>
        ) : (
          <span className={styles.muted}>—</span>
        ),
    },
    {
      key: "settlement",
      header: "Settlement breakdown",
      cell: (r) => <SettlementBreakdown rows={r.settlement_by_currency} />,
    },
  ];
  return (
    <DataTable
      columns={columns}
      rows={data.trip_revenue}
      rowKey={(r) => r.trip_id}
      empty={
        <span className={styles.muted}>No revenue rows for this window.</span>
      }
    />
  );
}

function SettlementBreakdown({ rows }: { rows: SettlementCurrencyRow[] }) {
  if (rows.length === 0) {
    return <span className={styles.muted}>—</span>;
  }
  return (
    <ul className={styles.settlementList}>
      {rows.map((s) => (
        <li key={s.currency}>
          {s.currency}: {formatMinor(s.total_minor, s.currency)}{" "}
          <span className={styles.muted}>({s.folio_count})</span>
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
