import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";

import { api, type ApiError } from "../lib/api";

export function Signup() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fullName, setFullName] = useState("");
  const [orgName, setOrgName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api.signup({
        email,
        password,
        full_name: fullName,
        organization_name: orgName,
      });
      setDone(true);
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not create your account.");
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return (
      <div className="auth-shell">
        <div className="auth-stack">
          <h1 className="auth-wordmark">Liveaboard</h1>
          <div className="auth-card">
            <h1>Check your inbox</h1>
            <p>
              We've sent a verification link to <strong>{email}</strong>. Click
              it to activate your account, then sign in.
            </p>
            <p className="muted">
              <Link to="/login">Back to sign in</Link>
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-shell">
      <div className="auth-stack">
        <h1 className="auth-wordmark">Liveaboard</h1>
        <form className="auth-card" onSubmit={onSubmit}>
          <h1>Create an organization</h1>
          {error && <div className="error">{error}</div>}
          <div className="field">
            <label htmlFor="orgName">Organization name</label>
            <input
              id="orgName"
              type="text"
              autoComplete="organization"
              value={orgName}
              onChange={(e) => setOrgName(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label htmlFor="fullName">Your full name</label>
            <input
              id="fullName"
              type="text"
              autoComplete="name"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              required
            />
          </div>
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
              autoComplete="new-password"
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
            <p className="muted" style={{ fontSize: 12, marginTop: 6 }}>
              At least 8 characters with upper, lower, and a digit.
            </p>
          </div>
          <button className="primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
            {submitting ? "Creating…" : "Create organization"}
          </button>
          <p className="muted" style={{ marginTop: "var(--sp-md)" }}>
            Have an account? <Link to="/login">Sign in</Link>
          </p>
        </form>
      </div>
    </div>
  );
}
