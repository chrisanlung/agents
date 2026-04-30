import type { Metadata } from "next";
import Link from "next/link";
import { AlertCircle, Wallet } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { formatRupiah, formatDate } from "@/lib/format";
import type {
  FinanceBalance,
  TenantDisbursement,
  TenantDisbursementListResponse,
  PaymentTransaction,
  PaymentTransactionListResponse,
} from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Pagination } from "@/components/pagination";
import { DateRangePicker } from "@/components/date-range-picker";
import { FilterSelect } from "@/components/filter-select";
import { DisbursementStatusBadge } from "@/components/disbursement-status-badge";
import { PaymentStatusBadge } from "@/components/payment-status-badge";
import { KeuanganClientSection } from "./keuangan-client-section";
import { DisbursementDetailButton } from "./disbursement-detail-button";
import { KeuanganFaqTrigger } from "./keuangan-faq-trigger";

export const metadata: Metadata = {
  title: "Keuangan",
};

const PAGE_SIZE = 10;

const PROVIDER_LABELS: Record<string, string> = {
  ipaymu: "iPaymu QRIS",
  dummy: "Simulasi",
  midtrans: "Midtrans",
};

const TX_STATUS_OPTIONS = [
  { value: "", label: "Semua" },
  { value: "paid", label: "Dibayar" },
  { value: "settled", label: "Settled" },
  { value: "disbursed", label: "Siap Dicairkan" },
  { value: "failed", label: "Gagal" },
  { value: "expired", label: "Kadaluarsa" },
  { value: "voided", label: "Dibatalkan" },
];

interface PageProps {
  searchParams: Promise<{
    page?: string;
    tx_page?: string;
    from?: string;
    to?: string;
    tx_status?: string;
  }>;
}

