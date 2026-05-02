import { useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { SignedIn, SignedOut, SignIn, useAuth } from "@clerk/clerk-react";

import { api } from "../lib/api";

/**
 * Login renders Clerk's <SignIn /> component for unsigned-in users.
 * Once Clerk authenticates, the inner <Bridge /> exchanges the JWT for
 * an lb_session cookie and navigates to the dashboard.
 */
export function Login() {
  return (
    <div className="auth-shell">
      <SignedOut>
        <div className="auth-card">
          <h1>Log in</h1>
          <SignIn routing="path" path="/login" signUpUrl="/signup" />
          <p className="muted" style={{ marginTop: 16, textAlign: "center" }}>
            New here? <Link to="/signup">Create an organization</Link>
          </p>
        </div>
      </SignedOut>
      <SignedIn>
        <Bridge />
      </SignedIn>
    </div>
  );
}

function Bridge() {
  const navigate = useNavigate();
  const { getToken } = useAuth();

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const jwt = await getToken();
        if (!jwt) return;
        await api.exchange(jwt);
        if (!cancelled) navigate("/");
      } catch (err) {
        // membership_pending -> push to signup; everything else falls
        // through to the RequireSession-driven flow on next render.
        const e = err as { error?: string };
        if (!cancelled && e.error === "membership_pending") {
          navigate("/signup");
        } else if (!cancelled) {
          navigate("/");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getToken, navigate]);

  return (
    <div className="auth-card">
      <h1>Signing in…</h1>
      <p className="muted">Restoring your session.</p>
    </div>
  );
}
