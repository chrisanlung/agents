import { redirect } from "next/navigation";
import type { Metadata } from "next";
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
import { OpsHeader } from "@/components/ops-header";
import { ChangePasswordForm } from "./change-password-form";
import type { MembershipSummary } from "@/components/workspace-switcher";

export const metadata: Metadata = {
  title: "Ubah Kata Sandi",
};

interface MeResponse {
  user: { full_name: string; email: string };
  memberships: MembershipSummary[];
  tenant?: { name: string; slug: string };
}

interface PageProps {
  searchParams: Promise<{ reason?: string }>;
}

export default async function UbahKataSandiPage({ searchParams }: PageProps) {
  const params = await searchParams;

  // JWT must_change_password claim is the source of truth; ?reason=required is
  // the secondary signal from the dashboard banner link.
  let mustChange = false;
  try {
    const token = await getAccessToken();
    if (token) {
      const claims = decodeJWTPayload<{ must_change_password?: boolean }>(token);
      mustChange = claims.must_change_password === true;
    }
  } catch {
    // Non-fatal — fall back to query param.
  }
  const forced = mustChange || params.reason === "required";

  let me: MeResponse;
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
    me = meRes.value;
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

  return (
    <div className="min-h-screen bg-amber-50">
      <OpsHeader
        fullName={me.user.full_name}
        email={me.user.email}
        tenant={me.tenant ?? null}
        branchName={branchName}
        memberships={me.memberships}
      />

      <main className="mx-auto max-w-lg px-6 py-8">
        {forced && (
          <div
            role="alert"
            className="mb-4 flex items-start gap-3 rounded-lg border border-amber-300 bg-amber-100 p-4 text-sm text-amber-900"
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

        <Card className="bg-white shadow-sm">
          <CardHeader>
            <div className="flex items-center gap-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-amber-100 text-primary">
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
