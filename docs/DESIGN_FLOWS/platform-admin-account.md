# Platform-Admin Account Pages — Design Spec

_Owned by `ui-ux-expert`. Implemented by `nextjs-expert`. Backend verification included in §6._
_Status: Ready for review. Approved spec unlocks nextjs-expert implementation._

---

## 1. Overview

Three routes replace the current placeholder stubs in the platform-admin portal: `/profil`, `/pengaturan`, and `/pengaturan/ubah-kata-sandi`. Together they give the super-admin complete self-service over their identity and credentials — the same capability that tenant-admin users already have, adapted for the platform-admin context.

The profil page lets the super-admin update their display name, phone number, and optionally their avatar. The pengaturan landing page acts as a hub with one live tile (change password) and two disabled future tiles, so the page feels purposeful rather than empty on day one. The change-password page is a direct port from tenant-admin — same form, same validation, same success/error states — with only the header component and redirect target swapped out for the platform-admin equivalents.

All three pages reuse existing platform-admin tokens and components. No new design tokens are introduced.

---

## 2. ASCII Mockups

### 2A. `/profil` — Desktop (1280px, max-w-3xl container)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ [shield] Lustia Platform Console  [Dasbor][Tenant][Payout][Registrasi]  │
│                                                             [● BS ˅]    │
└─────────────────────────────────────────────────────────────────────────┘

  Profil Saya
  Kelola informasi akun Anda.

  ┌──────────────────────────────────────────────────────────────────┐
  │  Foto Profil                                                     │
  │  ┌──────────────┐                                                │
  │  │  [BS]        │  Budi Santoso                                  │
  │  │  bg-primary  │  Inisial dari nama lengkap                     │
  │  └──────────────┘  [Segera hadir]  ← muted badge / hint         │
  │  ─────────────────────────────────────────────────────────────── │
  │  Email                                                           │
  │  budi@lustia.id                                                  │
  │  Email tidak dapat diubah setelah akun dibuat.                   │
  │  ─────────────────────────────────────────────────────────────── │
  │  Nama Lengkap *                                                  │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │ Budi Santoso                                             │   │
  │  └──────────────────────────────────────────────────────────┘   │
  │  ─────────────────────────────────────────────────────────────── │
  │  Nomor Telepon                                                   │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │ 08123456789                                              │   │
  │  └──────────────────────────────────────────────────────────┘   │
  │  Hanya angka (5–30 digit)                                        │
  │  ─────────────────────────────────────────────────────────────── │
  │                    [Batal]  [Simpan Perubahan ▸]                 │
  └──────────────────────────────────────────────────────────────────┘

  ← Kembali ke Dasbor
```

### 2B. `/profil` — Mobile (375px)

```
┌─────────────────────────────────────────────────────────────┐
│ [shield] Lustia Platform Console             [● BS ˅]       │
└─────────────────────────────────────────────────────────────┘

  Profil Saya
  Kelola informasi akun Anda.

  ┌──────────────────────────────────────────────────────┐
  │  ┌──────────┐  Budi Santoso                          │
  │  │  [BS]    │  [Segera hadir]                        │
  │  └──────────┘                                        │
  │  Email                                               │
  │  budi@lustia.id                                      │
  │  Email tidak dapat diubah setelah akun dibuat.       │
  │                                                      │
  │  Nama Lengkap *                                      │
  │  ┌──────────────────────────────────────────────┐   │
  │  │ Budi Santoso                                 │   │
  │  └──────────────────────────────────────────────┘   │
  │                                                      │
  │  Nomor Telepon                                       │
  │  ┌──────────────────────────────────────────────┐   │
  │  │ 08123456789                                  │   │
  │  └──────────────────────────────────────────────┘   │
  │  Hanya angka (5–30 digit)                            │
  │                                                      │
  │  [Batal — full width outlined]                       │
  │  [Simpan Perubahan — full width filled]              │
  └──────────────────────────────────────────────────────┘

  ← Kembali ke Dasbor
```

---

### 2C. `/pengaturan` — Desktop

```
┌─────────────────────────────────────────────────────────────────────────┐
│ [shield] Lustia Platform Console  [Dasbor][Tenant][Payout][Registrasi]  │
│                                                             [● BS ˅]    │
└─────────────────────────────────────────────────────────────────────────┘

  Pengaturan
  Kelola keamanan dan preferensi akun Anda.

  ──── Keamanan ─────────────────────────────────────────────────────

  ┌──────────────────────────┐
  │ [Lock icon]              │
  │ Ubah Kata Sandi          │
  │ Perbarui kata sandi akun │
  │ Anda secara berkala untuk│
  │ keamanan.                │
  │                       [›]│
  └──────────────────────────┘

  ──── Segera Hadir ─────────────────────────────────────────────────

  ┌──────────────────────────┐  ┌──────────────────────────┐  ┌──────────────────────────┐
  │ [Bell icon]  muted       │  │ [Activity] muted         │  │ [Key icon] muted         │
  │ Notifikasi               │  │ Log Aktivitas            │  │ API Keys                 │
  │ Konfigurasi notifikasi   │  │ Riwayat aktivitas        │  │ Kelola kunci API untuk   │
  │ sistem.                  │  │ akun Anda.               │  │ integrasi.               │
  │  [Segera hadir]          │  │  [Segera hadir]          │  │  [Segera hadir]          │
  └──────────────────────────┘  └──────────────────────────┘  └──────────────────────────┘
     aria-disabled                 aria-disabled                 aria-disabled
