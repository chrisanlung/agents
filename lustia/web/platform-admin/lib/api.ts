// server-only: apiFetch uses cookies() from next/headers and must never run in the browser.
import "server-only";

import { getAccessToken } from "@/lib/session";

// AUTH_API_URL is validated at call time (not module load) so `next build`
// succeeds without requiring the env var during the build step itself.
const AUTH_API_URL = process.env.AUTH_API_URL ?? "";

/** Typed error thrown when the auth-service returns an error envelope. */
export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "ApiError";
  }
}

interface ErrorEnvelope {
  error: {
    code: string;
    message: string;
    details?: unknown;
  };
}

/**
 * Thin fetch wrapper for the auth-service.
 *
 * @param path   - Path relative to AUTH_API_URL (e.g. "/auth/login")
 * @param init   - Standard RequestInit options (method, body, headers, etc.)
 * @param opts   - { auth: true } to inject Authorization: Bearer <cookie token>
 */
export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
  opts: { auth?: boolean } = {}
): Promise<T> {
  if (!AUTH_API_URL) {
    throw new Error(
      "AUTH_API_URL environment variable is not set. Copy .env.local.example to .env.local."
    );
  }

  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");

  if (opts.auth) {
    const token = await getAccessToken();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
  }

  const response = await fetch(`${AUTH_API_URL}${path}`, {
    ...init,
    headers,
    // Opt out of Next.js Data Cache for auth requests — auth data must always be fresh.
    cache: "no-store",
  });

  if (!response.ok) {
    // Try to parse the error envelope; fall back to a generic message.
    let code = "INTERNAL";
    let message = `HTTP ${response.status}`;
    try {
      const body = (await response.json()) as ErrorEnvelope;
      code = body.error?.code ?? code;
      message = body.error?.message ?? message;
    } catch {
      // response body was not valid JSON
    }
    throw new ApiError(code, message, response.status);
  }

  // 204 No Content — return empty object cast to T
  if (response.status === 204) {
    return {} as T;
  }

  return response.json() as Promise<T>;
}
