import type { Metadata } from "next";
import { Building2 } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type {
  TenantListResponse,
  Tenant,
  TenantStatus,
} from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { Pagination } from "@/components/pagination";
import { SearchInput } from "@/components/search-input";
import { TenantTabs } from "./tenants-tabs";
import { PackageFilter } from "./package-filter";

export const metadata: Metadata = {
  title: "Manajemen Tenant",
};

const VALID_STATUSES: readonly TenantStatus[] = [
  "active",
  "suspended",
  "deactivated",
];
const VALID_PACKAGES = ["starter", "growth", "enterprise"] as const;
const PAGE_SIZE = 10;

interface PageProps {
  searchParams: Promise<{
    status?: string;
    page?: string;
    q?: string;
    package?: string;
  }>;
}

export default async function TenantsPage({ searchParams }: PageProps) {
  const {
    status: rawStatus,
    page: pageParam,
    q: rawQ,
    package: rawPackage,
  } = await searchParams;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  // Normalise the status filter: fall back to "active" if missing or invalid.
  const status: TenantStatus = VALID_STATUSES.includes(
    rawStatus as TenantStatus
  )
    ? (rawStatus as TenantStatus)
    : "active";

  // Backend rejects q < 2 chars (binding min=2). Trim + drop short queries
  // client-side so we don't fire 400s from the FE.
  const q = (rawQ ?? "").trim();
  const safeQ = q.length >= 2 ? q : "";
  const pkg = VALID_PACKAGES.includes(rawPackage as (typeof VALID_PACKAGES)[number])
    ? (rawPackage as string)
    : "";

  let tenants: Tenant[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;

  const params = new URLSearchParams({
    status,
    page: String(pageNum),
    limit: String(PAGE_SIZE),
  });
  if (safeQ) params.set("q", safeQ);
  if (pkg) params.set("package", pkg);

  try {
    const res = await apiFetch<TenantListResponse>(
      `/admin/tenants?${params.toString()}`,
      {},
      { auth: true }
    );
    tenants = res.data;
    totalCount = res.total_count;
    totalPages = res.total_pages;
    currentPage = res.page;
  } catch (err) {
    await handleApiError(err);
  }

  // Preserved across pagination + tab nav.
  const preservedParams: Record<string, string | undefined> = { status };
  if (safeQ) preservedParams.q = safeQ;
  if (pkg) preservedParams.package = pkg;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Building2 size={22} className="text-primary" aria-hidden="true" />
        <div>
          <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
            Manajemen Tenant
          </h1>
          <p className="text-sm text-muted-foreground">
            Kelola status tenant yang aktif, disuspend, dan dinonaktifkan.
          </p>
        </div>
      </div>

      <Card>
        <CardContent className="space-y-4 overflow-x-auto p-6">
          <div className="flex flex-col items-stretch gap-2 sm:flex-row sm:items-center sm:gap-3">
            <SearchInput
              placeholder="Cari nama atau slug tenant…"
              className="sm:w-80"
            />
            <PackageFilter current={pkg} />
          </div>
          <TenantTabs
            currentStatus={status}
            tenants={tenants}
            preservedParams={{
              q: safeQ || undefined,
              package: pkg || undefined,
            }}
          />
        </CardContent>
      </Card>

      <Pagination
        pathname="/tenants"
        searchParams={preservedParams}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}
