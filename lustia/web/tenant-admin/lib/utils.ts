import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// 5 accent shades that stay readable as <Badge> tinted backgrounds. Ordered so
// the most common category colors (emerald/sky) show first; remaining hash
// buckets cycle through.
const CATEGORY_PALETTE = [
  "bg-emerald-50 text-emerald-800 border-emerald-200",
  "bg-sky-50 text-sky-800 border-sky-200",
  "bg-violet-50 text-violet-800 border-violet-200",
  "bg-amber-50 text-amber-900 border-amber-200",
  "bg-rose-50 text-rose-800 border-rose-200",
];

/**
 * Returns a deterministic Tailwind class string to color a category badge. Two
 * services with the same category always get the same color; different
 * categories cycle through the palette. Input is lowercased + trimmed so
 * "Pijat" and "pijat" don't split into different colors.
 */
export function categoryColorClass(category: string | null | undefined): string {
  if (!category) return "bg-muted text-muted-foreground border-border";
  const normalized = category.toLowerCase().trim();
  let hash = 0;
  for (let i = 0; i < normalized.length; i++) {
    hash = (hash * 31 + normalized.charCodeAt(i)) >>> 0;
  }
  return CATEGORY_PALETTE[hash % CATEGORY_PALETTE.length];
}
