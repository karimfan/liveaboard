import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type FolioWarning, type TripLedger } from "../api";

export function TripConsumptionLedger() {
  const { id = "" } = useParams<{ id: string }>();
  const [ledger, setLedger] = useState<TripLedger | null>(null);
  const [guestQuery, setGuestQuery] = useState("");
  const [itemQuery, setItemQuery] = useState("");
  const [guestId, setGuestId] = useState("");
  const [itemId, setItemId] = useState("");
  const [qty, setQty] = useState(1);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [warnings, setWarnings] = useState<FolioWarning[]>([]);

  async function load() {
    setError(null);
    const next = await adminApi.getTripLedger(id);
    setLedger(next);
    setGuestId((current) => current || next.guests[0]?.trip_guest_id || "");
    setItemId((current) => current || next.catalog.find((item) => item.is_active && !item.archived_at)?.id || "");
  }

  useEffect(() => {
    void load().catch((err) => setError((err as { message?: string })?.message ?? "Failed to load ledger."));
  }, [id]);

  const inventoryByItem = useMemo(() => {
    const out = new Map<string, { quantity_on_hand: number; status: string }>();
    for (const row of ledger?.inventory ?? []) out.set(row.catalog_item_id, row);
    return out;
  }, [ledger]);

  const guests = useMemo(() => {
    const q = guestQuery.trim().toLowerCase();
    return (ledger?.guests ?? []).filter((guest) => !q || `${guest.full_name} ${guest.email}`.toLowerCase().includes(q));
  }, [ledger, guestQuery]);

  const items = useMemo(() => {
    const q = itemQuery.trim().toLowerCase();
    return (ledger?.catalog ?? [])
      .filter((item) => item.is_active && !item.archived_at)
      .filter((item) => !q || `${item.name} ${item.category_name}`.toLowerCase().includes(q));
  }, [ledger, itemQuery]);

  const selectedGuest = ledger?.guests.find((guest) => guest.trip_guest_id === guestId) ?? null;
  const selectedItem = ledger?.catalog.find((item) => item.id === itemId) ?? null;

  async function addLine() {
    if (!guestId || !itemId || qty <= 0) return;
    setBusy(true);
    setError(null);
    setWarnings([]);
    try {
      const result = await adminApi.addTripLedgerLine(id, {
        trip_guest_id: guestId,
        catalog_item_id: itemId,
        quantity: qty,
        client_request_id: requestID(),
      });
      setWarnings(result.warnings ?? []);
      setQty(1);
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not add line.");
    } finally {
      setBusy(false);
    }
  }

  if (!ledger) {
    return (
      <>
        <div className="admin-breadcrumb"><Link to={`/admin/trips/${id}/manifest`}>Manifest</Link></div>
        {error ? <div className="error">{error}</div> : <div className="muted">Loading...</div>}
      </>
    );
  }

  return (
    <>
      <div className="admin-breadcrumb"><Link to={`/admin/trips/${id}/manifest`}>Manifest</Link></div>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Consumption ledger</h1>
          <div className="admin-page-subtitle">{ledger.trip.itinerary} - {ledger.trip.start_date} to {ledger.trip.end_date}</div>
        </div>
        <span className={`chip ${ledger.trip.status === "active" ? "chip--active" : "chip--warning"}`}>{ledger.trip.status}</span>
      </div>

      {error && <div className="error">{error}</div>}
      {warnings.map((warning) => (
        <div key={`${warning.code}-${warning.catalog_item_id ?? ""}`} className="callout callout--warning">{warning.message} {warning.quantity_on_hand != null ? `On hand: ${warning.quantity_on_hand}` : ""}</div>
      ))}

      <div className="ledger-shell">
        <section className="ledger-panel ledger-panel--guests">
          <div className="ledger-panel__header">
            <h2>Guest</h2>
            <input value={guestQuery} onChange={(e) => setGuestQuery(e.target.value)} placeholder="Search guests" />
          </div>
          <div className="ledger-choice-grid ledger-choice-grid--guests">
            {guests.map((guest) => (
              <button
                key={guest.trip_guest_id}
                type="button"
                className={`ledger-choice ${guest.trip_guest_id === guestId ? "ledger-choice--selected" : ""}`}
                onClick={() => setGuestId(guest.trip_guest_id)}
              >
                <strong>{guest.full_name}</strong>
                <span>{money(guest.subtotal_usd_cents)} - {guest.line_count} lines</span>
              </button>
            ))}
          </div>
        </section>

        <section className="ledger-panel ledger-panel--items">
          <div className="ledger-panel__header">
            <h2>Item</h2>
            <input value={itemQuery} onChange={(e) => setItemQuery(e.target.value)} placeholder="Search items" />
          </div>
          <div className="ledger-choice-grid">
            {items.map((item) => {
              const stock = inventoryByItem.get(item.id);
              const stockText = item.stock_mode === "counted" ? `${stock?.quantity_on_hand ?? 0} on hand` : "not counted";
              const low = item.stock_mode === "counted" && (stock?.quantity_on_hand ?? 0) <= 0;
              return (
                <button
                  key={item.id}
                  type="button"
                  className={`ledger-choice ${item.id === itemId ? "ledger-choice--selected" : ""} ${low ? "ledger-choice--warn" : ""}`}
                  onClick={() => setItemId(item.id)}
                >
                  <strong>{item.name}</strong>
                  <span>{money(item.price_usd_cents)} - {stockText}</span>
                </button>
              );
            })}
          </div>
        </section>

        <aside className="ledger-panel ledger-panel--recent">
          <h2>Recent</h2>
          <div className="ledger-recent-list">
            {ledger.recent.map((line) => (
              <div className="ledger-recent" key={line.id}>
                <strong>{line.item_name}</strong>
                <span>{line.guest_full_name} - x{line.quantity} - {money(line.line_total_usd_cents)}</span>
              </div>
            ))}
            {ledger.recent.length === 0 && <div className="muted">No lines yet.</div>}
          </div>
        </aside>
      </div>

      <div className="ledger-submit">
        <div>
          <strong>{selectedGuest?.full_name ?? "Select guest"}</strong>
          <span>{selectedItem ? `${selectedItem.name} - ${money(selectedItem.price_usd_cents)}` : "Select item"}</span>
        </div>
        <div className="ledger-stepper">
          <button type="button" onClick={() => setQty((n) => Math.max(1, n - 1))} disabled={busy}>-</button>
          <input type="number" min="1" value={qty} onChange={(e) => setQty(Math.max(1, Number(e.target.value) || 1))} />
          <button type="button" onClick={() => setQty((n) => n + 1)} disabled={busy}>+</button>
        </div>
        <button className="primary" type="button" onClick={() => void addLine()} disabled={busy || ledger.trip.status !== "active" || !guestId || !itemId}>
          Add
        </button>
      </div>
    </>
  );
}

function money(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function requestID(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
