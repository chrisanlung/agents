import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import {
  Activity,
  Bell,
  ChevronRight,
  KeyRound,
  KeySquare,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { ConsoleHeader } from "@/components/console-header";
import type { RegistrationListResponse, AdminDisbursementListResponse } from "@/lib/types";

export const metadata: Metadata = {
  title: "Pengaturan",
};

interface MeResponse {
  user: {
    id: string;
    email: string;
    full_name: string;
    avatar_url?: string;
  };
}

// ── Tile sub-components ──────────────────────────────────────────────────────

interface SettingsTileProps {
  icon: LucideIcon;
  title: string;
  description: string;
  href: string;
}

function SettingsTile({ icon: Icon, title, description, href }: SettingsTileProps) {
  return (
    <Link
      href={href}
      className="group flex flex-col gap-3 rounded-lg border bg-card p-5
                 shadow-sm transition-shadow
                 hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring"
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
        <Icon size={20} className="text-primary" aria-hidden="true" />
      </div>
      <div>
        <p className="text-sm font-semibold text-foreground">{title}</p>
        <p className="mt-1 text-xs text-muted-foreground line-clamp-3">
          {description}
        </p>
      </div>
      <ChevronRight
        size={16}
        className="mt-auto self-end text-muted-foreground/60 transition-colors group-hover:text-foreground"
        aria-hidden="true"
      />
    </Link>
  );
}

interface SettingsTileDisabledProps {
  icon: LucideIcon;
  title: string;
  description: string;
}

function SettingsTileDisabled({
  icon: Icon,
  title,
  description,
}: SettingsTileDisabledProps) {
  return (
    <div
      className="flex flex-col gap-3 rounded-lg border bg-card p-5 shadow-sm opacity-50 cursor-not-allowed"
      aria-disabled="true"
      aria-label={`${title} — segera hadir`}
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
        <Icon size={20} className="text-muted-foreground" aria-hidden="true" />
      </div>
      <div>
        <p className="text-sm font-semibold text-foreground">{title}</p>
        <p className="mt-1 text-xs text-muted-foreground line-clamp-3">
          {description}
        </p>
      </div>
      <span className="mt-auto self-start inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
        Segera hadir
      </span>
    </div>
  );
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default async function PengaturanPage() {
  const [meRes, pendingRegRes, failedDisbRes] = await Promise.allSettled([
    apiFetch<MeResponse>("/auth/me", {}, { auth: true }),
    apiFetch<RegistrationListResponse>(
      "/admin/tenant-registrations?status=pending&limit=1&page=1",
      {},
      { auth: true }
    ),
    apiFetch<AdminDisbursementListResponse>(
      "/admin/disbursements?status=failed&limit=1&page=1",
      {},
      { auth: true }
    ),
  ]);

  if (meRes.status === "rejected") {
    const err = meRes.reason;
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const { user } = meRes.value;
  const pendingRegistrationCount =
    pendingRegRes.status === "fulfilled"
      ? pendingRegRes.value.total_count
      : 0;
  // AdminDisbursementListResponse uses "total"; fallback 0 on failure
  const failedDisbCount =
    failedDisbRes.status === "fulfilled" ? failedDisbRes.value.total : 0;

  return (
    <div className="min-h-screen bg-slate-50">
      <ConsoleHeader user={user} pendingRegistrationCount={pendingRegistrationCount} failedDisbursementCount={failedDisbCount} />

      <main className="mx-auto max-w-3xl space-y-8 px-6 py-8">
        {/* Page header */}
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Pengaturan
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Kelola keamanan dan preferensi akun Anda.
          </p>
        </div>

        {/* Keamanan section */}
        <section aria-labelledby="security-heading">
          <h2
            id="security-heading"
            className="mb-3 text-xs font-semibold uppercase tracking-widest text-muted-foreground"
          >
            Keamanan
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <SettingsTile
              icon={KeyRound}
              title="Ubah Kata Sandi"
              description="Perbarui kata sandi akun Anda secara berkala untuk keamanan."
              href="/pengaturan/ubah-kata-sandi"
            />
          </div>
        </section>

        {/* Segera Hadir section */}
        <section aria-labelledby="coming-soon-heading">
          <h2
            id="coming-soon-heading"
            className="mb-3 text-xs font-semibold uppercase tracking-widest text-muted-foreground"
          >
            Segera Hadir
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <SettingsTileDisabled
              icon={Bell}
              title="Notifikasi"
              description="Konfigurasi notifikasi email dan sistem."
            />
            <SettingsTileDisabled
              icon={Activity}
              title="Log Aktivitas"
              description="Riwayat aktivitas akun Anda."
            />
            <SettingsTileDisabled
              icon={KeySquare}
              title="API Keys"
              description="Kelola kunci API untuk integrasi pihak ketiga."
            />
          </div>
        </section>
      </main>
    </div>
  );
}