```

### 2D. `/pengaturan` — Mobile (375px)

```
┌─────────────────────────────────────────────────────────────┐
│ [shield] Lustia Platform Console             [● BS ˅]       │
└─────────────────────────────────────────────────────────────┘

  Pengaturan
  Kelola keamanan dan preferensi akun Anda.

  Keamanan
  ┌──────────────────────────────────────────────────────┐
  │ [Lock]  Ubah Kata Sandi                          [›] │
  │ Perbarui kata sandi akun Anda secara berkala         │
  │ untuk keamanan.                                      │
  └──────────────────────────────────────────────────────┘

  Segera Hadir
  ┌──────────────────────────────────────────────────────┐
  │ [Bell]  Notifikasi            [Segera hadir]  [muted]│
  └──────────────────────────────────────────────────────┘
  ┌──────────────────────────────────────────────────────┐
  │ [Activity]  Log Aktivitas     [Segera hadir]  [muted]│
  └──────────────────────────────────────────────────────┘
  ┌──────────────────────────────────────────────────────┐
  │ [Key]  API Keys               [Segera hadir]  [muted]│
  └──────────────────────────────────────────────────────┘
```

---

### 2E. `/pengaturan/ubah-kata-sandi` — Desktop

```
┌─────────────────────────────────────────────────────────────────────────┐
│ [shield] Lustia Platform Console  [Dasbor][Tenant][Payout][Registrasi]  │
│                                                             [● BS ˅]    │
└─────────────────────────────────────────────────────────────────────────┘

  [amber banner — only when forced (must_change_password=true)]
  ┌──────────────────────────────────────────────────────────────────────┐
  │ ⚠ Anda menggunakan kata sandi sementara                             │
  │   Buat kata sandi baru untuk melanjutkan ke akun Anda.              │
  └──────────────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────────┐
  │ ┌───────┐  Ubah Kata Sandi                                       │
  │ │[key] │  Untuk keamanan akun Anda, gunakan kata sandi          │
  │ └───────┘  yang kuat dan unik.                                   │
  │ ─────────────────────────────────────────────────────────────── │
  │  Kata Sandi Saat Ini                                             │
  │  ┌──────────────────────────────────────┐  [eye icon]           │
  │  │ ••••••••                             │                        │
  │  └──────────────────────────────────────┘                        │
  │  Kata Sandi Baru                                                 │
  │  ┌──────────────────────────────────────┐  [eye icon]           │
  │  │ ••••••••                             │                        │
  │  └──────────────────────────────────────┘                        │
  │  [■■□□] Sedang  ← PasswordStrength meter                        │
  │  Konfirmasi Kata Sandi Baru                                      │
  │  ┌──────────────────────────────────────┐  [eye icon]           │
  │  │ ••••••••                             │                        │
  │  └──────────────────────────────────────┘                        │
  │  ─────────────────────────────────────────────────────────────── │
  │  [Batal]                   [Simpan Kata Sandi Baru]             │
  └──────────────────────────────────────────────────────────────────┘
```

### 2F. Success state (replaces form)

```
  ┌──────────────────────────────────────────────────────────────────┐
  │            ┌─────────────────────────────┐                       │
  │            │ ✓  (emerald circle)         │                       │
  │            └─────────────────────────────┘                       │
  │            Kata sandi berhasil diubah                            │
  │            Demi keamanan, semua sesi Anda telah dikeluarkan.     │
  │            Silakan masuk kembali menggunakan kata sandi baru.    │
  │                                                                  │
  │                      [Masuk Kembali]                             │
  └──────────────────────────────────────────────────────────────────┘
```

---

## 3. Section-by-Section Spec

### 3.1 `/profil`

#### 3.1.1 Page shell

```
<div className="min-h-screen bg-slate-50">
  <ConsoleHeader user={user} pendingRegistrationCount={pendingCount} />
  <main className="mx-auto max-w-3xl space-y-6 px-6 py-8">
    ...
  </main>
