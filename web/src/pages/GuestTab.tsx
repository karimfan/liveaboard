import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { api, type GuestTabResponse } from "../lib/api";

export function GuestTab() {
  const { tripGuestId = "" } = useParams<{ tripGuestId: string }>();
  const [data, setData] = useState<GuestTabResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    api
      .guestTab(tripGuestId)
      .then((d) => !cancelled && setData(d))
      .catch((e) => !cancelled && setError(e?.message ?? "Could not load tab."));
    return () => {
      cancelled = true;
    };
  }, [tripGuestId]);

  if (error) {
    return (
      <div className="guest-registration-shell">
        <div className="error">{error}</div>
      </div>
    );
  }
  if (!data) {
    return (
      <div className="guest-registration-shell">
        <div className="muted">Loading…</div>
      </div>
    );
  }

  const t = data.trip;

  return (
    <div className="guest-registration-shell">
      <div className="guest-registration">
        <div className="guest-registration__header">
          <div>
            <h1>My tab</h1>
            <p className="muted">
              {t.boat_name} — {t.start_date} to {t.end_date} — {t.itinerary}
            </p>
          </div>
          <span className={`chip chip--${t.status}`}>{t.status}</span>
        </div>

        <div className="muted" style={{ marginBottom: "var(--sp-md)" }}>
          <Link to={`/guest/trips/${tripGuestId}/register`}>
            ← Trip registration
          </Link>
        </div>

        {!data.has_folio && (
          <div className="callout">
            No purchases recorded yet. Your tab will appear here once the
            crew records the first item.
          </div>
        )}

        {data.has_folio && (
          <>
            <section className="registration-section">
              <div className="registration-section__header">
                <div>
                  <h2>Items</h2>
                  <p className="muted">
                    {data.status === "closed"
                      ? "This tab is closed and settled."
                      : "This tab is open and updates as items are added."}
                  </p>
                </div>
              </div>
              {data.lines.length === 0 ? (
                <p className="muted">No line items yet.</p>
              ) : (
                <table className="admin-table">
                  <thead>
                    <tr>
                      <th>Item</th>
                      <th className="num">Qty</th>
                      <th className="num">Unit</th>
                      <th className="num">Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.lines.map((l) => (
                      <tr key={l.id}>
                        <td>
                          {l.item_name}
                          {l.line_type === "crew_tip" && (
                            <span className="muted"> · crew tip</span>
                          )}
                        </td>
                        <td className="num">{l.quantity}</td>
                        <td className="num">{usd(l.unit_price_usd_cents)}</td>
                        <td className="num">{usd(l.line_total_usd_cents)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>

            <section className="registration-section">
              <div className="registration-section__header">
                <div>
                  <h2>Totals</h2>
                </div>
              </div>
              <table className="admin-table">
                <tbody>
                  <tr>
                    <td>Subtotal</td>
                    <td className="num">{usd(data.subtotal_usd_cents)}</td>
                  </tr>
                  {data.card_fee_usd_cents > 0 && (
                    <tr>
                      <td>Card fee</td>
                      <td className="num">{usd(data.card_fee_usd_cents)}</td>
                    </tr>
                  )}
                  <tr>
                    <td>
                      <strong>Total (USD)</strong>
                    </td>
                    <td className="num">
                      <strong>{usd(data.total_usd_cents)}</strong>
                    </td>
                  </tr>
                  {data.settlement && (
                    <tr>
                      <td>
                        Settlement{" "}
                        <span className="muted">
                          ({data.settlement.payment_method ?? "—"})
                        </span>
                      </td>
                      <td className="num">
                        {data.settlement.currency}{" "}
                        {formatMinor(
                          data.settlement.total_minor,
                          data.settlement.currency_exp,
                        )}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </section>
          </>
        )}
      </div>
    </div>
  );
}

function usd(cents: number): string {
  const sign = cents < 0 ? "-" : "";
  const n = Math.abs(cents);
  return `${sign}$${(n / 100).toFixed(2)}`;
}

function formatMinor(minor: number, exp: number): string {
  const divisor = Math.pow(10, exp || 2);
  return (minor / divisor).toFixed(exp || 2);
}
