import type { Metadata } from "next";
import Link from "next/link";
import { AlertCircle, Calendar, Eye } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { getAccessToken } from "@/lib/session";
import { decodeJWTPayload } from "@/lib/jwt";
import type {
  BookingListResponse,
  Booking,
  BranchListResponse,
  Branch,
  ServiceListResponse,
  Service,
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
import { FilterSelect } from "@/components/filter-select";
import { Pagination } from "@/components/pagination";
import { BookingStatusBadge } from "@/components/booking-status-badge";
import { DateRangePicker } from "@/components/date-range-picker";

export const metadata: Metadata = {
  title: "Daftar Booking",
};

const PAGE_SIZE = 10;

// Default: this week (Mon–Sun)
function thisWeekRange() {
  const now = new Date();
  const dayOfWeek = now.getDay();
  const mon = new Date(now);
  mon.setDate(now.getDate() - ((dayOfWeek + 6) % 7));
  const sun = new Date(mon);
  sun.setDate(mon.getDate() + 6);
  return {
    from: mon.toISOString().slice(0, 10),
    to: sun.toISOString().slice(0, 10),
  };
}

interface PageProps {
  searchParams: Promise<{
    from?: string;
    to?: string;
    branch_id?: string;
    status?: string;
    service_id?: string;
    page?: string;
  }>;
}

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

function formatPrice(idr: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(idr);
}

export default async function BookingListPage({ searchParams }: PageProps) {
  const {
    from: fromParam,
    to: toParam,
    branch_id,
    status,
    service_id,
    page: pageParam,
  } = await searchParams;

  const defaults = thisWeekRange();
  const from = fromParam ?? defaults.from;
  const to = toParam ?? defaults.to;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  // Decode JWT to determine role (branch_admin vs tenant_admin)
  let isBranchAdmin = false;
  try {
    const token = await getAccessToken();
    if (token) {
      const claims = decodeJWTPayload<{ roles?: string[] }>(token);
      isBranchAdmin = claims.roles?.includes("branch_admin") === true;
    }
  } catch {
    // non-fatal
  }

  let bookings: Booking[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let branches: Branch[] = [];
  let services: Service[] = [];
  let fetchError = false;

  const params = new URLSearchParams({
    page: String(pageNum),
    limit: String(PAGE_SIZE),
    from,
    to,
  });
  if (branch_id) params.set("branch_id", branch_id);
  if (status) params.set("status", status);
  if (service_id) params.set("service_id", service_id);

  try {
    const [bookingRes, branchRes, serviceRes] = await Promise.allSettled([
      apiFetch<BookingListResponse>(
        `/tenant/bookings?${params.toString()}`,
        {},
        { auth: true }
      ),
      apiFetch<BranchListResponse>(
        "/tenant/branches?limit=200",
        {},
        { auth: true }
      ),
      apiFetch<ServiceListResponse>(
        "/tenant/services?limit=200&is_active=true",
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

    if (branchRes.status === "fulfilled") branches = branchRes.value.data;
    if (serviceRes.status === "fulfilled") services = serviceRes.value.data;
  } catch (err) {
    await handleApiError(err);
  }

  const showBranchColumn = !isBranchAdmin;
  const filterActive = !!(fromParam || toParam || branch_id || status || service_id);

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
          Daftar Booking
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {isBranchAdmin
            ? "Booking di cabang Anda"
            : "Semua booking di semua cabang Anda"}
        </p>
      </div>

      {/* Filter bar */}
      <FilterBar isActive={filterActive} resetHref="/booking">
        <DateRangePicker
          fromParam="from"
          toParam="to"
          fromLabel="Dari"
          toLabel="Sampai"
        />
        {!isBranchAdmin && branches.length > 0 && (
          <FilterSelect
            label="Cabang"
            name="branch_id"
            current={branch_id}
            options={[
              { value: "", label: "Semua" },
              ...branches.map((b) => ({ value: b.id, label: b.name })),
            ]}
          />
        )}
        <FilterSelect
          label="Status"
          name="status"
          current={status}
          options={[
            { value: "", label: "Semua" },
            { value: "paid", label: "Terbayar" },
            { value: "checked_in", label: "Check-in" },
            { value: "completed", label: "Selesai" },
            { value: "no_show", label: "Tidak Hadir" },
            { value: "cancelled", label: "Dibatalkan" },
          ]}
        />
        <FilterSelect
          label="Layanan"
          name="service_id"
          current={service_id}
          options={[
            { value: "", label: "Semua" },
            ...services.map((s) => ({ value: s.id, label: s.name })),
          ]}
        />
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
          <span>Gagal memuat data booking. Muat ulang halaman.</span>
        </div>
      )}

      {/* Table */}
      <Card>
        <CardContent className="overflow-x-auto p-0">
          {!fetchError && bookings.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Calendar
                size={40}
                className="text-muted-foreground/40"
                aria-hidden="true"
              />
              <div className="space-y-1">
                <p className="text-sm font-medium text-muted-foreground">
                  {filterActive
                    ? "Tidak ada booking pada periode ini."
                    : "Belum ada booking. Booking dari pelanggan akan muncul di sini."}
                </p>
              </div>
            </div>
          ) : (
            !fetchError && (
              <Table className="min-w-[800px]">
                <TableHeader className="bg-muted/30">
                  <TableRow>
                    <TableHead>Kode</TableHead>
                    <TableHead>Pelanggan</TableHead>
                    <TableHead>Layanan</TableHead>
                    {showBranchColumn && <TableHead>Cabang</TableHead>}
                    <TableHead>Jadwal</TableHead>
                    <TableHead>Terapis</TableHead>
                    <TableHead>Total</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Aksi</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody className="text-sm">
                  {bookings.map((booking) => (
                    <TableRow key={booking.id} className="h-14">
                      <TableCell className="font-mono text-xs">
                        {booking.code}
                      </TableCell>
                      <TableCell>
                        <div className="font-medium">
                          {booking.customer_name}
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {booking.customer_phone}
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="truncate max-w-[140px] block">
                          {booking.service_name}
                        </span>
                        {booking.addons.length > 0 && (
                          <span className="ml-1 rounded-full bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                            +{booking.addons.length}
                          </span>
                        )}
                      </TableCell>
                      {showBranchColumn && (
                        <TableCell className="text-muted-foreground">
                          {booking.branch_name}
                        </TableCell>
                      )}
                      <TableCell className="tabular-nums text-xs">
                        {formatDateTime(booking.scheduled_start)}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {booking.therapist_name ?? "—"}
                      </TableCell>
                      <TableCell className="tabular-nums">
                        {formatPrice(booking.total_price_idr)}
                      </TableCell>
                      <TableCell>
                        <BookingStatusBadge status={booking.status} />
                      </TableCell>
                      <TableCell>
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
        searchParams={{ from: fromParam, to: toParam, branch_id, status, service_id }}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}
