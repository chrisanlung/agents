"use client";

import { useState, useTransition } from "react";
import { Loader2, XCircle } from "lucide-react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
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
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import type { Booking } from "@/lib/types";
import { cancelBooking } from "../actions";

interface BookingDetailActionsProps {
  booking: Booking;
}

export function BookingDetailActions({ booking }: BookingDetailActionsProps) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [cancelOpen, setCancelOpen] = useState(false);
  const [cancelReason, setCancelReason] = useState("");

  const canCancel =
    booking.status === "paid" || booking.status === "checked_in";

  const isTerminal = ["completed", "cancelled", "no_show", "expired"].includes(
    booking.status
  );

  function handleCancel() {
    if (!cancelReason.trim()) return;
    setCancelOpen(false);
    startTransition(async () => {
      const result = await cancelBooking(booking.id, cancelReason.trim());
      if (!result.ok) {
        toast.error(
          result.error ?? result.errors?.reason ?? "Gagal membatalkan booking."
        );
        return;
      }
      toast.success("Booking berhasil dibatalkan.");
      setCancelReason("");
      router.refresh();
    });
  }

  return (
    <>
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Tindakan</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {isTerminal ? (
            <p className="text-sm text-muted-foreground">
              Tidak ada tindakan tersedia.
            </p>
          ) : canCancel ? (
            <Button
              variant="outline"
              className="w-full border-destructive text-destructive hover:bg-destructive/10"
              disabled={isPending}
              onClick={() => setCancelOpen(true)}
            >
              {isPending ? (
                <Loader2 size={16} className="animate-spin mr-1" aria-hidden="true" />
              ) : (
                <XCircle size={16} className="mr-1.5" aria-hidden="true" />
              )}
              Batalkan Booking
            </Button>
          ) : (
            <p className="text-sm text-muted-foreground">
              Tidak ada tindakan tersedia untuk status ini.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Cancel dialog */}
      <AlertDialog open={cancelOpen} onOpenChange={setCancelOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Batalkan Booking?</AlertDialogTitle>
            <AlertDialogDescription>
              Booking ini akan dibatalkan. Tindakan ini tidak dapat dibatalkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="px-6 pb-2">
            <Label htmlFor="cancel-reason" className="text-sm font-medium">
              Alasan Pembatalan
            </Label>
            <Textarea
              id="cancel-reason"
              rows={3}
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
              placeholder="Tuliskan alasan pembatalan..."
              className="mt-1.5"
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setCancelReason("")}>
              Batal
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleCancel}
              disabled={!cancelReason.trim()}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50"
            >
              Ya, Batalkan
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
