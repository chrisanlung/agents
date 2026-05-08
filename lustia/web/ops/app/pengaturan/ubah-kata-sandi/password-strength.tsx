"use client";

import { cn } from "@/lib/utils";

export type StrengthLevel = 0 | 1 | 2 | 3;

const LEVELS: Array<{ label: string; bar: string; text: string }> = [
  { label: "Lemah", bar: "bg-red-500", text: "text-red-700" },
  { label: "Sedang", bar: "bg-amber-500", text: "text-amber-700" },
  { label: "Kuat", bar: "bg-emerald-500", text: "text-emerald-700" },
  { label: "Sangat Kuat", bar: "bg-emerald-700", text: "text-emerald-800" },
];

/**
 * Compute strength level [0..3] from four character-class signals + length.
 *   0 Lemah        : < 8 chars OR only one class
 *   1 Sedang       : 8+  chars AND 2 classes
 *   2 Kuat         : 10+ chars AND 3 classes
 *   3 Sangat Kuat  : 12+ chars AND 4 classes
 */
export function computeStrength(pw: string): StrengthLevel {
  if (!pw) return 0;
  const len = pw.length;
  const classes =
    Number(/[a-z]/.test(pw)) +
    Number(/[A-Z]/.test(pw)) +
    Number(/\d/.test(pw)) +
    Number(/[^a-zA-Z0-9]/.test(pw));

  if (len < 8 || classes <= 1) return 0;
  if (len >= 12 && classes >= 4) return 3;
  if (len >= 10 && classes >= 3) return 2;
  return 1;
}

interface PasswordStrengthProps {
  value: string;
}

export function PasswordStrength({ value }: PasswordStrengthProps) {
  if (!value) return null;

  const level = computeStrength(value);
  const info = LEVELS[level];

  return (
    <div className="mt-1 space-y-1.5" aria-live="polite">
      <div
        role="meter"
        aria-valuemin={0}
        aria-valuemax={3}
        aria-valuenow={level}
        aria-label="Kekuatan kata sandi"
        className="flex gap-1"
      >
        {[0, 1, 2, 3].map((i) => (
          <div
            key={i}
            className={cn(
              "h-1.5 flex-1 rounded-full transition-colors motion-reduce:transition-none",
              i <= level ? info.bar : "bg-muted"
            )}
          />
        ))}
      </div>
      <p className={cn("text-xs font-medium", info.text)}>
        <span className="sr-only">Kekuatan kata sandi: </span>
        {info.label}
      </p>
    </div>
  );
}
