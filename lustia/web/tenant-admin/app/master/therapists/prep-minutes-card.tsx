"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { z } from "zod";

import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { updateTherapistPrep } from "./[id]/prep-actions";

// ─── Schema ──────────────────────────────────────────────────────────────────

const prepSchema = z.number().int().min(0).max(60);

// ─── Props ───────────────────────────────────────────────────────────────────

interface PrepMinutesCardProps {
  therapistId: string;
  initialPrepMinutes: number;
}

// ─── Component ───────────────────────────────────────────────────────────────

export function PrepMinutesCard({
  therapistId,
  initialPrepMinutes,
}: PrepMinutesCardProps) {
  // Controlled value as a string so the input stays responsive while typing.
  const [inputValue, setInputValue] = useState(String(initialPrepMinutes));
  // Last successfully saved value — used for optimistic rollback on error.
  const lastSavedRef = useRef(initialPrepMinutes);
  // Whether a PATCH is currently in-flight.
  const [isSaving, setIsSaving] = useState(false);
  // Inline validation error, shown on blur or after a failed save attempt.
  const [error, setError] = useState<string | null>(null);
  // Whether the user has blurred the field at least once (controls error visibility).
  const [touched, setTouched] = useState(false);

  // Hold the pending debounce timer.
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Hold the pending save value so the beforeunload handler can flush it.
  const pendingSaveRef = useRef<number | null>(null);

  // ── Validation ────────────────────────────────────────────────────────────

  function validate(raw: string): number | null {
    const num = parseInt(raw, 10);
    if (isNaN(num)) return null;
    const result = prepSchema.safeParse(num);
    return result.success ? num : null;
  }

  // ── Core save (fire-and-forget; handles optimistic rollback) ─────────────

  const performSave = useCallback(
    async (value: number) => {
      pendingSaveRef.current = null;
      setIsSaving(true);
      const result = await updateTherapistPrep(therapistId, value);
      setIsSaving(false);

      if (result.ok) {
        lastSavedRef.current = result.prep_minutes;
        toast.success("Waktu persiapan disimpan.");
      } else {
        // Optimistic rollback — revert to last known-good value.
        setInputValue(String(lastSavedRef.current));
        setError(null);
        toast.error(result.error || "Gagal menyimpan waktu persiapan. Coba lagi.");
      }
    },
    [therapistId]
  );

  // ── Debounced auto-save (1.5s after last keystroke) ───────────────────────

  function scheduleDebounce(value: number) {
    if (debounceRef.current !== null) clearTimeout(debounceRef.current);
    pendingSaveRef.current = value;
    debounceRef.current = setTimeout(() => {
      debounceRef.current = null;
      performSave(value);
    }, 1500);
  }

  // ── Navigate-away guard: warn the user if a debounce is pending.
  // v1 choice: native beforeunload prompt. This avoids holding a
  // reference to the Server Action in a sync flush (which is disallowed
  // in Next.js App Router for security). The prompt fires for both
  // tab-close and client-side navigation initiated outside React.
  useEffect(() => {
    function handleBeforeUnload(e: BeforeUnloadEvent) {
      if (pendingSaveRef.current !== null || isSaving) {
        // Returning any string triggers the browser native confirm dialog.
        e.preventDefault();
        e.returnValue = "";
      }
    }

    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
      // Cancel any pending debounce on unmount to prevent stale calls.
      if (debounceRef.current !== null) {
        clearTimeout(debounceRef.current);
        debounceRef.current = null;
        pendingSaveRef.current = null;
      }
    };
  }, [isSaving]);

  // ── Handlers ──────────────────────────────────────────────────────────────

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const raw = e.target.value;
    setInputValue(raw);

    const valid = validate(raw);
    if (valid !== null) {
      // Clear any existing error and schedule save.
      setError(null);
      scheduleDebounce(valid);
    } else {
      // Cancel any pending save for an invalid value.
      if (debounceRef.current !== null) clearTimeout(debounceRef.current);
      debounceRef.current = null;
      pendingSaveRef.current = null;
    }
  }

  function handleBlur() {
    setTouched(true);
    if (validate(inputValue) === null) {
      setError("0–60 menit.");
    } else {
      setError(null);
    }
  }

  const showError = (touched || isSaving) && error !== null;

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div
      className="rounded-lg border bg-card p-4"
      aria-busy={isSaving ? "true" : "false"}
    >
      <label
        htmlFor="prep-minutes"
        className="text-sm font-medium text-foreground"
      >
        Waktu Persiapan (menit)
      </label>

      <div className="mt-3 flex items-center gap-2">
        <Input
          id="prep-minutes"
          type="number"
          min={0}
          max={60}
          step={5}
          value={inputValue}
          onChange={handleChange}
          onBlur={handleBlur}
          disabled={isSaving}
          placeholder="10"
          className={cn("w-20", showError && "border-destructive focus-visible:ring-destructive")}
          aria-describedby="prep-minutes-hint prep-minutes-error"
          aria-invalid={showError ? "true" : "false"}
        />
        <span
          aria-hidden="true"
          className="text-sm text-muted-foreground select-none"
        >
          menit
        </span>
      </div>

      <p
        id="prep-minutes-hint"
        className="mt-1.5 text-sm text-muted-foreground"
      >
        Buffer setelah tiap booking sebelum jam berikutnya bisa di-book.
        Default 10 menit.
      </p>

      {showError && (
        <p
          id="prep-minutes-error"
          role="alert"
          className="mt-1 text-xs text-destructive"
        >
          {error}
        </p>
      )}

      {/* When error is not shown we still render the element (hidden) so
          aria-describedby always resolves to a DOM node. */}
      {!showError && (
        <p id="prep-minutes-error" className="sr-only" aria-hidden="true" />
      )}
    </div>
  );
}
