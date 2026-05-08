"use server";

import { apiFetch } from "@/lib/api";
import type { DisbursementDetail } from "@/lib/types";

/**
 * Server action wrapper for the disbursement-detail dialog.
 *
 * The lazy-fetch dialog is a client component; it cannot import `@/lib/api`
 * directly because that module is `server-only` (uses cookies() from
 * next/headers). This action proxies the call through the server runtime so
 * the cookie-based auth still works.
 */
export async function fetchDisbursementDetail(
  id: string
): Promise<{ ok: true; data: DisbursementDetail } | { ok: false; error: string }> {
  try {
    const data = await apiFetch<DisbursementDetail>(
      `/tenant/finance/disbursements/${id}`,
      {},
      { auth: true }
    );
    return { ok: true, data };
  } catch (err) {
    const message = err instanceof Error ? err.message : "unknown";
    return { ok: false, error: message };
  }
}
