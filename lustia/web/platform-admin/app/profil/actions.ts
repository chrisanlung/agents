"use server";

import { redirect } from "next/navigation";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";

const schema = z.object({
  full_name: z
    .string()
    .trim()
    .min(1, "Nama wajib diisi")
    .max(200, "Nama terlalu panjang"),
  phone: z
    .string()
    .regex(/^\d{5,30}$/, "Nomor telepon harus 5–30 digit angka")
    .optional()
    .or(z.literal("")),
});

export interface UserProfileResponse {
  id: string;
  email: string;
  full_name: string;
  phone: string | null;
  avatar_url: string | null;
  is_active: boolean;
  is_super_admin?: boolean;
  must_change_password: boolean;
}

export type UpdateProfileState =
  | { status: "idle" }
  | {
      status: "error";
      fieldErrors?: Record<string, string[]>;
      formError?: string;
    }
  | { status: "success"; user: UserProfileResponse };

export async function updateProfileAction(
  _prev: UpdateProfileState,
  formData: FormData
): Promise<UpdateProfileState> {
  const raw = {
    full_name: String(formData.get("full_name") ?? ""),
    phone: String(formData.get("phone") ?? ""),
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

  let result: { user: UserProfileResponse };
  try {
    result = await apiFetch<{ user: UserProfileResponse }>(
      "/auth/me",
      {
        method: "PATCH",
        body: JSON.stringify({
          full_name: parsed.data.full_name,
          phone: parsed.data.phone || null,
        }),
      },
      { auth: true }
    );
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 401) {
        redirect("/login");
      }
      if (err.status === 429 || err.code === "RATE_LIMITED") {
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
      // 400 — try to extract field errors from message
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

  return { status: "success", user: result.user };
}
