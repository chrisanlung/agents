# Platform-Admin Dashboard — Design Spec

_Owned by `ui-ux-expert`. Implemented by `nextjs-expert`. Backend additions flagged for `go-expert`._
_Status: Ready for review. Approved spec unlocks nextjs-expert + go-expert implementation._

---

## 1. Overview

The platform-admin dashboard at `/dashboard` is the super-admin's operational triage surface. It replaces the current placeholder (profile card + "coming soon" stub) with a screen that answers four questions within three seconds of landing: How many tenant registrations need my attention right now? Is money moving correctly — how much is pending and how much settled this week? Are there failures I need to act on? And where do I go to act?

The layout follows the same visual language as the ops portal dashboard — white cards on a slate-50 background, the same `KpiCard` recipe, the same "Aksi Cepat" block — so a user who already knows the ops portal experiences this screen as a familiar sibling, not a new design system. The KPI grid drives urgency. The two panels below it give enough context to decide whether to escalate. Everything else (deep tables, filters, timelines) lives on the dedicated sub-screens that each KPI and panel row links to directly.

Data is fetched in a single `Promise.allSettled` on the Server Component. Each section degrades independently if its individual fetch fails — the KPI card for that data shows an inline error state; other sections render normally. The screen never shows a global 500 because one endpoint is slow.

---

## 2. Mockups

### 2A. Desktop (lg — 1280px, max-w-5xl container)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  [shield icon] Lustia Platform Console   [Dasbor] [Tenant] [Payout]     │
│                                          [Registrasi]       [● BS ˅]    │
└─────────────────────────────────────────────────────────────────────────┘
                                    ← header, bg-white, border-b, shadow-sm
                              [● BS ˅] = avatar circle + first name + chevron

[yellow banner — visible only when must_change_password=true]
┌────────────────────────────────────────────────────────────────────────┐
│ ⚠  Perubahan kata sandi diperlukan                                     │
│    Akun Anda memerlukan perubahan kata sandi sebelum mengakses fitur.  │
└────────────────────────────────────────────────────────────────────────┘

Selamat datang, Budi!   ·   Kamis, 1 Mei 2026
─────────────────────────────────────────────────────────────────────────

┌───────────────┐  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ [building icon]│  │ [clipboard  ] │  │ [banknote   ] │  │ [chart icon ] │
│  bg-primary/10│  │  bg-amber/10  │  │  bg-primary/10│  │  bg-emerald/10│
│               │  │               │  │               │  │               │
│      127      │  │    !! 4 !!    │  │      12       │  │  Rp 8.450.000 │
│  Tenant Aktif │  │  Pendaftaran  │  │  Disbursement │  │ Volume Disetel│
│               │  │  Pending      │  │  Pending      │  │ Minggu Ini    │
└───────────────┘  └───────────────┘  └───────────────┘  └───────────────┘
   link → /tenants    link →             link →             link →
           /registrasi  /disbursements    /reconciliation

─────────────────────────────────────────────────────────────────────────
AKSI CEPAT

┌──────────────────────────┐  ┌──────────────────────────┐  ┌────────────────────────────┐
│ [clipboard icon]         │  │ [refresh-cw icon]        │  │ [banknote icon]            │
│ Review Pendaftaran       │  │ Reconciliation           │  │ Buat Disbursement          │
│ Tinjau dan setujui atau  │  │ Settlement               │  │ Cairkan saldo tenant yang  │
│ tolak permintaan tenant. │  │ Tarik laporan settlement │  │ siap dibayar.              │
│                          │  │ harian dari iPaymu.      │  │                            │
│                          ›  │                          ›  │                            ›│
└──────────────────────────┘  └──────────────────────────┘  └────────────────────────────┘

─────────────────────────────────────────────────────────────────────────
┌───────────────────────────────────────┐  ┌──────────────────────────────────────┐
│ Pendaftaran Tenant Terbaru            │  │ Disbursement Terbaru                 │
│ ─────────────────────────────────────│  │ ───────────────────────────────────  │
│ Nama Tenant      Tanggal    Status   │  │ Tenant       Nominal    Status       │
│ PT Segar Bugar   2 jam lalu [Pending]│  │ PT Segar…   Rp 2.450K  [Pending]    │
│ Klinik Ananda    1 hr lalu  [Pending]│  │ Klinik An…  Rp 1.100K  [Processing] │
│ Wellness House   kemarin    [Pending]│  │ Wellness H…  Rp 900K   [Transferred] │
│                                      │  │ Spa Bintang  Rp 850K   [FAILED]      │  ← red pill, bold
│                                      │  │ Zen Retreat  Rp 560K   [Cancelled]   │
│ Tidak ada pendaftaran (empty state)  │  │                                      │
│                                      │  │ Belum ada disbursement (empty state) │
│ ─────────────────────────────────────│  │ ───────────────────────────────────  │
│ Lihat semua →                        │  │ Lihat semua →                        │
└───────────────────────────────────────┘  └──────────────────────────────────────┘

─────────────────────────────────────────────────────────────────────────
[compact profile row removed — identity and actions in header dropdown §9]
─────────────────────────────────────────────────────────────────────────
```

### 2B. Mobile (375px — single column)

```
┌─────────────────────────────────────────┐
│ [shield] Lustia Platform Console        │
│ [Dasbor] [Tenant] [Payout] [Registrasi] │  ← scrollable nav row
│                              [● BS ˅]   │  ← avatar-only trigger (no first name)
└─────────────────────────────────────────┘

[yellow banner — conditional]

Selamat datang, Budi!
Kamis, 1 Mei 2026

┌───────────────────────────────────────┐
│ [icon]  127       Tenant Aktif        │
└───────────────────────────────────────┘
┌───────────────────────────────────────┐
│ [icon]  !! 4 !!   Pendaftaran Pending │   ← amber accent
└───────────────────────────────────────┘
┌───────────────────────────────────────┐
│ [icon]  12        Disbursement Pending│
└───────────────────────────────────────┘
┌───────────────────────────────────────┐
│ [icon]  Rp 8.450.000  Volume Disetel  │
│                       Minggu Ini      │
└───────────────────────────────────────┘

┌───────────────────────────────────────┐
│ [clipboard] Review Pendaftaran        │
│ Tinjau dan setujui atau tolak...    › │
└───────────────────────────────────────┘
┌───────────────────────────────────────┐
│ [refresh-cw] Reconciliation Settlement│
│ Tarik laporan settlement harian...  › │
└───────────────────────────────────────┘
┌───────────────────────────────────────┐
│ [banknote] Buat Disbursement          │
│ Cairkan saldo tenant yang siap...   › │
└───────────────────────────────────────┘

┌───────────────────────────────────────┐
│ Pendaftaran Tenant Terbaru            │
│ ─────────────────────────────────────│
│ PT Segar Bugar    2 jam lalu [Pending]│
│ Klinik Ananda     1 hr lalu  [Pending]│
│ Wellness House    kemarin    [Pending]│
│ ─────────────────────────────────────│
│ Lihat semua →                         │
└───────────────────────────────────────┘

