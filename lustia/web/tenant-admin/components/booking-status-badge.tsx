import { Badge } from "@/components/ui/badge";
import type { BookingStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

interface BookingStatusBadgeProps {
  status: BookingStatus;
  className?: string;
}

const STATUS_CONFIG: Record<
  BookingStatus,
  { label: string; className: string }
> = {
  pending_payment: {
    label: "Menunggu Bayar",
    className: "border-amber-300 text-amber-700 bg-amber-50",
  },
  paid: {
    label: "Terbayar",
    className: "border-emerald-300 text-emerald-700 bg-emerald-50",
  },
  checked_in: {
    label: "Check-in",
    className:
      "border-transparent bg-primary text-primary-foreground",
  },
  completed: {
    label: "Selesai",
    className: "border-border text-muted-foreground bg-muted",
  },
  cancelled: {
    label: "Dibatalkan",
    className:
      "border-transparent bg-destructive text-destructive-foreground",
  },
  no_show: {
    label: "Tidak Hadir",
    className: "border-transparent bg-amber-500 text-white",
  },
  expired: {
    label: "Kedaluwarsa",
    className: "border-border text-muted-foreground/60 bg-muted",
  },
};

export function BookingStatusBadge({
  status,
  className,
}: BookingStatusBadgeProps) {
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
