import { redirect } from "next/navigation";
import type { Metadata } from "next";
import { Zap, LogOut } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { getAccessToken } from "@/lib/session";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { logoutAction } from "@/app/dashboard/actions";
import { selectTenantAction } from "./actions";

export const metadata: Metadata = {
  title: "Pilih Tenant",
};

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Operations";

export interface MembershipSummary {
  tenant_id: string;
  tenant_name: string;
  tenant_slug: string;
  roles: string[];
  branches: string[];
  status: string;
}

interface MeResponse {
  user: {
    id: string;
    email: string;
    full_name: string;
    is_active: boolean;
    is_super_admin?: boolean;
  };
  active_membership_id: string | null;
  memberships: MembershipSummary[];
  tenant?: {
    id: string;
    name: string;
    slug: string;
    status: string;
  };
}

/**
 * Select-tenant Server Component.
 *
 * Gate logic (ADR 0007 §2.4):
 * 1. No access_token cookie → redirect /login (middleware already handles this,
 *    but defensive check here too).
 * 2. GET /auth/me returns active_membership_id !== null → user already scoped,
 *    redirect /dashboard.
 * 3. Otherwise render a card grid of memberships.
 */
export default async function SelectTenantPage() {
  // Defensive: middleware should already guard this, but double-check.
  const token = await getAccessToken();
  if (!token) {
    redirect("/login");
  }

  let data: MeResponse;
  try {
    data = await apiFetch<MeResponse>("/auth/me", {}, { auth: true });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  // Already has an active tenant scope — go straight to dashboard.
  if (data.active_membership_id !== null) {
    redirect("/dashboard");
  }

  const { user, memberships } = data;

  return (
    <main className="flex min-h-screen flex-col items-center justify-center bg-amber-50 px-4 py-12">
      <div className="w-full max-w-2xl space-y-8">
        {/* Header */}
        <div className="flex flex-col items-center gap-3 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
            <Zap size={24} aria-hidden="true" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight text-foreground">
              {appName}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Selamat datang, {user.full_name}
            </p>
          </div>
        </div>

        {/* Title card */}
        <div className="text-center">
          <h2 className="text-2xl font-bold tracking-tight text-foreground">
            Pilih workspace
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Anda adalah anggota di beberapa tenant. Pilih yang ingin Anda kelola.
          </p>
        </div>

        {/* Membership card grid */}
        {memberships.length === 0 ? (
          <Card className="border-border shadow-sm">
            <CardContent className="flex flex-col items-center justify-center py-12 text-center">
              <p className="text-sm text-muted-foreground">
                Akun Anda tidak memiliki keanggotaan aktif.
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            {memberships.map((membership) => (
              <MembershipCard key={membership.tenant_id} membership={membership} />
            ))}
          </div>
        )}

        {/* Logout link */}
        <div className="flex justify-center">
          <form action={logoutAction}>
            <button
              type="submit"
              className="flex items-center gap-1.5 text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
            >
              <LogOut size={14} aria-hidden="true" />
              Keluar
            </button>
          </form>
        </div>
      </div>
    </main>
  );
}

function MembershipCard({ membership }: { membership: MembershipSummary }) {
  const rolesLabel =
    membership.roles && membership.roles.length > 0
      ? membership.roles.join(", ")
      : "—";

  return (
    <Card className="border-border shadow-sm transition-shadow hover:shadow-md">
      <CardHeader className="pb-2">
        <CardTitle className="text-base">{membership.tenant_name}</CardTitle>
        <CardDescription className="font-mono text-xs">
          {membership.tenant_slug}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          <span className="font-medium">Peran:</span> {rolesLabel}
        </p>
        {/* Server Action form — works without JS (progressive enhancement) */}
        <form
          action={async () => {
            "use server";
            await selectTenantAction(membership.tenant_id);
          }}
        >
          <button
            type="submit"
            className="inline-flex h-9 w-full items-center justify-center rounded-md bg-primary px-4 text-sm font-medium text-primary-foreground shadow-sm transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
          >
            Pilih
          </button>
        </form>
      </CardContent>
    </Card>
  );
}
