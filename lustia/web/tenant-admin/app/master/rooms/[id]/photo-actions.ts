"use server";

/**
 * Server actions for room photo upload and deletion.
 * ADR 0012 §2.5 — reuses ADR 0011 storage pipeline exactly.
 * Frontend reads only photo_url (resolved URL) — never photo_key.
 */

import { revalidatePath } from "next/cache";

import { getAccessToken } from "@/lib/session";
import { ApiError } from "@/lib/api";
import type { Room } from "@/lib/types";

const AUTH_API_URL = process.env.AUTH_API_URL ?? "";

export type PhotoActionResult =
  | { ok: true; data: Room }
  | { ok: false; error: string; code?: string };

// ─── Helpers ──────────────────────────────────────────────────────────────────

async function buildAuthHeaders(): Promise<HeadersInit> {
  const token = await getAccessToken();
  const headers: HeadersInit = {};
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  return headers;
}

async function parseErrorResponse(res: Response): Promise<ApiError> {
  let code = "INTERNAL";
  let message = `HTTP ${res.status}`;
  try {
    const body = (await res.json()) as {
      error?: { code?: string; message?: string };
    };
    code = body.error?.code ?? code;
    message = body.error?.message ?? message;
  } catch {
    // response body was not valid JSON
  }
  return new ApiError(code, message, res.status);
}

function mapUploadError(code: string, status: number): string {
  if (status === 413 || code === "IMAGE_TOO_LARGE") return "Ukuran melebihi 5 MB.";
  if (status === 415 || code === "INVALID_IMAGE_FORMAT")
    return "Format tidak didukung (JPEG, PNG, WebP).";
  if (code === "IMAGE_DIMENSIONS_TOO_LARGE")
    return "Dimensi gambar melebihi batas (4096×4096 px).";
  if (status === 422 || code === "VALIDATION") return "File tidak valid. Coba lagi.";
  if (status === 429 || code === "UPLOAD_QUOTA_EXCEEDED")
    return "Batas unggah per jam tercapai. Coba lagi nanti.";
  return "Gagal mengunggah. Coba lagi.";
}

// ─── Upload photo ─────────────────────────────────────────────────────────────

/**
 * POST /api/v1/tenant/rooms/:id/photo
 *
 * Sends the file as multipart/form-data with field name `photo`.
 * apiFetch always sets Content-Type: application/json, which would break
 * multipart — so we call fetch directly here and inject the auth token the
 * same way apiFetch does (via getAccessToken()).
 */
export async function uploadRoomPhoto(
  roomId: string,
  formData: FormData
): Promise<PhotoActionResult> {
  if (!AUTH_API_URL) {
    return { ok: false, error: "AUTH_API_URL tidak dikonfigurasi." };
  }

  try {
    const authHeaders = await buildAuthHeaders();

    // Do NOT set Content-Type manually — fetch sets it automatically with the
    // correct boundary when the body is a FormData instance.
    const res = await fetch(
      `${AUTH_API_URL}/tenant/rooms/${roomId}/photo`,
      {
        method: "POST",
        headers: authHeaders,
        body: formData,
        cache: "no-store",
      }
    );

    if (!res.ok) {
      const err = await parseErrorResponse(res);
      return {
        ok: false,
        error: mapUploadError(err.code, res.status),
        code: err.code,
      };
    }

    const data = (await res.json()) as Room;

    revalidatePath("/master/rooms");
    revalidatePath(`/master/rooms/${roomId}`);

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return {
        ok: false,
        error: mapUploadError(err.code, err.status),
        code: err.code,
      };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Delete photo ─────────────────────────────────────────────────────────────

/**
 * DELETE /api/v1/tenant/rooms/:id/photo
 *
 * Removes the current photo. Returns 204 on success (no body).
 * We return an updated Room-like object with photo_url: null for convenience.
 */
export async function removeRoomPhoto(
  roomId: string
): Promise<PhotoActionResult> {
  if (!AUTH_API_URL) {
    return { ok: false, error: "AUTH_API_URL tidak dikonfigurasi." };
  }

  try {
    const authHeaders = await buildAuthHeaders();

    const res = await fetch(
      `${AUTH_API_URL}/tenant/rooms/${roomId}/photo`,
      {
        method: "DELETE",
        headers: authHeaders,
        cache: "no-store",
      }
    );

    if (!res.ok) {
      const err = await parseErrorResponse(res);
      return { ok: false, error: err.message, code: err.code };
    }

    // 204 No Content — re-fetch the room to get the updated object
    const authHeaders2 = await buildAuthHeaders();
    const roomRes = await fetch(
      `${AUTH_API_URL}/tenant/rooms/${roomId}`,
      {
        method: "GET",
        headers: { ...authHeaders2, "Content-Type": "application/json" },
        cache: "no-store",
      }
    );

    let data: Room;
    if (roomRes.ok) {
      data = (await roomRes.json()) as Room;
    } else {
      // Fallback: return a partial object indicating photo removed
      data = { photo_url: null } as unknown as Room;
    }

    revalidatePath("/master/rooms");
    revalidatePath(`/master/rooms/${roomId}`);

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message, code: err.code };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}
