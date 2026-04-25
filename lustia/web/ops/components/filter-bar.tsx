import Link from "next/link";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

interface FilterBarProps {
  children: React.ReactNode;
  /** Change container color + show a "Hapus filter" link when true. */
  isActive?: boolean;
  /** Target URL for the reset link (pathname only, no query string). */
  resetHref?: string;
  className?: string;
}

/**
 * Framed container for list-page filter controls.
 * - Subtle border + muted background clearly separates the filter region.
 * - When `isActive` is true, the container shifts to a primary-tinted state and
 *   exposes a "× Hapus filter" link so users can quickly return to the
 *   unfiltered list — a major finding from the UX review (2026-04-24).
 * - No icon/label prefix; the framed box + inline dropdowns already read as
 *   "filter this list" without extra chrome.
 */
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
