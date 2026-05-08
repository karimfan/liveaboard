import { useEffect, useState, type FormEvent } from "react";

import { adminApi, type PaymentSettings } from "../api";

const currencies = ["USD", "EUR", "GBP", "AUD", "IDR", "THB", "SGD", "PHP", "JPY"];
const methods = [
  { value: "card", label: "Card" },
  { value: "cash", label: "Cash" },
  { value: "other", label: "Other" },
];

export function OrganizationPayments() {
  const [settings, setSettings] = useState<PaymentSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    adminApi.paymentSettings()
      .then(setSettings)
      .catch((err) => setError((err as { message?: string })?.message ?? "Failed to load payment settings."));
  }, []);

  function toggleCurrency(code: string) {
    if (!settings || code === "USD") return;
    const set = new Set(settings.supported_currencies);
    if (set.has(code)) set.delete(code);
    else set.add(code);
    const supported = Array.from(set).sort();
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
            <label>Supported currencies</label>
            <div className="toggle-grid">
              {currencies.map((code) => {
                const readiness = settings.rate_readiness.find((r) => r.currency === code);
                const enabled = settings.supported_currencies.includes(code);
                return (
                  <label key={code} className="check-row">
                    <input
                      type="checkbox"
                      checked={enabled}
                      disabled={code === "USD"}
                      onChange={() => toggleCurrency(code)}
                    />
                    {code}
                    {enabled && code !== "USD" && (
                      <span className={readiness?.ready ? "muted" : "error-inline"}>
                        {readiness?.ready ? "rate ready" : "rate needed"}
                      </span>
                    )}
                  </label>
                );
              })}
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
