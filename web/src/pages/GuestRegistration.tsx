import { useEffect, useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";

import { api } from "../lib/api";
import {
  RegistrationSections,
  emptyRegistrationPayload,
  mergeRegistrationPayload,
  normalizeRegistrationPayload,
  type RegistrationPayload,
} from "../lib/registration";

export function GuestRegistration() {
  const { tripGuestId = "" } = useParams<{ tripGuestId: string }>();
  const [payload, setPayload] = useState<RegistrationPayload>(emptyRegistrationPayload);
  const [status, setStatus] = useState<"draft" | "submitted">("draft");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.guestRegistration(tripGuestId)
      .then((res) => {
        if (cancelled) return;
        setPayload(mergeRegistrationPayload(res.payload as RegistrationPayload));
        setStatus(res.status);
      })
      .catch((err) => !cancelled && setError((err as { message?: string })?.message ?? "Could not load registration."));
    return () => {
      cancelled = true;
    };
  }, [tripGuestId]);

  function update(section: string, field: string, value: unknown) {
    setPayload((prev) => ({
      ...prev,
      [section]: { ...(prev[section] ?? {}), [field]: value },
    }));
  }

  async function saveDraft() {
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      const res = await api.saveGuestRegistration(tripGuestId, normalizeRegistrationPayload(payload));
      setStatus(res.status);
      setMessage("Draft saved. You can return to this registration link later.");
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not save draft.");
    } finally {
      setSaving(false);
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      const res = await api.submitGuestRegistration(tripGuestId, normalizeRegistrationPayload(payload));
      setStatus(res.status);
      setMessage(status === "submitted" ? "Changes saved." : "Registration submitted.");
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not save registration.");
    } finally {
      setSaving(false);
    }
  }

  const alreadySubmitted = status === "submitted";

  return (
    <div className="guest-registration-shell">
      <form className="guest-registration" onSubmit={submit}>
        <div className="guest-registration__header">
          <div>
            <h1>Guest registration</h1>
            <p className="muted">
              {alreadySubmitted
                ? "You've submitted your registration. You can still update any details before your trip."
                : "Save a draft any time and return later from your registration link."}
            </p>
          </div>
          <span className="chip chip--active">{status}</span>
        </div>
        {error && <div className="error">{error}</div>}
        {message && <div className="callout">{message}</div>}

        <RegistrationSections mode="edit" payload={payload} onChange={update} />

        <div className="guest-registration__actions">
          {!alreadySubmitted && (
            <button type="button" className="secondary" onClick={saveDraft} disabled={saving}>
              Save draft
            </button>
          )}
          <button type="submit" className="primary" disabled={saving}>
            {saving
              ? "Saving..."
              : alreadySubmitted
              ? "Save changes"
              : "Submit registration"}
          </button>
        </div>
      </form>
    </div>
  );
}
