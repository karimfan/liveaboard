import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";

import { adminApi } from "../api";
import type { ApiError } from "../../lib/api";
import { ImportJobView } from "./ImportJob";

export function ImportLiveaboard() {
  const [url, setUrl] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const job = await adminApi.kickLiveaboardImport(url.trim());
      setJobId(job.id);
    } catch (err) {
      const apiErr = err as ApiError;
      setError(apiErr.message ?? "Could not start import.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Import from liveaboard.com</h1>
          <div className="admin-page-subtitle">
            Paste the boat's listing URL on liveaboard.com.
          </div>
        </div>
        <Link to="/admin/import" className="ghost">← Back</Link>
      </div>

      {!jobId ? (
        <form onSubmit={onSubmit} className="admin-card" style={{ maxWidth: 640 }}>
          {error && <div className="error">{error}</div>}
          <div className="field">
            <label htmlFor="url">Boat URL</label>
            <input
              id="url"
              type="url"
              placeholder="https://www.liveaboard.com/diving/indonesia/gaia-love"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              required
              autoFocus
            />
            <p className="muted" style={{ fontSize: 12, marginTop: 6 }}>
              We honor liveaboard.com's robots.txt and rate-limit at 1
              request per second. A full 18-month scrape takes about
              30 seconds.
            </p>
          </div>
          <button className="primary" type="submit" disabled={submitting}>
            {submitting ? "Starting…" : "Start import"}
          </button>
        </form>
      ) : (
        <ImportJobView jobId={jobId} />
      )}
    </>
  );
}
