# Design System

_Owned by `ui-ux-expert`. Consumed by `nextjs-expert` and `flutter-expert`._

## Design tokens

### Spacing scale (4 base)
`4, 8, 12, 16, 24, 32, 48, 64, 96`

### Type scale
| Token | Size / line-height | Usage |
| --- | --- | --- |
| `display-lg` |  |  |
| `heading-md` |  |  |
| `body-md` |  |  |
| `caption` |  |  |

### Color (semantic roles)
| Token | Light | Dark | Usage |
| --- | --- | --- | --- |
| `bg-surface` |  |  |  |
| `bg-elevated` |  |  |  |
| `text-primary` |  |  |  |
| `text-muted` |  |  |  |
| `border-subtle` |  |  |  |
| `accent` |  |  |  |
| `danger` |  |  |  |
| `success` |  |  |  |

### Radius
`0, 4, 8, 12, 16, 999`

### Motion
| Token | Duration | Easing | Usage |
| --- | --- | --- | --- |
| `fast` | 150ms | ease-out | Hover, fade |
| `medium` | 225ms | ease-out | Modal, drawer |

## Component inventory

### shadcn/ui primitives (vendored into each web app under `components/ui/`)
- [x] Button (primary, secondary, ghost, outline, link variants) — `components/ui/button.tsx`
- [x] Input — `components/ui/input.tsx`
- [x] Label — `components/ui/label.tsx`
- [x] Card, CardHeader, CardContent, CardFooter, CardTitle, CardDescription — `components/ui/card.tsx`
- [x] Form (RHF-wired: Form, FormField, FormItem, FormLabel, FormControl, FormMessage) — `components/ui/form.tsx`
- [x] Toaster (sonner wrapper) — `components/ui/sonner.tsx`
- [ ] Input, Textarea, Select (Textarea/Select still needed)
- [ ] Checkbox, Radio, Switch
- [ ] Modal, Drawer
- [ ] Table, Pagination
- [ ] Nav, Tabs
- [ ] Empty / Loading / Error states

### App-level composed components (Phase 2 login + dashboard)
- [x] `LoginForm` — email/password/tenant-slug form wired to Server Action (all three portals)
- [x] `SignOutButton` — client component wrapping logout Server Action
- [x] `ProfileField` — key/value display for user profile data
- [x] `DashboardLoading` — skeleton loading state for dashboard
- [x] `MustChangePasswordBanner` — yellow alert banner for password rotation gate (inline in dashboard page)

**Token additions made in `app/globals.css` (flagged for `ui-ux-expert` review):**
- `--ring`: per-portal focus ring colour (indigo/teal/amber). Not yet in `DESIGN_SYSTEM.md` token table.

## Screen flows
_List and link Mermaid flow diagrams per major user journey._

### Phase 2 — Login flow (all three portals)
```
/ → cookie present? → /dashboard (Server Component fetches /auth/me)
                  ↓ no cookie
               /login → LoginForm (RHF + Server Action)
                           ↓ POST /auth/login
                           ↓ role check (JWT decode, server-side)
                           ↓ pass → set HttpOnly cookies → redirect /dashboard
                           ↓ fail → toast (role mismatch / bad credentials / rate limit)
```

## Accessibility baseline
- Contrast ≥ 4.5:1 text, 3:1 UI
- Focus visible on all interactive elements
- Keyboard navigable, logical tab order
- `prefers-reduced-motion` respected
- Target size ≥ 24×24 px (44×44 on touch)

## Open questions
-

---

## Phase 4 — Master Operational Data (Therapists, Services, Availability)

_Owned by `ui-ux-expert`. Implemented by `nextjs-expert`. Backend by `go-expert`._

---

### 1. Information Architecture

**Decision: add a top-level "Operasional" nav item** in `AppHeader`, on par with "Dasbor" and "Cabang". Do not put it under a dropdown.

