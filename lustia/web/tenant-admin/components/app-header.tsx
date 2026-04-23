import Link from "next/link";
import { Building2, LayoutDashboard, MapPin, Stethoscope } from "lucide-react";

import { WorkspaceSwitcher, type MembershipSummary } from "@/components/workspace-switcher";
import { UserMenu } from "@/components/user-menu";
import { cn } from "@/lib/utils";

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Tenant Portal";

type NavKey = "dashboard" | "branches" | "operasional";

interface NavItem {
  key: NavKey;
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
}

const NAV_ITEMS: NavItem[] = [
  { key: "dashboard", href: "/dashboard", label: "Dasbor", icon: LayoutDashboard },
  { key: "branches", href: "/branches", label: "Cabang", icon: MapPin },
  { key: "operasional", href: "/master/therapists", label: "Operasional", icon: Stethoscope },
];

interface AppHeaderProps {
  /** The user's full name for the avatar + dropdown */
  fullName: string;
  /** The user's email, shown in the dropdown */
  email: string;
  /** Which nav item is active (so we can highlight + set aria-current) */
  activeNav: NavKey;
  /** Tenant summary — if present, renders the workspace switcher */
  tenant?: { name: string; slug: string } | null;
  /** Memberships for the workspace switcher (used when the user has more than one) */
  memberships?: MembershipSummary[];
}

/**
 * AppHeader — shared header for all post-login pages.
 *
 * Layout (left → right): logo + product name · nav links · workspace switcher · user avatar dropdown.
 * The workspace switcher and avatar appear on the right so the navigation stays
 * closer to the brand and doesn't crowd the user controls.
 */
export function AppHeader({
  fullName,
  email,
  activeNav,
  tenant,
  memberships,
}: AppHeaderProps) {
  return (
    <header className="border-b border-pink-100/70 bg-gradient-to-r from-pink-100 via-pink-50 to-white px-6 py-3 shadow-sm backdrop-blur-sm">
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Link
            href="/dashboard"
            className="flex items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
            aria-label={appName}
          >
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <Building2 size={16} aria-hidden="true" />
            </div>
            <span className="font-semibold text-foreground">{appName}</span>
          </Link>

          <nav aria-label="Menu utama" className="ml-4 hidden items-center gap-1 sm:flex">
            {NAV_ITEMS.map((item) => {
              const Icon = item.icon;
              const isActive = item.key === activeNav;
              return (
                <Link
                  key={item.key}
                  href={item.href}
                  className={cn(
                    "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                    isActive
                      ? "bg-white/70 text-foreground shadow-sm ring-1 ring-pink-200"
                      : "text-muted-foreground hover:bg-white/50 hover:text-foreground"
                  )}
                  aria-current={isActive ? "page" : undefined}
                >
                  <Icon size={15} aria-hidden="true" />
                  {item.label}
                </Link>
              );
            })}
          </nav>
        </div>

        <div className="flex items-center gap-3">
          {tenant && memberships && (
            <WorkspaceSwitcher
              currentTenantName={tenant.name}
              currentTenantSlug={tenant.slug}
              memberships={memberships}
            />
          )}
          <UserMenu fullName={fullName} email={email} />
        </div>
      </div>
    </header>
  );
}

export type { NavKey };
