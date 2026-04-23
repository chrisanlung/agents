"use server";

import { redirect } from "next/navigation";
import { revalidateTag } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Therapist } from "@/lib/types";

// ─── Shared schema ───────────────────────────────────────────────────────────

const therapistSchema = z.object({
  full_name: z.string().min(1, "Nama lengkap wajib diisi.").max(200),
  gender: z.enum(["male", "female", "other"]).optional().nullable(),
  phone: z.string().max(30).optional().nullable(),
  email: z
    .string()
    .email("Format email tidak valid.")
    .optional()
    .nullable()
    .or(z.literal("")),
  bio: z.string().max(500).optional().nullable(),
  joined_at: z.string().optional().nullable(),
  branch_id: z.string().uuid("Branch ID tidak valid.").optional().nullable(),
});

export type TherapistFormResult =
  | { ok: true; data: Therapist }
  | { ok: false; errors: Record<string, string>; error?: string };

// ─── Create ──────────────────────────────────────────────────────────────────

export async function createTherapist(
  formData: FormData
): Promise<TherapistFormResult> {
  const raw = Object.fromEntries(formData.entries());
  // Treat empty strings as null for optional fields
  const cleaned = Object.fromEntries(
    Object.entries(raw).map(([k, v]) => [k, v === "" ? null : v])
  );

  const parsed = therapistSchema.safeParse(cleaned);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  const body: Record<string, unknown> = {
    full_name: parsed.data.full_name,
  };
  if (parsed.data.branch_id) body.branch_id = parsed.data.branch_id;
  if (parsed.data.gender) body.gender = parsed.data.gender;
  if (parsed.data.phone) body.phone = parsed.data.phone;
  if (parsed.data.email) body.email = parsed.data.email;
  if (parsed.data.bio) body.bio = parsed.data.bio;
  if (parsed.data.joined_at) body.joined_at = parsed.data.joined_at;

  try {
    const data = await apiFetch<Therapist>(
      "/tenant/therapists",
      { method: "POST", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidateTag("therapists");
    redirect(`/master/therapists/${data.id}?tab=layanan`);

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

export async function updateTherapist(
  id: string,
  formData: FormData
): Promise<TherapistFormResult> {
  const raw = Object.fromEntries(formData.entries());
  const cleaned = Object.fromEntries(
    Object.entries(raw).map(([k, v]) => [k, v === "" ? null : v])
  );

  const parsed = therapistSchema.safeParse(cleaned);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  const body: Record<string, unknown> = {
    full_name: parsed.data.full_name,
  };
  if (parsed.data.gender !== undefined) body.gender = parsed.data.gender;
  if (parsed.data.phone !== undefined) body.phone = parsed.data.phone;
  if (parsed.data.email !== undefined) body.email = parsed.data.email;
  if (parsed.data.bio !== undefined) body.bio = parsed.data.bio;
  if (parsed.data.joined_at !== undefined) body.joined_at = parsed.data.joined_at;

  try {
    const data = await apiFetch<Therapist>(
      `/tenant/therapists/${id}`,
      { method: "PATCH", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidateTag("therapists");
    revalidateTag(`therapist-${id}`);

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Toggle Status ───────────────────────────────────────────────────────────

export type ToggleTherapistStatusResult =
  | { ok: true; is_active: boolean }
  | { ok: false; error: string };

export async function toggleTherapistStatus(
  id: string,
  currentActive: boolean
): Promise<ToggleTherapistStatusResult> {
  const is_active = !currentActive;

  try {
    await apiFetch<Therapist>(
      `/tenant/therapists/${id}/status`,
      { method: "PATCH", body: JSON.stringify({ is_active }) },
      { auth: true }
    );

    revalidateTag("therapists");
    revalidateTag(`therapist-${id}`);

    return { ok: true, is_active };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Delete ──────────────────────────────────────────────────────────────────

export type DeleteTherapistResult =
  | { ok: true }
  | { ok: false; error: string };

export async function deleteTherapist(
  id: string
): Promise<DeleteTherapistResult> {
  try {
    await apiFetch<void>(
      `/tenant/therapists/${id}`,
      { method: "DELETE" },
      { auth: true }
    );

    revalidateTag("therapists");

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
