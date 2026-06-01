import { useEffect, useState } from "react";

import { appConfig } from "./config";

export type DevFlags = {
  filesystem_email: boolean;
};

const ALL_FALSE: DevFlags = {
  filesystem_email: false,
};

// Module-level cache so multiple components on the same page only
// fetch /api/dev/flags once.
let cache: DevFlags | null = null;
let inFlight: Promise<DevFlags> | null = null;

async function fetchFlags(): Promise<DevFlags> {
  if (cache) return cache;
  if (!inFlight) {
    inFlight = fetch(`${appConfig.apiBase}/dev/flags`, { credentials: "include" })
      .then((r) => r.json() as Promise<DevFlags>)
      .then((r) => {
        cache = r;
        return r;
      })
      .catch(() => {
        // Non-fatal: on failure assume no dev affordances.
        return ALL_FALSE;
      })
      .finally(() => {
        inFlight = null;
      });
  }
  return inFlight;
}

// useDevFlags returns the current dev-mode flags. Defaults to all-
// false until the fetch resolves so dev-only UI never flashes in
// production builds.
export function useDevFlags(): DevFlags {
  const [flags, setFlags] = useState<DevFlags>(cache ?? ALL_FALSE);
  useEffect(() => {
    let cancelled = false;
    void fetchFlags().then((f) => {
      if (!cancelled) setFlags(f);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  return flags;
}
