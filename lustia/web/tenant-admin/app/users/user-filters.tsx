"use client";

import { useRouter, usePathname } from "next/navigation";
import { useCallback } from "react";

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Role } from "@/lib/types";

const ROLE_LABELS: Record<string, string> = {
  super_admin: "Super Admin",
  tenant_admin: "Admin Tenant",
  branch_admin: "Admin Cabang",
  therapist: "Terapis",
  finance: "Keuangan",
};

interface UserFiltersProps {
  roles: Role[];
  branches: { id: string; name: string }[];
  currentRoleId: string;
  currentBranchId: string;
  currentIsActive: string;
}

export function UserFilters({
  roles,
  branches,
  currentRoleId,
  currentBranchId,
  currentIsActive,
}: UserFiltersProps) {
  const router = useRouter();
  const pathname = usePathname();

  const buildUrl = useCallback(
    (overrides: Record<string, string>) => {
      const p = new URLSearchParams();
      const current: Record<string, string> = {};
      if (currentRoleId) current.role_id = currentRoleId;
      if (currentBranchId) current.branch_id = currentBranchId;
      if (currentIsActive) current.is_active = currentIsActive;

      const merged = { ...current, ...overrides };
      Object.entries(merged).forEach(([k, v]) => {
        if (v && v !== "all") p.set(k, v);
      });
      // Reset to page 1 on filter change
      p.delete("page");
      const qs = p.toString();
      return qs ? `${pathname}?${qs}` : pathname;
    },
    [currentRoleId, currentBranchId, currentIsActive, pathname]
  );

  return (
    <div className="flex flex-wrap items-center gap-3">
      {/* Role filter */}
      <Select
        value={currentRoleId || "all"}
        onValueChange={(v) => router.push(buildUrl({ role_id: v }))}
      >
        <SelectTrigger className="w-44">
          <SelectValue placeholder="Semua Role" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">Semua Role</SelectItem>
          {roles.map((r) => (
            <SelectItem key={r.id} value={r.id}>
              {ROLE_LABELS[r.name] ?? r.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Branch filter */}
      <Select
        value={currentBranchId || "all"}
        onValueChange={(v) => router.push(buildUrl({ branch_id: v }))}
      >
        <SelectTrigger className="w-44">
          <SelectValue placeholder="Semua Cabang" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">Semua Cabang</SelectItem>
          {branches.map((b) => (
            <SelectItem key={b.id} value={b.id}>
              {b.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Status filter */}
      <Select
        value={currentIsActive || "all"}
        onValueChange={(v) => router.push(buildUrl({ is_active: v }))}
      >
        <SelectTrigger className="w-40">
          <SelectValue placeholder="Semua Status" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">Semua Status</SelectItem>
          <SelectItem value="true">Aktif</SelectItem>
          <SelectItem value="false">Nonaktif</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}
