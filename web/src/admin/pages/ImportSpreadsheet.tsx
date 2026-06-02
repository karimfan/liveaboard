import { useEffect, useState, type ChangeEvent } from "react";
import { Link } from "react-router-dom";

import {
  adminApi,
  type Boat,
  type SpreadsheetPreviewResponse,
  type VesselMappingChoice,
} from "../api";
import type { ApiError } from "../../lib/api";
import {
  ActionBar,
  Button,
  Card,
  Chip,
  type Column,
  DataTable,
  Field,
  PageHeader,
} from "../components";

import styles from "./ImportSpreadsheet.module.css";

// ImportSpreadsheet drives the multi-step wizard:
//   1. pick a file (.csv or .xlsx)
//   2. preview rows + warnings
//   3. map unknown vessel names to existing boats or create new ones
//   4. commit
//
// The preview is persisted server-side under preview_id; the commit
// step references that id rather than re-uploading. preview_id
// expires server-side after 1 hour.

type Step = "pick" | "review" | "done";

export function ImportSpreadsheet() {
  const [step, setStep] = useState<Step>("pick");
  const [error, setError] = useState<string | null>(null);

  const [preview, setPreview] = useState<SpreadsheetPreviewResponse | null>(
    null,
  );
  const [boats, setBoats] = useState<Boat[]>([]);
  const [mapping, setMapping] = useState<Record<string, VesselMappingChoice>>(
    {},
  );
  const [skipped, setSkipped] = useState<Set<number>>(new Set());

  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<{
    inserted: number;
    updated: number;
    deleted: number;
  } | null>(null);

  useEffect(() => {
    // Pre-load org boats so the mapping dropdowns have options.
    adminApi
      .listBoats()
      .then((r) => setBoats(r.boats ?? []))
      .catch(() => undefined);
  }, []);

  async function onFileChosen(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (!f) return;
    setError(null);
    try {
      const resp = await adminApi.previewSpreadsheet(f);
      setPreview(resp);
      // Auto-pick mappings for vessels with a server-side suggestion.
      const m: Record<string, VesselMappingChoice> = {};
      for (const v of resp.payload.vessel_names) {
        const sug = resp.vessel_suggestions[v];
        if (sug) {
          m[v] = { mode: "existing", boat_id: sug.boat_id };
        }
      }
      setMapping(m);
      setStep("review");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr?.message ?? "Could not parse the file.");
    }
  }

  async function onCommit() {
    if (!preview) return;
    setSubmitting(true);
    setError(null);
    try {
      const res = await adminApi.commitSpreadsheet({
        preview_id: preview.preview_id,
        vessel_mapping: mapping,
        rows_to_skip: Array.from(skipped),
      });
      setResult({
        inserted: res.trips_inserted,
        updated: res.trips_updated,
        deleted: res.trips_deleted,
      });
      setStep("done");
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr?.message ?? "Could not commit the import.");
    } finally {
      setSubmitting(false);
    }
  }

  const allMapped =
    preview?.payload.vessel_names.every((v) => mapping[v] !== undefined) ??
    false;

  return (
    <>
      <PageHeader
        title="Upload spreadsheet"
        subtitle="CSV or XLSX with vessel name, dates, and itinerary."
        actions={
          <Link to="/admin/import">
            <Button variant="quiet">← Back</Button>
          </Link>
        }
      />

      {error && <div className={styles.error}>{error}</div>}

      {step === "pick" && (
        <Card className={styles.pickCard}>
          <p className={styles.intro}>
            Required columns: <strong>vessel name</strong>,{" "}
            <strong>trip start date</strong>, <strong>trip end date</strong>,{" "}
            <strong>itinerary</strong>. Optional:{" "}
            <strong>number of guests</strong>.
          </p>
          <p className={styles.hint}>
            Dates must be ISO (YYYY-MM-DD), "Jan 2, 2026", or "2 Jan 2026".
            Other formats are skipped with a warning.
          </p>
          <p className={styles.hint}>File size limit: 2&nbsp;MB.</p>
          <div className={styles.fileField}>
            <Field label="Choose a .csv or .xlsx file" htmlFor="file">
              <input
                id="file"
                type="file"
                accept=".csv,.xlsx"
                onChange={onFileChosen}
              />
            </Field>
          </div>
        </Card>
      )}

      {step === "review" && preview && (
        <ReviewStep
          preview={preview}
          boats={boats}
          mapping={mapping}
          onMappingChange={setMapping}
          skipped={skipped}
          onSkippedChange={setSkipped}
          allMapped={allMapped}
          submitting={submitting}
          onCommit={onCommit}
          onCancel={() => {
            setPreview(null);
            setMapping({});
            setSkipped(new Set());
            setStep("pick");
          }}
        />
      )}

      {step === "done" && result && (
        <Card title="Import complete">
          <ul className={styles.resultList}>
            <li>Trips inserted: {result.inserted}</li>
            <li>Trips updated: {result.updated}</li>
            <li>Trips removed: {result.deleted}</li>
          </ul>
          <Link to="/admin/trips">
            <Button variant="primary">View trips →</Button>
          </Link>
        </Card>
      )}
    </>
  );
}

type VesselRow = { vessel: string };

