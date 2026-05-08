import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { AlertTriangle, KeyRound } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { getAccessToken } from "@/lib/session";
import { decodeJWTPayload } from "@/lib/jwt";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { OpsHeader } from "@/components/ops-header";
import { SignOutButton } from "@/app/dashboard/sign-out-button";
import type { MembershipSummary } from "@/components/workspace-switcher";

export const metadata: Metadata = {
  title: "Pengaturan",
};

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

export default async function PengaturanPage() {
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
      if (branches.length === 1) branchName = branches[0].name;
      else if (branches.length > 1) branchName = `${branches.length} Cabang`;
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const { user, tenant, memberships } = data;

  // must_change_password claim
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

  return (
    <div className="min-h-screen bg-amber-50">
      <OpsHeader
        fullName={user.full_name}
        email={user.email}
        tenant={tenant ?? null}
        branchName={branchName}
        memberships={memberships}
      />

      <main className="mx-auto max-w-2xl space-y-6 px-6 py-8">
        <div>
          <h1 className="text-2xl font-semibold text-foreground">Pengaturan</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Kelola profil dan keamanan akun Anda.
          </p>
        </div>

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
                Kata sandi sementara masih aktif.{" "}
                <Link
                  href="/pengaturan/ubah-kata-sandi?reason=required"
                  className="font-medium underline underline-offset-4 hover:text-yellow-900"
                >
                  Ubah sekarang
                </Link>
              </p>
            </div>
          </div>
        )}

        {/* ── Profil ───────────────────────────────────────────────────── */}
        <Card>
          <CardHeader>
            <CardTitle>Profil Anda</CardTitle>
            <CardDescription>
              Informasi akun staf Anda pada platform Lustia Operations.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
              <ProfileField
                label="Cabang"
                value={
                  user.branches && user.branches.length > 0
                    ? user.branches.join(", ")
                    : "—"
                }
              />
              {user.phone && (
                <ProfileField label="Telepon" value={user.phone} />
              )}
              <ProfileField
                label="Status Akun"
                value={user.is_active ? "Aktif" : "Tidak Aktif"}
              />
            </dl>
          </CardContent>
        </Card>

        {/* ── Keamanan ─────────────────────────────────────────────────── */}
        <Card>
          <CardHeader>
            <CardTitle>Keamanan</CardTitle>
            <CardDescription>
              Kelola kata sandi dan keamanan akun Anda.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between rounded-lg border bg-background px-4 py-3">
              <div className="flex items-center gap-3">
                <KeyRound
                  size={18}
                  className="shrink-0 text-muted-foreground"
                  aria-hidden="true"
                />
                <div>
                  <p className="text-sm font-medium text-foreground">
                    Ubah Kata Sandi
                  </p>
                  <p className="text-xs text-muted-foreground">
                    Ganti kata sandi akun Anda secara berkala
                  </p>
                </div>
              </div>
              <Button asChild size="sm" variant="outline">
                <Link href="/pengaturan/ubah-kata-sandi">Ubah</Link>
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* ── Keluar ───────────────────────────────────────────────────── */}
        <Card className="border-destructive/20">
          <CardHeader>
            <CardTitle className="text-base text-destructive">
              Keluar dari Akun
            </CardTitle>
            <CardDescription>
              Anda akan keluar dari semua sesi aktif di perangkat ini.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SignOutButton />
          </CardContent>
        </Card>
      </main>
    </div>
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
