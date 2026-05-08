import type { Metadata } from "next";
import Link from "next/link";
import {
  AlertCircle,
  Calendar,
  Plus,
  QrCode,
} from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type {
  BookingListResponse,
  Booking,
  TherapistListResponse,
  Therapist,
} from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { FilterBar } from "@/components/filter-bar";
import { FilterDate } from "@/components/filter-date";
import { FilterSelect } from "@/components/filter-select";
import { Pagination } from "@/components/pagination";
import { BookingStatusBadge } from "@/components/booking-status-badge";
import { BookingRowActions } from "./booking-row-actions";

export const metadata: Metadata = {
  title: "Booking",
};

const PAGE_SIZE = 10;

interface PageProps {
  searchParams: Promise<{
    status?: string;
    date?: string;
    therapist_id?: string;
    page?: string;
  }>;
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString("id-ID", {
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

export default async function BookingPage({ searchParams }: PageProps) {
  const {
    status,
    date: dateParam,
    therapist_id,
    page: pageParam,
  } = await searchParams;

  const today = new Date().toISOString().slice(0, 10);
  const selectedDate = dateParam ?? today;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  let bookings: Booking[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let therapists: Therapist[] = [];
  let fetchError = false;

  const params = new URLSearchParams({
    page: String(pageNum),
    limit: String(PAGE_SIZE),
    from: selectedDate,
    to: selectedDate,
  });
  if (status) params.set("status", status);
  if (therapist_id) params.set("therapist_id", therapist_id);

  try {
    const [bookingRes, therapistRes] = await Promise.allSettled([
      apiFetch<BookingListResponse>(
        `/tenant/bookings?${params.toString()}`,
        {},
        { auth: true }
      ),
      apiFetch<TherapistListResponse>(
        "/tenant/therapists?limit=200&is_active=true",
        {},
        { auth: true }
      ),
    ]);

    if (bookingRes.status === "fulfilled") {
      bookings = bookingRes.value.data;
      totalCount = bookingRes.value.total_count;
      totalPages = bookingRes.value.total_pages;
      currentPage = bookingRes.value.page;
    } else {
      const err = bookingRes.reason;
      if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
        await handleApiError(err);
      }
      fetchError = true;
    }

    if (therapistRes.status === "fulfilled") {
      therapists = therapistRes.value.data;
    }
  } catch (err) {
    await handleApiError(err);
  }

  const filterActive = !!(status || therapist_id || dateParam);

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
              Booking Hari Ini
            </h1>
            {totalCount > 0 && (
              <span className="rounded-full bg-primary px-2.5 py-0.5 text-xs font-semibold text-primary-foreground">
                {totalCount} booking
              </span>
            )}
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            {formatDate(selectedDate)}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" asChild>
            <Link href="/booking/checkin">
              <QrCode size={16} aria-hidden="true" />
              Check-in
            </Link>
          </Button>
          <Button asChild>
            <Link href="/booking/new">
              <Plus size={16} aria-hidden="true" />
              Buat Booking
            </Link>
          </Button>
        </div>
      </div>

      {/* Filter bar */}
      <FilterBar isActive={filterActive} resetHref="/booking">
        <FilterDate label="Tanggal" name="date" current={selectedDate} />
        <FilterSelect
          label="Status"
          name="status"
          current={status}
          options={[
            { value: "", label: "Semua" },
            { value: "pending_payment", label: "Menunggu Bayar" },
            { value: "paid", label: "Terbayar" },
            { value: "checked_in", label: "Check-in" },
            { value: "completed", label: "Selesai" },
            { value: "no_show", label: "Tidak Hadir" },
            { value: "cancelled", label: "Dibatalkan" },
            { value: "expired", label: "Kedaluwarsa" },
          ]}
        />
        {therapists.length > 0 && (
          <FilterSelect
            label="Terapis"
            name="therapist_id"
            current={therapist_id}
            options={[
              { value: "", label: "Semua" },
              ...therapists.map((t) => ({
                value: t.id,
                label: t.full_name,
              })),
            ]}
          />
        )}
      </FilterBar>

      {/* Error state */}
      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle
            size={16}
            className="mt-0.5 shrink-0 text-red-600"
            aria-hidden="true"
          />
          <span>
            Gagal memuat daftar booking. Muat ulang halaman.
          </span>
        </div>
      )}

      {/* Table */}
      <Card>
        <CardContent className="overflow-x-auto p-0">
          {!fetchError && bookings.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Calendar
                size={48}
                className="text-muted-foreground/40"
                aria-hidden="true"
              />
              <div className="space-y-1">
                <p className="text-sm font-medium text-muted-foreground">
                  Tidak ada booking hari ini.
                </p>
                <p className="text-xs text-muted-foreground/80">
                  Booking baru akan muncul di sini.
                </p>
              </div>
            </div>
          ) : (
            !fetchError && (
              <Table className="min-w-[860px]">
                <TableHeader className="bg-muted/30">
                  <TableRow>
                    <TableHead style={{ width: 80 }}>Waktu</TableHead>
                    <TableHead style={{ width: 180 }}>Pelanggan</TableHead>
                    <TableHead style={{ width: 160 }}>Layanan</TableHead>
                    <TableHead style={{ width: 140 }}>Terapis</TableHead>
                    <TableHead style={{ width: 100 }}>Ruangan</TableHead>
                    <TableHead style={{ width: 90 }}>Total</TableHead>
                    <TableHead style={{ width: 110 }}>Status</TableHead>
                    <TableHead style={{ width: 120 }}>Aksi</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody className="text-sm">
                  {bookings.map((booking) => (
                    <TableRow
                      key={booking.id}
                      className="h-14 cursor-pointer"
                    >
                      <TableCell className="tabular-nums font-medium">
                        {formatTime(booking.scheduled_start)}
                      </TableCell>
                      <TableCell>
                        <div className="font-medium">
                          {booking.customer_name}
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {booking.customer_phone}
                        </div>
                      </TableCell>
                      <TableCell className="max-w-[160px] truncate">
                        {booking.service_name}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {booking.therapist_name ?? "—"}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {booking.room_name ?? "—"}
                      </TableCell>
                      <TableCell className="tabular-nums">
                        {formatPrice(booking.total_price_idr)}
                      </TableCell>
                      <TableCell>
                        <BookingStatusBadge status={booking.status} />
                      </TableCell>
                      <TableCell>
                        <BookingRowActions booking={booking} />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )
          )}
        </CardContent>
      </Card>

      <Pagination
        pathname="/booking"
        searchParams={{ status, date: dateParam, therapist_id }}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}

