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
  return parseResponse<T>(resp, `${method} ${path}`);
}

// parseResponse turns a fetch Response into a typed body, preserving
// the response shape on error and exposing the raw text when the body
// isn't valid JSON. Without this, a malformed response surfaces as
// V8's bare "Unexpected non-whitespace character at position N"
// SyntaxError and the operator never sees what the server returned.
async function parseResponse<T>(resp: Response, label: string): Promise<T> {
  const text = await resp.text();
  let parsed: unknown = null;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      const snippet = text.length > 200 ? `${text.slice(0, 200)}…` : text;
      throw {
        error: "invalid_response",
        message: `${label}: server returned non-JSON response (HTTP ${resp.status}). Body: ${snippet}`,
      } as ApiError;
    }
  }
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
  num_guests: number | null;
  cruise_director_user_ids: string[];
  cruise_director_names: string[];
};

// Sprint 013 — assign/unassign endpoints return the updated director
// list for the trip so the SPA can patch the row in place without an
// extra fetch.
export type TripDirectorsView = {
  trip_id: string;
  cruise_director_user_ids: string[];
  cruise_director_names: string[];
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

export type CatalogCategory = {
  id: string;
  template_key: string | null;
  name: string;
  sort_order: number;
  is_default_seed: boolean;
  archived_at: string | null;
  item_count: number;
};

export type CatalogItem = {
  id: string;
  category_id: string;
  category_name: string;
  template_key: string | null;
  name: string;
  description: string | null;
  unit: string;
  charge_type: string;
  stock_mode: "none" | "counted";
  price_usd_cents: number;
  is_taxable: boolean;
  is_required_fee: boolean;
  is_active: boolean;
  archived_at: string | null;
};

export type BoatInventoryItem = {
  id: string;
  boat_id: string;
  catalog_item_id: string;
  item_name: string;
  category_name: string;
  unit: string;
  stock_mode: "counted";
  price_usd_cents: number;
  quantity_on_hand: number;
  quantity_reserved: number;
  reorder_level: number | null;
  par_level: number | null;
  last_counted_at: string | null;
  notes: string | null;
  status: "ok" | "low" | "out";
};

export type InventoryBoatSummary = {
  boat_id: string;
  boat_name: string;
  low_stock_count: number;
  out_stock_count: number;
};

export type FXRate = {
  id: string;
  provider: string;
  base_currency: string;
  quote_currency: string;
  rate_numerator: number;
  rate_denominator: number;
  as_of: string;
  fetched_at: string;
  expires_at: string;
};

export type CheckoutQuote = {
  id: string;
  source_currency: string;
  target_currency: string;
  source_amount_cents: number;
  target_amount_minor: number;
  currency_exponent: number;
  rate_provider: string;
  rate_numerator: number;
  rate_denominator: number;
  rate_as_of: string;
  expires_at: string;
  created_at: string;
  lines: {
    id: string;
    catalog_item_id: string | null;
    item_name: string;
    quantity: number;
    unit_price_usd_cents: number;
    line_total_usd_cents: number;
    sort_order: number;
  }[];
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

  // Sprint 013 — 1:N director assignment.
  addCruiseDirector: (tripId: string, userId: string) =>
    call<TripDirectorsView>("POST", `/admin/trips/${tripId}/cruise-directors`, {
      user_id: userId,
    }),

  removeCruiseDirector: (tripId: string, userId: string) =>
    call<TripDirectorsView>(
      "DELETE",
      `/admin/trips/${tripId}/cruise-directors/${encodeURIComponent(userId)}`,
    ),

  listUsers: () => call<{ users: AdminUser[] }>("GET", "/admin/users"),

  patchOrganization: (input: { name: string; currency: string | null }) =>
    call<Organization>("PATCH", "/organization", input),

  listCatalogCategories: () =>
    call<{ categories: CatalogCategory[] }>("GET", "/admin/catalog/categories"),
  createCatalogCategory: (input: { name: string; sort_order: number }) =>
    call<CatalogCategory>("POST", "/admin/catalog/categories", input),
  updateCatalogCategory: (id: string, input: { name: string; sort_order: number; archived?: boolean }) =>
    call<CatalogCategory>("PATCH", `/admin/catalog/categories/${id}`, input),

  listCatalogItems: () => call<{ items: CatalogItem[] }>("GET", "/admin/catalog/items"),
  createCatalogItem: (input: {
    category_id: string;
    name: string;
    description: string | null;
    unit: string;
    charge_type: string;
    stock_mode: "none" | "counted";
    price_usd_cents: number;
    is_taxable: boolean;
    is_required_fee: boolean;
    is_active?: boolean;
  }) => call<CatalogItem>("POST", "/admin/catalog/items", input),
  updateCatalogItem: (id: string, input: {
    category_id: string;
    name: string;
    description: string | null;
    unit: string;
    charge_type: string;
    stock_mode: "none" | "counted";
    price_usd_cents: number;
    is_taxable: boolean;
    is_required_fee: boolean;
    is_active?: boolean;
    archived?: boolean;
  }) => call<CatalogItem>("PATCH", `/admin/catalog/items/${id}`, input),
  applyCatalogDefaults: () =>
    call<{ status: string }>("POST", "/admin/catalog/defaults/apply"),

  inventoryBoatSummary: () =>
    call<{ boats: InventoryBoatSummary[] }>("GET", "/admin/inventory/boats"),
  listBoatInventory: (boatId: string) =>
    call<{ items: BoatInventoryItem[] }>("GET", `/admin/boats/${boatId}/inventory`),
  setBoatInventory: (boatId: string, itemId: string, input: {
    quantity_on_hand: number;
    reorder_level: number | null;
    par_level: number | null;
    notes: string | null;
  }) => call<BoatInventoryItem>("PUT", `/admin/boats/${boatId}/inventory/${itemId}`, input),
  adjustBoatInventory: (boatId: string, itemId: string, input: {
    movement_type: string;
    delta_quantity: number;
    note: string | null;
  }) => call<{ item: BoatInventoryItem }>("POST", `/admin/boats/${boatId}/inventory/${itemId}/adjustments`, input),

  listFXRates: () => call<{ rates: FXRate[] }>("GET", "/admin/fx/rates"),
  createFXRate: (input: {
    provider: string;
    base_currency: string;
    quote_currency: string;
    rate_numerator: number;
    rate_denominator: number;
    as_of: string;
    expires_at: string;
  }) => call<FXRate>("POST", "/admin/fx/rates", input),

  checkoutQuote: (input: { target_currency: string; source_amount_cents: number }) =>
    call<CheckoutQuote>("POST", "/checkout/quote", input),

  // --- Sprint 012 trip imports ---

  kickLiveaboardImport: (url: string) =>
    call<ImportJob>("POST", "/admin/import/liveaboard", { url }),

  getImportJob: (id: string) =>
    call<ImportJob>("GET", `/admin/import/jobs/${encodeURIComponent(id)}`),

  // Spreadsheet preview: multipart upload, NOT JSON. Bypasses the
  // shared `call` helper because of the body shape, but reuses the
  // shared response parser.
  previewSpreadsheet: async (file: File): Promise<SpreadsheetPreviewResponse> => {
    const fd = new FormData();
    fd.append("file", file);
    const resp = await fetch(`${appConfig.apiBase}/admin/import/spreadsheet/preview`, {
      method: "POST",
      credentials: "include",
      body: fd,
    });
    return parseResponse<SpreadsheetPreviewResponse>(resp, "POST /admin/import/spreadsheet/preview");
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
