/**
 * format.ts — shared formatting helpers for Lustia platform-admin.
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
 * Formats a Date as a full Indonesian long date string.
 * e.g. "Kamis, 1 Mei 2026"
 *
 * Uses Asia/Jakarta timezone via the "sv-SE" locale trick for the raw date,
 * then formats with "id-ID" for human-readable Indonesian output.
 */
export function formatLongDate(date: Date = new Date()): string {
  return date.toLocaleDateString("id-ID", {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "Asia/Jakarta",
  });
}

/**
 * Returns today's date in Asia/Jakarta as a YYYY-MM-DD string.
 * Uses Intl.DateTimeFormat with "sv-SE" locale which naturally produces
 * YYYY-MM-DD — the most reliable cross-platform approach.
 */
export function todayJakarta(): string {
  return new Intl.DateTimeFormat("sv-SE", {
    timeZone: "Asia/Jakarta",
  }).format(new Date());
}

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
