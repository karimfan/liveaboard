import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { api, type GuestTabResponse } from "../lib/api";
import styles from "./GuestTab.module.css";

const tripStatusChip: Record<string, string> = {
  planned: styles.chipPlanned,
  active: styles.chipActive,
  completed: styles.chipCompleted,
  cancelled: styles.chipCancelled,
};

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
      .catch(
        (e) => !cancelled && setError(e?.message ?? "Could not load tab."),
      );
    return () => {
      cancelled = true;
    };
  }, [tripGuestId]);

  if (error) {
    return (
      <div className={styles.shell}>
        <div className={styles.error}>{error}</div>
      </div>
    );
  }
  if (!data) {
    return (
      <div className={styles.shell}>
        <div className={styles.muted}>Loading…</div>
      </div>
    );
  }

  const t = data.trip;

  return (
    <div className={styles.shell}>
      <div className={styles.card}>
        <div className={styles.header}>
          <div>
            <h1>My tab</h1>
            <p className={styles.muted}>
              {t.boat_name} — {t.start_date} to {t.end_date} — {t.itinerary}
            </p>
          </div>
          <span className={`${styles.chip} ${tripStatusChip[t.status] ?? ""}`}>
            {t.status}
          </span>
        </div>

        <div className={styles.muted} style={{ marginBottom: "var(--sp-md)" }}>
          <Link to={`/guest/trips/${tripGuestId}/register`}>
            ← Trip registration
          </Link>
        </div>

        {!data.has_folio && (
          <div className={styles.callout}>
            No purchases recorded yet. Your tab will appear here once the crew
            records the first item.
          </div>
        )}

        {data.has_folio && (
          <>
            <section className={styles.section}>
              <div className={styles.sectionHeader}>
                <div>
                  <h2>Items</h2>
                  <p className={styles.muted}>
                    {data.status === "closed"
                      ? "This tab is closed and settled."
                      : "This tab is open and updates as items are added."}
                  </p>
                </div>
              </div>
              {data.lines.length === 0 ? (
                <p className={styles.muted}>No line items yet.</p>
              ) : (
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Item</th>
                      <th className={styles.num}>Qty</th>
                      <th className={styles.num}>Unit</th>
                      <th className={styles.num}>Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.lines.map((l) => (
                      <tr key={l.id}>
                        <td>
                          {l.item_name}
                          {l.line_type === "crew_tip" && (
                            <span className={styles.muted}> · crew tip</span>
                          )}
                        </td>
                        <td className={styles.num}>{l.quantity}</td>
                        <td className={styles.num}>
                          {usd(l.unit_price_usd_cents)}
                        </td>
                        <td className={styles.num}>
                          {usd(l.line_total_usd_cents)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>

            <section className={styles.section}>
              <div className={styles.sectionHeader}>
                <div>
                  <h2>Totals</h2>
                </div>
              </div>
              <table className={styles.table}>
                <tbody>
                  <tr>
                    <td>Subtotal</td>
                    <td className={styles.num}>
                      {usd(data.subtotal_usd_cents)}
                    </td>
                  </tr>
                  {data.card_fee_usd_cents > 0 && (
                    <tr>
                      <td>Card fee</td>
                      <td className={styles.num}>
                        {usd(data.card_fee_usd_cents)}
                      </td>
                    </tr>
                  )}
                  <tr>
                    <td>
                      <strong>Total (USD)</strong>
                    </td>
                    <td className={styles.num}>
                      <strong>{usd(data.total_usd_cents)}</strong>
                    </td>
                  </tr>
                  {data.settlement && (
                    <tr>
                      <td>
                        Settlement{" "}
                        <span className={styles.muted}>
                          ({data.settlement.payment_method ?? "—"})
                        </span>
                      </td>
                      <td className={styles.num}>
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
