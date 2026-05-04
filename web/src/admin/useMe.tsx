import {
  createContext,
  useCallback,
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
  phone: string | null;
  role: "org_admin" | "cruise_director";
  organization_id: string;
};

type State =
  | { loaded: false; me: null; error: null; refresh: () => Promise<void> }
  | { loaded: true; me: Me; error: null; refresh: () => Promise<void> }
  | { loaded: true; me: null; error: string; refresh: () => Promise<void> };

const noopRefresh = async () => {};

const Ctx = createContext<State>({
  loaded: false,
  me: null,
  error: null,
  refresh: noopRefresh,
});

/**
 * MeProvider fetches /api/me once at mount and exposes the result via
 * useMe(). The exposed refresh() lets pages re-fetch after a profile
 * edit so the chrome (sidebar role label, contact card) reflects the
 * new state without a hard reload.
 */
export function MeProvider({ children }: { children: ReactNode }) {
  const [partial, setPartial] = useState<Omit<State, "refresh">>({
    loaded: false,
    me: null,
    error: null,
  });

  const refresh = useCallback(async () => {
    try {
      const m = await api.me();
      setPartial({
        loaded: true,
        me: {
          id: m.id,
          email: m.email,
          full_name: m.full_name,
          phone: m.phone,
          role: m.role,
          organization_id: m.organization_id,
        },
        error: null,
      });
    } catch (e) {
      let msg = "Failed to load profile";
      if (e && typeof e === "object" && "message" in e) {
        msg = String((e as { message: unknown }).message);
      }
      setPartial({ loaded: true, me: null, error: msg });
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const value = { ...partial, refresh } as State;
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
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
