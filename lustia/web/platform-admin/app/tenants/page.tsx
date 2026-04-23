import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { Metadata } from "next";
import { Building2, Loader2 } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { TenantListResponse, Tenant, TenantStatus } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { TenantTabs } from "./tenants-tabs";

export const metadata: Metadata = {
  title: "Manajemen Tenant",
};

const STATUSES: TenantStatus[] = ["active", "suspended", "deactivated"];

export default async function TenantsPage() {
  // Fetch all relevant statuses in parallel
  let tenantsByStatus: Record<string, Tenant[]> = {};

  try {
    const results = await Promise.allSettled(
      STATUSES.map((status) =>
        apiFetch<TenantListResponse>(
          `/admin/tenants?status=${status}&limit=50`,
          {},
          { auth: true }
        )
      )
    );

    results.forEach((result, index) => {
      const status = STATUSES[index];
      if (result.status === "fulfilled") {
        tenantsByStatus[status] = result.value.data;
      } else {
        tenantsByStatus[status] = [];
      }
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Building2 size={22} className="text-primary" aria-hidden="true" />
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            Manajemen Tenant
          </h1>
          <p className="text-sm text-muted-foreground">
            Kelola status tenant yang aktif, disuspend, dan dinonaktifkan.
          </p>
        </div>
      </div>

      <Card>
        <CardContent className="p-6">
          <Suspense fallback={null}>
            <TenantTabs tenantsByStatus={tenantsByStatus} />
          </Suspense>
        </CardContent>
      </Card>
    </div>
  );
}
