"use server";

import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";

const registerSchema = z.object({
  company_name: z.string().min(2, "Nama perusahaan minimal 2 karakter").max(200),
  package: z.enum(["starter", "growth", "enterprise"]),
  contact_name: z.string().min(1, "Nama kontak wajib diisi").max(200),
  contact_email: z.string().email("Email tidak valid").max(320),
  contact_phone: z.string().max(30).optional().or(z.literal("")),
});

export type RegisterFormState =
  | { status: "idle" }
  | { status: "error"; fieldErrors?: Record<string, string[]>; toast?: string }
  | { status: "success"; registrationId: string; contactEmail: string };

interface CompanyRegistrationResponse {
  registration_id: string;
  status: string;
}

export async function registerAction(
  _prev: RegisterFormState,
  formData: FormData
): Promise<RegisterFormState> {
  const raw = {
    company_name: (formData.get("company_name") as string | null)?.trim() ?? "",
    package: formData.get("package") ?? "starter",
    contact_name: (formData.get("contact_name") as string | null)?.trim() ?? "",
    contact_email: (formData.get("contact_email") as string | null)?.trim() ?? "",
    contact_phone: (formData.get("contact_phone") as string | null)?.trim() ?? "",
  };

  const parsed = registerSchema.safeParse(raw);
  if (!parsed.success) {
    const fieldErrors: Record<string, string[]> = {};
    for (const [field, errors] of Object.entries(parsed.error.flatten().fieldErrors)) {
      fieldErrors[field] = errors ?? [];
    }
    return { status: "error", fieldErrors };
  }

  const body: Record<string, string> = {
    company_name: parsed.data.company_name,
    package: parsed.data.package,
    contact_name: parsed.data.contact_name,
    contact_email: parsed.data.contact_email,
  };
  if (parsed.data.contact_phone) {
    body.contact_phone = parsed.data.contact_phone;
  }

  try {
    const res = await apiFetch<CompanyRegistrationResponse>("/register/company", {
      method: "POST",
      body: JSON.stringify(body),
    });
    return {
      status: "success",
      registrationId: res.registration_id,
      contactEmail: parsed.data.contact_email,
    };
  } catch (err) {
    if (err instanceof ApiError) {
      return mapApiErrorToState(err);
    }
    return { status: "error", toast: "Layanan tidak tersedia. Silakan coba lagi." };
  }
}

function mapApiErrorToState(err: ApiError): RegisterFormState {
  if (err.code === "VALIDATION") {
    return { status: "error", toast: err.message || "Data tidak valid." };
  }
  if (
    err.code === "CONFLICT" ||
    err.code === "CONFLICT_SLUG_TAKEN" ||
    err.code === "CONFLICT_EMAIL_PENDING"
  ) {
    // Generic message — backend collapses uniqueness errors to prevent
    // email/slug enumeration from unauthenticated callers. Keep the message
    // neutral so it doesn't reveal which specific field is taken.
    return {
      status: "error",
      toast:
        "Data yang Anda masukkan sudah digunakan. Silakan periksa email atau nama perusahaan, lalu coba lagi.",
    };
  }
  if (err.code === "RATE_LIMITED") {
    return {
      status: "error",
      toast: "Terlalu banyak percobaan. Mohon tunggu sebentar sebelum mencoba lagi.",
    };
  }
  if (err.status >= 500) {
    return { status: "error", toast: "Layanan tidak tersedia. Silakan coba lagi." };
  }
  return { status: "error", toast: err.message || "Gagal mengirim pendaftaran." };
}
