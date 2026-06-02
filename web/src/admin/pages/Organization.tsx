import { useEffect, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";

import { api } from "../../lib/api";
import { adminApi } from "../api";
import { CurrencyPicker } from "../CurrencyPicker";
import { Button, Card, Field, PageHeader } from "../components";

import styles from "./Organization.module.css";

type OrgView = {
  id: string;
  name: string;
  currency: string | null;
  created_at: string;
};

export function Organization() {
  const navigate = useNavigate();
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
        const view = {
          id: o.id,
          name: o.name,
          currency: o.currency,
          created_at: o.created_at,
        };
        setOrg(view);
        setName(view.name);
        setCurrency(view.currency ?? "");
      })
      .catch(
        (e) => !cancelled && setError(e?.message ?? "Failed to load org."),
      );
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
      navigate("/admin", { replace: true });
    } catch (e) {
      setError((e as { message?: string })?.message ?? "Save failed.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <PageHeader
        title="Organization"
        subtitle="Org profile, currency, and defaults."
      />

      {error && <div className={styles.error}>{error}</div>}
      {!org ? (
        <div className={styles.loading}>Loading…</div>
      ) : (
        <Card title="Profile" className={styles.card}>
          <form className={styles.form} onSubmit={onSave}>
            <Field label="Organization name" htmlFor="org-name">
              <input
                id="org-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </Field>
            <Field label="Country currency" htmlFor="org-currency">
              <CurrencyPicker
                id="org-currency"
                value={currency}
                onChange={setCurrency}
                allowClear
                placeholder="Search currency…"
              />
              <div className={styles.hint}>
                The organization's local currency. Reports headline in USD; this
                currency is automatically added to your accepted checkout
                currencies on Payments.
              </div>
            </Field>
            <Field label="Created">
              <div className={styles.readonly}>
                {new Date(org.created_at).toLocaleDateString()}
              </div>
            </Field>
            <div className={styles.actions}>
              <Button
                variant="primary"
                type="submit"
                disabled={submitting || name.trim() === ""}
              >
                {submitting ? "Saving…" : "Save"}
              </Button>
              {saved && <span className={styles.savedNote}>✓ Saved</span>}
            </div>
          </form>
        </Card>
      )}
    </>
  );
}
