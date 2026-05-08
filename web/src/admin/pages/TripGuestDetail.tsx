import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { adminApi, type GuestRegistrationDetail, type Trip } from "../api";
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
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setError(null);

    Promise.all([adminApi.guestRegistration(id, guestId), adminApi.tripManifest(id)])
      .then(([reg, manifest]) => {
        if (cancelled) return;
        setDetail(reg);
        setTrip(manifest.trip);
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

      {reg && (
        <div className="admin-card">
          <div className="admin-card__title-row">
            <h2 className="admin-card__title">Registration</h2>
            <span className="chip chip--active">{reg.status}</span>
          </div>
          <RegistrationSections mode="read" payload={payload} />
        </div>
      )}
    </>
  );
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString();
}
