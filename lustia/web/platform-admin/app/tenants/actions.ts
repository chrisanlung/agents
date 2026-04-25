"use server";

import { revalidateTag } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { Tenant } from "@/lib/types";

const changeTenantStatusSchema = z.object({
  id: z.string().uuid(),
  status: z.enum(["active", "suspended", "deactivated"]),
  reason: z.string().max(500).optional(),
});

export type ChangeTenantStatusResult =
  | { ok: true; data: Tenant }
  | { ok: false; error: string };

export async function changeTenantStatus(
  formData: FormData
): Promise<ChangeTenantStatusResult> {
  const parsed = changeTenantStatusSchema.safeParse({
    id: formData.get("id"),
    status: formData.get("status"),
    reason: formData.get("reason") ?? undefined,
  });

  if (!parsed.success) {
    return { ok: false, error: "Input tidak valid." };
  }

  const { id, status, reason } = parsed.data;
  const body: Record<string, unknown> = { status };
  if (reason) body.reason = reason;

  try {
    const data = await apiFetch<Tenant>(
      `/admin/tenants/${id}/status`,
      {
        method: "PATCH",
        body: JSON.stringify(body),
      },
      { auth: true }
    );

    revalidateTag("tenants");

    return { ok: true, data };
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.code === "INVALID_STATUS_TRANSITION") {
        return {
          ok: false,
          error: "Transisi status tidak valid untuk tenant ini.",
        };
      }
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan. Coba lagi." };
  }
}
