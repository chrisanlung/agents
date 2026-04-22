"use server";

import { redirect } from "next/navigation";
import { apiFetch } from "@/lib/api";
import { clearSessionCookies, getRefreshToken } from "@/lib/session";

/**
 * Server Action: logout.
 * Attempts to revoke the refresh token on the backend (best-effort — we clear
 * the session cookie regardless of whether the API call succeeds).
 */
export async function logoutAction(): Promise<void> {
  const refreshToken = await getRefreshToken();

  try {
    await apiFetch(
      "/auth/logout",
      {
        method: "POST",
        body: JSON.stringify({ refresh_token: refreshToken ?? "" }),
      },
      { auth: true }
    );
  } catch {
    // Best-effort: if the logout API fails (token already expired, network
    // issue, etc.) we still clear local cookies so the user is logged out
    // from this browser session.
  }

  await clearSessionCookies();
  redirect("/login");
}
