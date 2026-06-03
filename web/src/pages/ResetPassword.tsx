import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { api, type ApiError } from "../lib/api";
import styles from "./auth.module.css";

export function ResetPassword() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const token = params.get("token") ?? "";
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (!token) {
      setError("Missing reset token.");
      return;
    }
    setSubmitting(true);
    try {
      await api.resetPassword(token, password);
      navigate("/");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not reset password.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className={styles.authShell}>
      <div className={styles.authStack}>
        <h1 className={styles.authWordmark}>Liveaboard</h1>
        <form className={styles.authCard} onSubmit={onSubmit}>
          <h1>Set a new password</h1>
          {error && <div className={styles.error}>{error}</div>}
          <div className={styles.field}>
            <label htmlFor="password">New password</label>
            <input
              id="password"
              type="password"
              autoComplete="new-password"
              minLength={8}
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
            {submitting ? "Saving…" : "Set new password"}
          </button>
          <p className={styles.muted} style={{ marginTop: "var(--sp-md)" }}>
            <Link to="/login">Back to sign in</Link>
          </p>
        </form>
      </div>
    </div>
  );
}
