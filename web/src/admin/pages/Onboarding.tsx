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
import { Button, Field } from "../components";

import styles from "./Onboarding.module.css";

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
  const [importJob, setImportJob] = useState<ImportJob | null>(null);

  const jobId = params.get("job");

  async function load() {
    setError(null);
    try {
      setState(await adminApi.onboarding());
    } catch (e) {
      setError(
        (e as { message?: string })?.message ??
          "Failed to load onboarding state.",
      );
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

  // Wizard-level import poll: when ?job= is set, poll the job
  // status every 2s, irrespective of which step the operator is
  // currently viewing. They can keep configuring layouts or kick
  // off another import while the scrape runs. On success we strip
  // the ?job param, refresh state, and (only if the operator is
  // still on the boats step) auto-advance to layouts.
  useEffect(() => {
    if (!jobId) {
      setImportJob(null);
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    async function tick() {
      try {
        const j = await adminApi.getImportJob(jobId!);
        if (cancelled) return;
        setImportJob(j);
        if (j.status === "succeeded" || j.status === "failed") {
          await load();
          // Drop ?job once terminal so the banner clears. If the
          // operator is still parked on the boats step, also jump
          // them to layouts — they came here to set those up.
          const p = new URLSearchParams(params);
          p.delete("job");
          if (j.status === "succeeded" && p.get("step") === "boats") {
            p.set("step", "layouts");
          }
          setParams(p, { replace: true });
          return;
        }
        timer = setTimeout(tick, 2000);
      } catch {
        if (cancelled) return;
        // Soft-fail: keep the banner showing the last-known status;
        // the user can refresh.
      }
    }
    void tick();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [jobId]);

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
      setError(
        (e as { message?: string })?.message ?? "Could not dismiss onboarding.",
      );
      return;
    }
    navigate("/admin", { replace: true });
  }

  if (error) {
    return (
      <div className={styles.onboarding}>
        <div className={styles.error}>{error}</div>
      </div>
    );
  }
  if (!state) {
    return (
      <div className={styles.onboarding}>
        <div className={styles.muted}>Loading…</div>
      </div>
    );
  }

  return (
    <div className={styles.onboarding}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.pageTitle}>Get your organization set up</h1>
          <div className={styles.pageSubtitle}>
            Four steps. You can skip any of them and come back from the
            Overview.
          </div>
        </div>
        <Button variant="quiet" onClick={() => void skipAll()}>
          Skip all
        </Button>
      </header>

      <Stepper state={state} current={stepKey} onPick={setStep} />

      {importJob && <ImportBanner job={importJob} />}

      <section className={styles.step}>
        <h2 className={styles.stepTitle}>
          {state.steps.find((s) => s.key === stepKey)?.label}
        </h2>
        <p className={styles.intro}>{STEP_INTRO[stepKey]}</p>
        {stepKey === "currency" && (
          <CurrencyStep
            state={state}
            onSaved={() => void load().then(advance)}
          />
        )}
        {stepKey === "boats" && <BoatsStep state={state} />}
        {stepKey === "layouts" && (
          <LayoutsStep
            state={state}
            fromGate={params.get("from_gate") === "1"}
            highlightBoatId={params.get("boat") ?? undefined}
            highlightBoatName={params.get("boat_name") ?? undefined}
          />
        )}
        {stepKey === "directors" && <DirectorsStep />}
      </section>

      <footer className={styles.footer}>
        <Button variant="quiet" onClick={advance}>
          Skip this step
        </Button>
        <Button
          variant="primary"
          onClick={advance}
          disabled={
            !state.steps.find((s) => s.key === stepKey)?.done &&
            stepKey !== "currency"
          }
        >
          {stepKey === STEP_ORDER[STEP_ORDER.length - 1]
            ? "Finish"
            : "Continue"}
        </Button>
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
    <ol className={styles.stepper}>
      {state.steps.map((s, i) => (
        <li
          key={s.key}
          className={[
            styles.stepperItem,
            s.done ? styles.stepperItemDone : "",
            s.key === current ? styles.stepperItemCurrent : "",
          ]
            .filter(Boolean)
            .join(" ")}
        >
          <button
            type="button"
            onClick={() => onPick(s.key)}
            className={styles.stepperBtn}
          >
            <span className={styles.stepperNum}>{s.done ? "✓" : i + 1}</span>
            <span className={styles.stepperLabel}>{s.label}</span>
            {s.hint && <span className={styles.stepperHint}>{s.hint}</span>}
          </button>
        </li>
      ))}
    </ol>
  );
}

// --- Step views ---

function CurrencyStep({
  state,
  onSaved,
}: {
  state: OnboardingState;
  onSaved: () => void;
}) {
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
      await adminApi.patchOrganization({
        name: org.name,
        currency: currency || null,
      });
      onSaved();
    } catch (e) {
      setError(
        (e as { message?: string })?.message ?? "Could not save currency.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className={styles.form} onSubmit={save}>
      {done && (
        <div className={styles.muted}>
          Already set to {currentCurrency}. You can change it here or move on.
        </div>
      )}
      <div className={styles.formField}>
        <Field label="Country currency" htmlFor="onboarding-currency">
          <CurrencyPicker
            id="onboarding-currency"
            value={currency}
            onChange={setCurrency}
            allowClear={false}
            placeholder="Search currency…"
          />
        </Field>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      <Button
        variant="primary"
        type="submit"
        disabled={submitting || currency === ""}
      >
        {submitting ? "Saving…" : "Save and continue"}
      </Button>
    </form>
  );
}

