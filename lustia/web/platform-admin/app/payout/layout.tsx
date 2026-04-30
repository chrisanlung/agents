import { redirect } from "next/navigation";
import { ShieldCheck } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { NavLinks } from "@/components/nav-links";
import { PayoutSubNav } from "./payout-sub-nav";

interface RegistrationListResponse {
  data: Array<{ id: string }>;
  total_count: number;
}

export default async function PayoutLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  let pendingRegistrationCount = 0;

  try {
    const regList = await apiFetch<RegistrationListResponse>(
      "/admin/tenant-registrations?status=pending&limit=1",
      {},
      { auth: true }
    );
    pendingRegistrationCount = regList.data.length > 0 ? regList.data.length : 0;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    // Non-fatal: badge shows 0
  }

  const appName =
    process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Platform Console";

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b bg-white px-6 py-4 shadow-sm">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <ShieldCheck size={16} aria-hidden="true" />
            </div>
            <span className="font-semibold text-foreground">{appName}</span>
          </div>
          <NavLinks pendingCount={pendingRegistrationCount} />
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-8">
        <PayoutSubNav />
        {children}
      </main>
    </div>
  );
}
