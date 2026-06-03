import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";

import { api, type ApiError } from "../lib/api";
import styles from "./auth.module.css";

export function Login() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [needsVerification, setNeedsVerification] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setNeedsVerification(false);
    setSubmitting(true);
    try {
      await api.login(email, password);
      navigate("/");
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr.error === "verification_required") {
        setNeedsVerification(true);
      } else {
        setError(apiErr.message ?? "Could not sign in.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function resend() {
    try {
      await api.resendVerification(email);
      setError("Verification email sent — check your inbox.");
      setNeedsVerification(false);
    } catch {
      setError("Could not resend verification email.");
    }
  }

  return (
    <div className={styles.authShell}>
      <div className={styles.authStack}>
        <h1 className={styles.authWordmark}>Liveaboard</h1>
        <form className={styles.authCard} onSubmit={onSubmit}>
          <h1>Sign in</h1>
          {error && <div className={styles.error}>{error}</div>}
          {needsVerification && (
            <div className={styles.error}>
              Please verify your email before signing in.{" "}
              <button type="button" className="link" onClick={resend}>
                Resend verification
              </button>
            </div>
          )}
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
          <div className={styles.field}>
            <label htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
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
            {submitting ? "Signing in…" : "Sign in"}
          </button>
          <p className={styles.muted} style={{ marginTop: "var(--sp-md)" }}>
            <Link to="/forgot-password">Forgot password?</Link>
          </p>
          <p className={styles.muted}>
            New here? <Link to="/signup">Create an organization</Link>
          </p>
        </form>
      </div>
    </div>
  );
}
