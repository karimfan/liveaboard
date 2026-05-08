import { useEffect, useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";

import { api, type GuestDocument } from "../lib/api";
import { appConfig } from "../lib/config";
import {
  RegistrationSections,
  emptyRegistrationPayload,
  mergeRegistrationPayload,
  normalizeRegistrationPayload,
  type RegistrationPayload,
} from "../lib/registration";

export function GuestRegistration() {
  const { tripGuestId = "" } = useParams<{ tripGuestId: string }>();
  const [payload, setPayload] = useState<RegistrationPayload>(emptyRegistrationPayload);
  const [status, setStatus] = useState<"draft" | "submitted">("draft");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [documents, setDocuments] = useState<GuestDocument[]>([]);
  const [docCategory, setDocCategory] = useState("travel_document");
  const [docDisplayName, setDocDisplayName] = useState("");
  const [docNotes, setDocNotes] = useState("");
  const [docFile, setDocFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.guestRegistration(tripGuestId), api.guestDocuments(tripGuestId)])
      .then(([res, docs]) => {
        if (cancelled) return;
        setPayload(mergeRegistrationPayload(res.payload as RegistrationPayload));
        setStatus(res.status);
        setDocuments(docs.documents);
      })
      .catch((err) => !cancelled && setError((err as { message?: string })?.message ?? "Could not load registration."));
    return () => {
      cancelled = true;
    };
  }, [tripGuestId]);

  function update(section: string, field: string, value: unknown) {
    setPayload((prev) => ({
      ...prev,
      [section]: { ...(prev[section] ?? {}), [field]: value },
    }));
  }

  async function saveDraft() {
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      const res = await api.saveGuestRegistration(tripGuestId, normalizeRegistrationPayload(payload));
      setStatus(res.status);
      setMessage("Draft saved. You can return to this registration link later.");
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not save draft.");
    } finally {
      setSaving(false);
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      const res = await api.submitGuestRegistration(tripGuestId, normalizeRegistrationPayload(payload));
      setStatus(res.status);
      setMessage(status === "submitted" ? "Changes saved." : "Registration submitted.");
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not save registration.");
    } finally {
      setSaving(false);
    }
  }

  async function uploadDocument() {
    if (!docFile) {
      setError("Choose a file to upload.");
      return;
    }
    if (docFile.size > 10 * 1024 * 1024) {
      setError("Documents must be 10 MiB or smaller.");
      return;
    }
    setUploading(true);
    setError(null);
    setMessage(null);
    try {
      const doc = await api.uploadGuestDocument(tripGuestId, {
        file: docFile,
        category: docCategory,
        display_name: docDisplayName,
        notes: docNotes,
      });
      setDocuments((prev) => [doc, ...prev]);
      setDocDisplayName("");
      setDocNotes("");
      setDocFile(null);
      setMessage("Document uploaded.");
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not upload document.");
    } finally {
      setUploading(false);
    }
  }

  const alreadySubmitted = status === "submitted";

  return (
    <div className="guest-registration-shell">
      <form className="guest-registration" onSubmit={submit}>
        <div className="guest-registration__header">
          <div>
            <h1>Guest registration</h1>
            <p className="muted">
              {alreadySubmitted
                ? "You've submitted your registration. You can still update any details before your trip."
                : "Save a draft any time and return later from your registration link."}
            </p>
          </div>
          <span className="chip chip--active">{status}</span>
        </div>
        {error && <div className="error">{error}</div>}
        {message && <div className="callout">{message}</div>}

        <RegistrationSections mode="edit" payload={payload} onChange={update} />

        <section className="registration-section">
          <div className="registration-section__header">
            <div>
              <h2>Documents</h2>
              <p className="muted">Upload passport or travel document, dive certification, insurance, waiver, medical notes, or other trip documents. PDF, JPEG, PNG, HEIC, and HEIF are accepted up to 10 MiB.</p>
            </div>
          </div>
          <div className="document-upload">
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
            <button type="button" className="secondary" disabled={uploading} onClick={uploadDocument}>
              {uploading ? "Uploading..." : "Upload document"}
            </button>
          </div>
          <DocumentList documents={documents} />
        </section>

        <div className="guest-registration__actions">
          {!alreadySubmitted && (
            <button type="button" className="secondary" onClick={saveDraft} disabled={saving}>
              Save draft
            </button>
          )}
          <button type="submit" className="primary" disabled={saving}>
            {saving
              ? "Saving..."
              : alreadySubmitted
              ? "Save changes"
              : "Submit registration"}
          </button>
        </div>
      </form>
    </div>
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

function DocumentList({ documents }: { documents: GuestDocument[] }) {
  if (documents.length === 0) {
    return <div className="muted">No documents uploaded yet.</div>;
  }
  return (
    <div className="document-list">
      {documents.map((doc) => (
        <div key={doc.id} className="document-row">
          <div>
            <strong>{doc.display_name}</strong>
            <div className="muted">
              {categoryLabel(doc.category)} · {doc.original_filename} · {formatBytes(doc.size_bytes)}
            </div>
          </div>
          <div className="document-row__actions">
            <a className="secondary" href={`${appConfig.apiBase}${doc.view_url}`} target="_blank" rel="noreferrer">View</a>
            <a className="secondary" href={`${appConfig.apiBase}${doc.download_url}`}>Download</a>
          </div>
        </div>
      ))}
    </div>
  );
}

function categoryLabel(value: string): string {
  return documentCategories.find((c) => c.value === value)?.label ?? value;
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KiB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}
