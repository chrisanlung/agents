import { NextResponse, type NextRequest } from "next/server";

/**
 * Middleware — cookie-presence guard + scope check.
 *
 * Two concerns:
 * 1. No cookie → redirect /login (existing behaviour, unchanged).
 * 2. Cookie present but JWT scope === "user" on /dashboard → redirect /select-tenant.
 *    This happens when a multi-tenant user logged in successfully but hasn't
 *    picked a workspace yet (ADR 0007 §2.4).
 *
 * We do NOT verify the JWT signature here — the Edge runtime has no Node.js
 * crypto and the private key is server-only. We only decode the middle segment
 * to read the `scope` claim, which is sufficient for this routing decision.
 * The dashboard Server Component performs the real auth check via GET /auth/me.
 */
export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const accessTokenCookie = request.cookies.get("access_token");
  const hasSession = accessTokenCookie !== undefined;

  const isProtected =
    pathname.startsWith("/dashboard") ||
    pathname.startsWith("/branches") ||
    pathname.startsWith("/onboarding") ||
    pathname.startsWith("/settings") ||
    pathname.startsWith("/pengaturan");

  if (isProtected) {
    if (!hasSession) {
      return NextResponse.redirect(new URL("/login", request.url));
    }

    const claims = readJWTClaims(accessTokenCookie.value);

    // If the token is user-scoped (no active tenant), force tenant selection.
    if (claims?.scope === "user") {
      return NextResponse.redirect(new URL("/select-tenant", request.url));
    }

    // Forced password-change gate: until the flag is cleared, every protected
    // route funnels to the change-password screen. /pengaturan/ubah-kata-sandi
    // itself is the only exemption.
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

  return NextResponse.next();
}

interface JWTClaims {
  scope?: string;
  must_change_password?: boolean;
}

/**
 * Decodes a JWT payload without verifying the signature. Safe for Edge runtime —
 * uses atob (available in all modern runtimes). Used only for routing decisions;
 * the real auth check happens server-side via GET /auth/me.
 */
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
  ],
};
