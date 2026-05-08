"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { AlertCircle, ArrowRight, Banknote } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { DisbursementStatusBadge } from "@/components/disbursement-status-badge";
import { formatRupiah } from "@/lib/format";
import type { AdminDisbursement } from "@/lib/types";

interface RecentDisbursementsPanelProps {
  disbursements: AdminDisbursement[] | null;
  /** Pass true when the underlying fetch failed */
  error?: boolean;
}

/**
 * Right panel — "Disbursement Terbaru".
 *
 * Displays up to 5 most-recent disbursements of any status.  Rows navigate
 * to the disbursement detail page via useRouter (Client Component).
 */
export function RecentDisbursementsPanel({
  disbursements,
  error = false,
}: RecentDisbursementsPanelProps) {
  const router = useRouter();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <CardTitle className="text-base">Disbursement Terbaru</CardTitle>
        <Link
          href="/payout/disbursements"
          className="flex items-center gap-1 text-xs text-muted-foreground transition-colors duration-150 hover:text-foreground"
        >
          Lihat semua <ArrowRight size={12} aria-hidden="true" />
        </Link>
      </CardHeader>
      <CardContent className="p-0">
        {error ? (
          <div
            role="alert"
            className="m-4 flex items-start gap-2 rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
          >
            <AlertCircle
              size={16}
              className="mt-0.5 shrink-0 text-red-600"
              aria-hidden="true"
            />
            <span>
              Gagal memuat disbursement.{" "}
              <a href="/dashboard" className="underline">
                Muat ulang
              </a>
              .
            </span>
          </div>
        ) : !disbursements || disbursements.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-10 text-center">
            <Banknote
              size={32}
              className="text-muted-foreground/30"
              aria-hidden="true"
            />
            <p className="text-sm text-muted-foreground">
              Belum ada disbursement.
            </p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/30">
              <tr>
                <th
                  scope="col"
                  className="px-4 py-2 text-left text-xs font-medium text-muted-foreground"
                >
                  Tenant
                </th>
                <th
                  scope="col"
                  className="px-4 py-2 text-left text-xs font-medium text-muted-foreground"
                >
                  Nominal
                </th>
                <th
                  scope="col"
                  className="px-4 py-2 text-left text-xs font-medium text-muted-foreground"
                >
                  Status
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {disbursements.map((d) => (
                <tr
                  key={d.id}
                  className="cursor-pointer hover:bg-muted/40"
                  onClick={() =>
                    router.push(`/payout/disbursements/${d.id}`)
                  }
                >
                  <td className="max-w-[120px] truncate px-4 py-3 font-medium">
                    {d.tenant_name}
                  </td>
                  <td className="px-4 py-3 tabular-nums text-sm font-medium">
                    {formatRupiah(d.net_amount_idr)}
                  </td>
                  <td className="px-4 py-3">
                    <DisbursementStatusBadge status={d.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </CardContent>
    </Card>
  );
}
