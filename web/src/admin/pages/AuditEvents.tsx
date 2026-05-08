import { useEffect, useState, type FormEvent } from "react";

import { adminApi, type AuditEvent } from "../api";

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
      setError((err as { message?: string })?.message ?? "Failed to load audit events.");
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
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Audit</h1>
          <div className="admin-page-subtitle">Operational changes across the organization.</div>
        </div>
      </div>

      <form className="admin-card audit-filters" onSubmit={submit}>
        <label>
          Action
          <input value={action} onChange={(e) => setAction(e.target.value)} placeholder="guest.document_uploaded" />
        </label>
        <label>
          Actor
          <select value={actorType} onChange={(e) => setActorType(e.target.value)}>
            <option value="">Any</option>
            <option value="staff">Staff</option>
            <option value="guest">Guest</option>
            <option value="system">System</option>
          </select>
        </label>
        <label>
          Entity
          <input value={entityType} onChange={(e) => setEntityType(e.target.value)} placeholder="guest_document" />
        </label>
        <label>
          Trip ID
          <input value={tripId} onChange={(e) => setTripId(e.target.value)} placeholder="Optional UUID" />
        </label>
        <button type="submit" className="secondary">Search</button>
      </form>

      {error && <div className="error">{error}</div>}
      <div className="admin-card">
        {loading ? (
          <div className="muted">Loading...</div>
        ) : events.length === 0 ? (
          <div className="muted">No events found.</div>
        ) : (
          <div className="audit-table">
            <div className="audit-table__head">
              <span>Time</span>
              <span>Actor</span>
              <span>Action</span>
              <span>Entity</span>
              <span>Summary</span>
            </div>
            {events.map((event) => (
              <div key={event.id} className="audit-table__row">
                <span>{formatDateTime(event.created_at)}</span>
                <span>{event.actor_type}</span>
                <span>{event.action}</span>
                <span>{event.entity_type}</span>
                <span>{summary(event)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  );
}

function summary(event: AuditEvent): string {
  const parts: string[] = [];
  for (const key of ["display_name", "category", "status", "payment_method", "settlement_currency"]) {
    const value = event.metadata[key];
    if (typeof value === "string" && value.trim()) parts.push(value.replaceAll("_", " "));
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
