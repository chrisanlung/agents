"use client";

import { useRouter } from "next/navigation";
import type { Branch } from "@/lib/types";

interface BranchOverflowSelectProps {
  branches: Branch[];
  currentBranchId?: string;
  currentStatus?: string;
  /** Pass branches.length so the label reads correctly when nothing is picked */
  label: string;
}

/**
 * Native <select> that auto-submits to /master/therapists on change. Used for
 * the overflow branches beyond the first 3 filter pills — keeps the filter bar
 * to one line on tenants with many branches without loading extra client JS.
 */
export function BranchOverflowSelect({
  branches,
  currentBranchId,
  currentStatus,
  label,
}: BranchOverflowSelectProps) {
  const router = useRouter();
  const activeInOverflow = currentBranchId && branches.some((b) => b.id === currentBranchId);

  return (
    <select
      value={activeInOverflow ? currentBranchId : ""}
      onChange={(e) => {
        const value = e.target.value;
        const params = new URLSearchParams();
        if (value) params.set("branch_id", value);
        if (currentStatus) params.set("is_active", currentStatus);
        const qs = params.toString();
        router.push(`/master/therapists${qs ? `?${qs}` : ""}`);
      }}
      className={`rounded-md border-0 px-2.5 py-1 text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring ${
        activeInOverflow
          ? "bg-primary text-primary-foreground"
          : "bg-muted text-muted-foreground hover:bg-muted/80"
      }`}
      aria-label="Pilih cabang lain"
    >
      <option value="" disabled hidden>
        {label}
      </option>
      {branches.map((b) => (
        <option key={b.id} value={b.id}>
          {b.name}
        </option>
      ))}
    </select>
  );
}
