import { useEffect, useState, type FormEvent } from "react";

import { api, type ApiError, type PendingEmailChange } from "../../lib/api";
import { useMe } from "../useMe";
import { Button, Field, PageHeader } from "../components";

import styles from "./Account.module.css";

// Account is the per-user self-service page: edit profile (Sprint
// 010), change password, change email (two-phase, with a confirmation
// email), and view pending change-email state with a cancel button.

export function Account() {
  return (
    <>
      <PageHeader title="Account" />
      <MyProfileSection />
      <hr className={styles.divider} />
      <ChangePasswordSection />
      <hr className={styles.divider} />
      <ChangeEmailSection />
    </>
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
      <h2 className={styles.sectionTitle}>My profile</h2>
      <form onSubmit={onSubmit} className={styles.form}>
        {msg && <div className={styles.success}>{msg}</div>}
        {error && <div className={styles.error}>{error}</div>}
        <Field label="Full name" htmlFor="profile-name">
          <input
            id="profile-name"
            type="text"
            autoComplete="name"
            value={fullName}
            onChange={(e) => setFullName(e.target.value)}
            required
          />
        </Field>
        <Field label="Phone (optional)" htmlFor="profile-phone">
          <input
            id="profile-phone"
            type="tel"
            autoComplete="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
        </Field>
        <Field label="Email">
          <div className={styles.emailReadout}>
            {me.loaded && me.me ? me.me.email : "—"}
            <span className={styles.emailHint}>
              (change via the section below)
            </span>
          </div>
        </Field>
        <Button variant="primary" type="submit" disabled={submitting}>
          {submitting ? "Saving…" : "Save"}
        </Button>
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
      <h2 className={styles.sectionTitle}>Change password</h2>
      <form onSubmit={onSubmit} className={styles.form}>
        {msg && <div className={styles.success}>{msg}</div>}
        {error && <div className={styles.error}>{error}</div>}
        <Field label="Current password" htmlFor="current">
          <input
            id="current"
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            required
          />
        </Field>
        <Field label="New password" htmlFor="next">
          <input
            id="next"
            type="password"
            autoComplete="new-password"
            minLength={8}
            value={next}
            onChange={(e) => setNext(e.target.value)}
            required
          />
        </Field>
        <Button variant="primary" type="submit" disabled={submitting}>
          {submitting ? "Saving…" : "Update password"}
        </Button>
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
      <h2 className={styles.sectionTitle}>Change email</h2>
      {pending && (
        <div className={styles.callout}>
          <p>
            Pending verification at <strong>{pending.new_email}</strong>. The
            previous email remains active until that link is clicked.
          </p>
          <Button variant="quiet" onClick={onCancel} disabled={submitting}>
            Cancel pending change
          </Button>
        </div>
      )}
      <form onSubmit={onSubmit} className={styles.form}>
        {msg && <div className={styles.success}>{msg}</div>}
        {error && <div className={styles.error}>{error}</div>}
        <Field label="New email" htmlFor="newEmail">
          <input
            id="newEmail"
            type="email"
            value={newEmail}
            onChange={(e) => setNewEmail(e.target.value)}
            required
          />
        </Field>
        <Field label="Current password" htmlFor="emailpw">
          <input
            id="emailpw"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </Field>
        <Button variant="primary" type="submit" disabled={submitting}>
          {submitting ? "Sending…" : "Send confirmation"}
        </Button>
      </form>
    </section>
  );
}
