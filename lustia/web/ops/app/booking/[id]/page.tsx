import type { Metadata } from "next";
import Link from "next/link";
import { ChevronLeft, Copy } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type { Booking } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { BookingStatusBadge } from "@/components/booking-status-badge";
import { BookingDetailActions } from "./booking-detail-actions";

export const metadata: Metadata = {
  title: "Detail Booking",
};

interface PageProps {
  params: Promise<{ id: string }>;
}

function formatDateTime(iso: string | null) {
  if (!iso) return "—";
  return new Date(iso).toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function formatPrice(idr: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(idr);
}

const PAYMENT_METHOD_LABELS: Record<string, string> = {
  midtrans: "Midtrans",
  paid_at_venue: "Bayar di Tempat",
};

export default async function BookingDetailPage({ params }: PageProps) {
  const { id } = await params;

  let booking!: Booking;
  try {
    booking = await apiFetch<Booking>(
      `/tenant/bookings/${id}`,
      {},
      { auth: true }
    );
  } catch (err) {
    await handleApiError(err);
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <Button variant="ghost" size="sm" asChild className="-ml-1">
            <Link href="/booking">
              <ChevronLeft size={16} aria-hidden="true" />
              Kembali ke Daftar Booking
            </Link>
          </Button>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
              Detail Booking
            </h1>
            <span className="rounded-md border border-border bg-muted px-2.5 py-0.5 font-mono text-sm font-medium">
              {booking.code}
            </span>
          </div>
        </div>
        <BookingStatusBadge status={booking.status} className="text-sm" />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Left: booking info */}
        <div className="space-y-4">
          {/* Customer */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Pelanggan</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <InfoRow label="Nama" value={booking.customer_name} bold />
              <InfoRow
                label="WhatsApp"
                value={booking.customer_phone}
                copyable
              />
              <InfoRow
                label="Email"
                value={booking.customer_email}
                copyable
              />
            </CardContent>
          </Card>

          {/* Booking details */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Detail Booking</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <InfoRow label="Layanan" value={booking.service_name} />
              {booking.addons.length > 0 && (
                <div>
                  <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Tambahan
                  </span>
                  <ul className="mt-1 space-y-0.5 pl-2">
                    {booking.addons.map((a) => (
                      <li key={a.addon_id} className="text-foreground">
                        {a.name}{" "}
                        <span className="text-muted-foreground">
                          (+{formatPrice(a.price_idr)})
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              <InfoRow
                label="Slot Mulai"
                value={formatDateTime(booking.scheduled_start)}
              />
              <InfoRow
                label="Slot Selesai"
                value={formatDateTime(booking.scheduled_end)}
              />
              <InfoRow
                label="Terapis"
                value={booking.therapist_name ?? "—"}
              />
              <InfoRow label="Ruangan" value={booking.room_name ?? "—"} />
            </CardContent>
          </Card>

          {/* Payment */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Pembayaran</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <InfoRow
                label="Total"
                value={formatPrice(booking.total_price_idr)}
                bold
              />
              <InfoRow
                label="Metode"
                value={
                  booking.payment_method
                    ? (PAYMENT_METHOD_LABELS[booking.payment_method] ??
                      booking.payment_method)
                    : "—"
                }
              />
              {booking.payment_reference && (
                <InfoRow
                  label="Referensi"
                  value={booking.payment_reference}
                  copyable
                />
              )}
              <InfoRow
                label="Dibayar pada"
                value={formatDateTime(booking.paid_at)}
              />
            </CardContent>
          </Card>
        </div>

        {/* Right: actions + timeline */}
        <div className="space-y-4">
          <BookingDetailActions booking={booking} />

          {/* Status timeline */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Riwayat Status</CardTitle>
            </CardHeader>
            <CardContent>
              <StatusTimeline booking={booking} />
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

function InfoRow({
  label,
  value,
  bold,
  copyable,
}: {
  label: string;
  value: string;
  bold?: boolean;
  copyable?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-2">
      <span className="shrink-0 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </span>
      <span
        className={`text-right text-sm ${bold ? "font-semibold text-foreground" : "text-foreground"} ${copyable ? "flex items-center gap-1" : ""}`}
      >
        {value}
        {copyable && (
          <Copy
            size={12}
            className="text-muted-foreground opacity-50"
            aria-hidden="true"
          />
        )}
      </span>
    </div>
  );
}

function StatusTimeline({ booking }: { booking: Booking }) {
  const entries: { label: string; timestamp: string | null }[] = [
    { label: "Booking dibuat", timestamp: booking.created_at },
    ...(booking.paid_at
      ? [{ label: "Pembayaran dikonfirmasi", timestamp: booking.paid_at }]
      : []),
    ...(booking.checked_in_at
      ? [{ label: "Check-in", timestamp: booking.checked_in_at }]
      : []),
    ...(booking.completed_at
      ? [{ label: "Selesai", timestamp: booking.completed_at }]
      : []),
    ...(booking.cancelled_at
      ? [
          {
            label: `Dibatalkan${booking.cancel_reason ? `: ${booking.cancel_reason}` : ""}`,
            timestamp: booking.cancelled_at,
          },
        ]
      : []),
  ].filter((e): e is { label: string; timestamp: string } => e.timestamp !== null);

  if (entries.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">Belum ada riwayat.</p>
    );
  }

  return (
    <ol className="space-y-3">
      {entries.map((entry, idx) => (
        <li key={idx} className="flex items-start gap-3 text-sm">
          <span
            className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-primary"
            aria-hidden="true"
          />
          <div>
            <p className="font-medium text-foreground">{entry.label}</p>
            <p className="text-xs text-muted-foreground">
              {formatDateTime(entry.timestamp)}
            </p>
          </div>
        </li>
      ))}
    </ol>
  );
}
