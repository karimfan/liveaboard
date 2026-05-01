import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";

import { api, type ApiError } from "../lib/api";

export function Signup() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fullName, setFullName] = useState("");
  const [orgName, setOrgName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [verificationToken, setVerificationToken] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const result = await api.signup({
        email,
        password,
        full_name: fullName,
        organization_name: orgName,
      });
      if (result.verification_token) {
        setVerificationToken(result.verification_token);
      } else {
        navigate("/login");
      }
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Signup failed.");
    } finally {
      setSubmitting(false);
    }
  }

  if (verificationToken) {
    return (
      <div className="auth-shell">
        <div className="auth-card">
          <h1>Verify your email</h1>
          <p className="muted">
            In production we email a verification link. For local development the token is
            shown below — paste it on the verify page or click through.
          </p>
          <div className="callout">{verificationToken}</div>
          <Link to={`/verify-email?token=${encodeURIComponent(verificationToken)}`}>
            <button className="primary">Continue</button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-shell">
      <form className="auth-card" onSubmit={onSubmit}>
        <h1>Create your organization</h1>
        <p className="muted" style={{ marginBottom: 24 }}>
          You'll be the first Organization Admin.
        </p>

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
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
          <div className="muted" style={{ marginTop: 4 }}>
            8+ characters, at least one upper, one lower, one digit.
          </div>
        </div>

        <button className="primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
          {submitting ? "Creating…" : "Create organization"}
        </button>

        <p className="muted" style={{ marginTop: 16, textAlign: "center" }}>
          Already have an account? <Link to="/login">Log in</Link>
        </p>
      </form>
    </div>
  );
}
