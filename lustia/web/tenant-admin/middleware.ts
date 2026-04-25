import { NextResponse, type NextRequest } from "next/server";

/**
 * Middleware — cookie-presence guard, scope routing, and silent refresh.
 *
 * Three concerns:
 * 1. No cookie → redirect /login.
 * 2. Cookie present but JWT scope === "user" on protected pages → /select-tenant.
 * 3. Access-token about to expire (<60s) → proactively call /auth/refresh and
 *    rewrite the cookies on the response. The Server Component that runs after
 *    middleware then sees a fresh token, so the user never sees a spurious
 *    /login redirect mid-session while their refresh token is still valid.
 *
 * We do NOT verify the JWT signature here — the Edge runtime has no Node.js
 * crypto. We only decode the middle segment to read `scope`, `exp`, and
 * `must_change_password`. The real auth check still happens server-side via
 * GET /auth/me (which rejects a forged or revoked token at the API boundary).
 */

const ACCESS_TOKEN_COOKIE = "access_token";
const REFRESH_TOKEN_COOKIE = "refresh_token";
const LAST_ACTIVITY_COOKIE = "lustia_last_activity";
const ACCESS_TOKEN_MAX_AGE = 30 * 60;
const REFRESH_TOKEN_MAX_AGE = 14 * 24 * 60 * 60;
const REFRESH_LEAD_SECONDS = 60;
// Idle timeout: if no client-side activity was recorded within this window,
// the middleware stops silent-refreshing the access token so the session
// expires naturally and the next protected request kicks the user to /login
// with a "session expired" flash. 30 minutes matches the access token TTL.
const IDLE_TIMEOUT_MS = 30 * 60 * 1000;

const AUTH_API_URL = process.env.AUTH_API_URL ?? "";
const isProduction = process.env.NODE_ENV === "production";

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const accessTokenCookie = request.cookies.get(ACCESS_TOKEN_COOKIE);
  const refreshTokenCookie = request.cookies.get(REFRESH_TOKEN_COOKIE);
  const hasSession = accessTokenCookie !== undefined;

  const isProtected =
    pathname.startsWith("/dashboard") ||
    pathname.startsWith("/branches") ||
    pathname.startsWith("/onboarding") ||
    pathname.startsWith("/settings") ||
    pathname.startsWith("/pengaturan") ||
    pathname.startsWith("/master");

  // Silent refresh — only when we have both cookies, the access token is
  // expired or near-expiry, AND the user has been active within the idle
  // window. If the user has been idle longer than IDLE_TIMEOUT_MS, we skip
  // refresh so the session expires naturally (security: a laptop left open
  // at a spa counter shouldn't keep renewing its own session forever).
  let response: NextResponse | null = null;
  if (hasSession && refreshTokenCookie) {
    const claims = readJWTClaims(accessTokenCookie.value);
    const needsRefresh =
      !claims?.exp || claims.exp - Math.floor(Date.now() / 1000) <= REFRESH_LEAD_SECONDS;
    const isIdle = isIdleTooLong(request.cookies.get(LAST_ACTIVITY_COOKIE)?.value);

    if (needsRefresh && !isIdle) {
      const refreshed = await tryRefresh(refreshTokenCookie.value);
      if (refreshed) {
        response = NextResponse.next();
        writeSessionCookies(response, refreshed.access_token, refreshed.refresh_token);
        // Update the request's claim view so subsequent checks use the new token.
        accessTokenCookie.value = refreshed.access_token;
      }
    }
  }

  if (isProtected) {
    if (!hasSession) {
      return NextResponse.redirect(new URL("/login", request.url));
    }

    const claims = readJWTClaims(accessTokenCookie.value);

    // Wrong-portal guard: tenant-admin only serves scope=tenant / scope=user.
    // Super-admin sessions (scope=platform) belong on platform-admin. Clear
    // their cookies and punt them to /login with a flash so they don't
    // silently run queries with tenant_id='__platform__' that blow up
    // every downstream SQL call.
    if (claims?.scope === "platform") {
      const url = new URL("/login", request.url);
      url.searchParams.set(
        "flash",
        "Akun super admin harus masuk melalui portal Platform Admin."
      );
      const r = NextResponse.redirect(url);
      r.cookies.delete(ACCESS_TOKEN_COOKIE);
      r.cookies.delete(REFRESH_TOKEN_COOKIE);
      return r;
    }

    // If the token is user-scoped (no active tenant), force tenant selection.
    if (claims?.scope === "user") {
      return NextResponse.redirect(new URL("/select-tenant", request.url));
    }

    // Forced password-change gate: until the flag is cleared, every protected
    // route funnels to the change-password screen.
    if (
      claims?.must_change_password === true &&
      !pathname.startsWith("/pengaturan/ubah-kata-sandi")
    ) {
      const url = new URL("/pengaturan/ubah-kata-sandi", request.url);
      url.searchParams.set("reason", "required");
      return NextResponse.redirect(url);
    }
  }

  if (pathname.startsWith("/select-tenant") && !hasSession) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  if (pathname.startsWith("/login") && hasSession) {
    const claims = readJWTClaims(accessTokenCookie.value);
    if (claims?.scope === "user") {
      return NextResponse.redirect(new URL("/select-tenant", request.url));
    }
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return response ?? NextResponse.next();
}

