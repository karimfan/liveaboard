import { useEffect, useMemo, useState, type FormEvent } from "react";

import { adminApi, type Boat, type CatalogItem, type PriceOverride, type Trip } from "../api";

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
    const activeItems = (catalog.items ?? []).filter((item) => !item.archived_at);
    const boatRows = fleet.boats ?? [];
    const tripRows = (tripList.trips ?? []).filter((trip) => trip.status !== "cancelled");
    setItems(activeItems);
    setBoats(boatRows);
    setTrips(tripRows);
    setOverrides(overrideList.overrides ?? []);
    setItemId((current) => current || activeItems[0]?.id || "");
    setBoatId((current) => current || boatRows[0]?.id || "");
    setTripId((current) => current || tripRows[0]?.id || "");
  }

  useEffect(() => {
    void load().catch((err) => setError((err as { message?: string })?.message ?? "Failed to load pricing."));
  }, []);

  const selectedItem = useMemo(() => items.find((item) => item.id === itemId) ?? null, [items, itemId]);

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
        await adminApi.upsertBoatPriceOverride({ catalog_item_id: itemId, boat_id: boatId, price_usd_cents: cents, notes });
      } else {
        await adminApi.upsertTripPriceOverride({ catalog_item_id: itemId, trip_id: tripId, price_usd_cents: cents, notes });
      }
      setPrice("");
      setNotes("");
      setSaved(true);
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to save override.");
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
      setError((err as { message?: string })?.message ?? "Failed to archive override.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Pricing</h1>
          <div className="admin-page-subtitle">Per-item boat and trip price overrides. Base prices stay in the catalog.</div>
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="pricing-layout">
        <form className="admin-card pricing-form" onSubmit={save}>
          <h2 className="admin-card__title">Override price</h2>
          <div className="form-grid">
            <div className="field">
              <label htmlFor="pricing-scope">Scope</label>
              <select id="pricing-scope" value={scope} onChange={(e) => setScope(e.target.value as Scope)}>
                <option value="boat">Boat</option>
                <option value="trip">Trip</option>
              </select>
            </div>
            <div className="field">
              <label htmlFor="pricing-item">Item</label>
              <select id="pricing-item" value={itemId} onChange={(e) => setItemId(e.target.value)}>
                {items.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
              </select>
            </div>
          </div>
          <div className="form-grid">
            {scope === "boat" ? (
              <div className="field">
                <label htmlFor="pricing-boat">Boat</label>
                <select id="pricing-boat" value={boatId} onChange={(e) => setBoatId(e.target.value)}>
                  {boats.map((boat) => <option key={boat.id} value={boat.id}>{boat.name}</option>)}
                </select>
              </div>
            ) : (
              <div className="field">
                <label htmlFor="pricing-trip">Trip</label>
                <select id="pricing-trip" value={tripId} onChange={(e) => setTripId(e.target.value)}>
                  {trips.map((trip) => <option key={trip.id} value={trip.id}>{trip.itinerary} - {trip.start_date}</option>)}
                </select>
              </div>
            )}
            <div className="field">
              <label htmlFor="pricing-price">Override USD price</label>
              <input
                id="pricing-price"
                type="number"
                min="0"
                step="0.01"
                placeholder={selectedItem ? (selectedItem.price_usd_cents / 100).toFixed(2) : "0.00"}
                value={price}
                onChange={(e) => setPrice(e.target.value)}
                required
              />
            </div>
          </div>
          <div className="field">
            <label htmlFor="pricing-notes">Notes</label>
            <textarea id="pricing-notes" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} />
          </div>
          <button className="primary" disabled={busy || !itemId || (scope === "boat" ? !boatId : !tripId)}>
            {busy ? "Saving..." : "Save override"}
          </button>
          {saved && <span className="muted saved-inline">Saved</span>}
        </form>

        <section className="admin-card">
          <h2 className="admin-card__title">Active overrides</h2>
          <table className="admin-table">
            <thead>
              <tr>
                <th>Item</th>
                <th>Scope</th>
                <th>Target</th>
                <th className="num">Price</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {overrides.map((override) => (
                <tr key={override.id}>
                  <td>
                    <strong>{override.item_name}</strong>
                    {override.notes && <div className="muted">{override.notes}</div>}
                  </td>
                  <td><span className="chip chip--planned">{override.scope}</span></td>
                  <td>{override.boat_name ?? override.trip_label ?? "Unknown"}</td>
                  <td className="num">{money(override.price_usd_cents)}</td>
                  <td><button className="secondary" type="button" onClick={() => void archive(override.id)} disabled={busy}>Archive</button></td>
                </tr>
              ))}
              {overrides.length === 0 && <tr><td colSpan={5} className="muted">No overrides configured.</td></tr>}
            </tbody>
          </table>
        </section>
      </div>
    </>
  );
}

function money(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}
