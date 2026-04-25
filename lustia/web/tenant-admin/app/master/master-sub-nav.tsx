"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { DoorOpen, PackagePlus, Sparkles, Stethoscope } from "lucide-react";

import { cn } from "@/lib/utils";

const ITEMS = [
  { href: "/master/rooms", label: "Ruangan", icon: DoorOpen },
  { href: "/master/therapists", label: "Terapis", icon: Stethoscope },
  { href: "/master/services", label: "Layanan", icon: Sparkles },
  { href: "/master/addons", label: "Tambahan", icon: PackagePlus },
];

/**
 * Sub-nav inside /master/*. Two tabs: Terapis and Layanan. Highlights the
 * active tab based on pathname — therapist detail and service detail routes
 * also highlight their parent.
 */
export function MasterSubNav() {
  const pathname = usePathname();

  return (
    <nav
      aria-label="Menu master data"
      className="mb-6 flex items-center gap-1 border-b border-border/60"
    >
      {ITEMS.map((item) => {
        const Icon = item.icon;
        const isActive = pathname.startsWith(item.href);
        return (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
            aria-current={isActive ? "page" : undefined}
          >
            <Icon size={15} aria-hidden="true" />
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