</div>
```

- Container: `max-w-3xl` — narrower than the dashboard's `max-w-5xl`. Profile forms benefit from a tighter line measure; a narrower container signals "focused task" to the user.
- `ConsoleHeader` requires `user` and `pendingRegistrationCount`. Both come from the `/auth/me` fetch performed at the top of the Server Component.

#### 3.1.2 Page header strip

```
<div>
  <h1 className="text-2xl font-semibold tracking-tight text-foreground">
    Profil Saya
  </h1>
  <p className="mt-1 text-sm text-muted-foreground">
    Kelola informasi akun Anda.
  </p>
</div>
```

Token refs: `text-foreground` (BS-T4 `on-surface`), `text-muted-foreground` (BS-T4 `on-surface-variant`), `text-2xl font-semibold` = heading-md tier.

#### 3.1.3 Profile card + form

**Card shell:**
```
<Card className="bg-white shadow-sm">
  <CardContent className="p-6">
    <form action={updateProfileAction} className="space-y-6">
      ...
    </form>
  </CardContent>
</Card>
```

**Avatar section (v1 — placeholder treatment):**

```
<div className="flex items-center gap-4">
  <div className="flex h-16 w-16 shrink-0 items-center justify-center
                  rounded-full bg-primary/10 text-primary
                  text-xl font-semibold ring-2 ring-border"
       aria-hidden="true">
    {initials}
  </div>
  <div>
    <p className="text-sm font-medium text-foreground">{user.full_name}</p>
    <p className="mt-1 text-xs text-muted-foreground">
      Perubahan foto profil segera hadir.
    </p>
  </div>
</div>
<hr className="border-border" />
```

- Avatar initials computed same as `getInitials()` in `platform-user-menu.tsx` (at most 2 initials from full name).
- `h-16 w-16` (64px) — larger than the header's 36px to give it visual weight as the page's identity anchor.
- Avatar upload is deferred. Mark with plain text "Perubahan foto profil segera hadir." — no disabled button, no separate badge. A button would imply interactivity; muted hint text does not.
- Separator `<hr>` between the avatar section and fields.

**Email field (read-only):**

```
<div className="space-y-1.5">
  <label className="text-sm font-medium text-foreground">Email</label>
  <div className="flex h-10 items-center rounded-md border bg-muted px-3
                  text-sm text-muted-foreground">
    {user.email}
  </div>
  <p className="text-xs text-muted-foreground">
    Email tidak dapat diubah setelah akun dibuat.
  </p>
</div>
```

- Rendered as a styled `<div>`, not an `<input>`. Prevents accidental editing. Screen readers encounter plain text in context, which is correct — there is nothing to do here.
- `bg-muted` signals the field is inert. Copy is identical to tenant-admin.

**Nama Lengkap field (required):**

```
<FormField
  control={form.control}
  name="full_name"
  render={({ field }) => (
    <FormItem>
      <FormLabel>
        Nama Lengkap
        <span className="ml-1 text-destructive" aria-hidden="true">*</span>
      </FormLabel>
      <FormControl>
        <Input
          type="text"
          placeholder="Nama lengkap Anda"
          autoComplete="name"
          maxLength={200}
          {...field}
        />
      </FormControl>
      <FormMessage />
    </FormItem>
  )}
/>
```

- Required. Zod: `z.string().min(1, "Nama wajib diisi").max(200, "Nama terlalu panjang")`.
- `maxLength={200}` attribute on the input prevents typing beyond the server-side limit.
- Asterisk is `aria-hidden="true"` because the `required` attribute on the input already conveys requirement to screen readers.

**Nomor Telepon field (optional):**

```
<FormField
  control={form.control}
  name="phone"
  render={({ field }) => (
    <FormItem>
      <FormLabel>Nomor Telepon</FormLabel>
      <FormControl>
        <Input
          type="tel"
          placeholder="Contoh: 08123456789"
          autoComplete="tel"
          inputMode="numeric"
          maxLength={30}
          {...field}
          onChange={(e) => {
            // Digit-only filter: strip non-digit characters on input
            field.onChange(e.target.value.replace(/\D/g, ""));
          }}
        />
      </FormControl>
      <p className="text-xs text-muted-foreground">
        Hanya angka (5–30 digit)
      </p>
      <FormMessage />
    </FormItem>
  )}
/>
```

- Optional. Zod: `z.string().regex(/^\d{5,30}$/, "Nomor telepon harus 5–30 digit angka").optional().or(z.literal(""))`.
- `inputMode="numeric"` opens the numeric keyboard on mobile without a number spinner.
- Digit-only filter applied `onChange` so the field never contains non-digit characters.

**Action buttons:**

```
<div className="flex flex-col-reverse gap-3 pt-2 sm:flex-row sm:justify-end">
  <Button
    type="button"
    variant="outline"
    onClick={() => router.push("/dashboard")}
    disabled={isPending}
  >
    Batal
  </Button>
  <Button
    type="submit"
    disabled={isPending || !isDirty}
    aria-busy={isPending}
  >
    {isPending ? (
      <>
        <Loader2 className="mr-2 animate-spin" size={16} aria-hidden="true" />
        Menyimpan…
      </>
    ) : (
      "Simpan Perubahan"
    )}
  </Button>
