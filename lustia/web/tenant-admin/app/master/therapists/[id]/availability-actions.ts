"use server";

import { revalidateTag } from "next/cache";

import { apiFetch, ApiError } from "@/lib/api";
import type { AvailabilityWindow, AvailabilityResponse } from "@/lib/types";

// ─── Full-replace availability (PUT /therapists/:id/availability §11.7.2) ───

export type SaveAvailabilityResult =
  | { ok: true; data: AvailabilityResponse }
  | { ok: false; error: string };

export async function saveAvailability(
  therapistId: string,
  windows: Omit<AvailabilityWindow, "id">[]
): Promise<SaveAvailabilityResult> {
  try {
    const data = await apiFetch<AvailabilityResponse>(
      `/tenant/therapists/${therapistId}/availability`,
      {
        method: "PUT",
        body: JSON.stringify({ windows }),
      },
      { auth: true }
    );

    revalidateTag(`therapist-availability-${therapistId}`, { expire: 0 });

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.code === "AVAILABILITY_OVERLAP") {
        return {
          ok: false,
          error: "Jadwal yang dimasukkan saling tumpang tindih. Periksa kembali.",
        };
      }
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
