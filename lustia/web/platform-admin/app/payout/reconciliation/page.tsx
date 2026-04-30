import type { Metadata } from "next";
import { RefreshCw, AlertCircle } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { formatRupiah, formatDate } from "@/lib/format";
import { relativeTime } from "@/lib/relative-time";
import type { SettlementBatch, SettlementBatchListResponse } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Pagination } from "@/components/pagination";
import { ReconciliationClient } from "./reconciliation-client";
import { BatchDetailButton } from "./batch-detail-button";

export const metadata: Metadata = {
  title: "Rekonsiliasi Settlement",
};

const PAGE_SIZE = 10;

const PROVIDER_LABELS: Record<string, string> = {
  ipaymu: "iPaymu QRIS",
  dummy: "Simulasi",
};

interface PageProps {
  searchParams: Promise<{ page?: string }>;
}

export default async function ReconciliationPage({ searchParams }: PageProps) {
  const { page } = await searchParams;
  const pageNum = Math.max(1, Number(page) || 1);

  let batches: SettlementBatch[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let fetchError = false;

  try {
    const res = await apiFetch<SettlementBatchListResponse>(
      `/admin/settlement-batches?page=${pageNum}&limit=${PAGE_SIZE}`,
      {},
      { auth: true }
    );
    batches = res.data;
    totalCount = res.total;
    totalPages = res.total_pages;
    currentPage = res.page;
  } catch (err) {
    await handleApiError(err);
    fetchError = true;
  }

  const today = new Date().toISOString().slice(0, 10);
  const latestBatch = batches[0] ?? null;
  const alreadyReconciledToday =
    latestBatch !== null &&
    new Date(latestBatch.settled_at).toISOString().slice(0, 10) === today;

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
            Rekonsiliasi Settlement
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Tarik laporan settlement harian dari iPaymu dan cocokkan transaksi.
          </p>
        </div>
        <ReconciliationClient
          alreadyReconciledToday={alreadyReconciledToday}
          lastBatchAt={latestBatch?.created_at ?? null}
        />
      </div>

      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle
            size={16}
            className="mt-0.5 shrink-0 text-red-600"
            aria-hidden="true"
          />
          <span>Gagal memuat riwayat settlement. Muat ulang halaman.</span>
        </div>
      )}

      {/* Batch list */}
      <Card className="mt-6">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Riwayat Settlement Batch</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          {!fetchError && batches.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <RefreshCw
                size={36}
                className="text-muted-foreground/40"
                aria-hidden="true"
              />
              <p className="text-sm text-muted-foreground">
                Belum ada rekonsiliasi.
              </p>
              <p className="text-xs text-muted-foreground">
                Tarik laporan settlement iPaymu untuk memulai.
              </p>
            </div>
          ) : (
            <Table className="min-w-[700px]">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead className="w-[130px]">Tanggal Settle</TableHead>
                  <TableHead className="w-[100px]">Provider</TableHead>
                  <TableHead className="w-[90px] text-center">
                    Transaksi
                  </TableHead>
                  <TableHead className="w-[130px]">Total</TableHead>
                  <TableHead className="w-[130px]">Ditarik</TableHead>
                  <TableHead className="min-w-[120px]">Admin</TableHead>
                  <TableHead className="w-[80px]" />
                </TableRow>
              </TableHeader>
              <TableBody className="text-sm">
                {batches.map((batch) => (
                  <TableRow key={batch.id} className="h-14">
                    <TableCell>{formatDate(batch.settled_at)}</TableCell>
                    <TableCell>
                      {PROVIDER_LABELS[batch.provider] ?? batch.provider}
                    </TableCell>
                    <TableCell className="text-center tabular-nums">
                      {batch.transaction_count}
                    </TableCell>
                    <TableCell className="tabular-nums font-medium">
                      {formatRupiah(batch.total_amount_idr)}
                    </TableCell>
                    <TableCell
                      title={formatDate(batch.created_at)}
                      className="tabular-nums"
                    >
                      {relativeTime(batch.created_at)}
                    </TableCell>
                    <TableCell>{batch.created_by_name ?? "—"}</TableCell>
                    <TableCell>
                      <BatchDetailButton batch={batch} />
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
          pathname="/payout/reconciliation"
          searchParams={{}}
          page={currentPage}
          totalPages={totalPages}
          totalCount={totalCount}
          pageSize={PAGE_SIZE}
        />
      )}
    </div>
  );
}
