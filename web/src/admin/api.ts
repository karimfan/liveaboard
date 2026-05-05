import { appConfig } from "../lib/config";

export type ApiError = { error: string; message: string };

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const resp = await fetch(`${appConfig.apiBase}${path}`, {
    method,
    credentials: "include",
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await resp.text();
  const parsed = text ? (JSON.parse(text) as unknown) : null;
  if (!resp.ok) {
    throw (parsed ?? { error: "unknown", message: resp.statusText }) as ApiError;
  }
  return parsed as T;
}

// --- types ---

export type SetupStep = {
  key: string;
  label: string;
  done: boolean;
  hint: string;
  href: string;
};

export type Overview = {
  setup: { pct: number; steps: SetupStep[] };
  counts: { boats: number; trips: number; cruise_directors: number };
  trips_needing_attention: {
    id: string;
    boat_name: string;
    itinerary: string;
    start_date: string;
    end_date: string;
    reason: string;
  }[];
};

export type Boat = {
  id: string;
  slug: string;
  name: string;
  source_name: string;
  image_url: string | null;
  source_url: string;
  last_synced: string;
};

export type Trip = {
  id: string;
  boat_id: string;
  boat_name: string;
  start_date: string;
  end_date: string;
  itinerary: string;
  departure_port: string | null;
  return_port: string | null;
  price_text: string | null;
  availability_text: string | null;
  cruise_director_user_id: string | null;
  cruise_director_name: string | null;
};

export type AdminUser = {
  id: string;
  email: string;
  full_name: string;
  phone: string | null;
  role: "org_admin" | "cruise_director";
  is_active: boolean;
};

export type Organization = {
  id: string;
  name: string;
  currency: string | null;
  created_at: string;
  updated_at: string;
};

// --- endpoints ---

export const adminApi = {
  overview: () => call<Overview>("GET", "/admin/overview"),

  listBoats: () => call<{ boats: Boat[] }>("GET", "/admin/boats"),
  getBoat: (id: string) => call<Boat>("GET", `/admin/boats/${id}`),
  listBoatTrips: (id: string) =>
    call<{ trips: Trip[] }>("GET", `/admin/boats/${id}/trips`),

  listTrips: () =>
    call<{ trips: Trip[]; scope: "all" | "assigned_to_me" }>("GET", "/admin/trips"),

  assignCruiseDirector: (tripId: string, cruiseDirectorUserId: string | null) =>
    call<{ ok: true }>("PATCH", `/admin/trips/${tripId}`, {
      cruise_director_user_id: cruiseDirectorUserId,
    }),

  listUsers: () => call<{ users: AdminUser[] }>("GET", "/admin/users"),

  patchOrganization: (input: { name: string; currency: string | null }) =>
    call<Organization>("PATCH", "/organization", input),

  // --- Sprint 012 trip imports ---

  kickLiveaboardImport: (url: string) =>
    call<ImportJob>("POST", "/admin/import/liveaboard", { url }),

  getImportJob: (id: string) =>
    call<ImportJob>("GET", `/admin/import/jobs/${encodeURIComponent(id)}`),

  // Spreadsheet preview: multipart upload, NOT JSON. Bypasses the
  // shared `call` helper because of the body shape.
  previewSpreadsheet: async (file: File): Promise<SpreadsheetPreviewResponse> => {
    const fd = new FormData();
    fd.append("file", file);
    const resp = await fetch(`${appConfig.apiBase}/admin/import/spreadsheet/preview`, {
      method: "POST",
      credentials: "include",
      body: fd,
    });
    const text = await resp.text();
    const parsed = text ? (JSON.parse(text) as unknown) : null;
    if (!resp.ok) {
      throw (parsed ?? { error: "unknown", message: resp.statusText }) as ApiError;
    }
    return parsed as SpreadsheetPreviewResponse;
  },

  commitSpreadsheet: (input: {
    preview_id: string;
    vessel_mapping: Record<string, VesselMappingChoice>;
    rows_to_skip: number[];
  }) =>
    call<{
      job_id: string;
      trips_inserted: number;
      trips_updated: number;
      trips_deleted: number;
    }>("POST", "/admin/import/spreadsheet/commit", input),
};

// --- import types ---

export type ImportJob = {
  id: string;
  source: "liveaboard_com" | "spreadsheet";
  source_input: string;
  status: "queued" | "running" | "succeeded" | "failed";
  started_at: string;
  completed_at: string | null;
  boats_inserted?: number;
  boats_updated?: number;
  trips_inserted?: number;
  trips_updated?: number;
  trips_deleted?: number;
  error_message?: string;
};

export type SpreadsheetRow = {
  line_number: number;
  vessel_name: string;
  start_date: string;
  end_date: string;
  itinerary: string;
  num_guests?: number | null;
};

export type SpreadsheetWarning = {
  line_number: number;
  code: string;
  message: string;
};

export type SpreadsheetPreview = {
  filename: string;
  source_fingerprint: string;
  headers: string[];
  rows: SpreadsheetRow[];
  warnings: SpreadsheetWarning[];
  vessel_names: string[];
};

export type SpreadsheetPreviewResponse = {
  preview_id: string;
  expires_at: string;
  payload: SpreadsheetPreview;
  vessel_suggestions: Record<string, { boat_id: string; display_name: string }>;
};

export type VesselMappingChoice =
  | { mode: "existing"; boat_id: string }
  | { mode: "create_new" };

