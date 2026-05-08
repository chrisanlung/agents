"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";

interface FilterDateProps {
  label: string;
  name: string;
  current: string;
}

/**
 * FilterDate — URL-based date filter as a native <input type="date">.
 * Changing the value navigates to the same pathname with the updated query
 * string (preserves other params, resets page to 1 per ADR 0013).
 */
export function FilterDate({ label, name, current }: FilterDateProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

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
      <input
        type="date"
        name={name}
        value={current}
        onChange={(e) => onChange(e.target.value)}
        className="rounded-md border-0 bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
        aria-label={label}
      />
    </label>
  );
}
