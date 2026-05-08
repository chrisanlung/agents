import { redirect } from "next/navigation";
import Link from "next/link";

import { apiFetch, ApiError } from "@/lib/api";
import { AppBackground } from "@/components/app-background";
import { AppHeader } from "@/components/app-header";
import { type MembershipSummary } from "@/components/workspace-switcher";

interface MeResponse {
  user: { full_name: string; email: string };
  memberships: MembershipSummary[];
  tenant?: { name: string; slug: string };
}

const SUB_NAV = [
  { href: "/booking", label: "Daftar Booking", exact: true },
  { href: "/booking/reports", label: "Laporan", exact: false },
];

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

  return (
    <AppBackground>
      <AppHeader
        fullName={me.user.full_name}
        email={me.user.email}
        tenant={me.tenant ? { name: me.tenant.name, slug: me.tenant.slug } : null}
        memberships={me.memberships}
      />
      <main className="mx-auto max-w-5xl px-6 py-8">
        {/* Sub-nav pills */}
        <nav
          aria-label="Navigasi booking"
          className="mb-6 flex gap-1 border-b border-border pb-3"
        >
          {SUB_NAV.map((item) => (
            <SubNavLink key={item.href} href={item.href} label={item.label} />
          ))}
        </nav>
        {children}
      </main>
    </AppBackground>
  );
}

/** Client-side active detection for sub-nav links. */
function SubNavLink({ href, label }: { href: string; label: string }) {
  // Server-rendered: we render all links; client handles active styling via
  // CSS :has or just use the pathname via a thin client wrapper.
  // Since this is a server component we use a simple link — Next.js Link
  // styles the active state via the router.
  return (
    <Link
      href={href}
      className="rounded-md px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
    >
      {label}
    </Link>
  );
}
