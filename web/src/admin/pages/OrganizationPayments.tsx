import { useEffect, useState, type FormEvent } from "react";

import { api } from "../../lib/api";
import { adminApi, type PaymentSettings } from "../api";
import { CurrencyMultiPicker } from "../CurrencyPicker";
import {
  Button,
  Card,
  Chip,
  type ChipVariant,
  Field,
  PageHeader,
} from "../components";

import styles from "./OrganizationPayments.module.css";

const methods = [
  { value: "card", label: "Card" },
  { value: "cash", label: "Cash" },
  { value: "other", label: "Other" },
];

function rateChipLabel(status: "fresh" | "stale" | "missing"): string {
  switch (status) {
    case "fresh":
      return "fresh";
    case "stale":
      return "stale";
    case "missing":
      return "rate needed";
  }
}

function rateChipVariant(status: "fresh" | "stale" | "missing"): ChipVariant {
  switch (status) {
    case "fresh":
      return "success";
    case "stale":
      return "warning";
    case "missing":
      return "error";
  }
}

function formatRefreshAgo(iso: string): string {
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return "recently";
  const ms = Date.now() - t;
  if (ms < 0) return "just now";
  const mins = Math.floor(ms / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins} min ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} hour${hours === 1 ? "" : "s"} ago`;
  const days = Math.floor(hours / 24);
  return `${days} day${days === 1 ? "" : "s"} ago`;
}

export function OrganizationPayments() {
  const [settings, setSettings] = useState<PaymentSettings | null>(null);
  const [orgCurrency, setOrgCurrency] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    adminApi
      .paymentSettings()
      .then(setSettings)
      .catch((err) =>
        setError(
          (err as { message?: string })?.message ??
            "Failed to load payment settings.",
        ),
      );
    api
      .organization()
      .then((o) => setOrgCurrency(o.currency))
      .catch(() => {
        /* non-fatal: picker just won't lock the country currency */
      });
  }, []);

  function setSupported(supported: string[]) {
    if (!settings) return;
    // USD is always supported.
    if (!supported.includes("USD")) supported = ["USD", ...supported].sort();
    const defaultCurrency = supported.includes(settings.default_currency)
      ? settings.default_currency
      : "USD";
    setSettings({
      ...settings,
      supported_currencies: supported,
      default_currency: defaultCurrency,
    });
  }

  function toggleMethod(method: string) {
    if (!settings) return;
    const set = new Set(settings.enabled_payment_methods);
    if (set.has(method)) set.delete(method);
    else set.add(method);
    setSettings({
      ...settings,
      enabled_payment_methods: Array.from(set).sort(),
    });
  }

  async function save(e: FormEvent) {
    e.preventDefault();
    if (!settings) return;
    setSaving(true);
    setSaved(false);
    setError(null);
    try {
      const updated = await adminApi.updatePaymentSettings({
        default_currency: settings.default_currency,
        supported_currencies: settings.supported_currencies,
        enabled_payment_methods: settings.enabled_payment_methods,
        card_fee_basis_points: settings.card_fee_basis_points,
        folio_email_footer: settings.folio_email_footer,
      });
      setSettings(updated);
      setSaved(true);
    } catch (err) {
      setError(
        (err as { message?: string })?.message ??
          "Failed to save payment settings.",
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <PageHeader
        title="Payments"
        subtitle="Checkout currencies, offline methods, and card fee settings."
      />
      {error && <div className={styles.error}>{error}</div>}
      {!settings ? (
        <div className={styles.loading}>Loading...</div>
      ) : (
        <Card title="Checkout settings" className={styles.card}>
          <form onSubmit={save}>
            <Field label="Accepted currencies">
              <CurrencyMultiPicker
                value={settings.supported_currencies}
                onChange={setSupported}
                locked={orgCurrency ? ["USD", orgCurrency] : ["USD"]}
                placeholder="Add a currency…"
              />
              <div className={styles.rateReadiness}>
                {settings.supported_currencies
                  .filter((c) => c !== "USD")
                  .map((c) => {
                    const r = settings.rate_readiness.find(
                      (x) => x.currency === c,
                    );
                    const status = r?.status ?? "missing";
                    return (
                      <Chip key={c} variant={rateChipVariant(status)}>
                        {c}: {rateChipLabel(status)}
                      </Chip>
                    );
                  })}
              </div>
              <div className={styles.note}>
                Rates auto-refresh daily from the European Central Bank (via
                Frankfurter). USD is always supported; the country currency is
                locked here so a conversion target is always available.
                {settings.auto_refresh_at && (
                  <>
                    {" "}
                    Last refresh {formatRefreshAgo(settings.auto_refresh_at)}.
                  </>
                )}
              </div>
            </Field>
            <div className={styles.formGrid}>
              <Field
                label="Default settlement currency"
                htmlFor="default-currency"
              >
                <select
                  id="default-currency"
                  value={settings.default_currency}
                  onChange={(e) =>
                    setSettings({
                      ...settings,
                      default_currency: e.target.value,
                    })
                  }
                >
                  {settings.supported_currencies.map((code) => (
                    <option key={code} value={code}>
                      {code}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Card fee percent" htmlFor="card-fee">
                <input
                  id="card-fee"
                  type="number"
                  min="0"
                  max="20"
                  step="0.01"
                  value={(settings.card_fee_basis_points / 100).toFixed(2)}
                  onChange={(e) =>
                    setSettings({
                      ...settings,
                      card_fee_basis_points: Math.round(
                        Number(e.target.value) * 100,
                      ),
                    })
                  }
                />
              </Field>
            </div>
            <Field label="Payment methods">
              <div className={styles.toggleGrid}>
                {methods.map((m) => (
                  <label key={m.value} className={styles.checkRow}>
                    <input
                      type="checkbox"
                      checked={settings.enabled_payment_methods.includes(
                        m.value,
                      )}
                      onChange={() => toggleMethod(m.value)}
                    />
                    {m.label}
                  </label>
                ))}
              </div>
            </Field>
            <Field label="Folio email footer" htmlFor="folio-footer">
              <textarea
                id="folio-footer"
                rows={4}
                value={settings.folio_email_footer ?? ""}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    folio_email_footer: e.target.value || null,
                  })
                }
              />
            </Field>
            <div className={styles.actions}>
              <Button
                type="submit"
                variant="primary"
                disabled={
                  saving || settings.enabled_payment_methods.length === 0
                }
              >
                {saving ? "Saving..." : "Save"}
              </Button>
              {saved && <span className={styles.saved}>Saved</span>}
            </div>
          </form>
        </Card>
      )}
    </>
  );
}
