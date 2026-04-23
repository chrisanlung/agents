/**
 * relative-time.ts — Indonesian relative time formatter.
 *
 * No external dependency — uses only native Date arithmetic.
 * Formats past timestamps as "2 jam lalu", "3 hari lalu", etc.
 */

const SECOND = 1_000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

/**
 * Formats a past (or future) Date/ISO-string relative to now in Indonesian.
 *
 * Examples:
 *   "baru saja"        — < 45 seconds
 *   "1 menit lalu"     — 45s – 90s
 *   "5 menit lalu"     — 1.5m – 45m
 *   "1 jam lalu"       — 45m – 90m
 *   "3 jam lalu"       — 90m – 22h
 *   "kemarin"          — 22h – 36h
 *   "2 hari lalu"      — 36h – 6d
 *   "1 minggu lalu"    — 6d – 14d
 *   "2 minggu lalu"    — 14d – 25d
 *   "1 bulan lalu"     — 25d – 45d
 *   "3 bulan lalu"     — 45d – 345d
 *   "1 tahun lalu"     — 345d – 545d
 *   "2 tahun lalu"     — 545d+
 */
export function relativeTime(date: Date | string): string {
  const d = typeof date === "string" ? new Date(date) : date;
  const diffMs = Date.now() - d.getTime();
  const abs = Math.abs(diffMs);
  const suffix = diffMs >= 0 ? " lalu" : " lagi";

  if (abs < 45 * SECOND) return "baru saja";
  if (abs < 90 * SECOND) return `1 menit${suffix}`;
  if (abs < 45 * MINUTE) return `${Math.round(abs / MINUTE)} menit${suffix}`;
  if (abs < 90 * MINUTE) return `1 jam${suffix}`;
  if (abs < 22 * HOUR) return `${Math.round(abs / HOUR)} jam${suffix}`;
  if (abs < 36 * HOUR) return diffMs >= 0 ? "kemarin" : "besok";
  if (abs < 6 * DAY) return `${Math.round(abs / DAY)} hari${suffix}`;
  if (abs < 14 * DAY) return `1 minggu${suffix}`;
  if (abs < 25 * DAY) return `${Math.round(abs / WEEK)} minggu${suffix}`;
  if (abs < 45 * DAY) return `1 bulan${suffix}`;
  if (abs < 345 * DAY) return `${Math.round(abs / MONTH)} bulan${suffix}`;
  if (abs < 545 * DAY) return `1 tahun${suffix}`;
  return `${Math.round(abs / YEAR)} tahun${suffix}`;
}
