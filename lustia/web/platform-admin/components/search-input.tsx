"use client";

import { useEffect, useState, useTransition } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Search, X } from "lucide-react";

import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

interface SearchInputProps {
  placeholder?: string;
  paramName?: string;
  /** Min chars before navigating; matches backend `min=2` binding tag. */
  minChars?: number;
  /** Debounce in ms before pushing the new query. */
  debounceMs?: number;
  className?: string;
}

/**
 * Client-side search input with debounce that writes to a URL search param.
 * Resets `page` to 1 on every change so the user lands on the first page of
 * results. Preserves all other sibling filter params.
 */
export function SearchInput({
  placeholder = "Cari…",
  paramName = "q",
  minChars = 2,
  debounceMs = 300,
  className,
}: SearchInputProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [, startTransition] = useTransition();
  const [value, setValue] = useState(searchParams?.get(paramName) ?? "");

  useEffect(() => {
    const timer = setTimeout(() => {
      const params = new URLSearchParams(searchParams?.toString() ?? "");
      const trimmed = value.trim();
      // Reset to page 1 whenever the search term changes.
      params.delete("page");
      if (trimmed.length >= minChars) {
        if (params.get(paramName) === trimmed) return; // no-op
        params.set(paramName, trimmed);
      } else {
        if (!params.has(paramName)) return; // no-op
        params.delete(paramName);
      }
      const qs = params.toString();
      startTransition(() => {
        router.replace(qs ? `${pathname}?${qs}` : pathname);
      });
    }, debounceMs);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  return (
    <div className={cn("relative", className)}>
      <Search
        size={15}
        aria-hidden="true"
        className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
      />
      <Input
        type="search"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder={placeholder}
        className="h-9 pl-8 pr-8 text-sm"
        aria-label={placeholder}
      />
      {value && (
        <button
          type="button"
          onClick={() => setValue("")}
          aria-label="Hapus pencarian"
          className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <X size={14} aria-hidden="true" />
        </button>
      )}
    </div>
  );
}
