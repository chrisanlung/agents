"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTransition } from "react";
import {
  Calendar,
  LayoutDashboard,
  LogOut,
  Loader2,
  Plus,
  QrCode,
  Settings,
  Store,
  Zap,
  KeyRound,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { logoutAction } from "@/app/dashboard/actions";
import type { MembershipSummary } from "@/components/workspace-switcher";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Operations";

// ─── Nav types ────────────────────────────────────────────────────────────────

export type NavKey =
  | "dashboard"
  | "booking"
  | "checkin"
  | "booking-new"
  | "pengaturan"
  | null;

// ─── Nav config ───────────────────────────────────────────────────────────────

interface NavItem {
  key: NavKey;
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
  /** Prefix match wins over exact. Exact `/booking` wins over `/booking/checkin`. */
  exactMatch?: boolean;
}

const NAV_ITEMS: NavItem[] = [
  {
    key: "dashboard",
    href: "/dashboard",
    label: "Dasbor",
    icon: LayoutDashboard,
    exactMatch: true,
  },
  {
    key: "booking",
    href: "/booking",
    label: "Booking",
    icon: Calendar,
  },
  {
    key: "checkin",
    href: "/booking/checkin",
    label: "Check-in",
    icon: QrCode,
  },
  {
    key: "booking-new",
    href: "/booking/new",
    label: "Booking Baru",
    icon: Plus,
  },
];

// ─── Active-state helper ──────────────────────────────────────────────────────

export function getActiveNavKey(pathname: string): NavKey {
  if (pathname === "/dashboard") return "dashboard";
  if (pathname.startsWith("/booking/checkin")) return "checkin";
  if (pathname.startsWith("/booking/new")) return "booking-new";
  if (pathname.startsWith("/booking")) return "booking";
  if (pathname.startsWith("/pengaturan")) return "pengaturan";
  return null;
}

// ─── Initials helper ──────────────────────────────────────────────────────────

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
}

// ─── UserMenu ─────────────────────────────────────────────────────────────────

interface UserMenuProps {
  fullName: string;
  email: string;
}

function UserMenu({ fullName, email }: UserMenuProps) {
  const [isPending, startTransition] = useTransition();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-9 rounded-full bg-white/80 p-0 text-sm font-semibold text-primary shadow-sm ring-1 ring-amber-200 hover:bg-white"
          aria-label={`Menu akun ${fullName}`}
        >
          {initials(fullName)}
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-60">
        <div className="px-2 py-2">
          <p className="truncate text-sm font-semibold text-foreground">
            {fullName}
          </p>
          <p className="truncate text-xs text-muted-foreground">{email}</p>
        </div>

        <DropdownMenuSeparator />

        <DropdownMenuItem asChild>
          <Link href="/pengaturan" className="cursor-pointer">
            <Settings size={14} className="mr-2" aria-hidden="true" />
            Pengaturan
          </Link>
        </DropdownMenuItem>

        <DropdownMenuItem asChild>
          <Link
            href="/pengaturan/ubah-kata-sandi"
            className="cursor-pointer"
          >
            <KeyRound size={14} className="mr-2" aria-hidden="true" />
            Ubah Kata Sandi
          </Link>
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          disabled={isPending}
          onSelect={(e) => {
            e.preventDefault();
            startTransition(() => logoutAction());
          }}
          className="cursor-pointer text-destructive focus:text-destructive"
        >
          {isPending ? (
            <Loader2 size={14} className="mr-2 animate-spin" aria-hidden="true" />
          ) : (
            <LogOut size={14} className="mr-2" aria-hidden="true" />
          )}
          Keluar
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// ─── OpsHeader ────────────────────────────────────────────────────────────────

export interface OpsHeaderProps {
  fullName: string;
  email: string;
  tenant?: { name: string; slug: string } | null;
  /** Branch label shown on the right (cabang assigned ke staff). */
  branchName?: string | null;
  memberships?: MembershipSummary[];
}

/**
 * OpsHeader — shared navigation header for all authenticated ops portal pages.
 *
 * Active nav key auto-derived from pathname via `getActiveNavKey`.
 * Simpler than tenant-admin's AppHeader: no dropdowns, flat nav only.
 *
 * Layout (left → right): logo · Dasbor · Booking · Check-in · Booking Baru ·
 *   [workspace switcher] · user avatar dropdown.
 */
export function OpsHeader({
  fullName,
  email,
  tenant,
  branchName,
  memberships: _memberships,
}: OpsHeaderProps) {
  const pathname = usePathname();
  const activeKey = getActiveNavKey(pathname);

  const NAV_BASE =
    "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors";
  const NAV_ACTIVE =
    "bg-white/70 text-foreground shadow-sm ring-1 ring-amber-200";
  const NAV_INACTIVE =
    "text-muted-foreground hover:bg-white/50 hover:text-foreground";

  return (
    <header className="border-b border-amber-100/70 bg-gradient-to-r from-amber-100 via-amber-50 to-white px-6 py-3 shadow-sm backdrop-blur-sm">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-4">
        {/* Brand + nav */}
        <div className="flex items-center gap-3">
          <Link
            href="/dashboard"
            className="flex items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
            aria-label={tenant?.name ?? appName}
          >
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <Zap size={16} aria-hidden="true" />
            </div>
            <span className="font-semibold text-foreground">
              {tenant?.name ?? appName}
            </span>
          </Link>

          {/* Desktop nav */}
          <nav
            aria-label="Menu utama"
            className="ml-4 hidden items-center gap-1 sm:flex"
          >
            {NAV_ITEMS.map((item) => {
              const isActive = item.key === activeKey;
              const Icon = item.icon;
              return (
                <Link
                  key={item.key}
                  href={item.href}
                  className={cn(NAV_BASE, isActive ? NAV_ACTIVE : NAV_INACTIVE)}
                  aria-current={isActive ? "page" : undefined}
                >
                  <Icon size={15} aria-hidden="true" />
                  {item.label}
                </Link>
              );
            })}
          </nav>
        </div>

        {/* Right controls */}
        <div className="flex items-center gap-3">
          {branchName && (
            <span className="hidden items-center gap-1.5 rounded-full bg-white/80 px-3 py-1 text-sm font-medium text-foreground shadow-sm ring-1 ring-amber-200 sm:inline-flex">
              <Store size={14} aria-hidden="true" />
              {branchName}
            </span>
          )}
          <UserMenu fullName={fullName} email={email} />
        </div>
      </div>
    </header>
  );
}
