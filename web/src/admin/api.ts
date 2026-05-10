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
  status: "planned" | "active" | "completed" | "cancelled";
  started_at: string | null;
  completed_at: string | null;
  cancelled_at: string | null;
  removed_from_source_at: string | null;
  cruise_director_user_ids: string[];
  cruise_director_names: string[];
  manifest_summary?: ManifestSummary;
};

export type TripReadinessIssue = {
  code: string;
  severity: "blocker" | "warning";
  message: string;
  trip_guest_id?: string;
};

export type TripLifecycle = {
  trip: {
    id: string;
    status: Trip["status"];
    started_at: string | null;
    completed_at: string | null;
    cancelled_at: string | null;
    cancellation_reason: string | null;
    removed_from_source_at: string | null;
  };
  readiness: {
    trip_id: string;
    status: Trip["status"];
    can_start: boolean;
    can_complete: boolean;
    blockers: TripReadinessIssue[];
    warnings: TripReadinessIssue[];
    guests: {
      trip_guest_id: string;
      full_name: string;
      email: string;
      registration_status: string | null;
      document_count: number;
      has_cabin_assignment: boolean;
      folio_status: string | null;
      registration_complete: boolean;
    }[];
  };
};

export type ManifestSummary = {
  guest_count: number;
  submitted_count: number;
  expected_count: number | null;
  has_warning: boolean;
};

export type TripGuest = {
  id: string;
  full_name: string;
  email: string;
  status: string;
  invite_send_status: string;
  invite_last_error: string | null;
  invite_last_sent_at: string | null;
  invite_expires_at: string | null;
  account_created_at: string | null;
  revoked_at: string | null;
  registration_status: string | null;
  registration_submitted_at: string | null;
  cabin_assignment: CabinAssignment | null;
};

export type TripManifest = {
  trip: Trip;
  summary: ManifestSummary;
  guests: TripGuest[];
};

export type GuestRegistrationDetail = {
  trip_guest: TripGuest;
  registration: {
    id: string;
    status: "draft" | "submitted";
    payload: Record<string, unknown>;
    submitted_at: string | null;
    created_at: string;
    updated_at: string;
  } | null;
};

export type CabinAssignment = {
  id: string;
  trip_id: string;
  trip_guest_id: string;
  boat_id: string;
  berth_id: string;
  cabin_label: string;
  berth_label: string;
  display_label: string;
  assigned_at: string;
  notes: string | null;
};

export type BoatCabinBerth = {
  id: string;
  boat_id: string;
  cabin_id: string;
  berth_label: string;
  display_label: string;
  sort_order: number;
  notes: string | null;
  is_active: boolean;
};

export type BoatCabin = {
  id: string;
  boat_id: string;
  label: string;
  deck: string | null;
  sort_order: number;
  notes: string | null;
  is_active: boolean;
  berths: BoatCabinBerth[];
};

export type CabinLayout = {
  boat_id: string;
  active_cabin_count: number;
  active_berth_count: number;
  cabins: BoatCabin[];
};

export type CabinLayoutInput = {
  source: "ranges" | "paste" | "csv" | "cabins";
  ranges?: { from: number; to: number; berths: string[]; deck?: string | null }[];
  paste?: string;
  csv?: string;
  cabins?: {
    label: string;
    deck?: string | null;
    sort_order?: number;
    notes?: string | null;
    berths: { berth_label: string; sort_order?: number; notes?: string | null }[];
  }[];
};

export type CabinLayoutPreview = {
  cabins: NonNullable<CabinLayoutInput["cabins"]>;
  warnings: string[];
};

export type TripCabinGuest = {
  id: string;
  full_name: string;
  email: string;
  status: string;
};

