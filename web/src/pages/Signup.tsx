import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { SignedIn, SignedOut, SignUp, useAuth, useUser } from "@clerk/clerk-react";

import { api, type ApiError } from "../lib/api";

/**
 * Signup is a two-phase flow:
 *
 *   1. <SignedOut>: render Clerk's <SignUp /> component (chromeless via
 *      appearance overrides; our wordmark provides the page chrome).
 *
 *   2. <SignedIn>: the user has a Clerk identity but no org yet. Show
 *      our org-name form, POST /api/signup-complete with the Clerk JWT,
 *      and on success navigate to /.
 */
export function Signup() {
  return (
    <div className="auth-shell">
      <div className="auth-stack">
        <SignedOut>
          <h1 className="auth-wordmark">Liveaboard</h1>
          <SignUp routing="path" path="/signup" signInUrl="/login" />
          <p className="muted auth-aside">
            Already have an account? <Link to="/login">Log in</Link>
          </p>
        </SignedOut>
        <SignedIn>
          <FinishSignup />
        </SignedIn>
      </div>
    </div>
  );
}

function FinishSignup() {
  const navigate = useNavigate();
  const { getToken } = useAuth();
  const { user } = useUser();
  const [orgName, setOrgName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const inferredFullName = user
    ? [user.firstName, user.lastName].filter(Boolean).join(" ")
    : "";

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const jwt = await getToken();
      if (!jwt) throw { error: "no_token", message: "Clerk session missing." } as ApiError;
      await api.signupComplete(jwt, orgName, inferredFullName);
      navigate("/");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not create organization.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="auth-card" onSubmit={onSubmit}>
      <h1>Name your organization</h1>
      <p className="muted" style={{ marginBottom: "var(--sp-lg)" }}>
        You're signed in as {user?.primaryEmailAddress?.emailAddress}. Pick a
        name for your organization to finish setup.
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
      <button
        className="primary"
        type="submit"
        disabled={submitting || orgName.trim() === ""}
        style={{ width: "100%" }}
      >
        {submitting ? "Creating…" : "Create organization"}
      </button>
    </form>
  );
}
