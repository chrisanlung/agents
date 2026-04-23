import { redirect } from "next/navigation";
import type { Metadata } from "next";
import { Mail, Shield, User as UserIcon } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { AppBackground } from "@/components/app-background";
import { AppHeader } from "@/components/app-header";
import { type MembershipSummary } from "@/components/workspace-switcher";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export const metadata: Metadata = {
  title: "Pengaturan",
};

interface MeResponse {
  user: {
    full_name: string;
    email: string;
    phone?: string;
    is_active: boolean;
    is_super_admin?: boolean;
  };
  active_membership_id: string | null;
  memberships: MembershipSummary[];
  tenant?: { id: string; name: string; slug: string; status: string };
}

export default async function SettingsPage() {
  let me: MeResponse;
  try {
    me = await apiFetch<MeResponse>("/auth/me", {}, { auth: true });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const { user, tenant, memberships, active_membership_id } = me;
  const activeMembership =
    memberships.find((m) => m.membership_id === active_membership_id) ??
    memberships[0];
  const activeRoles = activeMembership?.roles ?? [];

  return (
    <AppBackground>
      <AppHeader
        fullName={user.full_name}
        email={user.email}
        activeNav="dashboard"
        tenant={tenant ? { name: tenant.name, slug: tenant.slug } : null}
        memberships={memberships}
      />

      <main className="mx-auto max-w-3xl space-y-6 px-6 py-8">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Pengaturan
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Informasi akun Anda. Fitur edit akan tersedia pada pembaruan
            berikutnya.
          </p>
        </div>

        <Card className="bg-white/90 backdrop-blur-sm">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <UserIcon size={18} className="text-primary" aria-hidden="true" />
              Profil Akun
            </CardTitle>
            <CardDescription>Detail identitas Anda di Lustia.</CardDescription>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Nama Lengkap" value={user.full_name} />
              <Field label="Email" value={user.email} icon={<Mail size={14} />} />
              {user.phone && <Field label="Telepon" value={user.phone} />}
              <Field
                label="Status Akun"
                value={user.is_active ? "Aktif" : "Tidak Aktif"}
              />
            </dl>
          </CardContent>
        </Card>

        <Card className="bg-white/90 backdrop-blur-sm">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Shield size={18} className="text-primary" aria-hidden="true" />
              Akses &amp; Tenant
            </CardTitle>
            <CardDescription>
              Tenant aktif Anda dan peran yang dimiliki.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {tenant && (
                <>
                  <Field label="Nama Perusahaan" value={tenant.name} />
                  <Field label="Slug" value={tenant.slug} mono />
                  <Field label="Status Tenant" value={tenant.status} />
                </>
              )}
              <div>
                <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  Peran
                </dt>
                <dd className="mt-2 flex flex-wrap gap-1.5">
                  {activeRoles.length > 0 ? (
                    activeRoles.map((role) => (
                      <Badge key={role} variant="secondary">
                        {role}
                      </Badge>
                    ))
                  ) : (
                    <span className="text-sm text-muted-foreground">—</span>
                  )}
                </dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        <Card className="bg-white/90 backdrop-blur-sm">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Shield size={18} className="text-primary" aria-hidden="true" />
              Keamanan
            </CardTitle>
            <CardDescription>
              Kelola kata sandi dan pengaturan keamanan akun.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <a
              href="/pengaturan/ubah-kata-sandi"
              className="inline-flex items-center gap-1.5 text-sm font-medium text-primary underline-offset-4 hover:underline"
            >
              Ubah kata sandi
            </a>
          </CardContent>
        </Card>

        <p className="text-center text-xs text-muted-foreground">
          Preferensi notifikasi akan hadir pada fase berikutnya.
        </p>
      </main>
    </AppBackground>
  );
}

function Field({
  label,
  value,
  icon,
  mono,
}: {
  label: string;
  value: string;
  icon?: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div>
      <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </dt>
      <dd
        className={`mt-1 flex items-center gap-1.5 text-sm text-foreground ${
          mono ? "font-mono" : ""
        }`}
      >
        {icon}
        <span className="truncate">{value}</span>
      </dd>
    </div>
  );
}