function ReviewStep(props: {
  preview: SpreadsheetPreviewResponse;
  boats: Boat[];
  mapping: Record<string, VesselMappingChoice>;
  onMappingChange: (m: Record<string, VesselMappingChoice>) => void;
  skipped: Set<number>;
  onSkippedChange: (s: Set<number>) => void;
  allMapped: boolean;
  submitting: boolean;
  onCommit: () => void;
  onCancel: () => void;
}) {
  const {
    preview,
    boats,
    mapping,
    onMappingChange,
    skipped,
    onSkippedChange,
    allMapped,
    submitting,
    onCommit,
    onCancel,
  } = props;
  const p = preview.payload;

  function setVessel(vessel: string, choice: VesselMappingChoice | null) {
    const next = { ...mapping };
    if (choice === null) {
      delete next[vessel];
    } else {
      next[vessel] = choice;
    }
    onMappingChange(next);
  }

  function toggleSkip(line: number) {
    const next = new Set(skipped);
    if (next.has(line)) next.delete(line);
    else next.add(line);
    onSkippedChange(next);
  }

  // Group warnings by line for the per-row chip.
  const warningsByLine = new Map<number, string[]>();
  for (const w of p.warnings) {
    const arr = warningsByLine.get(w.line_number) ?? [];
    arr.push(w.message);
    warningsByLine.set(w.line_number, arr);
  }

  const vesselColumns: Column<VesselRow>[] = [
    {
      key: "vessel",
      header: "Vessel name in file",
      cell: (r) => r.vessel,
    },
    {
      key: "mapto",
      header: "Map to",
      cell: (r) => (
        <select
          value={vesselSelectValue(mapping[r.vessel])}
          onChange={(e) => {
            const val = e.target.value;
            if (val === "") {
              setVessel(r.vessel, null);
            } else if (val === "__create__") {
              setVessel(r.vessel, { mode: "create_new" });
            } else {
              setVessel(r.vessel, { mode: "existing", boat_id: val });
            }
          }}
        >
          <option value="">— choose —</option>
          {boats.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name} (existing)
            </option>
          ))}
          <option value="__create__">
            + Create new boat &laquo;{r.vessel}&raquo;
          </option>
        </select>
      ),
    },
  ];

  return (
    <>
      <Card className={styles.calloutCard}>
        <p className={styles.callout}>
          <strong>Heads up:</strong> committing this upload reconciles
          spreadsheet trips per boat. Future trips for the boats in this file
          that aren't in the upload will be removed. Liveaboard.com-sourced
          trips are not touched.
        </p>
      </Card>

      <h2 className={styles.sectionTitle}>Vessel mapping</h2>
      {p.vessel_names.length === 0 ? (
        <p className={styles.empty}>No vessels found in the file.</p>
      ) : (
        <DataTable
          columns={vesselColumns}
          rows={p.vessel_names.map((v) => ({ vessel: v }))}
          rowKey={(r) => r.vessel}
        />
      )}

      <h2 className={styles.sectionTitle}>Rows ({p.rows.length})</h2>
      <table className={styles.rowsTable}>
        <thead>
          <tr>
            <th>Line</th>
            <th>Vessel</th>
            <th>Itinerary</th>
            <th>Dates</th>
            <th>Guests</th>
            <th>Status</th>
            <th>Skip</th>
          </tr>
        </thead>
        <tbody>
          {p.rows.map((row) => {
            const ws = warningsByLine.get(row.line_number) ?? [];
            const hasError =
              !row.vessel_name ||
              !row.start_date ||
              !row.end_date ||
              ws.some((m) => m.toLowerCase().includes("date"));
            return (
              <tr
                key={row.line_number}
                className={
                  skipped.has(row.line_number) ? styles.skipped : undefined
                }
              >
                <td>{row.line_number}</td>
                <td>
                  {row.vessel_name || <span className={styles.muted}>—</span>}
                </td>
                <td>
                  {row.itinerary || <span className={styles.muted}>—</span>}
                </td>
                <td>
                  {row.start_date && row.end_date ? (
                    `${row.start_date.slice(0, 10)} → ${row.end_date.slice(0, 10)}`
                  ) : (
                    <span className={styles.muted}>—</span>
                  )}
                </td>
                <td>
                  {row.num_guests ?? <span className={styles.muted}>—</span>}
                </td>
                <td>
                  {ws.length === 0 ? (
                    <Chip variant="success">ok</Chip>
                  ) : (
                    <span title={ws.join("\n")}>
                      <Chip variant={hasError ? "error" : "warning"}>
                        {ws.length} warn
                      </Chip>
                    </span>
                  )}
                </td>
                <td>
                  <input
                    type="checkbox"
                    checked={skipped.has(row.line_number)}
                    onChange={() => toggleSkip(row.line_number)}
                    aria-label={`Skip line ${row.line_number}`}
                  />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>

      <ActionBar
        secondary={
          <Button type="button" variant="quiet" onClick={onCancel}>
            Cancel
          </Button>
        }
        primary={
          <Button
            variant="primary"
            type="button"
            disabled={submitting || !allMapped}
            onClick={onCommit}
            title={
              !allMapped ? "Map every vessel before committing" : undefined
            }
          >
            {submitting ? "Importing…" : "Import trips"}
          </Button>
        }
      />
    </>
  );
}

function vesselSelectValue(choice: VesselMappingChoice | undefined): string {
  if (!choice) return "";
  if (choice.mode === "create_new") return "__create__";
  return choice.boat_id;
}
