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
    className: "border-border text-muted-foreground bg-muted",
  },
  processing: {
    label: "Sedang Diproses",
    className: "border-amber-300 text-amber-700 bg-amber-50",
  },
  transferred: {
    label: "Sudah Ditransfer",
    className: "border-emerald-300 text-emerald-700 bg-emerald-50",
  },
  failed: {
    label: "Gagal",
    className: "border-transparent bg-destructive text-destructive-foreground",
  },
  cancelled: {
    label: "Dibatalkan",
    className: "border-border text-muted-foreground/60 bg-muted",
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
