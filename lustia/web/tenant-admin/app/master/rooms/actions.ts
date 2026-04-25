"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Room, RoomSortOrderItem } from "@/lib/types";

// ─── Shared schema ────────────────────────────────────────────────────────────

const amenityItem = z
  .string()
  .min(1)
  .max(80, "Fasilitas maksimal 80 karakter.");

const roomSchema = z.object({
  branch_id: z.string().uuid("Branch tidak valid."),
  name: z
    .string()
    .min(1, "Nama ruangan wajib diisi.")
    .max(120, "Nama maksimal 120 karakter."),
  description: z
    .string()
    .max(500, "Deskripsi maksimal 500 karakter.")
    .optional()
    .nullable(),
  room_type: z.enum(["single", "couple", "group", "vip"], {
    required_error: "Tipe ruangan wajib dipilih.",
    invalid_type_error: "Tipe ruangan tidak valid.",
  }),
  capacity: z.coerce
    .number({ invalid_type_error: "Kapasitas harus berupa angka." })
    .int("Kapasitas harus bilangan bulat.")
    .min(1, "Kapasitas minimal 1.")
    .max(20, "Kapasitas maksimal 20."),
  amenities: z
    .array(amenityItem)
    .max(20, "Fasilitas maksimal 20 item.")
    .default([]),
  is_active: z.boolean(),
});

const updateRoomSchema = roomSchema.partial().omit({ branch_id: true });

export type RoomActionResult =
  | { ok: true; data?: Room }
  | { ok: false; errors: Record<string, string>; error?: string; code?: string };

// ─── Create ───────────────────────────────────────────────────────────────────

export async function createRoom(
  formData: FormData
): Promise<RoomActionResult> {
  let amenities: string[] = [];
  try {
    amenities = JSON.parse(formData.get("amenities") as string ?? "[]");
  } catch {
    // fall through with empty array
  }

  const raw = {
    branch_id: formData.get("branch_id"),
    name: formData.get("name"),
    description: formData.get("description") || null,
    room_type: formData.get("room_type"),
    capacity: formData.get("capacity"),
    amenities,
    is_active: formData.get("is_active") === "true",
  };

  const parsed = roomSchema.safeParse(raw);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  const body: Record<string, unknown> = {
    branch_id: parsed.data.branch_id,
    name: parsed.data.name,
    room_type: parsed.data.room_type,
    capacity: parsed.data.capacity,
    amenities: parsed.data.amenities,
    is_active: parsed.data.is_active,
  };
  if (parsed.data.description) body.description = parsed.data.description;

  try {
    const data = await apiFetch<Room>(
      "/tenant/rooms",
      { method: "POST", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidatePath("/master/rooms");
    redirect(
      `/master/rooms?flash=${encodeURIComponent(`Ruangan "${data.name}" berhasil dibuat.`)}`
    );

    // unreachable — redirect throws
    return { ok: true, data };
  } catch (err) {
    if (err instanceof Error && err.message === "NEXT_REDIRECT") throw err;
    if (err instanceof ApiError) {
      if (err.status === 409) {
        return {
          ok: false,
          errors: { name: "Nama ruangan sudah digunakan di cabang ini." },
          code: "DUPLICATE_ROOM_NAME",
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Update ───────────────────────────────────────────────────────────────────

export async function updateRoom(
  id: string,
  formData: FormData
): Promise<RoomActionResult> {
  let amenities: string[] = [];
  try {
    amenities = JSON.parse(formData.get("amenities") as string ?? "[]");
  } catch {
    // fall through with empty array
  }

  const raw = {
    name: formData.get("name"),
    description: formData.get("description") || null,
    room_type: formData.get("room_type"),
    capacity: formData.get("capacity"),
    amenities,
  };

  const parsed = updateRoomSchema.safeParse(raw);

  if (!parsed.success) {
    const errors: Record<string, string> = {};
    parsed.error.errors.forEach((e) => {
      const field = e.path.join(".");
      errors[field] = e.message;
    });
    return { ok: false, errors };
  }

  // `is_active` flows through toggleRoomStatus — not sent here.
  try {
    const data = await apiFetch<Room>(
      `/tenant/rooms/${id}`,
      {
        method: "PATCH",
        body: JSON.stringify({
          name: parsed.data.name,
          description: parsed.data.description,
          room_type: parsed.data.room_type,
          capacity: parsed.data.capacity,
          amenities: parsed.data.amenities,
        }),
      },
      { auth: true }
    );

    revalidatePath("/master/rooms");

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 409) {
        return {
          ok: false,
          errors: { name: "Nama ruangan sudah digunakan di cabang ini." },
          code: "DUPLICATE_ROOM_NAME",
        };
      }
      return { ok: false, errors: {}, error: err.message };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Toggle Status ────────────────────────────────────────────────────────────

export type ToggleRoomStatusResult =
  | { ok: true; is_active: boolean }
  | { ok: false; error: string };

export async function toggleRoomStatus(
  id: string,
  currentActive: boolean
): Promise<ToggleRoomStatusResult> {
  const is_active = !currentActive;

  try {
    await apiFetch<Room>(
      `/tenant/rooms/${id}/status`,
      { method: "PATCH", body: JSON.stringify({ is_active }) },
      { auth: true }
    );

    revalidatePath("/master/rooms");

    return { ok: true, is_active };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Delete ───────────────────────────────────────────────────────────────────

export type DeleteRoomResult = { ok: true } | { ok: false; error: string };

export async function deleteRoom(id: string): Promise<DeleteRoomResult> {
  try {
    await apiFetch<void>(
      `/tenant/rooms/${id}`,
      { method: "DELETE" },
      { auth: true }
    );

    revalidatePath("/master/rooms");

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Reorder ──────────────────────────────────────────────────────────────────

export type ReorderRoomsResult = { ok: true } | { ok: false; error: string };

export async function reorderRooms(
  branchId: string,
  items: RoomSortOrderItem[]
): Promise<ReorderRoomsResult> {
  try {
    await apiFetch<void>(
      "/tenant/rooms/reorder",
      { method: "PUT", body: JSON.stringify({ branch_id: branchId, items }) },
      { auth: true }
    );

    revalidatePath("/master/rooms");

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
