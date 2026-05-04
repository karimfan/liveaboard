import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { api, type ApiError, type InvitationLookup } from "../lib/api";

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
      <div className="auth-shell">
        <div className="auth-stack">
          <h1 className="auth-wordmark">Liveaboard</h1>
          <div className="auth-card">
            <h1>Invitation not valid</h1>
            <p className="error">{lookupErr}</p>
            <p className="muted">
              <Link to="/login">Back to sign in</Link>
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (!invitation) {
    return (
      <div className="auth-shell">
        <div className="auth-stack">
          <h1 className="auth-wordmark">Liveaboard</h1>
          <div className="auth-card">
            <h1>Loading…</h1>
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
          <h1>Hi {invitation.full_name}.</h1>
          <p className="muted" style={{ marginBottom: "var(--sp-md)" }}>
            You've been invited to <strong>{invitation.organization_name}</strong>{" "}
            as a {invitation.role.replace("_", " ")}. Set a password to finish
            joining. You can update your details from your account page later.
          </p>
          {error && <div className="error">{error}</div>}
          <div className="field">
            <label htmlFor="email">Email</label>
            <input id="email" type="email" value={invitation.email} disabled />
          </div>
          <div className="field">
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
          <button className="primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
            {submitting ? "Joining…" : "Accept invitation"}
          </button>
        </form>
      </div>
    </div>
  );
}