</div>
```

- Desktop: row layout, right-aligned. Mobile: full-width column (reversed order so primary action is below the secondary, which matches the reading flow on a small screen).
- "Simpan Perubahan" is `disabled` when `!isDirty` (no field has changed from the initial value). `isDirty` comes from RHF's `formState.isDirty`.
- "Batal" navigates to `/dashboard`. On mobile, navigating back is the primary way to dismiss, so placing "Batal" as the second button (visually first in the column) is consistent with mobile conventions.

#### 3.1.4 States

| State | Behaviour |
|---|---|
| Loading (initial data fetch) | Replace card content with skeleton: `h-16 w-16 rounded-full animate-pulse bg-muted` for avatar, three `h-9 rounded-md animate-pulse bg-muted` for inputs. Use `motion-safe:animate-pulse`. |
| Default (form loaded, no changes) | "Simpan Perubahan" button disabled (`!isDirty`). |
| Dirty (user edited a field) | "Simpan Perubahan" button enabled. |
| Submitting | `isPending=true`. Button shows spinner + "Menyimpan…". Both buttons `disabled`. |
| Success | Sonner green toast: "Profil berhasil diperbarui." Form's `isDirty` resets to false (RHF `reset(updatedValues)`). Buttons return to default state. |
| Validation error (client-side) | Inline `FormMessage` below each failing field. |
| Server error — wrong credentials / field error | `FormMessage` on the relevant field via `form.setError`. |
| Server error — generic | Inline alert above the form: `role="alert"` banner with destructive border/bg, icon `AlertTriangle`, and message "Terjadi kesalahan. Coba lagi." |
| 401 (session expired mid-edit) | `redirect("/login")` in Server Action. |

#### 3.1.5 Back link

```
<Link
  href="/dashboard"
  className="inline-flex items-center gap-1 text-sm text-muted-foreground
             hover:text-foreground underline-offset-4 hover:underline"
>
  <ChevronLeft size={14} aria-hidden="true" />
  Kembali ke Dasbor
</Link>
```

Sits below the Card, outside `<CardContent>`. Light treatment; not a primary action.

---

### 3.2 `/pengaturan`

#### 3.2.1 Page shell

Same `min-h-screen bg-slate-50` wrapper with `ConsoleHeader`. Container: `max-w-3xl space-y-8 px-6 py-8`. Slightly wider spacing (`space-y-8`) compared to `/profil` because tile groups need more breathing room between sections.

#### 3.2.2 Page header strip

```
<div>
  <h1 className="text-2xl font-semibold tracking-tight text-foreground">
    Pengaturan
  </h1>
  <p className="mt-1 text-sm text-muted-foreground">
    Kelola keamanan dan preferensi akun Anda.
  </p>
</div>
```

#### 3.2.3 "Keamanan" section

```
<section aria-labelledby="security-heading">
  <h2 id="security-heading"
      className="mb-3 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
    Keamanan
  </h2>
  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
    <SettingsTile
      icon={Lock}
      title="Ubah Kata Sandi"
      description="Perbarui kata sandi akun Anda secara berkala untuk keamanan."
      href="/pengaturan/ubah-kata-sandi"
    />
  </div>
</section>
```

**`SettingsTile` anatomy (live variant):**

```
<Link
  href={href}
  className="group flex flex-col gap-3 rounded-lg border bg-card p-5
             shadow-sm transition-shadow
             hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring"
>
  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10">
    <Icon size={20} className="text-primary" aria-hidden="true" />
  </div>
  <div>
    <p className="text-sm font-semibold text-foreground">{title}</p>
    <p className="mt-1 text-xs text-muted-foreground line-clamp-3">{description}</p>
  </div>
  <ChevronRight
    size={16}
    className="mt-auto self-end text-muted-foreground/60
               group-hover:text-foreground transition-colors"
    aria-hidden="true"
  />
</Link>
```

Token refs: `bg-primary/10` (PA-T1 icon bg), `text-primary` (PA-T1 icon color), `bg-card`, `shadow-sm`, `focus-visible:ring-ring`, `text-muted-foreground`.

**`SettingsTile` anatomy (disabled / "Segera hadir" variant):**

```
<div
  className="flex flex-col gap-3 rounded-lg border bg-card p-5
             shadow-sm opacity-50 cursor-not-allowed"
  aria-disabled="true"
  role="article"
  aria-label="{title} — segera hadir"
>
  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
    <Icon size={20} className="text-muted-foreground" aria-hidden="true" />
  </div>
  <div>
    <p className="text-sm font-semibold text-foreground">{title}</p>
    <p className="mt-1 text-xs text-muted-foreground line-clamp-3">{description}</p>
  </div>
  <span className="mt-auto self-start inline-flex items-center rounded-full
                   bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
    Segera hadir
  </span>
