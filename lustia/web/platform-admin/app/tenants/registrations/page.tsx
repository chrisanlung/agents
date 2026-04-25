import type { Metadata } from "next";
import Link from "next/link";
import { ClipboardList, Inbox } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import {
  type RegistrationListResponse,
  type TenantRegistration,
} from "@/lib/types";
import { relativeTime } from "@/lib/relative-time";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/pagination";

const PAGE_SIZE = 10;

export const metadata: Metadata = {
  title: "Antrian Registrasi",
};

const PACKAGE_LABELS: Record<string, string> = {
  starter: "Starter",
  growth: "Growth",
  enterprise: "Enterprise",
};

interface PageProps {
  searchParams: Promise<{ page?: string }>;
}

export default async function RegistrationsPage({ searchParams }: PageProps) {
  const { page: pageParam } = await searchParams;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  let registrations: TenantRegistration[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;

  const params = new URLSearchParams({
    status: "pending",
    page: String(pageNum),
    limit: String(PAGE_SIZE),
  });

  try {
    const res = await apiFetch<RegistrationListResponse>(
      `/admin/tenant-registrations?${params.toString()}`,
      {},
      { auth: true }
    );
    registrations = res.data;
    totalCount = res.total_count;
    totalPages = res.total_pages;
    currentPage = res.page;
  } catch (err) {
    await handleApiError(err);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <ClipboardList size={22} className="text-primary" aria-hidden="true" />
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            Antrian Registrasi Tenant
          </h1>
          <p className="text-sm text-muted-foreground">
            Tinjau dan setujui atau tolak permohonan registrasi perusahaan baru.
          </p>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">
              Menunggu Peninjauan
            </CardTitle>
            <Badge variant={totalCount > 0 ? "default" : "muted"}>
              {totalCount} pending
            </Badge>
          </div>
          <CardDescription>
            Hanya registrasi dengan status &quot;pending&quot; yang ditampilkan.
          </CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {registrations.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Inbox size={36} className="text-muted-foreground/40" aria-hidden="true" />
              <p className="text-sm text-muted-foreground">
                Tidak ada registrasi yang menunggu peninjauan.
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Perusahaan</TableHead>
                  <TableHead>Slug</TableHead>
                  <TableHead>Paket</TableHead>
                  <TableHead>Kontak</TableHead>
                  <TableHead>Waktu</TableHead>
                  <TableHead className="text-right">Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {registrations.map((reg) => (
                  <RegistrationRow key={reg.id} registration={reg} />
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Pagination
        pathname="/tenants/registrations"
        searchParams={{}}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}

function RegistrationRow({
  registration: reg,
}: {
  registration: TenantRegistration;
}) {
  return (
    <TableRow>
      <TableCell className="font-medium">{reg.company_name}</TableCell>
      <TableCell>
        <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
          {reg.requested_slug}
        </code>
      </TableCell>
      <TableCell>
        <Badge variant="secondary">
          {PACKAGE_LABELS[reg.package] ?? reg.package}
        </Badge>
      </TableCell>
      <TableCell>
        <div className="text-sm">
          <p className="font-medium">{reg.contact_name}</p>
          <p className="text-muted-foreground">{reg.contact_email}</p>
        </div>
      </TableCell>
      <TableCell>
        <time
          dateTime={reg.created_at}
          className="text-sm text-muted-foreground"
          title={new Date(reg.created_at).toLocaleString("id-ID")}
        >
          {relativeTime(reg.created_at)}
        </time>
      </TableCell>
      <TableCell className="text-right">
        <Button asChild size="sm" variant="outline">
          <Link href={`/tenants/registrations/${reg.id}`}>Tinjau</Link>
        </Button>
      </TableCell>
    </TableRow>
  );
}