function BoatsStep({ state }: { state: OnboardingState }) {
  const done = state.steps.find((s) => s.key === "boats")?.done;
  return (
    <div className={styles.choices}>
      {done && (
        <div className={styles.muted}>
          You already have boats in your fleet. Add more or move on.
        </div>
      )}
      <Link
        to="/admin/import/liveaboard?return=onboarding/boats"
        className={styles.choice}
      >
        <strong>Import from liveaboard.com</strong>
        <span className={styles.choiceHint}>
          Pull boat + trip data from a liveaboard.com listing URL.
        </span>
      </Link>
      <Link
        to="/admin/import/spreadsheet?return=onboarding/boats"
        className={styles.choice}
      >
        <strong>Import a spreadsheet</strong>
        <span className={styles.choiceHint}>
          Upload an Excel sheet with your boats and upcoming trips.
        </span>
      </Link>
      <Link to="/admin/fleet?return=onboarding/boats" className={styles.choice}>
        <strong>Open Fleet</strong>
        <span className={styles.choiceHint}>
          Add or edit boats by hand from the Fleet page.
        </span>
      </Link>
    </div>
  );
}

// ImportBanner renders the in-flight job status above the step body
// while the wizard's parent component polls. Visible on every step
// so the operator can configure layouts or kick another import
// without losing sight of the running job.
function ImportBanner({ job }: { job: ImportJob }) {
  if (job.status === "queued" || job.status === "running") {
    return (
      <div className={styles.callout}>
        <strong>Import in progress.</strong> Fetching trips, ~1 request per
        second. You can keep using the wizard — we'll surface the boat in the
        Layouts step as soon as it lands.
      </div>
    );
  }
  if (job.status === "succeeded") {
    return (
      <div className={styles.success}>
        Import succeeded. Trips inserted: {job.trips_inserted ?? 0}, updated:{" "}
        {job.trips_updated ?? 0}.
      </div>
    );
  }
  if (job.status === "failed") {
    return (
      <div className={styles.error}>
        Import failed: {job.error_message ?? "unknown error"}
      </div>
    );
  }
  return null;
}

function LayoutsStep({
  state,
  fromGate,
  highlightBoatId,
  highlightBoatName,
}: {
  state: OnboardingState;
  fromGate?: boolean;
  highlightBoatId?: string;
  highlightBoatName?: string;
}) {
  const boats = state.boats_without_layouts;
  return (
    <>
      {fromGate && (
        <div className={styles.callout}>
          <strong>That action needs a cabin layout first.</strong>{" "}
          {highlightBoatName
            ? `Configure ${highlightBoatName}'s cabins below, then return to what you were doing.`
            : "Configure the boat's cabins below, then return to what you were doing."}
        </div>
      )}
      {boats.length === 0 ? (
        <div className={styles.muted}>
          Every boat in your fleet has a usable cabin layout. Move on or open
          Fleet to refine any layout.
        </div>
      ) : (
        <ul className={styles.rows}>
          {boats.map((b: OnboardingBoatWithoutLayout) => (
            <li
              key={b.boat_id}
              className={[
                styles.row,
                b.boat_id === highlightBoatId ? styles.rowHighlight : "",
              ]
                .filter(Boolean)
                .join(" ")}
            >
              <span>{b.boat_name}</span>
              <Link
                to={`/admin/fleet/${encodeURIComponent(b.boat_id)}/cabins?return=onboarding/layouts`}
              >
                <Button variant="secondary">Set up layout →</Button>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

function DirectorsStep() {
  return (
    <div className={styles.choices}>
      <Link
        to="/admin/users?return=onboarding/directors"
        className={styles.choice}
      >
        <strong>Open Users</strong>
        <span className={styles.choiceHint}>
          Invite Cruise Directors by email. They get a registration link and
          land in their own admin chrome.
        </span>
      </Link>
    </div>
  );
}

// --- Locale → currency best-effort guess ---

const LOCALE_CURRENCY: Record<string, string> = {
  US: "USD",
  GB: "GBP",
  AU: "AUD",
  CA: "CAD",
  NZ: "NZD",
  ID: "IDR",
  JP: "JPY",
  KR: "KRW",
  TH: "THB",
  PH: "PHP",
  SG: "SGD",
  MY: "MYR",
  MV: "MVR",
  AE: "AED",
  SA: "SAR",
  EG: "EGP",
  BH: "BHD",
  KW: "KWD",
  OM: "OMR",
  DE: "EUR",
  FR: "EUR",
  IT: "EUR",
  ES: "EUR",
  PT: "EUR",
  NL: "EUR",
  IE: "EUR",
  AT: "EUR",
  BE: "EUR",
  FI: "EUR",
  GR: "EUR",
  LU: "EUR",
  MT: "EUR",
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
