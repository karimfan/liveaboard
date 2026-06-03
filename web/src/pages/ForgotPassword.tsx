import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";

import { api } from "../lib/api";
import styles from "./auth.module.css";

export function ForgotPassword() {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await api.forgotPassword(email);
    } finally {
      setSubmitting(false);
      setDone(true);
    }
  }

  return (
    <div className={styles.authShell}>
      <div className={styles.authStack}>
        <h1 className={styles.authWordmark}>Liveaboard</h1>
        {done ? (
          <div className={styles.authCard}>
            <h1>Check your inbox</h1>
            <p>
              If an account exists for that email, a reset link is on its way.
            </p>
            <p className={styles.muted}>
              <Link to="/login">Back to sign in</Link>
            </p>
          </div>
        ) : (
          <form className={styles.authCard} onSubmit={onSubmit}>
            <h1>Forgot your password?</h1>
            <p
              className={styles.muted}
              style={{ marginBottom: "var(--sp-md)" }}
            >
              Enter your email and we'll send you a link to set a new one.
            </p>
            <div className={styles.field}>
              <label htmlFor="email">Email</label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <button
              className="primary"
              type="submit"
              disabled={submitting}
              style={{ width: "100%" }}
            >
              {submitting ? "Sending…" : "Send reset link"}
            </button>
            <p className={styles.muted} style={{ marginTop: "var(--sp-md)" }}>
              <Link to="/login">Back to sign in</Link>
            </p>
          </form>
        )}
      </div>
    </div>
  );
}
