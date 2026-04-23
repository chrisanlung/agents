import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import type { Metadata } from "next";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowRight,
  CalendarDays,
  LineChart,
  MapPin,
  Plus,
  Users,
  Wallet,
} from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { getAccessToken } from "@/lib/session";
import { decodeJWTPayload } from "@/lib/jwt";
import type { OnboardingState } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { type MembershipSummary } from "@/components/workspace-switcher";
import { AppBackground } from "@/components/app-background";
import { AppHeader } from "@/components/app-header";
import { cn } from "@/lib/utils";

const SKIP_ONBOARDING_COOKIE = "lustia_skipped_onboarding";

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

interface MeResponse {
  user: UserProfile;
  active_membership_id: string | null;
  memberships: MembershipSummary[];
  tenant?: {
    id: string;
    name: string;
    slug: string;
    status: string;
  };
}

const STATUS_BADGE: Record<
  string,
  { label: string; variant: "success" | "warning" | "muted" }
> = {
  active: { label: "Aktif", variant: "success" },
  suspended: { label: "Disuspend", variant: "warning" },
  deactivated: { label: "Dinonaktifkan", variant: "muted" },
};

export default async function DashboardPage() {
  let data: MeResponse;

  try {
    data = await apiFetch<MeResponse>("/auth/me", {}, { auth: true });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const { user, tenant, memberships, active_membership_id } = data;

  const activeMembership =
    memberships.find((m) => m.membership_id === active_membership_id) ??
    memberships[0];
  const activeRoles = activeMembership?.roles ?? [];

  // Decode JWT for must_change_password
  let mustChange = false;
  try {
    const token = await getAccessToken();
    if (token) {
      const claims = decodeJWTPayload<{ must_change_password?: boolean }>(token);
      mustChange = claims.must_change_password === true;
    }
  } catch {
    // Non-fatal
  }

  // Fetch onboarding state (for KPI + limit)
  let onboarding: OnboardingState | null = null;
  try {
    onboarding = await apiFetch<OnboardingState>(
      "/tenant/onboarding-state",
      {},
      { auth: true }
    );
  } catch {
    // Non-fatal — renders KPI without counts
  }

  // Onboarding redirect
  if (!mustChange && activeRoles.includes("tenant_admin") && tenant && onboarding) {
    const skippedCookie = (await cookies()).get(SKIP_ONBOARDING_COOKIE);
    if (!skippedCookie && !onboarding.has_branches) {
      redirect("/onboarding/welcome");
    }
  }

  const activeBranchCount = onboarding?.active_branch_count ?? 0;
  const maxBranches = onboarding?.max_branches ?? null;
  const branchAtLimit =
    maxBranches !== null && maxBranches !== 999 && activeBranchCount >= maxBranches;
  const branchLimitLabel =
    maxBranches === null ? "—" : maxBranches === 999 ? "∞" : String(maxBranches);
  const branchBadge = STATUS_BADGE[tenant?.status ?? ""] ?? null;

  return (
    <AppBackground>
      <AppHeader
        fullName={user.full_name}
        email={user.email}
        activeNav="dashboard"
        tenant={tenant ? { name: tenant.name, slug: tenant.slug } : null}
        memberships={memberships}
      />

      <main className="mx-auto max-w-5xl space-y-6 px-6 py-8">
        {/* Password change alert — stays as-is */}
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
                Akun Anda memerlukan perubahan kata sandi sebelum dapat mengakses
                fitur lain.{" "}
                <Link
                  href="/pengaturan/ubah-kata-sandi?reason=required"
                  className="font-medium text-yellow-900 underline underline-offset-4 hover:text-yellow-950"
                >
                  Ubah kata sandi
                </Link>
              </p>
            </div>
          </div>
        )}

        {/* Greeting strip */}
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight text-foreground">
              Selamat datang, {firstName(user.full_name)}
            </h1>
            {tenant && (
              <p className="mt-1 text-sm text-muted-foreground">
                Mengelola <span className="font-medium">{tenant.name}</span>
              </p>
            )}
          </div>
          {branchBadge && (
            <Badge variant={branchBadge.variant}>{branchBadge.label}</Badge>
          )}
        </div>

        {/* KPI strip */}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <KPITile
            icon={<MapPin size={18} aria-hidden="true" />}
            label="Cabang Aktif"
            value={`${activeBranchCount} / ${branchLimitLabel}`}
            tone={branchAtLimit ? "warning" : "default"}
          />
          <KPITile
            icon={<CalendarDays size={18} aria-hidden="true" />}
            label="Booking Hari Ini"
            value="—"
            comingSoon
          />
          <KPITile
            icon={<Users size={18} aria-hidden="true" />}
            label="Terapis Aktif"
            value="—"
            comingSoon
          />
          <KPITile
            icon={<Wallet size={18} aria-hidden="true" />}
            label="Pendapatan Bulan Ini"
            value="—"
            comingSoon
          />
        </div>

        {/* Quick actions */}
        <section aria-labelledby="quick-actions-title" className="space-y-3">
          <h2
            id="quick-actions-title"
            className="text-sm font-semibold uppercase tracking-wide text-muted-foreground"
          >
            Aksi Cepat
          </h2>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <QuickActionCard
              icon={<MapPin size={20} aria-hidden="true" />}
              title="Kelola Cabang"
              description="Lihat daftar cabang, atur status, dan perbarui kontak."
              href="/branches"
            />
            <QuickActionCard
              icon={<Plus size={20} aria-hidden="true" />}
              title={branchAtLimit ? "Batas Cabang Tercapai" : "Tambah Cabang"}
              description={
                branchAtLimit
                  ? `Anda sudah mencapai batas ${maxBranches} cabang. Tingkatkan paket untuk menambah cabang.`
                  : "Daftarkan lokasi cabang baru untuk operasional Anda."
              }
              href={branchAtLimit ? undefined : "/branches/new"}
            />
            <QuickActionCard
              icon={<Users size={20} aria-hidden="true" />}
              title="Terapis & Jadwal"
              description="Kelola data terapis dan jadwal kerja."
              comingSoon
            />
            <QuickActionCard
              icon={<CalendarDays size={20} aria-hidden="true" />}
              title="Booking"
              description="Pantau dan kelola booking dari pelanggan."
              comingSoon
            />
            <QuickActionCard
              icon={<LineChart size={20} aria-hidden="true" />}
              title="Laporan"
              description="Insight pendapatan, utilisasi, dan performa cabang."
              comingSoon
            />
          </div>
        </section>

        {/* Activity feed placeholder */}
        <section aria-labelledby="activity-title" className="space-y-3">
          <h2
            id="activity-title"
            className="text-sm font-semibold uppercase tracking-wide text-muted-foreground"
          >
            Aktivitas Terbaru
          </h2>
          <Card className="border-dashed bg-white/70 backdrop-blur-sm">
            <CardContent className="flex flex-col items-center justify-center gap-2 py-10 text-center">
              <p className="text-sm text-muted-foreground">
                Belum ada aktivitas. Aktivitas cabang, booking, dan staf akan
                muncul di sini.
              </p>
            </CardContent>
          </Card>
        </section>
      </main>
    </AppBackground>
  );
}

