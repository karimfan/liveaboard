import { appConfig } from "./config";

export type ApiError = { error: string; message: string };

function url(path: string): string {
  return `${appConfig.apiBase}${path}`;
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  const resp = await fetch(url(path), {
    method,
    credentials: "include",
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await resp.text();
  let parsed: unknown = null;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      const snippet = text.length > 200 ? `${text.slice(0, 200)}…` : text;
      throw {
        error: "invalid_response",
        message: `${method} ${path}: server returned non-JSON response (HTTP ${resp.status}). Body: ${snippet}`,
      } as ApiError;
    }
  }
  if (!resp.ok) {
    const err = (parsed ?? { error: "unknown", message: resp.statusText }) as ApiError;
    throw err;
  }
  return parsed as T;
}

export type Me = {
  id: string;
  email: string;
  full_name: string;
  phone: string | null;
  role: "org_admin" | "cruise_director";
  organization_id: string;
  email_verified: boolean;
};

export type Invitation = {
  id: string;
  email: string;
  full_name: string;
  phone: string | null;
  role: string;
  expires_at: string;
};

export type InvitationLookup = {
  email: string;
  full_name: string;
  role: string;
  organization_name: string;
  expires_at: string;
};

export type PendingEmailChange = { new_email: string; expires_at: string };

export type CruiseDirectorOverview = {
  profile: {
    id: string;
    full_name: string;
    email: string;
    phone: string | null;
    role: string;
    organization_name: string;
  };
  stats: { upcoming: number; active: number; past: number };
  trips: Array<{
    id: string;
    boat_id: string;
    boat_name: string;
    itinerary: string;
    start_date: string;
    end_date: string;
    status: "upcoming" | "active" | "past";
  }>;
};

export type GuestInviteLookup = {
  trip_guest_id: string;
  email: string;
  full_name: string;
  organization_name: string;
  boat_name: string;
  itinerary: string;
  start_date: string;
  end_date: string;
  expires_at: string;
};

export type GuestRegistration = {
  id?: string;
  trip_guest_id?: string;
  status: "draft" | "submitted";
  payload: Record<string, unknown>;
  submitted_at?: string | null;
};

export const api = {
  // --- Public auth ---
  signup: (input: {
    email: string;
    password: string;
    full_name: string;
    organization_name: string;
  }) => call<{ status: "ok"; message: string }>("POST", "/auth/signup", input),

  verifyEmail: (token: string) =>
    call<{ status: "ok" }>("POST", "/auth/verify-email", { token }),

  resendVerification: (email: string) =>
    call<{ status: "ok" }>("POST", "/auth/resend-verification", { email }),

  login: (email: string, password: string) =>
    call<Me>("POST", "/auth/login", { email, password }),

  logout: () => call<{ status: "ok" }>("POST", "/auth/logout"),

  forgotPassword: (email: string) =>
    call<{ status: "ok"; message: string }>("POST", "/auth/forgot-password", { email }),

  resetPassword: (token: string, newPassword: string) =>
    call<Me>("POST", "/auth/reset-password", { token, new_password: newPassword }),

  // --- Authenticated (cookie) ---
  me: () => call<Me>("GET", "/me"),

  organization: () =>
    call<{
      id: string;
      name: string;
      currency: string | null;
      created_at: string;
      stats: { boats: number; active_trips: number; total_guests: number };
    }>("GET", "/organization"),

  // --- Account self-service ---
  changePassword: (currentPassword: string, newPassword: string) =>
    call<{ status: "ok" }>("POST", "/account/change-password", {
      current_password: currentPassword,
      new_password: newPassword,
    }),

  requestEmailChange: (newEmail: string, currentPassword: string) =>
    call<{ status: "ok"; message: string }>("POST", "/account/request-email-change", {
      new_email: newEmail,
      current_password: currentPassword,
    }),

  pendingEmailChange: () =>
    call<{ pending: PendingEmailChange | null }>("GET", "/account/pending-email-change"),

  cancelEmailChange: () => call<{ status: "ok" }>("POST", "/account/cancel-email-change"),

  confirmEmailChange: (token: string) =>
    call<Me>("POST", "/account/confirm-email-change", { token }),

  // --- Invitations (admin) ---
  listInvitations: () =>
    call<{ invitations: Invitation[] }>("GET", "/invitations"),

  invite: (input: {
    email: string;
    full_name: string;
    phone?: string;
    role?: string;
  }) =>
    call<Invitation>("POST", "/invitations", {
      email: input.email,
      full_name: input.full_name,
      phone: input.phone ?? "",
      role: input.role ?? "cruise_director",
    }),

  resendInvitation: (id: string) =>
    call<Invitation>("POST", `/invitations/${encodeURIComponent(id)}/resend`),

  revokeInvitation: (id: string) =>
    call<{ status: "ok" }>("DELETE", `/invitations/${encodeURIComponent(id)}`),

  // --- Invitations (public, token-bearing) ---
  lookupInvitation: (token: string) =>
    call<InvitationLookup>("GET", `/invitations/lookup?token=${encodeURIComponent(token)}`),

  // Sprint 010: invitee no longer types a name — the admin captured it
  // at invite time. The accept page only collects a password.
  acceptInvitation: (token: string, password: string) =>
    call<Me>("POST", "/invitations/accept", { token, password }),

  // --- Profile (Sprint 010) ---
  updateProfile: (input: { full_name: string; phone: string | null }) =>
    call<Me>("PATCH", "/account/profile", input),

  cruiseDirectorOverview: () =>
    call<CruiseDirectorOverview>("GET", "/admin/cruise-director-overview"),

  lookupGuestInvite: (token: string) =>
    call<GuestInviteLookup>("GET", `/guest/invitations/${encodeURIComponent(token)}`),

  acceptGuestInvite: (token: string, password: string) =>
    call<{ guest: { id: string; email: string }; trip_guest_id: string }>(
      "POST",
      `/guest/invitations/${encodeURIComponent(token)}/accept`,
      { password },
    ),

  guestLogout: () => call<{ status: "ok" }>("POST", "/guest/logout"),

  guestRegistration: (tripGuestId: string) =>
    call<GuestRegistration>("GET", `/guest/trip-registrations/${encodeURIComponent(tripGuestId)}`),

  saveGuestRegistration: (tripGuestId: string, payload: Record<string, unknown>) =>
    call<GuestRegistration>("PATCH", `/guest/trip-registrations/${encodeURIComponent(tripGuestId)}`, payload),

  submitGuestRegistration: (tripGuestId: string, payload: Record<string, unknown>) =>
    call<GuestRegistration>("POST", `/guest/trip-registrations/${encodeURIComponent(tripGuestId)}/submit`, payload),
};
