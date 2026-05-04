import { useEffect, useState, type FormEvent } from "react";

import { api } from "../../lib/api";
import { adminApi } from "../api";

type OrgView = {
  id: string;
  name: string;
  currency: string | null;
  created_at: string;
};

export function Organization() {
  const [org, setOrg] = useState<OrgView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // Form fields
  const [name, setName] = useState("");
  const [currency, setCurrency] = useState("");

  useEffect(() => {
    let cancelled = false;
    api
      .organization()
      .then((o) => {
        if (cancelled) return;
        const view = { id: o.id, name: o.name, currency: o.currency, created_at: o.created_at };
        setOrg(view);
        setName(view.name);
        setCurrency(view.currency ?? "");
      })
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load org."));
    return () => {
      cancelled = true;
    };
  }, []);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSaved(false);
    setSubmitting(true);
    try {
      const updated = await adminApi.patchOrganization({
        name,
        currency: currency.trim() === "" ? null : currency,
      });
      setOrg({
        id: updated.id,
        name: updated.name,
        currency: updated.currency,
        created_at: updated.created_at,
      });
      setSaved(true);
    } catch (e) {
      setError((e as { message?: string })?.message ?? "Save failed.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Organization</h1>
          <div className="admin-page-subtitle">
            Org profile, currency, and defaults.
          </div>
        </div>
      </div>

      {error && <div className="error">{error}</div>}
      {!org ? (
        <div className="muted">Loading…</div>
      ) : (
        <form className="admin-card" onSubmit={onSave} style={{ maxWidth: 560 }}>
          <h2 className="admin-card__title">Profile</h2>
          <div className="field">
            <label htmlFor="org-name">Organization name</label>
            <input
              id="org-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label htmlFor="org-currency">Default currency</label>
            <select
              id="org-currency"
              value={currency}
              onChange={(e) => setCurrency(e.target.value)}
            >
              <option value="">— not set —</option>
              <option value="USD">USD — US dollar</option>
              <option value="EUR">EUR — Euro</option>
              <option value="AUD">AUD — Australian dollar</option>
              <option value="GBP">GBP — British pound</option>
            </select>
            <div className="muted" style={{ marginTop: "var(--sp-xs)" }}>
              Multi-currency support is on the roadmap; today an org has one default.
            </div>
          </div>
          <div className="field">
            <label>Created</label>
            <div className="muted">
              {new Date(org.created_at).toLocaleDateString()}
            </div>
          </div>
          <button
            className="primary"
            type="submit"
            disabled={submitting || name.trim() === ""}
            style={{ marginTop: "var(--sp-sm)" }}
          >
            {submitting ? "Saving…" : "Save"}
          </button>
          {saved && (
            <span className="muted" style={{ marginLeft: "var(--sp-md)" }}>
              ✓ Saved
            </span>
          )}
        </form>
      )}
    </>
  );
}
