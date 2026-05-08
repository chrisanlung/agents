"use client";

import { useState } from "react";
import { AlertCircle } from "lucide-react";

import {
  Dialog,
  DialogContent,
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
import { fetchSettlementBatchDetail } from "../actions";
import { formatRupiah, formatDate } from "@/lib/format";
import type { SettlementBatch, SettlementBatchDetail } from "@/lib/types";

const PROVIDER_LABELS: Record<string, string> = {
  ipaymu: "iPaymu QRIS",
  dummy: "Simulasi",
};

const ISSUE_LABELS: Record<string, string> = {
  not_in_lustia: "Ada di iPaymu, tidak ada di Lustia",
  not_in_provider: "Ada di Lustia, tidak ada di iPaymu",
};

interface BatchDetailDialogProps {
  batch: SettlementBatch;
  open: boolean;
  onClose: () => void;
}

export function BatchDetailDialog({
  batch,
  open,
  onClose,
}: BatchDetailDialogProps) {
  const [detail, setDetail] = useState<SettlementBatchDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [mismatchOpen, setMismatchOpen] = useState(false);

  async function fetchDetail() {
    if (detail) return;
    setLoading(true);
    setError(false);
    const result = await fetchSettlementBatchDetail(batch.id);
    if (result.ok) {
      setDetail(result.data);
    } else {
      setError(true);
    }
    setLoading(false);
  }

  function handleOpenChange(val: boolean) {
    if (val) {
      fetchDetail();
    } else {
      onClose();
    }
  }

  const hasMismatches = (detail?.mismatches?.length ?? 0) > 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>
            Detail Batch {formatDate(batch.settled_at)}
          </DialogTitle>
        </DialogHeader>

        {/* POU-A6: Mismatch warning banner */}
        {!loading && detail && hasMismatches && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 mb-4"
          >
            <AlertCircle
              size={16}
              className="shrink-0 mt-0.5 text-red-600"
              aria-hidden="true"
            />
            <div>
              <p className="font-medium">
                {detail.mismatches.length} transaksi tidak cocok — investigasi
                manual diperlukan.
              </p>
              <button
                className="mt-1 text-xs underline"
                onClick={() => setMismatchOpen(!mismatchOpen)}
              >
                {mismatchOpen ? "Sembunyikan" : "Lihat detail"}
              </button>
              {mismatchOpen && (
                <Table className="mt-2">
                  <TableHeader>
                    <TableRow>
                      <TableHead>Provider Reference</TableHead>
                      <TableHead>Nominal</TableHead>
                      <TableHead>Masalah</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {detail.mismatches.map((m) => (
                      <TableRow key={m.provider_reference}>
                        <TableCell className="font-mono text-xs">
                          {m.provider_reference}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {formatRupiah(m.amount_idr)}
                        </TableCell>
                        <TableCell>
                          {ISSUE_LABELS[m.issue] ?? m.issue}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          </div>
        )}

        {error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 mb-4"
          >
            <AlertCircle
              size={16}
              className="shrink-0 mt-0.5 text-red-600"
              aria-hidden="true"
            />
            <span>Gagal memuat detail batch. Coba lagi.</span>
          </div>
        )}

        {loading && (
          <div className="flex h-20 items-center justify-center text-sm text-muted-foreground">
            Memuat transaksi...
          </div>
        )}

        {!loading && detail && (
          <div className="max-h-80 overflow-y-auto rounded-md border">
            <Table>
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead>Tanggal</TableHead>
                  <TableHead>Provider Reference</TableHead>
                  <TableHead>Tenant</TableHead>
                  <TableHead className="tabular-nums">Bruto</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="text-sm">
                {detail.transactions.map((tx) => (
                  <TableRow key={tx.id}>
                    <TableCell className="tabular-nums text-muted-foreground">
                      {formatDate(tx.settled_at)}
                    </TableCell>
                    <TableCell className="font-mono text-xs truncate max-w-[160px]">
                      {tx.provider_reference}
                    </TableCell>
                    <TableCell>{tx.tenant_name}</TableCell>
                    <TableCell className="tabular-nums">
                      {formatRupiah(tx.received_amount_idr)}
                    </TableCell>
                    <TableCell>{tx.status}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        <p className="text-xs text-muted-foreground mt-1">
          Provider:{" "}
          <span className="font-medium">
            {PROVIDER_LABELS[detail?.provider ?? batch.provider] ??
              batch.provider}
          </span>{" "}
          · {batch.transaction_count} transaksi ·{" "}
          {formatRupiah(batch.total_amount_idr)}
        </p>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Tutup
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
