import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type Trip } from "../api";
import { useMe } from "../useMe";
import { AssignDirector, useCruiseDirectors } from "../AssignDirector";
import {
  Button,
  Chip,
  type ChipVariant,
  type Column,
  DataTable,
  Empty,
  PageHeader,
} from "../components";

import styles from "./Trips.module.css";

export function Trips() {
  const me = useMe();
  const [trips, setTrips] = useState<Trip[] | null>(null);
  const [scope, setScope] = useState<"all" | "assigned_to_me">("all");
  const [statusFilter, setStatusFilter] = useState("operational");
  const [error, setError] = useState<string | null>(null);

  const isAdmin = me.loaded && me.me?.role === "org_admin";
  const { directors } = useCruiseDirectors(isAdmin);

  useEffect(() => {
    let cancelled = false;
    adminApi
      .listTrips()
      .then((res) => {
        if (cancelled) return;
        setTrips(res.trips ?? []);
        setScope(res.scope);
      })
      .catch(
        (e) => !cancelled && setError(e?.message ?? "Failed to load trips."),
      );
    return () => {
      cancelled = true;
    };
  }, []);

  // After a successful add/remove, patch the row in place with the
  // server-returned director list so the chips re-render.
  function onChanged(tripId: string, ids: string[], names: string[]) {
    setTrips((prev) =>
      prev
        ? prev.map((t) =>
            t.id === tripId
              ? {
                  ...t,
                  cruise_director_user_ids: ids,
                  cruise_director_names: names,
                }
              : t,
          )
        : prev,
    );
  }

  const subtitle =
    scope === "assigned_to_me"
      ? "Trips assigned to you, in chronological order."
      : "All upcoming trips across the fleet, in chronological order.";

  const columns: Column<Trip>[] = [
    {
      key: "dates",
      header: "Dates",
      width: "150px",
      cell: (t) => (
        <span className={styles.tabular}>
          {t.start_date} → {t.end_date}
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      cell: (t) => {
        const label = t.removed_from_source_at ? "removed" : t.status;
        return <Chip variant={statusVariant(label)}>{label}</Chip>;
      },
    },
    {
      key: "boat",
      header: "Boat",
      cell: (t) => <Link to={`/admin/fleet/${t.boat_id}`}>{t.boat_name}</Link>,
    },
    { key: "itinerary", header: "Itinerary", cell: (t) => t.itinerary },
    {
      key: "director",
      header: "Director",
      cell: (t) => (
        <AssignDirector
          trip={t}
          directors={directors}
          canEdit={isAdmin}
          onChanged={onChanged}
        />
      ),
    },
    {
      key: "guests",
      header: "Guests",
      cell: (t) => <ManifestSummary trip={t} />,
    },
    {
      key: "price",
      header: "Price",
      align: "right",
      tabular: true,
      cell: (t) => t.price_text ?? "—",
    },
    {
      key: "availability",
      header: "Availability",
      cell: (t) =>
        t.availability_text ? (
          <Chip
            variant={
              t.availability_text.toUpperCase().includes("FULL")
                ? "error"
                : "success"
            }
          >
            {t.availability_text}
          </Chip>
        ) : (
          <span>—</span>
        ),
    },
    {
      key: "actions",
      header: "",
      cell: (t) => <Link to={`/admin/trips/${t.id}/manifest`}>Manifest</Link>,
    },
  ];

  return (
    <>
      <PageHeader
        title="Trips"
        subtitle={subtitle}
        actions={
          isAdmin && (
            <Link to="/admin/import">
              <Button variant="primary">+ Import trips</Button>
            </Link>
          )
        }
      />

      {error && <div className={styles.error}>{error}</div>}
      {!trips ? (
        <div className={styles.loading}>Loading…</div>
      ) : trips.length === 0 ? (
        <Empty
          title="No trips yet"
          hint={
            isAdmin ? (
              <>
                <Link to="/admin/import">Import trips</Link> from liveaboard.com
                or upload a spreadsheet.
              </>
            ) : (
              "You haven't been assigned to any trips."
            )
          }
        />
      ) : (
        <>
          <div className={styles.toolbar}>
            <label>
              Status
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
              >
                <option value="operational">Operational</option>
                <option value="planned">Planned</option>
                <option value="active">Active</option>
                <option value="completed">Completed</option>
                <option value="cancelled">Cancelled</option>
                <option value="removed">Removed from source</option>
                <option value="all">All</option>
              </select>
            </label>
          </div>
          <DataTable
            columns={columns}
            rows={filterTrips(trips, statusFilter)}
            rowKey={(t) => t.id}
          />
        </>
      )}
    </>
  );
}

// Map a trip status/availability label to a shared Chip variant.
function statusVariant(label: string): ChipVariant {
  switch (label) {
    case "active":
      return "success";
    case "completed":
      return "info";
    case "cancelled":
    case "removed":
      return "error";
    default:
      return "neutral";
  }
}

function filterTrips(trips: Trip[], filter: string): Trip[] {
  if (filter === "all") return trips;
  if (filter === "removed")
    return trips.filter((t) => t.removed_from_source_at);
  if (filter === "operational")
    return trips.filter(
      (t) => !t.removed_from_source_at && t.status !== "cancelled",
    );
  return trips.filter((t) => !t.removed_from_source_at && t.status === filter);
}

function ManifestSummary({ trip }: { trip: Trip }) {
  const summary = trip.manifest_summary;
  if (!summary) return <span>0 guests</span>;
  return (
    <div className={styles.manifestSummary}>
      <span>{summary.guest_count} guests</span>
      {summary.submitted_count > 0 && (
        <span>{summary.submitted_count} submitted</span>
      )}
      {summary.expected_count != null && (
        <span>{summary.expected_count} expected</span>
      )}
      {summary.has_warning && (
        <span className={styles.manifestWarning}>over expected</span>
      )}
    </div>
  );
}