┌───────────────────────────────────────┐
│ Disbursement Terbaru                  │
│ ─────────────────────────────────────│
│ PT Segar…   Rp 2.450K   [Pending]    │
│ Klinik An…  Rp 1.100K   [Processing] │
│ Wellness H… Rp 900K     [Transferred]│
│ Spa Bintang Rp 850K     [FAILED]     │
│ ─────────────────────────────────────│
│ Lihat semua →                         │
└───────────────────────────────────────┘

[compact profile row removed — identity and actions in header dropdown §9]
```

---

## 3. Section-by-Section Spec

### 3.1 Header (existing — do not redesign)

No changes. The `<header>` in the current `DashboardPage` renders `NavLinks` + `SignOutButton`. The `pendingCount` badge on the Registrasi nav link remains alive — it is fed from the same pending-registration fetch that powers KPI card 2. Pass the `total_count` from that fetch rather than the current `data.length` hack (see Open Questions §5.1).

---

### 3.2 must_change_password Banner (existing — preserve as-is)

Token references: `border-yellow-300 bg-yellow-50 text-yellow-800`. Identical copy and markup to the current implementation. No changes beyond keeping the component mounted.

---

### 3.3 Greeting Strip

**Purpose:** orient the user (name + date). Low visual weight — this is a courtesy line, not a heading.

**Layout:**
```
<section aria-label="Sapaan">
  <h1 class="text-2xl font-semibold text-foreground">Selamat datang, [first_name]!</h1>
  <p  class="mt-1 text-sm text-muted-foreground">[tanggal panjang Indonesia]</p>
</section>
```

- Container: `py-0` (no extra vertical padding beyond the `space-y-6` from `<main>`).
- First name only: `user.full_name.split(" ")[0]` — matches the ops portal pattern.
- Date format: `new Date().toLocaleDateString("id-ID", { weekday: "long", day: "numeric", month: "long", year: "numeric" })` → "Kamis, 1 Mei 2026".

**Token references:**
- `text-foreground` (BS-T4 `on-surface`) for the greeting text.
- `text-muted-foreground` (BS-T4 `on-surface-variant`) for the date subtitle.
- `text-2xl font-semibold` = heading-md tier.
- `text-sm` = body-md tier.

**States:**
- Active: always shown. No loading state needed (derived from `/auth/me` which is required to reach the page).
- No empty/error state — the name is always available from the auth guard.

**Microcopy:** "Selamat datang, [Nama]!" — exclamation mark included, warm but brief.

**Click behavior:** none — decorative strip.

---

### 3.4 KPI Grid (4 cards)

**Layout:** `<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">`

- Mobile (< 640px): 1 column, full-width cards stacked vertically.
- Tablet (sm, 640px+): 2 × 2 grid.
- Desktop (lg, 1024px+): 4 columns side by side.
- Gap: `gap-4` (16px, from spacing scale).

**Card anatomy (PA-T1 — see §6 new token)**:
```
<Link href="[target]" class="block rounded-lg border bg-card shadow-sm
     transition-shadow hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring">
  <div class="flex items-center gap-4 p-5">
    <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full [iconBg]">
      [LucideIcon size=20 aria-hidden="true"]
    </div>
    <div class="min-w-0">
      <p class="text-xs font-medium uppercase tracking-wide text-muted-foreground">[label]</p>
      <p class="mt-0.5 text-3xl font-bold tabular-nums text-foreground">[value]</p>
      <p class="mt-0.5 text-xs text-muted-foreground">[sub-label]</p>
    </div>
  </div>
</Link>
```

- The entire card is a `<Link>` — single tappable region. No nested interactive elements inside.
- `tabular-nums` via Tailwind `font-variant-numeric: tabular-nums` on the number paragraph.
- `focus-visible:ring-2 focus-visible:ring-ring` for keyboard focus — WCAG 2.4.7.
- Hover: `hover:shadow-md` — 150ms `transition-shadow` (BS-T6 `fast`).

**The 4 KPI cards:**

| # | Label | Value source | Sub-label | Icon | Icon bg | Link |
|---|---|---|---|---|---|---|
| 1 | Tenant Aktif | `GET /admin/tenants?status=active&limit=1` → `total_count` | "tenant terdaftar aktif" | `Building2` | `bg-primary/10 text-primary` | `/tenants?status=active` |
| 2 | Pendaftaran Pending | `GET /admin/tenant-registrations?status=pending&limit=1` → `total_count` | "menunggu persetujuan" | `ClipboardList` | `bg-amber-100 text-amber-600` (or `bg-primary/10` when 0) | `/tenants/registrations?status=pending` |
| 3 | Disbursement Pending | `GET /admin/disbursements?status=pending&limit=1` → `total` | "pencairan menunggu" | `Banknote` | `bg-primary/10 text-primary` | `/payout/disbursements?status=pending` |
| 4 | Volume Disetel Minggu Ini | see Open Questions §5.2 | "7 hari terakhir" | `TrendingUp` | `bg-emerald-100 text-emerald-600` | `/payout/reconciliation` |

**KPI card 2 — urgency accent rule:**
- When `total_count > 0`: icon bg switches to `bg-amber-100`, icon color to `text-amber-600`, the numeric value is rendered in `text-amber-700 font-bold`. The amber tint draws the eye without aggressive red — this is a queue, not a failure.
- When `total_count === 0`: icon bg `bg-primary/10`, value rendered in standard `text-foreground`. Neutral — "inbox zero" state.

**KPI card 4 — Rupiah display:**
- Format: `Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 })` → "Rp 8.450.000".
- When value is 0: show "Rp 0" in `text-muted-foreground` (not bold, same size). Sub-label: "tidak ada settlement minggu ini".
- The `text-3xl font-bold` class still applies but the Rupiah string may overflow at small container widths. Guard with `overflow-hidden text-ellipsis whitespace-nowrap` or reduce to `text-2xl` when the formatted string exceeds 12 characters (implementation decision for nextjs-expert; flag in handoff notes).

**States — per card:**

| State | Behaviour |
|---|---|
| Loading | Replace card content with a `<Skeleton>` block: one `h-4 w-20 rounded` sub for label, one `h-8 w-12 rounded` for the number. Animate with `animate-pulse`. |
| Success | Card renders as spec above. |
| Error (fetch failed) | Card renders with muted icon bg, value displays `—`, sub-label becomes `"Gagal memuat"` in `text-destructive`, no link wrapping (or link still present but value is `—`). Do not throw; show the partial state. |

---

### 3.5 Aksi Cepat Row

**Purpose:** one-tap access to the three most common actions from this screen.

**Layout:** `<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">`

- Mobile: 3 stacked cards.
- sm and above: 3-column row.
- Section heading: `<h2 class="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3">Aksi Cepat</h2>`

**Card anatomy:**
```
<Link href="[target]" class="group flex items-center gap-4 rounded-lg border bg-card p-5
     transition-shadow hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring">
  <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/10">
    [LucideIcon size=20 class="text-primary" aria-hidden="true"]
  </div>
  <div class="min-w-0 flex-1">
    <p class="font-semibold text-foreground text-sm">[title]</p>
    <p class="mt-0.5 text-xs text-muted-foreground line-clamp-2">[description]</p>
  </div>
  <ChevronRight size=16 class="shrink-0 text-muted-foreground/60 group-hover:text-foreground
       transition-colors duration-fast" aria-hidden="true" />
</Link>
```

