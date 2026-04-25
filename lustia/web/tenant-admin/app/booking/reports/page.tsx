import type { Metadata } from "next";
import { AlertCircle } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { getAccessToken } from "@/lib/session";
import { decodeJWTPayload } from "@/lib/jwt";
import type {
  BookingReportSummary,
  BranchListResponse,
  Branch,
} from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { FilterBar } from "@/components/filter-bar";
import { FilterSelect } from "@/components/filter-select";
import { DateRangePicker } from "@/components/date-range-picker";
import { BookingBarChart } from "./booking-bar-chart";

export const metadata: Metadata = {
  title: "Laporan Booking",
};

// Default: this month
function thisMonthRange() {
  const now = new Date();
  const from = new Date(now.getFullYear(), now.getMonth(), 1)
    .toISOString()
    .slice(0, 10);
  const to = new Date(now.getFullYear(), now.getMonth() + 1, 0)
    .toISOString()
    .slice(0, 10);
  return { from, to };
}

interface PageProps {
  searchParams: Promise<{
    from?: string;
    to?: string;
    branch_id?: string;
  }>;
}

function formatPrice(idr: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(idr);
}

function formatPct(rate: number) {
  return `${(rate * 100).toFixed(1)}%`;
}

export default async function ReportsPage({ searchParams }: PageProps) {
  const { from: fromParam, to: toParam, branch_id } = await searchParams;

  const defaults = thisMonthRange();
  const from = fromParam ?? defaults.from;
  const to = toParam ?? defaults.to;

  let isBranchAdmin = false;
  try {
    const token = await getAccessToken();
    if (token) {
      const claims = decodeJWTPayload<{ roles?: string[] }>(token);
      isBranchAdmin = claims.roles?.includes("branch_admin") === true;
    }
  } catch {
    // non-fatal
  }

  let summary: BookingReportSummary | null = null;
  let branches: Branch[] = [];
  let fetchError = false;

  const params = new URLSearchParams({ from, to });
  if (branch_id) params.set("branch_id", branch_id);

  try {
    const [summaryRes, branchRes] = await Promise.allSettled([
      apiFetch<BookingReportSummary>(
        `/tenant/reports/bookings/summary?${params.toString()}`,
        {},
        { auth: true }
      ),
      apiFetch<BranchListResponse>(
        "/tenant/branches?limit=200",
        {},
        { auth: true }
      ),
    ]);

    if (summaryRes.status === "fulfilled") {
      summary = summaryRes.value;
    } else {
      const err = summaryRes.reason;
      if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
        await handleApiError(err);
      }
      fetchError = true;
    }

    if (branchRes.status === "fulfilled") branches = branchRes.value.data;
  } catch (err) {
    await handleApiError(err);
  }

  const filterActive = !!(fromParam || toParam || branch_id);
  const noShowRate = summary?.no_show_rate ?? 0;
  const isHighNoShow = noShowRate > 0.1;

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
          Laporan Booking
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Ringkasan performa booking
        </p>
      </div>

      {/* Filter */}
      <FilterBar isActive={filterActive} resetHref="/booking/reports">
        <DateRangePicker fromParam="from" toParam="to" fromLabel="Dari" toLabel="Sampai" />
        {!isBranchAdmin && branches.length > 0 && (
          <FilterSelect
            label="Cabang"
            name="branch_id"
            current={branch_id}
            options={[
              { value: "", label: "Semua" },
              ...branches.map((b) => ({ value: b.id, label: b.name })),
            ]}
          />
        )}
      </FilterBar>

      {/* Error state */}
      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle
            size={16}
            className="mt-0.5 shrink-0 text-red-600"
            aria-hidden="true"
          />
          <span>Gagal memuat laporan. Muat ulang halaman.</span>
        </div>
      )}

      {/* Metric cards */}
      {!fetchError && (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <MetricCard
            title="Total Booking"
            value={String(summary?.total_bookings ?? 0)}
            subtitle="periode ini"
            valueClassName="text-primary"
          />
          <MetricCard
            title="Total Pendapatan"
            value={formatPrice(summary?.total_revenue_idr ?? 0)}
            subtitle="dari booking terbayar"
            valueClassName="text-emerald-600"
          />
          <MetricCard
            title="Tingkat No-show"
            value={summary ? formatPct(noShowRate) : "—"}
            subtitle="dari booking terbayar"
            valueClassName={
              isHighNoShow ? "text-amber-600" : "text-emerald-600"
            }
          />
          <MetricCard
            title="Booking Dibatalkan"
            value={String(summary?.cancelled_count ?? 0)}
            subtitle="oleh operator"
            valueClassName="text-foreground"
          />
        </div>
      )}

      {/* Bar chart */}
      {!fetchError && summary && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Booking per Cabang</CardTitle>
          </CardHeader>
          <CardContent>
            {summary.by_branch.length === 0 ? (
              <div className="flex h-60 items-center justify-center text-sm text-muted-foreground">
                Tidak ada data booking pada periode ini.
              </div>
            ) : (
              <BookingBarChart
                data={summary.by_branch}
                from={from}
                to={to}
              />
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function MetricCard({
  title,
  value,
  subtitle,
  valueClassName,
}: {
  title: string;
  value: string;
  subtitle: string;
  valueClassName?: string;
}) {
  return (
    <Card>
      <CardContent className="pt-4">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {title}
        </p>
        <p className={`mt-1 text-2xl font-bold tabular-nums ${valueClassName ?? ""}`}>
          {value}
        </p>
        <p className="mt-0.5 text-xs text-muted-foreground">{subtitle}</p>
      </CardContent>
    </Card>
  );
}
