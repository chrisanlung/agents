"use server";

import { redirect } from "next/navigation";
import { apiFetch } from "@/lib/api";
import { clearSessionCookies, getRefreshToken } from "@/lib/session";

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
    // Best-effort: clear local cookies regardless of API response.
  }

  await clearSessionCookies();
  redirect("/login");
}