- Full card is the `<Link>` — no nested button or link.
- `line-clamp-2` on description to prevent layout shift on long translations.
- `ChevronRight` icon signals navigation affordance.

**The 3 action cards:**

| Title | Description | Icon | Link |
|---|---|---|---|
| Review Pendaftaran | Tinjau dan setujui atau tolak permohonan registrasi tenant baru. | `ClipboardList` | `/tenants/registrations?status=pending` |
| Reconciliation Settlement | Tarik laporan settlement harian dari iPaymu dan cocokkan transaksi. | `RefreshCw` | `/payout/reconciliation` |
| Buat Disbursement | Cairkan saldo tenant yang siap dibayar ke rekening mitra. | `Banknote` | `/payout/tenant-payout` |

Note on "Buat Disbursement" target: disbursement creation is initiated from the Pencairan Tenant page (`/payout/tenant-payout`), where tenant cards show a "Cairkan" button that opens `CreateDisbursementDialog`. There is no standalone `/payout/disbursements/new` route. The link target is `/payout/tenant-payout`.

**States:**
- No loading, no error, no empty — these are static navigation items. Always rendered.
- Mobile: each card is full-width, horizontal layout with icon on left, chevron on right.

---

### 3.6 Two Side-by-Side Panels

**Layout:** `<div class="grid grid-cols-1 gap-6 md:grid-cols-2">`

- Mobile: stacked vertically, left panel first.
- md+: two equal columns side by side.
- Gap: `gap-6` (24px).

---

#### 3.6.1 Left Panel — "Pendaftaran Tenant Terbaru"

**Data source:** `GET /admin/tenant-registrations?status=pending&limit=5`

**Card structure:**
```
<Card>
  <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-3">
    <CardTitle class="text-base">Pendaftaran Tenant Terbaru</CardTitle>
    <Link href="/tenants/registrations" class="text-xs text-muted-foreground
         hover:text-foreground flex items-center gap-1">
      Lihat semua <ArrowRight size=12 />
    </Link>
  </CardHeader>
  <CardContent class="p-0">
    <table class="w-full text-sm">
      <thead class="border-b bg-muted/30">
        <tr>
          <th scope="col" class="px-4 py-2 text-left text-xs font-medium
               text-muted-foreground">Nama Tenant</th>
          <th scope="col" class="px-4 py-2 text-left text-xs font-medium
               text-muted-foreground">Tanggal Daftar</th>
          <th scope="col" class="px-4 py-2 text-left text-xs font-medium
               text-muted-foreground">Status</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-border">
        [rows — each is a <tr> wrapping a <Link> via row-link technique]
      </tbody>
    </table>
  </CardContent>
</Card>
```

**Row:**
```
<tr class="hover:bg-muted/40 cursor-pointer" onClick → /tenants/registrations/[id]>
  <td class="px-4 py-3 font-medium truncate max-w-[140px]">[company_name]</td>
  <td class="px-4 py-3 text-muted-foreground tabular-nums whitespace-nowrap">
    [relativeTime(created_at)]
  </td>
  <td class="px-4 py-3">
    <Badge class="bg-amber-100 text-amber-800 border-amber-200">Pending</Badge>
  </td>
</tr>
```

- Row is `<tr>` with an `onClick` → `router.push` (Client Component wrapper, or use `data-href` progressive enhancement). Since this is a Server Component page, implement rows as `<tr>` containing `<td>` with a visually-hidden `<Link>` spanning the row via absolute positioning, or use the `next-link-tr` trick (see implementation note below).
- Implementation note for nextjs-expert: wrap the `<tbody>` in a `"use client"` sub-component `<RegistrationTableBody>` that uses `useRouter` for row clicks. This is the same pattern as `DisbursementsPage` (existing `onClick` on `<TableRow>`).
- `truncate max-w-[140px]` on company name — handles long names in a fixed-width panel.
- Relative time (e.g., "2 jam lalu") uses the existing `relativeTime` helper already present in this codebase.

**Empty state:**
```
<div class="flex flex-col items-center gap-3 py-10 text-center">
  <Inbox size=32 class="text-muted-foreground/30" />
  <p class="text-sm text-muted-foreground">
    Tidak ada pendaftaran menunggu review.
  </p>
</div>
```

**Error state:**
```
<div role="alert" class="flex items-start gap-2 m-4 rounded-md border border-red-300
     bg-red-50 px-4 py-3 text-sm text-red-800">
  <AlertCircle size=16 class="mt-0.5 shrink-0 text-red-600" />
  <span>Gagal memuat pendaftaran. <a href="/dashboard" class="underline">Muat ulang</a>.</span>
</div>
```

**Loading state:** three skeleton rows inside the `<tbody>`, each with a `h-4 rounded animate-pulse bg-muted` block per cell.

**Footer link:** rendered inside `CardHeader` as the "Lihat semua" link, not inside `CardContent` (keeps it aligned with the card title at all times, including empty/error states).

**Token references:**
- Panel title: `text-base font-semibold` = heading-md tier.
- Row font: `text-sm` = body-md.
- Muted cells: `text-muted-foreground` (BS-T4 `on-surface-variant`).
- Status pill: `bg-amber-100 text-amber-800 border-amber-200` — same amber as KPI card 2.
- Panel background: `bg-card` (white in light mode) from card shell.

---

#### 3.6.2 Right Panel — "Disbursement Terbaru"

**Data source:** `GET /admin/disbursements?limit=5` (no status filter — show latest 5 regardless of status, sorted `created_at DESC` by backend default)

**Card structure:** identical shell to left panel.

**Columns:** Tenant · Nominal · Status

**Row:**
```
<tr class="hover:bg-muted/40 cursor-pointer" onClick → /payout/disbursements/[id]>
  <td class="px-4 py-3 font-medium truncate max-w-[120px]">[tenant_name]</td>
  <td class="px-4 py-3 tabular-nums text-sm font-medium">[formatRupiah(net_amount_idr)]</td>
  <td class="px-4 py-3">
    <DisbursementStatusBadge status=[d.status] />
  </td>
</tr>
```

**`DisbursementStatusBadge` — color contract (PA-T2, see §6):**

| Status | Label (Indonesian) | bg | text | border | Weight |
|---|---|---|---|---|---|
| `pending` | Menunggu | `bg-amber-100` | `text-amber-800` | `border-amber-200` | normal |
| `processing` | Diproses | `bg-blue-100` | `text-blue-800` | `border-blue-200` | normal |
| `transferred` | Ditransfer | `bg-emerald-100` | `text-emerald-800` | `border-emerald-200` | normal |
| `failed` | GAGAL | `bg-red-100` | `text-red-800` | `border-red-300` | `font-bold` |
| `cancelled` | Dibatalkan | `bg-muted` | `text-muted-foreground` | `border-border` | normal |

Critical: `failed` status MUST have bold label text AND the text label "GAGAL" (uppercase) in addition to color, satisfying WCAG 1.4.1 (color not the sole differentiator). The existing `DisbursementStatusBadge` component already exists at `components/disbursement-status-badge.tsx` — verify it meets this rule; if the "GAGAL" label is not bold/uppercase, request a correction from nextjs-expert as part of this ticket.

