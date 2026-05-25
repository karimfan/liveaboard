// fakeData generates plausible but obviously-synthetic values for
// the dev-mode "fill test data" buttons. Names use example.invalid
// for emails so production never accidentally mails them.

import {
  emptyRegistrationPayload,
  type RegistrationPayload,
} from "./registration";

const FIRST_NAMES = [
  "Aya", "Bram", "Cira", "Doro", "Eden", "Frida", "Gus",
  "Hana", "Ines", "Jules", "Kai", "Lior", "Mira", "Noa",
  "Otto", "Petra", "Quinn", "Rea", "Saba", "Tomi",
  "Una", "Vesa", "Wim", "Xan", "Yara", "Zev",
];

const LAST_NAMES = [
  "Akao", "Bekker", "Cabrera", "Dasari", "Erlich", "Frye",
  "Goto", "Halim", "Iyer", "Joshi", "Kovac", "Lopes",
  "Mireles", "Naumov", "Okafor", "Patel", "Qureshi",
  "Rasmussen", "Sato", "Tahir", "Uribe", "Vega",
  "Wessels", "Xu", "Yamada", "Zammit",
];

const ISSUING_COUNTRIES = ["NL", "GB", "US", "AU", "DE", "FR", "JP", "ZA", "BR", "ES"];
const NATIONALITIES = ["Dutch", "British", "American", "Australian", "German", "French", "Japanese", "South African", "Brazilian", "Spanish"];
const CERT_AGENCIES = ["PADI", "SSI", "NAUI", "RAID"];
const CERT_LEVELS = ["Open Water", "Advanced", "Rescue", "Divemaster", "Instructor"];

function pick<T>(xs: T[]): T {
  return xs[Math.floor(Math.random() * xs.length)];
}

function short(): string {
  // 4 alphanumeric chars — keeps email collisions vanishingly rare
  // across a single fill-the-boat click.
  return Math.random().toString(36).slice(2, 6);
}

// fakeGuestIdentity returns a fresh (full_name, email) pair for the
// boat-fill action on the manifest page.
export function fakeGuestIdentity(): { full_name: string; email: string } {
  const first = pick(FIRST_NAMES);
  const last = pick(LAST_NAMES);
  return {
    full_name: `${first} ${last}`,
    email: `${first.toLowerCase()}.${last.toLowerCase()}.${short()}@example.invalid`,
  };
}

// fakeRegistrationPayload returns a fully-populated registration
// payload for the "Fill test data" button on the guest registration
// page. Field names match the live form so the round-trip works.
export function fakeRegistrationPayload(): RegistrationPayload {
  const { full_name: legalName } = fakeGuestIdentity();
  const dob = randomDateOfBirth();
  const arrival = futureDateTime(7);
  const departure = futureDateTime(14);
  const documentExpiry = futureDate(365 * 5);
  const insuranceExpiry = futureDate(365);
  const country = pick(ISSUING_COUNTRIES);
  return {
    ...emptyRegistrationPayload,
    identity: {
      legal_name: legalName,
      preferred_name: legalName.split(" ")[0],
      date_of_birth: dob,
      nationality: pick(NATIONALITIES),
      country_of_residence: pick(NATIONALITIES),
      phone: "+1 555 " + short(),
    },
    travel_document: {
      document_type: "passport",
      document_number: country + short().toUpperCase() + short().toUpperCase(),
      issuing_country: country,
      expires_on: documentExpiry,
      will_provide_later: false,
    },
    travel_logistics: {
      arrival_from: "AMS",
      arrival_flight_number: "KL" + short().slice(0, 3).toUpperCase(),
      arrival_at: arrival,
      arrival_location: "Bali",
      departure_to: "AMS",
      departure_flight_number: "KL" + short().slice(0, 3).toUpperCase(),
      departure_at: departure,
      departure_location: "Bali",
      hotel_before_trip: "Hotel Tugu",
      hotel_after_trip: "Hotel Tugu",
    },
    emergency_contact: {
      name: "Test Contact " + short(),
      relationship: "Partner",
      phone: "+1 555 " + short(),
      email: `emergency.${short()}@example.invalid`,
    },
    dive_insurance: {
      provider: "DAN",
      policy_number: "DAN-" + short().toUpperCase() + short().toUpperCase(),
      expires_on: insuranceExpiry,
      will_provide_later: false,
    },
    dive_profile: {
      certification_agency: pick(CERT_AGENCIES),
      certification_level: pick(CERT_LEVELS),
      logged_dives: String(50 + Math.floor(Math.random() * 500)),
      last_dive_on: pastDate(180),
      nitrox_certified: Math.random() < 0.5,
      strong_current_experience: Math.random() < 0.5,
      camera: Math.random() < 0.3,
    },
    dietary: {
      dietary_requirements: "",
      allergies: "",
      medical_notes: "",
      no_dietary_or_allergy_notes: true,
    },
    rental_gear: {
      needs_rental_gear: false,
      items: "",
      height: "",
      weight: "",
      bcd_size: "",
      wetsuit_size: "",
      fins_size: "",
      notes: "",
    },
    notes: {
      general: "Synthetic test profile (filesystem email mode).",
      destination_or_permit_notes: "",
    },
  };
}

function randomDateOfBirth(): string {
  // 25-55 years ago.
  const now = new Date();
  const year = now.getUTCFullYear() - (25 + Math.floor(Math.random() * 30));
  const month = 1 + Math.floor(Math.random() * 12);
  const day = 1 + Math.floor(Math.random() * 28);
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

function futureDate(daysOut: number): string {
  const d = new Date(Date.now() + daysOut * 24 * 60 * 60 * 1000);
  return d.toISOString().slice(0, 10);
}

function pastDate(daysBack: number): string {
  const d = new Date(Date.now() - daysBack * 24 * 60 * 60 * 1000);
  return d.toISOString().slice(0, 10);
}

function futureDateTime(daysOut: number): string {
  // datetime-local format: YYYY-MM-DDTHH:MM
  const d = new Date(Date.now() + daysOut * 24 * 60 * 60 * 1000);
  return d.toISOString().slice(0, 16);
}
