import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";

import {
  adminApi,
  type CatalogItem,
  type GuestFolio as GuestFolioData,
  type PaymentSettings,
} from "../api";
import {
  Button,
  Card,
  Chip,
  type ChipVariant,
  type Column,
  DataTable,
  PageHeader,
} from "../components";

import styles from "./GuestFolio.module.css";

type FolioLine = GuestFolioData["lines"][number];

export function GuestFolio() {
  const { id = "", guestId = "" } = useParams<{
    id: string;
    guestId: string;
  }>();
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
    setItems(
      (itemsRes.items ?? []).filter((i) => i.is_active && !i.archived_at),
    );
    try {
      setFolio(await adminApi.getGuestFolio(id, guestId));
    } catch {
      setFolio(await adminApi.openGuestFolio(id, guestId));
    }
  }

  useEffect(() => {
    void load().catch((err) =>
      setError(
        (err as { message?: string })?.message ?? "Failed to load checkout.",
      ),
    );
  }, [id, guestId]);

  useEffect(() => {
    if (!itemId && items.length > 0) setItemId(items[0].id);
  }, [items, itemId]);

  const totals = useMemo(() => {
    const subtotal =
      folio?.lines.reduce((sum, l) => sum + l.line_total_usd_cents, 0) ?? 0;
    const feeBps =
      settings?.card_fee_basis_points ?? folio?.card_fee_basis_points ?? 0;
    const fee =
      paymentMethod === "card"
        ? Math.floor((subtotal * feeBps + 5000) / 10000)
        : 0;
    return { subtotal, fee, total: subtotal + fee };
  }, [folio, settings, paymentMethod]);

  async function addItem(e: FormEvent) {
    e.preventDefault();
    if (!itemId) return;
    await mutate(() =>
      adminApi.addGuestFolioLine(id, guestId, {
        line_type: "catalog_item",
        catalog_item_id: itemId,
        quantity: Number(qty),
      }),
    );
    setQty("1");
  }

  async function addTip(e: FormEvent) {
    e.preventDefault();
    await mutate(() =>
      adminApi.addGuestFolioLine(id, guestId, {
        line_type: "crew_tip",
        tip_usd_cents: dollarsToCents(tip),
      }),
    );
  }

  async function updateQty(lineId: string, nextQty: number) {
    if (nextQty <= 0) return;
    await mutate(() =>
      adminApi.updateGuestFolioLine(id, guestId, lineId, { quantity: nextQty }),
    );
  }

  async function removeLine(lineId: string) {
    await mutate(() => adminApi.deleteGuestFolioLine(id, guestId, lineId));
  }

  async function closeFolio() {
    await mutate(
      () =>
        adminApi.closeGuestFolio(id, guestId, {
          payment_method: paymentMethod,
          settlement_currency: currency,
        }),
      "Folio closed and email queued.",
    );
  }

  async function resend() {
    await mutate(
      () => adminApi.resendGuestFolioEmail(id, guestId),
      "Folio email resent.",
    );
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
      setError(
        (err as { message?: string })?.message ?? "Checkout update failed.",
      );
    } finally {
      setBusy(false);
    }
  }

  if (!folio) {
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

  const closed = folio.status === "closed";

  const lineColumns: Column<FolioLine>[] = [
    {
      key: "item",
      header: "Item",
      cell: (line) => (
        <>
          {line.item_name}
          <div className={styles.muted}>
            {line.line_type === "crew_tip" ? "Crew tip" : line.stock_mode}
          </div>
        </>
      ),
    },
    {
      key: "qty",
      header: "Qty",
      align: "right",
      tabular: true,
      cell: (line) =>
        closed || line.line_type === "crew_tip" ? (
          line.quantity
        ) : (
          <input
            className={styles.qtyInput}
            type="number"
            min="1"
            value={line.quantity}
            onChange={(e) => void updateQty(line.id, Number(e.target.value))}
          />
        ),
    },
    {
      key: "unit",
      header: "Unit",
      align: "right",
      tabular: true,
      cell: (line) => money(line.unit_price_usd_cents),
    },
    {
      key: "total",
      header: "Total",
      align: "right",
      tabular: true,
      cell: (line) => money(line.line_total_usd_cents),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (line) =>
        !closed && (
          <Button
            variant="quiet"
            onClick={() => void removeLine(line.id)}
            disabled={busy}
          >
            Remove
          </Button>
        ),
    },
  ];

  return (
    <>
      <div className={styles.breadcrumb}>
        <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
      </div>
      <PageHeader
        title="Checkout"
        subtitle={`${folio.guest_full_name} - ${folio.boat_name} - ${folio.start_date} to ${folio.end_date}`}
        actions={
          <Chip variant={statusVariant(folio.status)}>{folio.status}</Chip>
        }
      />
      {error && <div className={styles.error}>{error}</div>}
      {message && <div className={styles.callout}>{message}</div>}

      <div className={styles.layout}>
        <div>
          {!closed && (
            <Card className={styles.tools}>
              <form onSubmit={addItem} className={styles.toolRow}>
                <select
                  value={itemId}
                  onChange={(e) => setItemId(e.target.value)}
                >
                  {items.map((i) => (
                    <option key={i.id} value={i.id}>
                      {i.name} - {money(i.price_usd_cents)}
                    </option>
                  ))}
                </select>
                <input
                  type="number"
                  min="1"
                  value={qty}
                  onChange={(e) => setQty(e.target.value)}
                />
                <Button type="submit" variant="primary" disabled={busy}>
                  Add
                </Button>
              </form>
              <form
                onSubmit={addTip}
                className={`${styles.toolRow} ${styles.toolRowWide}`}
              >
                <input
                  placeholder="Crew tip USD"
                  value={tip}
                  onChange={(e) => setTip(e.target.value)}
                />
                <Button type="submit" variant="secondary" disabled={busy}>
                  Set tip
                </Button>
              </form>
            </Card>
          )}

          <DataTable
            columns={lineColumns}
            rows={folio.lines}
            rowKey={(line) => line.id}
            empty={<span className={styles.muted}>No folio lines yet.</span>}
          />
        </div>

        <Card className={styles.summary} title="Total">
          <div className={styles.totalRow}>
            <span>Subtotal</span>
            <strong>
              {money(closed ? folio.subtotal_usd_cents : totals.subtotal)}
            </strong>
          </div>
          <div className={styles.totalRow}>
            <span>Card fee</span>
            <strong>
              {money(closed ? folio.card_fee_usd_cents : totals.fee)}
            </strong>
          </div>
          <div className={`${styles.totalRow} ${styles.totalRowGrand}`}>
            <span>Total USD</span>
            <strong>
              {money(closed ? folio.total_usd_cents : totals.total)}
            </strong>
          </div>
          {!closed ? (
            <>
              <div className={styles.field}>
                <label>Payment method</label>
                <select
                  value={paymentMethod}
                  onChange={(e) => setPaymentMethod(e.target.value)}
                >
                  {(
                    settings?.enabled_payment_methods ?? [
                      "card",
                      "cash",
                      "other",
                    ]
                  ).map((m) => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  ))}
                </select>
              </div>
              <div className={styles.field}>
                <label>Settlement currency</label>
                <select
                  value={currency}
                  onChange={(e) => setCurrency(e.target.value)}
                >
                  {(settings?.supported_currencies ?? ["USD"]).map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </select>
              </div>
              <Button
                variant="primary"
                onClick={() => void closeFolio()}
                disabled={busy || folio.lines.length === 0}
              >
                Close as paid
              </Button>
            </>
          ) : (
            <>
              <div className={styles.totalRow}>
                <span>Paid by</span>
                <strong>{folio.payment_method}</strong>
              </div>
              <div className={styles.totalRow}>
                <span>Settlement</span>
                <strong>
                  {minor(folio.settlement_total_minor, folio.currency_exponent)}{" "}
                  {folio.settlement_currency}
                </strong>
              </div>
              <div className={styles.totalRow}>
                <span>Email</span>
                <strong>{folio.email_send_status}</strong>
              </div>
              {folio.email_last_error && (
                <div className={styles.errorInline}>
                  {folio.email_last_error}
                </div>
              )}
              <Button
                variant="secondary"
                onClick={() => void resend()}
                disabled={busy}
              >
                Resend email
              </Button>
            </>
          )}
        </Card>
      </div>
    </>
  );
}

// Map a folio status label to a shared Chip variant.
function statusVariant(label: string): ChipVariant {
  switch (label) {
    case "paid":
    case "settled":
      return "success";
    case "closed":
      return "info";
    case "cancelled":
      return "error";
    default:
      return "neutral";
  }
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
