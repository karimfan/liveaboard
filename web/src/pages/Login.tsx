import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";

import { api, type ApiError } from "../lib/api";

export function Login() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api.login(email, password);
      navigate("/");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Login failed.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-shell">
      <form className="auth-card" onSubmit={onSubmit}>
        <h1>Log in</h1>
        {error && <div className="error">{error}</div>}
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
          {submitting ? "Signing in…" : "Log in"}
        </button>
        <p className="muted" style={{ marginTop: 16, textAlign: "center" }}>
          New here? <Link to="/signup">Create an organization</Link>
        </p>
      </form>
    </div>
  );
}
