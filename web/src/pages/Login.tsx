import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { SignedIn, SignedOut, SignIn, useAuth } from "@clerk/clerk-react";

import { api } from "../lib/api";

/**
 * Login renders Clerk's <SignIn /> component for unsigned-in users.
 * Once Clerk authenticates, the inner <Bridge /> exchanges the JWT for
 * an lb_session cookie and navigates to the dashboard.
 *
 * The wordmark above is our chrome; Clerk's component renders without
 * its own card thanks to the appearance overrides in clerkAppearance.ts.
 */
export function Login() {
  return (
    <div className="auth-shell">
      <div className="auth-stack">
        <SignedOut>
          <h1 className="auth-wordmark">Liveaboard</h1>
          <SignIn routing="path" path="/login" signUpUrl="/signup" />
        </SignedOut>
        <SignedIn>
          <Bridge />
        </SignedIn>
      </div>
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
