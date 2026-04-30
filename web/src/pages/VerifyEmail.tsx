import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { api, type ApiError } from "../lib/api";

type State = "verifying" | "ok" | "error";

export function VerifyEmail() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [state, setState] = useState<State>("verifying");
  const [message, setMessage] = useState<string>("");

  useEffect(() => {
    const token = params.get("token");
    if (!token) {
      setState("error");
      setMessage("Missing token.");
      return;
    }
    api
      .verifyEmail(token)
      .then(() => setState("ok"))
      .catch((err: ApiError) => {
        setState("error");
        setMessage(err.message ?? "Verification failed.");
      });
  }, [params]);

  return (
    <div className="auth-shell">
      <div className="auth-card">
        {state === "verifying" && <h1>Verifying…</h1>}
        {state === "ok" && (
          <>
            <h1>Email verified</h1>
            <p className="muted">You can now log in.</p>
            <button className="primary" onClick={() => navigate("/login")}>
              Continue to login
            </button>
          </>
        )}
        {state === "error" && (
          <>
            <h1>Couldn't verify</h1>
            <div className="error">{message}</div>
            <Link to="/signup">Start over</Link>
          </>
        )}
      </div>
    </div>
  );
}
