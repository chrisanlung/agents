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

  if (pathname.startsWith("/dashboard")) {
    if (!hasSession) {
      return NextResponse.redirect(new URL("/login", request.url));
    }

    // If the token is user-scoped (no active tenant), force tenant selection.
    const scope = readJWTScope(accessTokenCookie.value);
    if (scope === "user") {
      return NextResponse.redirect(new URL("/select-tenant", request.url));
    }
  }

  if (pathname.startsWith("/select-tenant") && !hasSession) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  if (pathname.startsWith("/login") && hasSession) {
    // Only redirect to dashboard if already fully scoped; otherwise let the
    // user go to select-tenant (the login action already handled that redirect,
    // but a direct /login visit after acquiring a user-scoped token should not
    // loop back to login).
    const scope = readJWTScope(accessTokenCookie.value);
    if (scope !== "user") {
      return NextResponse.redirect(new URL("/dashboard", request.url));
    }
    return NextResponse.redirect(new URL("/select-tenant", request.url));
  }

  return NextResponse.next();
}

/**
 * Reads the `scope` claim from a JWT without verifying the signature.
 * Safe for Edge runtime — uses atob (available in all modern runtimes).
 * Returns undefined if parsing fails for any reason.
 */
function readJWTScope(token: string): string | undefined {
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
    const payload = JSON.parse(json) as { scope?: string };
    return payload.scope;
  } catch {
    return undefined;
  }
}

export const config = {
  matcher: ["/login", "/dashboard/:path*", "/select-tenant/:path*", "/select-tenant"],
};
