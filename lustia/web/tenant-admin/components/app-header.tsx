"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Building2,
  Calendar,
  ChevronDown,
  Database,
  DoorOpen,
  LayoutDashboard,
  MapPin,
  PackagePlus,
  Settings,
  Sparkles,
  Stethoscope,
  Users,
  Wallet,
} from "lucide-react";

import { WorkspaceSwitcher, type MembershipSummary } from "@/components/workspace-switcher";
import { UserMenu } from "@/components/user-menu";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Tenant Portal";

// ─── Nav types ───────────────────────────────────────────────────────────────

export type NavKey = "dashboard" | "booking" | "master-data" | "manajemen";

interface NavChild {
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
  description: string;
}

interface StandaloneNavItem {
  kind: "link";
  key: NavKey;
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
  /** Extra pathname prefix(es) that also count as active */
  activePrefix?: string | string[];
}

interface DropdownNavItem {
  kind: "dropdown";
  key: NavKey;
  label: string;
  icon: typeof LayoutDashboard;
  /** Pathname prefix(es): if any matches the current path this group is active */
  activePrefix: string | string[];
  children: NavChild[];
}

type NavItem = StandaloneNavItem | DropdownNavItem;

// ─── Nav config ──────────────────────────────────────────────────────────────

const NAV_ITEMS: NavItem[] = [
  {
    kind: "link",
    key: "dashboard",
    href: "/dashboard",
    label: "Dasbor",
    icon: LayoutDashboard,
  },
  {
    kind: "link",
    key: "booking",
    href: "/booking",
    label: "Booking",
    icon: Calendar,
    activePrefix: "/booking",
  },
  {
    kind: "dropdown",
    key: "master-data",
    label: "Master Data",
    icon: Database,
    activePrefix: ["/branches", "/master"],
    children: [
      {
        href: "/branches",
        label: "Cabang",
        icon: MapPin,
        description: "Lokasi & jam operasional",
      },
      {
        href: "/master/rooms",
        label: "Ruangan",
        icon: DoorOpen,
        description: "Daftar ruangan & kapasitas",
      },
      {
        href: "/master/therapists",
        label: "Terapis",
        icon: Stethoscope,
        description: "Profil & jadwal terapis",
      },
      {
        href: "/master/services",
        label: "Layanan",
        icon: Sparkles,
        description: "Daftar layanan & harga",
      },
      {
        href: "/master/addons",
        label: "Tambahan",
        icon: PackagePlus,
        description: "Add-on & extra",
      },
    ],
  },
  {
    kind: "dropdown",
    key: "manajemen",
    label: "Manajemen",
    icon: Settings,
    activePrefix: ["/users", "/keuangan"],
    children: [
      {
        href: "/users",
        label: "Pengguna",
        icon: Users,
        description: "Akun staf & branch admin",
      },
      {
        href: "/keuangan",
        label: "Keuangan",
        icon: Wallet,
        description: "Saldo, pencairan, transaksi",
      },
    ],
  },
];

// ─── Active-state helper ─────────────────────────────────────────────────────

/**
 * Derive the active nav key from a pathname string.
 * Useful in layouts that want to auto-detect the active item.
 */
export function getActiveNavKey(pathname: string): NavKey {
  if (pathname.startsWith("/booking")) return "booking";
  if (pathname.startsWith("/branches") || pathname.startsWith("/master")) return "master-data";
  if (pathname.startsWith("/users") || pathname.startsWith("/keuangan")) return "manajemen";
  return "dashboard";
}

// ─── Sub-components ──────────────────────────────────────────────────────────

const NAV_LINK_BASE =
  "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors";
const NAV_LINK_ACTIVE = "bg-white/70 text-foreground shadow-sm ring-1 ring-pink-200";
const NAV_LINK_INACTIVE = "text-muted-foreground hover:bg-white/50 hover:text-foreground";

function StandaloneLink({
  item,
  activeKey,
}: {
  item: StandaloneNavItem;
  activeKey: NavKey;
}) {
  const Icon = item.icon;
  const isActive = item.key === activeKey;
  return (
    <Link
      href={item.href}
      className={cn(NAV_LINK_BASE, isActive ? NAV_LINK_ACTIVE : NAV_LINK_INACTIVE)}
      aria-current={isActive ? "page" : undefined}
    >
      <Icon size={15} aria-hidden="true" />
      {item.label}
    </Link>
  );
}

function NavDropdown({
  item,
  activeKey,
}: {
  item: DropdownNavItem;
  activeKey: NavKey;
}) {
  const Icon = item.icon;
  const isActive = item.key === activeKey;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className={cn(NAV_LINK_BASE, isActive ? NAV_LINK_ACTIVE : NAV_LINK_INACTIVE)}
          aria-expanded={undefined /* Radix manages aria-expanded */}
        >
          <Icon size={15} aria-hidden="true" />
          {item.label}
          <ChevronDown size={13} aria-hidden="true" className="ml-0.5 opacity-60" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-72 p-2">
        {item.children.map((child) => {
          const ChildIcon = child.icon;
          return (
            <DropdownMenuItem key={child.href} asChild>
              <Link
                href={child.href}
                className="flex cursor-pointer items-start gap-3 rounded-sm px-2 py-2"
              >
                <ChildIcon
                  size={18}
                  aria-hidden="true"
                  className="mt-0.5 shrink-0 text-primary"
                />
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium leading-none">{child.label}</span>
                  <span className="text-xs text-muted-foreground">{child.description}</span>
                </div>
              </Link>
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// ─── AppHeader ────────────────────────────────────────────────────────────────

interface AppHeaderProps {
  /** The user's full name for the avatar + dropdown */
  fullName: string;
  /** The user's email, shown in the dropdown */
  email: string;
  /** Tenant summary — if present, renders the workspace switcher */
  tenant?: { name: string; slug: string } | null;
  /** Memberships for the workspace switcher (used when the user has more than one) */
  memberships?: MembershipSummary[];
}

/**
 * AppHeader — shared header for all post-login pages.
 *
 * Active nav key is derived from the pathname automatically via `getActiveNavKey`
 * so call-site layouts no longer need to pass `activeNav` manually.
 *
 * Layout (left → right): logo · nav items (Dasbor / Booking / Master Data ▾ / Manajemen ▾) · workspace switcher · user avatar.
 *
 * Mobile (< sm): nav is hidden; a simple hamburger + Sheet drawer is deferred — flagged as TODO.
 */
export function AppHeader({ fullName, email, tenant, memberships }: AppHeaderProps) {
  const pathname = usePathname();
  const activeKey = getActiveNavKey(pathname);

  return (
    <header className="border-b border-pink-100/70 bg-gradient-to-r from-pink-100 via-pink-50 to-white px-6 py-3 shadow-sm backdrop-blur-sm">
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4">
        {/* Brand + nav */}
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

          {/* Desktop nav */}
          <nav aria-label="Menu utama" className="ml-4 hidden items-center gap-1 sm:flex">
            {NAV_ITEMS.map((item) =>
              item.kind === "link" ? (
                <StandaloneLink key={item.key} item={item} activeKey={activeKey} />
              ) : (
                <NavDropdown key={item.key} item={item} activeKey={activeKey} />
              )
            )}
          </nav>
          {/* TODO: mobile hamburger + Sheet drawer (deferred) */}
        </div>

        {/* Right controls */}
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
