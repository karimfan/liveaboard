import { useEffect, useState, type FormEvent } from "react";

import { adminApi, type AuditEvent } from "../api";
import { Button, Card, Field, PageHeader } from "../components";

import styles from "./AuditEvents.module.css";

export function AuditEvents() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [action, setAction] = useState("");
  const [actorType, setActorType] = useState("");
  const [entityType, setEntityType] = useState("");
  const [tripId, setTripId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    load({});
  }, []);

  async function load(extra: Record<string, string>) {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = { limit: "100", ...extra };
      const res = await adminApi.auditEvents(params);
      setEvents(res.events);
    } catch (err) {
      setError(
        (err as { message?: string })?.message ??
          "Failed to load audit events.",
      );
    } finally {
      setLoading(false);
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    const params: Record<string, string> = { limit: "100" };
    if (action.trim()) params.action = action.trim();
    if (actorType) params.actor_type = actorType;
    if (entityType.trim()) params.entity_type = entityType.trim();
    if (tripId.trim()) params.trip_id = tripId.trim();
    void load(params);
  }

  return (
    <>
      <PageHeader
        title="Audit"
        subtitle="Operational changes across the organization."
      />

      <Card>
        <form className={styles.filters} onSubmit={submit}>
          <Field label="Action" htmlFor="audit-action">
            <input
              id="audit-action"
              value={action}
              onChange={(e) => setAction(e.target.value)}
              placeholder="guest.document_uploaded"
            />
          </Field>
          <Field label="Actor" htmlFor="audit-actor">
            <select
              id="audit-actor"
              value={actorType}
              onChange={(e) => setActorType(e.target.value)}
            >
              <option value="">Any</option>
              <option value="staff">Staff</option>
              <option value="guest">Guest</option>
              <option value="system">System</option>
            </select>
          </Field>
          <Field label="Entity" htmlFor="audit-entity">
            <input
              id="audit-entity"
              value={entityType}
              onChange={(e) => setEntityType(e.target.value)}
              placeholder="guest_document"
            />
          </Field>
          <Field label="Trip ID" htmlFor="audit-trip">
            <input
              id="audit-trip"
              value={tripId}
              onChange={(e) => setTripId(e.target.value)}
              placeholder="Optional UUID"
            />
          </Field>
          <Button type="submit" variant="secondary">
            Search
          </Button>
        </form>
      </Card>

      {error && <div className={styles.error}>{error}</div>}
      <Card>
        {loading ? (
          <div className={styles.loading}>Loading...</div>
        ) : events.length === 0 ? (
          <div className={styles.empty}>No events found.</div>
        ) : (
          <div className={styles.table}>
            <div className={styles.head}>
              <span>Time</span>
              <span>Actor</span>
              <span>Action</span>
              <span>Entity</span>
              <span>Summary</span>
            </div>
            {events.map((event) => (
              <div key={event.id} className={styles.row}>
                <span>{formatDateTime(event.created_at)}</span>
                <span>{event.actor_type}</span>
                <span>{event.action}</span>
                <span>{event.entity_type}</span>
                <span>{summary(event)}</span>
              </div>
            ))}
          </div>
        )}
      </Card>
    </>
  );
}

function summary(event: AuditEvent): string {
  const parts: string[] = [];
  for (const key of [
    "display_name",
    "category",
    "status",
    "payment_method",
    "settlement_currency",
  ]) {
    const value = event.metadata[key];
    if (typeof value === "string" && value.trim())
      parts.push(value.replaceAll("_", " "));
  }
  const size = event.metadata.size_bytes;
  if (typeof size === "number") parts.push(formatBytes(size));
  return parts.join(" · ") || "—";
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KiB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}

function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString();
}
