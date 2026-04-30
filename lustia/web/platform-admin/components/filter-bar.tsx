import Link from "next/link";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

interface FilterBarProps {
  children: React.ReactNode;
  isActive?: boolean;
  resetHref?: string;
  className?: string;
}

export function FilterBar({
  children,
  isActive = false,
  resetHref,
  className,
}: FilterBarProps) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border px-3 py-2 transition-colors",
        isActive
          ? "border-primary/40 bg-primary/5"
          : "border-border bg-muted/40",
        className
      )}
      role="region"
      aria-label="Filter daftar"
    >
      <div className="flex flex-1 flex-wrap items-center gap-x-4 gap-y-2">
        {children}
      </div>
      {isActive && resetHref && (
        <Link
          href={resetHref}
          className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium text-primary hover:bg-primary/10"
        >
          <X size={12} aria-hidden="true" />
          Hapus filter
        </Link>
      )}
    </div>
  );
}
