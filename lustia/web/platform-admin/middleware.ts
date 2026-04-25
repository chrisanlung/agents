import { NextResponse, type NextRequest } from "next/server";

/**
 * Middleware — cookie-presence guard + silent refresh.
 *
 * - No cookie + protected path → redirect /login.
 * - Cookie present + /login → redirect /dashboard.
 * - Access token expired or within 60s of expiry → call /auth/refresh and
 *   rewrite cookies on the response so Server Components see a fresh token.
 *
 * We decode but never verify the JWT here (Edge runtime has no Node crypto).
 * The real auth check happens at the backend on every API call.
 */

const ACCESS_TOKEN_COOKIE = "access_token";
const REFRESH_TOKEN_COOKIE = "refresh_token";
const LAST_ACTIVITY_COOKIE = "lustia_last_activity";
const ACCESS_TOKEN_MAX_AGE = 30 * 60;
const REFRESH_TOKEN_MAX_AGE = 14 * 24 * 60 * 60;
const REFRESH_LEAD_SECONDS = 60;
const IDLE_TIMEOUT_MS = 30 * 60 * 1000;

const AUTH_API_URL = process.env.AUTH_API_URL ?? "";
const isProduction = process.env.NODE_ENV === "production";

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const accessTokenCookie = request.cookies.get(ACCESS_TOKEN_COOKIE);
  const refreshTokenCookie = request.cookies.get(REFRESH_TOKEN_COOKIE);
  const hasSession = accessTokenCookie !== undefined;

  // Silent refresh — gated by idle timeout so sessions don't extend
  // indefinitely when nobody is using the browser (see tenant-admin
  // middleware for full rationale).
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
        accessTokenCookie.value = refreshed.access_token;
      }
    }
  }

  if (pathname.startsWith("/dashboard") || pathname.startsWith("/tenants")) {
    if (!hasSession) {
      return NextResponse.redirect(new URL("/login", request.url));
    }

    // Wrong-portal guard: platform-admin is super-admin-only. Tenant-scoped
    // sessions belong on tenant-admin or ops. Kick them out with a flash so
    // they don't hit 403s on every admin endpoint.
    const claims = readJWTClaims(accessTokenCookie.value);
    if (claims?.scope && claims.scope !== "platform") {
      const url = new URL("/login", request.url);
      url.searchParams.set(
        "flash",
        "Halaman ini hanya untuk super admin. Silakan masuk dari portal tenant atau operasional."
      );
      const r = NextResponse.redirect(url);
      r.cookies.delete(ACCESS_TOKEN_COOKIE);
      r.cookies.delete(REFRESH_TOKEN_COOKIE);
      return r;
    }
  }

  if (pathname.startsWith("/login") && hasSession) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return response ?? NextResponse.next();
}

interface JWTClaims {
  scope?: string;
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
    return JSON.parse(atob(padded)) as JWTClaims;
  } catch {
    return undefined;
  }
}

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
  matcher: ["/login", "/dashboard/:path*", "/tenants/:path*", "/tenants"],
};
