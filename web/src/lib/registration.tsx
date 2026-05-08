import { type ReactNode } from "react";

export type RegistrationPayload = Record<string, any>;

export const emptyRegistrationPayload: RegistrationPayload = {
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

export function mergeRegistrationPayload(payload: RegistrationPayload | null | undefined): RegistrationPayload {
  const merged: RegistrationPayload = { ...emptyRegistrationPayload };
  for (const key of Object.keys(emptyRegistrationPayload)) {
    merged[key] = { ...emptyRegistrationPayload[key], ...((payload?.[key] as Record<string, unknown>) ?? {}) };
  }
  return merged;
}

export function normalizeRegistrationPayload(payload: RegistrationPayload): RegistrationPayload {
  const loggedDives = payload.dive_profile?.logged_dives;
  return {
    ...payload,
    dive_profile: {
      ...payload.dive_profile,
      logged_dives: loggedDives === "" || loggedDives == null ? null : Number(loggedDives),
    },
  };
}

type Mode = "edit" | "read";

type Props = {
  mode: Mode;
  payload: RegistrationPayload;
  onChange?: (section: string, field: string, value: unknown) => void;
};

// RegistrationSections renders the registration form in either edit or read
// mode. Edit mode is what guests see; read mode is what staff see on the
// per-guest detail page. Field structure stays identical between the two so
// labels never drift.
export function RegistrationSections({ mode, payload, onChange }: Props) {
  return (
    <>
      <Section title="Identity">
        <Field mode={mode} label="Legal name" value={payload.identity.legal_name} onChange={(v) => onChange?.("identity", "legal_name", v)} required />
        <Field mode={mode} label="Preferred name" value={payload.identity.preferred_name} onChange={(v) => onChange?.("identity", "preferred_name", v)} />
        <Field mode={mode} label="Date of birth" type="date" value={payload.identity.date_of_birth} onChange={(v) => onChange?.("identity", "date_of_birth", v)} required />
        <Field mode={mode} label="Nationality" value={payload.identity.nationality} onChange={(v) => onChange?.("identity", "nationality", v)} required />
        <Field mode={mode} label="Phone" value={payload.identity.phone} onChange={(v) => onChange?.("identity", "phone", v)} />
      </Section>

      <Section title="Travel document">
        <Field mode={mode} label="Document number" value={payload.travel_document.document_number} onChange={(v) => onChange?.("travel_document", "document_number", v)} />
        <Field mode={mode} label="Issuing country" value={payload.travel_document.issuing_country} onChange={(v) => onChange?.("travel_document", "issuing_country", v)} />
        <Field mode={mode} label="Expires on" type="date" value={payload.travel_document.expires_on} onChange={(v) => onChange?.("travel_document", "expires_on", v)} />
        <Check mode={mode} label="I will provide this later" checked={payload.travel_document.will_provide_later} onChange={(v) => onChange?.("travel_document", "will_provide_later", v)} />
      </Section>

      <Section title="Travel logistics">
        <Field mode={mode} label="Arrival from" value={payload.travel_logistics.arrival_from} onChange={(v) => onChange?.("travel_logistics", "arrival_from", v)} />
        <Field mode={mode} label="Arrival flight" value={payload.travel_logistics.arrival_flight_number} onChange={(v) => onChange?.("travel_logistics", "arrival_flight_number", v)} />
        <Field mode={mode} label="Arrival location" value={payload.travel_logistics.arrival_location} onChange={(v) => onChange?.("travel_logistics", "arrival_location", v)} />
        <Field mode={mode} label="Departure to" value={payload.travel_logistics.departure_to} onChange={(v) => onChange?.("travel_logistics", "departure_to", v)} />
        <Field mode={mode} label="Hotel before trip" value={payload.travel_logistics.hotel_before_trip} onChange={(v) => onChange?.("travel_logistics", "hotel_before_trip", v)} />
        <Field mode={mode} label="Hotel after trip" value={payload.travel_logistics.hotel_after_trip} onChange={(v) => onChange?.("travel_logistics", "hotel_after_trip", v)} />
      </Section>

      <Section title="Emergency contact">
        <Field mode={mode} label="Name" value={payload.emergency_contact.name} onChange={(v) => onChange?.("emergency_contact", "name", v)} required />
        <Field mode={mode} label="Relationship" value={payload.emergency_contact.relationship} onChange={(v) => onChange?.("emergency_contact", "relationship", v)} />
        <Field mode={mode} label="Phone" value={payload.emergency_contact.phone} onChange={(v) => onChange?.("emergency_contact", "phone", v)} required />
        <Field mode={mode} label="Email" type="email" value={payload.emergency_contact.email} onChange={(v) => onChange?.("emergency_contact", "email", v)} />
      </Section>

      <Section title="Diving">
        <Field mode={mode} label="Certification agency" value={payload.dive_profile.certification_agency} onChange={(v) => onChange?.("dive_profile", "certification_agency", v)} required />
        <Field mode={mode} label="Certification level" value={payload.dive_profile.certification_level} onChange={(v) => onChange?.("dive_profile", "certification_level", v)} required />
        <Field mode={mode} label="Logged dives" type="number" value={payload.dive_profile.logged_dives} onChange={(v) => onChange?.("dive_profile", "logged_dives", v)} required />
        <Field mode={mode} label="Insurance provider" value={payload.dive_insurance.provider} onChange={(v) => onChange?.("dive_insurance", "provider", v)} />
        <Field mode={mode} label="Policy number" value={payload.dive_insurance.policy_number} onChange={(v) => onChange?.("dive_insurance", "policy_number", v)} />
        <Check mode={mode} label="I will provide insurance details later" checked={payload.dive_insurance.will_provide_later} onChange={(v) => onChange?.("dive_insurance", "will_provide_later", v)} />
      </Section>

      <Section title="Dietary and gear">
        <Field mode={mode} label="Dietary requirements" value={payload.dietary.dietary_requirements} onChange={(v) => onChange?.("dietary", "dietary_requirements", v)} />
        <Field mode={mode} label="Allergies" value={payload.dietary.allergies} onChange={(v) => onChange?.("dietary", "allergies", v)} />
        <Check mode={mode} label="No dietary requirements or allergies to report" checked={payload.dietary.no_dietary_or_allergy_notes} onChange={(v) => onChange?.("dietary", "no_dietary_or_allergy_notes", v)} />
        <Check mode={mode} label="I need rental gear" checked={payload.rental_gear.needs_rental_gear} onChange={(v) => onChange?.("rental_gear", "needs_rental_gear", v)} />
        <Field mode={mode} label="Gear notes" value={payload.rental_gear.notes} onChange={(v) => onChange?.("rental_gear", "notes", v)} />
      </Section>

      <Section title="Notes">
        <Textarea mode={mode} label="General notes" value={payload.notes.general} onChange={(v) => onChange?.("notes", "general", v)} />
      </Section>
    </>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="registration-section">
      <h2>{title}</h2>
      <div className="form-grid">{children}</div>
    </section>
  );
}

type FieldProps = {
  mode: Mode;
  label: string;
  value: unknown;
  onChange: (v: string) => void;
  type?: string;
  required?: boolean;
};

function Field({ mode, label, value, onChange, type = "text", required = false }: FieldProps) {
  const id = label.toLowerCase().replaceAll(" ", "-");
  if (mode === "read") {
    return (
      <div className="field field--read">
        <label>{label}</label>
        <div className="field__readout">{formatValue(value, type)}</div>
      </div>
    );
  }
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type={type}
        value={(value as string | number | undefined) ?? ""}
        onChange={(e) => onChange(e.target.value)}
        required={required}
      />
    </div>
  );
}

function Textarea({ mode, label, value, onChange }: { mode: Mode; label: string; value: string; onChange: (v: string) => void }) {
  if (mode === "read") {
    return (
      <div className="field field--read field--full">
        <label>{label}</label>
        <div className="field__readout field__readout--block">{formatValue(value, "text") || "—"}</div>
      </div>
    );
  }
  return (
    <>
      <label>{label}</label>
      <textarea rows={5} value={value ?? ""} onChange={(e) => onChange(e.target.value)} />
    </>
  );
}

function Check({ mode, label, checked, onChange }: { mode: Mode; label: string; checked: boolean; onChange: (v: boolean) => void }) {
  if (mode === "read") {
    return (
      <div className="field field--read">
        <label>{label}</label>
        <div className="field__readout">{checked ? "Yes" : "No"}</div>
      </div>
    );
  }
  return (
    <label className="check-row">
      <input type="checkbox" checked={!!checked} onChange={(e) => onChange(e.target.checked)} /> {label}
    </label>
  );
}

function formatValue(value: unknown, type: string): string {
  if (value === null || value === undefined) return "—";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  const s = String(value).trim();
  if (s === "") return "—";
  if (type === "date") return s;
  return s;
}
