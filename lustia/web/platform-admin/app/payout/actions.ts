"use server";

import { revalidatePath } from "next/cache";
import { apiFetch } from "@/lib/api";
import type {
  ReconcileResponse,
  AdminDisbursementDetail,
  CreateDisbursementInput,
  SettlementBatchDetail,
} from "@/lib/types";

// ─── Settlement batch lazy-fetch (from BatchDetailDialog) ────────────────────

export async function fetchSettlementBatchDetail(
  id: string
): Promise<{ ok: true; data: SettlementBatchDetail } | { ok: false }> {
  try {
    const data = await apiFetch<SettlementBatchDetail>(
      `/admin/settlement-batches/${id}`,
      {},
      { auth: true }
    );
    return { ok: true, data };
  } catch {
    return { ok: false };
  }
}

// ─── Reconciliation ───────────────────────────────────────────────────────────

export async function reconcileAction(date: string): Promise<
  | { ok: true; data: ReconcileResponse }
  | { ok: false; message: string }
> {
  try {
    const data = await apiFetch<ReconcileResponse>(
      "/admin/settlement/reconcile",
      {
        method: "POST",
        body: JSON.stringify({ date }),
      },
      { auth: true }
    );
    revalidatePath("/payout/reconciliation");
    return { ok: true, data };
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "Gagal menarik laporan iPaymu.";
    return { ok: false, message };
  }
}

// ─── Disbursement creation ────────────────────────────────────────────────────

export async function createDisbursementAction(
  input: CreateDisbursementInput
): Promise<
  | { ok: true; data: AdminDisbursementDetail }
  | { ok: false; message: string }
> {
  try {
    const data = await apiFetch<AdminDisbursementDetail>(
      "/admin/disbursements",
      {
        method: "POST",
        body: JSON.stringify(input),
      },
      { auth: true }
    );
    revalidatePath("/payout/tenant-payout");
    revalidatePath("/payout/disbursements");
    return { ok: true, data };
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "Gagal membuat pencairan.";
    return { ok: false, message };
  }
}

// ─── Disbursement status transitions ─────────────────────────────────────────

export async function startProcessingAction(id: string): Promise<
  | { ok: true }
  | { ok: false; message: string }
> {
  try {
    await apiFetch(
      `/admin/disbursements/${id}/processing`,
      { method: "POST" },
      { auth: true }
    );
    revalidatePath(`/payout/disbursements/${id}`);
    revalidatePath("/payout/disbursements");
    return { ok: true };
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "Gagal memperbarui status.";
    return { ok: false, message };
  }
}

export async function markTransferredAction(
  id: string,
  bankReference?: string,
  notes?: string
): Promise<{ ok: true } | { ok: false; message: string }> {
  try {
    await apiFetch(
      `/admin/disbursements/${id}/transferred`,
      {
        method: "POST",
        body: JSON.stringify({ bank_reference: bankReference, notes }),
      },
      { auth: true }
    );
    revalidatePath(`/payout/disbursements/${id}`);
    revalidatePath("/payout/disbursements");
    return { ok: true };
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "Gagal memperbarui status.";
    return { ok: false, message };
  }
}

export async function markFailedAction(
  id: string,
  reason: string
): Promise<{ ok: true } | { ok: false; message: string }> {
  try {
    await apiFetch(
      `/admin/disbursements/${id}/failed`,
      {
        method: "POST",
        body: JSON.stringify({ reason }),
      },
      { auth: true }
    );
    revalidatePath(`/payout/disbursements/${id}`);
    revalidatePath("/payout/disbursements");
    return { ok: true };
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "Gagal memperbarui status.";
    return { ok: false, message };
  }
}

export async function cancelDisbursementAction(id: string): Promise<
  | { ok: true }
  | { ok: false; message: string }
> {
  try {
    await apiFetch(
      `/admin/disbursements/${id}/cancel`,
      { method: "POST" },
      { auth: true }
    );
    revalidatePath(`/payout/disbursements/${id}`);
    revalidatePath("/payout/disbursements");
    return { ok: true };
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "Gagal membatalkan pencairan.";
    return { ok: false, message };
  }
}