Rationale: therapist CRUD and service CRUD are daily operational tasks — burying them two levels deep adds unnecessary friction. The header currently has two nav items; three remains comfortable at the `max-w-5xl` container width without truncating on a 1280-wide viewport. A disclosure/secondary-nav creates a hidden affordance problem: tenant_admins who are new to the portal will not find master data when they need it most (initial setup).

**`NavKey` extension:** add `"operasional"` to the union in `app-header.tsx`.
**Icon:** `Stethoscope` (lucide) for the nav item.

#### Route tree

```
/master
  /therapists              — list
  /therapists/new          — create form
  /therapists/[id]         — detail + edit (profile tab, availability tab, services tab)
  /services                — list
  /services/new            — create form
  /services/[id]           — detail + edit (profile tab, therapists tab — read-only)
```

A shared `app/master/layout.tsx` mirrors `app/branches/layout.tsx`: fetches `/auth/me`, renders `AppBackground` + `AppHeader activeNav="operasional"`, wraps `<main className="mx-auto max-w-5xl px-6 py-8">`.

**Availability lives on the therapist detail page as a tab** (not its own route). The three concerns on `/master/therapists/[id]` — profile, services, availability — are tightly related to one subject (the therapist). A tab layout keeps them co-located without a long single-scroll page. Route-level splitting would add a meaningless URL hop.

---

### 2. Screen Specs

#### 2.1 Therapist List — `/master/therapists`

**Purpose:** browse, search, and manage all therapists visible to the caller.

**Layout:**

```
[Page header row]  "Terapis"  [subtitle]              [+ Tambah Terapis]
[Filter bar]       Branch dropdown (tenant_admin only, >1 branch) | Status toggle (Semua / Aktif / Nonaktif)
[Table / card grid]
  Columns: Foto avatar (32×32) | Nama | Cabang (tenant_admin only) | Status badge | Bergabung sejak | ⋯ menu
[Pagination]
```

Above the fold: page header + filter bar + first ~10 rows. No secondary fold concern.

**Data shown per row:** `full_name`, `branch.name` (conditional), `is_active` badge (`Aktif` / `Nonaktif`), `joined_at` date (tabular-nums), avatar initial fallback if no photo.

**Primary action:** "Tambah Terapis" button (primary variant, top-right of page header). Row ⋯ menu: "Lihat / Edit", "Aktifkan" or "Nonaktifkan" (context-sensitive), "Hapus".

**Branch column rule:**
- `tenant_admin` + more than one branch: show branch column + branch filter dropdown.
- `tenant_admin` + exactly one branch: hide branch column (implicit). Hide filter dropdown.
- `branch_admin`: never show branch column; never show filter dropdown. Implicit scope.

**Empty state:** centered illustration area inside a dashed Card.
> "Belum ada terapis. Tambahkan terapis pertama untuk memulai." + "Tambah Terapis" button.

**Loading state:** `DashboardLoading`-style skeleton — three rows of [avatar circle | two line stubs | badge stub | date stub].

**Error state:** full-width inline alert (`border-red-300 bg-red-50`) with icon + "Gagal memuat data terapis. Coba lagi." + retry button.

**Mobile (< 640px):** table collapses to card-per-therapist. Show: avatar + name (bold) + branch (if applicable) + status badge. Tap the card to navigate to detail. ⋯ menu becomes a tap target bottom-right of each card. Pagination remains.

---

#### 2.2 Therapist Detail + Edit — `/master/therapists/[id]`

**Purpose:** edit profile, manage assigned services, set weekly availability.

**Layout:** page header ("Detail Terapis" + name as subtitle) then a `Tabs` component with three tabs:

| Tab | Label | Content |
|---|---|---|
| `profil` | Profil | Edit form (fields below) |
| `layanan` | Layanan | Assigned services mapper |
| `ketersediaan` | Ketersediaan | `AvailabilityEditor` component |

