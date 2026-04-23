"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Copy } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type DayKey = "mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun";

interface DaySchedule {
  open: boolean;
  from: string; // "HH:MM"
  to: string;   // "HH:MM"
}

type Schedule = Record<DayKey, DaySchedule>;

const DAY_ORDER: DayKey[] = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];

const DAY_LABELS: Record<DayKey, string> = {
  mon: "Senin",
  tue: "Selasa",
  wed: "Rabu",
  thu: "Kamis",
  fri: "Jumat",
  sat: "Sabtu",
  sun: "Minggu",
};

const DEFAULT_SCHEDULE: Schedule = {
  mon: { open: true, from: "09:00", to: "17:00" },
  tue: { open: true, from: "09:00", to: "17:00" },
  wed: { open: true, from: "09:00", to: "17:00" },
  thu: { open: true, from: "09:00", to: "17:00" },
  fri: { open: true, from: "09:00", to: "17:00" },
  sat: { open: true, from: "09:00", to: "14:00" },
  sun: { open: false, from: "09:00", to: "17:00" },
};

/**
 * Parse a JSON string such as {"mon":"09:00-17:00","sun":null} into a Schedule.
 * Falls back to DEFAULT_SCHEDULE when the input is missing/invalid for that day.
 */
function parseSchedule(raw: string | undefined): Schedule {
  const out: Schedule = { ...DEFAULT_SCHEDULE };
  if (!raw) return out;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return out;
  }
  if (!parsed || typeof parsed !== "object") return out;
  const obj = parsed as Record<string, unknown>;
  for (const key of DAY_ORDER) {
    const v = obj[key];
    if (v === null || v === undefined) {
      out[key] = { ...out[key], open: false };
      continue;
    }
    if (typeof v === "string") {
      const m = v.match(/^(\d{2}):(\d{2})-(\d{2}):(\d{2})$/);
      if (m) {
        out[key] = { open: true, from: `${m[1]}:${m[2]}`, to: `${m[3]}:${m[4]}` };
      }
    }
  }
  return out;
}

/** Serialize Schedule back to JSON string. */
function serializeSchedule(s: Schedule): string {
  const out: Record<DayKey, string | null> = {
    mon: null, tue: null, wed: null, thu: null, fri: null, sat: null, sun: null,
  };
  for (const key of DAY_ORDER) {
    const d = s[key];
    out[key] = d.open ? `${d.from}-${d.to}` : null;
  }
  return JSON.stringify(out);
}

interface OperationalHoursFieldProps {
  /** Controlled value — JSON string as stored by react-hook-form */
  value?: string;
  /** Called with serialized JSON whenever the user edits any day */
  onChange: (json: string) => void;
  disabled?: boolean;
}

export function OperationalHoursField({
  value,
  onChange,
  disabled,
}: OperationalHoursFieldProps) {
  // Local UI state mirrors `value`; we only push back via onChange.
  const [schedule, setSchedule] = useState<Schedule>(() => parseSchedule(value));

  // Re-sync from outside when value changes and differs from our serialized state.
  // Prevents loss of edits that happen inside this component (we skip when serialized matches).
  const lastSerialized = useRef<string>(serializeSchedule(schedule));
  useEffect(() => {
    if (value === undefined) return;
    if (value === lastSerialized.current) return;
    const parsed = parseSchedule(value);
    setSchedule(parsed);
    lastSerialized.current = serializeSchedule(parsed);
  }, [value]);

  function update(key: DayKey, patch: Partial<DaySchedule>) {
    const next: Schedule = { ...schedule, [key]: { ...schedule[key], ...patch } };
    const json = serializeSchedule(next);
    lastSerialized.current = json;
    setSchedule(next);
    onChange(json);
  }

  function copyMondayToWeekdays() {
    const mon = schedule.mon;
    const next: Schedule = {
      ...schedule,
      tue: { ...mon },
      wed: { ...mon },
      thu: { ...mon },
      fri: { ...mon },
    };
    const json = serializeSchedule(next);
    lastSerialized.current = json;
    setSchedule(next);
    onChange(json);
  }

  const canCopyMondayToWeekdays = useMemo(() => schedule.mon.open, [schedule.mon.open]);

  return (
    <div className="space-y-2 rounded-lg border bg-card p-3">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          Atur jam operasional untuk setiap hari. Matikan toggle untuk hari tutup.
        </p>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled || !canCopyMondayToWeekdays}
          onClick={copyMondayToWeekdays}
          className="h-7 gap-1.5 text-xs"
        >
          <Copy size={12} aria-hidden="true" />
          Salin Senin ke Sel&ndash;Jum
        </Button>
      </div>

      <div className="divide-y">
        {DAY_ORDER.map((key) => {
          const day = schedule[key];
          return (
            <div
              key={key}
              className="grid grid-cols-[7rem_auto_1fr] items-center gap-3 py-2 sm:grid-cols-[8rem_auto_1fr]"
            >
              <span className="text-sm font-medium text-foreground">
                {DAY_LABELS[key]}
              </span>

              <label className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                <input
                  type="checkbox"
                  className="h-4 w-4 cursor-pointer rounded border-input accent-primary"
                  checked={day.open}
                  disabled={disabled}
                  onChange={(e) => update(key, { open: e.target.checked })}
                  aria-label={`${DAY_LABELS[key]} buka`}
                />
                <span className={cn(day.open ? "text-foreground" : "text-muted-foreground")}>
                  {day.open ? "Buka" : "Tutup"}
                </span>
              </label>

              <div
                className={cn(
                  "flex items-center gap-2 transition",
                  day.open ? "opacity-100" : "pointer-events-none opacity-40"
                )}
              >
                <Input
                  type="time"
                  value={day.from}
                  disabled={disabled || !day.open}
                  onChange={(e) => update(key, { from: e.target.value })}
                  className="w-28"
                  step={300}
                  aria-label={`${DAY_LABELS[key]} mulai`}
                />
                <span className="text-xs text-muted-foreground">&ndash;</span>
                <Input
                  type="time"
                  value={day.to}
                  disabled={disabled || !day.open}
                  onChange={(e) => update(key, { to: e.target.value })}
                  className="w-28"
                  step={300}
                  aria-label={`${DAY_LABELS[key]} selesai`}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
