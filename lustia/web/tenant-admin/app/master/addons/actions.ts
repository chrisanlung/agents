"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Addon, AddonSortOrderItem } from "@/lib/types";

// ─── Shared schema ───────────────────────────────────────────────────────────

const addonSchema = z.object({
  name: z
    .string()
    .min(1, "Nama add-on wajib diisi.")
    .max(120, "Nama maksimal 120 karakter."),
  description: z
    .string()
    .max(500, "Deskripsi maksimal 500 karakter.")
    .optional()
    .nullable(),
  price_idr: z.coerce
    .number({ invalid_type_error: "Harga harus berupa angka." })
    .int("Harga harus bilangan bulat.")
    .min(0, "Harga tidak boleh negatif."),
  is_active: z.boolean(),
});

export type AddonActionResult =
  | { ok: true; data?: Addon }
  | { ok: false; errors: Record<string, string>; error?: string; code?: string };

// ─── Create ──────────────────────────────────────────────────────────────────

export async function createAddon(
  formData: FormData
): Promise<AddonActionResult> {
  const raw = {
    name: formData.get("name"),
    description: formData.get("description") || null,
    price_idr: formData.get("price_idr"),
    is_active: formData.get("is_active") === "true",
  };

  const parsed = addonSchema.safeParse(raw);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  const body: Record<string, unknown> = {
    name: parsed.data.name,
    price_idr: parsed.data.price_idr,
    is_active: parsed.data.is_active,
  };
  if (parsed.data.description) body.description = parsed.data.description;

  try {
    const data = await apiFetch<Addon>(
      "/tenant/addons",
      { method: "POST", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidatePath("/master/addons");
    redirect(`/master/addons?flash=${encodeURIComponent(`Add-on "${data.name}" berhasil dibuat.`)}`);

    // unreachable — redirect throws
    return { ok: true, data };
  } catch (err) {
    if (err instanceof Error && err.message === "NEXT_REDIRECT") throw err;
    if (err instanceof ApiError) {
      if (err.status === 409) {
        return {
          ok: false,
          errors: { name: "Nama add-on sudah digunakan." },
          code: "DUPLICATE",
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Update ──────────────────────────────────────────────────────────────────

export async function updateAddon(
  id: string,
  formData: FormData
): Promise<AddonActionResult> {
  const raw = {
    name: formData.get("name"),
    description: formData.get("description") || null,
    price_idr: formData.get("price_idr"),
    is_active: formData.get("is_active") === "true",
  };

  const parsed = addonSchema.safeParse(raw);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  // `is_active` is NOT sent here — status changes flow through
  // `toggleAddonStatus` (PATCH /addons/:id/status). The Update endpoint only
  // accepts name / description / price / sort_order; sending `is_active` would
  // be silently dropped by Gin's binding and mislead the caller.
  try {
    const data = await apiFetch<Addon>(
      `/tenant/addons/${id}`,
      {
        method: "PATCH",
        body: JSON.stringify({
          name: parsed.data.name,
          description: parsed.data.description,
          price_idr: parsed.data.price_idr,
        }),
      },
      { auth: true }
    );

    revalidatePath("/master/addons");

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 409) {
        return {
          ok: false,
          errors: { name: "Nama add-on sudah digunakan." },
          code: "DUPLICATE",
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Toggle Status ────────────────────────────────────────────────────────────

export type ToggleAddonStatusResult =
  | { ok: true; is_active: boolean }
  | { ok: false; error: string };

export async function toggleAddonStatus(
  id: string,
  currentActive: boolean
): Promise<ToggleAddonStatusResult> {
  const is_active = !currentActive;

  try {
    await apiFetch<Addon>(
      `/tenant/addons/${id}/status`,
      { method: "PATCH", body: JSON.stringify({ is_active }) },
      { auth: true }
    );

    revalidatePath("/master/addons");

    return { ok: true, is_active };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Delete ───────────────────────────────────────────────────────────────────

export type DeleteAddonResult = { ok: true } | { ok: false; error: string };

export async function deleteAddon(id: string): Promise<DeleteAddonResult> {
  try {
    await apiFetch<void>(
      `/tenant/addons/${id}`,
      { method: "DELETE" },
      { auth: true }
    );

    revalidatePath("/master/addons");

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Reorder ──────────────────────────────────────────────────────────────────

export type ReorderAddonsResult = { ok: true } | { ok: false; error: string };

export async function reorderAddons(
  items: AddonSortOrderItem[]
): Promise<ReorderAddonsResult> {
  try {
    await apiFetch<void>(
      "/tenant/addons/reorder",
      { method: "PUT", body: JSON.stringify({ items }) },
      { auth: true }
    );

    revalidatePath("/master/addons");

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
