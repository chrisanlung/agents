import { redirect } from "next/navigation";
import type { Metadata } from "next";
import { AlertTriangle, KeyRound } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { getAccessToken } from "@/lib/session";
import { decodeJWTPayload } from "@/lib/jwt";
import { ConsoleHeader } from "@/components/console-header";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { RegistrationListResponse, AdminDisbursementListResponse } from "@/lib/types";
import { ChangePasswordForm } from "./change-password-form";

export const metadata: Metadata = {
  title: "Ubah Kata Sandi",
};

interface MeResponse {
  user: {
    full_name: string;
    email: string;
    avatar_url?: string;
  };
}

interface PageProps {
  searchParams: Promise<{ reason?: string }>;
}

export default async function UbahKataSandiPage({ searchParams }: PageProps) {
  const params = await searchParams;

  // The JWT's must_change_password claim is the source of truth for forced mode;
  // the ?reason=required query param is a secondary signal from the banner link.
  let mustChange = false;
  try {
    const token = await getAccessToken();
    if (token) {
      const claims = decodeJWTPayload<{ must_change_password?: boolean }>(token);
      mustChange = claims.must_change_password === true;
    }
  } catch {
    // Non-fatal — fall back to query param signal.
  }
  const forced = mustChange || params.reason === "required";

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

      <main className="mx-auto max-w-lg px-6 py-8">
        {forced && (
          <div
            role="alert"
            className="mb-4 flex items-start gap-3 rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900"
          >
            <AlertTriangle
              size={18}
              className="mt-0.5 shrink-0 text-amber-600"
              aria-hidden="true"
            />
            <div>
              <p className="font-medium">Anda menggunakan kata sandi sementara</p>
              <p className="mt-1 text-amber-800">
                Buat kata sandi baru untuk melanjutkan ke akun Anda.
              </p>
            </div>
          </div>
        )}

        <Card className="bg-white/95 shadow-sm backdrop-blur-sm">
          <CardHeader>
            <div className="flex items-center gap-2">
              {/* bg-primary/10 (indigo) instead of tenant-admin's bg-pink-100 */}
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <KeyRound size={18} aria-hidden="true" />
              </div>
              <div>
                <CardTitle className="text-lg">
                  {forced ? "Buat Kata Sandi Baru" : "Ubah Kata Sandi"}
                </CardTitle>
                <CardDescription>
                  {forced
                    ? "Kata sandi sementara hanya bisa dipakai sekali."
                    : "Untuk keamanan akun Anda, gunakan kata sandi yang kuat dan unik."}
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <ChangePasswordForm forced={forced} />
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
