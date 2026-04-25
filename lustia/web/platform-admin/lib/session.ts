// server-only: cookie helpers run exclusively on the server.
import "server-only";

import { cookies } from "next/headers";

const ACCESS_TOKEN_COOKIE = "access_token";
const REFRESH_TOKEN_COOKIE = "refresh_token";

// 30 minutes in seconds (matches auth-service JWT exp claim)
const ACCESS_TOKEN_MAX_AGE = 30 * 60;

// 14 days in seconds (matches auth-service refresh token lifetime)
const REFRESH_TOKEN_MAX_AGE = 14 * 24 * 60 * 60;

const isProduction = process.env.NODE_ENV === "production";

/**
 * Sets the access_token and refresh_token HttpOnly cookies.
 *
 * CSRF note: SameSite=Lax provides the CSRF protection here because we rely on
 * Bearer token auth (the access token is read from the cookie server-side and
 * injected into the Authorization header — browsers never auto-attach it).
 * If any auth-mutating action is ever exposed without a separate Bearer flow,
 * a CSRF token becomes mandatory — see SECURITY.md §6.4.
 */
export async function setSessionCookies(
  accessToken: string,
  refreshToken: string
): Promise<void> {
  const cookieStore = await cookies();

  cookieStore.set(ACCESS_TOKEN_COOKIE, accessToken, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    secure: isProduction,
    maxAge: ACCESS_TOKEN_MAX_AGE,
  });

  cookieStore.set(REFRESH_TOKEN_COOKIE, refreshToken, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    secure: isProduction,
    maxAge: REFRESH_TOKEN_MAX_AGE,
  });
}

/** Clears both session cookies (logout). */
export async function clearSessionCookies(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.delete(ACCESS_TOKEN_COOKIE);
  cookieStore.delete(REFRESH_TOKEN_COOKIE);
}

/** Returns the raw access token string, or null if not present. */
export async function getAccessToken(): Promise<string | null> {
  const cookieStore = await cookies();
  return cookieStore.get(ACCESS_TOKEN_COOKIE)?.value ?? null;
}

/** Returns the raw refresh token string, or null if not present. */
export async function getRefreshToken(): Promise<string | null> {
  const cookieStore = await cookies();
  return cookieStore.get(REFRESH_TOKEN_COOKIE)?.value ?? null;
}
