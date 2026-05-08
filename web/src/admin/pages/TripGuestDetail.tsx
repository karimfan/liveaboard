import { useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type AuditEvent, type GuestDocument, type GuestRegistrationDetail, type Trip } from "../api";
import { appConfig } from "../../lib/config";
import {
  RegistrationSections,
  emptyRegistrationPayload,
  mergeRegistrationPayload,
  type RegistrationPayload,
} from "../../lib/registration";

export function TripGuestDetail() {
  const { id = "", guestId = "" } = useParams<{ id: string; guestId: string }>();
  const [detail, setDetail] = useState<GuestRegistrationDetail | null>(null);
  const [trip, setTrip] = useState<Trip | null>(null);
  const [documents, setDocuments] = useState<GuestDocument[]>([]);
  const [activity, setActivity] = useState<AuditEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [docCategory, setDocCategory] = useState("travel_document");
  const [docDisplayName, setDocDisplayName] = useState("");
  const [docNotes, setDocNotes] = useState("");
  const [docFile, setDocFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setError(null);

    Promise.all([
      adminApi.guestRegistration(id, guestId),
      adminApi.tripManifest(id),
      adminApi.guestDocuments(id, guestId),
      adminApi.guestActivity(id, guestId),
    ])
      .then(([reg, manifest, docs, events]) => {
        if (cancelled) return;
        setDetail(reg);
        setTrip(manifest.trip);
        setDocuments(docs.documents);
        setActivity(events.events);
      })
      .catch((err) => {
        if (cancelled) return;
        setError((err as { message?: string })?.message ?? "Failed to load guest details.");
      });
    return () => {
      cancelled = true;
    };
  }, [id, guestId]);

  if (error) {
    return (
      <>
        <div className="admin-breadcrumb">
          <Link to="/admin/trips">Trips</Link>
          {" / "}
          <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
        </div>
        <div className="error">{error}</div>
      </>
    );
  }
  if (!detail || !trip) {
    return (
      <>
        <div className="admin-breadcrumb">
          <Link to="/admin/trips">Trips</Link>
          {" / "}
          <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
        </div>
        <div className="muted">Loading...</div>
      </>
    );
  }

  const guest = detail.trip_guest;
  const reg = detail.registration;
  const payload: RegistrationPayload = reg
    ? mergeRegistrationPayload(reg.payload as RegistrationPayload)
    : emptyRegistrationPayload;

  async function uploadDocument(e: FormEvent) {
    e.preventDefault();
    if (!docFile) {
      setError("Choose a file to upload.");
      return;
    }
    setUploading(true);
    setError(null);
    try {
      const doc = await adminApi.uploadGuestDocument(id, guestId, {
        file: docFile,
        category: docCategory,
        display_name: docDisplayName,
        notes: docNotes,
      });
      const events = await adminApi.guestActivity(id, guestId);
      setDocuments((prev) => [doc, ...prev]);
      setActivity(events.events);
      setDocDisplayName("");
      setDocNotes("");
      setDocFile(null);
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not upload document.");
    } finally {
      setUploading(false);
    }
  }

  async function archiveDocument(documentId: string) {
    setError(null);
    try {
      const archived = await adminApi.archiveGuestDocument(id, guestId, documentId);
      const events = await adminApi.guestActivity(id, guestId);
      setDocuments((prev) => prev.map((doc) => (doc.id === documentId ? archived : doc)));
      setActivity(events.events);
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not archive document.");
    }
  }

  return (
    <>
      <div className="admin-breadcrumb">
        <Link to="/admin/trips">Trips</Link>
        {" / "}
        <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
      </div>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">{guest.full_name}</h1>
          <div className="admin-page-subtitle">
            {trip.boat_name} — {trip.start_date} to {trip.end_date} — {trip.itinerary}
          </div>
        </div>
        <div className="admin-page-actions">
          <Link className="secondary" to={`/admin/trips/${id}/cabins`}>
            Change cabin
          </Link>
          <Link className="secondary" to={`/admin/trips/${id}/guests/${guestId}/folio`}>
            Open checkout
          </Link>
        </div>
      </div>

      <div className="admin-card guest-detail-summary">
        <div className="form-grid">
          <div className="field field--read">
            <label>Email</label>
            <div className="field__readout">{guest.email}</div>
          </div>
          <div className="field field--read">
            <label>Status</label>
            <div className="field__readout">
              <span className="chip chip--active">{guest.status.replaceAll("_", " ")}</span>
            </div>
          </div>
          <div className="field field--read">
            <label>Cabin</label>
            <div className="field__readout">
              {guest.cabin_assignment?.display_label ?? <span className="chip chip--warning">Needs cabin</span>}
            </div>
          </div>
          <div className="field field--read">
            <label>Account created</label>
            <div className="field__readout">{formatDate(guest.account_created_at)}</div>
          </div>
          <div className="field field--read">
            <label>Submitted</label>
            <div className="field__readout">{formatDate(guest.registration_submitted_at)}</div>
          </div>
        </div>
      </div>

      {!reg && (
        <div className="callout">
          This guest hasn't started their registration yet. Use Resend on the manifest if the invite needs another nudge.
        </div>
      )}

      <div className="admin-card">
        <div className="admin-card__title-row">
          <h2 className="admin-card__title">Documents</h2>
          <span className="chip">{documents.filter((doc) => !doc.archived_at).length} active</span>
        </div>
        <form className="document-upload" onSubmit={uploadDocument}>
          <label>
            Category
            <select value={docCategory} onChange={(e) => setDocCategory(e.target.value)}>
              {documentCategories.map((c) => (
                <option key={c.value} value={c.value}>{c.label}</option>
              ))}
            </select>
          </label>
          <label>
            Display name
            <input value={docDisplayName} onChange={(e) => setDocDisplayName(e.target.value)} placeholder="Optional" />
          </label>
          <label>
            Notes
            <input value={docNotes} onChange={(e) => setDocNotes(e.target.value)} placeholder="Optional" />
          </label>
          <label>
            File
            <input
              type="file"
              accept=".pdf,.jpg,.jpeg,.png,.heic,.heif,application/pdf,image/jpeg,image/png,image/heic,image/heif"
              onChange={(e) => setDocFile(e.target.files?.[0] ?? null)}
            />
          </label>
          <button type="submit" className="secondary" disabled={uploading}>
            {uploading ? "Uploading..." : "Upload"}
          </button>
        </form>
        <DocumentList documents={documents} onArchive={archiveDocument} />
      </div>

      {reg && (
        <div className="admin-card">
          <div className="admin-card__title-row">
            <h2 className="admin-card__title">Registration</h2>
            <span className="chip chip--active">{reg.status}</span>
          </div>
          <RegistrationSections mode="read" payload={payload} />
        </div>
      )}

      <div className="admin-card">
        <div className="admin-card__title-row">
          <h2 className="admin-card__title">Activity</h2>
          <span className="chip">{activity.length} events</span>
        </div>
        <ActivityList events={activity} />
      </div>
    </>
  );
}