</div>
```

- Use `<div>` not `<Link>` for disabled tiles — `<a href="">` with `aria-disabled` still receives keyboard focus and registers a click; a plain `<div role="article">` does not and cannot be accidentally activated.
- `opacity-50` communicates "not yet available" without cluttering the layout with a separate explanatory paragraph.
- The "Segera hadir" pill is redundant with `opacity-50` for sighted users but provides a text signal for anyone using a screen reader at high zoom.
- No hover shadow on the disabled tile (no `:hover` styles).

#### 3.2.4 "Segera Hadir" section

```
<section aria-labelledby="coming-soon-heading">
  <h2 id="coming-soon-heading"
      className="mb-3 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
    Segera Hadir
  </h2>
  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
    <SettingsTileDisabled
      icon={Bell}
      title="Notifikasi"
      description="Konfigurasi notifikasi email dan sistem."
    />
    <SettingsTileDisabled
      icon={Activity}
      title="Log Aktivitas"
      description="Riwayat aktivitas akun Anda."
    />
    <SettingsTileDisabled
      icon={KeyRound}
      title="API Keys"
      description="Kelola kunci API untuk integrasi pihak ketiga."
    />
  </div>
</section>
```

Icons: `Bell`, `Activity`, `KeyRound` — all from `lucide-react`, already vendored.

#### 3.2.5 Accessibility notes for tiles

- Live tile: full `<Link>` — natively keyboard-focusable. Focus ring: `focus-visible:ring-2 focus-visible:ring-ring`. WCAG 2.4.7 satisfied.
- Disabled tiles: `<div>` with `aria-disabled="true"`. They do NOT appear in the tab sequence, which is correct — a user should not be able to focus a non-functional control. WCAG 4.1.2 satisfied by the `role="article"` + `aria-label` pattern.
- Section headings use `aria-labelledby` pointing to the `<h2>` — landmark regions are properly named.

---

### 3.3 `/pengaturan/ubah-kata-sandi`

This page is a direct port from `lustia/web/tenant-admin/app/pengaturan/ubah-kata-sandi/`. The following documents what stays identical and what changes.

#### 3.3.1 What stays identical (copy from tenant-admin verbatim)

- `ChangePasswordForm` component: all three fields (`old_password`, `new_password`, `confirm_password`), `EyeToggle`, `PasswordStrength` meter, Zod schema, `useActionState` + `useTransition` wiring, `useEffect` for server-side field error surfacing, the success state (emerald circle + copy + "Masuk Kembali" button), the forced-mode amber banner, and the "keluar dari akun ini" escape hatch.
- Copy: all Indonesian label, placeholder, error, and success strings are unchanged.
- Validation rules: min 8, max 128, confirm-match, no-same-as-current checks.
- `PasswordStrength` component: four-segment meter with Lemah / Sedang / Kuat / Sangat Kuat labels and `aria-live="polite"`.

#### 3.3.2 What changes

| Element | Tenant-admin value | Platform-admin value |
|---|---|---|
| Header component | `<AppBackground><AppHeader ...>` | `<div className="min-h-screen bg-slate-50"><ConsoleHeader ...>` |
| `changePasswordAction` import path | `./actions` (same file co-located in the same folder) | Same — the platform-admin version of the action lives in its own `app/pengaturan/ubah-kata-sandi/actions.ts` |
| API endpoint called | `POST /auth/me/password` | `POST /auth/me/password` — **identical** (verified in §6) |
| `forcedLogoutAction` | `./actions` | Same — platform-admin version calls `/auth/logout` + `clearSessionCookies()` |
| Post-success redirect | `/login` (after `setTimeout 2500ms`) | `/login` — **identical** |
| "Batal" button destination | `router.back()` | `router.back()` — **identical** |
| `must_change_password` banner | Banner shown on this page if `forced=true` | **Identical** — same amber banner markup and copy |
| Banner link on `/dashboard` | Currently reads "Segera hadir" (span) | **Must be updated** to `<Link href="/pengaturan/ubah-kata-sandi?reason=required">` once this route exists (see §7.3) |

#### 3.3.3 Page shell

```
<div className="min-h-screen bg-slate-50">
  <ConsoleHeader user={user} pendingRegistrationCount={pendingCount} />
  <main className="mx-auto max-w-lg px-6 py-8">
    {forced && (
      <div role="alert" className="mb-4 flex items-start gap-3 rounded-lg
           border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900">
        <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-600"
                       aria-hidden="true" />
        <div>
          <p className="font-medium">Anda menggunakan kata sandi sementara</p>
          <p className="mt-1 text-amber-800">
            Buat kata sandi baru untuk melanjutkan ke akun Anda.
          </p>
        </div>
      </div>
    )}
    <Card className="bg-white/95 shadow-sm backdrop-blur-sm">
      <CardHeader>
        <div className="flex items-center gap-2">
          <div className="flex h-9 w-9 items-center justify-center
                          rounded-lg bg-primary/10 text-primary">
            <KeyRound size={18} aria-hidden="true" />
          </div>
          <div>
            <CardTitle className="text-lg">
              {forced ? "Buat Kata Sandi Baru" : "Ubah Kata Sandi"}
            </CardTitle>
            <CardDescription>
              {forced
                ? "Kata sandi sementara hanya bisa dipakai sekali."
                : "Untuk keamanan akun Anda, gunakan kata sandi yang kuat dan unik."}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <ChangePasswordForm forced={forced} />
      </CardContent>
    </Card>
  </main>
