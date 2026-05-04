import { useEffect, useState, type ReactNode } from "react";
import { Navigate } from "react-router-dom";

import { api, type ApiError } from "./api";

type State = "checking" | "authed" | "guest";

/**
 * RequireSession guards routes that need an authenticated app session.
 * After Sprint 009 the only auth surface is our own cookie session, so
 * the resolution is simple: hit /api/me; if 200 we are authed, otherwise
 * we redirect to /login.
 */
export function RequireSession({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>("checking");

  useEffect(() => {
    let cancelled = false;
    api
      .me()
      .then(() => {
        if (!cancelled) setState("authed");
      })
      .catch((err) => {
        // Any error — typically `unauthenticated` — sends them to /login.
        // We swallow the specific code; the login page lets them retry.
        void (err as ApiError);
        if (!cancelled) setState("guest");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (state === "checking") return null;
  if (state === "guest") return <Navigate to="/login" replace />;
  return <>{children}</>;
}
