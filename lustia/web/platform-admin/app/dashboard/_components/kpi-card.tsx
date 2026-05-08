import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface KpiCardProps {
  label: string;
  value: string;
  subLabel: string;
  icon: LucideIcon;
  iconBg: string;
  iconColor: string;
  valueClassName?: string;
  href: string;
  /** When true the card renders in error state with value "—" */
  error?: boolean;
}

/**
 * PA-T1 KPI card recipe — a full-card Link with icon circle, big number,
 * label and optional sub-label.  Used by all 4 KPI cards on the
 * platform-admin dashboard.  The card is the <Link>; no nested interactive
 * elements are allowed inside it.
 */
export function KpiCard({
  label,
  value,
  subLabel,
  icon: Icon,
  iconBg,
  iconColor,
  valueClassName,
  href,
  error = false,
}: KpiCardProps) {
  return (
    <Link
      href={href}
      className={cn(
        "block rounded-lg border bg-card shadow-sm",
        "transition-shadow duration-150 hover:shadow-md",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      )}
    >
      <div className="flex items-center gap-4 p-5">
        <div
          className={cn(
            "flex h-11 w-11 shrink-0 items-center justify-center rounded-full",
            error ? "bg-muted" : iconBg
          )}
        >
          <Icon
            size={20}
            className={error ? "text-muted-foreground/50" : iconColor}
            aria-hidden="true"
          />
        </div>
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            {label}
          </p>
          <p
            className={cn(
              "mt-0.5 text-3xl font-bold tabular-nums",
              error ? "text-muted-foreground" : (valueClassName ?? "text-foreground")
            )}
          >
            {error ? "—" : value}
          </p>
          <p
            className={cn(
              "mt-0.5 text-xs",
              error ? "text-destructive" : "text-muted-foreground"
            )}
          >
            {error ? "Gagal memuat" : subLabel}
          </p>
        </div>
      </div>
    </Link>
  );
}
