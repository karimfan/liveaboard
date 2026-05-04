import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";

import { api, type ApiError } from "../lib/api";

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
    <div className="auth-shell">
      <div className="auth-stack">
        <h1 className="auth-wordmark">Liveaboard</h1>
        <form className="auth-card" onSubmit={onSubmit}>
          <h1>Sign in</h1>
          {error && <div className="error">{error}</div>}
          {needsVerification && (
            <div className="error">
              Please verify your email before signing in.{" "}
              <button type="button" className="link" onClick={resend}>
                Resend verification
              </button>
            </div>
          )}
          <div className="field">
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
          <div className="field">
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
          <button className="primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
            {submitting ? "Signing in…" : "Sign in"}
          </button>
          <p className="muted" style={{ marginTop: "var(--sp-md)" }}>
            <Link to="/forgot-password">Forgot password?</Link>
          </p>
          <p className="muted">
            New here? <Link to="/signup">Create an organization</Link>
          </p>
        </form>
      </div>
    </div>
  );
}
