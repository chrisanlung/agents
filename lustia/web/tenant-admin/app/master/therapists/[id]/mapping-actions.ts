"use server";

import { revalidateTag } from "next/cache";

import { apiFetch, ApiError } from "@/lib/api";
import type { ServiceMappingResponse } from "@/lib/types";

// ─── Full-replace service mapping (PUT /therapists/:id/services §11.6.2) ────

export type UpdateMappingResult =
  | { ok: true; data: ServiceMappingResponse }
  | { ok: false; error: string };

export async function updateServiceMapping(
  therapistId: string,
  serviceIds: string[]
): Promise<UpdateMappingResult> {
  try {
    const data = await apiFetch<ServiceMappingResponse>(
      `/tenant/therapists/${therapistId}/services`,
      {
        method: "PUT",
        body: JSON.stringify({ service_ids: serviceIds }),
      },
      { auth: true }
    );

    revalidateTag(`therapist-${therapistId}`);
    revalidateTag("therapists");

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
