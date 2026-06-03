import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { api, type ApiError, type InvitationLookup } from "../lib/api";
import styles from "./auth.module.css";

export function AcceptInvitation() {
  const navigate = useNavigate();
  const { token = "" } = useParams<{ token: string }>();

  const [invitation, setInvitation] = useState<InvitationLookup | null>(null);
  const [lookupErr, setLookupErr] = useState<string | null>(null);

  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) {
      setLookupErr("Missing invitation token.");
      return;
    }
    let cancelled = false;
    api
      .lookupInvitation(token)
      .then((inv) => {
        if (!cancelled) setInvitation(inv);
      })
      .catch((err: ApiError) => {
        if (!cancelled) setLookupErr(err.message ?? "Invitation not found.");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      // Sprint 010: name + phone come from the invitation row (the
      // admin captured them at invite time). We only ask for a
      // password here.
      await api.acceptInvitation(token, password);
      navigate("/admin");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not accept invitation.");
    } finally {
      setSubmitting(false);
    }
  }

  if (lookupErr) {
    return (
      <div className={styles.authShell}>
        <div className={styles.authStack}>
          <h1 className={styles.authWordmark}>Liveaboard</h1>
          <div className={styles.authCard}>
            <h1>Invitation not valid</h1>
            <p className={styles.error}>{lookupErr}</p>
            <p className={styles.muted}>
              <Link to="/login">Back to sign in</Link>
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (!invitation) {
    return (
      <div className={styles.authShell}>
        <div className={styles.authStack}>
          <h1 className={styles.authWordmark}>Liveaboard</h1>
          <div className={styles.authCard}>
            <h1>Loading…</h1>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.authShell}>
      <div className={styles.authStack}>
        <h1 className={styles.authWordmark}>Liveaboard</h1>
        <form className={styles.authCard} onSubmit={onSubmit}>
          <h1>Hi {invitation.full_name}.</h1>
          <p className={styles.muted} style={{ marginBottom: "var(--sp-md)" }}>
            You've been invited to{" "}
            <strong>{invitation.organization_name}</strong> as a{" "}
            {invitation.role.replace("_", " ")}. Set a password to finish
            joining. You can update your details from your account page later.
          </p>
          {error && <div className={styles.error}>{error}</div>}
          <div className={styles.field}>
            <label htmlFor="email">Email</label>
            <input id="email" type="email" value={invitation.email} disabled />
          </div>
          <div className={styles.field}>
            <label htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              autoComplete="new-password"
              minLength={8}
              autoFocus
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          <button
            className="primary"
            type="submit"
            disabled={submitting}
            style={{ width: "100%" }}
          >
            {submitting ? "Joining…" : "Accept invitation"}
          </button>
        </form>
      </div>
    </div>
  );
}
