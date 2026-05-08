"use server";

import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import { getRefreshToken, clearSessionCookies } from "@/lib/session";

const schema = z
  .object({
    old_password: z.string().min(1, "Kata sandi saat ini wajib diisi"),
    new_password: z
      .string()
      .min(8, "Kata sandi baru minimal 8 karakter")
      .max(128, "Kata sandi baru maksimal 128 karakter"),
    confirm_password: z.string().min(1, "Konfirmasi kata sandi wajib diisi"),
  })
  .refine((v) => v.new_password === v.confirm_password, {
    path: ["confirm_password"],
    message: "Konfirmasi kata sandi tidak cocok",
  })
  .refine((v) => v.new_password !== v.old_password, {
    path: ["new_password"],
    message: "Kata sandi baru tidak boleh sama dengan kata sandi saat ini",
  });

export type ChangePasswordState =
  | { status: "idle" }
  | {
      status: "error";
      fieldErrors?: Record<string, string[]>;
      formError?: string;
    }
  | { status: "success" };

/**
 * Change password server action.
 *
 * Backend's ChangePassword revokes EVERY refresh token for this user (including
 * the current one), so we cannot rotate the access-token in place — the JWT's
 * must_change_password claim would stay true and middleware would bounce the
 * user back here. We clear the session cookies and let the form redirect the
 * user to /login to re-authenticate with the new password. This mirrors how
 * sensitive credential changes are handled by GitHub/Google/etc.
 */
export async function changePasswordAction(
  _prev: ChangePasswordState,
  formData: FormData
): Promise<ChangePasswordState> {
  const raw = {
    old_password: String(formData.get("old_password") ?? ""),
    new_password: String(formData.get("new_password") ?? ""),
    confirm_password: String(formData.get("confirm_password") ?? ""),
  };

  const parsed = schema.safeParse(raw);
  if (!parsed.success) {
    const fieldErrors: Record<string, string[]> = {};
    for (const [field, errors] of Object.entries(
      parsed.error.flatten().fieldErrors
    )) {
      fieldErrors[field] = errors ?? [];
    }
    return { status: "error", fieldErrors };
  }

  try {
    await apiFetch<void>(
      "/auth/me/password",
      {
        method: "POST",
        body: JSON.stringify({
          old_password: parsed.data.old_password,
          new_password: parsed.data.new_password,
        }),
      },
      { auth: true }
    );
  } catch (err) {
    return mapChangePasswordError(err);
  }

  // Current session has been invalidated by the backend. Clear cookies so the
  // next request is unauthenticated — the form shows a "masuk kembali" CTA.
  await clearSessionCookies();

  return { status: "success" };
}

/** Hard logout — used by the "Atau, keluar dari akun ini" escape hatch in forced mode. */
export async function forcedLogoutAction(): Promise<void> {
  try {
    const refreshToken = await getRefreshToken();
    await apiFetch(
      "/auth/logout",
      {
        method: "POST",
        body: JSON.stringify({ refresh_token: refreshToken ?? "" }),
      },
      { auth: true }
    );
  } catch {
    // Best-effort — clear cookies regardless.
  }
  await clearSessionCookies();
}

function mapChangePasswordError(err: unknown): ChangePasswordState {
  if (err instanceof ApiError) {
    if (err.status === 401 || err.code === "INVALID_CREDENTIALS") {
      return {
        status: "error",
        fieldErrors: { old_password: ["Kata sandi saat ini tidak tepat"] },
      };
    }
    if (err.code === "RATE_LIMITED" || err.status === 429) {
      return {
        status: "error",
        formError:
          "Terlalu banyak percobaan. Tunggu beberapa menit sebelum mencoba lagi.",
      };
    }
    if (err.status >= 500) {
      return {
        status: "error",
        formError: "Terjadi kesalahan. Periksa koneksi Anda dan coba lagi.",
      };
    }
    return {
      status: "error",
      formError: err.message || "Permintaan tidak valid. Coba lagi.",
    };
  }
  return {
    status: "error",
    formError: "Terjadi kesalahan. Periksa koneksi Anda dan coba lagi.",
  };
}
