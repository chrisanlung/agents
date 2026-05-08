"use server";

import { revalidatePath } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Booking } from "@/lib/types";

// ─── Sync Payment Status ───────────────────────────────────────────────────────

export async function syncPaymentStatus(
  id: string
): Promise<BookingActionResult> {
  try {
    const data = await apiFetch<Booking>(
      `/tenant/bookings/${id}/sync-payment`,
      { method: "POST", body: "{}" },
      { auth: true }
    );
    revalidatePath("/booking");
    revalidatePath(`/booking/${id}`);
    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

export type BookingActionResult =
  | { ok: true; data?: Booking }
  | { ok: false; errors: Record<string, string>; error?: string; code?: string };

// ─── Cancel ───────────────────────────────────────────────────────────────────

const cancelSchema = z.object({
  id: z.string().uuid(),
  reason: z
    .string()
    .min(1, "Alasan pembatalan wajib diisi.")
    .max(500, "Alasan maksimal 500 karakter."),
});

export async function cancelBooking(
  id: string,
  reason: string
): Promise<BookingActionResult> {
  const parsed = cancelSchema.safeParse({ id, reason });
  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      errors[e.path.join(".")] = e.message;
    });
    return { ok: false, errors };
  }

  try {
    const data = await apiFetch<Booking>(
      `/tenant/bookings/${parsed.data.id}/cancel`,
      {
        method: "POST",
        body: JSON.stringify({ reason: parsed.data.reason }),
      },
      { auth: true }
    );

    revalidatePath("/booking");
    revalidatePath(`/booking/${parsed.data.id}`);

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 409) {
        return {
          ok: false,
          errors: {},
          error:
            "Booking tidak dapat dibatalkan. Periksa status booking.",
          code: err.code,
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}