Default active tab: `profil`. Tab state is URL-driven via `?tab=profil|layanan|ketersediaan` so deep-linking works and back/forward navigation is correct.

**Profile tab:** standard RHF+zod form. Fields: Nama Lengkap (required), No. Telepon, Email, Bio (Textarea, max 500 chars), Bergabung Sejak (date input). Photo upload is **deferred — see scope flag §7**. Save via Server Action. Save button at bottom of form: "Simpan Perubahan". Inline field-level error messages via `FormMessage`.

**Loading state:** skeleton matching the form field layout (3 input height stubs, 1 textarea stub, 1 date stub).

**Error state:** toast "Gagal menyimpan. Coba lagi." (sonner, destructive variant). Form stays populated; user does not lose input.

**Mobile:** full-width single-column form. Tabs scroll horizontally if labels overflow (shadcn `Tabs` default behavior is adequate).

---

#### 2.3 Therapist New — `/master/therapists/new`

**Purpose:** create a therapist profile.

**Layout:** page header "Tambah Terapis" + back link "← Kembali ke Daftar Terapis". Single-column form, identical fields to the profile tab on detail. Branch selector (Select) shown at top only for `tenant_admin` to assign the therapist to a branch — required field. `branch_admin` never sees it (server injects branch from JWT).

**Primary action:** "Simpan & Buat Terapis" (primary button, bottom of form). Secondary: "Batal" ghost button navigates back.

**Post-save redirect:** navigate to `/master/therapists/[newId]?tab=layanan` so the user is immediately prompted to assign services. Toast: "Terapis berhasil ditambahkan."

**Empty / loading / error:** same patterns as 2.2.

---

#### 2.4 Service List — `/master/services`

**Purpose:** browse tenant-scoped services (not branch-filtered — services are tenant-wide per ADR 0009).

**Layout:**

```
[Page header]  "Layanan"  [subtitle]                 [+ Tambah Layanan]
[Filter bar]   Kategori dropdown | Status toggle (Semua / Aktif / Nonaktif)
[Table]
  Columns: Nama | Kategori badge | Durasi | Harga | Status | ⋯ menu
```

No branch column or branch filter anywhere on this screen — services are tenant-scoped.

**Data shown:** `name`, `category` (Badge, color-coded by category value), `duration_minutes` formatted as "60 menit", `price_idr` formatted as "Rp 150.000" (tabular-nums, `id-ID` locale), `is_active` badge.

**Empty state:** "Belum ada layanan. Tambahkan layanan pertama untuk memulai." + "Tambah Layanan" button.

**Loading / error / mobile:** same patterns as therapist list (§2.1). No branch card-collapse needed; table columns are fewer.

---

#### 2.5 Service Detail + Edit — `/master/services/[id]`

**Purpose:** edit service data; see which therapists currently offer it.

**Layout:** two tabs.

| Tab | Label | Content |
|---|---|---|
| `detail` | Detail | Edit form |
| `terapis` | Terapis | Read-only list of therapists offering this service |

**Detail tab fields:** Nama Layanan (required), Kategori (Select), Durasi (number input, suffix "menit"), Harga (number input, prefix "Rp"), Deskripsi (Textarea). Save: "Simpan Perubahan".

**Terapis tab:** read-only table of therapists currently mapped to this service. Columns: Nama | Cabang | Status mapping (`Aktif` / `Nonaktif`). Caption: "Penugasan dikelola dari halaman masing-masing terapis." Link each row to `/master/therapists/[id]?tab=layanan`. No edit controls on this tab — mapping is managed from the therapist side exclusively.

---

#### 2.6 Service New — `/master/services/new`

**Purpose:** create a service.

Identical layout to Therapist New (§2.3). Fields from §2.5 detail tab. Post-save: navigate to `/master/services/[id]`, toast "Layanan berhasil ditambahkan."

---

### 3. Non-Trivial Component Designs

