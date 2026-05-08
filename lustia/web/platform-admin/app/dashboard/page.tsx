import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import {
  AlertTriangle,
  Banknote,
  Building2,
  ClipboardList,
  RefreshCw,
  TrendingUp,
} from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { getAccessToken } from "@/lib/session";
import { decodeJWTPayload } from "@/lib/jwt";
import { formatRupiah, formatLongDate, todayJakarta } from "@/lib/format";
import type {
  TenantListResponse,
  RegistrationListResponse,
  AdminDisbursementListResponse,
  SettlementBatchSummary,
} from "@/lib/types";
import { ConsoleHeader } from "@/components/console-header";
import { KpiCard } from "./_components/kpi-card";
import { QuickActionCard } from "./_components/quick-action-card";
import { RecentRegistrationsPanel } from "./_components/recent-registrations-panel";
import { RecentDisbursementsPanel } from "./_components/recent-disbursements-panel";

export const metadata: Metadata = {
  title: "Dasbor",
};

interface UserProfile {
  id: string;
  email: string;
  full_name: string;
  phone?: string;
  avatar_url?: string;
  is_active: boolean;
  is_super_admin?: boolean;
}

/** ADR 0007 §2.3 — /auth/me response */
interface MeResponse {
  user: UserProfile;
  active_membership_id: string | null;
  memberships: Array<{
    tenant_id: string;
    tenant_name: string;
    tenant_slug: string;
    roles: string[];
    branches: string[];
    status: string;
  }>;
  tenant?: {
    id: string;
    name: string;
    slug: string;
    status: string;
  };
}

/**
 * Platform-Admin Dashboard — Server Component.
 *
 * All data fetches run in a single Promise.allSettled so no individual
 * section failure blocks the rest of the page.  The /auth/me result is
 * handled first: 401 → redirect, other errors → throw.
 *
 * Fetch map:
 *   [0] /auth/me                                              — identity + redirect guard
 *   [1] /admin/tenants?status=active&limit=1                  — KPI 1 (active tenant count)
 *   [2] /admin/tenant-registrations?status=pending&limit=5    — KPI 2 + left panel rows
 *   [3] /admin/disbursements?status=pending&limit=1           — KPI 3 (pending disburse count)
 *   [4] /admin/settlement-batches/summary?from=…&to=…         — KPI 4 (weekly volume)
 *   [5] /admin/disbursements?limit=5                          — right panel rows (any status)
 *   [6] /admin/disbursements?status=failed&limit=1            — nav badge (failed disburse count)
 */
