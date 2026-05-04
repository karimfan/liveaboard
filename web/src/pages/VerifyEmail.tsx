import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";

import { api, type ApiError } from "../lib/api";

type State = "verifying" | "ok" | "error";

export function VerifyEmail() {
  const [params] = useSearchParams();
  const token = params.get("token") ?? "";
  const [state, setState] = useState<State>("verifying");
  const [message, setMessage] = useState<string>("");

  // React 18 StrictMode mounts effects twice in dev. Verification is a
  // single-use token, so the second call would race the first and 404.
  // Guard with a ref so we only call /verify-email once per token.
  const fired = useRef(false);

  useEffect(() => {
    if (!token) {
      setState("error");
      setMessage("Missing verification token.");
      return;
    }
    if (fired.current) return;
    fired.current = true;

    api
      .verifyEmail(token)
      .then(() => setState("ok"))
      .catch((err: ApiError) => {
        setState("error");
        setMessage(err.message ?? "Verification failed.");
      });
  }, [token]);

  return (
    <div className="auth-shell">
      <div className="auth-stack">
        <h1 className="auth-wordmark">Liveaboard</h1>
        <div className="auth-card">
          {state === "verifying" && <h1>Verifying…</h1>}
          {state === "ok" && (
            <>
              <h1>Email verified</h1>
              <p>You can now sign in.</p>
              <p style={{ marginTop: "var(--sp-md)" }}>
                <Link to="/login" className="primary" style={{ display: "inline-block" }}>
                  Continue to sign in
                </Link>
              </p>
            </>
          )}
          {state === "error" && (
            <>
              <h1>Verification failed</h1>
              <p className="error" style={{ marginBottom: "var(--sp-md)" }}>
                {message}
              </p>
              <p className="muted">
                <Link to="/login">Back to sign in</Link>
              </p>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
