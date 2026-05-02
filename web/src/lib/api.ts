import { appConfig } from "./config";

export type ApiError = { error: string; message: string };

function url(path: string): string {
  // path is always like "/me", "/login", etc.
  // appConfig.apiBase is "/api" (same-origin) or an absolute URL.
  return `${appConfig.apiBase}${path}`;
}

async function call<T>(method: string, path: string, body?: unknown, jwt?: string): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (jwt) {
    headers["Authorization"] = `Bearer ${jwt}`;
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

export const api = {
  // Phase 5/6 auth orchestration. The SPA calls these once after Clerk
  // SignIn / SignUp; the Bearer JWT is used only by these two endpoints,
  // which mint the lb_session cookie. Every other endpoint is cookie-auth.
  signupComplete: (jwt: string, organizationName: string, fullName?: string) =>
    call<{ ok: true; organization_id: string; user_id: string }>(
      "POST",
      "/signup-complete",
      { organization_name: organizationName, full_name: fullName },
      jwt,
    ),

  exchange: (jwt: string) =>
    call<{ ok: true }>("POST", "/auth/exchange", {}, jwt),

  logout: () => call<{ ok: true }>("POST", "/logout"),

  me: () =>
    call<{
      id: string;
      email: string;
      full_name: string;
      role: string;
      organization_id: string;
    }>("GET", "/me"),

  organization: () =>
    call<{
      id: string;
      name: string;
      currency: string | null;
      created_at: string;
      stats: { boats: number; active_trips: number; total_guests: number };
    }>("GET", "/organization"),

  // Org-admin endpoints.
  invite: (email: string, role: string) =>
    call<{
      ok: true;
      invitation: { id: string; email: string; role: string; status: string };
    }>("POST", "/invitations", { email, role }),

  resendInvite: (id: string) =>
    call<{ ok: true }>("POST", `/invitations/${encodeURIComponent(id)}/resend`),

  deactivateUser: (userId: string) =>
    call<{ ok: true }>("POST", `/users/${encodeURIComponent(userId)}/deactivate`),
};
