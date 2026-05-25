import { useEffect, useState, type FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { api, type ApiError, type GuestInviteLookup } from "../lib/api";

export function GuestInvitation() {
  const navigate = useNavigate();
  const { token = "" } = useParams<{ token: string }>();
  const [invite, setInvite] = useState<GuestInviteLookup | null>(null);
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.lookupGuestInvite(token)
      .then((res) => !cancelled && setInvite(res))
      .catch((err: ApiError) => !cancelled && setError(err.message ?? "Registration link is not valid."));
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function accept(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const res = await api.acceptGuestInvite(token, password);
      navigate(`/guest/trips/${res.trip_guest_id}/register`, { replace: true });
    } catch (err) {
      setError((err as ApiError).message ?? "Could not start registration.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-shell">
      <div className="auth-stack guest-auth-stack">
        <h1 className="auth-wordmark">Liveaboard</h1>
        <form className="auth-card" onSubmit={accept}>
          <h1>{!invite ? "Trip registration" : invite.has_account ? "Sign in to continue" : "Create your account"}</h1>
          {error && <div className="error">{error}</div>}
          {!invite ? (
            !error && <p className="muted">Loading...</p>
          ) : (
            <>
              <p className="muted">
                {invite.organization_name} invited {invite.full_name} to register for {invite.boat_name}, {invite.start_date} to {invite.end_date}.
              </p>
              <p className="muted">
                {invite.has_account
                  ? "You already have a Liveaboard guest account for this email. Sign in to open your trip registration."
                  : "Choose a password to create your guest account. You'll then complete your trip registration form."}
              </p>
              <div className="field">
                <label>Email</label>
                <input value={invite.email} disabled />
              </div>
              <div className="field">
                <label htmlFor="guest-password">
                  {invite.has_account ? "Password" : "Choose a password"}
                </label>
                <input
                  id="guest-password"
                  type="password"
                  minLength={8}
                  autoComplete={invite.has_account ? "current-password" : "new-password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
                {!invite.has_account && (
                  <div className="muted" style={{ marginTop: "var(--sp-xs)", fontSize: "var(--fs-xs)" }}>
                    At least 8 characters, with one uppercase, one lowercase, and one digit.
                  </div>
                )}
              </div>
              <button className="primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
                {submitting
                  ? "Opening registration..."
                  : invite.has_account
                  ? "Sign in & continue"
                  : "Create account & continue"}
              </button>
            </>
          )}
        </form>
      </div>
    </div>
  );
}
