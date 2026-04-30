import type { Metadata } from "next";
import Link from "next/link";
import { Banknote } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { formatRupiah, formatDate } from "@/lib/format";
import { relativeTime } from "@/lib/relative-time";
import type {
  AdminDisbursement,
  AdminDisbursementListResponse,
  DisbursementStatus,
} from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Pagination } from "@/components/pagination";
import { FilterBar } from "@/components/filter-bar";
import { FilterSelect } from "@/components/filter-select";
import { DisbursementStatusBadge } from "@/components/disbursement-status-badge";

export const metadata: Metadata = {
  title: "Disburse Aktif",
};

const PAGE_SIZE = 10;

const VALID_STATUSES: readonly DisbursementStatus[] = [
  "pending",
  "processing",
  "transferred",
  "failed",
  "cancelled",
];

const STATUS_OPTIONS = [
  { value: "", label: "Semua" },
  { value: "pending", label: "Menunggu" },
  { value: "processing", label: "Sedang Diproses" },
  { value: "transferred", label: "Sudah Ditransfer" },
  { value: "failed", label: "Gagal" },
];

interface PageProps {
  searchParams: Promise<{ status?: string; page?: string }>;
}

export default async function DisbursementsPage({ searchParams }: PageProps) {
  const { status: rawStatus, page: pageParam } = await searchParams;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  const status = VALID_STATUSES.includes(rawStatus as DisbursementStatus)
    ? (rawStatus as DisbursementStatus)
    : undefined;

  const params = new URLSearchParams({
    page: String(pageNum),
    limit: String(PAGE_SIZE),
  });
  if (status) params.set("status", status);

  let disbursements: AdminDisbursement[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;

  try {
    const res = await apiFetch<AdminDisbursementListResponse>(
      `/admin/disbursements?${params.toString()}`,
      {},
      { auth: true }
    );
    disbursements = res.data;
    totalCount = res.total;
    totalPages = res.total_pages;
    currentPage = res.page;
  } catch (err) {
    await handleApiError(err);
  }

  const isFilterActive = !!status;

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
          Disburse Aktif
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Kelola pencairan yang sedang berjalan.
        </p>
      </div>

      {/* Filter */}
      <FilterBar
        isActive={isFilterActive}
        resetHref="/payout/disbursements"
      >
        <FilterSelect
          label="Status"
          name="status"
          current={status}
          options={STATUS_OPTIONS}
        />
      </FilterBar>

      {/* Table */}
      <Card>
        <CardContent className="overflow-x-auto p-0">
          {disbursements.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Banknote
                size={36}
                className="text-muted-foreground/40"
                aria-hidden="true"
              />
              <p className="text-sm text-muted-foreground">
                Tidak ada pencairan aktif.
              </p>
              <p className="text-xs text-muted-foreground">
                Buat pencairan dari halaman Pencairan Tenant.
              </p>
            </div>
          ) : (
            <Table className="min-w-[680px]">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead className="min-w-[150px]">Tenant</TableHead>
                  <TableHead className="min-w-[160px]">Periode</TableHead>
                  <TableHead className="w-[130px]">Net</TableHead>
                  <TableHead className="w-[140px]">Status</TableHead>
                  <TableHead className="w-[120px]">Dibuat</TableHead>
                  <TableHead className="w-[90px]" />
                </TableRow>
              </TableHeader>
              <TableBody className="text-sm">
                {disbursements.map((d) => (
                  <TableRow
                    key={d.id}
                    className="h-14 cursor-pointer hover:bg-muted/30"
                    onClick={(e) => {
                      // Allow clicking the row but not the button cell
                      const target = e.target as HTMLElement;
                      if (!target.closest("a") && !target.closest("button")) {
                        window.location.href = `/payout/disbursements/${d.id}`;
                      }
                    }}
                  >
                    <TableCell className="font-medium">
                      {d.tenant_name}
                    </TableCell>
                    <TableCell>
                      {formatDate(d.period_start)} –{" "}
                      {formatDate(d.period_end)}
                    </TableCell>
                    <TableCell className="tabular-nums font-medium">
                      {formatRupiah(d.net_amount_idr)}
                    </TableCell>
                    <TableCell>
                      <DisbursementStatusBadge status={d.status} />
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {relativeTime(d.created_at)}
                    </TableCell>
                    <TableCell>
                      <Button variant="outline" size="sm" asChild>
                        <Link href={`/payout/disbursements/${d.id}`}>
                          Kelola
                        </Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {totalPages > 1 && (
        <Pagination
          pathname="/payout/disbursements"
          searchParams={{ status }}
          page={currentPage}
          totalPages={totalPages}
          totalCount={totalCount}
          pageSize={PAGE_SIZE}
        />
      )}
    </div>
  );
}
