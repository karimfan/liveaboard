import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useOutletContext } from "react-router-dom";

import {
  adminApi,
  type Boat,
  type BoatInventoryItem,
  type CatalogItem,
  type Trip,
} from "../api";
import { useMe } from "../useMe";
import { AssignDirector, useCruiseDirectors } from "../AssignDirector";
import {
  Button,
  Chip,
  type ChipVariant,
  type Column,
  DataTable,
  Empty,
} from "../components";

import styles from "./BoatTabs.module.css";

type Ctx = { boat: Boat; trips: Trip[]; refreshBoat: () => Promise<void> };

function useBoatCtx(): Ctx {
  return useOutletContext<Ctx>();
}

export function BoatTrips() {
  const me = useMe();
  const ctx = useBoatCtx();

  // Local copy so director assignments patch in place without a
  // refetch of the boat detail page.
  const [trips, setTrips] = useState<Trip[]>(ctx.trips);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshMessage, setRefreshMessage] = useState<string | null>(null);
  const [refreshError, setRefreshError] = useState<string | null>(null);

  const isAdmin = me.loaded && me.me?.role === "org_admin";
  const { directors } = useCruiseDirectors(isAdmin);

  useEffect(() => {
    setTrips(ctx.trips);
  }, [ctx.trips]);

  function onChanged(tripId: string, ids: string[], names: string[]) {
    setTrips((prev) =>
      prev.map((t) =>
        t.id === tripId
          ? {
              ...t,
              cruise_director_user_ids: ids,
              cruise_director_names: names,
            }
          : t,
      ),
    );
  }

  async function refreshFromLiveaboard() {
    if (!ctx.boat.source_url || refreshing) return;
    setRefreshing(true);
    setRefreshError(null);
    setRefreshMessage("Refresh queued.");
    try {
      const job = await adminApi.kickLiveaboardImport(ctx.boat.source_url);
      setRefreshMessage("Refreshing from liveaboard.com...");
      for (;;) {
        await new Promise((resolve) => window.setTimeout(resolve, 1500));
        const latest = await adminApi.getImportJob(job.id);
        if (latest.status === "failed") {
          throw new Error(latest.error_message || "Liveaboard refresh failed.");
        }
        if (latest.status === "succeeded") {
          await ctx.refreshBoat();
          setRefreshMessage(
            `Refresh complete: ${latest.trips_inserted ?? 0} added, ${latest.trips_updated ?? 0} updated, ${latest.trips_deleted ?? 0} removed.`,
          );
          return;
        }
      }
    } catch (err) {
      setRefreshError(
        (err as { message?: string })?.message ?? "Liveaboard refresh failed.",
      );
      setRefreshMessage(null);
    } finally {
      setRefreshing(false);
    }
  }

  const columns: Column<Trip>[] = [
    {
      key: "dates",
      header: "Dates",
      cell: (t) => (
        <span className={styles.dates}>
          {t.start_date} → {t.end_date}
        </span>
      ),
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
      cell: (t) =>
        t.manifest_summary ? (
          <span>{t.manifest_summary.guest_count} guests</span>
        ) : (
          <span className={styles.muted}>0 guests</span>
        ),
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
          <span className={styles.muted}>—</span>
        ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (t) => <Link to={`/admin/trips/${t.id}/manifest`}>Manifest</Link>,
    },
  ];

  return (
    <>
      <div className={styles.toolbar}>
        <div>
          {refreshError && <div className={styles.error}>{refreshError}</div>}
          {refreshMessage && !refreshError && (
            <div className={styles.muted}>{refreshMessage}</div>
          )}
        </div>
        {isAdmin && ctx.boat.source_url && (
          <Button
            variant="secondary"
            onClick={refreshFromLiveaboard}
            disabled={refreshing}
          >
            {refreshing ? "Refreshing..." : "Refresh from liveaboard.com"}
          </Button>
        )}
      </div>
      {trips.length === 0 ? (
        <Empty title="No trips yet" hint="This boat has no scheduled trips." />
      ) : (
        <DataTable columns={columns} rows={trips} rowKey={(t) => t.id} />
      )}
    </>
  );
}

