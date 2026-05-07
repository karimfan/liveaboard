import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useParams } from "react-router-dom";

import { api } from "../lib/api";

type Payload = Record<string, any>;

const emptyPayload: Payload = {
  identity: { legal_name: "", preferred_name: "", date_of_birth: "", nationality: "", country_of_residence: "", phone: "" },
  travel_document: { document_type: "passport", document_number: "", issuing_country: "", expires_on: "", will_provide_later: false },
  travel_logistics: { arrival_from: "", arrival_flight_number: "", arrival_at: "", arrival_location: "", departure_to: "", departure_flight_number: "", departure_at: "", departure_location: "", hotel_before_trip: "", hotel_after_trip: "" },
  emergency_contact: { name: "", relationship: "", phone: "", email: "" },
  dive_insurance: { provider: "", policy_number: "", expires_on: "", will_provide_later: false },
  dive_profile: { certification_agency: "", certification_level: "", logged_dives: "", last_dive_on: "", nitrox_certified: false, strong_current_experience: false, camera: false },
  dietary: { dietary_requirements: "", allergies: "", medical_notes: "", no_dietary_or_allergy_notes: false },
  rental_gear: { needs_rental_gear: false, items: "", height: "", weight: "", bcd_size: "", wetsuit_size: "", fins_size: "", notes: "" },
  notes: { general: "", destination_or_permit_notes: "" },
};