**Empty state:**
```
<div class="flex flex-col items-center gap-3 py-10 text-center">
  <Banknote size=32 class="text-muted-foreground/30" />
  <p class="text-sm text-muted-foreground">Belum ada disbursement.</p>
</div>
```

**Error state:** same pattern as left panel with copy "Gagal memuat disbursement."

**Loading state:** three skeleton rows.

**Footer link:** "Lihat semua →" → `/payout/disbursements` — same placement as left panel.

---

### 3.7 Compact Profile Row

> **REMOVED — moved to header dropdown (§9).**
>
> The `ProfileRow` Server Component (`app/dashboard/_components/profile-row.tsx`) and the `<ProfileRow />` usage at the bottom of `DashboardPage` must be deleted by nextjs-expert as part of implementing §9.
> The compact profile row no longer renders on the dashboard page or any other page. User identity and account actions are exclusively in the header avatar dropdown described in §9.
>
> The original spec content below is kept for historical reference only and must not be re-implemented.

---

## 4. Color Usage

All values map to Tailwind utility classes backed by the existing shadcn/ui CSS variable tokens. No new hex values are introduced.

| Design role | Tailwind class(es) | CSS variable | Usage on this screen |
|---|---|---|---|
| Page background | `bg-slate-50` | — | `<div class="min-h-screen bg-slate-50">` (existing pattern) |
| Card surface | `bg-card` | `--card` | All Card components; dropdown panel via `bg-popover` |
| Primary text | `text-foreground` | `--foreground` | Names, numbers, card titles |
| Secondary text | `text-muted-foreground` | `--muted-foreground` | Sub-labels, dates, footer text |
| Primary accent | `text-primary` / `bg-primary/10` | `--primary` | Icon fills on KPI 1, 3; avatar initials; Aksi Cepat icons |
| Warning amber | `text-amber-600` / `bg-amber-100` | — (Tailwind palette) | KPI card 2 when pending > 0; Registration status pill |
| Success emerald | `text-emerald-600` / `bg-emerald-100` | — | KPI card 4 icon; "Transferred" disbursement pill |
| Danger red | `text-red-800` / `bg-red-100` / `border-red-300` | — | "Failed" disbursement pill; error alert banners |
| Info blue | `text-blue-800` / `bg-blue-100` | — | "Processing" disbursement pill |
| Cancelled muted | `text-muted-foreground` / `bg-muted` | `--muted` | "Cancelled" disbursement pill |
| Destructive | `text-destructive` | `--destructive` | Error state inline copy |
| Focus ring | `focus-visible:ring-2 focus-visible:ring-ring` | `--ring` | All interactive cards (KPI, Aksi Cepat) |
| Card border | `border` | `--border` | Default card border |
| Table header bg | `bg-muted/30` | `--muted` at 30% opacity | Panel table `<thead>` |
| Row hover | `hover:bg-muted/40` | `--muted` at 40% opacity | Panel table row hover |

---

## 5. Open Questions

### 5.1 Pending Registration Count — total_count vs data.length

**Current state:** the existing `DashboardPage` fetches `limit=1` then uses `data.length` (0 or 1) as the badge count, which is always wrong for counts > 1.

**Required fix (not a new endpoint):** pass `limit=1` and read `total_count` from the response (already in `RegistrationListResponse.total_count`). The current code already receives this field but discards it. This is a bug fix in the current implementation, not a new endpoint. Flagged for nextjs-expert: change line 93 of `platform-admin/app/dashboard/page.tsx` to `pendingRegistrationCount = regList.total_count`.

---

### 5.2 Volume Disetel Minggu Ini — missing endpoint

**What the KPI needs:** total Rupiah volume of settlement batches whose `settled_at` falls within the past 7 calendar days, summed across all providers.

**Why the existing `/admin/settlement-batches` list won't work efficiently:** the list returns paginated `SettlementBatch` rows. To compute the weekly total, the dashboard would need to fetch all pages for the week and sum client-side — unpredictable latency, wastes bandwidth, breaks on large history.

**Proposed endpoint for go-expert:**

```
GET /admin/settlement-batches/summary?from=YYYY-MM-DD&to=YYYY-MM-DD

Response 200:
{
  "from": "2026-04-25",
  "to": "2026-05-01",
  "total_amount_idr": 8450000,
  "transaction_count": 47,
  "batch_count": 7
}

Auth: Bearer (super_admin only — same as other /admin/* endpoints)
Query params:
  from  — ISO date, required. Inclusive lower bound on settled_at.
  to    — ISO date, required. Inclusive upper bound on settled_at.
```

**Dashboard consumption:** the Server Component will compute `from` and `to` as the past 7 days (`new Date()` - 6 days to today), call this endpoint, and render `total_amount_idr` as the KPI value. If the endpoint is not yet available, the KPI card renders `—` with sub-label "data belum tersedia" (non-blocking for the rest of the dashboard).

**GORM query hint:** `SELECT COALESCE(SUM(total_amount_idr),0) AS total_amount_idr, COUNT(*) AS batch_count, COALESCE(SUM(transaction_count),0) AS transaction_count FROM settlement_batches WHERE settled_at >= ? AND settled_at <= ?`

---

### 5.3 Disbursement "latest 5" — sort order confirmation

**Assumption:** `GET /admin/disbursements?limit=5` (no status filter) returns rows sorted by `created_at DESC` by default (most recent first). If the backend does not guarantee this sort order without an explicit `sort=created_at_desc` parameter, go-expert should either (a) document the default sort in `API_CONTRACT.md`, or (b) add the sort parameter to the endpoint spec so nextjs-expert can pass it explicitly.

---

### 5.4 Active Tenant Count — total_count with limit=1

**Assumption:** `GET /admin/tenants?status=active&limit=1` returns `total_count` in the response envelope (consistent with `RegistrationListResponse` and `AdminDisbursementListResponse`). Verify the `/admin/tenants` list response shape includes `total_count`. If it uses a different key (e.g., `total`), nextjs-expert should normalise it at the callsite. go-expert to confirm or correct in `API_CONTRACT.md`.

---

### 5.5 /profil and /pengaturan routes for platform-admin

> **Updated — scope expanded by §9 (header user menu dropdown).**
> The compact profile row that previously referenced `/pengaturan` has been removed (see §3.7). Both `/profil` and `/pengaturan` are now reachable via the header avatar dropdown on every authenticated page.

**Required by:** the header avatar dropdown menu items "Profil Saya" (`/profil`) and "Pengaturan" (`/pengaturan`), and (when live) the `must_change_password` banner link.

**Current state:** neither `/profil` nor `/pengaturan` routes exist in platform-admin. The ops portal (tenant-admin) has `/pengaturan/ubah-kata-sandi`. Flagged for nextjs-expert + go-expert to scope in a separate ticket.

**Placeholder treatment (until routes exist):** render both menu items as `<Link>` pointing to the target route, with `aria-disabled="true"` and `title="Segera hadir"` on the `<DropdownMenuItem>`. The items are visible and keyboard-focusable but navigation is blocked at the route level (a 404 or redirect to `/dashboard` is acceptable). Do not hide the items — hiding them creates an expectation gap the first time the route ships.

---

