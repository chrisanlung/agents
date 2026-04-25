"use server";

import { revalidatePath } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Booking } from "@/lib/types";

export type BookingActionResult =
  | { ok: true; data?: Booking }
  | { ok: false; errors: Record<string, string>; error?: string; code?: string };

// ─── Check-in ────────────────────────────────────────────────────────────────

const checkinSchema = z.object({
  id: z.string().uuid(),
  code: z.string().optional(),
});

export async function checkInBooking(
  id: string,
  code?: string
): Promise<BookingActionResult> {
  const parsed = checkinSchema.safeParse({ id, code });
  if (!parsed.success) {
    return { ok: false, errors: { id: "ID booking tidak valid." } };
  }

  try {
    const body: Record<string, string> = {};
    if (parsed.data.code) body.code = parsed.data.code;

    const data = await apiFetch<Booking>(
      `/tenant/bookings/${parsed.data.id}/checkin`,
      { method: "POST", body: JSON.stringify(body) },
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
          error: "Booking tidak dapat di-check-in. Periksa status booking.",
          code: err.code,
        };
      }
      if (err.status === 400) {
        return {
          ok: false,
          errors: {},
          error: "Kode booking tidak valid.",
          code: err.code,
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Mark complete ────────────────────────────────────────────────────────────

export async function markComplete(id: string): Promise<BookingActionResult> {
  try {
    const data = await apiFetch<Booking>(
      `/tenant/bookings/${id}/complete`,
      { method: "POST", body: JSON.stringify({}) },
      { auth: true }
    );

    revalidatePath("/booking");
    revalidatePath(`/booking/${id}`);

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, errors: {}, error: err.message, code: err.code };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Mark no-show ─────────────────────────────────────────────────────────────

export async function markNoShow(id: string): Promise<BookingActionResult> {
  try {
    const data = await apiFetch<Booking>(
      `/tenant/bookings/${id}/no-show`,
      { method: "POST", body: JSON.stringify({}) },
      { auth: true }
    );

    revalidatePath("/booking");
    revalidatePath(`/booking/${id}`);

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, errors: {}, error: err.message, code: err.code };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

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

// ─── Concierge create ─────────────────────────────────────────────────────────

const conciergeSchema = z.object({
  branch_id: z.string().uuid("Cabang wajib dipilih."),
  service_id: z.string().uuid("Layanan wajib dipilih."),
  addon_ids: z.array(z.string().uuid()).default([]),
  room_id: z.string().uuid().optional().nullable(),
  therapist_id: z.string().uuid().optional().nullable(),
  scheduled_start: z.string().min(1, "Slot waktu wajib dipilih."),
  customer_name: z
    .string()
    .min(1, "Nama pelanggan wajib diisi.")
    .max(200, "Nama maksimal 200 karakter."),
  customer_phone: z
    .string()
    .min(7, "Nomor WhatsApp tidak valid.")
    .max(20, "Nomor WhatsApp tidak valid."),
  customer_email: z.string().email("Email tidak valid."),
});

export type ConciergeBookingResult =
  | { ok: true; bookingId: string }
  | { ok: false; errors: Record<string, string>; error?: string; code?: string };

export async function createConciergeBooking(
  formData: FormData
): Promise<ConciergeBookingResult> {
  let addonIds: string[] = [];
  try {
    addonIds = JSON.parse((formData.get("addon_ids") as string) ?? "[]");
  } catch {
    // fall through with empty array
  }

  const raw = {
    branch_id: formData.get("branch_id"),
    service_id: formData.get("service_id"),
    addon_ids: addonIds,
    room_id: formData.get("room_id") || null,
    therapist_id: formData.get("therapist_id") || null,
    scheduled_start: formData.get("scheduled_start"),
    customer_name: formData.get("customer_name"),
    customer_phone: formData.get("customer_phone"),
    customer_email: formData.get("customer_email"),
  };

  const parsed = conciergeSchema.safeParse(raw);
  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      errors[e.path.join(".")] = e.message;
    });
    return { ok: false, errors };
  }

  const body: Record<string, unknown> = {
    branch_id: parsed.data.branch_id,
    service_id: parsed.data.service_id,
    addon_ids: parsed.data.addon_ids,
    scheduled_start: parsed.data.scheduled_start,
    customer_name: parsed.data.customer_name,
    customer_phone: parsed.data.customer_phone,
    customer_email: parsed.data.customer_email,
    payment_method: "paid_at_venue",
  };
  if (parsed.data.room_id) body.room_id = parsed.data.room_id;
  if (parsed.data.therapist_id) body.therapist_id = parsed.data.therapist_id;

  try {
    const resp = await apiFetch<{ booking_id: string; code: string }>(
      "/tenant/bookings",
      { method: "POST", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidatePath("/booking");

    return { ok: true, bookingId: resp.booking_id };
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 409) {
        return {
          ok: false,
          errors: {},
          error:
            "Slot ini sudah tidak tersedia. Pilih slot lain.",
          code: err.code,
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}
