"use server";

import { redirect } from "next/navigation";
import { revalidateTag } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { ApproveRegistrationResponse, TenantRegistration } from "@/lib/types";

// ─── Approve ────────────────────────────────────────────────────────────────

const approveSchema = z.object({
  id: z.string().uuid(),
  package: z.enum(["starter", "growth", "enterprise"]).optional(),
  max_branches: z.coerce.number().int().min(1).optional(),
});

export type ApproveResult =
  | { ok: true; data: ApproveRegistrationResponse }
  | { ok: false; error: string };

export async function approveRegistration(
  formData: FormData
): Promise<ApproveResult> {
  const raw = {
    id: formData.get("id"),
    package: formData.get("package") ?? undefined,
    max_branches: formData.get("max_branches") ?? undefined,
  };

  const parsed = approveSchema.safeParse(raw);
  if (!parsed.success) {
    return { ok: false, error: "Input tidak valid." };
  }

  const { id, ...overrides } = parsed.data;

  // Build the request body — only include overrides that were actually provided
  const body: Record<string, unknown> = {};
  if (overrides.package) body.package = overrides.package;
  if (overrides.max_branches !== undefined) body.max_branches = overrides.max_branches;

  try {
    const data = await apiFetch<ApproveRegistrationResponse>(
      `/admin/tenant-registrations/${id}/approve`,
      {
        method: "POST",
        body: JSON.stringify(body),
      },
      { auth: true }
    );

    // Invalidate the registration list so the pending badge updates
    revalidateTag("tenant-registrations");
    revalidateTag("tenants");

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.code === "TENANT_SLUG_TAKEN") {
        return { ok: false, error: "Slug tenant sudah digunakan oleh tenant lain." };
      }
      if (err.code === "REGISTRATION_NOT_PENDING") {
        return { ok: false, error: "Registrasi ini sudah diproses sebelumnya." };
      }
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Reject ─────────────────────────────────────────────────────────────────

const rejectSchema = z.object({
  id: z.string().uuid(),
  reason: z.string().min(1, "Alasan penolakan wajib diisi.").max(1000),
});

export type RejectResult =
  | { ok: true; data: TenantRegistration }
  | { ok: false; error: string };

export async function rejectRegistration(
  formData: FormData
): Promise<RejectResult> {
  const parsed = rejectSchema.safeParse({
    id: formData.get("id"),
    reason: formData.get("reason"),
  });

  if (!parsed.success) {
    const message =
      parsed.error.errors[0]?.message ?? "Input tidak valid.";
    return { ok: false, error: message };
  }

  try {
    const data = await apiFetch<TenantRegistration>(
      `/admin/tenant-registrations/${parsed.data.id}/reject`,
      {
        method: "POST",
        body: JSON.stringify({ reason: parsed.data.reason }),
      },
      { auth: true }
    );

    revalidateTag("tenant-registrations");

    // Redirect to the list after successful rejection
    redirect("/tenants/registrations");

    return { ok: true, data };
  } catch (err) {
    // redirect() throws — re-throw it so Next.js can handle the navigation
    if (err instanceof Error && err.message === "NEXT_REDIRECT") throw err;

    if (err instanceof ApiError) {
      if (err.code === "REGISTRATION_NOT_PENDING") {
        return { ok: false, error: "Registrasi ini sudah diproses sebelumnya." };
      }
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}