</div>
```

Note: `bg-primary/10` for the `KeyRound` icon container. In platform-admin the primary color is indigo, so this will render as `indigo-50`/`indigo-600` — intentionally matching the platform-admin token rather than the `bg-pink-100` from tenant-admin.

#### 3.3.4 `forced` detection

Identical to tenant-admin: read `must_change_password` from the JWT via `getAccessToken()` + `decodeJWTPayload`. Fall back to `?reason=required` query param as a secondary signal. No change to this logic.

#### 3.3.5 States

All states are identical to tenant-admin. See `change-password-form.tsx` for the canonical implementation. Key states:

| State | Behaviour |
|---|---|
| Default | Form with three fields and eye-toggle buttons. |
| Forced | Amber banner above card. Submit button reads "Aktifkan Akun" with `ShieldCheck` icon. "Atau, keluar dari akun ini" link at the bottom. |
| Submitting | `isPending=true`. Submit button shows `Loader2 animate-spin` + "Menyimpan…". |
| Success | Card content replaced with emerald circle + success copy + "Masuk Kembali" button. After 2500ms, `router.replace("/login")`. |
| Wrong old password | `FormMessage` on `old_password`: "Kata sandi saat ini tidak tepat". |
| Rate limited | Form-level alert: "Terlalu banyak percobaan. Tunggu beberapa menit sebelum mencoba lagi." |
| Server error | Form-level alert: "Terjadi kesalahan. Periksa koneksi Anda dan coba lagi." |

---

## 4. Token References

No new tokens introduced. All values are drawn from existing system tokens.

| Element | Token / class | Source |
|---|---|---|
| Page background | `bg-slate-50` | Existing platform-admin page shells |
| Card surface | `bg-card` / `bg-white` | shadcn `--card` |
| Card shadow | `shadow-sm` | Existing card pattern |
| Page title | `text-2xl font-semibold tracking-tight text-foreground` | heading-md tier (BS-T3) |
| Page subtitle | `text-sm text-muted-foreground` | body-md + BS-T4 `on-surface-variant` |
| Section heading | `text-xs font-semibold uppercase tracking-widest text-muted-foreground` | Aksi Cepat heading pattern from dashboard spec |
| Avatar circle bg | `bg-primary/10` | PA-T1 icon container bg (indigo/10) |
| Avatar circle text | `text-primary text-xl font-semibold` | PA-T1 icon color |
| Avatar ring | `ring-2 ring-border` | shadcn `--border` |
| Tile icon bg (live) | `bg-primary/10` | PA-T1 |
| Tile icon color (live) | `text-primary` | PA-T1 |
| Tile icon bg (disabled) | `bg-muted` | shadcn `--muted` |
| Tile icon color (disabled) | `text-muted-foreground` | shadcn `--muted-foreground` |
| Tile hover shadow | `hover:shadow-md` + `transition-shadow` 150ms | BS-T6 `fast` |
| Tile focus ring | `focus-visible:ring-2 focus-visible:ring-ring` | `--ring` (indigo) |
| Live tile chevron | `text-muted-foreground/60` → `group-hover:text-foreground` | Aksi Cepat chevron pattern |
| "Segera hadir" pill | `bg-muted text-muted-foreground` | `--muted` / `--muted-foreground` |
| Email read-only field bg | `bg-muted` | `--muted` |
| Read-only field text | `text-muted-foreground` | `--muted-foreground` |
| Forced banner | `border-amber-300 bg-amber-50 text-amber-900` | Existing `must_change_password` banner tokens |
| Password card header icon bg | `bg-primary/10` | PA-T1 (adjusted from tenant-admin's `bg-pink-100`) |
| Success circle | `bg-emerald-100 text-emerald-700` | Existing success states |
| Destructive alert border | `border-destructive/40 bg-destructive/5 text-destructive` | shadcn `--destructive` |
| Form input | shadcn `Input` component | `components/ui/input.tsx` |
| Form label / message | shadcn `FormLabel` / `FormMessage` | `components/ui/form.tsx` |

---

## 5. Server Action Wiring

### 5.1 `/profil` — `updateProfileAction`

**File:** `app/profil/actions.ts` (new file, platform-admin)

**Endpoint:** `PATCH /auth/me`

**Request shape:**
```json
{
  "full_name": "Budi Santoso",
  "phone": "08123456789"
}
```
Fields are sent only when non-empty (omit `avatar_url` for v1 since the avatar uploader is deferred). Both fields use pointer-style binding on the backend (`omitempty`) so sending only one field is valid.

**Response shape (200):** `UserProfileResponse` — identical to `GET /auth/me`'s `user` object:
```json
{
  "id": "...",
  "email": "budi@lustia.id",
  "full_name": "Budi Santoso",
  "phone": "08123456789",
  "avatar_url": null,
  "is_active": true,
  "is_super_admin": true,
  "must_change_password": false
}
```

**Server Action signature:**
```ts
export type UpdateProfileState =
  | { status: "idle" }
  | { status: "error"; fieldErrors?: Record<string, string[]>; formError?: string }
  | { status: "success"; user: UserProfileResponse };

