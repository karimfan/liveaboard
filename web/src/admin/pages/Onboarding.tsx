import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { api } from "../../lib/api";
import {
  adminApi,
  type ImportJob,
  type OnboardingState,
  type OnboardingStep,
  type OnboardingBoatWithoutLayout,
} from "../api";
import { CurrencyPicker } from "../CurrencyPicker";

type StepKey = OnboardingStep["key"];

const STEP_ORDER: StepKey[] = ["currency", "boats", "layouts", "directors"];

const STEP_INTRO: Record<StepKey, string> = {
  currency:
    "Pick your country currency. Reports headline in USD, and this currency is automatically added to the checkout currencies you accept.",
  boats:
    "Add the boats you operate. Import from liveaboard.com, upload a spreadsheet, or open the Fleet page to add one by hand.",
  layouts:
    "Lay out cabins and berths for each boat. Guests can't be added to a trip until their boat has at least one cabin with a berth.",
  directors:
    "Invite the Cruise Directors who run trips. You can always invite more later.",
};

export function Onboarding() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const [state, setState] = useState<OnboardingState | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      setState(await adminApi.onboarding());
    } catch (e) {
      setError((e as { message?: string })?.message ?? "Failed to load onboarding state.");
    }
  }

  useEffect(() => {
    void load();
  }, []);

  // Refresh state when the window regains focus so returning from a
  // deep-linked editor updates the layouts/boats/directors steps.
  useEffect(() => {
    function onFocus() {
      void load();
    }
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, []);

  const stepKey = useMemo<StepKey>(() => {
    const raw = params.get("step");
    if (raw && (STEP_ORDER as string[]).includes(raw)) return raw as StepKey;
    if (state) {
      const next = state.steps.find((s) => !s.done);
      if (next) return next.key;
    }
    return "currency";
  }, [params, state]);

  function setStep(next: StepKey) {
    const p = new URLSearchParams(params);
    p.set("step", next);
    setParams(p, { replace: false });
  }

  function advance() {
    const idx = STEP_ORDER.indexOf(stepKey);
    if (idx < 0 || idx === STEP_ORDER.length - 1) {
      // Last step → land on Overview.
      navigate("/admin");
      return;
    }
    setStep(STEP_ORDER[idx + 1]);
  }

  async function skipAll() {
    try {
      await adminApi.dismissOnboarding();
    } catch (e) {
      setError((e as { message?: string })?.message ?? "Could not dismiss onboarding.");
      return;
    }
    navigate("/admin", { replace: true });
  }

  if (error) {
    return (
      <div className="onboarding">
        <div className="error">{error}</div>
      </div>
    );
  }
  if (!state) {
    return (
      <div className="onboarding">
        <div className="muted">Loading…</div>
      </div>
    );
  }

  return (
    <div className="onboarding">
      <header className="onboarding__header">
        <div>
          <h1 className="admin-page-title">Get your organization set up</h1>
          <div className="admin-page-subtitle">
            Four steps. You can skip any of them and come back from the
            Overview.
          </div>
        </div>
        <button type="button" className="ghost" onClick={() => void skipAll()}>
          Skip all
        </button>
      </header>

      <Stepper state={state} current={stepKey} onPick={setStep} />

      <section className="onboarding__step">
        <h2>{state.steps.find((s) => s.key === stepKey)?.label}</h2>
        <p className="muted">{STEP_INTRO[stepKey]}</p>
        {stepKey === "currency" && (
          <CurrencyStep state={state} onSaved={() => void load().then(advance)} />
        )}
        {stepKey === "boats" && (
          <BoatsStep
            state={state}
            onImportFinished={() => {
              // Drop the ?job param and advance to layouts.
              const p = new URLSearchParams(params);
              p.delete("job");
              p.set("step", "layouts");
              setParams(p, { replace: true });
              void load();
            }}
          />
        )}
        {stepKey === "layouts" && <LayoutsStep state={state} />}
        {stepKey === "directors" && <DirectorsStep />}
      </section>

      <footer className="onboarding__footer">
        <button type="button" className="ghost" onClick={advance}>
          Skip this step
        </button>
        <button
          type="button"
          className="primary"
          onClick={advance}
          disabled={!state.steps.find((s) => s.key === stepKey)?.done && stepKey !== "currency"}
        >
          {stepKey === STEP_ORDER[STEP_ORDER.length - 1] ? "Finish" : "Continue"}
        </button>
      </footer>
    </div>
  );
}

