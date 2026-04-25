import type { Metadata } from "next";
import Link from "next/link";
import { Plus, Sparkles, AlertCircle } from "lucide-react";

import { cn, categoryColorClass } from "@/lib/utils";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { FilterSelect } from "@/components/filter-select";
import { FilterBar } from "@/components/filter-bar";
import { Pagination } from "@/components/pagination";

const PAGE_SIZE = 10;
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
  searchParams: Promise<{
    is_active?: string;
    category?: string;
    page?: string;
  }>;
}

function formatPrice(priceIdr: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
  }).format(priceIdr);
}

export default async function ServicesPage({ searchParams }: PageProps) {
  const { is_active, category, page: pageParam } = await searchParams;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  let services: Service[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let allCategories: string[] = [];
  let fetchError = false;

  const params = new URLSearchParams({ page: String(pageNum), limit: String(PAGE_SIZE) });
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
      totalCount = servicesRes.value.total_count;
      totalPages = servicesRes.value.total_pages;
      currentPage = servicesRes.value.page;
    } else {
      const err = servicesRes.reason;
      if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
        await handleApiError(err);
      }
      fetchError = true;
    }

    if (allServicesRes.status === "fulfilled") {
      const seen = new Set<string>();
      allCategories = allServicesRes.value.data
        .map((s) => s.category)
        .filter((c): c is string => !!c && !seen.has(c) && seen.add(c) !== undefined);
    }
  } catch (err) {
    await handleApiError(err);
  }

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground sm:text-2xl">Layanan</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Kelola katalog layanan untuk semua cabang
          </p>
        </div>
        <Button asChild className="w-full sm:w-auto">
          <Link href="/master/services/new">
            <Plus size={16} aria-hidden="true" />
            Tambah Layanan
          </Link>
        </Button>
      </div>

      {/* Filter bar */}
      <FilterBar
        isActive={!!(is_active || category)}
        resetHref="/master/services"
      >
        {allCategories.length > 0 && (
          <FilterSelect
            label="Kategori"
            name="category"
            current={category}
            options={[
              { value: "", label: "Semua" },
              ...allCategories.map((c) => ({ value: c, label: c })),
            ]}
          />
        )}

        <FilterSelect
          label="Status"
          name="is_active"
          current={is_active}
          options={[
            { value: "", label: "Semua" },
            { value: "true", label: "Aktif" },
            { value: "false", label: "Nonaktif" },
          ]}
        />
      </FilterBar>

      {/* Error state */}
      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle size={16} className="mt-0.5 shrink-0 text-red-600" aria-hidden="true" />
          <span>Gagal memuat data. Muat ulang halaman.</span>
        </div>
      )}

      {/* Table */}
      <Card>
        <CardContent className="overflow-x-auto p-0">
          {!fetchError && services.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Sparkles size={40} className="text-muted-foreground/50" aria-hidden="true" />
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">
                  {is_active === "false"
                    ? "Tidak ada layanan nonaktif."
                    : is_active === "true"
                      ? "Tidak ada layanan aktif saat ini."
                      : category
                        ? `Tidak ada layanan pada kategori "${category}".`
                        : "Belum ada layanan."}
                </p>
                {!is_active && !category && (
                  <p className="text-xs text-muted-foreground/80">
                    Tambahkan layanan yang tersedia di spa Anda.
                  </p>
                )}
              </div>
              {!is_active && !category && (
                <Button asChild size="sm">
                  <Link href="/master/services/new">
                    <Plus size={14} aria-hidden="true" />
                    Tambah Layanan
                  </Link>
                </Button>
              )}
            </div>
          ) : (
            <Table className="min-w-[480px]">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead>Nama</TableHead>
                  <TableHead>Kategori</TableHead>
                  <TableHead>Durasi</TableHead>
                  <TableHead>Harga</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {services.map((s) => (
                  <TableRow key={s.id} className="h-14">
                    <TableCell className="font-medium">{s.name}</TableCell>
                    <TableCell>
                      {s.category ? (
                        <Badge
                          variant="outline"
                          className={cn("border", categoryColorClass(s.category))}
                        >
                          {s.category}
                        </Badge>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="tabular-nums font-normal">
                        {s.duration_minutes} menit
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="tabular-nums font-normal">
                        {formatPrice(s.price_idr)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={s.is_active ? "success" : "muted"}>
                        {s.is_active ? "Aktif" : "Nonaktif"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <ServiceRowActions service={s} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Pagination */}
      <Pagination
        pathname="/master/services"
        searchParams={{ is_active, category }}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}
