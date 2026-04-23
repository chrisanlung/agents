"use client";

import { useEffect, useMemo, useRef, useState, useTransition } from "react";
import { Copy, Plus, X, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { saveAvailability } from "./[id]/availability-actions";
import type { AvailabilityWindow } from "@/lib/types";

// ─── Types ───────────────────────────────────────────────────────────────────

type DayKey = "sun" | "mon" | "tue" | "wed" | "thu" | "fri" | "sat";

/** dow follows DB convention: 0=Sun, 1=Mon, …, 6=Sat */
const DAY_ORDER: DayKey[] = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];
const DAY_DOW: Record<DayKey, number> = {
  sun: 0,
  mon: 1,
  tue: 2,
  wed: 3,
  thu: 4,
  fri: 5,
  sat: 6,
};
const DAY_LABELS: Record<DayKey, string> = {
  mon: "Senin",
  tue: "Selasa",
  wed: "Rabu",
  thu: "Kamis",
  fri: "Jumat",
  sat: "Sabtu",
  sun: "Minggu",
};

interface Window {
  start: string; // "HH:MM"
  end: string;   // "HH:MM"
}

interface DayWindows {
  active: boolean;
  windows: Window[];
}

type WeeklyAvailability = Record<DayKey, DayWindows>;

// ─── Helpers ─────────────────────────────────────────────────────────────────

const DEFAULT_WINDOW: Window = { start: "09:00", end: "17:00" };

const DEFAULT_AVAILABILITY: WeeklyAvailability = {
  mon: { active: false, windows: [{ ...DEFAULT_WINDOW }] },
  tue: { active: false, windows: [{ ...DEFAULT_WINDOW }] },
  wed: { active: false, windows: [{ ...DEFAULT_WINDOW }] },
  thu: { active: false, windows: [{ ...DEFAULT_WINDOW }] },
  fri: { active: false, windows: [{ ...DEFAULT_WINDOW }] },
  sat: { active: false, windows: [{ ...DEFAULT_WINDOW }] },
  sun: { active: false, windows: [{ ...DEFAULT_WINDOW }] },
};

function windowsToWeekly(apiWindows: AvailabilityWindow[]): WeeklyAvailability {
  const result: WeeklyAvailability = JSON.parse(JSON.stringify(DEFAULT_AVAILABILITY));

  // Group by dow
  const grouped: Partial<Record<DayKey, Window[]>> = {};
  for (const w of apiWindows) {
    const dayKey = Object.entries(DAY_DOW).find(([, v]) => v === w.dow)?.[0] as DayKey | undefined;
    if (!dayKey) continue;
    if (!grouped[dayKey]) grouped[dayKey] = [];
    grouped[dayKey]!.push({ start: w.start, end: w.end });
  }

  for (const [dk, wins] of Object.entries(grouped) as [DayKey, Window[]][]) {
    result[dk] = { active: true, windows: wins };
  }

  return result;
}

function weeklyToApiWindows(
  weekly: WeeklyAvailability
): Omit<AvailabilityWindow, "id">[] {
  const out: Omit<AvailabilityWindow, "id">[] = [];
  for (const dayKey of DAY_ORDER) {
    const day = weekly[dayKey];
    if (!day.active) continue;
    for (const w of day.windows) {
      out.push({ dow: DAY_DOW[dayKey], start: w.start, end: w.end });
    }
  }
  return out;
}

/** Validate a single day's windows. Returns error message or null. */
function validateDay(windows: Window[]): string | null {
  for (const w of windows) {
    if (w.start >= w.end) return "Waktu mulai harus sebelum waktu selesai.";
  }
  // Check overlaps (sort by start then check adjacent)
  const sorted = [...windows].sort((a, b) => a.start.localeCompare(b.start));
  for (let i = 1; i < sorted.length; i++) {
    if (sorted[i - 1].end > sorted[i].start) {
      return "Jendela waktu tidak boleh saling tumpang tindih.";
    }
  }
  return null;
}

// ─── Props ───────────────────────────────────────────────────────────────────

interface AvailabilityEditorProps {
  therapistId: string;
  initialWindows: AvailabilityWindow[];
}

// ─── Component ───────────────────────────────────────────────────────────────

