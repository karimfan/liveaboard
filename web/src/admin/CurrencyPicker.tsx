import { useEffect, useMemo, useRef, useState } from "react";

import { adminApi } from "./api";
import styles from "./CurrencyPicker.module.css";

export type Currency = {
  code: string;
  name: string;
  exponent: number;
};

// Module-level cache for the currency catalog so multiple pickers on
// the same page only fetch once.
let cache: Currency[] | null = null;
let inFlight: Promise<Currency[]> | null = null;

export function useCurrencies(): {
  currencies: Currency[];
  loading: boolean;
  error: string | null;
} {
  const [currencies, setCurrencies] = useState<Currency[]>(cache ?? []);
  const [loading, setLoading] = useState(cache === null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    if (cache) {
      setCurrencies(cache);
      setLoading(false);
      return;
    }
    if (!inFlight) {
      inFlight = adminApi
        .listCurrencies()
        .then((r) => {
          cache = r.currencies;
          return cache;
        })
        .finally(() => {
          inFlight = null;
        });
    }
    inFlight
      .then((list) => {
        if (cancelled) return;
        setCurrencies(list);
        setLoading(false);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(e?.message ?? "Failed to load currencies.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return { currencies, loading, error };
}

function filterCurrencies(catalog: Currency[], query: string): Currency[] {
  const q = query.trim().toLowerCase();
  if (q === "") return catalog;
  return catalog.filter(
    (c) => c.code.toLowerCase().includes(q) || c.name.toLowerCase().includes(q),
  );
}

// CurrencyPicker is a searchable single-select. The user types a code
// or name; the dropdown filters live. Selecting a row commits.
// `value` and `onChange` work over the 3-letter code; empty string
// means "not set".
export function CurrencyPicker(props: {
  value: string;
  onChange: (code: string) => void;
  id?: string;
  allowClear?: boolean;
  placeholder?: string;
}) {
  const { currencies, loading, error } = useCurrencies();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, []);

  const selected = useMemo(
    () => currencies.find((c) => c.code === props.value),
    [currencies, props.value],
  );
  const filtered = useMemo(
    () => filterCurrencies(currencies, query).slice(0, 50),
    [currencies, query],
  );

  function commit(c: Currency) {
    props.onChange(c.code);
    setQuery("");
    setOpen(false);
  }

  const inputValue = open
    ? query
    : selected
      ? `${selected.code} — ${selected.name}`
      : "";

  return (
    <div className={styles.picker} ref={rootRef}>
      <input
        id={props.id}
        type="text"
        value={inputValue}
        placeholder={props.placeholder ?? "Search currency…"}
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
        }}
        autoComplete="off"
      />
      {props.allowClear && selected && (
        <button
          type="button"
          className={styles.clear}
          onClick={() => props.onChange("")}
          aria-label="Clear"
        >
          ×
        </button>
      )}
      {open && (
        <div className={styles.menu}>
          {loading && (
            <div className={`${styles.msg} ${styles.mutedText}`}>Loading…</div>
          )}
          {error && (
            <div className={`${styles.msg} ${styles.error}`}>{error}</div>
          )}
          {!loading && !error && filtered.length === 0 && (
            <div className={`${styles.msg} ${styles.mutedText}`}>No match.</div>
          )}
          {filtered.map((c) => (
            <button
              key={c.code}
              type="button"
              className={
                styles.row +
                (c.code === props.value ? ` ${styles.isSelected}` : "")
              }
              onClick={() => commit(c)}
            >
              <span className={styles.code}>{c.code}</span>
              <span className={styles.name}>{c.name}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// CurrencyMultiPicker is a searchable multi-select. The selected
// currencies render as removable chips above the search input.
// onChange receives the new array of 3-letter codes. `locked` codes
// (e.g. USD always-on, or the org's country currency) cannot be
// removed.
export function CurrencyMultiPicker(props: {
  value: string[];
  onChange: (codes: string[]) => void;
  locked?: string[];
  placeholder?: string;
}) {
  const { currencies, loading, error } = useCurrencies();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const lockedSet = useMemo(() => new Set(props.locked ?? []), [props.locked]);
  const valueSet = useMemo(() => new Set(props.value), [props.value]);

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, []);

  const filtered = useMemo(
    () =>
      filterCurrencies(currencies, query)
        .filter((c) => !valueSet.has(c.code))
        .slice(0, 50),
    [currencies, query, valueSet],
  );

  function add(c: Currency) {
    if (valueSet.has(c.code)) return;
    props.onChange([...props.value, c.code].sort());
    setQuery("");
  }
  function remove(code: string) {
    if (lockedSet.has(code)) return;
    props.onChange(props.value.filter((c) => c !== code));
  }

  return (
    <div className={styles.picker} ref={rootRef}>
      <div className={styles.chips}>
        {props.value.map((code) => {
          const meta = currencies.find((c) => c.code === code);
          const isLocked = lockedSet.has(code);
          return (
            <span
              key={code}
              className={styles.chip + (isLocked ? ` ${styles.isLocked}` : "")}
              title={isLocked ? `${code} is required` : ""}
            >
              {code}
              {meta && <span className={styles.mutedText}> — {meta.name}</span>}
              {!isLocked && (
                <button
                  type="button"
                  className={styles.chipX}
                  onClick={() => remove(code)}
                  aria-label={`Remove ${code}`}
                >
                  ×
                </button>
              )}
            </span>
          );
        })}
      </div>
      <input
        type="text"
        value={query}
        placeholder={props.placeholder ?? "Add a currency…"}
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
        }}
        autoComplete="off"
      />
      {open && (
        <div className={styles.menu}>
          {loading && (
            <div className={`${styles.msg} ${styles.mutedText}`}>Loading…</div>
          )}
          {error && (
            <div className={`${styles.msg} ${styles.error}`}>{error}</div>
          )}
          {!loading && !error && filtered.length === 0 && (
            <div className={`${styles.msg} ${styles.mutedText}`}>No match.</div>
          )}
          {filtered.map((c) => (
            <button
              key={c.code}
              type="button"
              className={styles.row}
              onClick={() => add(c)}
            >
              <span className={styles.code}>{c.code}</span>
              <span className={styles.name}>{c.name}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
