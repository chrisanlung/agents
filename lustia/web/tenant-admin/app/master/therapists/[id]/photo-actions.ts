"use server";

/**
 * Server actions for therapist photo upload and deletion.
 * ADR 0011 §2.4 — photo_key is managed server-side via dedicated endpoints.
 * Frontend reads only photo_url (resolved URL) — never photo_key.
 */

import { revalidateTag } from "next/cache";

import { getAccessToken } from "@/lib/session";
import { ApiError } from "@/lib/api";
import type { Therapist } from "@/lib/types";

const AUTH_API_URL = process.env.AUTH_API_URL ?? "";

export type PhotoActionResult =
  | { ok: true; data: Therapist }
  | { ok: false; error: string; code?: string };

// ─── Helpers ─────────────────────────────────────────────────────────────────

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

// ─── Upload photo ─────────────────────────────────────────────────────────────

/**
 * POST /api/v1/tenant/therapists/:id/photo
 *
 * Sends the file as multipart/form-data with field name `photo`.
 * apiFetch always sets Content-Type: application/json, which would break
 * multipart — so we call fetch directly here and inject the auth token the
 * same way apiFetch does (via getAccessToken()).
 */
export async function uploadTherapistPhoto(
  therapistId: string,
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
      `${AUTH_API_URL}/tenant/therapists/${therapistId}/photo`,
      {
        method: "POST",
        headers: authHeaders,
        body: formData,
        cache: "no-store",
      }
    );

    if (!res.ok) {
      const err = await parseErrorResponse(res);
      return { ok: false, error: err.message, code: err.code };
    }

    const data = (await res.json()) as Therapist;

    revalidateTag("therapists", { expire: 0 });
    revalidateTag(`therapist-${therapistId}`, { expire: 0 });

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message, code: err.code };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Delete photo ─────────────────────────────────────────────────────────────

/**
 * DELETE /api/v1/tenant/therapists/:id/photo
 *
 * Removes the current photo. Returns the updated therapist DTO with
 * photo_url: null on success.
 */
export async function deleteTherapistPhoto(
  therapistId: string
): Promise<PhotoActionResult> {
  if (!AUTH_API_URL) {
    return { ok: false, error: "AUTH_API_URL tidak dikonfigurasi." };
  }

  try {
    const authHeaders = await buildAuthHeaders();

    const res = await fetch(
      `${AUTH_API_URL}/tenant/therapists/${therapistId}/photo`,
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

    const data = (await res.json()) as Therapist;

    revalidateTag("therapists", { expire: 0 });
    revalidateTag(`therapist-${therapistId}`, { expire: 0 });

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message, code: err.code };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}
