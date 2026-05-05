import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { adminApi, type ImportJob } from "../api";
import type { ApiError } from "../../lib/api";

// ImportJobView polls /api/admin/import/jobs/{id} every 2s until
// the status is terminal (succeeded or failed). Used by both the
// liveaboard and (for completeness) spreadsheet wizards.
export function ImportJobView({ jobId }: { jobId: string }) {
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
        if (j.status === "succeeded" || j.status === "failed") {
          return; // stop polling
        }
        timer = setTimeout(tick, 2000);
      } catch (e) {
        if (cancelled) return;
        const apiErr = e as ApiError;
        setError(apiErr?.message ?? "Failed to load job.");
      }
    }
    void tick();

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [jobId]);

  if (error) return <div className="error">{error}</div>;
  if (!job) return <div className="muted">Loading…</div>;

  return (
    <div className="admin-card">
      <h2 className="admin-card__title">
        {sourceLabel(job.source)} import
      </h2>
      <p className="muted" style={{ marginBottom: "var(--sp-md)" }}>
        Status: <strong>{job.status}</strong>
        {job.source === "liveaboard_com" && job.status === "running" && (
          <span> · fetching the boat's full schedule, ~1 request per second</span>
        )}
      </p>

      {job.status === "succeeded" && (
        <div className="success">
          Imported successfully.
          <ul style={{ marginTop: "var(--sp-sm)", listStyle: "none", padding: 0 }}>
            <li>Trips inserted: {job.trips_inserted ?? 0}</li>
            <li>Trips updated: {job.trips_updated ?? 0}</li>
            <li>Trips removed: {job.trips_deleted ?? 0}</li>
          </ul>
          <p style={{ marginTop: "var(--sp-md)" }}>
            <Link to="/admin/trips">View trips →</Link>
          </p>
        </div>
      )}

      {job.status === "failed" && (
        <div className="error">
          Import failed: {job.error_message ?? "unknown error"}
        </div>
      )}
    </div>
  );
}

function sourceLabel(s: ImportJob["source"]): string {
  switch (s) {
    case "liveaboard_com":
      return "liveaboard.com";
    case "spreadsheet":
      return "Spreadsheet";
  }
}