## 6. Asset / Icon List

All icons from `lucide-react`. Versions already vendored in the project.

| Icon name | Used in |
|---|---|
| `ShieldCheck` | Header brand icon |
| `AlertTriangle` | must_change_password banner |
| `Building2` | KPI card 1 — Tenant Aktif |
| `ClipboardList` | KPI card 2 — Pendaftaran Pending; Aksi Cepat card 1 |
| `Banknote` | KPI card 3 — Disbursement Pending; Aksi Cepat card 3; panel empty state |
| `TrendingUp` | KPI card 4 — Volume Disetel |
| `RefreshCw` | Aksi Cepat card 2 — Reconciliation |
| `ChevronRight` | Aksi Cepat card chevron |
| `ArrowRight` | Panel footer "Lihat semua" link |
| `Inbox` | Left panel empty state |
| `AlertCircle` | Error alert banners inside panels |
| `ChevronDown` | Header avatar dropdown trigger — chevron signal |
| `User` | Header dropdown menu item — Profil Saya |
| `Settings` | Header dropdown menu item — Pengaturan |
| `LogOut` | Header dropdown menu item — Keluar |
| `Loader2` | Header dropdown — sign-out in-progress spinner |

Icon sizing:
- KPI card icons: `size={20}` (inside h-11 w-11 circle).
- Aksi Cepat icons: `size={20}` (inside h-10 w-10 circle).
- Panel empty state icons: `size={32}`.
- Panel "Lihat semua" ArrowRight: `size={12}`.
- Aksi Cepat ChevronRight: `size={16}`.
- Error alert AlertCircle: `size={16}`.

---

## 7. Data Fetch Strategy (Server Component)

All data fetches run in a single `Promise.allSettled` call. No fetch blocks any other. Each result is destructured individually.

```
Conceptual fetch map:

const [meRes, tenantCountRes, pendingRegRes, pendingDisbRes, volumeRes, recentRegRes, recentDisbRes] =
  await Promise.allSettled([
    apiFetch("/auth/me"),                                              // required — redirect 401
    apiFetch("/admin/tenants?status=active&limit=1"),                  // KPI 1
    apiFetch("/admin/tenant-registrations?status=pending&limit=5"),   // KPI 2 + left panel
    apiFetch("/admin/disbursements?status=pending&limit=1"),           // KPI 3
    apiFetch("/admin/settlement-batches/summary?from=...&to=..."),    // KPI 4 (new endpoint)
    // recentRegRes is same data as pendingRegRes — reuse, no second fetch
    apiFetch("/admin/disbursements?limit=5"),                          // right panel
  ]);
```

Note: `pendingRegRes` and `recentRegRes` use the SAME call — `GET /admin/tenant-registrations?status=pending&limit=5` serves both the KPI count (`total_count`) and the left panel (up to 5 rows). Only 5 calls total.

The `/auth/me` result is handled first:
- If it settled with `401` → `redirect("/login")`.
- If it settled with any other error → `throw` (unexpected; global error boundary handles it).
- If fulfilled → extract `user`, `mustChange`, `pendingCount`.

For every other result:
- Fulfilled → render normally.
- Rejected → render the section's inline error state.

---

## 8. Accessibility Checklist

| Criterion | Implementation |
|---|---|
| WCAG 1.4.1 Color not sole differentiator | Status pills all include text labels ("Pending", "GAGAL", etc.). "Failed" is also bold + uppercase. |
| WCAG 1.4.3 Contrast ≥ 4.5:1 | Amber-800 on amber-100 = 7.2:1 ✓. Red-800 on red-100 = 6.1:1 ✓. Emerald-800 on emerald-100 = 5.9:1 ✓. Blue-800 on blue-100 = 6.4:1 ✓. Muted-foreground on card = ~4.6:1 (verify against specific theme). |
| WCAG 2.1.1 Keyboard | All interactive cards are `<Link>` or `<a>` — natively keyboard-focusable. No `div onClick` without role. |
| WCAG 2.4.3 Focus order | Logical DOM order: header (brand → nav links → avatar trigger) → banner → greeting → KPI grid (L→R, T→B) → Aksi Cepat (L→R) → left panel rows → right panel rows. Profile row removed. |
| WCAG 2.4.7 Focus visible | `focus-visible:ring-2 focus-visible:ring-ring` on all Link-wrapped card. |
| WCAG 4.1.2 Name / Role / Value | Table `<th scope="col">` on all column headers. Nav badge `aria-label="{n} registrasi menunggu"`. Icons `aria-hidden="true"` throughout. |
| Numbers | `tabular-nums` (Tailwind `font-variant-numeric`) on all numeric KPI values, Rupiah amounts, and relative times. |
| Motion | No animation on this screen beyond `transition-shadow` and `transition-colors` — both are fast (150ms) and non-vestibular. No scrolling animations, no parallax. Skeletons use `animate-pulse` — disable under `prefers-reduced-motion: reduce` by wrapping in `motion-safe:animate-pulse`. |
| Touch targets | KPI cards and Aksi Cepat cards span full column width (min 100% of 1-col mobile ≈ 375px wide). Avatar trigger is `h-9 w-9` (36px) — meets the 24px floor and is close enough to 44px that the surrounding header provides adequate touch margin. The full trigger button including the first-name label at md+ is wider and therefore easier to hit. |
| WCAG 4.1.2 Dropdown | `DropdownMenuTrigger` from Radix UI automatically applies `aria-haspopup="menu"` and `aria-expanded`. `DropdownMenuContent` receives `role="menu"`. Each `DropdownMenuItem` receives `role="menuitem"`. Focus is trapped inside the open menu; Escape closes and returns focus to the trigger. This is handled by the Radix primitive and requires no custom implementation. |

---

## 9. Header — User Menu Dropdown

### 9.1 Why

The compact "Profil Anda" footer card (§3.7, now removed) was visually heavy for what it communicated — two lines of identity text and a single link to a settings page that does not yet exist. It also duplicated the sign-out affordance pattern from a header that already had a `SignOutButton`, creating two distinct exit paths with different visual treatments. Consolidating user identity, Profil, Pengaturan, and Keluar into a single header avatar dropdown eliminates that duplication, reduces page chrome below the content panels, and matches the interaction pattern already established in the tenant-admin portal. A super-admin who also uses the tenant portal encounters the same trigger shape and the same menu vocabulary, with only the items relevant to their scope — no workspace switcher, no tenant-context items.

---

### 9.2 Trigger Anatomy

The trigger is a `<Button variant="ghost">` managed by `DropdownMenuTrigger`. It appears at the right end of the header, after `NavLinks`.

**Desktop (md and above — ≥ 768px):**

```
┌──────────────────────────────────────────────────────────────────────────┐
│ [shield] Lustia Platform Console  [Dasbor][Tenant][Payout][Registrasi]   │
│                                          ┌──────────────────────────┐   │
│                                          │ ● BS  Budi  ˅            │   │
│                                          └──────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘

  ●  = avatar circle, h-9 w-9 (36px), rounded-full
       bg-primary/10 text-primary ring-1 ring-border
       contains initials (e.g. "BS" for Budi Santoso)
       font: text-xs font-semibold

  "Budi" = user's first name (full_name.split(" ")[0])
            text-sm font-medium text-foreground
            hidden below md breakpoint

  ˅  = ChevronDown size={14}, text-muted-foreground, aria-hidden="true"
        always visible (desktop and mobile)
```

