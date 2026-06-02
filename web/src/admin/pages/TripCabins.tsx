import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type TripCabinBoard, type TripManifest } from "../api";
import {
  Button,
  Card,
  type Column,
  DataTable,
  Empty,
  PageHeader,
} from "../components";

import styles from "./TripCabins.module.css";

export function TripCabins() {
  const { id = "" } = useParams<{ id: string }>();
  const [board, setBoard] = useState<TripCabinBoard | null>(null);
  const [manifest, setManifest] = useState<TripManifest | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      const [b, m] = await Promise.all([
        adminApi.tripCabinBoard(id),
        adminApi.tripManifest(id),
      ]);
      setBoard(b);
      setManifest(m);
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to load cabin board.",
      );
    }
  }

  useEffect(() => {
    if (id) void load();
  }, [id]);

  const unassigned = useMemo(() => board?.unassigned_guests ?? [], [board]);

  async function assign(guestId: string, berthId: string) {
    if (!guestId) return;
    setError(null);
    setMessage(null);
    try {
      await adminApi.assignGuestCabin(id, guestId, { berth_id: berthId });
      setMessage("Cabin assignment updated.");
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Could not assign berth.",
      );
    }
  }

  async function unassign(guestId: string) {
    setError(null);
    setMessage(null);
    try {
      await adminApi.unassignGuestCabin(id, guestId);
      setMessage("Guest unassigned.");
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Could not unassign guest.",
      );
    }
  }

  if (!board || !manifest) {
    return (
      <>
        <div className={styles.breadcrumb}>
          <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
        </div>
        {error ? (
          <div className={styles.error}>{error}</div>
        ) : (
          <div className={styles.muted}>Loading...</div>
        )}
      </>
    );
  }

  const unassignedColumns: Column<(typeof unassigned)[number]>[] = [
    { key: "name", header: "Guest", cell: (g) => g.full_name },
    { key: "email", header: "Email", cell: (g) => g.email },
  ];

  return (
    <>
      <div className={styles.breadcrumb}>
        <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
      </div>
      <PageHeader
        title="Cabin board"
        subtitle={`${manifest.trip.boat_name} - ${manifest.trip.start_date} to ${manifest.trip.end_date}`}
      />
      {error && <div className={styles.error}>{error}</div>}
      {message && <div className={styles.callout}>{message}</div>}

      {board.cabins.length === 0 ? (
        <Empty
          title="No cabin layout"
          hint="This boat needs a cabin layout before guests can be assigned."
        />
      ) : (
        <div className={styles.board}>
          {board.cabins.map((c) => (
            <Card key={c.id} title={`Cabin ${c.label}`}>
              {c.deck && <p className={styles.deck}>{c.deck}</p>}
              <div className={styles.berths}>
                {c.berths.map((b) => (
                  <div className={styles.berthRow} key={b.id}>
                    <div>
                      <strong>{b.display_label}</strong>
                      {b.guest ? (
                        <div>
                          {b.guest.full_name}
                          <br />
                          <span className={styles.guestEmail}>
                            {b.guest.email}
                          </span>
                        </div>
                      ) : (
                        <span className={styles.muted}>Available</span>
                      )}
                    </div>
                    <div className={styles.berthActions}>
                      {b.guest ? (
                        <Button
                          variant="quiet"
                          onClick={() => unassign(b.guest!.id)}
                        >
                          Unassign
                        </Button>
                      ) : (
                        <select
                          defaultValue=""
                          onChange={(e) => assign(e.target.value, b.id)}
                        >
                          <option value="">Assign guest...</option>
                          {unassigned.map((g) => (
                            <option key={g.id} value={g.id}>
                              {g.full_name}
                            </option>
                          ))}
                        </select>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </Card>
          ))}
        </div>
      )}

      {unassigned.length > 0 && (
        <Card title="Needs cabin">
          <DataTable
            columns={unassignedColumns}
            rows={unassigned}
            rowKey={(g) => g.id}
          />
        </Card>
      )}
    </>
  );
}
