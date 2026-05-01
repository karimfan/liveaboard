export type ApiError = { error: string; message: string };

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const resp = await fetch(path, {
    method,
    credentials: "include",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
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
  signup: (input: {
    email: string;
    password: string;
    full_name: string;
    organization_name: string;
  }) => call<{ ok: true; verification_token?: string }>("POST", "/api/signup", input),

  verifyEmail: (token: string) => call<{ ok: true }>("POST", "/api/verify-email", { token }),

  login: (email: string, password: string) =>
    call<{ ok: true }>("POST", "/api/login", { email, password }),

  logout: () => call<{ ok: true }>("POST", "/api/logout"),

  me: () =>
    call<{
      id: string;
      email: string;
      full_name: string;
      role: string;
      organization_id: string;
    }>("GET", "/api/me"),

  organization: () =>
    call<{
      id: string;
      name: string;
      currency: string | null;
      created_at: string;
      stats: { boats: number; active_trips: number; total_guests: number };
    }>("GET", "/api/organization"),
};
