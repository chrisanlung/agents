"use client";

import { useState, useCallback, useTransition } from "react";
import {
  AlertCircle,
  CheckCircle2,
  ChevronLeft,
  Loader2,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { BookingStatusBadge } from "@/components/booking-status-badge";
import { CheckinScanner } from "@/components/checkin-scanner";
import type { Booking } from "@/lib/types";
import { checkInBooking } from "../actions";

type ScanState =
  | { phase: "scan" }
  | { phase: "loading_lookup"; code: string }
  | { phase: "found"; booking: Booking; code: string }
  | { phase: "not_found"; code: string }
  | { phase: "confirmed"; booking: Booking };

function formatDateTime(iso: string) {
  return new Date(iso).toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function CheckinFlow() {
  const router = useRouter();
  const [state, setState] = useState<ScanState>({ phase: "scan" });
  const [isPending, startTransition] = useTransition();

  const handleScan = useCallback(
    (code: string) => {
      if (state.phase !== "scan") return;
      setState({ phase: "loading_lookup", code });

      startTransition(async () => {
        try {
          // We call through Next.js fetch (no direct env access on client).
          // Use the `/api/booking-lookup` proxy route instead of direct API.
          const res = await fetch(
            `/api/booking-lookup?code=${encodeURIComponent(code)}`
          );
          if (res.ok) {
            const booking: Booking = await res.json();
            setState({ phase: "found", booking, code });
          } else {
            setState({ phase: "not_found", code });
          }
        } catch {
          setState({ phase: "not_found", code });
        }
      });
    },
    [state.phase]
  );

  function handleReset() {
    setState({ phase: "scan" });
  }

  function handleConfirmCheckin() {
    if (state.phase !== "found") return;
    const { booking, code } = state;

    startTransition(async () => {
      const result = await checkInBooking(booking.id, code);
      if (!result.ok) {
        toast.error(result.error ?? "Gagal check-in. Coba lagi.");
        return;
      }
      setState({
        phase: "confirmed",
        booking: result.data ?? { ...booking, status: "checked_in" },
      });
      toast.success(
        `Check-in berhasil. Selamat datang, ${booking.customer_name}!`
      );

      // Auto-reset after 1.5s
      setTimeout(() => {
        setState({ phase: "scan" });
        router.refresh();
      }, 1500);
    });
  }

  const isLookingUp =
    state.phase === "loading_lookup" ||
    (state.phase === "scan" && isPending);

  return (
    <div className="space-y-6">
      <div>
        <Button variant="ghost" size="sm" asChild className="-ml-1">
          <Link href="/booking">
            <ChevronLeft size={16} aria-hidden="true" />
            Kembali ke Daftar Booking
          </Link>
        </Button>
      </div>

      {/* Scanner (only shown in scan phase) */}
      {(state.phase === "scan" || state.phase === "loading_lookup") && (
        <CheckinScanner onScan={handleScan} isLoading={isLookingUp} />
      )}

      {/* Booking found panel */}
      {state.phase === "found" && (
        <Card className="border-emerald-200 bg-emerald-50">
          <CardHeader className="pb-2">
            <CardTitle className="flex items-center gap-2 text-emerald-800">
              <CheckCircle2 size={20} aria-hidden="true" />
              Booking Ditemukan
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div>
              <p className="text-lg font-semibold text-foreground">
                {state.booking.customer_name}
              </p>
              <p className="text-sm text-muted-foreground">
                {state.booking.customer_phone}
              </p>
            </div>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-muted-foreground">Layanan:</span>{" "}
                {state.booking.service_name}
              </div>
              <div>
                <span className="text-muted-foreground">Slot:</span>{" "}
                {formatDateTime(state.booking.scheduled_start)}
              </div>
              <div>
                <span className="text-muted-foreground">Terapis:</span>{" "}
                {state.booking.therapist_name ?? "—"}
              </div>
              <div>
                <span className="text-muted-foreground">Ruangan:</span>{" "}
                {state.booking.room_name ?? "—"}
              </div>
            </div>
            <BookingStatusBadge status={state.booking.status} />
          </CardContent>
          <CardFooter className="flex justify-end gap-2">
            <Button variant="outline" onClick={handleReset}>
              Batal
            </Button>
            <Button
              onClick={handleConfirmCheckin}
              disabled={
                isPending || state.booking.status !== "paid"
              }
            >
              {isPending ? (
                <Loader2 size={16} className="animate-spin mr-1" aria-hidden="true" />
              ) : (
                <CheckCircle2 size={14} className="mr-1.5" aria-hidden="true" />
              )}
              Konfirmasi Check-in
            </Button>
          </CardFooter>
        </Card>
      )}

      {/* Not found panel */}
      {state.phase === "not_found" && (
        <Card className="border-red-200 bg-red-50">
          <CardContent className="flex flex-col items-center gap-3 py-8 text-center">
            <AlertCircle size={32} className="text-red-500" aria-hidden="true" />
            <div className="space-y-1">
              <p className="font-medium text-red-800">Kode tidak ditemukan.</p>
              <p className="text-sm text-red-600">
                Periksa kembali kode yang dimasukkan.
              </p>
              <p className="font-mono text-sm font-medium text-red-700">
                {state.code}
              </p>
            </div>
            <Button variant="outline" onClick={handleReset}>
              Coba Lagi
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Confirmed panel */}
      {state.phase === "confirmed" && (
        <Card className="border-emerald-200 bg-emerald-50">
          <CardContent className="flex flex-col items-center gap-3 py-8 text-center">
            <CheckCircle2 size={40} className="text-emerald-600" aria-hidden="true" />
            <div className="space-y-1">
              <p className="text-lg font-semibold text-emerald-800">
                Check-in Berhasil!
              </p>
              <p className="text-sm text-emerald-700">
                Selamat datang,{" "}
                <span className="font-medium">
                  {state.booking.customer_name}
                </span>
                !
              </p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
