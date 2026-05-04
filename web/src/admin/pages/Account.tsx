import { useEffect, useState, type FormEvent } from "react";

import { api, type ApiError, type PendingEmailChange } from "../../lib/api";

// Account is the per-user self-service page: change password, change
// email (two-phase, with a confirmation email), and view pending
// change-email state with a cancel button.

export function Account() {
  return (
    <div className="admin-page">
      <h1>Account</h1>
      <ChangePasswordSection />
      <hr style={{ margin: "var(--sp-xl) 0" }} />
      <ChangeEmailSection />
    </div>
  );
}

function ChangePasswordSection() {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setMsg(null);
    setSubmitting(true);
    try {
      await api.changePassword(current, next);
      setMsg("Password updated.");
      setCurrent("");
      setNext("");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not change password.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section>
      <h2>Change password</h2>
      <form onSubmit={onSubmit} style={{ maxWidth: 480 }}>
        {msg && <div className="success">{msg}</div>}
        {error && <div className="error">{error}</div>}
        <div className="field">
          <label htmlFor="current">Current password</label>
          <input
            id="current"
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="next">New password</label>
          <input
            id="next"
            type="password"
            autoComplete="new-password"
            minLength={8}
            value={next}
            onChange={(e) => setNext(e.target.value)}
            required
          />
        </div>
        <button className="primary" type="submit" disabled={submitting}>
          {submitting ? "Saving…" : "Update password"}
        </button>
      </form>
    </section>
  );
}

function ChangeEmailSection() {
  const [pending, setPending] = useState<PendingEmailChange | null>(null);
  const [newEmail, setNewEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function refreshPending() {
    try {
      const { pending } = await api.pendingEmailChange();
      setPending(pending);
    } catch {
      // ignore — non-critical
    }
  }

  useEffect(() => {
    void refreshPending();
  }, []);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setMsg(null);
    setSubmitting(true);
    try {
      await api.requestEmailChange(newEmail, password);
      setMsg("Confirmation email sent. Check the new address to finish.");
      setNewEmail("");
      setPassword("");
      await refreshPending();
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not request email change.");
    } finally {
      setSubmitting(false);
    }
  }

  async function onCancel() {
    setSubmitting(true);
    try {
      await api.cancelEmailChange();
      await refreshPending();
      setMsg("Pending email change cancelled.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section>
      <h2>Change email</h2>
      {pending && (
        <div className="callout" style={{ maxWidth: 480, marginBottom: "var(--sp-md)" }}>
          <p>
            Pending verification at <strong>{pending.new_email}</strong>. The
            previous email remains active until that link is clicked.
          </p>
          <button className="ghost" onClick={onCancel} disabled={submitting}>
            Cancel pending change
          </button>
        </div>
      )}
      <form onSubmit={onSubmit} style={{ maxWidth: 480 }}>
        {msg && <div className="success">{msg}</div>}
        {error && <div className="error">{error}</div>}
        <div className="field">
          <label htmlFor="newEmail">New email</label>
          <input
            id="newEmail"
            type="email"
            value={newEmail}
            onChange={(e) => setNewEmail(e.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="emailpw">Current password</label>
          <input
            id="emailpw"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        <button className="primary" type="submit" disabled={submitting}>
          {submitting ? "Sending…" : "Send confirmation"}
        </button>
      </form>
    </section>
  );
}
