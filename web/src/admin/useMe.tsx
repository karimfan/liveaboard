import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

import { api } from "../lib/api";

export type Me = {
  id: string;
  email: string;
  full_name: string;
  role: "org_admin" | "site_director";
  organization_id: string;
};

type State =
  | { loaded: false; me: null; error: null }
  | { loaded: true; me: Me; error: null }
  | { loaded: true; me: null; error: string };

const Ctx = createContext<State>({ loaded: false, me: null, error: null });

/**
 * MeProvider fetches /api/me once at mount and exposes the result via
 * useMe(). Used by the admin chrome to render role-gated nav items.
 */
export function MeProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ loaded: false, me: null, error: null });

  useEffect(() => {
    let cancelled = false;
    api
      .me()
      .then((m) => {
        if (cancelled) return;
        setState({
          loaded: true,
          me: {
            id: m.id,
            email: m.email,
            full_name: m.full_name,
            role: m.role as Me["role"],
            organization_id: m.organization_id,
          },
          error: null,
        });
      })
      .catch((e) => {
        if (cancelled) return;
        const msg =
          (e && typeof e === "object" && "message" in e && (e as { message: string }).message) ||
          "Failed to load profile";
        setState({ loaded: true, me: null, error: msg });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return <Ctx.Provider value={state}>{children}</Ctx.Provider>;
}

export function useMe(): State {
  return useContext(Ctx);
}

export function useMeOrThrow(): Me {
  const s = useMe();
  if (!s.loaded || !s.me) {
    throw new Error("useMeOrThrow called before MeProvider loaded");
  }
  return s.me;
}
