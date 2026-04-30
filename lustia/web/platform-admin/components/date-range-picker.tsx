"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useMemo } from "react";

interface DateRangePickerProps {
  fromParam?: string;
  toParam?: string;
  fromLabel?: string;
  toLabel?: string;
}

export function DateRangePicker({
  fromParam = "from",
  toParam = "to",
  fromLabel = "Dari",
  toLabel = "Sampai",
}: DateRangePickerProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const from = useMemo(
    () => searchParams.get(fromParam) ?? "",
    [searchParams, fromParam]
  );
  const to = useMemo(
    () => searchParams.get(toParam) ?? "",
    [searchParams, toParam]
  );

  function handleChange(param: string, value: string) {
    const params = new URLSearchParams(searchParams?.toString() ?? "");
    if (value) {
      params.set(param, value);
    } else {
      params.delete(param);
    }
    params.delete("page");
    const qs = params.toString();
    router.push(qs ? `${pathname}?${qs}` : pathname);
  }

  return (
    <div className="flex items-center gap-2 text-sm text-muted-foreground">
      <label className="flex items-center gap-1.5">
        <span>{fromLabel}</span>
        <input
          type="date"
          value={from}
          onChange={(e) => handleChange(fromParam, e.target.value)}
          className="rounded-md border-0 bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          aria-label={fromLabel}
        />
      </label>
      <label className="flex items-center gap-1.5">
        <span>{toLabel}</span>
        <input
          type="date"
          value={to}
          onChange={(e) => handleChange(toParam, e.target.value)}
          className="rounded-md border-0 bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          aria-label={toLabel}
        />
      </label>
    </div>
  );
}
