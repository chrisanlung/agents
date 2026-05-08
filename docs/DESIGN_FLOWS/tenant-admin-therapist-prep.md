# Waktu Persiapan — Therapist Prep Buffer Input

Screen: `/master/therapists/[id]` — Jadwal tab

---

## 1. Why

`prep_minutes` is a per-therapist scheduling buffer that the booking engine
inserts immediately after each confirmed booking ends, blocking that window from
being offered to the next customer. It belongs on the Jadwal tab because that is
where operators reason about a therapist's time — the weekly grid and the buffer
are one mental model, not two. Surfacing it here avoids a separate settings
detour and makes the cause-and-effect (schedule + buffer = available slots)
visible in one place.

---

## 2. Placement

Above the weekly grid, inside the existing `<CardContent className="p-6">` that
wraps `<AvailabilityEditor>`. A small flat card (`rounded-lg border bg-card p-4`)
separates it visually from the grid card below without adding a full elevation
step.

```
┌──────────────────────────────────────────────────────────┐
│  Waktu Persiapan                    (card title, text-sm  │
│                                      font-medium)         │
│  ┌──────────┐                                             │
│  │  10      │ menit                                       │
│  └──────────┘                                             │
│  Buffer setelah tiap booking sebelum jam berikutnya       │
│  bisa di-book. Default 10 menit.                          │
└──────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────┐
│  [weekly grid — existing AvailabilityEditor]              │
└──────────────────────────────────────────────────────────┘

[Simpan Jadwal]  ← existing save button; unchanged
```

The prep card and the weekly grid card share `space-y-4` inherited from
`AvailabilityEditor`'s outer `<div className="space-y-4">`. No layout change
to the parent.

---

## 3. Input

```
<Input
  type="number"
  id="prep-minutes"
  min={0}
  max={60}
  step={5}
  defaultValue={10}
  placeholder="10"
  className="w-20"          // 80px — wide enough for two digits + spinner
  aria-describedby="prep-minutes-hint prep-minutes-error"
/>
<span aria-hidden="true" className="text-sm text-muted-foreground select-none">
  menit
</span>
```

The two elements sit in `<div className="flex items-center gap-2 mt-3">`.
`aria-hidden="true"` on the suffix span is correct — the unit is already
communicated in the `<label>` text ("Waktu Persiapan (menit)") so the span is
purely visual.

Label: `<label htmlFor="prep-minutes" className="text-sm font-medium
text-foreground">Waktu Persiapan (menit)</label>`. The "(menit)" qualifier in
the label makes the field unambiguous to screen-reader users even if focus lands
on the input before they read the suffix.

---

## 4. Helper text

```
<p id="prep-minutes-hint" className="mt-1.5 text-sm text-muted-foreground">
  Buffer setelah tiap booking sebelum jam berikutnya bisa di-book.
  Default 10 menit.
</p>
```

`text-sm text-muted-foreground` — matches `<FormDescription>` role in the
design system. No new token.

---

## 5. Validation

Schema: `z.number().int().min(0).max(60)`.

`step={5}` on the `<input>` is a UX nudge for spinner clicks — typing `12` or
`7` is valid and must not be blocked. Zod only enforces min/max/int.

Inline error (shown only after a blur or save attempt, not on every keystroke):

```
<p id="prep-minutes-error" role="alert"
   className="mt-1 text-xs text-destructive">
  0–60 menit.
</p>
```

The short message matches the existing error pattern in `availability-editor.tsx`
(`text-xs text-destructive`). No new style.

---

## 6. Save flow

**Decision: debounced auto-save, 1.5 s after last keystroke.**

Rationale: this is a single numeric field detached from the multi-day weekly
grid, which has its own explicit "Simpan Jadwal" button. Coupling prep_minutes
to that button would make two unrelated mutations fire together, complicating
rollback and progress feedback. A dedicated auto-save keeps the save scopes
independent, matches user expectation for single-field settings (the value you
type is the value that sticks), and avoids a second primary button competing for
attention beside "Simpan Jadwal".

Trade-off acknowledged: debounced saves can surprise users who navigate away
mid-edit. Mitigate with the toast and — if the network call is in-flight on
unmount — cancel the debounce and fire immediately (or warn). Document this
edge case for nextjs-expert.

Success toast: `toast.success("Waktu persiapan disimpan.")` — sonner, matches
existing toast calls in `availability-editor.tsx`.

---

## 7. Loading / error states

**Initial value:** fetched as part of `GET /tenant/therapists/:id` (the Server
Component already calls this). The `therapist` object should carry
`prep_minutes: number` (default 10 from the DB). No separate loading skeleton —
the whole Jadwal tab renders once the parent promise resolves.

**Save error:** `toast.error("Gagal menyimpan waktu persiapan. Coba lagi.")`
plus optimistic rollback — revert the input to its previous confirmed value.
Implement with a `useRef` holding the last successfully saved value; on error,
reset controlled state to that ref.

**Disabled state:** input and suffix inherit `disabled` while the debounce
request is in-flight. Use `aria-busy="true"` on the card container during the
request so assistive technology knows an update is pending.

---

## 8. Tokens used

All from the existing design system — no new tokens introduced.

| Token / class | Usage |
|---|---|
| `rounded-lg border bg-card p-4` | prep card container — matches schedule grid card |
| `text-sm font-medium text-foreground` | card title and field label |
| `text-sm text-muted-foreground` | helper text (FormDescription pattern) |
| `text-xs text-destructive` | inline validation error |
| `space-y-4` | vertical rhythm between prep card and grid card |
| `flex items-center gap-2` | input + suffix row |
| `mt-3`, `mt-1.5`, `mt-1` | internal vertical spacing (4px scale) |
| `w-20` | input width (80px) |
| `select-none` | prevent suffix "menit" from being text-selected with the input |
| `aria-hidden="true"` | decorative suffix span |
| `role="alert"` | error paragraph for AT live-region announcement |
| `aria-busy="true"` | card container during in-flight save |

---

## Cross-agent flags

- **go-expert / API_CONTRACT.md:** `prep_minutes` must be returned in
  `GET /tenant/therapists/:id` response and accepted in
  `PATCH /tenant/therapists/:id`. Confirm field name and default value (10)
  with go-expert before nextjs-expert implements the auto-save call.
- **nextjs-expert:** implement as a new `PrepMinutesInput` client component
  co-located in `app/master/therapists/`. Mount it inside `AvailabilityEditor`
  above the grid `<div className="rounded-lg border bg-card p-4">`. Debounce
  with a `useRef`-held `setTimeout` (no external library needed). Prop:
  `initialPrepMinutes: number`.
