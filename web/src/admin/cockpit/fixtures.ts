import type { CockpitResponse } from "./CockpitApi";

const now = new Date("2026-05-31T18:00:00Z").toISOString();

export const adminCockpitFixture: CockpitResponse = {
  identity: {
    user_id: "fixture-admin",
    organization_id: "fixture-org",
    role: "org_admin",
    assigned_voyages: 0,
    generated_at: now,
  },
  admin_cockpit: {
    setup_percent: 86,
    boat_count: 4,
    trip_count: 18,
    director_count: 5,
    lead_count: 9,
    quote_count: 6,
    active_voyages: 3,
    upcoming_voyages: 2,
  },
  voyage_lanes: [
    voyage("v1", "Solitude Adventurer", "Komodo Deep South", "active", 18, 16, 14, 2, 4, 728500),
    voyage("v2", "Manta Queen", "Raja Ampat Central", "active", 22, 20, 20, 2, 7, 1184000),
    voyage("v3", "Ari Explorer", "North Atolls", "active", 16, 16, 15, 1, 2, 412000),
    voyage("v4", "Solitude Adventurer", "Forgotten Islands", "planned", 18, 12, 10, 0, 0, 0),
    voyage("v5", "Manta Queen", "Banda Sea", "planned", 20, 19, 17, 1, 0, 0),
  ],
  blockers: [
    signal("director", "blocker", "Director unassigned", "Forgotten Islands · 13 days out", "/admin/trips"),
    signal("crew_cert", "blocker", "Crew certification", "Rafi · Dive Master expired", "/admin/users"),
    signal("equipment", "warning", "Equipment readiness", "Manta Queen · O2 kit service due", "/admin/fleet"),
    signal("cabins", "warning", "Cabins incomplete", "Banda Sea · 2 berths unassigned", "/admin/trips/v5/manifest"),
  ],
  money: {
    open_folio_count: 13,
    closed_folio_count: 31,
    outstanding_usd_cents: 2342500,
    settled_usd_cents: 8129000,
    deposit_pending_quotes: 4,
    held_quotes: 2,
    offline_deposit_usd_cents: 1840000,
    offline_refund_usd_cents: 120000,
  },
  inventory: [
    { boat_id: "b1", boat_name: "Manta Queen", item_name: "Nitrox 32", status: "low", quantity: 4, route: "/admin/inventory" },
    { boat_id: "b2", boat_name: "Ari Explorer", item_name: "Reef hooks", status: "out", quantity: 0, route: "/admin/inventory" },
  ],
  activity: [
    activity("quote.sent", "booking_quote"),
    activity("guest.folio_closed", "guest_folio"),
    activity("trip.started", "trip"),
    activity("guest.document_uploaded", "guest_document"),
    activity("inventory.adjusted", "stock_movement"),
    activity("guest.invited", "trip_guest"),
  ],
  commands: adminCommands(),
};

export const directorCockpitFixture: CockpitResponse = {
  identity: {
    user_id: "fixture-director",
    organization_id: "fixture-org",
    role: "cruise_director",
    assigned_voyages: 2,
    generated_at: now,
  },
  director_cockpit: {
    assigned_voyages: 2,
    active_voyages: 1,
    upcoming_voyages: 1,
    open_folios: 4,
    blockers: 2,
  },
  voyage_lanes: [
    voyage("d1", "Manta Queen", "Raja Ampat Central", "active", 22, 20, 20, 1, 4, 618000),
    voyage("d2", "Manta Queen", "Banda Sea", "planned", 20, 19, 17, 1, 0, 0),
  ],
  blockers: [
    signal("cabins", "warning", "Cabins incomplete", "Banda Sea · 2 berths unassigned", "/admin/trips/d2/manifest"),
    signal("folio", "warning", "Open folios", "4 guests still open on Raja Ampat Central", "/admin/trips/d1/ledger"),
  ],
  money: {
    open_folio_count: 4,
    closed_folio_count: 12,
    outstanding_usd_cents: 618000,
    settled_usd_cents: 2204000,
    deposit_pending_quotes: 0,
    held_quotes: 0,
    offline_deposit_usd_cents: 0,
    offline_refund_usd_cents: 0,
  },
  inventory: [],
  activity: [
    activity("guest.folio_line_added", "guest_folio_line"),
    activity("guest.cabin_assigned", "trip_cabin_assignment"),
    activity("guest.registration_submitted", "guest_registration"),
  ],
  commands: [
    { label: "Today cockpit", route: "/admin", kind: "core" },
    { label: "Trips", route: "/admin/trips", kind: "operations" },
    { label: "Audit", route: "/admin/audit", kind: "insights" },
    { label: "My account", route: "/admin/account", kind: "profile" },
  ],
};

function voyage(
  id: string,
  boat: string,
  itinerary: string,
  status: "planned" | "active",
  expected: number,
  guests: number,
  cabins: number,
  blockers: number,
  folios: number,
  outstanding: number,
) {
  return {
    id,
    boat_id: `${id}-boat`,
    boat_name: boat,
    itinerary,
    start_date: "2026-06-08T00:00:00Z",
    end_date: "2026-06-18T00:00:00Z",
    status,
    expected_guests: expected,
    guest_count: guests,
    submitted_count: Math.max(0, guests - 2),
    cabin_assignments: cabins,
    director_count: status === "active" ? 1 : blockers > 0 ? 0 : 1,
    open_folio_count: folios,
    outstanding_usd_cents: outstanding,
    blocker_count: blockers,
  };
}

function signal(kind: string, severity: "blocker" | "warning", label: string, detail: string, route: string) {
  return { kind, severity, label, detail, route };
}

function activity(action: string, entity: string) {
  return {
    id: `${action}-${entity}`,
    action,
    entity,
    created_at: now,
  };
}

function adminCommands() {
  return [
    { label: "Today cockpit", route: "/admin", kind: "core" },
    { label: "Trips", route: "/admin/trips", kind: "operations" },
    { label: "Import trips", route: "/admin/import", kind: "operations" },
    { label: "Inventory", route: "/admin/inventory", kind: "operations" },
    { label: "Fleet", route: "/admin/fleet", kind: "configuration" },
    { label: "Users", route: "/admin/users", kind: "configuration" },
    { label: "Payments", route: "/admin/organization/payments", kind: "configuration" },
    { label: "Reports", route: "/admin/reports", kind: "insights" },
  ];
}
