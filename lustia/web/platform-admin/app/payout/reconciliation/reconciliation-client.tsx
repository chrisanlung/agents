"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Loader2, RefreshCw } from "lucide-react";

import { Button } from "@/components/ui/button";
import { relativeTime } from "@/lib/relative-time";
import { reconcileAction } from "../actions";

interface ReconciliationClientProps {
  alreadyReconciledToday: boolean;
  lastBatchAt: string | null;
}

export function ReconciliationClient({
  alreadyReconciledToday,
  lastBatchAt,
}: ReconciliationClientProps) {
  const [isPending, setIsPending] = useState(false);

  async function handleReconcile() {
    setIsPending(true);
    const today = new Date().toISOString().slice(0, 10);
    const result = await reconcileAction(today);
    setIsPending(false);

    if (result.ok) {
      toast.success(
        `Laporan iPaymu berhasil ditarik. ${result.data.transaction_count} transaksi disettlement.`
      );
    } else {
      toast.error("Gagal menarik laporan iPaymu. Coba lagi.");
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <Button
        disabled={alreadyReconciledToday || isPending}
        onClick={handleReconcile}
        className="gap-2"
      >
        {isPending ? (
          <Loader2 size={16} className="animate-spin" aria-hidden="true" />
        ) : (
          <RefreshCw size={16} aria-hidden="true" />
        )}
        {isPending ? "Menarik laporan..." : "Tarik Laporan iPaymu Hari Ini"}
      </Button>
      {alreadyReconciledToday && lastBatchAt && (
        <p className="text-xs text-muted-foreground">
          Terakhir: {relativeTime(lastBatchAt)}
        </p>
      )}
    </div>
  );
}