function firstName(fullName: string): string {
  return fullName.trim().split(/\s+/)[0] || fullName;
}

interface KPITileProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  tone?: "default" | "warning";
  comingSoon?: boolean;
}

function KPITile({ icon, label, value, tone = "default", comingSoon }: KPITileProps) {
  return (
    <div
      className={cn(
        "flex items-center gap-3 rounded-xl border bg-white/90 p-4 shadow-sm backdrop-blur-sm transition",
        tone === "warning" && "border-amber-300 bg-amber-50/90",
        comingSoon && "opacity-70"
      )}
      title={comingSoon ? "Segera hadir" : undefined}
    >
      <div
        className={cn(
          "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
          tone === "warning"
            ? "bg-amber-100 text-amber-700"
            : "bg-pink-100 text-primary"
        )}
      >
        {icon}
      </div>
      <div className="min-w-0">
        <p className="truncate text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {label}
        </p>
        <p
          className={cn(
            "mt-0.5 truncate text-xl font-semibold",
            tone === "warning" ? "text-amber-900" : "text-foreground"
          )}
        >
          {value}
        </p>
      </div>
    </div>
  );
}

interface QuickActionCardProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  href?: string;
  comingSoon?: boolean;
}

function QuickActionCard({
  icon,
  title,
  description,
  href,
  comingSoon,
}: QuickActionCardProps) {
  const disabled = !href || comingSoon;
  const content = (
    <div
      className={cn(
        "group flex h-full flex-col gap-3 rounded-xl border bg-white/90 p-4 shadow-sm backdrop-blur-sm transition",
        disabled
          ? "cursor-not-allowed opacity-70"
          : "cursor-pointer hover:-translate-y-0.5 hover:border-pink-200 hover:shadow-md"
      )}
    >
      <div className="flex items-center justify-between">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-pink-100 text-primary">
          {icon}
        </div>
        {comingSoon && (
          <Badge variant="secondary" className="text-[10px]">
            Segera hadir
          </Badge>
        )}
        {!comingSoon && href && (
          <ArrowRight
            size={16}
            className="text-muted-foreground transition group-hover:translate-x-0.5 group-hover:text-primary"
            aria-hidden="true"
          />
        )}
      </div>
      <div>
        <p className="text-sm font-semibold text-foreground">{title}</p>
        <p className="mt-1 text-xs text-muted-foreground">{description}</p>
      </div>
    </div>
  );

  if (disabled) {
    return (
      <div
        title={comingSoon ? "Segera hadir" : undefined}
        aria-disabled="true"
      >
        {content}
      </div>
    );
  }

  return (
    <Link href={href!} className="block h-full">
      {content}
    </Link>
  );
}

