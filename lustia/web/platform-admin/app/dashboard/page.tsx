import { redirect } from "next/navigation";
import type { Metadata } from "next";
import { ShieldCheck, AlertTriangle } from "lucide-react";

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
import { SignOutButton } from "./sign-out-button";

export const metadata: Metadata = {
  title: "Dasbor",
};

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Platform Console";

interface UserProfile {
  id: string;
  email: string;
  full_name: string;
  phone?: string;
  avatar_url?: string;
  is_active: boolean;
  is_super_admin?: boolean;
  roles: string[];
  branches: string[];
}

/** ADR 0007 §2.3 — updated /auth/me response */
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
 * Dashboard Server Component.
 *
 * Calls GET /auth/me with the access token from the session cookie to verify
 * the session is still valid and fetch the user's current profile. If the
 * token is expired or invalid, the backend returns 401 and we redirect to
 * /login — this is the "real" auth check (middleware only checks cookie presence).
 */
export default async function DashboardPage() {
  let data: MeResponse;

  try {
    data = await apiFetch<MeResponse>("/auth/me", {}, { auth: true });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      // Token expired or invalid — send back to login.
      // Note: GET /auth/me is explicitly exempted from the PASSWORD_CHANGE_REQUIRED
      // gate (API_CONTRACT.md §3a), so a 403 here is an unexpected error, not a
      // session-expiry — we re-throw it rather than silently redirecting.
      redirect("/login");
    }
    throw err;
  }

  const { user } = data;

  // Decode the JWT to read must_change_password claim.
  // GET /auth/me is always permitted even when must_change_password=true, so we
  // reached this point. We read the claim from the access token to show the banner.
  let mustChange = false;
  try {
    const token = await getAccessToken();
    if (token) {
      const claims = decodeJWTPayload<{ must_change_password?: boolean }>(token);
      mustChange = claims.must_change_password === true;
    }
  } catch {
    // Non-fatal: banner just won't show if decoding fails
  }

  return (
    <div className="min-h-screen bg-slate-50">
      {/* Header */}
      <header className="border-b bg-white px-6 py-4 shadow-sm">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <ShieldCheck size={16} aria-hidden="true" />
            </div>
            <span className="font-semibold text-foreground">{appName}</span>
          </div>
          <SignOutButton />
        </div>
      </header>

      <main className="mx-auto max-w-5xl space-y-6 px-6 py-8">
        {/* must_change_password banner */}
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
                Akun Anda memerlukan perubahan kata sandi sebelum dapat mengakses fitur lain.{" "}
                <span
                  className="cursor-not-allowed font-medium underline underline-offset-4 opacity-60"
                  aria-disabled="true"
                  title="Segera hadir"
                >
                  Ubah kata sandi
                </span>{" "}
                <span className="text-xs text-yellow-600">(segera hadir)</span>
              </p>
            </div>
          </div>
        )}

        {/* Profile card */}
        <Card>
          <CardHeader>
            <CardTitle>Profil Anda</CardTitle>
            <CardDescription>
              Informasi akun dari layanan autentikasi.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <ProfileField label="Nama Lengkap" value={user.full_name} />
              <ProfileField label="Email" value={user.email} />
              <ProfileField
                label="Peran"
                value={user.roles.length > 0 ? user.roles.join(", ") : "—"}
              />
              <ProfileField
                label="Tenant"
                value="__platform__"
              />
              <ProfileField
                label="Status Akun"
                value={user.is_active ? "Aktif" : "Tidak Aktif"}
              />
              {user.phone && (
                <ProfileField label="Telepon" value={user.phone} />
              )}
            </dl>
          </CardContent>
        </Card>

        {/* Placeholder for future platform-admin screens */}
        <Card className="border-dashed">
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <p className="text-sm text-muted-foreground">
              Layar manajemen platform akan muncul di sini pada fase mendatang.
            </p>
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
