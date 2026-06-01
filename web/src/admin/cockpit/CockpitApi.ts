import { appConfig } from "../../lib/config";

export type CockpitResponse = {
  identity: {
    user_id: string;
    organization_id: string;
    role: "org_admin" | "cruise_director";
    assigned_voyages: number;
    generated_at: string;
  };
  admin_cockpit?: {
    setup_percent: number;
    boat_count: number;
    trip_count: number;
    director_count: number;
    lead_count: number;
    quote_count: number;
    active_voyages: number;
    upcoming_voyages: number;
  };
  director_cockpit?: {
    assigned_voyages: number;
    active_voyages: number;
    upcoming_voyages: number;
    open_folios: number;
    blockers: number;
  };
  voyage_lanes: CockpitVoyage[];
  blockers: CockpitSignal[];
  money: {
    open_folio_count: number;
    closed_folio_count: number;
    outstanding_usd_cents: number;
    settled_usd_cents: number;
    deposit_pending_quotes: number;
    held_quotes: number;
    offline_deposit_usd_cents: number;
    offline_refund_usd_cents: number;
  };
  inventory: CockpitInventory[];
  activity: CockpitActivity[];
  commands: CockpitCommand[];
  failed_sections?: string[];
};

export type CockpitVoyage = {
  id: string;
  boat_id: string;
  boat_name: string;
  itinerary: string;
  start_date: string;
  end_date: string;
  status: "planned" | "active" | "completed" | "cancelled";
  expected_guests?: number;
  guest_count: number;
  submitted_count: number;
  cabin_assignments: number;
  director_count: number;
  open_folio_count: number;
  outstanding_usd_cents: number;
  blocker_count: number;
};

export type CockpitSignal = {
  kind: string;
  severity: "blocker" | "warning" | "info";
  label: string;
  detail: string;
  trip_id?: string;
  boat_id?: string;
  entity_id?: string;
  route: string;
};

export type CockpitInventory = {
  boat_id: string;
  boat_name: string;
  item_name: string;
  status: "out" | "low" | string;
  quantity: number;
  route: string;
};

export type CockpitActivity = {
  id: string;
  action: string;
  entity: string;
  trip_id?: string;
  created_at: string;
};

export type CockpitCommand = {
  label: string;
  route: string;
  kind: string;
};

export async function fetchCockpit(): Promise<CockpitResponse> {
  const resp = await fetch(`${appConfig.apiBase}/admin/cockpit`, {
    credentials: "include",
  });
  if (!resp.ok) {
    throw new Error(`Cockpit failed (${resp.status})`);
  }
  return (await resp.json()) as CockpitResponse;
}