**Mobile (below md — < 768px):**

```
┌───────────────────────────────────────────────┐
│ [shield] Lustia Platform Console              │
│ [Dasbor][Tenant][Payout][Registrasi]  ● BS ˅  │
└───────────────────────────────────────────────┘

  First name label hidden. Avatar circle + chevron only.
  Nav row scrolls horizontally if needed (existing behavior).
```

**Trigger button class breakdown:**

```
<Button
  variant="ghost"
  className="flex items-center gap-2 rounded-full px-2 py-1
             focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
  aria-label="Menu akun [full_name]"    ← screen-reader label includes full name
>
  <!-- Avatar circle -->
  <span class="flex h-9 w-9 shrink-0 items-center justify-center
               rounded-full bg-primary/10 text-primary
               text-xs font-semibold ring-1 ring-border"
        aria-hidden="true">
    [initials]
  </span>

  <!-- First name — desktop only -->
  <span class="hidden md:block text-sm font-medium text-foreground">
    [firstName]
  </span>

  <!-- Chevron -->
  <ChevronDown size={14} class="text-muted-foreground" aria-hidden="true" />
</Button>
```

Note: the `aria-label` on the trigger includes the user's full name. This means a screen reader announces "Menu akun Budi Santoso, collapsed, button" — enough context without the first-name label being read twice.

---

### 9.3 Open-State Anatomy (Dropdown Panel)

The dropdown opens below the trigger, aligned to the right edge (`align="end"`). Width: `w-56` (224px). This is narrower than the tenant-admin's `w-60` because the platform-admin menu has one fewer item (no "Ubah Kata Sandi" since that lives under `/pengaturan`). The name+email header still fits at `w-56`.

```
┌──────────────────────────────────┐
│ Budi Santoso                     │  ← text-sm font-semibold text-foreground, truncate
│ budi@lustia.id                   │  ← text-xs text-muted-foreground, truncate
├──────────────────────────────────┤  ← DropdownMenuSeparator
│ [User icon]  Profil Saya         │  ← DropdownMenuItem → /profil
│ [Settings]   Pengaturan          │  ← DropdownMenuItem → /pengaturan
├──────────────────────────────────┤  ← DropdownMenuSeparator
│ [LogOut]     Keluar              │  ← DropdownMenuItem, text-destructive
└──────────────────────────────────┘

Panel tokens:
  - bg: bg-popover (CSS var --popover, white in light mode)
  - border: border (CSS var --border)
  - shadow: shadow-md (Radix default via DropdownMenuContent)
  - radius: rounded-md (Radix default, 6px)
  - animation: Radix fade-in/zoom-in on open (150ms), fade-out/zoom-out on close (100ms)
               no-op under prefers-reduced-motion (Radix respects this automatically)
```

**Header section (identity block):**
```
<div class="px-2 py-2">
  <p class="truncate text-sm font-semibold text-foreground">[full_name]</p>
  <p class="truncate text-xs text-muted-foreground">[email]</p>
</div>
```
This block is not a `DropdownMenuItem` — it is not interactive, not keyboard-selectable, and not announced as a menu item. It is a plain `<div>` inside `DropdownMenuContent`, identical to the tenant-admin pattern.

**Menu items:**

| Order | Label | Icon | Size | Href | Notes |
|---|---|---|---|---|---|
| 1 | Profil Saya | `User` | 14 | `/profil` | `<DropdownMenuItem asChild><Link>` |
| 2 | Pengaturan | `Settings` | 14 | `/pengaturan` | `<DropdownMenuItem asChild><Link>` |
| — | separator | — | — | — | `<DropdownMenuSeparator />` |
| 3 | Keluar | `LogOut` or `Loader2` | 14 | n/a | `onSelect` → server action; `text-destructive focus:text-destructive` |

Icon size `14` (14px) is the established standard from tenant-admin `user-menu.tsx` and must not change. Icons are `aria-hidden="true"` and spaced `mr-2` from label text.

---

### 9.4 Token References

No new tokens are introduced. All values are reused from existing sources.

| Element | Token / class | Source |
|---|---|---|
| Trigger avatar circle size | `h-9 w-9` (36px) | Tenant-admin `user-menu.tsx` — identical |
| Trigger avatar bg | `bg-primary/10` | PA-T1 icon container bg (platform-admin primary/10 is indigo/10) |
| Trigger avatar text | `text-primary text-xs font-semibold` | PA-T1 icon color |
| Trigger avatar ring | `ring-1 ring-border` | shadcn/ui `--border` token; neutral against the white header bg |
| Trigger first-name | `text-sm font-medium text-foreground` | BS-T4 / shadcn `--foreground` |
| Trigger chevron | `text-muted-foreground` size 14 | shadcn `--muted-foreground` |
| Focus ring on trigger | `focus-visible:ring-2 focus-visible:ring-ring` | `--ring` (indigo for platform-admin) |
| Dropdown panel width | `w-56` | shadcn convention; narrower than tenant-admin `w-60` (one fewer item) |
| Dropdown panel bg | `bg-popover` | shadcn `--popover` |
| Dropdown panel shadow | `shadow-md` | shadcn `DropdownMenuContent` default |
| Dropdown panel radius | `rounded-md` | shadcn `DropdownMenuContent` default |
| Identity block padding | `px-2 py-2` | Matches tenant-admin `user-menu.tsx` exactly |
| Identity name | `text-sm font-semibold text-foreground truncate` | shadcn `--foreground` |
| Identity email | `text-xs text-muted-foreground truncate` | shadcn `--muted-foreground` |
| Menu item height | `py-1.5` (via `DropdownMenuItem` base class) | shadcn `dropdown-menu.tsx` item primitive |
| Menu item icon | size 14, `mr-2`, `aria-hidden="true"` | Tenant-admin `user-menu.tsx` |
| Keluar item color | `text-destructive focus:text-destructive` | shadcn `--destructive` |
| Keluar spinner | `Loader2 size={14} animate-spin` | Tenant-admin `user-menu.tsx` |
| Separator | `DropdownMenuSeparator` → `-mx-1 my-1 h-px bg-muted` | shadcn `dropdown-menu.tsx` |
| Open/close animation | Radix default (fade + zoom, ~150ms) | `DropdownMenuContent` in `dropdown-menu.tsx` |

---

### 9.5 States

**Trigger — default:**
Avatar circle with initials + first name (md+) + chevron. No border other than the subtle `ring-1 ring-border`.

**Trigger — hover:**
`hover:bg-accent` — applied via `Button variant="ghost"` base class. The avatar circle itself does not change color on hover; the surrounding button region receives the accent tint. Transition: 150ms ease-out (BS-T6 `fast`).

**Trigger — focus (keyboard):**
`focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1`. Ring is indigo for platform-admin (same `--ring` variable as KPI cards). Must not be suppressed — Radix does not suppress focus on the trigger.

**Trigger — open (active):**
Radix sets `data-state="open"` on the trigger. Optionally apply `data-[state=open]:bg-accent` to mirror the hover tint while the menu is open, signaling the relationship between trigger and panel. This is a polish addition — include it.

