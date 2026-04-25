"use server";

import { redirect } from "next/navigation";
import { revalidateTag } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Branch, BranchStatus } from "@/lib/types";

// ─── Shared schema ───────────────────────────────────────────────────────────

// DEFAULT_OPERATIONAL_HOURS removed — was unused in server action (used in client form only)

const branchSchema = z.object({
  name: z.string().min(1, "Nama cabang wajib diisi.").max(200),
  code: z
    .string()
    .min(2, "Kode minimal 2 karakter.")
    .max(50)
    .regex(/^[a-z0-9][a-z0-9-]*[a-z0-9]$/, {
      message:
        "Kode hanya boleh berisi huruf kecil, angka, dan tanda hubung, serta tidak boleh diawali atau diakhiri tanda hubung.",
    }),
  address_line1: z.string().max(200).optional().nullable(),
  address_line2: z.string().max(200).optional().nullable(),
  city: z.string().max(100).optional().nullable(),
  province: z.string().max(100).optional().nullable(),
  postal_code: z.string().max(10).optional().nullable(),
  country: z.string().length(2).default("ID"),
  timezone: z.enum(["Asia/Jakarta", "Asia/Makassar", "Asia/Jayapura"]),
  contact_phone: z.string().max(30).optional().nullable(),
  contact_email: z.string().email().optional().nullable().or(z.literal("")),
  operational_hours: z.string().optional().nullable(),
});

export type BranchFormResult =
  | { ok: true; data: Branch }
  | { ok: false; errors: Record<string, string>; error?: string };

function buildBody(data: z.infer<typeof branchSchema>) {
  const body: Record<string, unknown> = {
    name: data.name,
    code: data.code,
    country: data.country,
    timezone: data.timezone,
  };

  if (data.address_line1) body.address_line1 = data.address_line1;
  if (data.address_line2) body.address_line2 = data.address_line2;
  if (data.city) body.city = data.city;
  if (data.province) body.province = data.province;
  if (data.postal_code) body.postal_code = data.postal_code;
  if (data.contact_phone) body.contact_phone = data.contact_phone;
  if (data.contact_email) body.contact_email = data.contact_email;

  // operational_hours is stored as JSON — parse if provided, else use default
  if (data.operational_hours) {
    try {
      body.operational_hours = JSON.parse(data.operational_hours);
    } catch {
      // Will be caught by the validate step
    }
  }

  return body;
}

// ─── Create ──────────────────────────────────────────────────────────────────

export async function createBranch(
  formData: FormData
): Promise<BranchFormResult> {
  const raw = Object.fromEntries(formData.entries());
  const parsed = branchSchema.safeParse(raw);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  try {
    const data = await apiFetch<Branch>(
      "/tenant/branches",
      {
        method: "POST",
        body: JSON.stringify(buildBody(parsed.data)),
      },
      { auth: true }
    );

    revalidateTag("branches");
    revalidateTag("onboarding-state");

    // Redirect to the branches list after success
    redirect("/branches?created=1");

    return { ok: true, data };
  } catch (err) {
    if (err instanceof Error && err.message === "NEXT_REDIRECT") throw err;

    if (err instanceof ApiError) {
      if (err.code === "BRANCH_LIMIT_REACHED") {
        return {
          ok: false,
          errors: {},
          error:
            "Batas cabang tercapai. Tingkatkan paket untuk menambah cabang.",
        };
      }
      if (err.code === "DUPLICATE_CODE") {
        return {
          ok: false,
          errors: { code: "Kode cabang sudah digunakan oleh cabang lain." },
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Update ──────────────────────────────────────────────────────────────────

export async function updateBranch(
  id: string,
  formData: FormData
): Promise<BranchFormResult> {
  const raw = Object.fromEntries(formData.entries());
  const parsed = branchSchema.safeParse(raw);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  try {
    const data = await apiFetch<Branch>(
      `/tenant/branches/${id}`,
      {
        method: "PATCH",
        body: JSON.stringify(buildBody(parsed.data)),
      },
      { auth: true }
    );

    revalidateTag("branches");

    redirect("/branches?updated=1");

    return { ok: true, data };
  } catch (err) {
    if (err instanceof Error && err.message === "NEXT_REDIRECT") throw err;

    if (err instanceof ApiError) {
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Toggle Status ───────────────────────────────────────────────────────────

export type ToggleBranchStatusResult =
  | { ok: true; newStatus: BranchStatus }
  | { ok: false; error: string };

export async function toggleBranchStatus(
  id: string,
  currentStatus: BranchStatus
): Promise<ToggleBranchStatusResult> {
  const newStatus: BranchStatus =
    currentStatus === "active" ? "inactive" : "active";

  try {
    await apiFetch<Branch>(
      `/tenant/branches/${id}/status`,
      {
        method: "PATCH",
        body: JSON.stringify({ status: newStatus }),
      },
      { auth: true }
    );

    revalidateTag("branches");

    return { ok: true, newStatus };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Delete ──────────────────────────────────────────────────────────────────

export type DeleteBranchResult =
  | { ok: true }
  | { ok: false; error: string };

export async function deleteBranch(id: string): Promise<DeleteBranchResult> {
  try {
    await apiFetch<void>(
      `/tenant/branches/${id}`,
      { method: "DELETE" },
      { auth: true }
    );

    revalidateTag("branches");
    revalidateTag("onboarding-state");

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
