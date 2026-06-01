import type { CockpitResponse } from "../CockpitApi";

import styles from "../Cockpit.module.css";

export function MoneyStrip({ money }: { money: CockpitResponse["money"] }) {
  return (
    <section className={styles.moneyStrip} aria-label="Money pulse">
      <Metric label="Open folios" value={money.open_folio_count.toString()} />
      <Metric label="Outstanding" value={formatUSD(money.outstanding_usd_cents)} />
      <Metric label="Settled" value={formatUSD(money.settled_usd_cents)} />
      <Metric label="Deposit pending" value={money.deposit_pending_quotes.toString()} />
      <Metric label="Held quotes" value={money.held_quotes.toString()} />
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.moneyMetric}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function formatUSD(cents: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(cents / 100);
}
