import { type NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

/**
 * GET /api/booking-lookup?code=XXXX-XXXX
 *
 * Client-side proxy to GET /tenant/bookings/by-code/:code (API §14.2.3).
 * Attaches the access_token cookie as a Bearer token to the upstream request.
 * Only the ops portal calls this — used by the CheckinFlow client component.
 */
export async function GET(req: NextRequest) {
  const code = req.nextUrl.searchParams.get("code");
  if (!code || !/^[A-Z0-9]{4}-[A-Z0-9]{4}$/i.test(code)) {
    return NextResponse.json(
      { error: { code: "BOOKING_CODE_INVALID", message: "Kode booking tidak valid." } },
      { status: 400 }
    );
  }

  const AUTH_API_URL = process.env.AUTH_API_URL ?? "";
  if (!AUTH_API_URL) {
    return NextResponse.json(
      { error: { code: "CONFIG_ERROR", message: "AUTH_API_URL tidak dikonfigurasi." } },
      { status: 500 }
    );
  }

  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;

  const upstream = await fetch(
    `${AUTH_API_URL}/tenant/bookings/by-code/${encodeURIComponent(code)}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      cache: "no-store",
    }
  );

  const body = await upstream.text();
  return new NextResponse(body, {
    status: upstream.status,
    headers: { "Content-Type": "application/json" },
  });
}
