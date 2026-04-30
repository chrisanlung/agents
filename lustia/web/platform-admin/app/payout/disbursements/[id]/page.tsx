import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { formatRupiah, formatDate } from "@/lib/format";
import type { AdminDisbursementDetail } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { DisbursementStatusBadge } from "@/components/disbursement-status-badge";
import { DisbursementTimeline } from "./disbursement-timeline";
import { DisbursementActions } from "./disbursement-actions";

export const metadata: Metadata = {
  title: "Detail Pencairan",
};

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function DisbursementDetailPage({ params }: PageProps) {
  const { id } = await params;

  let disbursement: AdminDisbursementDetail | null = null;

  try {
    disbursement = await apiFetch<AdminDisbursementDetail>(
      `/admin/disbursements/${id}`,
      {},
      { auth: true }
    );
  } catch (err) {
    await handleApiError(err);
  }

  if (!disbursement) {
    return null; // handleApiError redirects or throws
  }

  const totals = disbursement.transactions.reduce(
    (acc, tx) => ({
      gross: acc.gross + tx.received_amount_idr,
      fee: acc.fee + tx.platform_fee_idr,
      net: acc.net + tx.tenant_net_idr,
    }),
    { gross: 0, fee: 0, net: 0 }
  );

  const isFailedOrCancelled =
    disbursement.status === "failed" ||
    disbursement.status === "cancelled";

  return (
    <div className="space-y-6">
      {/* Back link */}
      <Link
        href="/payout/disbursements"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Disburse Aktif
      </Link>

      {/* Page header */}
      <div className="flex items-start justify-between mt-4">
        <div>
          <h1 className="text-xl font-semibold">
            Pencairan — {disbursement.tenant_name}
          </h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Periode {formatDate(disbursement.period_start)} –{" "}
            {formatDate(disbursement.period_end)}
          </p>
        </div>
        <DisbursementStatusBadge
          status={disbursement.status}
          className="text-sm"
        />
      </div>

      {/* Two-column layout on lg+ */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Left column: detail + timeline */}
        <div className="lg:col-span-2 space-y-4">
          {/* Detail card */}
          <Card>
            <CardContent className="pt-4 space-y-4">
              <dl className="text-sm space-y-2">
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Bruto</dt>
                  <dd className="tabular-nums">
                    {formatRupiah(disbursement.gross_amount_idr)}
                  </dd>
                </div>
                <div className="flex justify-between text-destructive">
                  <dt>Fee Platform 5%</dt>
                  <dd className="tabular-nums">
                    − {formatRupiah(disbursement.platform_fee_idr)}
                  </dd>
                </div>
                <div className="flex justify-between border-t pt-2">
                  <dt className="font-bold">Net</dt>
                  <dd className="tabular-nums font-bold text-primary">
                    {formatRupiah(disbursement.net_amount_idr)}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Ref Bank</dt>
                  <dd>{disbursement.bank_reference ?? "—"}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Catatan</dt>
                  <dd>{disbursement.notes ?? "—"}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Ditransfer oleh</dt>
                  <dd>{disbursement.transferred_by_name ?? "—"}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-muted-foreground">Tanggal Transfer</dt>
                  <dd>{formatDate(disbursement.transferred_at)}</dd>
                </div>
              </dl>

              {/* Transaction breakdown */}
              {disbursement.transactions.length > 0 && (
                <div className="max-h-64 overflow-y-auto rounded-md border mt-4">
                  <Table>
                    <TableHeader className="bg-muted/30">
                      <TableRow>
                        <TableHead>Tanggal</TableHead>
                        <TableHead>Booking</TableHead>
                        <TableHead className="tabular-nums">Bruto</TableHead>
                        <TableHead className="tabular-nums">Fee</TableHead>
                        <TableHead className="tabular-nums">Net</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody className="text-sm">
                      {disbursement.transactions.map((tx) => (
                        <TableRow key={tx.id}>
                          <TableCell className="tabular-nums text-muted-foreground">
                            {formatDate(tx.paid_at)}
                          </TableCell>
                          <TableCell className="font-mono text-xs">
                            {tx.booking_code}
                          </TableCell>
                          <TableCell className="tabular-nums">
                            {formatRupiah(tx.received_amount_idr)}
                          </TableCell>
                          <TableCell className="tabular-nums text-muted-foreground">
                            {formatRupiah(tx.platform_fee_idr)}
                          </TableCell>
                          <TableCell className="tabular-nums font-medium">
                            {formatRupiah(tx.tenant_net_idr)}
                          </TableCell>
                        </TableRow>
                      ))}
                      <TableRow className="border-t bg-muted/20 font-semibold">
                        <TableCell colSpan={2}>Total</TableCell>
                        <TableCell className="tabular-nums">
                          {formatRupiah(totals.gross)}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {formatRupiah(totals.fee)}
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {formatRupiah(totals.net)}
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Timeline */}
          <Card>
            <CardContent className="pt-4">
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-3">
                Riwayat Status
              </p>
              <DisbursementTimeline
                currentStatus={disbursement.status}
                createdAt={disbursement.created_at}
                transferredAt={disbursement.transferred_at}
                transferredByName={disbursement.transferred_by_name}
              />

              {/* Off-path terminal states shown as alert banners */}
              {isFailedOrCancelled && (
                <div
                  role="alert"
                  className="mt-4 flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive"
                >
                  <span className="font-medium">
                    {disbursement.status === "failed"
                      ? "Pencairan ini gagal."
                      : "Pencairan ini dibatalkan."}
                  </span>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Right column: actions */}
        <div>
          <DisbursementActions
            id={disbursement.id}
            status={disbursement.status}
            tenantName={disbursement.tenant_name}
          />
        </div>
      </div>
    </div>
  );
}
