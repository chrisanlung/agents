"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

const PAYOUT_SUB_NAV = [
  { href: "/payout/reconciliation", label: "Rekonsiliasi" },
  { href: "/payout/tenant-payout", label: "Pencairan Tenant" },
  { href: "/payout/disbursements", label: "Disburse Aktif" },
];

export function PayoutSubNav() {
  const pathname = usePathname();

  return (
    <nav
      aria-label="Navigasi payout"
      className="mb-6 flex gap-1 border-b border-border pb-3"
    >
      {PAYOUT_SUB_NAV.map((item) => {
        const isActive = pathname.startsWith(item.href);
        return (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
              isActive
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
            aria-current={isActive ? "page" : undefined}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
