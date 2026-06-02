import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

import { adminApi } from "../api";
import type { ApiError } from "../../lib/api";
import { Button, Card, Field, PageHeader } from "../components";
import { ImportJobView } from "./ImportJob";

import styles from "./ImportLiveaboard.module.css";

export function ImportLiveaboard() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const returnTo = params.get("return"); // e.g. "onboarding/boats"
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
      // If we came from the onboarding wizard, hand the in-flight job
      // back to it so the operator can watch progress + auto-advance
      // to layouts in one place.
      if (returnTo === "onboarding/boats") {
        navigate(
          `/admin/onboarding?step=boats&job=${encodeURIComponent(job.id)}`,
          { replace: true },
        );
        return;
      }
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
      <PageHeader
        title="Import from liveaboard.com"
        subtitle="Paste the boat's listing URL on liveaboard.com."
        actions={
          <Link to="/admin/import">
            <Button variant="quiet">← Back</Button>
          </Link>
        }
      />

      {!jobId ? (
        <Card>
          <form onSubmit={onSubmit} className={styles.form}>
            {error && <div className={styles.error}>{error}</div>}
            <Field label="Boat URL" htmlFor="url">
              <input
                id="url"
                type="url"
                placeholder="https://www.liveaboard.com/diving/indonesia/gaia-love"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                required
                autoFocus
              />
              <p className={styles.fieldHint}>
                We honor liveaboard.com's robots.txt and rate-limit at 1 request
                per second. We'll fetch every published trip on the boat's
                listing.
              </p>
            </Field>
            <Button variant="primary" type="submit" disabled={submitting}>
              {submitting ? "Starting…" : "Start import"}
            </Button>
          </form>
        </Card>
      ) : (
        <ImportJobView jobId={jobId} />
      )}
    </>
  );
}
