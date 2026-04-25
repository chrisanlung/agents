import { redirect } from "next/navigation";
import { Zap } from "lucide-react";
import Link from "next/link";

import { apiFetch, ApiError } from "@/lib/api";

interface MeResponse {
  user: { full_name: string; email: string };
  tenant?: { name: string; slug: string };
}

export default async function BookingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  let me: MeResponse;
  try {
    me = await apiFetch<MeResponse>("/auth/me", {}, { auth: true });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Operations";

  return (
    <div className="min-h-screen bg-amber-50">
      <header className="border-b bg-white px-6 py-3 shadow-sm">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <Link
              href="/dashboard"
              className="flex items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-primary"
              aria-label={appName}
            >
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <Zap size={16} aria-hidden="true" />
              </div>
              <span className="font-semibold text-foreground">{appName}</span>
            </Link>
            <nav
              aria-label="Menu utama"
              className="ml-4 hidden items-center gap-1 sm:flex"
            >
              <Link
                href="/dashboard"
                className="rounded-md px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-white hover:text-foreground"
              >
                Dasbor
              </Link>
              <Link
                href="/booking"
                className="rounded-md bg-white px-3 py-1.5 text-sm font-medium text-foreground shadow-sm ring-1 ring-amber-200"
                aria-current="page"
              >
                Booking
              </Link>
            </nav>
          </div>
          <span className="text-sm text-muted-foreground">
            {me.user.full_name}
            {me.tenant && (
              <span className="ml-1 text-xs opacity-60">
                — {me.tenant.name}
              </span>
            )}
          </span>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
    </div>
  );
}
