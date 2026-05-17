"use server";

import { redirect } from "next/navigation";
import { revalidateTag } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Service } from "@/lib/types";

// ─── Shared schema ───────────────────────────────────────────────────────────

const serviceSchema = z.object({
  name: z.string().min(1, "Nama layanan wajib diisi.").max(200),
  description: z.string().max(1000).optional().nullable(),
  category: z.string().max(100).optional().nullable(),
  duration_minutes: z.coerce
    .number({ invalid_type_error: "Durasi harus berupa angka." })
    .int("Durasi harus bilangan bulat.")
    .min(1, "Durasi minimal 1 menit.")
    .max(1440, "Durasi maksimal 1440 menit."),
  price_idr: z.coerce
    .number({ invalid_type_error: "Harga harus berupa angka." })
    .int("Harga harus bilangan bulat.")
    .min(0, "Harga tidak boleh negatif."),
});

export type ServiceFormResult =
  | { ok: true; data: Service }
  | { ok: false; errors: Record<string, string>; error?: string };

// ─── Create ──────────────────────────────────────────────────────────────────

export async function createService(
  formData: FormData
): Promise<ServiceFormResult> {
  const raw = Object.fromEntries(formData.entries());
  const cleaned = Object.fromEntries(
    Object.entries(raw).map(([k, v]) => [k, v === "" ? null : v])
  );

  const parsed = serviceSchema.safeParse(cleaned);

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
    duration_minutes: parsed.data.duration_minutes,
    price_idr: parsed.data.price_idr,
  };
  if (parsed.data.description) body.description = parsed.data.description;
  if (parsed.data.category) body.category = parsed.data.category;

  try {
    const data = await apiFetch<Service>(
      "/tenant/services",
      { method: "POST", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidateTag("services", { expire: 0 });
    redirect(`/master/services/${data.id}`);

    return { ok: true, data };
  } catch (err) {
    if (err instanceof Error && err.message === "NEXT_REDIRECT") throw err;
    if (err instanceof ApiError) {
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Update ──────────────────────────────────────────────────────────────────

export async function updateService(
  id: string,
  formData: FormData
): Promise<ServiceFormResult> {
  const raw = Object.fromEntries(formData.entries());
  const cleaned = Object.fromEntries(
    Object.entries(raw).map(([k, v]) => [k, v === "" ? null : v])
  );

  const parsed = serviceSchema.safeParse(cleaned);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  try {
    const data = await apiFetch<Service>(
      `/tenant/services/${id}`,
      {
        method: "PATCH",
        body: JSON.stringify({
          name: parsed.data.name,
          description: parsed.data.description ?? null,
          category: parsed.data.category ?? null,
          duration_minutes: parsed.data.duration_minutes,
          price_idr: parsed.data.price_idr,
        }),
      },
      { auth: true }
    );

    revalidateTag("services", { expire: 0 });
    revalidateTag(`service-${id}`, { expire: 0 });

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Toggle Status ───────────────────────────────────────────────────────────

export type ToggleServiceStatusResult =
  | { ok: true; is_active: boolean }
  | { ok: false; error: string };

export async function toggleServiceStatus(
  id: string,
  currentActive: boolean
): Promise<ToggleServiceStatusResult> {
  const is_active = !currentActive;

  try {
    await apiFetch<Service>(
      `/tenant/services/${id}/status`,
      { method: "PATCH", body: JSON.stringify({ is_active }) },
      { auth: true }
    );

    revalidateTag("services", { expire: 0 });
    revalidateTag(`service-${id}`, { expire: 0 });

    return { ok: true, is_active };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Delete ──────────────────────────────────────────────────────────────────

export type DeleteServiceResult =
  | { ok: true }
  | { ok: false; error: string };

export async function deleteService(
  id: string
): Promise<DeleteServiceResult> {
  try {
    await apiFetch<void>(
      `/tenant/services/${id}`,
      { method: "DELETE" },
      { auth: true }
    );

    revalidateTag("services", { expire: 0 });

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
