"use server";

import { revalidateTag } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Therapist } from "@/lib/types";

// ─── Schema ──────────────────────────────────────────────────────────────────

const prepSchema = z.object({
  prep_minutes: z.number().int().min(0).max(60),
});

// ─── Update prep_minutes (PATCH /tenant/therapists/:id) ──────────────────────

export type UpdatePrepResult =
  | { ok: true; prep_minutes: number }
  | { ok: false; error: string };

export async function updateTherapistPrep(
  therapistId: string,
  prepMinutes: number
): Promise<UpdatePrepResult> {
  const parsed = prepSchema.safeParse({ prep_minutes: prepMinutes });
  if (!parsed.success) {
    return { ok: false, error: "0–60 menit." };
  }

  try {
    const data = await apiFetch<Therapist>(
      `/tenant/therapists/${therapistId}`,
      {
        method: "PATCH",
        body: JSON.stringify({ prep_minutes: parsed.data.prep_minutes }),
      },
      { auth: true }
    );

    revalidateTag(`therapist-${therapistId}`, { expire: 0 });
    revalidateTag("therapists", { expire: 0 });

    return { ok: true, prep_minutes: data.prep_minutes };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}
