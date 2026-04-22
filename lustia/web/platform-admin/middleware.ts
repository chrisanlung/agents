import { NextResponse, type NextRequest } from "next/server";

/**
 * Middleware — cookie-presence guard only.
 *
 * We deliberately do NOT decode or verify the JWT here (Edge runtime has no
 * Node.js crypto; the private key is server-only). Cookie presence is a fast
 * heuristic that avoids serving protected pages to clearly unauthenticated
 * users. The dashboard Server Component does the real verification by calling
 * GET /auth/me with the token — if the token is expired or invalid the backend
 * returns 401 and we redirect to /login from the Server Component.
 */
export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasSession = request.cookies.has("access_token");

  if (pathname.startsWith("/dashboard") && !hasSession) {
    const loginUrl = new URL("/login", request.url);
    return NextResponse.redirect(loginUrl);
  }

  if (pathname.startsWith("/login") && hasSession) {
    const dashboardUrl = new URL("/dashboard", request.url);
    return NextResponse.redirect(dashboardUrl);
  }

  return NextResponse.next();
}

export const config = {
  /*
   * Match /login and /dashboard (and sub-paths) only.
   * Exclude static assets, _next internals, favicon, etc. so the middleware
   * does not run on every request.
   */
  matcher: ["/login", "/dashboard/:path*"],
};
