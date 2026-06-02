import { useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";

import {
  adminApi,
  type AuditEvent,
  type GuestDocument,
  type GuestRegistrationDetail,
  type Trip,
} from "../api";
import { appConfig } from "../../lib/config";
import {
  RegistrationSections,
  emptyRegistrationPayload,
  mergeRegistrationPayload,
  type RegistrationPayload,
} from "../../lib/registration";
import {
  Button,
  Card,
  Chip,
  type ChipVariant,
  PageHeader,
} from "../components";

import styles from "./TripGuestDetail.module.css";

export function TripGuestDetail() {
  const { id = "", guestId = "" } = useParams<{
    id: string;
    guestId: string;
  }>();
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
        setError(
          (err as { message?: string })?.message ??
            "Failed to load guest details.",
        );
      });
    return () => {
      cancelled = true;
    };
  }, [id, guestId]);

  if (error) {
    return (
      <>
        <div className={styles.breadcrumb}>
          <Link to="/admin/trips">Trips</Link>
          {" / "}
          <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
        </div>
        <div className={styles.error}>{error}</div>
      </>
    );
  }
  if (!detail || !trip) {
    return (
      <>
        <div className={styles.breadcrumb}>
          <Link to="/admin/trips">Trips</Link>
          {" / "}
          <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
        </div>
        <div className={styles.muted}>Loading...</div>
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
      setError(
        (err as { message?: string })?.message ?? "Could not upload document.",
      );
    } finally {
      setUploading(false);
    }
  }

  async function archiveDocument(documentId: string) {
    setError(null);
    try {
      const archived = await adminApi.archiveGuestDocument(
        id,
        guestId,
        documentId,
      );
      const events = await adminApi.guestActivity(id, guestId);
      setDocuments((prev) =>
        prev.map((doc) => (doc.id === documentId ? archived : doc)),
      );
      setActivity(events.events);
    } catch (err) {
      setError(
        (err as { message?: string })?.message ?? "Could not archive document.",
      );
    }
  }

  return (
    <>
      <div className={styles.breadcrumb}>
        <Link to="/admin/trips">Trips</Link>
        {" / "}
        <Link to={`/admin/trips/${id}/manifest`}>Manifest</Link>
      </div>
      <PageHeader
        title={guest.full_name}
        subtitle={`${trip.boat_name} — ${trip.start_date} to ${trip.end_date} — ${trip.itinerary}`}
        actions={
          <>
            <Link to={`/admin/trips/${id}/cabins`}>
              <Button variant="secondary">Change cabin</Button>
            </Link>
            <Link to={`/admin/trips/${id}/guests/${guestId}/folio`}>
              <Button variant="secondary">Open checkout</Button>
            </Link>
          </>
        }
      />

      <Card>
        <div className={styles.summaryGrid}>
          <div className={styles.readField}>
            <label>Email</label>
            <div className={styles.readout}>{guest.email}</div>
          </div>
          <div className={styles.readField}>
            <label>Status</label>
            <div className={styles.readout}>
              <Chip variant={statusVariant(guest.status)}>
                {guest.status.replaceAll("_", " ")}
              </Chip>
            </div>
          </div>
          <div className={styles.readField}>
            <label>Cabin</label>
            <div className={styles.readout}>
              {guest.cabin_assignment?.display_label ?? (
                <Chip variant="warning">Needs cabin</Chip>
              )}
            </div>
          </div>
          <div className={styles.readField}>
            <label>Account created</label>
            <div className={styles.readout}>
              {formatDate(guest.account_created_at)}
            </div>
          </div>
          <div className={styles.readField}>
            <label>Submitted</label>
            <div className={styles.readout}>
              {formatDate(guest.registration_submitted_at)}
            </div>
          </div>
        </div>
      </Card>

      {!reg && (
        <div className={styles.callout}>
          This guest hasn't started their registration yet. Use Resend on the
          manifest if the invite needs another nudge.
        </div>
      )}

      <Card
        title="Documents"
        actions={
          <Chip variant="neutral">
            {documents.filter((doc) => !doc.archived_at).length} active
          </Chip>
        }
      >
        <form className={styles.documentUpload} onSubmit={uploadDocument}>
          <label>
            Category
            <select
              value={docCategory}
              onChange={(e) => setDocCategory(e.target.value)}
            >
              {documentCategories.map((c) => (
                <option key={c.value} value={c.value}>
                  {c.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Display name
            <input
              value={docDisplayName}
              onChange={(e) => setDocDisplayName(e.target.value)}
              placeholder="Optional"
            />
          </label>
          <label>
            Notes
            <input
              value={docNotes}
              onChange={(e) => setDocNotes(e.target.value)}
              placeholder="Optional"
            />
          </label>
          <label>
            File
            <input
              type="file"
              accept=".pdf,.jpg,.jpeg,.png,.heic,.heif,application/pdf,image/jpeg,image/png,image/heic,image/heif"
              onChange={(e) => setDocFile(e.target.files?.[0] ?? null)}
            />
          </label>
          <Button type="submit" variant="secondary" disabled={uploading}>
            {uploading ? "Uploading..." : "Upload"}
          </Button>
        </form>
        <DocumentList documents={documents} onArchive={archiveDocument} />
      </Card>

      {reg && (
        <Card
          title="Registration"
          actions={
            <Chip variant={statusVariant(reg.status)}>{reg.status}</Chip>
          }
        >
          <RegistrationSections mode="read" payload={payload} />
        </Card>
      )}

      <Card
        title="Activity"
        actions={<Chip variant="neutral">{activity.length} events</Chip>}
      >
        <ActivityList events={activity} />
      </Card>
    </>
  );
}

// Map a guest/registration status label to a shared Chip variant.
function statusVariant(label: string): ChipVariant {
  switch (label) {
    case "active":
    case "paid":
    case "settled":
      return "success";
    case "cancelled":
    case "removed":
      return "error";
    case "pending":
      return "warning";
    case "submitted":
    case "completed":
      return "info";
    default:
      return "neutral";
  }
}

const documentCategories = [
  { value: "travel_document", label: "Travel document" },
  { value: "dive_certification", label: "Dive certification" },
  { value: "dive_insurance", label: "Dive insurance" },
  { value: "liability_waiver", label: "Liability waiver" },
  { value: "medical", label: "Medical" },
  { value: "other", label: "Other" },
];

function DocumentList({
  documents,
  onArchive,
}: {
  documents: GuestDocument[];
  onArchive: (id: string) => void;
}) {
  if (documents.length === 0)
    return <div className={styles.muted}>No documents uploaded.</div>;
  return (
    <div className={styles.list}>
      {documents.map((doc) => (
        <div
          key={doc.id}
          className={
            doc.archived_at ? `${styles.row} ${styles.rowArchived}` : styles.row
          }
        >
          <div>
            <strong>{doc.display_name}</strong>
            <div className={styles.muted}>
              {categoryLabel(doc.category)} · {doc.original_filename} ·{" "}
              {formatBytes(doc.size_bytes)}
              {doc.archived_at
                ? ` · archived ${formatDate(doc.archived_at)}`
                : ""}
            </div>
            {doc.notes && <div className={styles.muted}>{doc.notes}</div>}
          </div>
          <div className={styles.rowActions}>
            <a
              href={`${appConfig.apiBase}${doc.view_url}`}
              target="_blank"
              rel="noreferrer"
            >
              View
            </a>
            <a href={`${appConfig.apiBase}${doc.download_url}`}>Download</a>
            {!doc.archived_at && (
              <Button variant="secondary" onClick={() => onArchive(doc.id)}>
                Archive
              </Button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

function ActivityList({ events }: { events: AuditEvent[] }) {
  if (events.length === 0)
    return <div className={styles.muted}>No activity yet.</div>;
  return (
    <div className={styles.list}>
      {events.map((event) => (
        <div key={event.id} className={styles.row}>
          <div>
            <strong>{actionLabel(event.action)}</strong>
            <div className={styles.muted}>{summary(event)}</div>
          </div>
          <div className={styles.muted}>{formatDateTime(event.created_at)}</div>
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