export function GuestRegistration() {
  const { tripGuestId = "" } = useParams<{ tripGuestId: string }>();
  const [payload, setPayload] = useState<Payload>(emptyPayload);
  const [status, setStatus] = useState<"draft" | "submitted">("draft");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.guestRegistration(tripGuestId)
      .then((res) => {
        if (cancelled) return;
        setPayload(mergePayload(res.payload as Payload));
        setStatus(res.status);
      })
      .catch((err) => !cancelled && setError((err as { message?: string })?.message ?? "Could not load registration."));
    return () => {
      cancelled = true;
    };
  }, [tripGuestId]);

  function update(section: string, field: string, value: any) {
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
      const res = await api.saveGuestRegistration(tripGuestId, normalizePayload(payload));
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
      const res = await api.submitGuestRegistration(tripGuestId, normalizePayload(payload));
      setStatus(res.status);
      setMessage("Registration submitted.");
    } catch (err) {
      setError((err as { message?: string })?.message ?? "Could not submit registration.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="guest-registration-shell">
      <form className="guest-registration" onSubmit={submit}>
        <div className="guest-registration__header">
          <div>
            <h1>Guest registration</h1>
            <p className="muted">Save a draft any time and return later from your registration link.</p>
          </div>
          <span className="chip chip--active">{status}</span>
        </div>
        {error && <div className="error">{error}</div>}
        {message && <div className="callout">{message}</div>}

        <Section title="Identity">
          <Field label="Legal name" value={payload.identity.legal_name} onChange={(v) => update("identity", "legal_name", v)} required />
          <Field label="Preferred name" value={payload.identity.preferred_name} onChange={(v) => update("identity", "preferred_name", v)} />
          <Field label="Date of birth" type="date" value={payload.identity.date_of_birth} onChange={(v) => update("identity", "date_of_birth", v)} required />
          <Field label="Nationality" value={payload.identity.nationality} onChange={(v) => update("identity", "nationality", v)} required />
          <Field label="Phone" value={payload.identity.phone} onChange={(v) => update("identity", "phone", v)} />
        </Section>

        <Section title="Travel document">
          <Field label="Document number" value={payload.travel_document.document_number} onChange={(v) => update("travel_document", "document_number", v)} />
          <Field label="Issuing country" value={payload.travel_document.issuing_country} onChange={(v) => update("travel_document", "issuing_country", v)} />
          <Field label="Expires on" type="date" value={payload.travel_document.expires_on} onChange={(v) => update("travel_document", "expires_on", v)} />
          <Check label="I will provide this later" checked={payload.travel_document.will_provide_later} onChange={(v) => update("travel_document", "will_provide_later", v)} />
        </Section>

        <Section title="Travel logistics">
          <Field label="Arrival from" value={payload.travel_logistics.arrival_from} onChange={(v) => update("travel_logistics", "arrival_from", v)} />
          <Field label="Arrival flight" value={payload.travel_logistics.arrival_flight_number} onChange={(v) => update("travel_logistics", "arrival_flight_number", v)} />
          <Field label="Arrival location" value={payload.travel_logistics.arrival_location} onChange={(v) => update("travel_logistics", "arrival_location", v)} />
          <Field label="Departure to" value={payload.travel_logistics.departure_to} onChange={(v) => update("travel_logistics", "departure_to", v)} />
          <Field label="Hotel before trip" value={payload.travel_logistics.hotel_before_trip} onChange={(v) => update("travel_logistics", "hotel_before_trip", v)} />
          <Field label="Hotel after trip" value={payload.travel_logistics.hotel_after_trip} onChange={(v) => update("travel_logistics", "hotel_after_trip", v)} />
        </Section>

        <Section title="Emergency contact">
          <Field label="Name" value={payload.emergency_contact.name} onChange={(v) => update("emergency_contact", "name", v)} required />
          <Field label="Relationship" value={payload.emergency_contact.relationship} onChange={(v) => update("emergency_contact", "relationship", v)} />
          <Field label="Phone" value={payload.emergency_contact.phone} onChange={(v) => update("emergency_contact", "phone", v)} required />
          <Field label="Email" type="email" value={payload.emergency_contact.email} onChange={(v) => update("emergency_contact", "email", v)} />
        </Section>

        <Section title="Diving">
          <Field label="Certification agency" value={payload.dive_profile.certification_agency} onChange={(v) => update("dive_profile", "certification_agency", v)} required />
          <Field label="Certification level" value={payload.dive_profile.certification_level} onChange={(v) => update("dive_profile", "certification_level", v)} required />
          <Field label="Logged dives" type="number" value={payload.dive_profile.logged_dives} onChange={(v) => update("dive_profile", "logged_dives", v)} required />
          <Field label="Insurance provider" value={payload.dive_insurance.provider} onChange={(v) => update("dive_insurance", "provider", v)} />
          <Field label="Policy number" value={payload.dive_insurance.policy_number} onChange={(v) => update("dive_insurance", "policy_number", v)} />
          <Check label="I will provide insurance details later" checked={payload.dive_insurance.will_provide_later} onChange={(v) => update("dive_insurance", "will_provide_later", v)} />
        </Section>

        <Section title="Dietary and gear">
          <Field label="Dietary requirements" value={payload.dietary.dietary_requirements} onChange={(v) => update("dietary", "dietary_requirements", v)} />
          <Field label="Allergies" value={payload.dietary.allergies} onChange={(v) => update("dietary", "allergies", v)} />
          <Check label="No dietary requirements or allergies to report" checked={payload.dietary.no_dietary_or_allergy_notes} onChange={(v) => update("dietary", "no_dietary_or_allergy_notes", v)} />
          <Check label="I need rental gear" checked={payload.rental_gear.needs_rental_gear} onChange={(v) => update("rental_gear", "needs_rental_gear", v)} />
          <Field label="Gear notes" value={payload.rental_gear.notes} onChange={(v) => update("rental_gear", "notes", v)} />
        </Section>

        <Section title="Notes">
          <label>General notes</label>
          <textarea rows={5} value={payload.notes.general} onChange={(e) => update("notes", "general", e.target.value)} />
        </Section>

        <div className="guest-registration__actions">
          <button type="button" className="secondary" onClick={saveDraft} disabled={saving}>Save draft</button>
          <button type="submit" className="primary" disabled={saving}>{saving ? "Saving..." : "Submit registration"}</button>
        </div>
      </form>
    </div>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return <section className="registration-section"><h2>{title}</h2><div className="form-grid">{children}</div></section>;
}

function Field({ label, value, onChange, type = "text", required = false }: { label: string; value: string; onChange: (v: string) => void; type?: string; required?: boolean }) {
  const id = label.toLowerCase().replaceAll(" ", "-");
  return <div className="field"><label htmlFor={id}>{label}</label><input id={id} type={type} value={value ?? ""} onChange={(e) => onChange(e.target.value)} required={required} /></div>;
}

function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return <label className="check-row"><input type="checkbox" checked={!!checked} onChange={(e) => onChange(e.target.checked)} /> {label}</label>;
}

function mergePayload(payload: Payload): Payload {
  const merged: Payload = { ...emptyPayload };
  for (const key of Object.keys(emptyPayload)) {
    merged[key] = { ...emptyPayload[key], ...(payload?.[key] ?? {}) };
  }
  return merged;
}

function normalizePayload(payload: Payload): Payload {
  return {
    ...payload,
    dive_profile: {
      ...payload.dive_profile,
      logged_dives: payload.dive_profile.logged_dives === "" ? null : Number(payload.dive_profile.logged_dives),
    },
  };
}
