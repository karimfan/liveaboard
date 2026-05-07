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
          <h1>Trip registration</h1>
          {error && <div className="error">{error}</div>}
          {!invite ? (
            !error && <p className="muted">Loading...</p>
          ) : (
            <>
              <p className="muted">
                {invite.organization_name} invited {invite.full_name} to register for {invite.boat_name}, {invite.start_date} to {invite.end_date}.
              </p>
              <div className="field">
                <label>Email</label>
                <input value={invite.email} disabled />
              </div>
              <div className="field">
                <label htmlFor="guest-password">Password</label>
                <input
                  id="guest-password"
                  type="password"
                  minLength={8}
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
              <button className="primary" type="submit" disabled={submitting} style={{ width: "100%" }}>
                {submitting ? "Opening registration..." : "Continue"}
              </button>
            </>
          )}
        </form>
      </div>
    </div>
  );
}
