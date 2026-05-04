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
  const parsed = text ? (JSON.parse(text) as unknown) : null;
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
  role: string;
  organization_id: string;
  email_verified: boolean;
};

export type Invitation = {
  id: string;
  email: string;
  role: string;
  expires_at: string;
};

export type InvitationLookup = {
  email: string;
  role: string;
  organization_name: string;
  expires_at: string;
};

export type PendingEmailChange = { new_email: string; expires_at: string };

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

  invite: (email: string, role: string) =>
    call<Invitation>("POST", "/invitations", { email, role }),

  resendInvitation: (id: string) =>
    call<Invitation>("POST", `/invitations/${encodeURIComponent(id)}/resend`),

  revokeInvitation: (id: string) =>
    call<{ status: "ok" }>("DELETE", `/invitations/${encodeURIComponent(id)}`),

  // --- Invitations (public, token-bearing) ---
  lookupInvitation: (token: string) =>
    call<InvitationLookup>("GET", `/invitations/lookup?token=${encodeURIComponent(token)}`),

  acceptInvitation: (token: string, fullName: string, password: string) =>
    call<Me>("POST", "/invitations/accept", {
      token,
      full_name: fullName,
      password,
    }),
};