**Menu item — hover / keyboard focus:**
`focus:bg-accent focus:text-accent-foreground` — handled by shadcn `DropdownMenuItem` base class. No custom treatment needed.

**Menu item — disabled (placeholder routes):**
Until `/profil` and `/pengaturan` exist, items render with `aria-disabled="true"` and reduced opacity (`data-[disabled]:opacity-50` from the `DropdownMenuItem` base class). They are still focusable via keyboard but their `onSelect` is suppressed by Radix when `disabled` is set.

**Keluar — sign-out in progress:**
When `isPending` is true (from `useTransition`):
- `Loader2 size={14} class="mr-2 animate-spin"` replaces `LogOut` icon.
- `DropdownMenuItem disabled={isPending}` — prevents a second click.
- `motion-safe:animate-spin` should be used so the spinner stops under `prefers-reduced-motion`. If the user has reduced motion enabled, substitute a static `Loader2` without animation — the `disabled` state alone communicates in-progress.
- The item text "Keluar" remains visible (do not replace with "Memuat..." — the spinner is sufficient feedback for a fast action).

**Keyboard navigation order inside open menu:**
```
Tab / Arrow Down  →  Profil Saya
Arrow Down        →  Pengaturan
Arrow Down        →  Keluar
Arrow Up          ←  navigates back up
Home              →  Profil Saya (first item)
End               →  Keluar (last item)
Enter / Space     →  activates focused item
Escape            →  closes menu, focus returns to trigger
```
This is Radix `DropdownMenu` default behavior. No custom key handlers needed.

---

### 9.6 Behavior

**Link targets:**
- "Profil Saya" → `href="/profil"`. Route does not yet exist; placeholder treatment per §5.5.
- "Pengaturan" → `href="/pengaturan"`. Route does not yet exist; placeholder treatment per §5.5.
- Both use `<DropdownMenuItem asChild><Link href="...">` — the same pattern as tenant-admin `user-menu.tsx`. The menu closes automatically on navigation because Radix listens for the `select` event, which fires when a `Link` item is activated.

**Sign-out wiring:**
The "Keluar" `DropdownMenuItem` calls `logoutAction` via `useTransition`:

```
onSelect={(e) => {
  e.preventDefault();               // prevent Radix from closing the menu before transition starts
  startTransition(() => logoutAction());
}}
```

`logoutAction` is the same Server Action used by the existing `SignOutButton` at `app/dashboard/actions.ts`. The new component imports from the same path — no new server action is created. The `SignOutButton` component (`app/dashboard/sign-out-button.tsx`) is removed from all three header locations as part of this change.

**Where `SignOutButton` currently appears and must be replaced:**

| File | Location | Action |
|---|---|---|
| `app/dashboard/page.tsx` | Line 222, inside `<header>` | Replace with `<PlatformUserMenu>` |
| `app/tenants/layout.tsx` | Line 43, inside `<header>` | Replace with `<PlatformUserMenu>` |
| `app/payout/layout.tsx` | Line 47 (NavLinks only — no `SignOutButton` here yet) | Add `<PlatformUserMenu>` after `<NavLinks>` |

After this change, all three headers share the same user menu trigger. The `SignOutButton` component itself can be deleted if no other callers remain (verify with a codebase search before deleting).

**User data in layouts (non-dashboard pages):**
The dashboard page already fetches `/auth/me` and passes `user.full_name` / `user.email` to `ProfileRow`. The two layout files (`tenants/layout.tsx` and `payout/layout.tsx`) currently do not fetch `/auth/me` — they only fetch the pending registration count. To render the avatar and name in the dropdown, each layout must also fetch `/auth/me`.

Recommended approach for nextjs-expert: add a best-effort `/auth/me` call inside `TenantsLayout` and `PayoutLayout` alongside the existing pending-count fetch, using the same `Promise.allSettled` or a separate try/catch. If the call fails (e.g. network error), pass `fullName="?"` and `email=""` as fallbacks — the avatar will show "?" and the identity block will be empty rather than breaking the page. A 401 in these layouts should already redirect via the existing auth guard or the `apiFetch` wrapper.

**Component name:** `PlatformUserMenu` — a new `"use client"` component at `lustia/web/platform-admin/components/platform-user-menu.tsx`. This mirrors the tenant-admin pattern (`components/user-menu.tsx`) with the platform-admin item set. Do not reuse the tenant-admin component directly — the item list is different and the import path for `logoutAction` is platform-admin-specific.

---

### 9.7 Removal Checklist for nextjs-expert

The following must be deleted or emptied as part of implementing this section:

1. `app/dashboard/_components/profile-row.tsx` — delete the file.
2. `import { ProfileRow }` and `<ProfileRow ... />` in `app/dashboard/page.tsx` — remove both lines.
3. `<SignOutButton />` from `app/dashboard/page.tsx` header — remove; replace with `<PlatformUserMenu>`.
4. `<SignOutButton />` from `app/tenants/layout.tsx` header — remove; replace with `<PlatformUserMenu>`.
5. `import { SignOutButton }` from `app/tenants/layout.tsx` — remove if no longer used.
6. After confirming no other callers remain: delete `app/dashboard/sign-out-button.tsx`.
7. Add `<PlatformUserMenu>` to `app/payout/layout.tsx` header (it currently has no sign-out mechanism at all).
8. Add `/auth/me` fetch to `TenantsLayout` and `PayoutLayout` to supply `fullName` and `email` props.

The `logoutAction` Server Action in `app/dashboard/actions.ts` is retained — it is called by `PlatformUserMenu`.

**Mockup update:** the header sketches in §2A and §2B replace `[Sign Out btn]` with `[● BS ˅]` (desktop) and `[● BS ˅]` (mobile). The "Profil Anda" footer row is absent from both mockups.

---

## 10. Date Filter Audit (2026-05-03)

_Audited by `ui-ux-expert`. Scope: all `type="date"` inputs in `lustia/web/tenant-admin/` and `lustia/web/platform-admin/`._

### Audit result

All audited date inputs are correctly wired. The ops portal was the only offender.

### Evidence

**`lustia/web/tenant-admin/components/date-range-picker.tsx` (lines 60, 70)**
Both `<input type="date">` elements have `onChange={(e) => handleChange(fromParam/toParam, e.target.value)}`. `handleChange` calls `router.push(...)` to update the URL — the same correct pattern as the ops `FilterDate` component. Used by `app/booking/page.tsx:186`, `app/booking/reports/page.tsx:128`, and `app/keuangan/page.tsx:360`.

**`lustia/web/platform-admin/components/date-range-picker.tsx` (lines 49, 59)**
Same implementation, same `onChange → handleChange → router.push` pattern. Used by `app/payout/tenant-payout/page.tsx:74`.

**`lustia/web/tenant-admin/app/master/therapists/therapist-form.tsx:509`**
`<Input type="date" ... {...field} />` inside a React Hook Form `FormField`. The `{...field}` spread includes RHF's `onChange` handler, which updates the form state on every keystroke. This is a form-data input, not a filter input; it does not need to call `router.push` on change. This is correct behaviour.

### Summary table

