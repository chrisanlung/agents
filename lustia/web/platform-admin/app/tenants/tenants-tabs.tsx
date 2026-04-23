"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";

import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TenantStatusMenu } from "./tenant-status-menu";
import type { Tenant, TenantStatus } from "@/lib/types";

const TABS: { value: TenantStatus | "all"; label: string }[] = [
  { value: "active", label: "Aktif" },
  { value: "suspended", label: "Disuspend" },
  { value: "deactivated", label: "Dinonaktifkan" },
];

const STATUS_BADGE: Record<
  string,
  "success" | "warning" | "muted" | "destructive"
> = {
  active: "success",
  suspended: "warning",
  deactivated: "muted",
  pending_approval: "warning",
};

const STATUS_LABELS: Record<string, string> = {
  active: "Aktif",
  suspended: "Disuspend",
  deactivated: "Dinonaktifkan",
  pending_approval: "Menunggu",
};

const PACKAGE_LABELS: Record<string, string> = {
  starter: "Starter",
  growth: "Growth",
  enterprise: "Enterprise",
};

interface TenantTabsProps {
  tenantsByStatus: Record<string, Tenant[]>;
}

export function TenantTabs({ tenantsByStatus }: TenantTabsProps) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const currentTab =
    (searchParams.get("status") as TenantStatus) ?? "active";

  const handleTabChange = useCallback(
    (value: string) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set("status", value);
      router.replace(`${pathname}?${params.toString()}`);
    },
    [searchParams, router, pathname]
  );

  return (
    <Tabs value={currentTab} onValueChange={handleTabChange}>
      <TabsList>
        {TABS.map((tab) => (
          <TabsTrigger key={tab.value} value={tab.value}>
            {tab.label}
            {(tenantsByStatus[tab.value]?.length ?? 0) > 0 && (
              <Badge
                variant="secondary"
                className="ml-1.5 h-4 min-w-[1rem] px-1 text-[10px] leading-none"
              >
                {tenantsByStatus[tab.value].length}
              </Badge>
            )}
          </TabsTrigger>
        ))}
      </TabsList>

      {TABS.map((tab) => (
        <TabsContent key={tab.value} value={tab.value} className="mt-4">
          <TenantTable tenants={tenantsByStatus[tab.value] ?? []} />
        </TabsContent>
      ))}
    </Tabs>
  );
}

function TenantTable({ tenants }: { tenants: Tenant[] }) {
  if (tenants.length === 0) {
    return (
      <div className="flex items-center justify-center py-16 text-center">
        <p className="text-sm text-muted-foreground">Tidak ada tenant di sini.</p>
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Nama</TableHead>
          <TableHead>Slug</TableHead>
          <TableHead>Paket</TableHead>
          <TableHead>Cabang</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-right">Aksi</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tenants.map((tenant) => (
          <TableRow key={tenant.id}>
            <TableCell className="font-medium">{tenant.name}</TableCell>
            <TableCell>
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
                {tenant.slug}
              </code>
            </TableCell>
            <TableCell>
              <Badge variant="secondary">
                {PACKAGE_LABELS[tenant.package] ?? tenant.package}
              </Badge>
            </TableCell>
            <TableCell>
              {tenant.branch_count}/{tenant.max_branches === 999 ? "∞" : tenant.max_branches}
            </TableCell>
            <TableCell>
              <Badge variant={STATUS_BADGE[tenant.status] ?? "secondary"}>
                {STATUS_LABELS[tenant.status] ?? tenant.status}
              </Badge>
            </TableCell>
            <TableCell className="text-right">
              <TenantStatusMenu tenant={tenant} />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
