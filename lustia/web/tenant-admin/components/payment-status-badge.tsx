import { Badge } from "@/components/ui/badge";
import type { PaymentTransactionStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

interface PaymentStatusBadgeProps {
  status: PaymentTransactionStatus;
  className?: string;
}

const STATUS_CONFIG: Record<
  PaymentTransactionStatus,
  { label: string; className: string }
> = {
  awaiting: {
    label: "Menunggu Pembayaran",
    className: "border-border text-muted-foreground bg-muted",
  },
  paid: {
    label: "Dibayar — Diproses",
    className: "border-transparent bg-primary text-primary-foreground",
  },
  settled: {
    label: "Siap Dicairkan",
    className: "border-emerald-300 text-emerald-700 bg-emerald-50",
  },
  disbursed: {
    label: "Sudah Dicairkan",
    className: "border-border text-muted-foreground/70 bg-muted",
  },
  failed: {
    label: "Gagal",
    className: "border-transparent bg-destructive text-destructive-foreground",
  },
  expired: {
    label: "Kadaluarsa",
    className: "border-border text-muted-foreground/60 bg-muted",
  },
  voided: {
    label: "Dibatalkan",
    className: "border-transparent bg-destructive/80 text-destructive-foreground",
  },
};

export function PaymentStatusBadge({
  status,
  className,
}: PaymentStatusBadgeProps) {
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