interface JWTClaims {
  scope?: string;
  must_change_password?: boolean;
  exp?: number;
}

interface RefreshOk {
  access_token: string;
  refresh_token: string;
}

function readJWTClaims(token: string): JWTClaims | undefined {
  try {
    const segments = token.split(".");
    if (segments.length !== 3) return undefined;
    const base64url = segments[1];
    const base64 = base64url.replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64.padEnd(
      base64.length + ((4 - (base64.length % 4)) % 4),
      "="
    );
    const json = atob(padded);
    return JSON.parse(json) as JWTClaims;
  } catch {
    return undefined;
  }
}

/**
 * Returns true when the last-activity cookie is missing or older than
 * IDLE_TIMEOUT_MS. Missing cookie is treated as idle because it means the
 * ActivityTracker hasn't run yet OR the user started a new browser session —
 * both cases warrant re-authentication on expiry.
 */
function isIdleTooLong(raw: string | undefined): boolean {
  if (!raw) return true;
  const last = parseInt(raw, 10);
  if (!Number.isFinite(last)) return true;
  return Date.now() - last > IDLE_TIMEOUT_MS;
}

async function tryRefresh(refreshToken: string): Promise<RefreshOk | null> {
  if (!AUTH_API_URL) return null;
  try {
    const res = await fetch(`${AUTH_API_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
      cache: "no-store",
    });
    if (!res.ok) return null;
    const body = (await res.json()) as RefreshOk;
    if (!body.access_token || !body.refresh_token) return null;
    return body;
  } catch {
    return null;
  }
}

function writeSessionCookies(
  response: NextResponse,
  accessToken: string,
  refreshToken: string
): void {
  response.cookies.set(ACCESS_TOKEN_COOKIE, accessToken, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    secure: isProduction,
    maxAge: ACCESS_TOKEN_MAX_AGE,
  });
  response.cookies.set(REFRESH_TOKEN_COOKIE, refreshToken, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    secure: isProduction,
    maxAge: REFRESH_TOKEN_MAX_AGE,
  });
}

export const config = {
  matcher: [
    "/login",
    "/dashboard/:path*",
    "/branches/:path*",
    "/branches",
    "/onboarding/:path*",
    "/select-tenant/:path*",
    "/select-tenant",
    "/settings/:path*",
    "/settings",
    "/pengaturan/:path*",
    "/pengaturan",
    "/master/:path*",
    "/master",
  ],
};
