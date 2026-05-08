import { redirect } from "next/navigation";

import { apiFetch, ApiError } from "@/lib/api";
import { ConsoleHeader } from "@/components/console-header";
import { PayoutSubNav } from "./payout-sub-nav";

interface RegistrationListResponse {
  data: Array<{ id: string }>;
  total_count: number;
}

interface DisbursementListResponse {
  data: Array<{ id: string }>;
  total: number;
}

interface MeResponse {
  user: {
    id: string;
    email: string;
    full_name: string;
    avatar_url?: string;
  };
}

export default async function PayoutLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Fetch identity + pending count + failed disbursement count in parallel
  // /auth/me is required; others are best-effort (default 0 on failure)
  const [meRes, regRes, failedDisbRes] = await Promise.allSettled([
    apiFetch<MeResponse>("/auth/me", {}, { auth: true }),
    apiFetch<RegistrationListResponse>(
      "/admin/tenant-registrations?status=pending&limit=1",
      {},
      { auth: true }
    ),
    apiFetch<DisbursementListResponse>(
      "/admin/disbursements?status=failed&limit=1&page=1",
      {},
      { auth: true }
    ),
  ]);

  // Auth guard — redirect on 401, throw on other unexpected errors
  if (meRes.status === "rejected") {
    const err = meRes.reason;
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const user = meRes.value.user;

  // Pending count — use total_count; fallback 0 on failure
  const pendingRegistrationCount =
    regRes.status === "fulfilled" ? regRes.value.total_count : 0;

  // Failed disbursement count — AdminDisbursementListResponse uses "total"; fallback 0 on failure
  const failedDisbCount =
    failedDisbRes.status === "fulfilled" ? failedDisbRes.value.total : 0;

  return (
    <div className="min-h-screen bg-slate-50">
      <ConsoleHeader user={user} pendingRegistrationCount={pendingRegistrationCount} failedDisbursementCount={failedDisbCount} />

      <main className="mx-auto max-w-5xl px-6 py-8">
        <PayoutSubNav />
        {children}
      </main>
    </div>
  );
}