#### 3A. AvailabilityEditor

**Recommended pattern: per-day rows with multi-window support.**

Rationale: the `OperationalHoursField` in `app/branches/` is already exactly this pattern and users already know it. A grid/slot view is powerful but overkill — Phase 4 availability is a coarse weekly declaration, not a slot-level booking grid. A "hybrid with preview" adds build cost for marginal value at this phase. Keeping the same row-based affordance minimises cognitive load for admins who already set up branch hours the same way.

**Multi-window support (important distinction from branch hours):** a therapist CAN have two windows on the same day (e.g., 09:00–12:00 and 14:00–18:00). `OperationalHoursField` supports only one window per day. `AvailabilityEditor` must extend the data model to an array of windows per day.

**Internal data model:**

```
type Window = { start: string; end: string }          // "HH:MM"
type DayWindows = { active: boolean; windows: Window[] }
type WeeklyAvailability = Record<DayKey, DayWindows>
```

Serialised to the API shape `[{dow: 1, start: "09:00", end: "12:00"}, {dow: 1, start: "14:00", end: "18:00"}, ...]`.

**Per-day row layout:**

```
[Day label 8rem] [Kerja / Tidak kerja checkbox] [window chips row] [+ Tambah Jendela]
```

When `active = false`: day label + checkbox only; windows hidden (opacity-40, pointer-events-none), no "+ Tambah Jendela".

Each window chip:
```
[time input start] – [time input end]  [× remove]
```

"+ Tambah Jendela" button (ghost, size sm) appends a new window with defaults. Maximum 3 windows per day (practical limit; validate with zod).

**Copy button:** "Salin ke semua hari kerja" (copies all windows from Monday to Tue–Fri). Matches the precedent in `OperationalHoursField` (`copyMondayToWeekdays`). Button is disabled if Monday is inactive or has no windows.

**Validation rules (zod):**
1. `start` must be before `end` on each window.
2. Windows on the same day must not overlap. Check all pairs: `windowA.end <= windowB.start` for sorted windows.
3. If `active = true`, at least one window must exist.
4. Time precision: whole 5-minute increments (`step={300}` on `<input type="time">`).

**Keyboard interaction:** Tab moves through time inputs in document order. `×` remove button is keyboard-reachable and has `aria-label="Hapus jendela [start]–[end]"`. "Tidak kerja hari ini" checkbox collapses the window row with a 150ms opacity transition (respects `prefers-reduced-motion`).

**Mobile (< 640px):** window chips wrap below the checkbox. Day label and checkbox on one line; windows in a stacked sub-row below with full-width time inputs. "+ Tambah Jendela" spans full width.

**Phase 5 forward-compat:** availability is stored per-therapist as raw time windows with no reference to services or branches other than the therapist's own `branch_id`. The booking engine in Phase 5 intersects `therapist_availability` + `branch_operational_hours` + `therapist_service` to answer availability queries. **Do not add a service selector inside `AvailabilityEditor` — that would break the clean intersection model.** Flag to go-expert: the API shape `[{dow, start, end}]` must remain service-agnostic.

---

#### 3B. Therapist ↔ Service Mapping UI

**Recommended pattern: multi-select combobox with inline activation toggle on the therapist detail page (Layanan tab).**

Rationale: a dedicated modal adds a tap/click to open, a second click to commit, and a close interaction — three extra interactions for what is fundamentally a checklist. A combobox with search handles the scale concern (if a tenant has 30+ services, the user can type to filter). The dedicated modal pattern is appropriate for relationship objects with their own attributes (e.g., pricing overrides per-therapist); Phase 4 mappings have no such attributes beyond `is_active`, which the inline toggle handles.

**Layout of Layanan tab:**

```
[Section header: "Layanan yang Ditawarkan"]
[Combobox: "Cari dan tambah layanan..." — opens a popover with checkboxes]
[List of currently-assigned services as rows]
  Each row: [service name] [category badge] [duration] [Aktif toggle | Switch] [Lepas button]
```

