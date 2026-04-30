"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useMemo } from "react";
import { CheckCircle2 } from "lucide-react";

import { cn } from "@/lib/utils";

export interface FilterOption {
  value: string;
  label: string;
}

interface FilterSelectProps {
  label: string;
  name: string;
  options: FilterOption[];
  current?: string;
  widthClass?: string;
}

export function FilterSelect({
  label,
  name,
  options,
  current,
  widthClass,
}: FilterSelectProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const isActive = useMemo(
    () => current !== undefined && current !== "",
    [current]
  );

  const onChange = (next: string) => {
    const params = new URLSearchParams(searchParams?.toString() ?? "");
    if (next) {
      params.set(name, next);
    } else {
      params.delete(name);
    }
    params.delete("page");
    const qs = params.toString();
    router.push(qs ? `${pathname}?${qs}` : pathname);
  };

  return (
    <label className="flex items-center gap-1.5 text-sm text-muted-foreground">
      <span>{label}</span>
      <div className="relative flex items-center">
        {isActive && (
          <CheckCircle2
            size={12}
            className="pointer-events-none absolute left-2 text-primary-foreground"
            aria-hidden="true"
          />
        )}
        <select
          value={current ?? ""}
          onChange={(e) => onChange(e.target.value)}
          className={cn(
            "rounded-md border-0 py-1 pr-2.5 text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring",
            isActive
              ? "bg-primary pl-7 text-primary-foreground"
              : "bg-muted pl-2.5 text-muted-foreground hover:bg-muted/80",
            widthClass
          )}
          aria-label={label}
        >
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </div>
    </label>
  );
}