export default async function DashboardPage() {
  // ── Date helpers (Asia/Jakarta) ─────────────────────────────────────────────
  // "to" = today in Jakarta; "from" = today − 6 days (7-day inclusive window).
  const toDate = todayJakarta(); // "YYYY-MM-DD"
  const fromDate = (() => {
    const d = new Date(`${toDate}T00:00:00+07:00`);
    d.setDate(d.getDate() - 6);
    return new Intl.DateTimeFormat("sv-SE", {
      timeZone: "Asia/Jakarta",
    }).format(d);
  })();

  // ── Parallel fetch ──────────────────────────────────────────────────────────
  const [
    meRes,
    tenantCountRes,
    pendingRegRes,
    pendingDisbRes,
    volumeRes,
    recentDisbRes,
    failedDisbRes,
  ] = await Promise.allSettled([
    apiFetch<MeResponse>("/auth/me", {}, { auth: true }),
    apiFetch<TenantListResponse>(
      "/admin/tenants?status=active&limit=1&page=1",
      {},
      { auth: true }
    ),
    apiFetch<RegistrationListResponse>(
      "/admin/tenant-registrations?status=pending&limit=5&page=1",
      {},
      { auth: true }
    ),
    apiFetch<AdminDisbursementListResponse>(
      "/admin/disbursements?status=pending&limit=1&page=1",
      {},
      { auth: true }
    ),
    apiFetch<SettlementBatchSummary>(
      `/admin/settlement-batches/summary?from=${fromDate}&to=${toDate}`,
      {},
      { auth: true }
    ),
    apiFetch<AdminDisbursementListResponse>(
      "/admin/disbursements?limit=5&page=1",
      {},
      { auth: true }
    ),
    apiFetch<AdminDisbursementListResponse>(
      "/admin/disbursements?status=failed&limit=1&page=1",
      {},
      { auth: true }
    ),
  ]);

  // ── Auth guard — /auth/me must succeed ─────────────────────────────────────
  if (meRes.status === "rejected") {
    const err = meRes.reason;
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const { user } = meRes.value;

  // ── must_change_password ────────────────────────────────────────────────────
  let mustChange = false;
  try {
    const token = await getAccessToken();
    if (token) {
      const claims = decodeJWTPayload<{ must_change_password?: boolean }>(token);
      mustChange = claims.must_change_password === true;
    }
  } catch {
    // Non-fatal: banner won't show if JWT decoding fails
  }

  // ── Derive KPI values ────────────────────────────────────────────────────────

  // KPI 1 — Tenant Aktif
  const activeTenantCount =
    tenantCountRes.status === "fulfilled"
      ? tenantCountRes.value.total_count
      : null;

  // KPI 2 — Pendaftaran Pending (fix: use total_count, not data.length)
  const pendingRegCount =
    pendingRegRes.status === "fulfilled"
      ? pendingRegRes.value.total_count
      : null;

  // KPI 3 — Disbursement Pending
  // Note: AdminDisbursementListResponse uses "total" (not total_count) — per types.ts
  const pendingDisbCount =
    pendingDisbRes.status === "fulfilled"
      ? pendingDisbRes.value.total
      : null;

  // KPI 4 — Volume Disetel Minggu Ini
  const settlementVolume =
    volumeRes.status === "fulfilled"
      ? volumeRes.value.total_volume_idr
      : null;

  // Left panel
  const recentRegistrations =
    pendingRegRes.status === "fulfilled"
      ? pendingRegRes.value.data
      : null;

  // Right panel
  const recentDisbursements =
    recentDisbRes.status === "fulfilled"
      ? recentDisbRes.value.data
      : null;

  // Badge count for NavLinks — uses the corrected total_count (bug fix §5.1)
  const navPendingCount = pendingRegCount ?? 0;

  // Nav badge count for failed disbursements — AdminDisbursementListResponse uses "total"
  // On fetch error, default to 0 so the badge is absent (spec §11.4)
  const navFailedDisbCount =
    failedDisbRes.status === "fulfilled" ? failedDisbRes.value.total : 0;

  // ── Greeting date ────────────────────────────────────────────────────────────
  const longDate = formatLongDate();
  const firstName = user.full_name.split(" ")[0];

  // ── Volume display helpers ───────────────────────────────────────────────────
  const volumeFormatted =
    settlementVolume !== null ? formatRupiah(settlementVolume) : null;
  // Guard against long Rupiah strings (> 12 chars) overflowing the card
  const volumeValueClass =
    volumeFormatted && volumeFormatted.length > 12
      ? "text-2xl font-bold tabular-nums"
      : "text-3xl font-bold tabular-nums";

  return (
    <div className="min-h-screen bg-slate-50">
      {/* ── Header ──────────────────────────────────────────────────────────── */}
      <ConsoleHeader user={user} pendingRegistrationCount={navPendingCount} failedDisbursementCount={navFailedDisbCount} />

      <main className="mx-auto max-w-5xl space-y-6 px-6 py-8">
        {/* ── must_change_password banner (preserved as-is) ─────────────────── */}
        {mustChange && (
          <div
            role="alert"
            className="flex items-start gap-3 rounded-lg border border-yellow-300 bg-yellow-50 p-4"
          >
            <AlertTriangle
              size={20}
              className="mt-0.5 shrink-0 text-yellow-600"
              aria-hidden="true"
            />
            <div>
              <p className="font-medium text-yellow-800">
                Perubahan kata sandi diperlukan
              </p>
              <p className="mt-1 text-sm text-yellow-700">
                Akun Anda memerlukan perubahan kata sandi sebelum dapat mengakses fitur
                lain.{" "}
                <Link
                  href="/pengaturan/ubah-kata-sandi?reason=required"
                  className="font-medium underline underline-offset-4 text-yellow-800 hover:text-yellow-900"
                >
                  Ubah kata sandi
                </Link>
              </p>
            </div>
          </div>
        )}

        {/* ── Greeting strip ──────────────────────────────────────────────────── */}
        <section aria-label="Sapaan">
          <h1 className="text-2xl font-semibold text-foreground">
            Selamat datang, {firstName}!
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{longDate}</p>
        </section>

        {/* ── KPI grid ─────────────────────────────────────────────────────────── */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {/* KPI 1 — Tenant Aktif */}
          <KpiCard
            label="Tenant Aktif"
            value={activeTenantCount !== null ? String(activeTenantCount) : "—"}
            subLabel="tenant terdaftar aktif"
            icon={Building2}
            iconBg="bg-primary/10"
            iconColor="text-primary"
            href="/tenants?status=active"
            error={tenantCountRes.status === "rejected"}
          />

          {/* KPI 2 — Pendaftaran Pending (amber accent when count > 0) */}
          <KpiCard
            label="Pendaftaran Pending"
            value={pendingRegCount !== null ? String(pendingRegCount) : "—"}
            subLabel="menunggu persetujuan"
            icon={ClipboardList}
            iconBg={
              pendingRegCount !== null && pendingRegCount > 0
                ? "bg-amber-100"
                : "bg-primary/10"
            }
            iconColor={
              pendingRegCount !== null && pendingRegCount > 0
                ? "text-amber-600"
                : "text-primary"
            }
            valueClassName={
              pendingRegCount !== null && pendingRegCount > 0
                ? "text-amber-700 font-bold"
                : "text-foreground"
            }
            href="/tenants/registrations?status=pending"
            error={pendingRegRes.status === "rejected"}
          />

          {/* KPI 3 — Disbursement Pending */}
          <KpiCard
            label="Disbursement Pending"
            value={pendingDisbCount !== null ? String(pendingDisbCount) : "—"}
            subLabel="pencairan menunggu"
            icon={Banknote}
            iconBg="bg-primary/10"
            iconColor="text-primary"
            href="/payout/disbursements?status=pending"
            error={pendingDisbRes.status === "rejected"}
          />

          {/* KPI 4 — Volume Disetel Minggu Ini */}
          <KpiCard
            label="Volume Disetel"
            value={
              volumeFormatted ??
              (volumeRes.status === "rejected" ? "—" : "Rp 0")
            }
            subLabel={
              settlementVolume === 0
                ? "tidak ada settlement minggu ini"
                : "7 hari terakhir"
            }
            icon={TrendingUp}
            iconBg="bg-emerald-100"
            iconColor="text-emerald-600"
            valueClassName={
              settlementVolume === 0
                ? `${volumeValueClass} text-muted-foreground`
                : volumeValueClass
            }
            href="/payout/reconciliation"
            error={volumeRes.status === "rejected"}
          />
        </div>

        {/* ── Aksi Cepat ─────────────────────────────────────────────────────── */}
        <div>
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
            Aksi Cepat
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <QuickActionCard
              title="Review Pendaftaran"
              description="Tinjau dan setujui atau tolak permohonan registrasi tenant baru."
              icon={ClipboardList}
              href="/tenants/registrations?status=pending"
            />
            <QuickActionCard
              title="Reconciliation Settlement"
              description="Tarik laporan settlement harian dari iPaymu dan cocokkan transaksi."
              icon={RefreshCw}
              href="/payout/reconciliation"
            />
            <QuickActionCard
              title="Buat Disbursement"
              description="Cairkan saldo tenant yang siap dibayar ke rekening mitra."
              icon={Banknote}
              href="/payout/tenant-payout"
            />
          </div>
        </div>

        {/* ── Two-panel row ────────────────────────────────────────────────────── */}
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
          <RecentRegistrationsPanel
            registrations={recentRegistrations}
            error={pendingRegRes.status === "rejected"}
          />
          <RecentDisbursementsPanel
            disbursements={recentDisbursements}
            error={recentDisbRes.status === "rejected"}
          />
        </div>

      </main>
    </div>
  );
}