The combobox (shadcn `Popover` + `Command`) shows all tenant services with a checkbox. Already-assigned services are pre-checked. Checking adds the mapping; unchecking prompts an `AlertDialog` "Anda yakin ingin melepas layanan ini dari terapis?" before removing. This protects against accidental removal.

**`is_active` on the mapping row:** surfaced as a `Switch` on each assigned-service row, labelled "Aktif". Toggle sends `PATCH /therapists/:id/services/:serviceId/status` (or equivalent per go-expert's chosen endpoint granularity). This allows "temporarily not offering this service" without removing the mapping — matching the ADR 0009 intent. The distinction between deactivate (keep row, `is_active = false`) and remove (delete row) must be visually clear: use muted row styling for inactive mappings, with the Switch clearly Off.

**States on the combobox:** loading spinner inside the popover while fetching the services list; empty state "Tidak ada layanan ditemukan" if search returns nothing; error inline "Gagal memuat layanan."

**Mobile:** combobox popover becomes a bottom drawer (`Drawer` from shadcn/vaul) on viewports < 640px. Assigned services list is a single-column card stack with the Switch and Lepas button on the same row.

---

### 4. Microcopy

#### Nav item
- Label: **"Operasional"** (preferred over "Master Data" — more natural in Indonesian operational context; "Master Data" is a technical term unfamiliar to non-technical users)

#### Page titles + subtitles
| Screen | `<h1>` | Subtitle / `<p>` |
|---|---|---|
| Therapist list | Terapis | Kelola data terapis di semua cabang Anda |
| Therapist detail | Detail Terapis | (therapist full_name as secondary label below h1) |
| Therapist new | Tambah Terapis | Isi data dasar terapis baru |
| Service list | Layanan | Kelola katalog layanan untuk semua cabang |
| Service detail | Detail Layanan | (service name as secondary label) |
| Service new | Tambah Layanan | Isi detail layanan baru |

#### Therapist form field labels
| Field | Label | Note |
|---|---|---|
| `full_name` | Nama Lengkap | Required |
| `phone` | No. Telepon | Optional |
| `email` | Email | Optional |
| `bio` | Bio | Textarea; placeholder "Ceritakan sedikit tentang terapis ini..." |
| `joined_at` | Bergabung Sejak | Date; placeholder "dd/mm/yyyy" |
| `branch_id` | Cabang | Required; shown to tenant_admin only on /new |

#### Service form field labels
| Field | Label | Note |
|---|---|---|
| `name` | Nama Layanan | Required |
| `category` | Kategori | Select |
| `duration_minutes` | Durasi | Number; suffix label "menit" |
| `price_idr` | Harga | Number; prefix "Rp"; `id-ID` locale formatting on display |
| `description` | Deskripsi | Textarea; optional |

#### CTA buttons
| Action | Label | Variant |
|---|---|---|
| Create therapist | Tambah Terapis | primary |
| Create service | Tambah Layanan | primary |
| Save edits | Simpan Perubahan | primary |
| Save new | Simpan & Buat | primary |
| Activate | Aktifkan | outline (success tone) |
| Deactivate | Nonaktifkan | outline (warning tone) |
| Delete | Hapus | outline (destructive) |
| Assign services (combobox trigger) | Tambah Layanan | ghost |
| Remove mapping | Lepas | ghost (destructive) |
| Cancel | Batal | ghost |
| Copy availability to weekdays | Salin ke hari kerja | ghost |
| Add time window | + Tambah Jendela | ghost |

#### Confirmation dialogs (AlertDialog)
**Delete therapist:**
- Title: "Hapus Terapis?"
- Body: "Data [Nama Terapis] akan dihapus secara permanen dan tidak dapat dipulihkan. Tindakan ini tidak dapat dibatalkan."
- Confirm: "Ya, Hapus" (destructive)
- Cancel: "Batal"

**Delete service:**
- Title: "Hapus Layanan?"
- Body: "Layanan [Nama Layanan] akan dihapus. Terapis yang memiliki layanan ini akan kehilangan penugasannya."
- Confirm: "Ya, Hapus" (destructive)
- Cancel: "Batal"

**Remove service mapping:**
- Title: "Lepas Layanan?"
- Body: "Layanan [Nama Layanan] akan dilepas dari terapis ini."
- Confirm: "Ya, Lepas" (destructive)
- Cancel: "Batal"

#### Empty states
- Therapist list: "Belum ada terapis. Tambahkan terapis pertama untuk memulai."
- Service list: "Belum ada layanan. Tambahkan layanan pertama untuk memulai."
- Therapist's assigned services (Layanan tab, empty): "Terapis ini belum memiliki layanan. Gunakan pencarian di atas untuk menambahkan."
- Service's therapist list (read-only tab, empty): "Belum ada terapis yang menawarkan layanan ini."

#### Toasts (sonner)
| Event | Message | Variant |
|---|---|---|
| Therapist created | "Terapis berhasil ditambahkan." | success |
| Therapist updated | "Perubahan berhasil disimpan." | success |
| Therapist deleted | "Terapis berhasil dihapus." | success |
| Therapist activated | "Terapis diaktifkan." | success |
| Therapist deactivated | "Terapis dinonaktifkan." | success |
| Service created | "Layanan berhasil ditambahkan." | success |
| Service updated | "Perubahan berhasil disimpan." | success |
| Service deleted | "Layanan berhasil dihapus." | success |
| Availability saved | "Jadwal ketersediaan disimpan." | success |
| Service mapping added | "Layanan berhasil ditugaskan." | success |
| Service mapping removed | "Layanan berhasil dilepas." | success |
| Any mutation failure | "Gagal menyimpan. Coba lagi." | destructive |
| Load failure | "Gagal memuat data. Coba muat ulang halaman." | destructive |

---

### 5. Roles and Permissions

| Element | `tenant_admin` (>1 branch) | `tenant_admin` (1 branch) | `branch_admin` |
|---|---|---|---|
| Branch filter dropdown (therapist list) | Visible | Hidden | Hidden |
| Branch column (therapist list table) | Visible | Hidden | Hidden |
| Branch selector (therapist /new form) | Visible, required | Hidden (auto-assigned) | Hidden (auto-assigned from JWT) |
| Service list/CRUD | Full access | Full access | Full access (services are tenant-scoped; branch_admin can manage services) |
| Therapist list | All branches | All branches | Own branch only |

**Branch_admin** is further scoped at the API layer (RLS + service-layer check per ADR 0009 §2.7). The UI only needs to hide the branch controls — it does not enforce isolation itself.

---

### 6. Phase 5 Forward-Compatibility Flags

1. **Availability is service-agnostic.** `AvailabilityEditor` stores windows as `{dow, start, end}` only. No service_id field. The Phase 5 booking engine intersects availability with `therapist_service` separately. If a service selector were added here, the Phase 5 availability check would need special-casing.

2. **Availability windows are not branch-hours-bounded in the UI.** The editor does not visually clip windows to `branch.operational_hours`. In Phase 5, the booking engine will enforce this constraint server-side. Adding UI enforcement now would be premature and would require a branch-hours API call on every availability edit — scope that for Phase 5 when the constraint is actually enforced.

3. **No date overrides.** ADR 0009 §2.5 explicitly defers time-off and holiday exceptions to Phase 5. `AvailabilityEditor` must not expose a "date exception" affordance or the UX will diverge from the data model.

4. **`is_active` on `therapist_service` mapping** is surfaced in the Phase 4 UI (Switch per row). Phase 5 booking must respect this flag when finding available therapists. Flag to go-expert: the availability query endpoint in Phase 5 must filter `therapist_service.is_active = true`.

5. **Therapist `user_id` link.** Phase 4 admin creates a therapist profile that may or may not link to a portal user. Phase 5 (or the ops portal in Phase 4.5) will let therapists log in and view their own schedule. The detail form should reserve a read-only "Akun terhubung" field (show user email if linked, "Belum terhubung" if not) so the relationship is visible to admins — but the linking UI itself is deferred.

---

### 7. Scope Flags (items that should NOT ship in Phase 4)

- **Therapist photo upload.** Requires file storage (S3 or equivalent) not yet provisioned. Defer to Phase 5 or a dedicated Phase 4.5 media sprint. The form shows an avatar initial fallback; the `avatar_url` field accepts a URL string as a hidden future extension.
- **Service category management UI.** Categories exist as a Select on the service form. The list of valid categories is treated as a fixed enum for Phase 4. A "Kelola Kategori" CRUD screen is Phase 6+ scope.
- **Bulk actions** (bulk activate, bulk deactivate, bulk assign). Standard list interaction for Phase 6.
- **Therapist "Akun terhubung" linking UI.** Show the linked user email as read-only (§6 point 5), but the flow to search users and create the link is deferred.
- **Cross-branch therapist roster view** (one human, two branches). ADR 0009 §2.5 explicitly defers. The "Akun terhubung" field is the only Phase 4 bridge.

---

### 8. New Components — Component Inventory Additions

Add to the component inventory table (§ "App-level composed components"):

| Component | Path | Description |
|---|---|---|
| `AvailabilityEditor` | `components/availability-editor.tsx` | Per-day weekly availability editor with multi-window support. Controlled: accepts `value: AvailabilityWindow[]` + `onChange`. Wraps `OperationalHoursField`'s row layout; extends it with per-day multi-window arrays and overlap validation. |
| `ServiceCombobox` | `components/service-combobox.tsx` | Popover-based searchable multi-select for assigning services to a therapist. Accepts `tenantServices`, `assignedServiceIds`, `onToggle`. Mobile variant uses `Drawer`. |
| `AssignedServiceRow` | `components/assigned-service-row.tsx` | Single row in the assigned-services list: name, category badge, duration, `is_active` Switch, Lepas button. Used inside the Layanan tab. |
| `TherapistStatusBadge` | `components/therapist-status-badge.tsx` | `Badge` wrapper mapping `is_active` → "Aktif" (success) / "Nonaktif" (muted). Reuses existing `Badge` variants. |
| `MasterPageHeader` | `components/master-page-header.tsx` | Shared page header for all /master/* pages: `<h1>` + optional subtitle + optional right-side slot (primary CTA). Mirrors the pattern used in /branches. |

---

### 9. Token Audit

No new design tokens are required. All Phase 4 screens use existing tokens:
- Spacing: existing 4-base scale covers all layouts.
- Color: `success` and `danger` semantic tokens (already declared in the token table, values TBD) cover Aktif/Nonaktif badges and destructive CTAs. Confirm `success` maps to emerald-600 (consistent with the emerald/teal primary palette) and `danger` maps to red-600. These were already listed as needed tokens in the table — Phase 4 is the first phase that exercises them heavily, so the nextjs-expert should ensure they resolve correctly in `globals.css`.
- `--ring` focus ring: already added as a token in globals.css (per existing Phase 2 note). No change needed.
- Motion: `fast` (150ms ease-out) for checkbox/toggle transitions; `medium` (225ms ease-out) for the ServiceCombobox popover and Drawer. Both exist.

One flag: the `type scale` rows in the token table are empty (values not yet specified). Phase 4 does not add new scale steps, but the `nextjs-expert` is consuming heading-md and body-md in the new screens. **The orchestrator should instruct the `ui-ux-expert` to fill in the type scale values before the nextjs-expert ships Phase 4.**
