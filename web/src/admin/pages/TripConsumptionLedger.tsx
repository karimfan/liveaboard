import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type FolioWarning, type TripLedger } from "../api";
import { Button, Chip, PageHeader } from "../components";

import styles from "./TripConsumptionLedger.module.css";

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
    setItemId(
      (current) =>
        current ||
        next.catalog.find((item) => item.is_active && !item.archived_at)?.id ||
        "",
    );
  }

  useEffect(() => {
    void load().catch((err) =>
      setError(
        (err as { message?: string })?.message ?? "Failed to load ledger.",
      ),
    );
  }, [id]);

  const inventoryByItem = useMemo(() => {
    const out = new Map<string, { quantity_on_hand: number; status: string }>();
    for (const row of ledger?.inventory ?? [])
      out.set(row.catalog_item_id, row);
    return out;
  }, [ledger]);

  const guests = useMemo(() => {
    const q = guestQuery.trim().toLowerCase();
    return (ledger?.guests ?? []).filter(
      (guest) =>
        !q || `${guest.full_name} ${guest.email}`.toLowerCase().includes(q),
    );
  }, [ledger, guestQuery]);

  const items = useMemo(() => {
    const q = itemQuery.trim().toLowerCase();
    return (ledger?.catalog ?? [])
      .filter((item) => item.is_active && !item.archived_at)
      .filter(
        (item) =>
          !q || `${item.name} ${item.category_name}`.toLowerCase().includes(q),
      );
  }, [ledger, itemQuery]);

  const selectedGuest =
    ledger?.guests.find((guest) => guest.trip_guest_id === guestId) ?? null;
  const selectedItem =
    ledger?.catalog.find((item) => item.id === itemId) ?? null;

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

  return (
    <>
      <div className={styles.breadcrumb}>
        <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
      </div>
      <PageHeader
        title="Consumption ledger"
        subtitle={`${ledger.trip.itinerary} - ${ledger.trip.start_date} to ${ledger.trip.end_date}`}
        actions={
          <Chip
            variant={ledger.trip.status === "active" ? "success" : "warning"}
          >
            {ledger.trip.status}
          </Chip>
        }
      />

      {error && <div className={styles.error}>{error}</div>}
      {warnings.map((warning) => (
        <div
          key={`${warning.code}-${warning.catalog_item_id ?? ""}`}
          className={styles.calloutWarning}
        >
          {warning.message}{" "}
          {warning.quantity_on_hand != null
            ? `On hand: ${warning.quantity_on_hand}`
            : ""}
        </div>
      ))}

      <div className={styles.shell}>
        <section className={styles.panel}>
          <div className={styles.panelHeader}>
            <h2 className={styles.panelTitle}>Guest</h2>
            <input
              className={styles.input}
              value={guestQuery}
              onChange={(e) => setGuestQuery(e.target.value)}
              placeholder="Search guests"
            />
          </div>
          <div className={styles.choiceGridGuests}>
            {guests.map((guest) => (
              <button
                key={guest.trip_guest_id}
                type="button"
                className={
                  guest.trip_guest_id === guestId
                    ? `${styles.choice} ${styles.choiceSelected}`
                    : styles.choice
                }
                onClick={() => setGuestId(guest.trip_guest_id)}
              >
                <strong>{guest.full_name}</strong>
                <span>
                  {money(guest.subtotal_usd_cents)} - {guest.line_count} lines
                </span>
              </button>
            ))}
          </div>
        </section>

        <section className={styles.panel}>
          <div className={styles.panelHeader}>
            <h2 className={styles.panelTitle}>Item</h2>
            <input
              className={styles.input}
              value={itemQuery}
              onChange={(e) => setItemQuery(e.target.value)}
              placeholder="Search items"
            />
          </div>
          <div className={styles.choiceGrid}>
            {items.map((item) => {
              const stock = inventoryByItem.get(item.id);
              const stockText =
                item.stock_mode === "counted"
                  ? `${stock?.quantity_on_hand ?? 0} on hand`
                  : "not counted";
              const low =
                item.stock_mode === "counted" &&
                (stock?.quantity_on_hand ?? 0) <= 0;
              const price =
                item.effective_price_usd_cents ?? item.price_usd_cents;
              const source =
                item.price_source && item.price_source !== "base"
                  ? ` - ${item.price_source.replace("_", " ")}`
                  : "";
              const cls = [
                styles.choice,
                item.id === itemId ? styles.choiceSelected : "",
                low ? styles.choiceWarn : "",
              ]
                .filter(Boolean)
                .join(" ");
              return (
                <button
                  key={item.id}
                  type="button"
                  className={cls}
                  onClick={() => setItemId(item.id)}
                >
                  <strong>{item.name}</strong>
                  <span>
                    {money(price)}
                    {source} - {stockText}
                  </span>
                </button>
              );
            })}
          </div>
        </section>

        <aside className={`${styles.panel} ${styles.panelRecent}`}>
          <h2 className={styles.panelTitle}>Recent</h2>
          <div className={styles.recentList}>
            {ledger.recent.map((line) => (
              <div className={styles.recent} key={line.id}>
                <strong>{line.item_name}</strong>
                <span>
                  {line.guest_full_name} - x{line.quantity} -{" "}
                  {money(line.line_total_usd_cents)}
                </span>
              </div>
            ))}
            {ledger.recent.length === 0 && (
              <div className={styles.muted}>No lines yet.</div>
            )}
          </div>
        </aside>
      </div>

      <div className={styles.submit}>
        <div className={styles.submitSummary}>
          <strong>{selectedGuest?.full_name ?? "Select guest"}</strong>
          <span>
            {selectedItem
              ? `${selectedItem.name} - ${money(selectedItem.effective_price_usd_cents ?? selectedItem.price_usd_cents)}`
              : "Select item"}
          </span>
        </div>
        <div className={styles.stepper}>
          <button
            type="button"
            onClick={() => setQty((n) => Math.max(1, n - 1))}
            disabled={busy}
          >
            -
          </button>
          <input
            className={styles.input}
            type="number"
            min="1"
            value={qty}
            onChange={(e) => setQty(Math.max(1, Number(e.target.value) || 1))}
          />
          <button
            type="button"
            onClick={() => setQty((n) => n + 1)}
            disabled={busy}
          >
            +
          </button>
        </div>
        <Button
          variant="primary"
          onClick={() => void addLine()}
          disabled={
            busy || ledger.trip.status !== "active" || !guestId || !itemId
          }
        >
          Add
        </Button>
      </div>
    </>
  );
}

function money(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function requestID(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto)
    return crypto.randomUUID();
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
