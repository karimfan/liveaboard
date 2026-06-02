import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Boat } from "../api";
import {
  Button,
  Chip,
  type Column,
  DataTable,
  Empty,
  PageHeader,
} from "../components";

import styles from "./Fleet.module.css";

export function Fleet() {
  const [boats, setBoats] = useState<Boat[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    adminApi
      .listBoats()
      .then((res) => !cancelled && setBoats(res.boats ?? []))
      .catch(
        (e) => !cancelled && setError(e?.message ?? "Failed to load fleet."),
      );
    return () => {
      cancelled = true;
    };
  }, []);

  const columns: Column<Boat>[] = [
    {
      key: "name",
      header: "Name",
      cell: (b) => <Link to={`/admin/fleet/${b.id}`}>{b.name}</Link>,
    },
    {
      key: "source",
      header: "Source",
      cell: (b) =>
        b.source_url ? (
          <a href={b.source_url} target="_blank" rel="noreferrer">
            {b.source_url.replace(/^https?:\/\//, "")}
          </a>
        ) : (
          <span className={styles.muted}>—</span>
        ),
    },
    {
      key: "synced",
      header: "Last synced",
      cell: (b) => relativeTime(b.last_synced),
    },
    {
      key: "status",
      header: "Status",
      cell: () => <Chip variant="success">Active</Chip>,
    },
  ];

  return (
    <>
      <PageHeader
        title="Fleet"
        subtitle="All boats in your organization."
        actions={
          <Link to="/admin/import">
            <Button variant="primary">+ Import boat</Button>
          </Link>
        }
      />

      <div className={styles.filterBar}>
        <select defaultValue="active">
          <option value="active">Active</option>
          <option value="archived">Archived</option>
          <option value="all">All</option>
        </select>
        <input type="search" placeholder="Search boats..." />
        <div className={styles.spacer} />
      </div>

      {error && <div className={styles.error}>{error}</div>}

      {!boats ? (
        <div className={styles.loading}>Loading…</div>
      ) : boats.length === 0 ? (
        <Empty
          title="No boats yet"
          hint={
            <>
              <Link to="/admin/import">Import a boat</Link> from liveaboard.com
              or upload a spreadsheet to seed your fleet.
            </>
          }
        />
      ) : (
        <DataTable columns={columns} rows={boats} rowKey={(b) => b.id} />
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
