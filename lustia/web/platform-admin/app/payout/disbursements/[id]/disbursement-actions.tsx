"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { DisbursementStatus } from "@/lib/types";
import {
  startProcessingAction,
  markTransferredAction,
  markFailedAction,
  cancelDisbursementAction,
} from "../../actions";

interface DisbursementActionsProps {
  id: string;
  status: DisbursementStatus;
  tenantName: string;
}

export function DisbursementActions({
  id,
  status,
  tenantName,
}: DisbursementActionsProps) {
  const router = useRouter();
  const [isPending, setIsPending] = useState(false);

  // Dialog states
  const [transferDialogOpen, setTransferDialogOpen] = useState(false);
  const [failDialogOpen, setFailDialogOpen] = useState(false);
  const [cancelDialogOpen, setCancelDialogOpen] = useState(false);

  // Transfer dialog fields
  const [bankRef, setBankRef] = useState("");
  const [transferNotes, setTransferNotes] = useState("");
  const [transferConfirmed, setTransferConfirmed] = useState(false);

  // Fail dialog fields
  const [reason, setReason] = useState("");
  const [failConfirmed, setFailConfirmed] = useState(false);

  // Cancel dialog fields
  const [cancelConfirmed, setCancelConfirmed] = useState(false);

  async function handleStartProcessing() {
    setIsPending(true);
    const result = await startProcessingAction(id);
    setIsPending(false);
    if (result.ok) {
      toast.success("Pencairan dipindahkan ke status 'Sedang Diproses'.");
      router.refresh();
    } else {
      toast.error("Gagal memperbarui status. Coba lagi.");
    }
  }

  async function handleTransfer() {
    setIsPending(true);
    const result = await markTransferredAction(
      id,
      bankRef || undefined,
      transferNotes || undefined
    );
    setIsPending(false);
    setTransferDialogOpen(false);
    if (result.ok) {
      toast.success("Pencairan ditandai sudah ditransfer.");
      router.refresh();
    } else {
      toast.error("Gagal memperbarui status. Coba lagi.");
    }
  }

  async function handleFail() {
    setIsPending(true);
    const result = await markFailedAction(id, reason);
    setIsPending(false);
    setFailDialogOpen(false);
    if (result.ok) {
      toast.success("Pencairan ditandai gagal.");
      router.refresh();
    } else {
      toast.error("Gagal memperbarui status. Coba lagi.");
    }
  }

  async function handleCancel() {
    setIsPending(true);
    const result = await cancelDisbursementAction(id);
    setIsPending(false);
    setCancelDialogOpen(false);
    if (result.ok) {
      toast.success("Pencairan dibatalkan.");
      router.refresh();
    } else {
      toast.error("Gagal memperbarui status. Coba lagi.");
    }
  }

  const isTerminal = ["transferred", "failed", "cancelled"].includes(status);

  return (
    <>
      <Card className="sticky top-4">
        <CardContent className="pt-4 space-y-2">
          <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-3">
            Tindakan
          </p>

          {isTerminal ? (
            <p className="text-sm text-muted-foreground text-center">
              Tidak ada tindakan lebih lanjut.
            </p>
          ) : (
            <>
              {status === "pending" && (
                <Button
                  className="w-full"
                  disabled={isPending}
                  onClick={handleStartProcessing}
                  aria-label={`Mulai proses pencairan untuk ${tenantName}`}
                >
                  {isPending && (
                    <Loader2
                      className="animate-spin mr-2"
                      size={14}
                      aria-hidden="true"
                    />
                  )}
                  Mulai Proses
                </Button>
              )}

              {status === "processing" && (
                <Button
                  className="w-full"
                  disabled={isPending}
                  onClick={() => {
                    setTransferConfirmed(false);
                    setBankRef("");
                    setTransferNotes("");
                    setTransferDialogOpen(true);
                  }}
                  aria-label={`Tandai sudah transfer untuk ${tenantName}`}
                >
                  Tandai Sudah Transfer
                </Button>
              )}

              {(status === "pending" || status === "processing") && (
                <Button
                  variant="outline"
                  className="w-full"
                  disabled={isPending}
                  onClick={() => {
                    setFailConfirmed(false);
                    setReason("");
                    setFailDialogOpen(true);
                  }}
                >
                  Tandai Gagal
                </Button>
              )}

              {status === "pending" && (
                <Button
                  variant="ghost"
                  className="w-full text-destructive hover:text-destructive"
                  disabled={isPending}
                  onClick={() => {
                    setCancelConfirmed(false);
                    setCancelDialogOpen(true);
                  }}
                >
                  Batalkan
                </Button>
              )}
            </>
          )}
        </CardContent>
      </Card>

      {/* "Tandai Sudah Transfer" AlertDialog */}
      <AlertDialog open={transferDialogOpen} onOpenChange={setTransferDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Tandai Sudah Transfer?</AlertDialogTitle>
            <AlertDialogDescription>
              Konfirmasi bahwa transfer manual ke rekening tenant telah
              dilakukan. Tindakan ini akan dicatat di audit log dan tidak dapat
              dibatalkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-3 py-2">
            <div>
              <Label htmlFor="bank-ref">Referensi Bank (opsional)</Label>
              <Input
                id="bank-ref"
                value={bankRef}
                onChange={(e) => setBankRef(e.target.value)}
                placeholder="Contoh: TF2026042600001"
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="transfer-notes">Catatan (opsional)</Label>
              <Textarea
                id="transfer-notes"
                value={transferNotes}
                onChange={(e) => setTransferNotes(e.target.value)}
                rows={2}
                className="mt-1"
              />
            </div>
            <div className="flex items-center gap-2 pt-1">
              <Checkbox
                id="confirm-audit-transfer"
                checked={transferConfirmed}
                onCheckedChange={(val) =>
                  setTransferConfirmed(val === true)
                }
              />
              <Label htmlFor="confirm-audit-transfer" className="text-sm">
                Saya konfirmasi tindakan ini akan dicatat di audit log.
              </Label>
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              disabled={!transferConfirmed || isPending}
              onClick={handleTransfer}
            >
              {isPending && (
                <Loader2
                  className="animate-spin mr-2"
                  size={14}
                  aria-hidden="true"
                />
              )}
              Ya, Tandai Sudah Transfer
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* "Tandai Gagal" AlertDialog */}
      <AlertDialog open={failDialogOpen} onOpenChange={setFailDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Tandai Gagal?</AlertDialogTitle>
            <AlertDialogDescription>
              Pencairan akan ditandai gagal. Transaksi settled tetap tersimpan
              dan dapat dicairkan kembali.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-3 py-2">
            <div>
              <Label htmlFor="fail-reason">
                Alasan Kegagalan (wajib)
              </Label>
              <Textarea
                id="fail-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={2}
                placeholder="Jelaskan alasan kegagalan..."
                className="mt-1"
              />
            </div>
            <div className="flex items-center gap-2 pt-1">
              <Checkbox
                id="confirm-audit-fail"
                checked={failConfirmed}
                onCheckedChange={(val) => setFailConfirmed(val === true)}
              />
              <Label htmlFor="confirm-audit-fail" className="text-sm">
                Saya konfirmasi tindakan ini akan dicatat di audit log.
              </Label>
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              disabled={!failConfirmed || reason.length < 5 || isPending}
              onClick={handleFail}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isPending && (
                <Loader2
                  className="animate-spin mr-2"
                  size={14}
                  aria-hidden="true"
                />
              )}
              Ya, Tandai Gagal
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* "Batalkan" AlertDialog */}
      <AlertDialog open={cancelDialogOpen} onOpenChange={setCancelDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Batalkan Pencairan?</AlertDialogTitle>
            <AlertDialogDescription>
              Pencairan untuk {tenantName} akan dibatalkan. Transaksi settled
              tetap tersimpan dan dapat dicairkan di periode berikutnya.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-3 py-2">
            <div className="flex items-center gap-2 pt-1">
              <Checkbox
                id="confirm-audit-cancel"
                checked={cancelConfirmed}
                onCheckedChange={(val) => setCancelConfirmed(val === true)}
              />
              <Label htmlFor="confirm-audit-cancel" className="text-sm">
                Saya konfirmasi tindakan ini akan dicatat di audit log.
              </Label>
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              disabled={!cancelConfirmed || isPending}
              onClick={handleCancel}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isPending && (
                <Loader2
                  className="animate-spin mr-2"
                  size={14}
                  aria-hidden="true"
                />
              )}
              Ya, Batalkan Pencairan
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