export default async function KeuanganPage({ searchParams }: PageProps) {
  const { page, tx_page, from, to, tx_status } = await searchParams;

  const disbPage = Math.max(1, Number(page) || 1);
  const txPage = Math.max(1, Number(tx_page) || 1);

  // Parallel fetch: balance + disbursements + transactions
  let balance: FinanceBalance | null = null;
  let balanceError = false;

  let disbursements: TenantDisbursement[] = [];
  let disbTotal = 0;
  let disbTotalPages = 0;
  let disbCurrentPage = disbPage;
  let disbError = false;

  let transactions: PaymentTransaction[] = [];
  let txTotal = 0;
  let txTotalPages = 0;
  let txCurrentPage = txPage;
  let txError = false;

  const txParams = new URLSearchParams({
    page: String(txPage),
    limit: String(PAGE_SIZE),
  });
  if (from) txParams.set("from_date", from);
  if (to) txParams.set("to_date", to);
  if (tx_status) txParams.set("status", tx_status);

  try {
    const [balRes, disbRes, txRes] = await Promise.allSettled([
      apiFetch<FinanceBalance>("/tenant/finance/balance", {}, { auth: true }),
      apiFetch<TenantDisbursementListResponse>(
        `/tenant/finance/disbursements?page=${disbPage}&limit=${PAGE_SIZE}`,
        {},
        { auth: true }
      ),
      apiFetch<PaymentTransactionListResponse>(
        `/tenant/finance/transactions?${txParams.toString()}`,
        {},
        { auth: true }
      ),
    ]);

    if (balRes.status === "fulfilled") {
      balance = balRes.value;
    } else {
      const err = balRes.reason;
      if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
        await handleApiError(err);
      }
      balanceError = true;
    }

    if (disbRes.status === "fulfilled") {
      disbursements = disbRes.value.data;
      disbTotal = disbRes.value.total;
      disbTotalPages = disbRes.value.total_pages;
      disbCurrentPage = disbRes.value.page;
    } else {
      disbError = true;
    }

    if (txRes.status === "fulfilled") {
      transactions = txRes.value.data;
      txTotal = txRes.value.total;
      txTotalPages = txRes.value.total_pages;
      txCurrentPage = txRes.value.page;
    } else {
      txError = true;
    }
  } catch (err) {
    await handleApiError(err);
  }

  // KEU-A5: show empty state if zero across all three
  const isCompletelyEmpty =
    !balanceError &&
    balance !== null &&
    balance.in_process_idr === 0 &&
    balance.ready_to_disburse_idr === 0 &&
    balance.disbursed_idr === 0 &&
    transactions.length === 0 &&
    disbursements.length === 0;

  const preservedTxParams: Record<string, string | undefined> = {
    page: String(disbPage),
  };
  if (from) preservedTxParams.from = from;
  if (to) preservedTxParams.to = to;
  if (tx_status) preservedTxParams.tx_status = tx_status;

  return (
    <div className="space-y-8">
      {/* Page header */}
      <div>
        <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
          Keuangan
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Ringkasan pendapatan dan riwayat transaksi Anda.
        </p>
      </div>

      {/* KEU-A5: Full empty state */}
      {isCompletelyEmpty ? (
        <Card className="py-20">
          <CardContent className="flex flex-col items-center text-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-primary/10 mb-4">
              <Wallet size={32} className="text-primary" aria-hidden="true" />
            </div>
            <h2 className="text-base font-semibold text-foreground">
              Belum ada transaksi
            </h2>
            <p className="mt-2 text-sm text-muted-foreground max-w-xs">
              Pendapatan dari booking yang dibayar akan tampil di sini.
            </p>
            <KeuanganClientSection />
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Section 1: Saldo Anda */}
          <section>
            <h2 className="text-base font-semibold text-foreground mb-3">
              Saldo Anda
            </h2>

            {balanceError ? (
              <div
                role="alert"
                className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
              >
                <AlertCircle
                  size={16}
                  className="mt-0.5 shrink-0 text-red-600"
                  aria-hidden="true"
                />
                <span>Gagal memuat saldo. Muat ulang halaman.</span>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                {/* Dalam Proses */}
                <Card>
                  <CardContent className="pt-4">
                    <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      Dalam Proses
                    </p>
                    <p
                      className="mt-1 text-2xl font-bold tabular-nums text-foreground"
                      aria-label={formatRupiah(balance?.in_process_idr ?? 0)}
                    >
                      {balance
                        ? formatRupiah(balance.in_process_idr)
                        : <span className="h-7 w-32 animate-pulse rounded bg-muted block" />}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      Dana sedang diproses oleh sistem pembayaran.
                    </p>
                  </CardContent>
                </Card>

                {/* Siap Dicairkan */}
                <Card className="bg-primary/5 border-primary/30">
                  <CardContent className="pt-4">
                    <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      Siap Dicairkan
                    </p>
                    <p
                      className="mt-1 text-2xl font-bold tabular-nums text-primary"
                      aria-label={formatRupiah(balance?.ready_to_disburse_idr ?? 0)}
                    >
                      {balance
                        ? formatRupiah(balance.ready_to_disburse_idr)
                        : <span className="h-7 w-32 animate-pulse rounded bg-muted block" />}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      Akan dicairkan pada periode berikutnya (Senin).
                    </p>
                  </CardContent>
                </Card>

                {/* Sudah Dicairkan */}
                <Card>
                  <CardContent className="pt-4">
                    <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                      Sudah Dicairkan (30 hari)
                    </p>
                    <p
                      className="mt-1 text-2xl font-bold tabular-nums text-emerald-600"
                      aria-label={formatRupiah(balance?.disbursed_idr ?? 0)}
                    >
                      {balance
                        ? formatRupiah(balance.disbursed_idr)
                        : <span className="h-7 w-32 animate-pulse rounded bg-muted block" />}
                    </p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      Total dicairkan dalam 30 hari terakhir.
                    </p>
                  </CardContent>
                </Card>
              </div>
            )}
          </section>

          {/* Section 2: Riwayat Pencairan */}
          <section>
            <h2 className="text-base font-semibold text-foreground mb-3">
              Riwayat Pencairan
            </h2>

            <Card>
              <CardContent className="overflow-x-auto p-0">
                {disbError ? (
                  <div
                    role="alert"
                    className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 m-4"
                  >
                    <AlertCircle
                      size={16}
                      className="mt-0.5 shrink-0 text-red-600"
                      aria-hidden="true"
                    />
                    <span>Gagal memuat riwayat pencairan. Muat ulang halaman.</span>
                  </div>
                ) : disbursements.length === 0 ? (
                  <div className="flex flex-col items-center py-16 text-center">
                    <Wallet
                      size={40}
                      aria-hidden="true"
                      className="text-muted-foreground/50 mx-auto"
                    />
                    <p className="mt-3 text-sm font-medium text-muted-foreground">
                      Belum ada pencairan.
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground max-w-xs mx-auto">
                      Pencairan dilakukan setiap Senin pagi untuk transaksi yang
                      sudah settled.
                    </p>
                  </div>
                ) : (
                  <Table className="min-w-[700px]">
                    <TableHeader className="bg-muted/30">
                      <TableRow>
                        <TableHead className="min-w-[160px]">Periode</TableHead>
                        <TableHead className="w-[130px]">Bruto</TableHead>
                        <TableHead className="w-[140px]">
                          Fee Platform (5%)
                        </TableHead>
                        <TableHead className="w-[130px]">Net</TableHead>
                        <TableHead className="w-[130px]">Status</TableHead>
                        <TableHead className="w-[130px]">
                          Tanggal Transfer
                        </TableHead>
                        <TableHead className="w-[110px]">Ref Bank</TableHead>
                        <TableHead className="w-[64px]" />
                      </TableRow>
                    </TableHeader>
                    <TableBody className="text-sm">
                      {disbursements.map((d) => (
                        <DisbursementRow key={d.id} disbursement={d} />
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            {disbTotalPages > 1 && (
              <div className="mt-3">
                <Pagination
                  pathname="/keuangan"
                  searchParams={{
                    tx_page: String(txPage),
                    from,
                    to,
                    tx_status,
                  }}
                  page={disbCurrentPage}
                  totalPages={disbTotalPages}
                  totalCount={disbTotal}
                  pageSize={PAGE_SIZE}
                />
              </div>
            )}
          </section>

          {/* Section 3: Riwayat Transaksi */}
          <section>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-3">
              <h2 className="text-base font-semibold text-foreground">
                Riwayat Transaksi
              </h2>
              <div className="flex items-center gap-2 flex-wrap">
                <DateRangePicker
                  fromParam="from"
                  toParam="to"
                  fromLabel="Dari"
                  toLabel="Sampai"
                />
                <FilterSelect
                  label="Status"
                  name="tx_status"
                  current={tx_status}
                  options={TX_STATUS_OPTIONS}
                />
              </div>
            </div>

            <Card>
              <CardContent className="overflow-x-auto p-0">
                {txError ? (
                  <div
                    role="alert"
                    className="flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 m-4"
                  >
                    <AlertCircle
                      size={16}
                      className="mt-0.5 shrink-0 text-red-600"
                      aria-hidden="true"
                    />
                    <span>Gagal memuat riwayat transaksi. Muat ulang halaman.</span>
                  </div>
                ) : transactions.length === 0 ? (
                  <div className="flex flex-col items-center py-16 text-center">
                    <Wallet
                      size={40}
                      aria-hidden="true"
                      className="text-muted-foreground/50 mx-auto"
                    />
                    <p className="mt-3 text-sm font-medium text-muted-foreground">
                      Belum ada transaksi.
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground/80 max-w-xs mx-auto">
                      Pendapatan dari booking yang sudah dibayar akan tampil di
                      sini.
                    </p>
                  </div>
                ) : (
                  <Table className="min-w-[780px]">
                    <TableHeader className="bg-muted/30">
                      <TableRow>
                        <TableHead className="w-[110px]">Tanggal</TableHead>
                        <TableHead className="w-[100px]">Booking</TableHead>
                        <TableHead className="min-w-[130px]">Pelanggan</TableHead>
                        <TableHead className="w-[90px]">Metode</TableHead>
                        <TableHead className="w-[120px]">Bruto</TableHead>
                        <TableHead className="w-[120px]">Fee Platform</TableHead>
                        <TableHead className="w-[120px]">Net</TableHead>
                        <TableHead className="w-[130px]">Status</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody className="text-sm">
                      {transactions.map((tx) => (
                        <TableRow key={tx.id} className="h-14">
                          <TableCell className="tabular-nums text-muted-foreground">
                            {formatDate(tx.created_at)}
                          </TableCell>
                          <TableCell>
                            <Link
                              href={`/booking/${tx.booking_id}`}
                              className="text-primary underline-offset-2 hover:underline font-mono text-xs"
                            >
                              {tx.booking_code}
                            </Link>
                          </TableCell>
                          <TableCell>{tx.customer_name}</TableCell>
                          <TableCell>
                            {PROVIDER_LABELS[tx.provider] ?? tx.provider}
                          </TableCell>
                          <TableCell className="tabular-nums">
                            {formatRupiah(
                              tx.received_amount_idr ?? tx.expected_amount_idr
                            )}
                          </TableCell>
                          <TableCell className="tabular-nums text-muted-foreground">
                            {tx.platform_fee_idr !== null
                              ? formatRupiah(tx.platform_fee_idr)
                              : "—"}
                          </TableCell>
                          <TableCell className="tabular-nums font-medium">
                            {tx.tenant_net_idr !== null
                              ? formatRupiah(tx.tenant_net_idr)
                              : "—"}
                          </TableCell>
                          <TableCell>
                            <PaymentStatusBadge status={tx.status} />
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            {txTotalPages > 1 && (
              <div className="mt-3">
                <Pagination
                  pathname="/keuangan"
                  searchParams={{
                    page: String(disbPage),
                    from,
                    to,
                    tx_status,
                  }}
                  page={txCurrentPage}
                  totalPages={txTotalPages}
                  totalCount={txTotal}
                  pageSize={PAGE_SIZE}
                />
              </div>
            )}
          </section>

          {/* KEU-A8: FAQ link */}
          <p className="text-center text-xs text-muted-foreground mt-4">
            <KeuanganFaqTrigger />
          </p>
        </>
      )}
    </div>
  );
}

/**
 * DisbursementRow — renders a single row in the Riwayat Pencairan table.
 * The "Lihat" button is a client interaction handled via KeuanganClientSection.
 * We pass the disbursement data via data attributes to keep the server boundary.
 */
function DisbursementRow({ disbursement }: { disbursement: TenantDisbursement }) {
  const bankRef = disbursement.bank_reference;
  const truncatedRef = bankRef
    ? bankRef.substring(0, 12) + (bankRef.length > 12 ? "…" : "")
    : "—";

  return (
    <TableRow className="h-14" data-disbursement-id={disbursement.id}>
      <TableCell>
        {formatDate(disbursement.period_start)} –{" "}
        {formatDate(disbursement.period_end)}
      </TableCell>
      <TableCell className="tabular-nums">
        {formatRupiah(disbursement.gross_amount_idr)}
      </TableCell>
      <TableCell className="tabular-nums text-muted-foreground">
        {formatRupiah(disbursement.platform_fee_idr)}
      </TableCell>
      <TableCell className="tabular-nums font-medium">
        {formatRupiah(disbursement.net_amount_idr)}
      </TableCell>
      <TableCell>
        <DisbursementStatusBadge status={disbursement.status} />
      </TableCell>
      <TableCell>{formatDate(disbursement.transferred_at)}</TableCell>
      <TableCell
        className="truncate max-w-[110px] text-xs"
        title={bankRef ?? undefined}
      >
        {truncatedRef}
      </TableCell>
      <TableCell>
        <DisbursementDetailButton disbursement={disbursement} />
      </TableCell>
    </TableRow>
  );
}

