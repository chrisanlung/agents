import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { TenantStatusMenu } from "./tenant-status-menu";
import type { Tenant, TenantStatus } from "@/lib/types";

const TABS: { value: TenantStatus; label: string }[] = [
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
  currentStatus: TenantStatus;
  tenants: Tenant[];
  /** Sibling filter params preserved when switching tabs. */
  preservedParams?: { q?: string; package?: string };
}

/**
 * Server-rendered tab switcher + table. Each tab is a Link that navigates to
 * the first page of that status filter, preserving sibling filter params.
 */
export function TenantTabs({ currentStatus, tenants, preservedParams }: TenantTabsProps) {
  const buildHref = (status: TenantStatus) => {
    const p = new URLSearchParams({ status });
    if (preservedParams?.q) p.set("q", preservedParams.q);
    if (preservedParams?.package) p.set("package", preservedParams.package);
    return `/tenants?${p.toString()}`;
  };

  const hasFilter = !!(preservedParams?.q || preservedParams?.package);

  return (
    <div>
      <nav
        aria-label="Filter status tenant"
        className="mb-4 flex items-center gap-1 border-b border-border/60"
      >
        {TABS.map((tab) => {
          const isActive = tab.value === currentStatus;
          return (
            <Link
              key={tab.value}
              href={buildHref(tab.value)}
              className={cn(
                "-mb-px border-b-2 px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
              aria-current={isActive ? "page" : undefined}
            >
              {tab.label}
            </Link>
          );
        })}
      </nav>

      <TenantTable tenants={tenants} hasFilter={hasFilter} />
    </div>
  );
}

function TenantTable({ tenants, hasFilter }: { tenants: Tenant[]; hasFilter: boolean }) {
  if (tenants.length === 0) {
    return (
      <div className="flex items-center justify-center py-16 text-center">
        <p className="text-sm text-muted-foreground">
          {hasFilter
            ? "Tidak ada tenant cocok dengan pencarian."
            : "Tidak ada tenant di sini."}
        </p>
      </div>
    );
  }

  return (
    <Table className="min-w-[720px]">
      <TableHeader className="bg-muted/30">
        <TableRow>
          <TableHead>Nama</TableHead>
          <TableHead>Slug</TableHead>
          <TableHead>Paket</TableHead>
          <TableHead>Cabang</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Aksi</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tenants.map((tenant) => (
          <TableRow key={tenant.id} className="h-14">
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
              {tenant.branch_count}/
              {tenant.max_branches === 999 ? "∞" : tenant.max_branches}
            </TableCell>
            <TableCell>
              <Badge variant={STATUS_BADGE[tenant.status] ?? "secondary"}>
                {STATUS_LABELS[tenant.status] ?? tenant.status}
              </Badge>
            </TableCell>
            <TableCell>
              <TenantStatusMenu tenant={tenant} />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
