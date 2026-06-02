import { useEffect, useMemo, useState, type FormEvent } from "react";

import {
  adminApi,
  type Boat,
  type CatalogItem,
  type PriceOverride,
  type Trip,
} from "../api";
import {
  Button,
  Card,
  Chip,
  type Column,
  DataTable,
  Field,
  PageHeader,
} from "../components";

import styles from "./OrganizationPricing.module.css";

type Scope = "boat" | "trip";

export function OrganizationPricing() {
  const [items, setItems] = useState<CatalogItem[]>([]);
  const [boats, setBoats] = useState<Boat[]>([]);
  const [trips, setTrips] = useState<Trip[]>([]);
  const [overrides, setOverrides] = useState<PriceOverride[]>([]);
  const [scope, setScope] = useState<Scope>("boat");
  const [itemId, setItemId] = useState("");
  const [boatId, setBoatId] = useState("");
  const [tripId, setTripId] = useState("");
  const [price, setPrice] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);

  async function load() {
    setError(null);
    const [catalog, fleet, tripList, overrideList] = await Promise.all([
      adminApi.listCatalogItems(),
      adminApi.listBoats(),
      adminApi.listTrips(),
      adminApi.listPriceOverrides(),
    ]);
    const activeItems = (catalog.items ?? []).filter(
      (item) => !item.archived_at,
    );
    const boatRows = fleet.boats ?? [];
    const tripRows = (tripList.trips ?? []).filter(
      (trip) => trip.status !== "cancelled",
    );
    setItems(activeItems);
    setBoats(boatRows);
    setTrips(tripRows);
    setOverrides(overrideList.overrides ?? []);
    setItemId((current) => current || activeItems[0]?.id || "");
    setBoatId((current) => current || boatRows[0]?.id || "");
    setTripId((current) => current || tripRows[0]?.id || "");
  }

  useEffect(() => {
    void load().catch((err) =>
      setError(
        (err as { message?: string })?.message ?? "Failed to load pricing.",
      ),
    );
  }, []);

  const selectedItem = useMemo(
    () => items.find((item) => item.id === itemId) ?? null,
    [items, itemId],
  );

  async function save(e: FormEvent) {
    e.preventDefault();
    if (!itemId) return;
    const cents = Math.round(Number(price) * 100);
    if (!Number.isFinite(cents) || cents < 0) {
      setError("Price must be zero or greater.");
      return;
    }
    setBusy(true);
    setSaved(false);
    setError(null);
    try {
      if (scope === "boat") {
        await adminApi.upsertBoatPriceOverride({
          catalog_item_id: itemId,
          boat_id: boatId,
          price_usd_cents: cents,
          notes,
        });
      } else {
        await adminApi.upsertTripPriceOverride({
          catalog_item_id: itemId,
          trip_id: tripId,
          price_usd_cents: cents,
          notes,
        });
      }
      setPrice("");
      setNotes("");
      setSaved(true);
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to save override.",
      );
    } finally {
      setBusy(false);
    }
  }

  async function archive(id: string) {
    setBusy(true);
    setError(null);
    try {
      await adminApi.archivePriceOverride(id);
      await load();
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Failed to archive override.",
      );
    } finally {
      setBusy(false);
    }
  }

  const columns: Column<PriceOverride>[] = [
    {
      key: "item",
      header: "Item",
      cell: (override) => (
        <>
          <div className={styles.itemName}>{override.item_name}</div>
          {override.notes && (
            <div className={styles.notes}>{override.notes}</div>
          )}
        </>
      ),
    },
    {
      key: "scope",
      header: "Scope",
      cell: (override) => <Chip variant="neutral">{override.scope}</Chip>,
    },
    {
      key: "target",
      header: "Target",
      cell: (override) =>
        override.boat_name ?? override.trip_label ?? "Unknown",
    },
    {
      key: "price",
      header: "Price",
      align: "right",
      tabular: true,
      cell: (override) => money(override.price_usd_cents),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (override) => (
        <Button
          variant="secondary"
          type="button"
          onClick={() => void archive(override.id)}
          disabled={busy}
        >
          Archive
        </Button>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title="Pricing"
        subtitle="Per-item boat and trip price overrides. Base prices stay in the catalog."
      />

      {error && <div className={styles.error}>{error}</div>}

      <div className={styles.layout}>
        <Card title="Override price" className={styles.form}>
          <form onSubmit={save}>
            <div className={styles.formGrid}>
              <Field label="Scope" htmlFor="pricing-scope">
                <select
                  id="pricing-scope"
                  value={scope}
                  onChange={(e) => setScope(e.target.value as Scope)}
                >
                  <option value="boat">Boat</option>
                  <option value="trip">Trip</option>
                </select>
              </Field>
              <Field label="Item" htmlFor="pricing-item">
                <select
                  id="pricing-item"
                  value={itemId}
                  onChange={(e) => setItemId(e.target.value)}
                >
                  {items.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
            <div className={styles.formGrid}>
              {scope === "boat" ? (
                <Field label="Boat" htmlFor="pricing-boat">
                  <select
                    id="pricing-boat"
                    value={boatId}
                    onChange={(e) => setBoatId(e.target.value)}
                  >
                    {boats.map((boat) => (
                      <option key={boat.id} value={boat.id}>
                        {boat.name}
                      </option>
                    ))}
                  </select>
                </Field>
              ) : (
                <Field label="Trip" htmlFor="pricing-trip">
                  <select
                    id="pricing-trip"
                    value={tripId}
                    onChange={(e) => setTripId(e.target.value)}
                  >
                    {trips.map((trip) => (
                      <option key={trip.id} value={trip.id}>
                        {trip.itinerary} - {trip.start_date}
                      </option>
                    ))}
                  </select>
                </Field>
              )}
              <Field label="Override USD price" htmlFor="pricing-price">
                <input
                  id="pricing-price"
                  type="number"
                  min="0"
                  step="0.01"
                  placeholder={
                    selectedItem
                      ? (selectedItem.price_usd_cents / 100).toFixed(2)
                      : "0.00"
                  }
                  value={price}
                  onChange={(e) => setPrice(e.target.value)}
                  required
                />
              </Field>
            </div>
            <Field label="Notes" htmlFor="pricing-notes">
              <textarea
                id="pricing-notes"
                rows={3}
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </Field>
            <div className={styles.actions}>
              <Button
                type="submit"
                variant="primary"
                disabled={
                  busy || !itemId || (scope === "boat" ? !boatId : !tripId)
                }
              >
                {busy ? "Saving..." : "Save override"}
              </Button>
              {saved && <span className={styles.saved}>Saved</span>}
            </div>
          </form>
        </Card>

        <Card title="Active overrides">
          <DataTable
            columns={columns}
            rows={overrides}
            rowKey={(override) => override.id}
            empty={
              <span className={styles.notes}>No overrides configured.</span>
            }
          />
        </Card>
      </div>
    </>
  );
}

function money(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}
