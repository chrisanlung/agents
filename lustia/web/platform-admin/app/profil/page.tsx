import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ChevronLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { ConsoleHeader } from "@/components/console-header";
import { Card, CardContent } from "@/components/ui/card";
import type { RegistrationListResponse, AdminDisbursementListResponse } from "@/lib/types";
import type { UserProfileResponse } from "./actions";
import { ProfileForm } from "./profile-form";

export const metadata: Metadata = {
  title: "Profil Saya",
};

interface MeResponse {
  user: UserProfileResponse;
}

export default async function ProfilPage() {
  // Parallel fetch: /auth/me (required) + pending registration count + failed disbursement count (best-effort)
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

  // Auth guard
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
      <ConsoleHeader
        user={{
          full_name: user.full_name,
          email: user.email,
          avatar_url: user.avatar_url ?? undefined,
        }}
        pendingRegistrationCount={pendingRegistrationCount}
        failedDisbursementCount={failedDisbCount}
      />

      <main className="mx-auto max-w-3xl space-y-6 px-6 py-8">
        {/* Page header */}
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Profil Saya
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Kelola informasi akun Anda.
          </p>
        </div>

        {/* Profile card */}
        <Card className="bg-white shadow-sm">
          <CardContent className="p-6">
            <ProfileForm user={user} />
          </CardContent>
        </Card>

        {/* Back link */}
        <Link
          href="/dashboard"
          className="inline-flex items-center gap-1 text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
        >
          <ChevronLeft size={14} aria-hidden="true" />
          Kembali ke Dasbor
        </Link>
      </main>
    </div>
  );
}