export type TripCabinBoard = {
  trip_id: string;
  boat_id: string;
  cabins: {
    id: string;
    label: string;
    deck: string | null;
    sort_order: number;
    berths: {
      id: string;
      cabin_id: string;
      berth_label: string;
      display_label: string;
      sort_order: number;
      guest: TripCabinGuest | null;
      assignment_id: string | null;
    }[];
  }[];
  unassigned_guests: TripCabinGuest[];
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

export type PaymentSettings = {
  organization_id: string;
  default_currency: string;
  supported_currencies: string[];
  enabled_payment_methods: string[];
  card_fee_basis_points: number;
  folio_email_footer: string | null;
  rate_readiness: {
    currency: string;
    ready: boolean;
    rate?: FXRate;
  }[];
};

export type GuestFolioLine = {
  id: string;
  catalog_item_id: string | null;
  line_type: "catalog_item" | "crew_tip";
  item_name: string;
  quantity: number;
  unit_price_usd_cents: number;
  line_total_usd_cents: number;
  stock_mode: "none" | "counted";
  sort_order: number;
  created_at: string;
  updated_at: string;
  stock_posted_at?: string | null;
  client_request_id?: string | null;
};

export type FolioWarning = {
  code: string;
  message: string;
  catalog_item_id: string | null;
  quantity_on_hand: number | null;
};

export type GuestFolio = {
  id: string;
  organization_id: string;
  trip_id: string;
  trip_guest_id: string;
  status: "open" | "closed";
  closed_at: string | null;
  subtotal_usd_cents: number;
  card_fee_usd_cents: number;
  total_usd_cents: number;
  settlement_currency: string | null;
  settlement_total_minor: number | null;
  currency_exponent: number | null;
  rate_provider: string | null;
  rate_numerator: number | null;
  rate_denominator: number | null;
  rate_as_of: string | null;
  payment_method: string | null;
  card_fee_basis_points: number;
  email_send_status: "not_sent" | "sent" | "failed";
  email_last_sent_at: string | null;
  email_last_error: string | null;
  lines: GuestFolioLine[];
  organization_name: string;
  boat_name: string;
  itinerary: string;
  start_date: string;
  end_date: string;
  guest_full_name: string;
  guest_email: string;
  warnings: FolioWarning[];
};

export type TripLedger = {
  trip: {
    id: string;
    boat_id: string;
    status: Trip["status"];
    itinerary: string;
    start_date: string;
    end_date: string;
  };
  guests: {
    trip_guest_id: string;
    full_name: string;
    email: string;
    folio_id: string | null;
    folio_status: string | null;
    line_count: number;
    subtotal_usd_cents: number;
  }[];
  catalog: CatalogItem[];
  inventory: {
    catalog_item_id: string;
    quantity_on_hand: number;
    status: "ok" | "low" | "out";
  }[];
  recent: {
    id: string;
    trip_guest_id: string;
    guest_full_name: string;
    item_name: string;
    quantity: number;
    line_total_usd_cents: number;
    stock_mode: "none" | "counted";
    created_at: string;
  }[];
};

export type GuestDocument = {
  id: string;
  category: string;
  display_name: string;
  original_filename: string;
  content_type: string;
  size_bytes: number;
  notes: string | null;
  archived_at: string | null;
  created_at: string;
  view_url: string;
  download_url: string;
};

export type AuditEvent = {
  id: string;
  actor_type: "staff" | "guest" | "system";
  action: string;
  entity_type: string;
  entity_id: string | null;
  trip_id: string | null;
  trip_guest_id: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
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

  tripManifest: (tripId: string) =>
    call<TripManifest>("GET", `/admin/trips/${encodeURIComponent(tripId)}/manifest`),
  tripLifecycle: (tripId: string) =>
    call<TripLifecycle>("GET", `/admin/trips/${encodeURIComponent(tripId)}/lifecycle`),
  startTrip: (tripId: string, input: { acknowledged_warnings: string[]; reason: string }) =>
    call<TripLifecycle["trip"]>("POST", `/admin/trips/${encodeURIComponent(tripId)}/start`, input),
  completeTrip: (tripId: string, input: { acknowledged_warnings: string[]; reason: string }) =>
    call<TripLifecycle["trip"]>("POST", `/admin/trips/${encodeURIComponent(tripId)}/complete`, input),
  cancelTrip: (tripId: string, input: { reason: string }) =>
    call<TripLifecycle["trip"]>("POST", `/admin/trips/${encodeURIComponent(tripId)}/cancel`, input),

  addTripGuest: (tripId: string, input: { full_name: string; email: string; berth_id: string }) =>
    call<TripGuest>("POST", `/admin/trips/${encodeURIComponent(tripId)}/guests`, input),

  resendTripGuestInvite: (tripId: string, guestId: string) =>
    call<TripGuest>("POST", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/resend`),

  revokeTripGuestInvite: (tripId: string, guestId: string) =>
    call<{ status: string }>("DELETE", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/invite`),

  guestRegistration: (tripId: string, guestId: string) =>
    call<GuestRegistrationDetail>("GET", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/registration`),

  guestDocuments: (tripId: string, guestId: string) =>
    call<{ documents: GuestDocument[] }>("GET", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/documents`),
  uploadGuestDocument: async (
    tripId: string,
    guestId: string,
    input: { file: File; category: string; display_name?: string; notes?: string },
  ): Promise<GuestDocument> => {
    const fd = new FormData();
    fd.append("file", input.file);
    fd.append("category", input.category);
    if (input.display_name) fd.append("display_name", input.display_name);
    if (input.notes) fd.append("notes", input.notes);
    const resp = await fetch(`${appConfig.apiBase}/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/documents`, {
      method: "POST",
      credentials: "include",
      body: fd,
    });
    return parseResponse<GuestDocument>(resp, "POST guest document");
  },
  archiveGuestDocument: (tripId: string, guestId: string, documentId: string) =>
    call<GuestDocument>("DELETE", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/documents/${encodeURIComponent(documentId)}`),
  guestActivity: (tripId: string, guestId: string) =>
    call<{ events: AuditEvent[] }>("GET", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/activity`),
  auditEvents: (params: Record<string, string>) => {
    const qs = new URLSearchParams(params);
    const suffix = qs.toString() ? `?${qs.toString()}` : "";
    return call<{ events: AuditEvent[] }>("GET", `/admin/audit-events${suffix}`);
  },

  boatCabins: (boatId: string) =>
    call<CabinLayout>("GET", `/admin/boats/${encodeURIComponent(boatId)}/cabins`),
  previewBoatCabins: (boatId: string, input: CabinLayoutInput) =>
    call<CabinLayoutPreview>("POST", `/admin/boats/${encodeURIComponent(boatId)}/cabins/preview`, input),
  replaceBoatCabins: (boatId: string, input: CabinLayoutInput) =>
    call<CabinLayout>("PUT", `/admin/boats/${encodeURIComponent(boatId)}/cabins`, input),
  updateBoatCabin: (boatId: string, cabinId: string, input: { label: string; deck: string | null; sort_order: number; notes: string | null }) =>
    call<BoatCabin>("PATCH", `/admin/boats/${encodeURIComponent(boatId)}/cabins/${encodeURIComponent(cabinId)}`, input),
  deactivateBoatCabin: (boatId: string, cabinId: string) =>
    call<{ status: string }>("DELETE", `/admin/boats/${encodeURIComponent(boatId)}/cabins/${encodeURIComponent(cabinId)}`),
  updateBoatBerth: (boatId: string, cabinId: string, berthId: string, input: { berth_label: string; sort_order: number; notes: string | null }) =>
    call<BoatCabinBerth>("PATCH", `/admin/boats/${encodeURIComponent(boatId)}/cabins/${encodeURIComponent(cabinId)}/berths/${encodeURIComponent(berthId)}`, input),
  deactivateBoatBerth: (boatId: string, cabinId: string, berthId: string) =>
    call<{ status: string }>("DELETE", `/admin/boats/${encodeURIComponent(boatId)}/cabins/${encodeURIComponent(cabinId)}/berths/${encodeURIComponent(berthId)}`),
  tripCabinBoard: (tripId: string) =>
    call<TripCabinBoard>("GET", `/admin/trips/${encodeURIComponent(tripId)}/cabins`),
  assignGuestCabin: (tripId: string, guestId: string, input: { berth_id: string; notes?: string | null }) =>
    call<CabinAssignment>("PUT", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/cabin-assignment`, input),
  unassignGuestCabin: (tripId: string, guestId: string) =>
    call<{ status: string }>("DELETE", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/cabin-assignment`),

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

  paymentSettings: () =>
    call<PaymentSettings>("GET", "/admin/organization/payment-settings"),
  updatePaymentSettings: (input: {
    default_currency: string;
    supported_currencies: string[];
    enabled_payment_methods: string[];
    card_fee_basis_points: number;
    folio_email_footer: string | null;
  }) => call<PaymentSettings>("PATCH", "/admin/organization/payment-settings", input),

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

  getGuestFolio: (tripId: string, guestId: string) =>
    call<GuestFolio>("GET", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/folio`),
  openGuestFolio: (tripId: string, guestId: string) =>
    call<GuestFolio>("POST", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/folio`),
  addGuestFolioLine: (tripId: string, guestId: string, input: {
    line_type: "catalog_item" | "crew_tip";
    catalog_item_id?: string;
    quantity?: number;
    tip_usd_cents?: number;
    client_request_id?: string;
  }) => call<GuestFolio>("POST", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/folio/lines`, input),
  updateGuestFolioLine: (tripId: string, guestId: string, lineId: string, input: {
    quantity?: number;
    tip_usd_cents?: number;
  }) => call<GuestFolio>("PATCH", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/folio/lines/${encodeURIComponent(lineId)}`, input),
  deleteGuestFolioLine: (tripId: string, guestId: string, lineId: string) =>
    call<GuestFolio>("DELETE", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/folio/lines/${encodeURIComponent(lineId)}`),
  closeGuestFolio: (tripId: string, guestId: string, input: { payment_method: string; settlement_currency: string }) =>
    call<GuestFolio>("POST", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/folio/close`, input),
  resendGuestFolioEmail: (tripId: string, guestId: string) =>
    call<GuestFolio>("POST", `/admin/trips/${encodeURIComponent(tripId)}/guests/${encodeURIComponent(guestId)}/folio/resend-email`),
  getTripLedger: (tripId: string) =>
    call<TripLedger>("GET", `/admin/trips/${encodeURIComponent(tripId)}/ledger`),
  addTripLedgerLine: (tripId: string, input: {
    trip_guest_id: string;
    catalog_item_id: string;
    quantity: number;
    client_request_id?: string;
  }) => call<GuestFolio>("POST", `/admin/trips/${encodeURIComponent(tripId)}/ledger/lines`, input),

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
  unconfigured_boats?: { id: string; name: string; active_berth_count: number }[];
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
