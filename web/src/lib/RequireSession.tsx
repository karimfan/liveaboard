import { useEffect, useState, type ReactNode } from "react";
import { Navigate } from "react-router-dom";

import { api } from "./api";

type State = "checking" | "authed" | "guest";

export function RequireSession({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>("checking");

  useEffect(() => {
    let cancelled = false;
    api
      .me()
      .then(() => {
        if (!cancelled) setState("authed");
      })
      .catch(() => {
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
