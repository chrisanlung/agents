import type { Metadata } from "next";
import Link from "next/link";
import { Banknote, AlertCircle } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { formatDate } from "@/lib/format";
import type { TenantPayoutSummaryResponse } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { DateRangePicker } from "@/components/date-range-picker";
import { TenantPayoutCard } from "./tenant-payout-card";

export const metadata: Metadata = {
  title: "Pencairan Tenant",
};

/** Returns Monday–Sunday of the current ISO week as YYYY-MM-DD strings. */
function currentISOWeek(): { start: string; end: string } {
  const now = new Date();
  const day = now.getDay(); // 0=Sun..6=Sat
  const diffToMonday = day === 0 ? -6 : 1 - day; // ISO week starts Monday
  const monday = new Date(now);
  monday.setDate(now.getDate() + diffToMonday);
  const sunday = new Date(monday);
  sunday.setDate(monday.getDate() + 6);
  return {
    start: monday.toISOString().slice(0, 10),
    end: sunday.toISOString().slice(0, 10),
  };
}

interface PageProps {
  searchParams: Promise<{ period_start?: string; period_end?: string }>;
}

export default async function TenantPayoutPage({ searchParams }: PageProps) {
  const { period_start, period_end } = await searchParams;

  const defaults = currentISOWeek();
  const periodStart = period_start ?? defaults.start;
  const periodEnd = period_end ?? defaults.end;

  let summaries: TenantPayoutSummaryResponse["data"] = [];
  let fetchError = false;

  try {
    const res = await apiFetch<TenantPayoutSummaryResponse>(
      `/admin/payout/tenant-summary?period_start=${periodStart}&period_end=${periodEnd}`,
      {},
      { auth: true }
    );
    summaries = res.data ?? [];
  } catch (err) {
    await handleApiError(err);
    fetchError = true;
  }

  const hasBalance = summaries.some((t) => t.settled_amount_idr > 0);

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
            Pencairan Tenant
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Buat pencairan untuk tenant yang memiliki saldo siap dicairkan.
          </p>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span className="text-muted-foreground">Periode:</span>
          <DateRangePicker
            fromParam="period_start"
            toParam="period_end"
            fromLabel="Dari"
            toLabel="Sampai"
          />
        </div>
      </div>

      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle
            size={16}
            className="mt-0.5 shrink-0 text-red-600"
            aria-hidden="true"
          />
          <span>Gagal memuat data payout tenant. Muat ulang halaman.</span>
        </div>
      )}

      {/* Period summary */}
      {!fetchError && (
        <p className="text-xs text-muted-foreground">
          Periode: {formatDate(periodStart)} – {formatDate(periodEnd)}
        </p>
      )}

      {/* Tenant card grid */}
      {!fetchError && summaries.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-3 py-24 text-center">
          <Banknote
            size={40}
            className="text-muted-foreground/40"
            aria-hidden="true"
          />
          <p className="text-sm font-medium text-muted-foreground">
            Tidak ada saldo yang siap dicairkan.
          </p>
          <p className="text-xs text-muted-foreground max-w-xs">
            Jalankan rekonsiliasi iPaymu untuk mengisi saldo tenant.
          </p>
          <Button variant="outline" size="sm" asChild>
            <Link href="/payout/reconciliation">Ke Rekonsiliasi</Link>
          </Button>
        </div>
      ) : !fetchError && !hasBalance ? (
        <div className="flex flex-col items-center justify-center gap-3 py-24 text-center">
          <Banknote
            size={40}
            className="text-muted-foreground/40"
            aria-hidden="true"
          />
          <p className="text-sm font-medium text-muted-foreground">
            Tidak ada saldo yang siap dicairkan.
          </p>
          <p className="text-xs text-muted-foreground max-w-xs">
            Jalankan rekonsiliasi iPaymu untuk mengisi saldo tenant.
          </p>
          <Button variant="outline" size="sm" asChild>
            <Link href="/payout/reconciliation">Ke Rekonsiliasi</Link>
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {summaries.map((tenant) => (
            <TenantPayoutCard
              key={tenant.tenant_id}
              tenant={tenant}
              periodStart={periodStart}
              periodEnd={periodEnd}
            />
          ))}
        </div>
      )}
    </div>
  );
}
