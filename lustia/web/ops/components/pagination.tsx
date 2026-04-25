import Link from "next/link";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";

interface PaginationProps {
  /** Current pathname (without query). The component rewrites the query string. */
  pathname: string;
  /** Sibling query params from the page — passed through; `page` is excluded and overridden. */
  searchParams: Record<string, string | undefined>;
  /** Current page number, 1-indexed. */
  page: number;
  /** Total number of pages (0 when totalCount = 0). */
  totalPages: number;
  /** Total item count after filters. */
  totalCount: number;
  /** Page size used for this query. */
  pageSize: number;
  className?: string;
}

/**
 * Offset-based pagination control (ADR 0013). Renders numbered page buttons
 * with ellipsis (`« 1 … 4 [5] 6 … 12 »`), a "Menampilkan X-Y dari Z data"
 * hint line, and keyboard-accessible prev/next chevrons.
 *
 * Hidden entirely when totalPages ≤ 1.
 */
export function Pagination({
  pathname,
  searchParams,
  page,
  totalPages,
  totalCount,
  pageSize,
  className,
}: PaginationProps) {
  if (totalPages <= 1) return null;

  const buildUrl = (targetPage: number) => {
    const p = new URLSearchParams();
    Object.entries(searchParams).forEach(([k, v]) => {
      if (k !== "page" && v !== undefined && v !== "") p.set(k, v);
    });
    if (targetPage > 1) p.set("page", String(targetPage));
    const qs = p.toString();
    return qs ? `${pathname}?${qs}` : pathname;
  };

  // Compute page numbers to show: first, last, current, and ±1 around current.
  // Insert null as an ellipsis token where there is a gap > 1.
  const pageNumbers = buildPageRange(page, totalPages);

  const start = (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, totalCount);

  const prevDisabled = page <= 1;
  const nextDisabled = page >= totalPages;

  return (
    <nav
      aria-label="Navigasi halaman"
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 pt-1 text-sm",
        className
      )}
    >
      {totalCount > 0 && (
        <p className="text-xs text-muted-foreground">
          Menampilkan {start}–{end} dari {totalCount} data
        </p>
      )}

      <div className="flex items-center gap-1">
        {/* Prev chevron */}
        {prevDisabled ? (
          <button
            disabled
            aria-label="Halaman sebelumnya"
            className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-input bg-background text-muted-foreground opacity-40 cursor-not-allowed"
          >
            <ChevronLeft size={14} aria-hidden="true" />
          </button>
        ) : (
          <Link
            href={buildUrl(page - 1)}
            aria-label="Halaman sebelumnya"
            className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-input bg-background text-foreground hover:bg-muted"
          >
            <ChevronLeft size={14} aria-hidden="true" />
          </Link>
        )}

        {/* Page number buttons */}
        {pageNumbers.map((item, idx) =>
          item === null ? (
            <span
              key={`ellipsis-${idx}`}
              aria-hidden="true"
              className="px-1 text-xs text-muted-foreground select-none"
            >
              …
            </span>
          ) : item === page ? (
            <button
              key={item}
              disabled
              aria-label={`Halaman ${item} (halaman saat ini)`}
              aria-current="page"
              className="inline-flex h-7 min-w-7 items-center justify-center rounded-md bg-primary px-2 text-xs font-semibold text-primary-foreground cursor-default"
            >
              {item}
            </button>
          ) : (
            <Link
              key={item}
              href={buildUrl(item)}
              aria-label={`Halaman ${item}`}
              className="inline-flex h-7 min-w-7 items-center justify-center rounded-md border border-input bg-background px-2 text-xs font-medium text-foreground hover:bg-muted"
            >
              {item}
            </Link>
          )
        )}

        {/* Next chevron */}
        {nextDisabled ? (
          <button
            disabled
            aria-label="Halaman berikutnya"
            className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-input bg-background text-muted-foreground opacity-40 cursor-not-allowed"
          >
            <ChevronRight size={14} aria-hidden="true" />
          </button>
        ) : (
          <Link
            href={buildUrl(page + 1)}
            aria-label="Halaman berikutnya"
            className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-input bg-background text-foreground hover:bg-muted"
          >
            <ChevronRight size={14} aria-hidden="true" />
          </Link>
        )}
      </div>
    </nav>
  );
}

/**
 * Build the visible page-number list with ellipsis (`null`) tokens.
 *
 * Always shows: page 1, page totalPages, currentPage, currentPage±1.
 * Inserts `null` where there is a gap larger than 1.
 */
function buildPageRange(
  current: number,
  total: number
): (number | null)[] {
  const visible = new Set<number>();
  visible.add(1);
  visible.add(total);
  visible.add(current);
  if (current - 1 >= 1) visible.add(current - 1);
  if (current + 1 <= total) visible.add(current + 1);

  const sorted = Array.from(visible).sort((a, b) => a - b);

  const result: (number | null)[] = [];
  for (let i = 0; i < sorted.length; i++) {
    if (i > 0 && sorted[i] - sorted[i - 1] > 1) {
      result.push(null); // ellipsis
    }
    result.push(sorted[i]);
  }
  return result;
}
