import { useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type TripGuest, type TripManifest as TripManifestData } from "../api";

export function TripManifest() {
  const { id = "" } = useParams<{ id: string }>();
  const [data, setData] = useState<TripManifestData | null>(null);
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [detail, setDetail] = useState<{ guest: TripGuest; payload: unknown } | null>(null);

  async function load() {
    if (!id) return;
    setError(null);
    try {
      setData(await adminApi.tripManifest(id));
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to load manifest.");
    }
  }

  useEffect(() => {
    void load();
  }, [id]);

  async function addGuest(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setMessage(null);
    try {
      await adminApi.addTripGuest(id, { full_name: fullName, email });
      setFullName("");
      setEmail("");
      setMessage("Registration invite sent.");
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to add guest.");
    } finally {
      setSubmitting(false);
    }
  }

  async function resend(guestId: string) {
    setError(null);
    setMessage(null);
    try {
      await adminApi.resendTripGuestInvite(id, guestId);
      setMessage("Registration invite resent.");
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to resend invite.");
    }
  }

  async function revoke(guestId: string) {
    setError(null);
    setMessage(null);
    try {
      await adminApi.revokeTripGuestInvite(id, guestId);
      setMessage("Invite revoked.");
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Failed to revoke invite.");
    }
  }

  async function viewRegistration(guest: TripGuest) {
    setError(null);
    try {
      const reg = await adminApi.guestRegistration(id, guest.id);
      setDetail({ guest, payload: reg.payload });
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Submitted registration is not available.");
    }
  }

  if (!data) {
    return (
      <>
        <div className="admin-breadcrumb"><Link to="/admin/trips">Trips</Link></div>
        {error ? <div className="error">{error}</div> : <div className="muted">Loading...</div>}
      </>
    );
  }

  return (
    <>
      <div className="admin-breadcrumb"><Link to="/admin/trips">Trips</Link></div>
      <div className="admin-page-header">
        <div>
          <h1 className="admin-page-title">Manifest</h1>
          <div className="admin-page-subtitle">
            {data.trip.boat_name} - {data.trip.start_date} to {data.trip.end_date} - {data.trip.itinerary}
          </div>
        </div>
      </div>

      {error && <div className="error">{error}</div>}
      {message && <div className="callout">{message}</div>}

      <div className="manifest-summary">
        <div><strong>{data.summary.guest_count}</strong><span>Guests</span></div>
        <div><strong>{data.summary.submitted_count}</strong><span>Submitted</span></div>
        <div><strong>{data.summary.expected_count ?? "—"}</strong><span>Expected</span></div>
        {data.summary.has_warning && <div className="manifest-warning">Above expected count</div>}
      </div>

      <form className="admin-card manifest-add" onSubmit={addGuest}>
        <h2 className="admin-card__title">Add guest</h2>
        <div className="form-grid">
          <div className="field">
            <label htmlFor="guest-name">Full name</label>
            <input id="guest-name" value={fullName} onChange={(e) => setFullName(e.target.value)} required />
          </div>
          <div className="field">
            <label htmlFor="guest-email">Email</label>
            <input id="guest-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <button className="primary" type="submit" disabled={submitting}>{submitting ? "Sending..." : "Send invite"}</button>
        </div>
      </form>

      <table className="admin-table">
        <thead>
          <tr>
            <th>Guest</th>
            <th>Email</th>
            <th>Status</th>
            <th>Invite</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {data.guests.map((g) => (
            <tr key={g.id}>
              <td>{g.full_name}</td>
              <td>{g.email}</td>
              <td><span className="chip chip--active">{statusLabel(g.status)}</span></td>
              <td>{g.invite_last_error ? <span className="error-inline">{g.invite_last_error}</span> : g.invite_expires_at ?? "—"}</td>
              <td className="actions-cell">
                <button className="secondary" type="button" onClick={() => resend(g.id)}>Resend</button>
                <button className="ghost" type="button" onClick={() => revoke(g.id)}>Revoke</button>
                {g.status === "submitted" && <button className="secondary" type="button" onClick={() => viewRegistration(g)}>View</button>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {detail && (
        <div className="modal-backdrop" onClick={() => setDetail(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{detail.guest.full_name}</h2>
              <button className="ghost" onClick={() => setDetail(null)}>Close</button>
            </div>
            <pre className="registration-json">{JSON.stringify(detail.payload, null, 2)}</pre>
          </div>
        </div>
      )}
    </>
  );
}

function statusLabel(s: string): string {
  return s.replaceAll("_", " ");
}
