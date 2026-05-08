import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type CatalogItem, type GuestFolio as GuestFolioData, type PaymentSettings } from "../api";

export function GuestFolio() {
  const { id = "", guestId = "" } = useParams<{ id: string; guestId: string }>();
  const [folio, setFolio] = useState<GuestFolioData | null>(null);
  const [settings, setSettings] = useState<PaymentSettings | null>(null);
  const [items, setItems] = useState<CatalogItem[]>([]);
  const [itemId, setItemId] = useState("");
  const [qty, setQty] = useState("1");
  const [tip, setTip] = useState("");
  const [paymentMethod, setPaymentMethod] = useState("card");
  const [currency, setCurrency] = useState("USD");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    setError(null);
    const [settingsRes, itemsRes] = await Promise.all([
      adminApi.paymentSettings().catch(() => null),
      adminApi.listCatalogItems(),
    ]);
    if (settingsRes) {
      setSettings(settingsRes);
      setPaymentMethod(settingsRes.enabled_payment_methods[0] ?? "card");
      setCurrency(settingsRes.default_currency);
    }
    setItems((itemsRes.items ?? []).filter((i) => i.is_active && !i.archived_at));
    try {
      setFolio(await adminApi.getGuestFolio(id, guestId));
    } catch {
      setFolio(await adminApi.openGuestFolio(id, guestId));
    }
  }

  useEffect(() => {
    void load().catch((err) => setError((err as { message?: string })?.message ?? "Failed to load checkout."));
  }, [id, guestId]);

  useEffect(() => {
    if (!itemId && items.length > 0) setItemId(items[0].id);
  }, [items, itemId]);

  const totals = useMemo(() => {
    const subtotal = folio?.lines.reduce((sum, l) => sum + l.line_total_usd_cents, 0) ?? 0;
    const feeBps = settings?.card_fee_basis_points ?? folio?.card_fee_basis_points ?? 0;
    const fee = paymentMethod === "card" ? Math.floor((subtotal * feeBps + 5000) / 10000) : 0;
    return { subtotal, fee, total: subtotal + fee };
  }, [folio, settings, paymentMethod]);

  async function addItem(e: FormEvent) {
    e.preventDefault();
    if (!itemId) return;
    await mutate(() => adminApi.addGuestFolioLine(id, guestId, {
      line_type: "catalog_item",
      catalog_item_id: itemId,
      quantity: Number(qty),
    }));
    setQty("1");
  }

  async function addTip(e: FormEvent) {
    e.preventDefault();
    await mutate(() => adminApi.addGuestFolioLine(id, guestId, {
      line_type: "crew_tip",
      tip_usd_cents: dollarsToCents(tip),
    }));
  }

  async function updateQty(lineId: string, nextQty: number) {
    if (nextQty <= 0) return;
    await mutate(() => adminApi.updateGuestFolioLine(id, guestId, lineId, { quantity: nextQty }));
  }

  async function removeLine(lineId: string) {
    await mutate(() => adminApi.deleteGuestFolioLine(id, guestId, lineId));
  }

  async function closeFolio() {
    await mutate(() => adminApi.closeGuestFolio(id, guestId, {
      payment_method: paymentMethod,
      settlement_currency: currency,
    }), "Folio closed and email queued.");
  }

  async function resend() {
    await mutate(() => adminApi.resendGuestFolioEmail(id, guestId), "Folio email resent.");
  }

  async function mutate(fn: () => Promise<GuestFolioData>, okMessage?: string) {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const next = await fn();
      setFolio(next);
      if (okMessage) setMessage(okMessage);
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Checkout update failed.");
    } finally {
      setBusy(false);
    }
  }

  if (!folio) {
    return (
      <>
        <div className="admin-breadcrumb"><Link to={`/admin/trips/${id}/manifest`}>Manifest</Link></div>
        {error ? <div className="error">{error}</div> : <div className="muted">Loading...</div>}
      </>
    );
  }

  const closed = folio.status === "closed";
  return (
    <>
      <div className="admin-breadcrumb"><Link to={`/admin/trips/${id}/manifest`}>Manifest</Link></div>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Checkout</h1>
          <div className="admin-page-subtitle">{folio.guest_full_name} - {folio.boat_name} - {folio.start_date} to {folio.end_date}</div>
        </div>
        <span className="chip chip--active">{folio.status}</span>
      </div>
      {error && <div className="error">{error}</div>}
      {message && <div className="callout">{message}</div>}

      <div className="folio-layout">
        <div>
          {!closed && (
            <div className="admin-card folio-tools">
              <form onSubmit={addItem} className="folio-tool-row">
                <select value={itemId} onChange={(e) => setItemId(e.target.value)}>
                  {items.map((i) => <option key={i.id} value={i.id}>{i.name} - {money(i.price_usd_cents)}</option>)}
                </select>
                <input type="number" min="1" value={qty} onChange={(e) => setQty(e.target.value)} />
                <button className="primary" disabled={busy}>Add</button>
              </form>
              <form onSubmit={addTip} className="folio-tool-row">
                <input placeholder="Crew tip USD" value={tip} onChange={(e) => setTip(e.target.value)} />
                <button className="secondary" disabled={busy}>Set tip</button>
              </form>
            </div>
          )}

          <table className="admin-table">
            <thead>
              <tr><th>Item</th><th className="num">Qty</th><th className="num">Unit</th><th className="num">Total</th><th></th></tr>
            </thead>
            <tbody>
              {folio.lines.map((line) => (
                <tr key={line.id}>
                  <td>{line.item_name}<div className="muted">{line.line_type === "crew_tip" ? "Crew tip" : line.stock_mode}</div></td>
                  <td className="num">
                    {closed || line.line_type === "crew_tip" ? line.quantity : (
                      <input
                        className="qty-input"
                        type="number"
                        min="1"
                        value={line.quantity}
                        onChange={(e) => void updateQty(line.id, Number(e.target.value))}
                      />
                    )}
                  </td>
                  <td className="num">{money(line.unit_price_usd_cents)}</td>
                  <td className="num">{money(line.line_total_usd_cents)}</td>
                  <td className="actions-cell">
                    {!closed && <button className="ghost" onClick={() => void removeLine(line.id)} disabled={busy}>Remove</button>}
                  </td>
                </tr>
              ))}
              {folio.lines.length === 0 && <tr><td colSpan={5} className="muted">No folio lines yet.</td></tr>}
            </tbody>
          </table>
        </div>

        <aside className="admin-card folio-summary">
          <h2 className="admin-card__title">Total</h2>
          <div className="total-row"><span>Subtotal</span><strong>{money(closed ? folio.subtotal_usd_cents : totals.subtotal)}</strong></div>
          <div className="total-row"><span>Card fee</span><strong>{money(closed ? folio.card_fee_usd_cents : totals.fee)}</strong></div>
          <div className="total-row total-row--grand"><span>Total USD</span><strong>{money(closed ? folio.total_usd_cents : totals.total)}</strong></div>
          {!closed ? (
            <>
              <div className="field">
                <label>Payment method</label>
                <select value={paymentMethod} onChange={(e) => setPaymentMethod(e.target.value)}>
                  {(settings?.enabled_payment_methods ?? ["card", "cash", "other"]).map((m) => <option key={m} value={m}>{m}</option>)}
                </select>
              </div>
              <div className="field">
                <label>Settlement currency</label>
                <select value={currency} onChange={(e) => setCurrency(e.target.value)}>
                  {(settings?.supported_currencies ?? ["USD"]).map((c) => <option key={c} value={c}>{c}</option>)}
                </select>
              </div>
              <button className="primary" onClick={() => void closeFolio()} disabled={busy || folio.lines.length === 0}>Close as paid</button>
            </>
          ) : (
            <>
              <div className="total-row"><span>Paid by</span><strong>{folio.payment_method}</strong></div>
              <div className="total-row"><span>Settlement</span><strong>{minor(folio.settlement_total_minor, folio.currency_exponent)} {folio.settlement_currency}</strong></div>
              <div className="total-row"><span>Email</span><strong>{folio.email_send_status}</strong></div>
              {folio.email_last_error && <div className="error-inline">{folio.email_last_error}</div>}
              <button className="secondary" onClick={() => void resend()} disabled={busy}>Resend email</button>
            </>
          )}
        </aside>
      </div>
    </>
  );
}

function money(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`;
}

function dollarsToCents(value: string): number {
  return Math.round(Number(value || "0") * 100);
}

function minor(value: number | null, exponent: number | null): string {
  if (value == null || exponent == null) return "";
  return (value / Math.pow(10, exponent)).toFixed(exponent);
}
