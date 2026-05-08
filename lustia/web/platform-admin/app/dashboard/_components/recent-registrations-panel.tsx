"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { AlertCircle, ArrowRight, Inbox } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { relativeTime } from "@/lib/relative-time";
import type { TenantRegistration } from "@/lib/types";

interface RecentRegistrationsPanelProps {
  registrations: TenantRegistration[] | null;
  /** Pass true when the underlying fetch failed */
  error?: boolean;
}

/**
 * Left panel — "Pendaftaran Tenant Terbaru".
 *
 * Displays up to 5 most-recent pending registrations.  Rows are clickable
 * via useRouter (Client Component) — the spec calls for a tbody that handles
 * row-level navigation without nesting <a> inside <tr>.
 */
export function RecentRegistrationsPanel({
  registrations,
  error = false,
}: RecentRegistrationsPanelProps) {
  const router = useRouter();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <CardTitle className="text-base">Pendaftaran Tenant Terbaru</CardTitle>
        <Link
          href="/tenants/registrations"
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
              Gagal memuat pendaftaran.{" "}
              <a href="/dashboard" className="underline">
                Muat ulang
              </a>
              .
            </span>
          </div>
        ) : !registrations || registrations.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-10 text-center">
            <Inbox
              size={32}
              className="text-muted-foreground/30"
              aria-hidden="true"
            />
            <p className="text-sm text-muted-foreground">
              Tidak ada pendaftaran menunggu review.
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
                  Nama Tenant
                </th>
                <th
                  scope="col"
                  className="px-4 py-2 text-left text-xs font-medium text-muted-foreground"
                >
                  Tanggal Daftar
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
              {registrations.map((reg) => (
                <tr
                  key={reg.id}
                  className="cursor-pointer hover:bg-muted/40"
                  onClick={() =>
                    router.push(`/tenants/registrations/${reg.id}`)
                  }
                >
                  <td className="max-w-[140px] truncate px-4 py-3 font-medium">
                    {reg.company_name}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 tabular-nums text-muted-foreground">
                    {relativeTime(reg.created_at)}
                  </td>
                  <td className="px-4 py-3">
                    <Badge
                      variant="outline"
                      className="border-amber-200 bg-amber-100 text-amber-800"
                    >
                      Pending
                    </Badge>
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
