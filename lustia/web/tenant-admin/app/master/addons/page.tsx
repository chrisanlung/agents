import type { Metadata } from "next";
import Link from "next/link";
import { Plus, PackagePlus, AlertCircle } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import { FilterSelect } from "@/components/filter-select";
import { FilterBar } from "@/components/filter-bar";
import { Pagination } from "@/components/pagination";

import type { AddonListResponse, Addon } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { AddonReorderList } from "./addon-reorder-buttons";

export const metadata: Metadata = {
  title: "Tambahan",
};

const PAGE_SIZE = 10;

interface PageProps {
  searchParams: Promise<{
    is_active?: string;
    page?: string;
  }>;
}

export default async function AddonsPage({ searchParams }: PageProps) {
  const { is_active, page: pageParam } = await searchParams;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  let addons: Addon[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let fetchError = false;

  const params = new URLSearchParams({ page: String(pageNum), limit: String(PAGE_SIZE) });
  if (is_active) params.set("is_active", is_active);

  try {
    const res = await apiFetch<AddonListResponse>(
      `/tenant/addons?${params.toString()}`,
      {},
      { auth: true }
    );
    addons = res.data;
    totalCount = res.total_count;
    totalPages = res.total_pages;
    currentPage = res.page;
  } catch (err) {
    if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
      await handleApiError(err);
    }
    fetchError = true;
  }

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
            Tambahan
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Kelola katalog add-on untuk semua layanan Anda
          </p>
        </div>
        <Button asChild className="w-full sm:w-auto">
          <Link href="/master/addons/new">
            <Plus size={16} aria-hidden="true" />
            Tambah Add-on
          </Link>
        </Button>
      </div>

      {/* Filter bar */}
      <FilterBar isActive={!!is_active} resetHref="/master/addons">
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
          <AlertCircle
            size={16}
            className="mt-0.5 shrink-0 text-red-600"
            aria-hidden="true"
          />
          <span>Gagal memuat data. Muat ulang halaman.</span>
        </div>
      )}

      {/* Table / Empty state */}
      <Card>
        <CardContent className="overflow-x-auto p-0">
          {!fetchError && addons.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <PackagePlus
                size={40}
                className="text-muted-foreground/50"
                aria-hidden="true"
              />
              <div className="space-y-1">
                <p className="text-sm font-medium text-muted-foreground">
                  {is_active === "true"
                    ? "Tidak ada add-on aktif saat ini."
                    : is_active === "false"
                      ? "Tidak ada add-on nonaktif."
                      : "Belum ada tambahan"}
                </p>
                {!is_active && (
                  <p className="mx-auto max-w-xs text-xs text-muted-foreground/80">
                    Buat daftar add-on untuk ditawarkan ke pelanggan Anda.
                  </p>
                )}
              </div>
              {!is_active && (
                <Button asChild size="sm">
                  <Link href="/master/addons/new">
                    <Plus size={14} aria-hidden="true" />
                    Tambah Add-on
                  </Link>
                </Button>
              )}
            </div>
          ) : (
            !fetchError && <AddonReorderList initialAddons={addons} />
          )}
        </CardContent>
      </Card>

      {/* Pagination */}
      <Pagination
        pathname="/master/addons"
        searchParams={{ is_active }}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}
