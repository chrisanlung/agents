"use client";

import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface PaymentFaqDialogProps {
  open: boolean;
  onClose: () => void;
}

export function PaymentFaqDialog({ open, onClose }: PaymentFaqDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(val) => !val && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Skema Pembayaran Lustia</DialogTitle>
        </DialogHeader>

        <ol className="space-y-4 text-sm">
          <li>
            <strong>1. Dibayar (iPaymu Escrow)</strong>
            <p className="mt-1 text-muted-foreground">
              Setelah pelanggan membayar via QRIS, dana masuk ke rekening escrow
              iPaymu. Status transaksi menjadi &ldquo;Dibayar — Diproses&rdquo;.
            </p>
          </li>
          <li>
            <strong>2. Siap Dicairkan (H+1)</strong>
            <p className="mt-1 text-muted-foreground">
              iPaymu mentransfer dana ke rekening Lustia setiap hari kerja
              (H+1–H+3). Status berubah menjadi &ldquo;Siap Dicairkan&rdquo;.
              Fee platform 5% dipotong dari total bruto.
            </p>
          </li>
          <li>
            <strong>3. Sudah Dicairkan (Senin pagi)</strong>
            <p className="mt-1 text-muted-foreground">
              Setiap Senin pagi, Lustia mentransfer net ke rekening bank Anda.
              Status berubah menjadi &ldquo;Sudah Ditransfer&rdquo;.
            </p>
          </li>
        </ol>

        <p className="mt-4 text-xs text-muted-foreground">
          Pertanyaan? Hubungi{" "}
          <a href="mailto:support@lustia.id" className="underline">
            support@lustia.id
          </a>
          .
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
