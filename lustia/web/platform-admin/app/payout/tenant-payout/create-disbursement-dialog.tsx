"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { formatRupiah, formatDate } from "@/lib/format";
import type { TenantPayoutSummary } from "@/lib/types";
import { createDisbursementAction } from "../actions";

interface CreateDisbursementDialogProps {
  tenant: TenantPayoutSummary;
  periodStart: string;
  periodEnd: string;
  open: boolean;
  onClose: () => void;
}

export function CreateDisbursementDialog({
  tenant,
  periodStart,
  periodEnd,
  open,
  onClose,
}: CreateDisbursementDialogProps) {
  const router = useRouter();
  const [isPending, setIsPending] = useState(false);

  const fee = Math.floor(tenant.settled_amount_idr * 0.05);
  const net = tenant.settled_amount_idr - fee;

  async function handleCreate() {
    setIsPending(true);
    const result = await createDisbursementAction({
      tenant_id: tenant.tenant_id,
      period_start: periodStart,
      period_end: periodEnd,
    });
    setIsPending(false);

    if (result.ok) {
      toast.success(
        "Pencairan berhasil dibuat. Lakukan transfer manual dan tandai selesai."
      );
      onClose();
      router.push(`/payout/disbursements/${result.data.id}`);
    } else {
      toast.error("Gagal membuat pencairan. Coba lagi.");
    }
  }

  return (
    <Dialog open={open} onOpenChange={(val) => !val && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Buat Pencairan — {tenant.tenant_name}</DialogTitle>
          <DialogDescription>
            Tinjau kalkulasi sebelum membuat pencairan.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <dl className="text-sm space-y-2">
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Periode</dt>
              <dd>
                {formatDate(periodStart)} – {formatDate(periodEnd)}
              </dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Jumlah Transaksi</dt>
              <dd className="tabular-nums">{tenant.transaction_count}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Bruto</dt>
              <dd className="tabular-nums font-medium">
                {formatRupiah(tenant.settled_amount_idr)}
              </dd>
            </div>
            <div className="flex justify-between text-destructive">
              <dt>Fee Platform 5%</dt>
              <dd className="tabular-nums">− {formatRupiah(fee)}</dd>
            </div>
            <div className="flex justify-between border-t pt-2">
              <dt className="font-semibold">Net yang Ditransfer</dt>
              <dd className="tabular-nums font-bold text-primary">
                {formatRupiah(net)}
              </dd>
            </div>
          </dl>

          <p className="text-xs text-muted-foreground">
            Pencairan akan mencakup semua transaksi settled yang belum dicairkan
            pada periode ini.
          </p>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isPending}>
            Batal
          </Button>
          <Button onClick={handleCreate} disabled={isPending}>
            {isPending && (
              <Loader2 className="animate-spin mr-2" size={14} aria-hidden="true" />
            )}
            Buat &amp; Lanjut ke Transfer Manual
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
