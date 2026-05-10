import { useEffect, useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type TripCabinBoard, type TripLifecycle, type TripManifest as TripManifestData } from "../api";

export function TripManifest() {
  const { id = "" } = useParams<{ id: string }>();
  const [data, setData] = useState<TripManifestData | null>(null);
  const [board, setBoard] = useState<TripCabinBoard | null>(null);
  const [lifecycle, setLifecycle] = useState<TripLifecycle | null>(null);
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [berthId, setBerthId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [transitioning, setTransitioning] = useState(false);
  const [transitionReason, setTransitionReason] = useState("");

  async function load() {
    if (!id) return;
    setError(null);
    try {
      const [manifest, cabins, life] = await Promise.all([
        adminApi.tripManifest(id),
        adminApi.tripCabinBoard(id),
        adminApi.tripLifecycle(id),
      ]);
      setData(manifest);
      setBoard(cabins);
      setLifecycle(life);
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
      await adminApi.addTripGuest(id, { full_name: fullName, email, berth_id: berthId });
      setFullName("");
      setEmail("");
      setBerthId("");
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

  async function transition(action: "start" | "complete" | "cancel") {
    setTransitioning(true);
    setError(null);
    setMessage(null);
    try {
      const acknowledged = [...new Set(lifecycle?.readiness.warnings.map((w) => w.code) ?? [])];
      if (action === "start") {
        await adminApi.startTrip(id, { acknowledged_warnings: acknowledged, reason: transitionReason });
        setMessage("Trip started.");
      } else if (action === "complete") {
        await adminApi.completeTrip(id, { acknowledged_warnings: acknowledged, reason: transitionReason });
        setMessage("Trip completed.");
      } else {
        await adminApi.cancelTrip(id, { reason: transitionReason });
        setMessage("Trip cancelled.");
      }
      setTransitionReason("");
      await load();
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not update trip lifecycle.");
    } finally {
      setTransitioning(false);
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
        <div className="header-actions">
          {lifecycle?.trip.status === "active" && <Link className="primary" to={`/admin/trips/${id}/ledger`}>Ledger</Link>}
          <Link className="secondary" to={`/admin/trips/${id}/cabins`}>Cabin board</Link>
        </div>
      </div>

      {error && <div className="error">{error}</div>}
      {message && <div className="callout">{message}</div>}

      {lifecycle && (
        <div className="admin-card lifecycle-panel">
          <div className="admin-card__title-row">
            <h2 className="admin-card__title">Lifecycle</h2>
            <span className={`chip chip--${lifecycle.trip.status}`}>{lifecycle.trip.status}</span>
          </div>
          <div className="lifecycle-issues">
            {lifecycle.readiness.blockers.map((issue, i) => (
              <div key={`b-${i}`} className="error-inline">{issue.message}</div>
            ))}
            {lifecycle.readiness.warnings.map((issue, i) => (
              <div key={`w-${i}`} className="muted">Warning: {issue.message}</div>
            ))}
            {lifecycle.readiness.blockers.length === 0 && lifecycle.readiness.warnings.length === 0 && (
              <div className="muted">No lifecycle blockers or warnings.</div>
            )}
          </div>
          <div className="lifecycle-actions">
            <input
              value={transitionReason}
              onChange={(e) => setTransitionReason(e.target.value)}
              placeholder="Reason required for warnings, override, or cancellation"
            />
            {lifecycle.trip.status === "planned" && (
              <>
                <button type="button" className="secondary" disabled={transitioning || lifecycle.readiness.blockers.length > 0} onClick={() => transition("start")}>Start trip</button>
                <button type="button" className="ghost" disabled={transitioning} onClick={() => transition("cancel")}>Cancel</button>
              </>
            )}
            {lifecycle.trip.status === "active" && (
              <button type="button" className="secondary" disabled={transitioning} onClick={() => transition("complete")}>Complete trip</button>
            )}
          </div>
        </div>
      )}

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
          <div className="field">
            <label htmlFor="guest-berth">Cabin berth</label>
            <select id="guest-berth" value={berthId} onChange={(e) => setBerthId(e.target.value)} required>
              <option value="">Select berth...</option>
              {availableBerths(board).map((b) => (
                <option key={b.id} value={b.id}>{b.label}</option>
              ))}
            </select>
          </div>
          <button className="primary" type="submit" disabled={submitting || lifecycle?.trip.status === "completed" || lifecycle?.trip.status === "cancelled"}>{submitting ? "Sending..." : "Send invite"}</button>
        </div>
      </form>

      <table className="admin-table">
        <thead>
          <tr>
            <th>Guest</th>
            <th>Email</th>
            <th>Cabin</th>
            <th>Status</th>
            <th>Invite</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {data.guests.map((g) => (
            <tr key={g.id}>
              <td>
                <Link to={`/admin/trips/${id}/guests/${g.id}`}>{g.full_name}</Link>
              </td>
              <td>{g.email}</td>
              <td>{g.cabin_assignment?.display_label ?? <span className="chip chip--warning">Needs cabin</span>}</td>
              <td><span className="chip chip--active">{statusLabel(g.status)}</span></td>
              <td>{g.invite_last_error ? <span className="error-inline">{g.invite_last_error}</span> : g.invite_expires_at ?? "—"}</td>
              <td className="actions-cell">
                <Link to={`/admin/trips/${id}/guests/${g.id}`}>Details</Link>
                <button className="secondary" type="button" onClick={() => resend(g.id)}>Resend</button>
                <button className="ghost" type="button" onClick={() => revoke(g.id)}>Revoke</button>
                <Link to={`/admin/trips/${id}/guests/${g.id}/folio`}>Checkout</Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

function statusLabel(s: string): string {
  return s.replaceAll("_", " ");
}

function availableBerths(board: TripCabinBoard | null): { id: string; label: string }[] {
  if (!board) return [];
  return board.cabins.flatMap((c) =>
    c.berths
      .filter((b) => !b.guest)
      .map((b) => ({ id: b.id, label: `${c.label} - ${b.display_label}` })),
  );
}
