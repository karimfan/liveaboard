import { useEffect, useState, type FormEvent } from "react";

import { api } from "../../lib/api";
import { adminApi, type PaymentSettings } from "../api";
import { CurrencyMultiPicker } from "../CurrencyPicker";

const methods = [
  { value: "card", label: "Card" },
  { value: "cash", label: "Cash" },
  { value: "other", label: "Other" },
];

function rateChipLabel(status: "fresh" | "stale" | "missing"): string {
  switch (status) {
    case "fresh": return "fresh";
    case "stale": return "stale";
    case "missing": return "rate needed";
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
    adminApi.paymentSettings()
      .then(setSettings)
      .catch((err) => setError((err as { message?: string })?.message ?? "Failed to load payment settings."));
    api.organization()
      .then((o) => setOrgCurrency(o.currency))
      .catch(() => { /* non-fatal: picker just won't lock the country currency */ });
  }, []);

  function setSupported(supported: string[]) {
    if (!settings) return;
    // USD is always supported.
    if (!supported.includes("USD")) supported = ["USD", ...supported].sort();
    const defaultCurrency = supported.includes(settings.default_currency) ? settings.default_currency : "USD";
    setSettings({ ...settings, supported_currencies: supported, default_currency: defaultCurrency });
  }

  function toggleMethod(method: string) {
    if (!settings) return;
    const set = new Set(settings.enabled_payment_methods);
    if (set.has(method)) set.delete(method);
    else set.add(method);
    setSettings({ ...settings, enabled_payment_methods: Array.from(set).sort() });
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
      setError((err as { message?: string })?.message ?? "Failed to save payment settings.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Payments</h1>
          <div className="admin-page-subtitle">Checkout currencies, offline methods, and card fee settings.</div>
        </div>
      </div>
      {error && <div className="error">{error}</div>}
      {!settings ? (
        <div className="muted">Loading...</div>
      ) : (
        <form className="admin-card settings-card" onSubmit={save}>
          <h2 className="admin-card__title">Checkout settings</h2>
          <div className="field">
            <label>Accepted currencies</label>
            <CurrencyMultiPicker
              value={settings.supported_currencies}
              onChange={setSupported}
              locked={orgCurrency ? ["USD", orgCurrency] : ["USD"]}
              placeholder="Add a currency…"
            />
            <div className="rate-readiness">
              {settings.supported_currencies
                .filter((c) => c !== "USD")
                .map((c) => {
                  const r = settings.rate_readiness.find((x) => x.currency === c);
                  const status = r?.status ?? "missing";
                  return (
                    <span key={c} className={`rate-chip rate-chip--${status}`}>
                      {c}: {rateChipLabel(status)}
                    </span>
                  );
                })}
            </div>
            <div className="muted" style={{ marginTop: "var(--sp-xs)" }}>
              Rates auto-refresh daily from the European Central Bank
              (via Frankfurter). USD is always supported; the country
              currency is locked here so a conversion target is
              always available.
              {settings.auto_refresh_at && (
                <> Last refresh {formatRefreshAgo(settings.auto_refresh_at)}.</>
              )}
            </div>
          </div>
          <div className="form-grid">
            <div className="field">
              <label htmlFor="default-currency">Default settlement currency</label>
              <select
                id="default-currency"
                value={settings.default_currency}
                onChange={(e) => setSettings({ ...settings, default_currency: e.target.value })}
              >
                {settings.supported_currencies.map((code) => <option key={code} value={code}>{code}</option>)}
              </select>
            </div>
            <div className="field">
              <label htmlFor="card-fee">Card fee percent</label>
              <input
                id="card-fee"
                type="number"
                min="0"
                max="20"
                step="0.01"
                value={(settings.card_fee_basis_points / 100).toFixed(2)}
                onChange={(e) => setSettings({ ...settings, card_fee_basis_points: Math.round(Number(e.target.value) * 100) })}
              />
            </div>
          </div>
          <div className="field">
            <label>Payment methods</label>
            <div className="toggle-grid">
              {methods.map((m) => (
                <label key={m.value} className="check-row">
                  <input
                    type="checkbox"
                    checked={settings.enabled_payment_methods.includes(m.value)}
                    onChange={() => toggleMethod(m.value)}
                  />
                  {m.label}
                </label>
              ))}
            </div>
          </div>
          <div className="field">
            <label htmlFor="folio-footer">Folio email footer</label>
            <textarea
              id="folio-footer"
              rows={4}
              value={settings.folio_email_footer ?? ""}
              onChange={(e) => setSettings({ ...settings, folio_email_footer: e.target.value || null })}
            />
          </div>
          <button className="primary" disabled={saving || settings.enabled_payment_methods.length === 0}>
            {saving ? "Saving..." : "Save"}
          </button>
          {saved && <span className="muted saved-inline">Saved</span>}
        </form>
      )}
    </>
  );
}
