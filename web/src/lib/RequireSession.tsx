import { useEffect, useState, type ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "@clerk/clerk-react";

import { api, type ApiError } from "./api";

type State = "checking" | "authed" | "guest" | "needs-signup";

/**
 * RequireSession guards routes that need an authenticated app session.
 *
 * Resolution order:
 *  1. Try /api/me (cookie auth). If 200 -> authed.
 *  2. If 401 and Clerk says the browser is signed in: getToken() and
 *     POST /api/auth/exchange to mint lb_session, then re-try /api/me.
 *  3. If exchange returns membership_pending -> redirect to /signup so
 *     the org-name step can run.
 *  4. Anything else -> redirect to /login.
 */
export function RequireSession({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>("checking");
  const { isLoaded: clerkLoaded, isSignedIn, getToken } = useAuth();

  useEffect(() => {
    let cancelled = false;

    async function resolve() {
      try {
        await api.me();
        if (!cancelled) setState("authed");
        return;
      } catch (err) {
        const apiErr = err as ApiError;
        if (apiErr.error !== "unauthenticated") {
          if (!cancelled) setState("guest");
          return;
        }
      }

      // Cookie missing/invalid. Try the exchange path if Clerk has a session.
      if (!clerkLoaded) return;
      if (!isSignedIn) {
        if (!cancelled) setState("guest");
        return;
      }
      try {
        const jwt = await getToken();
        if (!jwt) {
          if (!cancelled) setState("guest");
          return;
        }
        await api.exchange(jwt);
        await api.me();
        if (!cancelled) setState("authed");
      } catch (err) {
        const apiErr = err as ApiError;
        if (apiErr.error === "membership_pending") {
          if (!cancelled) setState("needs-signup");
        } else {
          if (!cancelled) setState("guest");
        }
      }
    }

    resolve();
    return () => {
      cancelled = true;
    };
  }, [clerkLoaded, isSignedIn, getToken]);

  if (state === "checking") return null;
  if (state === "needs-signup") return <Navigate to="/signup" replace />;
  if (state === "guest") return <Navigate to="/login" replace />;
  return <>{children}</>;
}
