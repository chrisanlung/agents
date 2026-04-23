import { ShieldCheck } from "lucide-react";
import { NavLinks } from "@/components/nav-links";
import { apiFetch } from "@/lib/api";
import { SignOutButton } from "@/app/dashboard/sign-out-button";

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Platform Console";

interface RegistrationListResponse {
  data: Array<{ id: string }>;
  next_cursor: string | null;
}

export default async function TenantsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Fetch pending count for the nav badge — best-effort, non-fatal
  let pendingCount = 0;
  try {
    const res = await apiFetch<RegistrationListResponse>(
      "/admin/tenant-registrations?status=pending&limit=50",
      {},
      { auth: true }
    );
    pendingCount = res.data.length;
  } catch {
    // Non-fatal
  }

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b bg-white px-6 py-4 shadow-sm">
        <div className="mx-auto flex max-w-6xl items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <ShieldCheck size={16} aria-hidden="true" />
            </div>
            <span className="font-semibold text-foreground">{appName}</span>
          </div>
          <div className="flex items-center gap-4">
            <NavLinks pendingCount={pendingCount} />
            <SignOutButton />
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
    </div>
  );
}
