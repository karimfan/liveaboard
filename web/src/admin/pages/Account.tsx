import { useEffect, useState, type FormEvent } from "react";

import { api, type ApiError, type PendingEmailChange } from "../../lib/api";
import { useMe } from "../useMe";

// Account is the per-user self-service page: edit profile (Sprint
// 010), change password, change email (two-phase, with a confirmation
// email), and view pending change-email state with a cancel button.

export function Account() {
  return (
    <div className="admin-page">
      <h1>Account</h1>
      <MyProfileSection />
      <hr style={{ margin: "var(--sp-xl) 0" }} />
      <ChangePasswordSection />
      <hr style={{ margin: "var(--sp-xl) 0" }} />
      <ChangeEmailSection />
    </div>
  );
}

function MyProfileSection() {
  const me = useMe();
  const [fullName, setFullName] = useState("");
  const [phone, setPhone] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Hydrate from useMe once it loads.
  useEffect(() => {
    if (me.loaded && me.me) {
      setFullName(me.me.full_name);
      setPhone(me.me.phone ?? "");
    }
  }, [me.loaded, me.me]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setMsg(null);
    setSubmitting(true);
    try {
      await api.updateProfile({
        full_name: fullName,
        phone: phone.trim() === "" ? null : phone.trim(),
      });
      // Refresh useMe so the sidebar / contact card see the change live.
      await me.refresh();
      setMsg("Profile updated.");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not update profile.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section>
      <h2>My profile</h2>
      <form onSubmit={onSubmit} style={{ maxWidth: 480 }}>
        {msg && <div className="success">{msg}</div>}
        {error && <div className="error">{error}</div>}
        <div className="field">
          <label htmlFor="profile-name">Full name</label>
          <input
            id="profile-name"
            type="text"
            autoComplete="name"
            value={fullName}
            onChange={(e) => setFullName(e.target.value)}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="profile-phone">Phone (optional)</label>
          <input
            id="profile-phone"
            type="tel"
            autoComplete="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </div>
        <div className="field">
          <label>Email</label>
          <div className="muted" style={{ padding: "8px 0", fontSize: 14 }}>
            {me.loaded && me.me ? me.me.email : "—"}
            <span style={{ marginLeft: 8, fontSize: 12 }}>
              (change via the section below)
            </span>
          </div>
        </div>
        <button className="primary" type="submit" disabled={submitting}>
          {submitting ? "Saving…" : "Save"}
        </button>
      </form>
    </section>
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
