"use client";

import { useState, useTransition } from "react";
import {
  CheckCircle,
  Eye,
  Loader2,
  QrCode,
  UserX,
} from "lucide-react";
import Link from "next/link";
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
import type { Booking } from "@/lib/types";
import { markComplete, markNoShow } from "./actions";

interface BookingRowActionsProps {
  booking: Booking;
}

export function BookingRowActions({ booking }: BookingRowActionsProps) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [noShowOpen, setNoShowOpen] = useState(false);

  function handleMarkComplete() {
    startTransition(async () => {
      const result = await markComplete(booking.id);
      if (!result.ok) {
        toast.error(result.error ?? "Gagal menandai selesai.");
        return;
      }
      toast.success("Booking ditandai selesai.");
      router.refresh();
    });
  }

  function handleNoShow() {
    setNoShowOpen(false);
    startTransition(async () => {
      const result = await markNoShow(booking.id);
      if (!result.ok) {
        toast.error(result.error ?? "Gagal menandai tidak hadir.");
        return;
      }
      toast.success("Booking ditandai tidak hadir.");
      router.refresh();
    });
  }

  return (
    <>
      <div className="flex items-center gap-1">
        {booking.status === "paid" && (
          <Button variant="outline" size="sm" asChild>
            <Link href={`/booking/checkin?code=${booking.code}`}>
              <QrCode size={14} className="mr-1" aria-hidden="true" />
              Check-in
            </Link>
          </Button>
        )}
        {booking.status === "checked_in" && (
          <Button
            variant="outline"
            size="sm"
            disabled={isPending}
            onClick={handleMarkComplete}
            aria-label={`Tandai selesai: ${booking.customer_name}`}
          >
            {isPending ? (
              <Loader2 size={14} className="animate-spin" aria-hidden="true" />
            ) : (
              <CheckCircle size={14} className="mr-1" aria-hidden="true" />
            )}
            Selesai
          </Button>
        )}
        {booking.status === "paid" && (
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            disabled={isPending}
            onClick={() => setNoShowOpen(true)}
            title="Tandai tidak hadir"
            aria-label={`Tandai tidak hadir: ${booking.customer_name}`}
          >
            <UserX size={14} aria-hidden="true" />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          asChild
          aria-label={`Detail booking: ${booking.customer_name}`}
        >
          <Link href={`/booking/${booking.id}`}>
            <Eye size={14} aria-hidden="true" />
          </Link>
        </Button>
      </div>

      <AlertDialog open={noShowOpen} onOpenChange={setNoShowOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Tandai Tidak Hadir?</AlertDialogTitle>
            <AlertDialogDescription>
              Pelanggan{" "}
              <span className="font-medium">{booking.customer_name}</span> akan
              ditandai tidak hadir. Slot tidak akan dibebaskan secara otomatis.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleNoShow}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Ya, Tidak Hadir
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
