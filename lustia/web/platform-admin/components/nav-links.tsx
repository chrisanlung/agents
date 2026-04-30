"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LayoutDashboard, Building2, ClipboardList, Banknote } from "lucide-react";

import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";

interface NavLinksProps {
  /** Count of pending registrations — fetched server-side and passed down. */
  pendingCount: number;
}

const links = [
  { href: "/dashboard", label: "Dasbor", icon: LayoutDashboard },
  { href: "/tenants", label: "Tenant", icon: Building2 },
  { href: "/payout", label: "Payout", icon: Banknote },
  { href: "/tenants/registrations", label: "Registrasi", icon: ClipboardList },
] as const;

export function NavLinks({ pendingCount }: NavLinksProps) {
  const pathname = usePathname();

  return (
    <nav aria-label="Menu utama" className="flex items-center gap-1">
      {links.map(({ href, label, icon: Icon }) => {
        const isActive =
          href === "/dashboard"
            ? pathname === "/dashboard"
            : pathname.startsWith(href);

        return (
          <Link
            key={href}
            href={href}
            className={cn(
              "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors duration-fast",
              isActive
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-accent/60 hover:text-foreground"
            )}
            aria-current={isActive ? "page" : undefined}
          >
            <Icon size={15} aria-hidden="true" />
            {label}
            {href === "/tenants" && pendingCount > 0 && (
              <Badge
                variant="default"
                className="ml-0.5 h-4 min-w-[1rem] px-1 text-[10px] leading-none"
                aria-label={`${pendingCount} registrasi menunggu`}
              >
                {pendingCount > 99 ? "99+" : pendingCount}
              </Badge>
            )}
          </Link>
        );
      })}
    </nav>
  );
}