export async function updateProfileAction(
  _prev: UpdateProfileState,
  formData: FormData
): Promise<UpdateProfileState>
```

**Client-side RHF `reset` on success:** after `state.status === "success"`, call `form.reset({ full_name: state.user.full_name, phone: state.user.phone ?? "" })` to clear `isDirty` and show the saved values.

**Error mapping:**

| Backend code / status | Client message |
|---|---|
| 400 / VALIDATION | Field-level via `fieldErrors` |
| 401 | `redirect("/login")` |
| 429 / RATE_LIMITED | Form alert: "Terlalu banyak percobaan. Tunggu beberapa menit sebelum mencoba lagi." |
| 5xx | Form alert: "Terjadi kesalahan. Periksa koneksi Anda dan coba lagi." |

---

### 5.2 `/pengaturan/ubah-kata-sandi` — `changePasswordAction`

**File:** `app/pengaturan/ubah-kata-sandi/actions.ts` (new file, platform-admin)

**Endpoint:** `POST /auth/me/password`

**Request shape:**
```json
{
  "old_password": "current_password",
  "new_password": "new_secure_password"
}
```
(`confirm_password` is client-side only — never sent to the backend.)

**Response:** `204 No Content` on success.

**Server Action is identical in logic to tenant-admin's `changePasswordAction`.** Copy it verbatim and update only the import paths for `apiFetch`, `getRefreshToken`, `clearSessionCookies`.

**Error mapping:** identical to tenant-admin. See `lustia/web/tenant-admin/app/pengaturan/ubah-kata-sandi/actions.ts` → `mapChangePasswordError()`.

---

## 6. Backend Endpoint Verification

### 6.1 `PATCH /auth/me` — Profile update

**Finding:** Endpoint EXISTS.

Verified at:
- `lustia/services/auth/internal/controller/auth_controller.go:59` — `secured.PATCH("/me", h.handleUpdateMe)`
- `lustia/services/auth/internal/controller/dto_request.go:49–54` — `UpdateMeRequest { FullName *string, Phone *string, AvatarURL *string }`
- `lustia/services/auth/internal/controller/auth_controller.go:210` — responds `200 OK` with `toUserProfileResponse(prof)` — the same `UserProfileResponse` shape as `GET /auth/me`.

The endpoint accepts partial updates via pointer fields with `omitempty`. Sending only `full_name` without `phone` is valid. Sending only `phone` is valid. Sending neither is a no-op (200 OK with unchanged profile).

**Auth:** the endpoint is in the `secured` group (`jwtMW → tenantMW → scopeGateMW → pwdChangeMW`). Note: `pwdChangeMW` normally blocks callers with `must_change_password=true`. Since `PATCH /auth/me` is on the same `secured` group, a super-admin with `must_change_password=true` **cannot reach `/profil` to edit their profile** until they change their password first. This is intentional (the middleware enforces it) and correct behavior. The `/profil` form should not need special handling for this case.

**No go-expert action required for Scope 1.**

---

### 6.2 `POST /auth/me/password` — Change password

**Finding:** Endpoint EXISTS.

Verified at:
- `lustia/services/auth/internal/controller/auth_controller.go:60` — `secured.POST("/me/password", h.handleChangePassword)`
- `lustia/services/auth/internal/controller/dto_request.go:56–60` — `ChangePasswordRequest { OldPassword string, NewPassword string }`
- `lustia/services/auth/internal/controller/auth_controller.go:234` — responds `204 No Content` on success.

The endpoint is in the same `secured` group. The `pwdChangeMW` allowlist explicitly permits `/auth/me/password` (per the comment at line 39: "the allowlist paths (/auth/me, /auth/select-tenant, /auth/logout, /auth/me/password) are still reachable under scope=user"). A caller with `must_change_password=true` CAN reach this endpoint — the forced-change flow works correctly.

**This is the same endpoint used by both tenant-admin and platform-admin.** No separate admin-specific endpoint exists or is needed.

**No go-expert action required for Scope 1.**

---

## 7. Open Questions

### 7.1 Avatar upload (v1 deferred)

The backend already accepts `avatar_url` as a string URL (no binary upload endpoint). For v1, the avatar section shows initials + "segera hadir" hint and no uploader. When avatar upload is eventually built, it will require a separate file-upload endpoint (presigned S3 URL or similar) that does not currently exist. Flag for go-expert at that time.

### 7.2 pendingRegistrationCount on `/profil` and `/pengaturan`

Both pages call `ConsoleHeader` which requires `pendingRegistrationCount`. The dashboard page fetches this from `/admin/tenant-registrations?status=pending&limit=1` and reads `total_count`. The profil and pengaturan pages must either:
- (a) Perform the same best-effort fetch alongside the `/auth/me` call, or
- (b) Pass `pendingRegistrationCount={0}` as a simplification.

**Recommended (a)**: include the fetch in a `Promise.allSettled` alongside `/auth/me`. Pass `total_count ?? 0` to `ConsoleHeader` so the nav badge stays accurate. If the fetch fails, fall back to `0` (badge disappears — not a blocking UX failure).

**Flag for nextjs-expert:** implement the same dual-fetch pattern as `DashboardPage` for `/profil` and `/pengaturan` Server Components.

### 7.3 `must_change_password` banner link on `/dashboard`

The current banner in `DashboardPage` renders the "Ubah kata sandi" text as a `<span aria-disabled="true">` because the route did not exist. Once `/pengaturan/ubah-kata-sandi` ships, this must be updated to:

```tsx
<Link
  href="/pengaturan/ubah-kata-sandi?reason=required"
  className="font-medium underline underline-offset-4 text-yellow-800 hover:text-yellow-900"