| File | Line | Type | Pattern | Verdict |
|---|---|---|---|---|
| `tenant-admin/components/date-range-picker.tsx` | 60, 70 | Filter (URL-driven) | `onChange → router.push` | Correct |
| `platform-admin/components/date-range-picker.tsx` | 49, 59 | Filter (URL-driven) | `onChange → router.push` | Correct |
| `tenant-admin/app/master/therapists/therapist-form.tsx` | 509 | Form field | RHF `{...field}` spread | Correct |

No broken date filters found. The ops portal (`lustia/web/ops/`) was the only location that had `onChange={undefined}` (now fixed with `filter-date.tsx`). Neither tenant-admin nor platform-admin requires the ops `FilterDate` component to be ported.

---

## 11. Failed Disbursement Nav Badge

_Added by `ui-ux-expert`. Unlocks implementation by `nextjs-expert`._

---

### 11.1 Purpose

Surface a persistent, urgent nav signal when one or more disbursements are in `status=failed`. The operator must not need to navigate to `/payout` to discover a failure — the badge ensures the failure is visible on every authenticated screen.

---

### 11.2 ASCII Sketch — Nav (both badges at once)

```
┌──────────────────────────────────────────────────────────────────┐
│ [shield] Lustia Platform Console                                 │
│  [Dasbor]  [Tenant ⬤3]  [Payout ⬤2]  [Registrasi]   [● BS ˅]  │
└──────────────────────────────────────────────────────────────────┘

  [Tenant ⬤3]  — amber/primary fill badge (existing pending-registration pill)
  [Payout ⬤2]  — destructive fill badge (new failed-disbursement pill)

Single nav item expanded:

  ┌───────────────────────────────────────┐
  │  [Banknote icon]  Payout  ┌──┐        │
  │                           │ 2│        │  ← h-4, min-w-[1rem], px-1
  │                           └──┘        │    bg-destructive, text-destructive-foreground
  └───────────────────────────────────────┘
                               ↑
                  trailing the label, gap ml-0.5
                  visible only when failedDisbursementCount > 0
```

---

### 11.3 Token Mapping

No new tokens. All values reuse existing shadcn/ui CSS variables and established Tailwind classes.

| Element | Class / token | Source |
|---|---|---|
| Badge background | `bg-destructive` | shadcn `--destructive` |
| Badge text | `text-destructive-foreground` | shadcn `--destructive-foreground` (white on standard destructive red) |
| Badge size | `h-4 min-w-[1rem] px-1 text-[10px] leading-none` | Mirror of existing pending-count pill on `/tenants` |
| Badge position | `ml-0.5` trailing the label | Mirror of existing pending-count pill |
| Badge radius | `rounded-full` (shadcn `Badge` default) | Existing `Badge` primitive |
| Over-99 text | `"99+"` | Same cap as pending-count pill |
| Contrast | `--destructive-foreground` on `--destructive` | shadcn guarantees ≥ 4.5:1 for this pair |

The existing pending-count pill uses `variant="default"` (primary fill, neutral indigo/blue). The failed badge uses `bg-destructive` — a semantic switch from "queue" to "broken". The two badges are visually orthogonal: different hues, different semantic weight. A color-blind operator still distinguishes them because the failed badge sits on the Payout link (different position) and the pending badge sits on the Tenant link.

---

### 11.4 Behavior Table

| Condition | Badge state |
|---|---|
| `failedDisbursementCount === 0` | Badge absent — no DOM node rendered. "Inbox zero" reward. |
| `failedDisbursementCount` 1–99 | Badge shows exact count. |
| `failedDisbursementCount` > 99 | Badge shows `"99+"`. |
| Fetch errors (network / 5xx) | Treat as 0 — badge absent. Do not block nav render. |
| Loading (server fetch in progress) | Server Component — page does not render until fetch settles; no client loading shimmer needed. |

---

### 11.5 Data Flow

```
ConsoleHeader (or each layout's Server Component)
  │
  ├─ fetch /admin/tenant-registrations?status=pending&limit=1  → pendingCount
  ├─ fetch /admin/disbursements?status=failed&limit=1          → failedCount  (NEW)
  │     read total_count from AdminDisbursementListResponse
  │     on error → failedCount = 0
  │
  └─ <NavLinks pendingCount={n} failedDisbursementCount={m} />
```

Both fetches run in parallel via `Promise.allSettled`. The failed-count fetch is a second branch alongside the already-existing pending-count fetch — same URL shape, same response envelope key (`total_count` or `total` — confirm against API_CONTRACT.md and normalise at callsite).

Call sites that render `<NavLinks>` (and therefore `<ConsoleHeader>`):

| File | Current props | Required addition |
|---|---|---|
| `app/dashboard/page.tsx` | `pendingCount` | add `failedDisbursementCount` |
| `app/tenants/layout.tsx` | `pendingCount` | add `failedDisbursementCount` |
| `app/payout/layout.tsx` | `pendingCount` | add `failedDisbursementCount` |

All three already fetch the pending count; each needs one additional parallel fetch.

---

### 11.6 Component Diff (for nextjs-expert)

`NavLinksProps` gains one new optional prop with a safe default:

```ts
interface NavLinksProps {
  pendingCount: number;
  failedDisbursementCount?: number;   // default 0
}
```

Inside the render loop, mirror the existing pending pill guard:

```tsx
{href === "/payout" && (failedDisbursementCount ?? 0) > 0 && (
  <Badge
    className="ml-0.5 h-4 min-w-[1rem] px-1 text-[10px] leading-none
               bg-destructive text-destructive-foreground"
    aria-label={`${failedDisbursementCount} disbursement gagal — perlu tindakan`}
  >
    {(failedDisbursementCount ?? 0) > 99 ? "99+" : failedDisbursementCount}
  </Badge>
)}
```

Note: do NOT pass `variant="default"` or `variant="destructive"` to the Badge — those shadcn variants apply their own bg/text via CSS variables. Pass the Tailwind bg/text classes directly in `className` instead, exactly as the existing pending pill does.

---

### 11.7 Accessibility

| Criterion | Implementation |
|---|---|
| WCAG 1.4.1 Color not sole differentiator | Badge carries numeric text count. Position (Payout link vs. Tenant link) + count text both carry signal independent of color. |
| WCAG 1.4.3 Contrast ≥ 4.5:1 | `text-destructive-foreground` (white, #fff) on `bg-destructive` (shadcn default ≈ hsl(0 84.2% 60.2%) ≈ #f05252) = ~4.6:1. Passes AA. Verify against the actual `--destructive` value in `globals.css` if the theme has been customised. |
| WCAG 2.4.6 Meaningful label | `aria-label="{n} disbursement gagal — perlu tindakan"` describes both the count and the action required. Screen readers announce: "Payout, 2 disbursement gagal — perlu tindakan, link". |
| WCAG 1.4.1 Not relying on color alone | The badge text ("2", "99+") is sufficient without color. The `aria-label` provides full context for screen readers. |
| Motion | Badge is static — no animation on mount. No `prefers-reduced-motion` concern. |
| Touch target | The badge is decorative within the nav `<Link>` — the link itself is the interactive target (≥ 44×24 px from `px-3 py-1.5`). The badge does not add a secondary tap target. |
