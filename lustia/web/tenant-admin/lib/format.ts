/**
 * format.ts — shared formatting helpers for Lustia tenant-admin.
 *
 * All helpers are pure functions with no side effects.
 */

/**
 * Format an IDR amount as "Rp X.XXX" using Indonesian locale conventions.
 * e.g. 1500000 → "Rp 1.500.000"
 */
export function formatRupiah(amount: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
}

/**
 * Format an ISO-8601 date or datetime string as a short Indonesian date.
 * e.g. "2026-04-30T10:00:00Z" → "30 Apr 2026"
 */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "—";
  return d.toLocaleDateString("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}