>
  Ubah kata sandi
</Link>
```

**Flag for nextjs-expert:** update `DashboardPage` banner (around line 229 of `app/dashboard/page.tsx`) as part of the same ticket implementing these routes.

### 7.4 `/pengaturan` — no sub-layout

There is no `app/pengaturan/layout.tsx` file in the current platform-admin app structure (unlike the tenant-admin's `settings/` which has none either). The `/pengaturan` and `/pengaturan/ubah-kata-sandi` pages each include their own `ConsoleHeader` independently. This is consistent with the existing pattern for `profil/page.tsx` and `pengaturan/page.tsx` (which already do this). No layout file is needed.

---

## 8. Accessibility Checklist

| Criterion | Implementation |
|---|---|
| WCAG 1.4.1 Color not sole differentiator | Disabled tiles use both `opacity-50` and the "Segera hadir" text label. The amber forced-change banner uses both the `AlertTriangle` icon and text. |
| WCAG 1.4.3 Contrast ≥ 4.5:1 | `text-foreground` on `bg-card` (white): uses shadcn default black-on-white ratio. `text-muted-foreground` on `bg-card`: verify against theme (~4.6:1 for shadcn default, same note as dashboard spec). Read-only email: `text-muted-foreground` on `bg-muted` — verify; if it falls below 4.5:1, switch to `text-foreground` for the email value. |
| WCAG 2.1.1 Keyboard | Live tile: `<Link>` — native keyboard access. Disabled tiles: `<div>` — intentionally not in tab order. Form fields: native `<input>` elements via shadcn `Input`. All buttons: `<button>` elements. |
| WCAG 2.4.3 Focus order | Header → page title → avatar section → email → name input → phone input → Batal → Simpan. Logical DOM order. |
| WCAG 2.4.7 Focus visible | `focus-visible:ring-2 focus-visible:ring-ring` on live tile `<Link>` and form fields (shadcn default). EyeToggle buttons have `hover:bg-muted` — ensure they also get a focus ring via the shadcn `Button` ghost variant or explicit `focus-visible:ring`. |
| WCAG 4.1.2 Name/Role/Value | Required asterisk `aria-hidden="true"`. Avatar circle `aria-hidden="true"`. Icons in buttons `aria-hidden="true"`. EyeToggle has `aria-pressed` + `aria-label`. `FormLabel` is associated with `FormControl` via htmlFor/id (shadcn default). Error messages linked via `FormMessage` → `aria-describedby`. |
| Motion | Skeletons use `motion-safe:animate-pulse`. Tile hover uses `transition-shadow` 150ms (BS-T6 `fast`). `PasswordStrength` bar uses `transition-colors motion-reduce:transition-none` (already in the component). |
| Touch targets | Form inputs: full-width, `h-10` (40px height) — meets 24px floor comfortably. Buttons: `h-10` default. Tile link cards: full column width. EyeToggle: `h-7 w-7` (28px) — meets 24px floor; recommend `h-8 w-8` (32px) for additional margin, but 28px is acceptable. |