export function AvailabilityEditor({
  therapistId,
  initialWindows,
}: AvailabilityEditorProps) {
  const [availability, setAvailability] = useState<WeeklyAvailability>(() =>
    windowsToWeekly(initialWindows)
  );
  const [errors, setErrors] = useState<Partial<Record<DayKey, string>>>({});
  const [isPending, startTransition] = useTransition();

  // Keep in sync with server-pushed refreshes
  const lastJson = useRef(JSON.stringify(initialWindows));
  useEffect(() => {
    const json = JSON.stringify(initialWindows);
    if (json === lastJson.current) return;
    lastJson.current = json;
    setAvailability(windowsToWeekly(initialWindows));
  }, [initialWindows]);

  // ── Mutations ─────────────────────────────────────────────────────────────

  function updateDay(dayKey: DayKey, patch: Partial<DayWindows>) {
    setAvailability((prev) => ({
      ...prev,
      [dayKey]: { ...prev[dayKey], ...patch },
    }));
    setErrors((prev) => ({ ...prev, [dayKey]: undefined }));
  }

  function updateWindow(dayKey: DayKey, idx: number, patch: Partial<Window>) {
    const day = availability[dayKey];
    const newWindows = day.windows.map((w, i) =>
      i === idx ? { ...w, ...patch } : w
    );
    updateDay(dayKey, { windows: newWindows });
  }

  function addWindow(dayKey: DayKey) {
    const day = availability[dayKey];
    if (day.windows.length >= 3) return;
    updateDay(dayKey, { windows: [...day.windows, { ...DEFAULT_WINDOW }] });
  }

  function removeWindow(dayKey: DayKey, idx: number) {
    const day = availability[dayKey];
    const newWindows = day.windows.filter((_, i) => i !== idx);
    updateDay(dayKey, {
      windows: newWindows.length > 0 ? newWindows : [{ ...DEFAULT_WINDOW }],
    });
  }

  // Copy Monday windows to Tue–Fri
  const canCopyMonday = useMemo(
    () => availability.mon.active && availability.mon.windows.length > 0,
    [availability.mon]
  );

  function copyMondayToWeekdays() {
    const mon = availability.mon;
    setAvailability((prev) => ({
      ...prev,
      tue: { ...mon, windows: mon.windows.map((w) => ({ ...w })) },
      wed: { ...mon, windows: mon.windows.map((w) => ({ ...w })) },
      thu: { ...mon, windows: mon.windows.map((w) => ({ ...w })) },
      fri: { ...mon, windows: mon.windows.map((w) => ({ ...w })) },
    }));
    setErrors({});
  }

  // ── Save ──────────────────────────────────────────────────────────────────

  function handleSave() {
    // Validate all active days
    const newErrors: Partial<Record<DayKey, string>> = {};
    for (const dayKey of DAY_ORDER) {
      const day = availability[dayKey];
      if (!day.active) continue;
      const err = validateDay(day.windows);
      if (err) newErrors[dayKey] = err;
    }

    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    const apiWindows = weeklyToApiWindows(availability);

    startTransition(async () => {
      const result = await saveAvailability(therapistId, apiWindows);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success("Jadwal ketersediaan disimpan.");
    });
  }

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div className="space-y-4">
      <div className="rounded-lg border bg-card p-4">
        <div className="mb-3 flex items-center justify-between gap-2">
          <p className="text-xs text-muted-foreground">
            Atur jadwal ketersediaan mingguan. Setiap hari dapat memiliki
            hingga 3 jendela waktu.
          </p>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={isPending || !canCopyMonday}
            onClick={copyMondayToWeekdays}
            className="h-7 gap-1.5 text-xs"
          >
            <Copy size={12} aria-hidden="true" />
            Salin ke hari kerja
          </Button>
        </div>

        <div className="divide-y">
          {DAY_ORDER.map((dayKey) => {
            const day = availability[dayKey];
            const dayError = errors[dayKey];

            return (
              <div key={dayKey} className="py-3">
                <div className="grid grid-cols-[8rem_auto_1fr] items-start gap-3">
                  {/* Day label */}
                  <span className="pt-1 text-sm font-medium text-foreground">
                    {DAY_LABELS[dayKey]}
                  </span>

                  {/* Active checkbox */}
                  <label className="inline-flex items-center gap-2 pt-1 text-xs text-muted-foreground">
                    <input
                      type="checkbox"
                      className="h-4 w-4 cursor-pointer rounded border-input accent-primary"
                      checked={day.active}
                      disabled={isPending}
                      onChange={(e) =>
                        updateDay(dayKey, { active: e.target.checked })
                      }
                      aria-label={`${DAY_LABELS[dayKey]} kerja`}
                    />
                    <span
                      className={cn(
                        day.active
                          ? "text-foreground"
                          : "text-muted-foreground"
                      )}
                    >
                      {day.active ? "Kerja" : "Tidak kerja"}
                    </span>
                  </label>

                  {/* Windows */}
                  <div
                    className={cn(
                      "space-y-2 transition-opacity duration-150",
                      day.active
                        ? "opacity-100"
                        : "pointer-events-none opacity-40"
                    )}
                  >
                    {day.windows.map((win, idx) => (
                      <div
                        key={idx}
                        className="flex flex-wrap items-center gap-2"
                      >
                        <Input
                          type="time"
                          value={win.start}
                          step={300}
                          disabled={isPending || !day.active}
                          onChange={(e) =>
                            updateWindow(dayKey, idx, { start: e.target.value })
                          }
                          className="w-28"
                          aria-label={`${DAY_LABELS[dayKey]} jendela ${idx + 1} mulai`}
                        />
                        <span className="text-xs text-muted-foreground">–</span>
                        <Input
                          type="time"
                          value={win.end}
                          step={300}
                          disabled={isPending || !day.active}
                          onChange={(e) =>
                            updateWindow(dayKey, idx, { end: e.target.value })
                          }
                          className="w-28"
                          aria-label={`${DAY_LABELS[dayKey]} jendela ${idx + 1} selesai`}
                        />
                        {day.windows.length > 1 && (
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive"
                            disabled={isPending || !day.active}
                            onClick={() => removeWindow(dayKey, idx)}
                            aria-label={`Hapus jendela ${win.start}–${win.end}`}
                          >
                            <X size={12} aria-hidden="true" />
                          </Button>
                        )}
                      </div>
                    ))}

                    {day.active && day.windows.length < 3 && (
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-7 gap-1 text-xs"
                        disabled={isPending}
                        onClick={() => addWindow(dayKey)}
                      >
                        <Plus size={12} aria-hidden="true" />
                        Tambah Jendela
                      </Button>
                    )}

                    {dayError && (
                      <p className="text-xs text-destructive">{dayError}</p>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <Button
        type="button"
        onClick={handleSave}
        disabled={isPending}
      >
        {isPending && (
          <Loader2 size={14} className="animate-spin" aria-hidden="true" />
        )}
        Simpan Ketersediaan
      </Button>
    </div>
  );
}
