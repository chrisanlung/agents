"use server";

import { cookies } from "next/headers";

/**
 * Sets the lustia_skipped_onboarding session cookie so the dashboard
 * does not redirect to /onboarding/welcome for the rest of this browser session.
 */
export async function setSkipCookieAction(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.set("lustia_skipped_onboarding", "1", {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    // Session cookie — no maxAge so it expires when the browser closes.
    secure: process.env.NODE_ENV === "production",
  });
}