const documentCategories = [
  { value: "travel_document", label: "Travel document" },
  { value: "dive_certification", label: "Dive certification" },
  { value: "dive_insurance", label: "Dive insurance" },
  { value: "liability_waiver", label: "Liability waiver" },
  { value: "medical", label: "Medical" },
  { value: "other", label: "Other" },
];

function DocumentList({ documents, onArchive }: { documents: GuestDocument[]; onArchive: (id: string) => void }) {
  if (documents.length === 0) return <div className="muted">No documents uploaded.</div>;
  return (
    <div className="document-list">
      {documents.map((doc) => (
        <div key={doc.id} className={"document-row" + (doc.archived_at ? " is-archived" : "")}>
          <div>
            <strong>{doc.display_name}</strong>
            <div className="muted">
              {categoryLabel(doc.category)} · {doc.original_filename} · {formatBytes(doc.size_bytes)}
              {doc.archived_at ? ` · archived ${formatDate(doc.archived_at)}` : ""}
            </div>
            {doc.notes && <div className="muted">{doc.notes}</div>}
          </div>
          <div className="document-row__actions">
            <a className="secondary" href={`${appConfig.apiBase}${doc.view_url}`} target="_blank" rel="noreferrer">View</a>
            <a className="secondary" href={`${appConfig.apiBase}${doc.download_url}`}>Download</a>
            {!doc.archived_at && (
              <button type="button" className="secondary" onClick={() => onArchive(doc.id)}>Archive</button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

function ActivityList({ events }: { events: AuditEvent[] }) {
  if (events.length === 0) return <div className="muted">No activity yet.</div>;
  return (
    <div className="activity-list">
      {events.map((event) => (
        <div key={event.id} className="activity-row">
          <div>
            <strong>{actionLabel(event.action)}</strong>
            <div className="muted">{summary(event)}</div>
          </div>
          <div className="muted">{formatDateTime(event.created_at)}</div>
        </div>
      ))}
    </div>
  );
}

function actionLabel(action: string): string {
  return action.replace(/^guest\./, "").replaceAll("_", " ");
}

function summary(event: AuditEvent): string {
  const display = event.metadata.display_name;
  const category = event.metadata.category;
  const parts: string[] = [event.actor_type];
  if (typeof category === "string") parts.push(category.replaceAll("_", " "));
  if (typeof display === "string") parts.push(display);
  return parts.join(" · ");
}

function categoryLabel(value: string): string {
  return documentCategories.find((c) => c.value === value)?.label ?? value;
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KiB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString();
}

function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString();
}