function Stepper({
  state,
  current,
  onPick,
}: {
  state: OnboardingState;
  current: StepKey;
  onPick: (k: StepKey) => void;
}) {
  return (
    <ol className="onboarding__stepper">
      {state.steps.map((s, i) => (
        <li
          key={s.key}
          className={
            "stepper__item" +
            (s.done ? " is-done" : "") +
            (s.key === current ? " is-current" : "")
          }
        >
          <button type="button" onClick={() => onPick(s.key)} className="stepper__btn">
            <span className="stepper__num">{s.done ? "✓" : i + 1}</span>
            <span className="stepper__label">{s.label}</span>
            {s.hint && <span className="stepper__hint">{s.hint}</span>}
          </button>
        </li>
      ))}
    </ol>
  );
}

// --- Step views ---

function CurrencyStep({ state, onSaved }: { state: OnboardingState; onSaved: () => void }) {
  const [currency, setCurrency] = useState<string>("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const done = state.steps.find((s) => s.key === "currency")?.done;
  const currentCurrency = state.steps.find((s) => s.key === "currency")?.hint;

  // Pre-seed from the current value or a best-effort locale guess.
  useEffect(() => {
    if (currentCurrency && currentCurrency.length === 3) {
      setCurrency(currentCurrency);
      return;
    }
    const guess = guessCurrencyFromLocale();
    if (guess) setCurrency(guess);
  }, [currentCurrency]);

  async function save(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      // Reuse the existing organization patch endpoint. Fetch the org
      // first so we don't lose its name.
      const org = await api.organization();
      await adminApi.patchOrganization({ name: org.name, currency: currency || null });
      onSaved();
    } catch (e) {
      setError((e as { message?: string })?.message ?? "Could not save currency.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="onboarding__form" onSubmit={save}>
      {done && (
        <div className="muted" style={{ marginBottom: "var(--sp-sm)" }}>
          Already set to {currentCurrency}. You can change it here or move on.
        </div>
      )}
      <div className="field">
        <label htmlFor="onboarding-currency">Country currency</label>
        <CurrencyPicker
          id="onboarding-currency"
          value={currency}
          onChange={setCurrency}
          allowClear={false}
          placeholder="Search currency…"
        />
      </div>
      {error && <div className="error">{error}</div>}
      <button className="primary" type="submit" disabled={submitting || currency === ""}>
        {submitting ? "Saving…" : "Save and continue"}
      </button>
    </form>
  );
}

function BoatsStep({
  state,
  onImportFinished,
}: {
  state: OnboardingState;
  onImportFinished: () => void;
}) {
  const [params] = useSearchParams();
  const jobId = params.get("job");
  const done = state.steps.find((s) => s.key === "boats")?.done;

  if (jobId) {
    return (
      <BoatsStepImportProgress jobId={jobId} onFinished={onImportFinished} />
    );
  }

  return (
    <div className="onboarding__choices">
      {done && (
        <div className="muted" style={{ marginBottom: "var(--sp-sm)" }}>
          You already have boats in your fleet. Add more or move on.
        </div>
      )}
      <Link to="/admin/import/liveaboard?return=onboarding/boats" className="onboarding__choice">
        <strong>Import from liveaboard.com</strong>
        <span className="muted">Pull boat + trip data from a liveaboard.com listing URL.</span>
      </Link>
      <Link to="/admin/import/spreadsheet?return=onboarding/boats" className="onboarding__choice">
        <strong>Import a spreadsheet</strong>
        <span className="muted">Upload an Excel sheet with your boats and upcoming trips.</span>
      </Link>
      <Link to="/admin/fleet?return=onboarding/boats" className="onboarding__choice">
        <strong>Open Fleet</strong>
        <span className="muted">Add or edit boats by hand from the Fleet page.</span>
      </Link>
    </div>
  );
}

function BoatsStepImportProgress({
  jobId,
  onFinished,
}: {
  jobId: string;
  onFinished: () => void;
}) {
  const [job, setJob] = useState<ImportJob | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    async function tick() {
      try {
        const j = await adminApi.getImportJob(jobId);
        if (cancelled) return;
        setJob(j);
        if (j.status === "succeeded") {
          // Give the wizard a beat to settle, then auto-advance.
          timer = setTimeout(() => {
            if (!cancelled) onFinished();
          }, 600);
          return;
        }
        if (j.status === "failed") {
          return;
        }
        timer = setTimeout(tick, 2000);
      } catch (e) {
        if (cancelled) return;
        setError((e as { message?: string })?.message ?? "Could not load import job.");
      }
    }
    void tick();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [jobId, onFinished]);

  if (error) return <div className="error">{error}</div>;
  if (!job) return <div className="muted">Loading import…</div>;

  return (
    <div>
      <p>
        <strong>Import status:</strong> {job.status}
        {job.status === "running" && (
          <span className="muted"> — fetching trips, ~1 request per second</span>
        )}
      </p>
      {job.status === "succeeded" && (
        <div className="success">
          Imported. Moving on to cabin layouts…
          <ul style={{ marginTop: "var(--sp-sm)", listStyle: "none", padding: 0 }}>
            <li>Trips inserted: {job.trips_inserted ?? 0}</li>
            <li>Trips updated: {job.trips_updated ?? 0}</li>
          </ul>
        </div>
      )}
      {job.status === "failed" && (
        <div className="error">
          Import failed: {job.error_message ?? "unknown error"}
        </div>
      )}
      {(job.status === "queued" || job.status === "running") && (
        <p className="muted">
          You can leave this page — the import keeps running. Come back
          and the wizard will pick up where it left off.
        </p>
      )}
    </div>
  );
}

function LayoutsStep({ state }: { state: OnboardingState }) {
  const boats = state.boats_without_layouts;
  if (boats.length === 0) {
    return (
      <div className="muted">
        Every boat in your fleet has a usable cabin layout. Move on or open
        Fleet to refine any layout.
      </div>
    );
  }
  return (
    <ul className="onboarding__rows">
      {boats.map((b: OnboardingBoatWithoutLayout) => (
        <li key={b.boat_id} className="onboarding__row">
          <span>{b.boat_name}</span>
          <Link
            className="secondary"
            to={`/admin/fleet/${encodeURIComponent(b.boat_id)}/cabins?return=onboarding/layouts`}
          >
            Set up layout →
          </Link>
        </li>
      ))}
    </ul>
  );
}

function DirectorsStep() {
  return (
    <div className="onboarding__choices">
      <Link to="/admin/users?return=onboarding/directors" className="onboarding__choice">
        <strong>Open Users</strong>
        <span className="muted">
          Invite Cruise Directors by email. They get a registration link and
          land in their own admin chrome.
        </span>
      </Link>
    </div>
  );
}

// --- Locale → currency best-effort guess ---

const LOCALE_CURRENCY: Record<string, string> = {
  US: "USD", GB: "GBP", AU: "AUD", CA: "CAD", NZ: "NZD",
  ID: "IDR", JP: "JPY", KR: "KRW", TH: "THB", PH: "PHP",
  SG: "SGD", MY: "MYR", MV: "MVR", AE: "AED", SA: "SAR",
  EG: "EGP", BH: "BHD", KW: "KWD", OM: "OMR",
  DE: "EUR", FR: "EUR", IT: "EUR", ES: "EUR", PT: "EUR",
  NL: "EUR", IE: "EUR", AT: "EUR", BE: "EUR", FI: "EUR",
  GR: "EUR", LU: "EUR", MT: "EUR",
};

function guessCurrencyFromLocale(): string | null {
  try {
    const lang = navigator.language ?? "";
    const region = lang.split("-")[1]?.toUpperCase();
    if (region && LOCALE_CURRENCY[region]) return LOCALE_CURRENCY[region];
  } catch {
    // Best-effort only.
  }
  return null;
}
