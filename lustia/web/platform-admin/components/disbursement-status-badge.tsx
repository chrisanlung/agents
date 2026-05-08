import { Badge } from "@/components/ui/badge";
import type { DisbursementStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

interface DisbursementStatusBadgeProps {
  status: DisbursementStatus;
  className?: string;
}

const STATUS_CONFIG: Record<
  DisbursementStatus,
  { label: string; className: string }
> = {
  pending: {
    label: "Menunggu",
    className: "border-amber-200 bg-amber-100 text-amber-800",
  },
  processing: {
    label: "Diproses",
    className: "border-blue-200 bg-blue-100 text-blue-800",
  },
  transferred: {
    label: "Ditransfer",
    className: "border-emerald-200 bg-emerald-100 text-emerald-800",
  },
  // WCAG 1.4.1: "failed" uses bold + uppercase label in addition to color.
  failed: {
    label: "GAGAL",
    className: "border-red-300 bg-red-100 text-red-800 font-bold",
  },
  cancelled: {
    label: "Dibatalkan",
    className: "border-border bg-muted text-muted-foreground",
  },
};

export function DisbursementStatusBadge({
  status,
  className,
}: DisbursementStatusBadgeProps) {
  const config = STATUS_CONFIG[status] ?? {
    label: status,
    className: "border-border text-muted-foreground bg-muted",
  };

  return (
    <Badge
      variant="outline"
      className={cn("whitespace-nowrap", config.className, className)}
    >
      {config.label}
    </Badge>
  );
}
