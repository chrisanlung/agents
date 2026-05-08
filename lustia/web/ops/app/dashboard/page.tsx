import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowRight,
  Calendar,
  CheckCircle2,
  ClockAlert,
  Plus,
  QrCode,
} from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { getAccessToken } from "@/lib/session";
import { decodeJWTPayload } from "@/lib/jwt";
import type { BookingListResponse, Booking } from "@/lib/types";
import type { MembershipSummary } from "@/components/workspace-switcher";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { OpsHeader } from "@/components/ops-header";
import { BookingStatusBadge } from "@/components/booking-status-badge";

export const metadata: Metadata = {
  title: "Dasbor",
};

// ─── API response shapes ──────────────────────────────────────────────────────

interface UserProfile {
  id: string;
  email: string;
  full_name: string;
  phone?: string;
  is_active: boolean;
  roles: string[];
  branches: string[];
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

// ─── Formatters ───────────────────────────────────────────────────────────────

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

// ─── KPI helpers ──────────────────────────────────────────────────────────────

/**
 * Aggregate today's booking KPIs from the list fetched for the dashboard.
 * We fetch one page (up to 200 per call) for today; if > 200 total, the
 * counts in each category may be capped — acceptable for a dashboard widget.
 *
 * Status mapping:
 *   "Booking Hari Ini" → all non-cancelled / non-expired statuses
 *   "Belum Check-in"   → status === "paid" (awaiting check-in)
 *   "Selesai Hari Ini" → status === "completed"
 */
function aggregateKPIs(bookings: Booking[], totalCount: number) {
  let belumCheckin = 0;
  let selesai = 0;

  for (const b of bookings) {
    if (b.status === "paid") belumCheckin++;
    if (b.status === "completed") selesai++;
  }

  return { total: totalCount, belumCheckin, selesai };
}

// ─── Dashboard page ───────────────────────────────────────────────────────────

export default async function DashboardPage() {
  // ── Fetch /auth/me + branches in parallel ─────────────────────────────────
  let data: MeResponse;
  let branchName: string | null = null;
  try {
    const [meRes, branchesRes] = await Promise.allSettled([
      apiFetch<MeResponse>("/auth/me", {}, { auth: true }),
      apiFetch<{ data: { id: string; name: string }[] }>(
        "/tenant/branches?scope=mine&status=active&limit=200",
        {},
        { auth: true }
      ),
    ]);

    if (meRes.status === "rejected") {
      if (meRes.reason instanceof ApiError && meRes.reason.status === 401) {
        redirect("/login");
      }
      throw meRes.reason;
    }
    data = meRes.value;

    if (branchesRes.status === "fulfilled") {
      const branches = branchesRes.value.data;
      if (branches.length === 1) {
        branchName = branches[0].name;
      } else if (branches.length > 1) {
        branchName = `${branches.length} Cabang`;
      }
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const { user, tenant, memberships } = data;

  // ── must_change_password claim ──────────────────────────────────────────────
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

  // ── Today's bookings (limit 200 for KPI aggregate + preview) ───────────────
  const today = new Date().toISOString().slice(0, 10);
  let todayBookings: Booking[] = [];
  let totalCount = 0;
  let bookingFetchError = false;

  try {
    const res = await apiFetch<BookingListResponse>(
      `/tenant/bookings?from=${today}&to=${today}&limit=200&page=1`,
      {},
      { auth: true }
    );
    todayBookings = res.data;
    totalCount = res.total_count;
  } catch {
    // Non-critical: dashboard still renders without KPI data
    bookingFetchError = true;
  }

  const kpis = aggregateKPIs(todayBookings, totalCount);

  // Up to 5 upcoming bookings for today preview (non-cancelled, non-expired,
  // sorted by scheduled_start ascending — backend already returns them sorted).
  const upcomingPreview = todayBookings
    .filter((b) => b.status !== "cancelled" && b.status !== "expired")
    .slice(0, 5);

  return (
    <div className="min-h-screen bg-amber-50">
      <OpsHeader
        fullName={user.full_name}
        email={user.email}
        tenant={tenant ?? null}
        branchName={branchName}
        memberships={memberships}
      />

      <main className="mx-auto max-w-5xl space-y-6 px-6 py-8">
        {/* ── Forced password-change banner ─────────────────────────────── */}
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
                Akun Anda memerlukan perubahan kata sandi sebelum dapat
                mengakses fitur lain.{" "}
                <a
                  href="/pengaturan/ubah-kata-sandi?reason=required"
                  className="font-medium underline underline-offset-4 hover:text-yellow-900"
                >
                  Ubah kata sandi
                </a>
              </p>
            </div>
          </div>
        )}

        {/* ── Welcome ───────────────────────────────────────────────────── */}
        <div>
          <h1 className="text-2xl font-semibold text-foreground">
            Selamat datang, {user.full_name.split(" ")[0]}!
          </h1>
          {user.branches && user.branches.length > 0 && (
            <p className="mt-1 text-sm text-muted-foreground">
              Cabang Anda:{" "}
              <span className="font-medium text-foreground">
                {user.branches.join(", ")}
              </span>
            </p>
          )}
        </div>

        {/* ── KPI cards ─────────────────────────────────────────────────── */}
        {!bookingFetchError && (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <KpiCard
              icon={<Calendar size={20} className="text-primary" aria-hidden="true" />}
              label="Booking Hari Ini"
              value={kpis.total}
              description="semua status aktif"
              iconBg="bg-primary/10"
            />
            <KpiCard
              icon={<ClockAlert size={20} className="text-amber-600" aria-hidden="true" />}
              label="Belum Check-in"
              value={kpis.belumCheckin}
              description="status terbayar"
              iconBg="bg-amber-100"
            />
            <KpiCard
              icon={<CheckCircle2 size={20} className="text-emerald-600" aria-hidden="true" />}
              label="Selesai Hari Ini"
              value={kpis.selesai}
              description="layanan selesai"
              iconBg="bg-emerald-100"
            />
          </div>
        )}

        {/* ── Aksi Cepat ────────────────────────────────────────────────── */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Card className="group border-dashed transition-shadow hover:shadow-md">
            <CardContent className="flex flex-col items-center justify-center gap-3 py-8 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
                <QrCode size={24} className="text-primary" aria-hidden="true" />
              </div>
              <div>
                <p className="font-semibold text-foreground">Scan QR Check-in</p>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Scan kode QR atau masukkan kode booking
                </p>
              </div>
              <Button asChild size="sm">
                <Link href="/booking/checkin">Mulai Check-in</Link>
              </Button>
            </CardContent>
          </Card>

          <Card className="group border-dashed transition-shadow hover:shadow-md">
            <CardContent className="flex flex-col items-center justify-center gap-3 py-8 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
                <Plus size={24} className="text-primary" aria-hidden="true" />
              </div>
              <div>
                <p className="font-semibold text-foreground">Booking Baru</p>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Buat booking atas nama pelanggan (concierge)
                </p>
              </div>
              <Button asChild size="sm" variant="outline">
                <Link href="/booking/new">Buat Booking</Link>
              </Button>
            </CardContent>
          </Card>
        </div>

        {/* ── Booking Hari Ini preview ──────────────────────────────────── */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
            <div>
              <CardTitle className="text-base">Booking Hari Ini</CardTitle>
              <CardDescription className="mt-0.5 text-xs">
                {new Date().toLocaleDateString("id-ID", {
                  weekday: "long",
                  day: "numeric",
                  month: "long",
                  year: "numeric",
                })}
              </CardDescription>
            </div>
            <Button variant="ghost" size="sm" asChild className="shrink-0">
              <Link href="/booking" className="flex items-center gap-1 text-sm">
                Lihat semua
                <ArrowRight size={14} aria-hidden="true" />
              </Link>
            </Button>
          </CardHeader>
          <CardContent className="pt-0">
            {bookingFetchError ? (
              <p className="py-4 text-center text-sm text-muted-foreground">
                Gagal memuat data booking.
              </p>
            ) : upcomingPreview.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-8 text-center">
                <Calendar
                  size={36}
                  className="text-muted-foreground/30"
                  aria-hidden="true"
                />
                <p className="text-sm text-muted-foreground">
                  Belum ada booking untuk hari ini.
                </p>
                <Button asChild size="sm" variant="outline">
                  <Link href="/booking/new">Booking Baru</Link>
                </Button>
              </div>
            ) : (
              <ul className="divide-y divide-border">
                {upcomingPreview.map((b) => (
                  <li key={b.id}>
                    <Link
                      href={`/booking/${b.id}`}
                      className="flex items-center gap-3 rounded-sm py-3 pr-2 transition-colors hover:bg-muted/40"
                    >
                      <span className="w-12 shrink-0 font-mono text-sm font-medium tabular-nums text-foreground">
                        {formatTime(b.scheduled_start)}
                      </span>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium text-foreground">
                          {b.customer_name}
                        </p>
                        <p className="truncate text-xs text-muted-foreground">
                          {b.service_name}
                          {b.room_name ? ` — ${b.room_name}` : ""}
                          {b.therapist_name ? ` · ${b.therapist_name}` : ""}
                        </p>
                      </div>
                      <BookingStatusBadge status={b.status} />
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* ── Profil Anda (compact) ─────────────────────────────────────── */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-base">Profil Anda</CardTitle>
            <Button variant="ghost" size="sm" asChild>
              <Link
                href="/pengaturan"
                className="text-xs text-muted-foreground hover:text-foreground"
              >
                Pengaturan
              </Link>
            </Button>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <ProfileField label="Nama Lengkap" value={user.full_name} />
              <ProfileField label="Email" value={user.email} />
              <ProfileField
                label="Peran"
                value={
                  user.roles && user.roles.length > 0
                    ? user.roles.join(", ")
                    : "—"
                }
              />
              {user.phone && (
                <ProfileField label="Telepon" value={user.phone} />
              )}
            </dl>
          </CardContent>
        </Card>
      </main>
    </div>
  );
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function KpiCard({
  icon,
  label,
  value,
  description,
  iconBg,
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
  description: string;
  iconBg: string;
}) {
  return (
    <Card>
      <CardContent className="flex items-center gap-4 py-5">
        <div
          className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-full ${iconBg}`}
        >
          {icon}
        </div>
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            {label}
          </p>
          <p className="mt-0.5 text-3xl font-bold tabular-nums text-foreground">
            {value}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
        </div>
      </CardContent>
    </Card>
  );
}

function ProfileField({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </dt>
      <dd className="mt-1 text-sm text-foreground">{value}</dd>
    </div>
  );
}