export function BoatInventory() {
  const { boat } = useBoatCtx();
  const [inventory, setInventory] = useState<BoatInventoryItem[]>([]);
  const [items, setItems] = useState<CatalogItem[]>([]);
  const [editing, setEditing] = useState<
    CatalogItem | BoatInventoryItem | null
  >(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  async function load() {
    setError(null);
    setLoading(true);
    try {
      const [inv, catalog] = await Promise.all([
        adminApi.listBoatInventory(boat.id),
        adminApi.listCatalogItems(),
      ]);
      setInventory(inv.items ?? []);
      setItems(
        (catalog.items ?? []).filter(
          (i) => i.stock_mode === "counted" && !i.archived_at,
        ),
      );
    } catch (e) {
      setError(
        (e as { message?: string })?.message ?? "Failed to load inventory.",
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [boat.id]);

  const inventoryByItem = useMemo(
    () => new Map(inventory.map((i) => [i.catalog_item_id, i])),
    [inventory],
  );

  if (loading) return <div className={styles.loading}>Loading…</div>;
  return (
    <>
      {error && <div className={styles.error}>{error}</div>}
      {items.length === 0 ? (
        <Empty
          title="No counted catalog items"
          hint="Add stock-tracked catalog items from Inventory before setting boat quantities."
        />
      ) : (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Item</th>
              <th>Category</th>
              <th className={styles.num}>On hand</th>
              <th className={styles.num}>Reorder</th>
              <th className={styles.num}>Par</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => {
              const row = inventoryByItem.get(item.id);
              return (
                <tr
                  key={item.id}
                  className={styles.clickRow}
                  onClick={() => setEditing(row ?? item)}
                >
                  <td>{item.name}</td>
                  <td>{item.category_name}</td>
                  <td className={styles.num}>{row?.quantity_on_hand ?? 0}</td>
                  <td className={styles.num}>{row?.reorder_level ?? "—"}</td>
                  <td className={styles.num}>{row?.par_level ?? "—"}</td>
                  <td>
                    <Chip variant={stockStatusVariant(row?.status)}>
                      {row?.status ?? "unset"}
                    </Chip>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      {editing && (
        <StockEditor
          boatId={boat.id}
          entry={editing}
          close={() => setEditing(null)}
          saved={() => {
            setEditing(null);
            void load();
          }}
          setError={setError}
        />
      )}
    </>
  );
}

// Map a boat-inventory status label to a shared Chip variant.
function stockStatusVariant(status: string | undefined): ChipVariant {
  switch (status) {
    case "out":
    case "low":
      return "error";
    case undefined:
      return "neutral";
    default:
      return "success";
  }
}

function StockEditor({
  boatId,
  entry,
  close,
  saved,
  setError,
}: {
  boatId: string;
  entry: CatalogItem | BoatInventoryItem;
  close: () => void;
  saved: () => void;
  setError: (s: string | null) => void;
}) {
  const catalogItemId =
    "catalog_item_id" in entry ? entry.catalog_item_id : entry.id;
  const name = "item_name" in entry ? entry.item_name : entry.name;
  const [qty, setQty] = useState(
    "quantity_on_hand" in entry ? String(entry.quantity_on_hand) : "0",
  );
  const [reorder, setReorder] = useState(
    "reorder_level" in entry && entry.reorder_level != null
      ? String(entry.reorder_level)
      : "",
  );
  const [par, setPar] = useState(
    "par_level" in entry && entry.par_level != null
      ? String(entry.par_level)
      : "",
  );
  const [delta, setDelta] = useState("");
  const [movement, setMovement] = useState("restock");
  const [note, setNote] = useState("");

  async function save(e: FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await adminApi.setBoatInventory(boatId, catalogItemId, {
        quantity_on_hand: Number(qty),
        reorder_level: reorder === "" ? null : Number(reorder),
        par_level: par === "" ? null : Number(par),
        notes: note || null,
      });
      if (delta !== "" && Number(delta) !== 0) {
        await adminApi.adjustBoatInventory(boatId, catalogItemId, {
          movement_type: movement,
          delta_quantity: Number(delta),
          note: note || null,
        });
      }
      saved();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Save failed.");
    }
  }

  return (
    <div className={styles.modalBackdrop}>
      <form className={styles.modal} onSubmit={save}>
        <h2>{name}</h2>
        <div className={styles.formRow}>
          <div className={styles.field}>
            <label>On hand</label>
            <input
              type="number"
              min="0"
              value={qty}
              onChange={(e) => setQty(e.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label>Reorder</label>
            <input
              type="number"
              min="0"
              value={reorder}
              onChange={(e) => setReorder(e.target.value)}
            />
          </div>
          <div className={styles.field}>
            <label>Par</label>
            <input
              type="number"
              min="0"
              value={par}
              onChange={(e) => setPar(e.target.value)}
            />
          </div>
        </div>
        <div className={styles.formRow}>
          <div className={styles.field}>
            <label>Adjustment</label>
            <input
              type="number"
              value={delta}
              onChange={(e) => setDelta(e.target.value)}
              placeholder="+48 or -2"
            />
          </div>
          <div className={styles.field}>
            <label>Movement</label>
            <select
              value={movement}
              onChange={(e) => setMovement(e.target.value)}
            >
              <option value="restock">restock</option>
              <option value="correction">correction</option>
              <option value="breakage">breakage</option>
              <option value="spoilage">spoilage</option>
              <option value="internal_use">internal use</option>
              <option value="initial_count">initial count</option>
            </select>
          </div>
        </div>
        <div className={styles.field}>
          <label>Note</label>
          <input value={note} onChange={(e) => setNote(e.target.value)} />
        </div>
        <div className={styles.modalActions}>
          <Button variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button type="submit" variant="primary">
            Save
          </Button>
        </div>
      </form>
    </div>
  );
}

export function BoatNotes() {
  return (
    <Empty
      title="Notes (placeholder)"
      hint="Free-form notes about this boat will live here."
    />
  );
}
