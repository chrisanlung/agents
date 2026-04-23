import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { Plus, Inbox, AlertCircle } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { ServiceListResponse, Service } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
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
import { ServiceRowActions } from "./service-row-actions";

export const metadata: Metadata = {
  title: "Layanan",
};

interface PageProps {
  searchParams: Promise<{ is_active?: string; category?: string }>;
}

function formatPrice(priceIdr: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
  }).format(priceIdr);
}

export default async function ServicesPage({ searchParams }: PageProps) {
  const { is_active, category } = await searchParams;

  let services: Service[] = [];
  let allCategories: string[] = [];
  let fetchError = false;

  const params = new URLSearchParams({ limit: "50" });
  if (is_active) params.set("is_active", is_active);
  if (category) params.set("category", category);

  try {
    const [servicesRes, allServicesRes] = await Promise.allSettled([
      apiFetch<ServiceListResponse>(
        `/tenant/services?${params.toString()}`,
        {},
        { auth: true }
      ),
      // Fetch all (no filter) to derive categories for filter bar
      apiFetch<ServiceListResponse>(
        "/tenant/services?limit=200",
        {},
        { auth: true }
      ),
    ]);

    if (servicesRes.status === "fulfilled") {
      services = servicesRes.value.data;
    } else {
      const err = servicesRes.reason;
      if (err instanceof ApiError && err.status === 401) redirect("/login");
      fetchError = true;
    }

    if (allServicesRes.status === "fulfilled") {
      const seen = new Set<string>();
      allCategories = allServicesRes.value.data
        .map((s) => s.category)
        .filter((c): c is string => !!c && !seen.has(c) && seen.add(c) !== undefined);
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    throw err;
  }

  function buildUrl(patch: Record<string, string | undefined>) {
    const p = new URLSearchParams();
    const merged = {
      is_active,
      category,
      ...patch,
    };
    Object.entries(merged).forEach(([k, v]) => {
      if (v !== undefined && v !== "") p.set(k, v);
    });
    const qs = p.toString();
    return `/master/services${qs ? `?${qs}` : ""}`;
  }

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Layanan</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Kelola katalog layanan untuk semua cabang
          </p>
        </div>
        <Button asChild>
          <Link href="/master/services/new">
            <Plus size={16} aria-hidden="true" />
            Tambah Layanan
          </Link>
        </Button>
      </div>

      {/* Filter bar */}
      <div className="flex flex-wrap items-center gap-2">
        {allCategories.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-sm text-muted-foreground">Kategori:</span>
            <Link
              href={buildUrl({ category: undefined })}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                !category
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted text-muted-foreground hover:bg-muted/80"
              }`}
            >
              Semua
            </Link>
            {allCategories.map((cat) => (
              <Link
                key={cat}
                href={buildUrl({ category: cat })}
                className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                  category === cat
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted text-muted-foreground hover:bg-muted/80"
                }`}
              >
                {cat}
              </Link>
            ))}
          </div>
        )}

        <div className="flex items-center gap-1.5">
          <span className="text-sm text-muted-foreground">Status:</span>
          {[
            { label: "Semua", value: undefined },
            { label: "Aktif", value: "true" },
            { label: "Nonaktif", value: "false" },
          ].map(({ label, value }) => (
            <Link
              key={label}
              href={buildUrl({ is_active: value })}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                is_active === value || (!is_active && value === undefined)
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted text-muted-foreground hover:bg-muted/80"
              }`}
            >
              {label}
            </Link>
          ))}
        </div>
      </div>

      {/* Error state */}
      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle size={16} className="mt-0.5 shrink-0 text-red-600" aria-hidden="true" />
          <span>Gagal memuat data. Coba muat ulang halaman.</span>
        </div>
      )}

      {/* Table */}
      <Card>
        <CardContent className="p-0">
          {!fetchError && services.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Inbox size={36} className="text-muted-foreground/40" aria-hidden="true" />
              <p className="text-sm text-muted-foreground">
                Belum ada layanan. Tambahkan layanan pertama untuk memulai.
              </p>
              <Button asChild size="sm">
                <Link href="/master/services/new">
                  <Plus size={14} aria-hidden="true" />
                  Tambah Layanan
                </Link>
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nama</TableHead>
                  <TableHead>Kategori</TableHead>
                  <TableHead>Durasi</TableHead>
                  <TableHead>Harga</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {services.map((s) => (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium">{s.name}</TableCell>
                    <TableCell>
                      {s.category ? (
                        <Badge variant="outline">{s.category}</Badge>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell className="tabular-nums text-sm text-muted-foreground">
                      {s.duration_minutes} menit
                    </TableCell>
                    <TableCell className="tabular-nums text-sm">
                      {formatPrice(s.price_idr)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={s.is_active ? "success" : "muted"}>
                        {s.is_active ? "Aktif" : "Nonaktif"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <ServiceRowActions service={s} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
