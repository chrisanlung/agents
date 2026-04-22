// server-only: this module runs exclusively on the server (Server Actions, Route Handlers).
// Do not import from client components.
import "server-only";

/**
 * Decodes the payload segment of a JWT without verifying the signature.
 *
 * This is intentionally NOT a signature-verify step. It is used only to read
 * claims from a token that the auth-service has already issued and returned to
 * us in the login response body. We trust the claim values because we trust the
 * response from our own backend (called server-side over localhost). The
 * signature is verified by the auth-service itself on every protected request.
 *
 * No JWT library is needed — base64url decode of segment [1] is enough.
 */
export function decodeJWTPayload<T>(token: string): T {
  const segments = token.split(".");
  if (segments.length !== 3) {
    throw new Error("Invalid JWT format");
  }

  const base64url = segments[1];
  // Convert base64url to standard base64
  const base64 = base64url.replace(/-/g, "+").replace(/_/g, "/");
  // Pad to a multiple of 4
  const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), "=");

  // atob is available in Node.js 16+ and all modern runtimes
  const jsonString = Buffer.from(padded, "base64").toString("utf-8");
  return JSON.parse(jsonString) as T;
}

/** Shape of the Lustia JWT payload claims (subset we care about). */
export interface JWTClaims {
  sub: string;
  iss: string;
  aud: string[];
  exp: number;
  iat: number;
  jti: string;
  /** "platform" | "tenant" | "user" — ADR 0007 §2.3 */
  scope?: "platform" | "tenant" | "user";
  tenant_id: string | null;
  /** UUID of the active membership, null when scope=user or scope=platform */
  membership_id?: string | null;
  roles: string[];
  permissions: string[];
  branches: string[];
  email?: string;
  full_name?: string;
  must_change_password?: boolean;
}
