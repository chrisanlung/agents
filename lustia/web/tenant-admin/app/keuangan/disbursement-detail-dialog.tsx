"use client";

import { useState } from "react";
import { toast } from "sonner";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { DisbursementStatusBadge } from "@/components/disbursement-status-badge";
import { AlertCircle } from "lucide-react";
import { apiFetch } from "@/lib/api";
import { formatRupiah, formatDate } from "@/lib/format";
import type { DisbursementDetail, TenantDisbursement } from "@/lib/types";

interface DisbursementDetailDialogProps {
  disbursement: TenantDisbursement;
  open: boolean;
  onClose: () => void;
}

export function DisbursementDetailDialog({
  disbursement,
  open,
  onClose,
}: DisbursementDetailDialogProps) {
  const [detail, setDetail] = useState<DisbursementDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  async function fetchDetail() {
    if (detail) return; // already loaded
    setLoading(true);
    setError(false);
    try {
      const data = await apiFetch<DisbursementDetail>(
        `/tenant/finance/disbursements/${disbursement.id}`,
        {},
        { auth: true }
      );
      setDetail(data);
    } catch {
      setError(true);
      toast.error("Gagal memuat detail pencairan. Coba lagi.");
    } finally {
      setLoading(false);
    }
  }

  function handleOpenChange(val: boolean) {
    if (val) {
      fetchDetail();
    } else {
      onClose();
    }
  }

  const totals = detail?.transactions.reduce(
    (acc, tx) => ({
      gross: acc.gross + tx.received_amount_idr,
      fee: acc.fee + tx.platform_fee_idr,
      net: acc.net + tx.tenant_net_idr,
    }),
    { gross: 0, fee: 0, net: 0 }
  );

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Detail Pencairan</DialogTitle>
          <DialogDescription>
            Periode {formatDate(disbursement.period_start)} –{" "}
            {formatDate(disbursement.period_end)}
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
          >
            <AlertCircle
              size={16}
              className="mt-0.5 shrink-0 text-red-600"
              aria-hidden="true"
            />
            <div>
              <span>Gagal memuat detail pencairan. Coba lagi.</span>
              <Button
                variant="outline"
                size="sm"
                className="ml-2"
                onClick={() => {
                  setDetail(null);
                  setError(false);
                  fetchDetail();
                }}
              >
                Coba Lagi
              </Button>
            </div>
          </div>
        )}

        {/* Summary row */}
        <div className="grid grid-cols-3 gap-4 mb-4">
          <div className="rounded-lg border bg-card p-3 text-center">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Bruto
            </p>
            <p
              className="mt-1 text-lg font-bold tabular-nums"
              aria-label={formatRupiah(disbursement.gross_amount_idr)}
            >
              {formatRupiah(disbursement.gross_amount_idr)}
            </p>
          </div>
          <div className="rounded-lg border bg-card p-3 text-center">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Fee 5%
            </p>
            <p
              className="mt-1 text-lg font-bold tabular-nums text-destructive"
              aria-label={formatRupiah(disbursement.platform_fee_idr)}
            >
              − {formatRupiah(disbursement.platform_fee_idr)}
            </p>
          </div>
          <div className="rounded-lg border bg-card p-3 text-center">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Net
            </p>
            <p
              className="mt-1 text-lg font-bold tabular-nums text-primary"
              aria-label={formatRupiah(disbursement.net_amount_idr)}
            >
              {formatRupiah(disbursement.net_amount_idr)}
            </p>
          </div>
        </div>

        {/* Transfer details */}
        <dl className="text-sm space-y-2 mb-4">
          <div className="flex items-center justify-between">
            <dt className="text-muted-foreground">Status</dt>
            <dd>
              <DisbursementStatusBadge status={disbursement.status} />
            </dd>
          </div>
          <div className="flex items-center justify-between">
            <dt className="text-muted-foreground">Tanggal Transfer</dt>
            <dd>{formatDate(disbursement.transferred_at)}</dd>
          </div>
          <div className="flex items-center justify-between">
            <dt className="text-muted-foreground">Ditransfer oleh</dt>
            <dd>{disbursement.transferred_by_name ?? "—"}</dd>
          </div>
          <div className="flex items-center justify-between">
            <dt className="text-muted-foreground">Ref Bank</dt>
            <dd
              className="truncate max-w-[200px]"
              title={disbursement.bank_reference ?? undefined}
            >
              {disbursement.bank_reference
                ? disbursement.bank_reference.substring(0, 12) +
                  (disbursement.bank_reference.length > 12 ? "…" : "")
                : "—"}
            </dd>
          </div>
          <div className="flex items-center justify-between">
            <dt className="text-muted-foreground">Catatan</dt>
            <dd>{disbursement.notes ?? "—"}</dd>
          </div>
        </dl>

        {/* Transaction breakdown */}
        {loading && (
          <div className="flex h-20 items-center justify-center text-sm text-muted-foreground">
            Memuat transaksi...
          </div>
        )}

        {!loading && detail && detail.transactions.length > 0 && (
          <div className="max-h-64 overflow-y-auto rounded-md border">
            <Table>
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead>Tanggal</TableHead>
                  <TableHead>Booking</TableHead>
                  <TableHead className="tabular-nums">Bruto</TableHead>
                  <TableHead className="tabular-nums">Fee</TableHead>
                  <TableHead className="tabular-nums">Net</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="text-sm">
                {detail.transactions.map((tx) => (
                  <TableRow key={tx.id}>
                    <TableCell className="text-muted-foreground tabular-nums">
                      {formatDate(tx.paid_at)}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-primary">
                      {tx.booking_code}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {formatRupiah(tx.received_amount_idr)}
                    </TableCell>
                    <TableCell className="tabular-nums text-muted-foreground">
                      {formatRupiah(tx.platform_fee_idr)}
                    </TableCell>
                    <TableCell className="tabular-nums font-medium">
                      {formatRupiah(tx.tenant_net_idr)}
                    </TableCell>
                  </TableRow>
                ))}
                {/* Footer totals */}
                {totals && (
                  <TableRow className="border-t bg-muted/20 font-semibold">
                    <TableCell colSpan={2}>Total</TableCell>
                    <TableCell className="tabular-nums">
                      {formatRupiah(totals.gross)}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {formatRupiah(totals.fee)}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {formatRupiah(totals.net)}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Tutup
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
