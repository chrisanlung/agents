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
  /addons                  — tenant-wide add-on catalog list (ADR 0010)
  /addons/new              — create add-on form
  /addons/[id]             — edit add-on form
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

### Therapist IA decision — 2026-04-22

**Verdict: stay on Option A (global list with branch filter). No route changes.**

**Rationale.** The dominant user is a single-branch starter-tenant owner. For that person, branch-nesting adds a mandatory click through Cabang before they can touch any therapist — pure overhead. For the multi-branch growth/enterprise tenant, the branch filter on the global list answers "show me Kemang staff" in one interaction, without losing the cross-branch view they need for cross-branch ops ("who is available company-wide today?"). Branch_admin's world is already constrained server-side and in the UI via the implicit scope rule — they land on the global list and see only their own branch, so the IA feels branch-local to them without nesting. Option B breaks the `tenant_admin` (1 branch) case with no benefit. Option C duplicates a surface that would need to stay in sync, creates two "sources of truth" in the user's head, and adds build cost for minimal gain at current scale.

**What to polish on Option A (not blockers, but should land before Phase 5):**
- The filter pills are server-rendered `<Link>` elements. When the tenant has >3 branches the filter row wraps awkwardly. Cap visible branch pills at 3; overflow into a `<Select>` dropdown for branches 4+. Threshold: `branches.length > 3`.
- The empty state when a branch filter is active should read "Belum ada terapis di cabang ini." rather than the generic message — currently the same string is shown regardless of filter state.
- Page subtitle ("Kelola data terapis di semua cabang Anda") is wrong for `branch_admin` (they see only one branch). Conditionally render "Kelola data terapis di cabang Anda" for that role.

**Cross-pattern ruling — availability, service mapping, operational data in general:**

Keep the current tab structure on `/master/therapists/[id]` (Profil / Layanan / Ketersediaan). These three concerns are properties of _one subject_ (the therapist record), not properties of a branch. Moving them into branch detail would mean navigating Cabang → [branch] → Terapis tab → [therapist] → availability — four hops instead of two, and the therapist is no longer a first-class object. The branch detail page (`/branches/[id]`) should remain scoped to branch-level configuration: name, address, operational hours, status. Therapist availability and service mapping belong on the therapist detail page. No cross-concern consolidation needed now.

**Phase 5 forward-compat.** The booking engine's core query is "which therapists at branch X can perform service Y at time T." That is a _read query_ against the existing data model — it does not require the managing UI to be branch-nested. The global list with `?branch_id=` filter is already a valid entry point for an ops staffer building a booking: they filter the list to the relevant branch, pick a therapist, and enter the booking flow. If Phase 5 adds a dedicated "create booking" flow, it will likely start from a date/service/branch picker (not a therapist list at all) and resolve available therapists as a step in that wizard — the IA of the master data pages is irrelevant to that flow.

**Microcopy adjustments (conditional on role):**

| Element | `tenant_admin` (>1 branch) | `tenant_admin` (1 branch) | `branch_admin` |
|---|---|---|---|
| Page subtitle | "Kelola data terapis di semua cabang Anda" | "Kelola data terapis Anda" | "Kelola data terapis di cabang Anda" |
| Empty state (no filter active) | "Belum ada terapis. Tambahkan terapis pertama untuk memulai." | same | same |
| Empty state (branch filter active) | "Belum ada terapis di cabang ini." | n/a | "Belum ada terapis di cabang Anda." |

No nav label changes, no route additions, no redirects required.

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

---

### List-page UX review — 2026-04-24

_Scope: `/master/therapists` and `/master/services` list pages only. Produced after reading the current implementation in full. No new shadcn primitives are introduced. All changes are spec-level; `nextjs-expert` executes._

---

#### Finding 1 — Visual hierarchy (CRITICAL — fix before any other change)

**Problem.** The page title (`text-xl font-semibold`) and the filter bar's "FILTER" label (`text-xs font-medium uppercase tracking-wide`) are fighting for the same visual tier. The border + background on the filter bar pulls the eye as strongly as the `<h1>`, which should be the dominant element. The "Aksi" column header is right-aligned while all content is left-aligned — a minor misalignment that erodes table scannability.

**Fix — page header:**
- `<h1>` stays `text-xl` on mobile but steps up to `text-2xl` on `sm:` and above (`sm:text-2xl font-semibold`). Currently `text-xl` is the same size as a card title — the page needs more authority.
- Subtitle stays `text-sm text-muted-foreground` but add `mt-0.5` (currently `mt-1`) — 4px reduction tightens the pairing so title + subtitle read as one unit, not two independent lines.
- "Tambah ..." button: keep primary variant. Add `size="default"` explicitly (it's the current default but makes intent clear to future readers). The gap between title block and button is fine at `justify-between`.

**Fix — "Aksi" column:**
- Remove `text-right` from the `<TableHead>` for "Aksi". Keep `text-right` on the `<TableCell>` for row actions. When the header is right-aligned and the column only contains a ghost icon button centered-in-cell, the misalignment is visible at a glance. Header: center or left; cell content: keep right-aligned within the cell via `flex justify-end`.

**Token clarification needed:** fill in the type scale table (currently empty) with:
| Token | Value |
|---|---|
| `heading-lg` | `text-2xl / line-height 1.2 / font-semibold` |
| `heading-md` | `text-xl / line-height 1.25 / font-semibold` |
| `body-md` | `text-sm / line-height 1.5 / font-normal` |
| `caption` | `text-xs / line-height 1.4 / font-normal` |

---

#### Finding 2 — List row design: keep Table, improve therapist row identity (HIGH)

**Decision: keep `<Table>` on both pages. Do not switch to card-per-row.**

Rationale: tenant size is 2–20 therapists, 2–30 services. At this scale a table is the right density. Card-per-row would reduce the visible count from ~10 to ~5–6 before scroll on a laptop viewport, which is worse for a list you scan to find a specific name. The existing avatar in the therapist table already provides visual identity. The card-per-row form factor is reserved for mobile (< 640px) per the existing design system spec.

**Therapist row — keep the avatar+name column, but merge avatar and name into a single `<TableCell>` flex group:**

Current: two cells — a narrow avatar cell (`w-10`) and a separate "Nama" cell. This creates a visual gap between the avatar and the name that weakens the identity cluster.

Spec: collapse into one cell. Width: `w-[220px]` min-width (or equivalent `min-w-[180px]`). Content:
```
<div class="flex items-center gap-2.5">
  <TherapistAvatar name={t.full_name} />     // existing component, keep as-is
  <span class="font-medium text-foreground leading-tight">{t.full_name}</span>
</div>
```
Remove the standalone avatar `<TableCell className="w-10">`. Result: one cell, better visual grouping, one fewer DOM column to maintain.

**Service row — add a visual badge cluster for duration + price:**

Currently duration and price are plain text cells with `tabular-nums text-sm text-muted-foreground`. They are factual but not distinctive — a spa owner scanning the list cannot instantly recognise a service's value proposition.

Spec: wrap duration and price each in a `<Badge variant="outline">` with small padding. This is not new data — it is the same text restyled. The Badge already exists; use the `outline` variant.

```
// Durasi cell
<Badge variant="outline" className="tabular-nums font-normal">
  {s.duration_minutes} mnt
</Badge>

// Harga cell
<Badge variant="outline" className="tabular-nums font-normal">
  {formatPrice(s.price_idr)}
</Badge>
```

Note: shorten `menit` to `mnt` inside the badge to avoid overflow on narrow columns. Full "menit" label is in the form and on detail pages where space is not constrained.

**Category badge color-coding on services:**

Currently all categories use `variant="outline"` — a single neutral gray. If a tenant has 4+ categories, they all look identical. Assign a deterministic color from a small palette based on the category string hash. Use Tailwind's existing color classes — no new tokens. Palette of 5 (cycling):

| Index mod 5 | Tailwind class pair |
|---|---|
| 0 | `bg-emerald-50 text-emerald-700 border-emerald-200` |
| 1 | `bg-sky-50 text-sky-700 border-sky-200` |
| 2 | `bg-violet-50 text-violet-700 border-violet-200` |
| 3 | `bg-amber-50 text-amber-700 border-amber-200` |
| 4 | `bg-rose-50 text-rose-700 border-rose-200` |

Implementation: a pure function `categoryColorClass(category: string): string` that sums char codes mod 5, returns the Tailwind string. Lives in `lib/utils.ts`. Apply to the Badge `className` on the services table. No new component needed.

---

#### Finding 3 — Row density (HIGH)

**Problem.** Shadcn's default `<TableRow>` has `border-b` and relies on the default `<td>` padding from the base stylesheet (`py-4` or similar). The result is generous rows that feel fine for dense data but mildly wasteful when rows only carry 4–5 short tokens.

**Spec.** Add `className="h-14"` to each `<TableRow>` in both pages. `h-14` = 56px, which gives a consistent row height that is:
- Comfortable for 44×44 touch targets on the action button (the ghost icon button is `size-icon` = 40px; 56px row height keeps it within the row with 8px headroom top and bottom).
- Tighter than the default ~64–68px implied by `py-4` on both cells.
- Consistent between therapist rows (which have an avatar) and service rows (text-only), since the avatar is `h-8 w-8` = 32px, well within 56px.

Add `className="text-sm"` to `<TableBody>` as a whole, so all cells inherit the correct body text size without per-cell declarations. Currently `text-sm` is applied individually on some cells and omitted on others (e.g., the `font-medium` name cell has no explicit size).

For the table header row: apply `className="bg-muted/30"` to `<TableHeader>`. The current header has no background — it blends into the card surface, making the column labels less distinct from the first data row. `bg-muted/30` is a near-white tint that adds just enough separation.

---

#### Finding 4 — Row actions: add an explicit "Edit" shortcut button (HIGH)

**Problem.** The `MoreHorizontal` three-dot menu is the only way to navigate to the therapist/service detail. For spa admins who are the primary users (non-technical, operating quickly), hiding the primary action (edit/view) behind a dropdown adds friction. The dropdown already includes a `Pencil` icon in the "Lihat / Edit" item — this signals edit intent is known; it just needs surfacing.

**Spec.** Add a ghost `size="icon"` `<Button>` with the `Pencil` icon directly in the `<TableCell>` (right-aligned), rendered before the `DropdownMenuTrigger`. The "Lihat / Edit" `<DropdownMenuItem>` remains in the dropdown for keyboard and assistive-tech parity, but is now also surfaced as a direct button.

Layout of the actions cell (both pages):
```
<div class="flex items-center justify-end gap-1">
  <Button variant="ghost" size="icon" asChild aria-label="Edit [name]">
    <Link href="/master/therapists/[id]"><Pencil size={14} /></Link>
  </Button>
  <DropdownMenu> ... </DropdownMenu>   // keeps toggle-status + delete
</div>
```

The `aria-label` on the pencil button must include the row entity name ("Edit Siti Rahmawati") so screen reader users can distinguish it from other pencil buttons in the list. Pass `therapist.full_name` / `service.name` as a prop to the row-actions component.

The `DropdownMenuTrigger` button's `aria-label` should similarly change to "Tindakan lainnya untuk [name]" to distinguish it when multiple rows are on screen.

**No change to `TherapistRowActions` or `ServiceRowActions` component file structure** — just add the pencil button inside the existing returned JSX, before the `<DropdownMenu>`. The component receives the entity name prop already (it has `therapist.full_name` via `therapist` prop).

---

#### Finding 5 — Filter bar clarity (HIGH)

**Problem.** The filter bar combines three things in one visual container: the "FILTER" icon+label, the branch pill group (therapists only), and the status dropdown. The icon+label is redundant — `aria-label="Filter daftar"` already communicates the region semantically. Visually, "FILTER" in uppercase small-caps reads as a section header, not a label for the controls, which creates a false hierarchy. The `FilterSelect` native `<select>` changes to emerald background when active — this is good — but the branch pills switch to emerald too, so a user with both a branch filter AND a status filter active sees two emerald blobs without a summary of "2 filters active."

**Spec — five targeted changes:**

1. **Remove the "FILTER" icon+label from `FilterBar`.** The container border + `bg-muted/40` is sufficient visual grouping. Removing the label frees 48–60px of horizontal space for the actual controls and eliminates the false section-header reading. Keep the `role="region"` and `aria-label="Filter daftar"` on the container div.

2. **Add an "active filters" signal to the container itself.** When any filter is active (branch_id or is_active is set), apply `border-primary/40 bg-primary/5` instead of `border-border bg-muted/40`. This is a two-class conditional on `FilterBar` — no new component. Pass an `isActive?: boolean` prop to `FilterBar`; each page computes `const filtersActive = !!(branch_id || is_active || category)` and passes it.

3. **Add an "X Hapus filter" clear-all link** inside `FilterBar` when `isActive` is true. Render as a `<Link href="/master/therapists">` (or services) ghost-styled anchor — `text-xs text-muted-foreground hover:text-foreground flex items-center gap-1`. Use `X` (lucide `X`, size 12) + label "Hapus filter". Position it at the far right of the flex row (`ml-auto`). This gives non-technical users a single obvious escape hatch from any filtered state.

4. **`FilterSelect` — add a visual indicator for active state beyond color alone.** Currently an active select shows emerald bg — good for sighted users, fails "don't rely on color alone" (WCAG 1.4.1). Add a `CheckCircle2` icon (lucide, size 11) prepended inside the select label text when `isActive`. Since `<select>` cannot contain child elements beyond `<option>`, render the icon *before* the `<select>` in the `<label>`, inside a `<span aria-hidden="true">`. The icon + color together satisfy WCAG 1.4.1.

5. **Branch pills — separate the "Cabang:" label from the pill group.** Currently `<span class="text-sm text-muted-foreground">Cabang:</span>` is inside the same `flex-wrap` row as the pills, causing it to sometimes orphan to a new line on narrow viewports. Wrap the entire branch group in a `<div class="flex items-center gap-1.5">` that does not wrap internally, then let the outer FilterBar flex container wrap that whole block. This keeps "Cabang: [Semua] [Kemang] [Sudirman]" on one logical line.

---

#### Finding 6 — Empty states (MEDIUM)

**Problem.** The `Inbox` icon (lucide) is a generic "empty mailbox" — it carries no spa/wellness connotation and is the same icon used across unrelated contexts in many apps. The `text-muted-foreground/40` opacity makes it very faint — borderline invisible on some displays.

**Spec — two changes:**

1. **Replace `Inbox` with context-appropriate icons:**
   - Therapist list: use `UserRound` (already imported in the file, used for avatar fallback). When the list is empty it is thematically consistent — "no people yet."
   - Service list: use `Sparkles` (already imported in `master-sub-nav.tsx`). A spa service catalog that is empty should evoke potential, not a mailbox.
   - Both: keep `size={40}`, change opacity from `text-muted-foreground/40` to `text-muted-foreground/50` — +10% opacity, still clearly decorative but not invisible on all displays.

2. **Add a short "why" line above the CTA on the unfiltered zero-data empty state.** Currently: icon → one-line message → CTA. Make it: icon → headline → supporting line → CTA.
   - Therapist list unfiltered: headline "Belum ada terapis", supporting "Tambahkan terapis pertama untuk melihat jadwal dan layanan mereka."
   - Service list unfiltered: headline "Belum ada layanan", supporting "Tambahkan layanan yang tersedia di spa Anda."
   - The supporting line uses `text-xs text-muted-foreground` and `max-w-xs mx-auto` to constrain the measure.

   For filter-active empty states (e.g. "Tidak ada terapis aktif saat ini."), keep the single-line copy — these are informational, not onboarding moments.

---

#### Finding 7 — Mobile / narrow responsiveness (MEDIUM)

The design system spec (§2.1) already mandates card-per-row at < 640px. The current implementation does not implement this — it renders the table at all viewport widths. This is the existing debt, not a new finding, but it becomes more painful with the row-actions changes above (two buttons side-by-side in a 36px-wide cell on a 375px screen is a guaranteed overflow).

**Spec for the table at < 640px:**

Add a wrapping strategy to both pages rather than a full card refactor (to minimise engineering cost):

1. On the outer `<Card>`, add `className="overflow-x-auto"`. This makes the table horizontally scrollable on small screens — a lower-cost, lower-risk fix than a full card-row rewrite, and the correct pattern for data tables on mobile.
2. On `<Table>`, add `className="min-w-[560px]"` (therapists, with branch column) or `min-w-[480px]` (services). This prevents the table from collapsing column widths below readable thresholds.
3. The filter bar's `flex-wrap` already handles overflow — no change needed there.
4. The page header `flex items-center justify-between` should collapse to `flex-col gap-3` at < 380px. Add `sm:flex-row flex-col gap-3` to the header div class, and `w-full sm:w-auto` to the CTA button so it does not clip.

This is a "good enough for Phase 4" fix. The full card-per-row mobile layout (as originally specced in §2.1) is still the recommended target for Phase 5 when mobile usage is validated.

---

#### Finding 8 — Microcopy polish (MEDIUM)

| Location | Current | Recommended | Reason |
|---|---|---|---|
| FilterBar label (both pages) | "FILTER" (uppercase) | remove entirely | Redundant; see Finding 5 |
| FilterSelect label prefix | "Status:" | "Status" (no colon, space only) | The colon is grammatically unnecessary before a dropdown; removing it reads more natural in Indonesian |
| FilterSelect label prefix | "Kategori:" | "Kategori" (no colon) | Same reason |
| Table header — therapist | "Bergabung Sejak" | "Bergabung" | On narrow columns "Bergabung Sejak" wraps; "Bergabung" is sufficient — the date format makes the meaning clear. On detail page, full label is fine. |
| Table header — services | "Durasi" | "Durasi" | Fine as-is |
| Table header — services | "Harga" | "Harga" | Fine as-is |
| Services empty state | "Tambahkan layanan pertama untuk memulai." | "Tambahkan layanan yang tersedia di spa Anda." | More descriptive of _what kind_ of thing to add — less generic |
| Row actions dropdown | "Lihat / Edit" | "Buka Detail" | "Lihat / Edit" is a compound action that signals ambiguity. "Buka Detail" is a navigation action — single, clear. Edit happens _inside_ the detail page. |
| Therapist row actions | aria-label "Menu tindakan terapis" | "Tindakan lainnya untuk [full_name]" | Must be unique per row (WCAG 2.4.6) |
| Error state (therapists) | "Gagal memuat data terapis. Coba lagi." | "Gagal memuat daftar terapis. Muat ulang halaman." | "Coba lagi" without a button is a dead instruction — the user has no way to retry without a full page reload. Either add a retry button next to the message, or change copy to direct the user to the correct action. If no retry button: use "Muat ulang halaman." If a retry button is added: keep "Coba lagi." |

---

#### Finding 9 — Change triage

| Priority | Change | Engineering effort | File(s) |
|---|---|---|---|
| CRITICAL | `h1` upgrade to `sm:text-2xl`, merge avatar+name into one cell | Trivial — 2 className edits | `therapists/page.tsx` |
| CRITICAL | Header `flex-col gap-3 sm:flex-row` at narrow viewport + `w-full sm:w-auto` on CTA | Trivial | both `page.tsx` |
| HIGH | Remove "FILTER" icon+label from `FilterBar`; add `isActive` prop + border/bg signal | Minor — `filter-bar.tsx`, 2 prop passes | `filter-bar.tsx`, both `page.tsx` |
| HIGH | Add "X Hapus filter" clear-all link in `FilterBar` | Minor | `filter-bar.tsx` |
| HIGH | Add `CheckCircle2` icon to `FilterSelect` active state | Trivial | `filter-select.tsx` |
| HIGH | Expose pencil `<Button>` before dropdown in row actions | Minor — `therapist-row-actions.tsx` + `service-row-actions.tsx`; update aria-labels | both row-actions files |
| HIGH | `<TableHeader>` gets `bg-muted/30`; `<TableRow>` gets `h-14`; `<TableBody>` gets `text-sm`; remove per-cell `text-sm` repetition | Trivial | both `page.tsx` |
| HIGH | `<Card>` gets `overflow-x-auto`; `<Table>` gets `min-w-[560px]` / `min-w-[480px]` | Trivial | both `page.tsx` |
| MEDIUM | Category badge color-coding (deterministic hash function) | Minor — add `categoryColorClass` to `lib/utils.ts`, update badge className | `lib/utils.ts`, `services/page.tsx` |
| MEDIUM | Merge duration + price into `<Badge variant="outline">` on services table | Trivial | `services/page.tsx` |
| MEDIUM | Branch pill group wrapped in non-wrapping inner div | Trivial | `therapists/page.tsx` |
| MEDIUM | Empty state icon swap (`UserRound`, `Sparkles`), opacity `50`, add supporting line | Trivial | both `page.tsx` |
| MEDIUM | "Aksi" `<TableHead>` remove `text-right` | 1-character edit | both `page.tsx` |
| LOW | Microcopy: "Lihat / Edit" → "Buka Detail" | Trivial | both row-actions files |
| LOW | Microcopy: remove colons from FilterSelect labels | Trivial | both `page.tsx` call-sites |
| LOW | Microcopy: "Bergabung Sejak" → "Bergabung" in table header | Trivial | `therapists/page.tsx` |
| LOW | Error state copy update | Trivial | both `page.tsx` |

**Refactor boundary:** nothing above requires a new component or a component extraction. All changes are confined to existing files. The `categoryColorClass` utility is the only new function; it belongs in `lib/utils.ts` alongside existing helpers. The `FilterBar` and `FilterSelect` changes are additive (new optional prop + conditional render) and are backward-compatible — all existing call sites that omit `isActive` continue to render the unfiltered style.

---

## Add-on catalog (ADR 0010)

_Owned by `ui-ux-expert`. Implemented by `nextjs-expert`. Backend contract in ADR 0010 §4.2._

_Scope: tenant-admin portal only. No ops-portal or platform-admin surface in Phase 4._

_Replaces the superseded "Per-service add-on editor" design. Add-ons are now a tenant-wide flat catalog — not embedded inside the service detail. See ADR 0010 for rationale._

---

### ADO-1. Nav placement

**Sidebar entry:** add a "Tambahan" item to the existing "Master Data" / "Operasional" nav section, alongside "Terapis" and "Layanan".

**Icon:** `PackagePlus` (lucide-react, already in the icon set). Rationale: conveys "add an optional extra / packaged item" more precisely than the alternatives. `Sparkles` is already reserved for the service-list empty state (Finding 6 of the list-page UX review). `LayoutList` is too generic. `PackagePlus` is unused elsewhere in the tenant-admin portal.

**Label:** "Tambahan" (Indonesian-first). Do not use "Add-on" — the UI is Indonesian throughout.

**Active route match:** `/master/addons` and all sub-routes (`/master/addons/new`, `/master/addons/[id]`).

**`NavKey` extension:** add `"tambahan"` to the `NavKey` union if the nav system type-guards active keys. The `AppHeader activeNav` prop on the shared `app/master/layout.tsx` does not need a new value — the addons pages share the existing `"operasional"` active nav state (same section as therapists and services).

---

### ADO-2. List page — `/master/addons`

**Purpose:** browse, search, and manage the tenant-wide add-on catalog.

**Layout (mirrors `/master/services` exactly):**

```
[Page header row]  "Tambahan"  [subtitle]              [+ Tambah Add-on]
[Filter bar]       Status toggle (Semua / Aktif / Nonaktif)
[Table / empty state]
[Pagination]
```

**Page header:**

- `<h1 className="text-xl font-semibold text-foreground sm:text-2xl">Tambahan</h1>`
- Subtitle: `<p className="mt-1 text-sm text-muted-foreground">Kelola katalog add-on untuk semua layanan Anda</p>`
- Header div: `className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"` (same narrow-viewport collapse pattern as therapists/services).
- CTA: `<Button asChild className="w-full sm:w-auto"><Link href="/master/addons/new"><Plus size={16} aria-hidden="true" />Tambah Add-on</Link></Button>`

**Table — `<Card><CardContent className="overflow-x-auto p-0">`:**

`<Table className="min-w-[480px]">` — same minimum width as the services table (5 columns, similar density).

| # | `<TableHead>` | Width / align | Cell content |
|---|---|---|---|
| 1 | Nama | `min-w-[200px]`, left | `addon.name` — `font-medium`. When `is_active = false`: additionally `line-through text-muted-foreground` (non-color inactive signal, WCAG 1.4.1). |
| 2 | Harga | `w-[130px]`, left | `<Badge variant="outline" className="tabular-nums font-normal">{formatPrice(addon.price_idr)}</Badge>` — matches service price badge style. |
| 3 | Status | `w-[90px]`, left | `<Badge variant={addon.is_active ? "success" : "muted"}>{addon.is_active ? "Aktif" : "Nonaktif"}</Badge>` |
| 4 | Urutan | `w-[96px]`, center | Up / down reorder controls — see ADO-5. Header label "Urutan". |
| 5 | Aksi | `w-[80px]`, left (header) / `flex justify-end` (cell) | Pencil edit button + three-dot dropdown. Same layout as services/therapists row actions. |

`<TableHeader className="bg-muted/30">` — consistent with other list pages.
`<TableRow className="h-14">` — 56px row height, consistent with services and therapists.
`<TableBody className="text-sm">` — inherits body text size for all cells.

**No "Aksi" header `text-right`** — per Finding 1 of the list-page UX review; header is left-aligned, cell content is `flex justify-end`.

---

### ADO-3. Pagination

Cursor-based, page size **10** (Lustia list convention, per `feedback_list_pagination` memory). Uses the existing `<Pagination>` component with the standard props interface:

```
<Pagination
  pathname="/master/addons"
  searchParams={{ is_active }}
  nextCursor={nextCursor}
  hasPrev={!!cursor}
  currentPageCount={addons.length}
  pageSize={PAGE_SIZE}
/>
```

`PAGE_SIZE = 10` constant at the top of the page file, matching the services and therapists pages.

The `<Pagination>` component renders `null` when `!hasPrev && !nextCursor`, so it does not clutter the page for tenants with fewer than 10 add-ons.

---

### ADO-4. Filter bar

**Spec:**

```
<FilterBar isActive={!!is_active} resetHref="/master/addons">
  <FilterSelect
    label="Status"
    name="is_active"
    current={is_active}
    options={[
      { value: "", label: "Semua" },
      { value: "true", label: "Aktif" },
      { value: "false", label: "Nonaktif" },
    ]}
  />
</FilterBar>
```

- `isActive` tints the bar (`border-primary/40 bg-primary/5`) and shows "× Hapus filter" link when any filter is active — the existing `FilterBar` behavior.
- `FilterSelect` shows `CheckCircle2` icon when active — existing `FilterSelect` behavior (WCAG 1.4.1 non-color cue).
- No branch filter — add-ons are tenant-scoped (no `branch_id` column on `addon`). No category filter in Phase 4. If categories are introduced in a future phase, a `FilterSelect` for category slots in here as a sibling.

---

### ADO-5. Reorder UX

**Controls:** `ChevronUp` and `ChevronDown` ghost icon buttons in the "Urutan" column, one pair per row.

**Cell layout (Urutan column):**

```
<div class="flex items-center justify-center gap-0.5">
  <Button variant="ghost" size="icon" className="h-7 w-7"
          aria-label="Pindah ke atas: [addon.name]"
          disabled={isFirst || isReordering}>
    <ChevronUp size={14} />
  </Button>
  <Button variant="ghost" size="icon" className="h-7 w-7"
          aria-label="Pindah ke bawah: [addon.name]"
          disabled={isLast || isReordering}>
    <ChevronDown size={14} />
  </Button>
</div>
```

**Disabled boundaries:**
- `isFirst`: ChevronUp disabled. The top-most item on the current page cannot move up.
- `isLast`: ChevronDown disabled. The bottom-most item on the current page cannot move down.
- `isReordering`: both buttons on all rows disabled while any reorder API call is in flight. Prevents conflicting concurrent requests.

Do not hide disabled buttons — their absence causes cell-width jitter. Use `disabled` prop, which maps to `opacity-50` and `aria-disabled="true"` via shadcn `Button`.

**Interaction model — optimistic update with server sync:**

1. User clicks ChevronUp or ChevronDown on row N.
2. Client immediately swaps positions between row N and row N±1 in local React state and re-renders the table. The user sees the change instantly.
3. Client calls `PUT /api/v1/tenant/addons/reorder` with the full current-page sorted array: `{"items": [{"id": "...", "sort_order": 0}, {"id": "...", "sort_order": 1}, ...]}`. `sort_order` values are reassigned as 0-based sequential integers matching the new order.
4. On success: no further action (server state matches optimistic state).
5. On failure: rollback local state to pre-click order. Toast: `toast.error("Gagal mengubah urutan. Coba lagi.")`. Re-enable buttons.

**Reorder scope — pagination boundary:**

Reorder applies only to the items visible on the current page. The `sort_order` values sent in the bulk PUT are scoped to that page's items. The backend handles the case where `sort_order` values from different pages might collide by treating the PUT as an authoritative set for the submitted IDs — existing items not in the payload are not moved. This is safe because: (a) the tenant is unlikely to reorder across page boundaries deliberately; (b) the backend's sort is `ORDER BY sort_order ASC` — the relative order of the submitted items is what matters, not the absolute integers. If a cross-page collision occurs after multiple reorder sessions, the backend normalises `sort_order` on its next list scan (or the next explicit PUT). The UI does not need to handle this — it shows what the API returns.

**No animation on row swap.** The table re-renders with the new order instantly. Row-slide animations would require `framer-motion` or `@dnd-kit/sortable` — excluded from Phase 4. Instant swap satisfies `prefers-reduced-motion` trivially.

---

### ADO-6. Create page — `/master/addons/new`

**Layout:** same grammar as `/master/services/new` and `/master/therapists/new`.

```
[Back link]  "← Kembali ke Daftar Add-on"
[Page header]  "Tambah Add-on"  /  subtitle "Isi detail add-on baru"
[Single-column form card]
  [CardContent className="p-6"]
    [form fields — see below]
    [button row: Batal | Simpan & Buat Add-on]
```

**Back link:** `<Link href="/master/addons" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"><ArrowLeft size={14} aria-hidden="true" />Kembali ke Daftar Add-on</Link>`

**Field order and spec:**

| # | Field | Input | Label | Constraint | Note |
|---|---|---|---|---|---|
| 1 | `name` | `Input` | Nama Add-on | Required. 1–120 chars. | `autoFocus`. Placeholder: "Aromaterapi". Validation msg: "Nama add-on wajib diisi." / "Nama maksimal 120 karakter." |
| 2 | `description` | `Textarea` (native, same pattern as service-form) | Deskripsi | Optional. max 500 chars. | `rows={3}`. Placeholder: "Keterangan singkat yang ditampilkan kepada pelanggan." Validation msg: "Deskripsi maksimal 500 karakter." |
| 3 | `price_idr` | `Input type="number"` | Harga | Required. Integer. min 0. | Prefix "Rp" as sibling `<span>` (same as service-form Harga). Placeholder: "0". `min={0}` `step={1}`. Validation msg: "Harga wajib diisi." / "Harga harus bilangan bulat." / "Harga tidak boleh negatif." |
| 4 | `is_active` | Switch (shadcn) | Status | Boolean | Label: "Aktif". Default `true`. `<div className="flex items-center justify-between">` wrapper. |

`sort_order` is not a form field. The server appends new add-ons at the end (`sort_order = current_max + 1`).

**Zod schema:**

```
name:        z.string().min(1, "Nama add-on wajib diisi.").max(120, "Nama maksimal 120 karakter.")
description: z.string().max(500, "Deskripsi maksimal 500 karakter.").optional()
price_idr:   z.coerce.number({ invalid_type_error: "Harga harus berupa angka." })
               .int("Harga harus bilangan bulat.")
               .min(0, "Harga tidak boleh negatif.")
is_active:   z.boolean()
```

**Inline validation:** `<FormMessage>` beneath each field on blur and on submit attempt. Matches service-form and therapist-form pattern.

**Button row:**

| Action | Label | Variant | Position |
|---|---|---|---|
| Cancel | Batal | ghost | Left |
| Submit | Simpan & Buat Add-on | primary | Right |

Submit: disabled while `isPending`; shows `<Loader2 className="animate-spin" size={14} />` in loading state.

"Batal" navigates to `/master/addons` — does NOT call `window.history.back()`.

**Post-save redirect:** navigate to `/master/addons/[newId]`. Toast: `"Add-on berhasil ditambahkan."` (success).

**Duplicate name error (409):** set `form.setError("name", { message: "Nama add-on sudah digunakan." })` — field-level, not toast.

---

### ADO-7. Edit page — `/master/addons/[id]`

**Layout:** same as create page but pre-populated. Page header: "Detail Add-on" / subtitle is the add-on name. No tabs — the add-on record is a simple flat object; tabs are not warranted.

```
[Back link]  "← Kembali ke Daftar Add-on"
[Page header row]
  [h1 "Detail Add-on"]  [subtitle: addon.name]  [Status Badge — right slot]
[Form card — identical fields to /new]
[Destructive actions — below the card]
```

**Status badge in header:** `<Badge variant={addon.is_active ? "success" : "muted"}>` — visible at a glance before the form loads, matching the service detail page pattern.

**Inactive record indicator on list row (ADO-2):** `line-through text-muted-foreground` on the Nama cell. This is the only list-level indicator — no row background change, no opacity reduction (opacity reduction degrades reorder button contrast).

**Save button:** "Simpan Perubahan" (primary). On success: stay on the page, toast `"Perubahan berhasil disimpan."` (success).

**Destructive actions section (below form card):**

Two secondary buttons in a right-aligned `<div className="flex justify-end gap-2 mt-4">`:

1. **Nonaktifkan / Aktifkan** — context-sensitive. When `is_active = true`: show "Nonaktifkan" (outline warning tone). When `is_active = false`: show "Aktifkan" (outline success tone). No confirmation dialog — toggle is reversible. Optimistic: flip badge and button label immediately, call `PATCH /addons/:id/status`, rollback on failure. Toast on success: `"Add-on diaktifkan."` or `"Add-on dinonaktifkan."`.

2. **Hapus** — outline destructive variant. Opens an `AlertDialog` confirmation before calling `DELETE /addons/:id`. On success: navigate to `/master/addons`, toast `"Add-on berhasil dihapus."`. On failure: toast `"Gagal menghapus add-on. Coba lagi."`.

---

### ADO-8. Activate / deactivate + soft-delete

**Activate / deactivate:**
- Available from both the list page (row dropdown menu) and the edit page (button below form).
- No confirmation dialog — toggle is reversible.
- Optimistic flip + `PATCH /api/v1/tenant/addons/:id/status` with body `{"is_active": true|false}`.
- Rollback on failure with toast.

**Dropdown menu on list row (Aksi cell):**

```
<div class="flex items-center justify-end gap-1">
  <Button variant="ghost" size="icon" asChild aria-label="Edit add-on: [addon.name]">
    <Link href="/master/addons/[id]"><Pencil size={14} /></Link>
  </Button>
  <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button variant="ghost" size="icon"
              aria-label="Tindakan lainnya untuk add-on: [addon.name]">
        <MoreHorizontal size={14} />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem>
        {addon.is_active
          ? <><EyeOff size={14} /> Nonaktifkan</>
          : <><Eye size={14} /> Aktifkan</>}
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem className="text-destructive">
        <Trash2 size={14} /> Hapus
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</div>
```

**Soft-delete confirmation dialog (AlertDialog):**

- Title: "Hapus Add-on?"
- Body: `"Add-on \"[addon.name]\" akan dihapus secara permanen dan tidak dapat dipulihkan."`
- Confirm: "Ya, Hapus" (destructive Button variant)
- Cancel: "Batal"
- Focus default on open: "Batal" (Cancel must be first focusable element in `AlertDialogFooter` — Radix default places focus on first button; render Cancel before Hapus in DOM order, visually reversed by `DialogFooter` flex-direction).

---

### ADO-9. Empty state

Shown when the API returns an empty `data` array for the unfiltered list.

**Layout (inside `CardContent`, vertically centered, `py-16`):**

```
[PackagePlus icon, size 40, text-muted-foreground/50, aria-hidden="true"]
[Headline]   "Belum ada tambahan"   — text-sm font-medium text-muted-foreground
[Supporting] "Buat daftar add-on untuk ditawarkan ke pelanggan Anda."
             — text-xs text-muted-foreground/80 max-w-xs mx-auto
[CTA]        "Tambah Add-on"        — Button size="sm" asChild Link href="/master/addons/new"
```

**Filter-active empty states (no CTA, single-line copy):**

| Filter state | Copy |
|---|---|
| `is_active=true` | "Tidak ada add-on aktif saat ini." |
| `is_active=false` | "Tidak ada add-on nonaktif." |

These filter-active states do not include the supporting line or CTA — they are informational, not onboarding moments.

---

### ADO-10. Accessibility

**Reorder buttons:**
- `aria-label` contains direction and add-on name: `"Pindah ke atas: Aromaterapi"`, `"Pindah ke bawah: Aromaterapi"`. Unique per row and per direction.
- When disabled, `aria-disabled="true"` is set automatically by shadcn `Button` via the `disabled` prop.
- Tab order within the Urutan cell: ChevronUp before ChevronDown in DOM order — matches visual order.

**Filter bar:**
- `role="region"` and `aria-label="Filter daftar"` on the `FilterBar` container (already implemented in the existing component).
- `FilterSelect` has `aria-label={label}` on the `<select>` element.
- Active state communicated by both color tint and `CheckCircle2` icon — satisfies WCAG 1.4.1 (not color alone).

**Status badges:**
- "Aktif" / "Nonaktif" text inside the badge is the primary semantic signal. The badge color is a reinforcing secondary signal. Text alone is unambiguous — satisfies WCAG 1.4.1.
- Inactive row additionally shows `line-through` on the Nama cell text — dual non-color signal.

**Edit (pencil) button:**
- `aria-label="Edit add-on: [addon.name]"` — unique per row, names the subject.

**Three-dot menu trigger:**
- `aria-label="Tindakan lainnya untuk add-on: [addon.name]"` — unique per row (WCAG 2.4.6).

**Page header CTA:**
- `<Button asChild>` wrapping a `<Link>` — renders as a semantic `<a>`, navigates correctly with keyboard Enter.

**Form fields:**
- Each field: `<FormLabel>` associated with its `<Input>` / `<Textarea>` / `<Switch>` via `htmlFor` / `id` (RHF-wired pattern already used in service-form and therapist-form).
- Error messages: `<FormMessage>` renders with `role="alert"` when present — screen readers announce field-level errors on submit.

**AlertDialog (delete confirmation):**
- Focus lands on "Batal" (Cancel) by default — Cancel is the first focusable element in `AlertDialogFooter`.
- Escape closes without deleting (Radix default).
- Focus returns to the trigger (the Hapus `DropdownMenuItem`) on close (Radix default).

**Reduced motion:**
- Optimistic row reorder does not animate. Instant swap, no transition. `prefers-reduced-motion` satisfied trivially.
- `FilterBar` color transition uses `transition-colors` (CSS property transition, not JS animation) — respects reduced-motion because CSS `transition` is suppressed by `prefers-reduced-motion: reduce` in Tailwind's `@media (prefers-reduced-motion: reduce)` base layer.

---

### ADO-11. Component reuse inventory

No new shadcn/ui components. All primitives are already vendored.

| Component | Where used |
|---|---|
| `Card`, `CardContent` | Outer wrapper for list table and form |
| `Table`, `TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell` | Add-on list table |
| `Badge` | Harga (outline), Status (success/muted), page-header status indicator on edit page |
| `Button` | CTA, reorder controls, row actions (pencil, more), form submit/cancel, AlertDialog confirm/cancel |
| `DropdownMenu`, `DropdownMenuTrigger`, `DropdownMenuContent`, `DropdownMenuItem`, `DropdownMenuSeparator` | Row actions |
| `AlertDialog`, `AlertDialogContent`, `AlertDialogHeader`, `AlertDialogTitle`, `AlertDialogDescription`, `AlertDialogFooter`, `AlertDialogAction`, `AlertDialogCancel` | Delete confirmation |
| `Form`, `FormField`, `FormItem`, `FormLabel`, `FormControl`, `FormMessage` | Create / edit form |
| `Input` | `name` and `price_idr` fields |
| `Switch` | `is_active` field |
| `Pagination` | List page pagination (`components/pagination.tsx`) |
| `FilterBar` | List page filter container (`components/filter-bar.tsx`) |
| `FilterSelect` | Status filter (`components/filter-select.tsx`) |

Native `<textarea>` (raw element + class string, same pattern as `service-form.tsx` / `therapist-form.tsx`) for the `description` field — not a new shadcn Textarea. Consistent with the rest of the app until Textarea is formally vendored.

**Lucide icons required (may need import on new files):**
- `PackagePlus` — nav icon, list empty state icon
- `ChevronUp`, `ChevronDown` — reorder controls
- `Eye`, `EyeOff` — activate/deactivate dropdown items
- `Trash2` — delete dropdown item
- `ArrowLeft` — back link
- `Plus` — page header CTA
- `Pencil` — row edit button
- `MoreHorizontal` — row three-dot trigger
- `AlertCircle` — error state inline alert
- `Loader2` — form submit loading spinner

---

### ADO-12. Microcopy additions

**Page titles / subtitles:**

| Screen | `<h1>` | Subtitle |
|---|---|---|
| Add-on list | Tambahan | Kelola katalog add-on untuk semua layanan Anda |
| Add-on new | Tambah Add-on | Isi detail add-on baru |
| Add-on detail | Detail Add-on | (add-on name as secondary label below h1) |

**CTA buttons (append to global table in §4):**

| Action | Label | Variant |
|---|---|---|
| Create add-on (list CTA) | Tambah Add-on | primary |
| Submit create form | Simpan & Buat Add-on | primary |
| Save edit form | Simpan Perubahan | primary |
| Deactivate add-on | Nonaktifkan | outline (warning tone) |
| Activate add-on | Aktifkan | outline (success tone) |
| Delete add-on (edit page) | Hapus | outline destructive |
| Confirm delete | Ya, Hapus | AlertDialogAction (destructive) |
| Cancel | Batal | ghost |

**Toasts (append to global table in §4):**

| Event | Message | Variant |
|---|---|---|
| Add-on created | "Add-on berhasil ditambahkan." | success |
| Add-on updated | "Perubahan berhasil disimpan." | success |
| Add-on deleted | "Add-on berhasil dihapus." | success |
| Add-on activated | "Add-on diaktifkan." | success |
| Add-on deactivated | "Add-on dinonaktifkan." | success |
| Reorder failed | "Gagal mengubah urutan. Coba lagi." | destructive |
| Delete failed | "Gagal menghapus add-on. Coba lagi." | destructive |
| Any mutation failure | "Gagal menyimpan. Coba lagi." | destructive |
| Load failure (list) | "Gagal memuat data. Muat ulang halaman." | (shown inline, not toast) |

**Confirmation dialog — delete add-on:**

- Title: "Hapus Add-on?"
- Body: `"Add-on \"[addon.name]\" akan dihapus secara permanen dan tidak dapat dipulihkan."`
- Confirm: "Ya, Hapus" (destructive)
- Cancel: "Batal"

---

### ADO-13. Open questions

1. **Delete recovery:** soft-delete at the DB layer (`deleted_at IS NOT NULL`) but no UI to recover. If operators need undo, a recovery screen is addable. Confirm this is acceptable for Phase 4.
2. **Reorder across pagination pages:** the current spec reorders only within the visible page. If an operator needs to promote an add-on from page 2 to the top of page 1, they must navigate pages manually. This is an edge case at projected catalog sizes (< 30 add-ons). Confirm this limitation is acceptable, or specify a drag-and-drop or explicit "move to top" shortcut for Phase 4.

---

## Therapist extended profile (ADR 0011)

_Owned by `ui-ux-expert`. Implemented by `nextjs-expert`. Backend contract in ADR 0011 §2.3–2.4._

_Phase 4 scope: admin-side form, list avatar, detail summary, placeholder banner. Customer-facing rendering is Phase 5._

_ADR contract summary: `photo_key` stored as opaque storage key (never a URL); URL resolved at controller boundary. `height_cm` 100–250, `weight_kg` 30–250, `build` in `{langsing, sedang, atletis, tegap}`. Upload endpoint: `POST /api/v1/tenant/therapists/:id/photo` — immediate on file-pick, separate from the profile PATCH._

---

### TEP-1: Admin form layout

#### Where photo picker sits

The photo picker occupies a **dedicated row at the very top of the Profil tab form**, spanning full column width, before all text fields. Rationale: the photo is the first and most visually prominent thing a customer will see in the booking UI. Placing it at the top of the admin form mirrors that priority and keeps it from being buried after several inputs. It is not placed inline beside the Nama/Gender grid because the circular preview (128×128) would create a mismatched row height with the text inputs beside it — awkward on mobile.

**Form skeleton (Profil tab, in order):**

```
[Photo picker — full width, see TEP-2]

[Nama Lengkap — full width, required]

[No. Telepon | Email] — 2-column on sm+, stacked on mobile

[Jenis Kelamin | Bergabung Sejak] — 2-column on sm+, stacked on mobile

[Bio — full width, textarea]

── Informasi Postur ──────────────────────────────── (section label)
[Tinggi (cm) | Berat (kg) | Postur] — 3-column on sm+, stacked on mobile

[Akun terhubung — read-only, edit mode only]

[Simpan Perubahan | Batal]
```

#### Body fields grouping

Height, weight, and build are grouped under a labelled subsection `"Informasi Postur"` — a `<p className="text-xs font-medium uppercase tracking-wide text-muted-foreground mt-6 mb-3">` divider label, no horizontal rule. This separates them visually from personal contact fields without adding a Card or collapsible. Three fields in one row on `sm:grid-cols-3` — they are short numeric inputs plus one Select, so the three-column grid is comfortable at ≥ 640px. Below 640px they stack to single column.

The subsection label copy is "Informasi Postur" (not "Fisik" to avoid sensitivity around body metrics; "Postur" is neutral and already the field name used in the customer-facing context).

#### Required-field indicators

Same pattern as the existing form: `<span className="text-destructive">*</span>` appended to the `<FormLabel>` text for mandatory fields. Mandatory fields in the extended form: `full_name`, `height_cm`, `weight_kg`, `build` (per ADR 0011 §2.3.2). Photo is NOT marked required — existing therapists may have no photo.

---

### TEP-2: Photo picker UX

#### Upload interaction: single button, no drag-and-drop

**Decision: single "Unggah Foto" button triggering a hidden `<input type="file">`. No drag-and-drop zone.**

Rationale: the form is used on a desktop browser by non-technical spa owners. Drag-and-drop is discoverable only to users who already know it exists; a labeled button is immediately obvious. Adding a drag-and-drop zone alongside the button would require a larger hit area that competes visually with the form layout. The form is primarily used on laptop/desktop (admin portal); mobile form usage is secondary. A single clear CTA with a label is the most accessible and lowest-complexity path. If Phase 5 analytics show mobile upload demand, drag-and-drop can be layered on without changing the existing button path.

#### Client-side pre-validation (before any network request)

Run synchronously in the `change` handler of the hidden file input, before calling the upload API:

1. **Type check:** `file.type` must be one of `image/jpeg`, `image/png`, `image/webp`. If not: show inline error "Format tidak didukung (JPEG, PNG, WebP)." — do NOT initiate the upload.
2. **Size check:** `file.size > 5 * 1024 * 1024` → inline error "Ukuran melebihi 5 MB." — do NOT initiate the upload.

Both errors clear on the next file selection attempt.

#### Photo preview — circular, 128×128

The preview area is always visible (even when no photo exists), occupying a fixed 128×128 region:

- **No photo yet (null `photo_key`):** a muted circle with initials (same `TherapistAvatar` pattern already used in the list — `bg-primary/10 text-primary`, initials from `full_name`). If the form is on a "new therapist" page where `full_name` may not yet be set, show a `UserRound` icon fallback. Size: `h-32 w-32 rounded-full`.
- **Photo present:** `<img>` tag with `src={resolvedPhotoUrl}` and `alt="Foto {therapist.full_name}"` (required, non-empty alt text — see TEP-9). Same `h-32 w-32 rounded-full object-cover` sizing.

The preview sits to the **left** of the action buttons on `sm+` viewports, stacked above on mobile:

```
[sm+ layout]
<div class="flex items-center gap-4">
  <img/initials circle (128×128)>
  <div class="flex flex-col gap-2">
    <Button "Unggah Foto" or "Ganti Foto">
    <Button "Hapus Foto" (destructive ghost, only when photo_key is set)>
    <p class="text-xs text-muted-foreground">JPEG, PNG, WebP. Maks. 5 MB.</p>
  </div>
</div>

[mobile: flex-col, preview centered, buttons below]
```

The hidden `<input type="file" accept="image/jpeg,image/png,image/webp">` is programmatically triggered by the Button click (`inputRef.current?.click()`). The file input itself has a `<label>` associated via `htmlFor` for screen-reader semantics, even though it is visually hidden.

#### Upload progress

When a file passes client-side validation, the upload begins immediately (`POST /api/v1/tenant/therapists/:id/photo`). During the in-flight request:

- The "Unggah Foto" / "Ganti Foto" button is replaced by an inline spinner (`<Loader2 className="animate-spin" size={14} aria-hidden="true" />`) alongside the text "Foto sedang diunggah…" — the same `isPending` + `Loader2` pattern used on all form submit buttons in this app. Button is disabled.
- "Hapus Foto" button is also disabled.
- The preview remains visible and unchanged during upload (the current photo, or the initials placeholder if first upload).

**Decision: inline spinner, not a progress bar.** The upload is a single small image (≤ 5 MB). A progress bar requires `XMLHttpRequest` with `onprogress`, which conflicts with the `fetch`-based `apiFetch` helper and adds meaningful complexity for a visual gain that is not worth it at ≤ 5 MB transfer sizes (typical upload time on a reasonable connection: < 2 s). A spinner communicates "this is happening" adequately.

#### Network failure

On upload failure, the `Loader2` reverts to the button label, and an inline error message appears below the button group:

`<p role="alert" className="text-sm text-destructive">Gagal mengunggah. Coba lagi.</p>`

The error clears when the user selects a new file (next `change` event). The previous photo (or placeholder) remains displayed — no visual regression.

On success: the API returns the updated therapist DTO with the resolved `photo_url`. The preview `src` is updated to the new URL. The form stores `photo_key` in a hidden local ref (not part of the RHF form submit payload — photo_key is set server-side via the upload endpoint, not via the profile PATCH).

#### Partial-save communication

**The critical UX edge case from ADR 0011 constraints:** upload fires on file-pick (photo is persisted immediately); other profile fields are saved only on "Simpan Perubahan". A user who picks a photo and closes the tab without saving other fields will have the photo saved but not the height/weight/build changes.

**Decision: show a persistent soft-amber informational note below the photo controls when a photo has just been uploaded but the form has not yet been saved:**

```
<p role="status" className="text-xs text-amber-700 bg-amber-50 border border-amber-200
   rounded-md px-3 py-2 mt-2">
  Foto tersimpan. Simpan formulir untuk menerapkan perubahan lainnya.
</p>
```

This note is **only shown in the interval between upload-success and form-submit** (controlled by a `photoJustUploaded` boolean state). It disappears when the user submits the form or navigates away. This makes the two-step nature of the save transparent without blocking the workflow. The note uses `role="status"` (live region, polite) so screen readers announce it after the upload completes.

#### Complete copy inventory

| Context | Copy |
|---|---|
| Button — no photo | "Unggah Foto" |
| Button — photo exists | "Ganti Foto" |
| Button — remove | "Hapus Foto" |
| During upload | "Foto sedang diunggah…" (inline, replaces button label) |
| Upload failure | "Gagal mengunggah. Coba lagi." |
| Invalid type | "Format tidak didukung (JPEG, PNG, WebP)." |
| Size exceeded | "Ukuran melebihi 5 MB." |
| Post-upload reminder | "Foto tersimpan. Simpan formulir untuk menerapkan perubahan lainnya." |
| Hint text below buttons | "JPEG, PNG, WebP. Maks. 5 MB." |
| Alt text on photo | "Foto {therapist_name}" |

#### Remove photo flow

"Hapus Foto" button (ghost destructive variant, only visible when `photo_key` is not null) opens a minimal `AlertDialog`:

- Title: "Hapus Foto?"
- Body: "Foto profil terapis ini akan dihapus."
- Confirm: "Ya, Hapus" (destructive)
- Cancel: "Batal"

On confirm: call `DELETE /api/v1/tenant/therapists/:id/photo` (go-expert must confirm this endpoint exists; if not, flag as open question). On success: preview reverts to initials placeholder, `photo_key` is null. On failure: toast "Gagal menghapus foto. Coba lagi." and no state change.

**Open question TEP-11.1:** ADR 0011 §2.4 specifies `POST .../photo` for upload. It does not specify a separate DELETE endpoint for photo removal. If the intent is to clear `photo_key` via a `PATCH /therapists/:id` with `{"photo_key": null}` instead, the "Hapus Foto" button can call the profile PATCH and the above AlertDialog is still appropriate. Flag to go-expert for confirmation.

---

### TEP-3: Height and weight inputs

Both fields use `<Input type="number" inputMode="numeric">` (not native `<input type="number">` bare, but the shadcn `Input` primitive, which is already vendored). No custom number-spinner component is introduced.

**Inline adornment pattern:** suffix labels "cm" and "kg" are rendered as a `<span>` to the right of the input, inside a wrapper div that simulates an "input with right addon":

```
<div class="flex items-center">
  <Input
    type="number"
    inputMode="numeric"
    min={100}   // height
    max={250}
    step={1}
    className="rounded-r-none"
    {...field}
  />
  <span class="flex h-10 items-center rounded-r-md border border-l-0 border-input
               bg-muted/50 px-3 text-sm text-muted-foreground select-none">
    cm
  </span>
</div>
```

This is the **same inline-adornment pattern as the service-form `Harga` field** (where "Rp" prefixes the input with `rounded-l-none`). The suffix variant mirrors that pattern symmetrically. No new component. The `<span>` is `aria-hidden="true"` — the unit is communicated in the field label (see below).

**Field labels and validation messages:**

| Field | Label | `aria-label` supplement | Min | Max | Required |
|---|---|---|---|---|---|
| `height_cm` | Tinggi (cm) | — label already includes unit | 100 | 250 | Yes |
| `weight_kg` | Berat (kg) | — label already includes unit | 30 | 250 | Yes |

Including the unit in the label text ("Tinggi (cm)") means the suffix adornment is purely visual reinforcement — screen readers read the label, not the adornment span. This satisfies WCAG 1.3.1 without redundant announcements.

**Validation messages (Indonesian):**

| Condition | Message |
|---|---|
| Empty / not a number | "Tinggi wajib diisi." / "Berat wajib diisi." |
| Below minimum | "Tinggi minimal 100 cm." / "Berat minimal 30 kg." |
| Above maximum | "Tinggi maksimal 250 cm." / "Berat maksimal 250 kg." |
| Non-integer | "Masukkan angka bulat." |

**Zod fragments:**

```ts
height_cm: z.coerce
  .number({ invalid_type_error: "Tinggi wajib diisi." })
  .int("Masukkan angka bulat.")
  .min(100, "Tinggi minimal 100 cm.")
  .max(250, "Tinggi maksimal 250 cm."),

weight_kg: z.coerce
  .number({ invalid_type_error: "Berat wajib diisi." })
  .int("Masukkan angka bulat.")
  .min(30, "Berat minimal 30 kg.")
  .max(250, "Berat maksimal 250 kg."),
```

---

### TEP-4: Build select

Use the existing shadcn `<Select>` / `<SelectTrigger>` / `<SelectContent>` / `<SelectItem>` primitives — already vendored in `therapist-form.tsx`. No new component.

**Options (in display order):**

| API value | Display label |
|---|---|
| `langsing` | Langsing |
| `sedang` | Sedang |
| `atletis` | Atletis |
| `tegap` | Tegap |

**Default on edit form:** pre-populate from `therapist.build`. On new form: no pre-selection (placeholder shown).

**Placeholder:** "Pilih postur" (consistent with "Pilih jenis kelamin" already in the form).

**Help text** rendered as a `<p>` below the `<Select>` via `<FormDescription>` (shadcn `form.tsx` already exports this component alongside `FormMessage`):

`"Kategori yang akan ditampilkan ke pelanggan."`

`<FormDescription>` renders as `text-sm text-muted-foreground` — no new token or style needed.

**Validation message:** "Postur wajib dipilih." (required field; shown via `<FormMessage>` on submit if empty).

**Zod fragment:**

```ts
build: z.enum(["langsing", "sedang", "atletis", "tegap"], {
  required_error: "Postur wajib dipilih.",
  invalid_type_error: "Pilih salah satu postur.",
}),
```

---

### TEP-5: Therapist list page — photo avatar

#### Avatar display

Replace the existing `TherapistAvatar` component (initials-only) with a conditional that shows:

1. **Photo present** (`therapist.photo_url` is a non-null, non-empty string returned by the API): `<img>` element, `32×32` (`h-8 w-8`), `rounded-full object-cover`. Alt: `"Foto {therapist.full_name}"`. CSS: `shrink-0`.
2. **No photo** (`photo_url` null): existing initials circle (`bg-primary/10 text-primary`, initials from `full_name`) — no change to current behavior, just the conditional wrapper is new.

Size: `h-8 w-8` (32×32 px). This matches the existing `TherapistAvatar` size — no layout change on the list row.

The component signature becomes:

```tsx
function TherapistAvatar({ name, photoUrl }: { name: string; photoUrl?: string | null }) { ... }
```

The existing `TherapistAvatar` in `page.tsx` is updated in place. No new component file.

#### Body fields in list — NOT shown

`height_cm`, `weight_kg`, `build` are **not added to the list table**. These fields are contextually interesting only when evaluating a specific therapist (booking intent). Showing them in a row alongside name, branch, status would add columns that the tenant admin never needs to scan across rows. The table already has 4–5 columns; adding 3 more would overflow on most laptop viewports. They belong on the detail page.

#### Incomplete-profile badge (TEP-7 — row level)

See TEP-7 for the placeholder-data indicator in the list row.

---

### TEP-6: Therapist detail page — photo and body fields

#### Photo display on detail

On the Profil tab, the photo is shown as the full 128×128 `<img>` (or initials circle) at the top of the form — this is the same element used in the edit form described in TEP-2. No separate "view-only" photo rendering is needed because the Profil tab is always the edit form (existing implementation).

For the page header area (above the Tabs, below the breadcrumb link), add a medium avatar `h-12 w-12` (48×48) to the left of the "Detail Terapis" / `therapist.full_name` block. This gives the page an immediate identity anchor without waiting for the form to load. If `photo_url` is null: initials circle. This is a **read-only display** — the upload controls are inside the form (Profil tab).

```
[← Kembali ke Daftar Terapis]

[Avatar 48×48] [Detail Terapis (h1)]      [Aktif badge]
               [therapist.full_name (p)]
```

#### Body fields on detail — definition row

On the Profil tab, below the form or alongside the "Akun terhubung" read-only block, show a compact summary of the body fields when the therapist already has values:

```
<div class="rounded-md border bg-muted/30 px-3 py-2 mt-4" role="region"
     aria-label="Informasi postur">
  <p class="text-xs text-muted-foreground">Informasi Postur</p>
  <p class="mt-0.5 text-sm text-foreground tabular-nums">
    Tinggi: 167 cm · Berat: 58 kg · Postur: Atletis
  </p>
</div>
```

This is a **read-only summary** that appears above the "Informasi Postur" editable fields, so the user sees current values at a glance before editing. The `build` value is displayed with its display label (e.g., "Atletis"), not the API enum value ("atletis"). Use a simple map: `{ langsing: "Langsing", sedang: "Sedang", atletis: "Atletis", tegap: "Tegap" }`.

The summary block uses the same visual pattern as the existing "Akun terhubung" read-only field — `rounded-md border bg-muted/30 px-3 py-2` — for visual consistency with no new token required.

This summary block is **only shown in edit mode** (when a therapist already exists). On the `/new` form it is omitted (no existing values to summarise).

---

### TEP-7: Placeholder data banner

#### Detection rule

A therapist row is considered "unconfirmed profile" (still holding migration defaults from ADR 0011 §2.3.2) when **all three** conditions are true simultaneously:

- `height_cm === 160`
- `weight_kg === 60`
- `build === "sedang"`
- `photo_url` is null or empty

The conjunction of all four is required to avoid false positives: a therapist who legitimately weighs 60 kg and is "sedang" build should not see a banner if they have a photo or have separately confirmed a different height.

**Note:** this detection relies on the migration defaults being exactly `160 / 60 / sedang`. If the tenant later explicitly sets a different therapist to those same values, they will not be flagged (because they will likely have set a photo too). If a tenant has a therapist who is genuinely 160 cm / 60 kg / sedang / no photo, the banner appears — this is an acceptable false positive at this stage, because it prompts an admin to review and explicitly confirm the values, which is the desired behavior.

#### List page — row-level indicator

In the "Nama" cell, after the `full_name` text, add an amber `<Badge>` inline:

```tsx
{isPlaceholderProfile(t) && (
  <Badge
    variant="outline"
    className="ml-1.5 border-amber-300 bg-amber-50 text-amber-700
               text-[10px] font-normal"
    aria-label="Profil belum dilengkapi"
  >
    Belum lengkap
  </Badge>
)}
```

The badge is inline with the name in the flex group. `text-[10px]` keeps it visually subordinate to the name. "Belum lengkap" is compact enough for the 200px minimum column width without wrapping.

`isPlaceholderProfile` is a pure utility function: `(t: Therapist) => t.height_cm === 160 && t.weight_kg === 60 && t.build === "sedang" && !t.photo_url`. Lives in `lib/utils.ts` alongside `categoryColorClass`.

The badge is a **passive indicator only** at list level — it is not a button. Clicking the row or the pencil button navigates to the detail page where the full banner and form are available.

#### Detail page — top-of-page banner

On the Profil tab (`TherapistDetailPage`), when `isPlaceholderProfile(therapist)` is true, render a soft-amber banner **above the form Card but below the Tabs component**:

```tsx
{isPlaceholderProfile(therapist) && (
  <div
    role="status"
    className="flex items-start gap-2 rounded-lg border border-amber-300
               bg-amber-50 px-4 py-3 text-sm text-amber-800"
  >
    <AlertCircle size={16} className="mt-0.5 shrink-0 text-amber-600"
                 aria-hidden="true" />
    <span>
      Profil belum dilengkapi — lengkapi data postur dan foto sebelum
      meluncurkan ke pelanggan.
    </span>
  </div>
)}
```

The banner uses `role="status"` (live region, polite) consistent with ADR spec. It uses `AlertCircle` from lucide-react (already imported in `page.tsx` for the error state). The color palette `border-amber-300 bg-amber-50 text-amber-800 text-amber-600` mirrors the `photoJustUploaded` note from TEP-2 — consistent amber semantic color for "attention, but not an error."

**"Click to focus form"** — the banner itself is not a link or button. The form fields are immediately below (or visible inside the Profil tab). The instruction "lengkapi data postur dan foto" is enough directional guidance — a link to the form would add complexity without meaningful value since the form is already on the same page.

The banner disappears on the next page load once the therapist is no longer in the placeholder state (i.e., after the admin saves a photo or modified body values).

---

### TEP-8: Customer-facing preview (Phase 5 note)

**Phase 4 does NOT include a customer-facing preview.** This section documents the design intent for Phase 5.

When the booking UI lands in Phase 5, the same `photo_url`, `height_cm`, `weight_kg`, and `build` fields will be displayed to customers on the therapist selection card. The anticipated rendering:

- **Therapist card (booking list):** circular photo avatar (64×64), name, `build` display label as a subtle chip (e.g., "Atletis").
- **Therapist modal / detail sheet (booking):** photo (128×128), full name, bio, `build` / `height_cm` / `weight_kg` in a small stats row.

In Phase 4 admin form, **no preview panel is added**. The "Kategori yang akan ditampilkan ke pelanggan." help text on the build Select (TEP-4) is the only forward-reference to the customer context.

The `nextjs-expert` should ensure `height_cm`, `weight_kg`, and `build` are included in the Therapist TypeScript type in `lib/types.ts` now (alongside `photo_url`), so Phase 5 customer screens can consume them without a type change.

---

### TEP-9: Accessibility

#### Photo `alt` text — mandatory rule

Every `<img>` render of a therapist photo must have `alt="Foto {therapist_name}"` where `therapist_name` is the therapist's `full_name`. This applies to:

- The 32×32 list avatar (`TherapistAvatar` component, when `photo_url` is present)
- The 48×48 detail page header avatar
- The 128×128 form preview in TEP-2

When the fallback initials circle is shown instead of an `<img>`, the wrapping div must have `aria-label="{therapist_name}"` so screen readers announce the identity without the photo. The existing `TherapistAvatar` implementation in `page.tsx` does not currently include this — it must be added.

#### File input accessibility

The hidden `<input type="file">` must be associated with a visible `<label>` via `htmlFor` / `id`. Even though the label is visually rendered as a `<Button>`, the semantic label must exist:

```tsx
<label htmlFor="photo-upload" className="sr-only">
  Unggah foto terapis
</label>
<input
  id="photo-upload"
  type="file"
  accept="image/jpeg,image/png,image/webp"
  className="sr-only"
  ref={fileInputRef}
  onChange={handleFileChange}
  aria-describedby="photo-upload-error photo-upload-hint"
/>
```

Error messages are linked via `aria-describedby="photo-upload-error"`. The hint text ("JPEG, PNG, WebP. Maks. 5 MB.") is linked via `aria-describedby="photo-upload-hint"`. Both IDs are on the corresponding `<p>` elements. When no error is present, `id="photo-upload-error"` can be absent (use conditional rendering); `aria-describedby` will simply resolve to only the hint.

#### Build Select

The `<Select>` wraps inside a `<FormItem>` with `<FormLabel>` associated via RHF's `control` prop — same as `gender` and `branch_id` already in the form. No additional ARIA attributes needed beyond what RHF + shadcn provide.

#### Height / Weight inputs

Both use `<FormItem>` / `<FormLabel>` / `<FormControl>` — the RHF pattern ensures `htmlFor` ↔ `id` association. The adornment `<span>` ("cm" / "kg") is `aria-hidden="true"` because the unit is already in the `<FormLabel>` text ("Tinggi (cm)", "Berat (kg)"). Avoid doubling the unit announcement.

#### Placeholder banner (TEP-7)

Both the list badge (`aria-label="Profil belum dilengkapi"`) and the detail banner (`role="status"`) have explicit accessible semantics. The banner copy communicates the action required in text — no color-only signal.

#### Focus management — photo upload

After a successful upload, focus should not jump unexpectedly. The `<Button>` that triggered the upload remains the natural focus position. The "Foto tersimpan. Simpan formulir untuk menerapkan perubahan lainnya." note appears below the button and is announced via its `role="status"` live region — no explicit `focus()` call needed.

---

### TEP-10: Component reuse inventory

No new shadcn/ui components are introduced. All primitives are already vendored.

| Primitive | Where used in TEP |
|---|---|
| `Input` | `height_cm`, `weight_kg` fields (type="number") |
| `Select`, `SelectTrigger`, `SelectContent`, `SelectItem` | `build` field |
| `Button` | "Unggah Foto", "Ganti Foto", "Hapus Foto", spinner-state |
| `Badge` | Placeholder "Belum lengkap" indicator on list row; amber amber tint via className override |
| `Form`, `FormField`, `FormItem`, `FormLabel`, `FormControl`, `FormMessage`, `FormDescription` | All new fields — same RHF wiring as existing form fields |
| `AlertDialog`, `AlertDialogContent`, `AlertDialogHeader`, `AlertDialogTitle`, `AlertDialogDescription`, `AlertDialogFooter`, `AlertDialogAction`, `AlertDialogCancel` | "Hapus Foto" confirmation |
| `AlertCircle` (lucide) | Detail page banner icon |

**Avatar:** the `TherapistAvatar` function in `therapist/page.tsx` is extended in place (new `photoUrl` prop). It is **not** the shadcn `Avatar` component — it is a plain hand-rolled component using `<img>` + a fallback `<div>`. Do NOT introduce the shadcn `Avatar` component (the constraint "no new shadcn components" applies, and the existing hand-rolled avatar is already consistent across the app).

**`FormDescription`:** this is part of `components/ui/form.tsx` (already vendored as part of the shadcn Form setup). It is not a new component — just an unused export from the existing file.

**New utility function:** `isPlaceholderProfile(t: Therapist): boolean` in `lib/utils.ts`. No component.

**New Lucide icons required** (may need import on modified files):

| Icon | Where |
|---|---|
| `Loader2` | Photo upload spinner — already imported in `therapist-form.tsx` |
| `AlertCircle` | Detail page banner — already imported in `therapists/page.tsx` for error state |
| `UserRound` | Photo fallback on `/new` before name is set — already imported in `therapists/page.tsx` |

No new icon imports are expected to be needed. All required icons are already in files that will be modified.

---

### TEP-11: Open questions

1. **Photo delete endpoint (TEP-2 flag):** ADR 0011 specifies `POST .../photo` for upload but does not specify a dedicated DELETE endpoint for removing a photo. Confirm with `go-expert` whether photo removal is `DELETE /tenant/therapists/:id/photo` or `PATCH /tenant/therapists/:id` with `{"photo_key": null}`. The UX flow (AlertDialog confirmation → API call → revert to initials) is the same either way; only the HTTP verb and path differ. **Blocking for `nextjs-expert` before implementing "Hapus Foto".**

2. **`photo_url` in the Therapist DTO vs `photo_key`:** ADR 0011 §2.4 states the API returns the resolved `photo_url` (not the raw `photo_key`) in the therapist DTO. Confirm with `go-expert` that the DTO field name is `photo_url` (resolved URL, nullable string) for use in `<img src>`, and that `photo_key` is never exposed to the frontend. The TypeScript type in `lib/types.ts` should have `photo_url: string | null`, not `photo_key`.

3. **Placeholder detection on new-therapist create form:** the `isPlaceholderProfile` check applies to existing therapists loaded from the API. On the `/new` form, default values for `height_cm`, `weight_kg`, `build` in the Zod schema must NOT pre-populate to `160 / 60 / sedang` — these should be empty/undefined so the admin is forced to fill them in. Confirm with `nextjs-expert` that zod defaults for the new form leave these fields blank (triggering required validation on submit) rather than pre-filling with the migration defaults.

4. **`FormDescription` vendored export:** `FormDescription` is part of `form.tsx` in the shadcn setup. Verify it is already exported from `components/ui/form.tsx` in the tenant-admin portal before using it in the build Select. If it is absent, a plain `<p className="text-sm text-muted-foreground">` is the fallback — no import needed.

---

## Ruangan (ADR 0012)

_Owned by `ui-ux-expert`. Implemented by `nextjs-expert`. Backend contract in ADR 0012 §2.3._

_Scope: tenant-admin portal only. Phase 4 ships the room catalog (CRUD + photo). Customer-facing room picker is Phase 5._

_Mirrors the grammar established by ADR 0010 (Add-on catalog) and ADR 0011 (Therapist extended profile). Read those sections before implementing._

---

### RM-1. Sidebar nav placement

**Sidebar entry:** add "Ruangan" to the existing Operasional section alongside "Terapis", "Layanan", and "Tambahan".

**Icon:** `DoorOpen` (lucide-react). Rationale: `DoorOpen` is a direct physical metaphor for the entity — a room with a door. `Bed` was considered but connotes accommodation/hospitality too specifically; the product is neutral across spa, clinic, and salon contexts. `DoorOpen` is unused elsewhere in the tenant-admin portal.

**Label:** "Ruangan" (Indonesian-first). Do not use "Room" or "Kamar" — the product term established in ADR 0012 is "Ruangan".

**Active route match:** `/master/rooms` and all sub-routes (`/master/rooms/new`, `/master/rooms/[id]`).

**`activeNav` value:** reuse `"operasional"` — no new key. Same as "Terapis", "Layanan", and "Tambahan". The `AppHeader activeNav` prop on `app/master/layout.tsx` does not require a new union member.

**NavKey note:** if the nav system maintains a typed `NavKey` union for sidebar items, add `"ruangan"` to that union (like `"tambahan"` was added for ADR 0010), but the `activeNav` section key for the Operasional group stays `"operasional"`.

---

### RM-2. List page — `/master/rooms`

**Purpose:** browse, filter, reorder, and manage all rooms within the tenant's accessible branches.

**Layout:**

```
[Page header row]  "Ruangan"  [subtitle]            [+ Tambah Ruangan]
[Filter bar]       Cabang pills + Status select + Tipe select
[Table / empty state]
[Pagination]
```

**Page header:**

- `<h1 className="text-xl font-semibold text-foreground sm:text-2xl">Ruangan</h1>`
- Subtitle adapts to branch context (same pattern as therapists page):
  - 1 branch: `"Kelola data ruangan di cabang Anda"`
  - >1 branch + filter active: `"Kelola data ruangan di cabang {branchName}"`
  - >1 branch + no filter: `"Kelola data ruangan di semua cabang Anda"`
- Header div: `className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"`
- CTA: `<Button asChild className="w-full sm:w-auto"><Link href="/master/rooms/new"><Plus size={16} aria-hidden="true" />Tambah Ruangan</Link></Button>`

**Table — `<Card><CardContent className="overflow-x-auto p-0">`:**

`<Table className="min-w-[640px]">` — 7 data columns plus thumbnail; wider than addon table to accommodate the extra columns.

| # | `<TableHead>` | Width / align | Cell content |
|---|---|---|---|
| 1 | Foto | `w-[56px]`, center | `RoomThumbnail` — 40×40 rounded square (`h-10 w-10 rounded-md object-cover`). No-photo fallback: `DoorOpen` icon centered in `bg-muted rounded-md` square. When `is_active = false`, thumbnail overlay: `opacity-60`. |
| 2 | Nama | `min-w-[160px]`, left | `room.name` — `font-medium`. When `is_active = false`: additionally `line-through text-muted-foreground`. |
| 3 | Cabang | `min-w-[120px]`, left | Branch name from resolved branches list. Show `—` if not found. Hidden when only one branch accessible (same `showBranchColumn` pattern as therapists). |
| 4 | Tipe | `w-[100px]`, left | `<Badge variant="outline">` with display label. See room_type label map below. |
| 5 | Kapasitas | `w-[90px]`, center | Plain number, `tabular-nums`. Header: "Kapasitas". |
| 6 | Status | `w-[90px]`, left | `<Badge variant={room.is_active ? "success" : "muted"}>{room.is_active ? "Aktif" : "Nonaktif"}</Badge>` |
| 7 | Urutan | `w-[96px]`, center | Up / down reorder buttons. See RM-5. Hidden when branch filter is not set to a single branch (see RM-5). |
| 8 | Aksi | `w-[80px]`, left (header) / `flex justify-end` (cell) | Pencil edit button + three-dot dropdown (DropdownMenu). Same layout as therapists and addons. |

`<TableHeader className="bg-muted/30">` — consistent with other list pages.
`<TableRow className="h-14">` — 56px row height.
`<TableBody className="text-sm">`.

**`room_type` display label map:**

| API value | Display label |
|---|---|
| `single` | Single |
| `couple` | Couple |
| `group` | Group |
| `vip` | VIP |

---

### RM-3. Pagination

Cursor-based, page size **10** (Lustia list convention). Uses the existing `<Pagination>` component:

```
<Pagination
  pathname="/master/rooms"
  searchParams={{ branch_id, is_active, room_type }}
  nextCursor={nextCursor}
  hasPrev={!!cursor}
  currentPageCount={rooms.length}
  pageSize={PAGE_SIZE}
/>
```

`PAGE_SIZE = 10` constant at top of page file. Renders `null` when `!hasPrev && !nextCursor`.

---

### RM-4. Filter bar

**Spec:**

```
<FilterBar isActive={filterActive} resetHref="/master/rooms">
  {showBranchFilter && (
    <div className="flex flex-wrap items-center gap-1.5">
      <span>Cabang</span>
      <Link "Semua" />
      {pillBranches.map(...)}
      {overflowBranches.length > 0 && <BranchOverflowSelect ... />}
    </div>
  )}
  <FilterSelect
    label="Status"
    name="is_active"
    options={[
      { value: "", label: "Semua" },
      { value: "true", label: "Aktif" },
      { value: "false", label: "Tidak aktif" },
    ]}
  />
  <FilterSelect
    label="Tipe"
    name="room_type"
    options={[
      { value: "", label: "Semua" },
      { value: "single", label: "Single" },
      { value: "couple", label: "Couple" },
      { value: "group", label: "Group" },
      { value: "vip", label: "VIP" },
    ]}
  />
</FilterBar>
```

- `filterActive` is `!!(branch_id || is_active || room_type)`.
- Branch pill overflow rule: same `PILL_CAP = 3` + `BranchOverflowSelect` pattern as the therapist list.
- Backend returns only the branches the calling user can manage — no client-side branch filtering needed.
- Status values: "Semua", "Aktif", "Tidak aktif" (Indonesian copy, consistent with therapist and service pages; note "Tidak aktif" not "Nonaktif" here — use whichever label the API contract maps to `is_active=false`; if `is_active=false` the existing FilterSelect across all pages uses "Nonaktif" — match that for consistency: `{ value: "false", label: "Nonaktif" }`).
- "× Hapus filter" link rendered by `FilterBar` when `isActive` is true — existing behavior, no change.

---

### RM-5. Reorder UX

**Controls:** `ChevronUp` and `ChevronDown` ghost icon buttons in the "Urutan" column, one pair per row.

**Visibility rule:** reorder buttons are visible only when the branch filter is set to a single specific branch (`branch_id` query param is present and non-empty). When `branch_id` is absent (showing "Semua cabang"), the entire Urutan column is hidden (`showReorder = !!branch_id`). Rationale from ADR 0012 §3.3: `sort_order` is per-branch; reordering across branches is undefined behavior. The column header "Urutan" is also hidden when `showReorder` is false — no empty column.

**Cell layout (Urutan column, same spec as ADO-5):**

```
<div class="flex items-center justify-center gap-0.5">
  <Button variant="ghost" size="icon" className="h-7 w-7"
          aria-label="Pindah ke atas: [room.name]"
          disabled={isFirst || isReordering}>
    <ChevronUp size={14} />
  </Button>
  <Button variant="ghost" size="icon" className="h-7 w-7"
          aria-label="Pindah ke bawah: [room.name]"
          disabled={isLast || isReordering}>
    <ChevronDown size={14} />
  </Button>
</div>
```

**Disabled boundaries:**
- `isFirst`: ChevronUp disabled. Top-most item on current page cannot move up.
- `isLast`: ChevronDown disabled. Bottom-most item on current page cannot move down.
- `isReordering`: both buttons on all rows disabled while any reorder call is in flight.

Do not hide disabled buttons — their absence causes cell-width jitter. Use the `disabled` prop.

**Interaction model — optimistic update with server sync:**

1. User clicks ChevronUp or ChevronDown on row N.
2. Client immediately swaps rows N and N±1 in local React state (instant re-render).
3. Client calls `PUT /api/v1/tenant/rooms/reorder` with the full current-page array: `{"branch_id": "...", "items": [{"id": "...", "sort_order": 0}, ...]}`. The `branch_id` is included in the request body per ADR 0012 §2.3 (service layer rejects if items span branches). `sort_order` values are 0-based sequential integers matching new order.
4. On success: no further action.
5. On failure: rollback local state to pre-click order. Toast: `"Gagal mengubah urutan. Coba lagi."`. Re-enable buttons.

**Page-scoped reorder:** same boundary semantics as ADO-5. Items not in the payload are not moved by the backend.

**No row-swap animation** — instant swap, no `framer-motion` or `@dnd-kit`. Satisfies `prefers-reduced-motion` trivially.

**UX hint on "Semua cabang" state:** when `branch_id` is absent and reorder is hidden, a subtle helper note appears below the filter bar (inside `FilterBar` container, as a `<p>` sibling to the filter controls):

```
<p className="text-xs text-muted-foreground">
  Pilih satu cabang untuk mengubah urutan tampilan ruangan.
</p>
```

This note is only rendered when `branches.length > 1` (i.e., the user has access to multiple branches and therefore the reorder column would otherwise be relevant). Single-branch tenants never see it.

---

### RM-6. Create page — `/master/rooms/new`

**Layout — same grammar as `/master/addons/new` and `/master/therapists/new`:**

```
[Back link]  "← Kembali ke Daftar Ruangan"
[Page header]  "Tambah Ruangan"  /  subtitle "Isi detail ruangan baru"
[Single-column form card]
  [CardContent className="p-6"]
    [form fields — see below]
    [button row: Batal | Simpan & Tambah Ruangan]
```

**Back link:** `<Link href="/master/rooms" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"><ArrowLeft size={14} aria-hidden="true" />Kembali ke Daftar Ruangan</Link>`

**Field order and spec:**

| # | Field | Input | Label | Required | Constraint | Notes |
|---|---|---|---|---|---|---|
| 1 | `name` | `Input` | Nama Ruangan | Yes | 1–120 chars | `autoFocus`. Placeholder: `"VIP 1"`. |
| 2 | `branch_id` | `Select` | Cabang | Yes | UUID from user-accessible branches | Populated from branches API. Placeholder: `"Pilih cabang"`. For `branch_admin` with only one branch, pre-select that branch and disable the Select (user cannot change it). |
| 3 | `room_type` | `Select` | Tipe Ruangan | Yes | `single \| couple \| group \| vip` | Placeholder: `"Pilih tipe"`. Options: Single / Couple / Group / VIP. |
| 4 | `capacity` | `Input type="number"` | Kapasitas | Yes | Integer 1–20, DEFAULT 1 | `min={1}` `max={20}` `step={1}` `inputMode="numeric"`. Suffix adornment: `"orang"` (same inline-adornment pattern as TEP-3 height/weight). |
| 5 | `amenities` | Tag input (see note) | Fasilitas | No | Free-text array | Hint text below field: `"Pisahkan dengan koma — contoh: shower, aromaterapi, tv"`. On blur/comma: split, trim, deduplicate. Each tag shown as `<Badge variant="secondary">` with an `×` remove button. |
| 6 | `description` | `Textarea` (native, same pattern as service/addon forms) | Deskripsi | No | max 500 chars | `rows={3}`. Placeholder: `"Keterangan ruangan yang ditampilkan kepada pelanggan."` |
| 7 | `is_active` | Switch (shadcn) | Status | — | Boolean, DEFAULT true | Label: `"Aktif"`. `<div className="flex items-center justify-between">` wrapper. |

`sort_order` is not a form field. The server appends new rooms at the end of the branch's sort order.

**No photo on create.** Consistent with the therapist pattern (ADR 0011): save the record first, upload photo after. Photo upload section appears only on the edit page.

**Amenities tag input implementation note:** Phase 4 does not vendor a dedicated tag-input library. Implement as a controlled `Input` that accumulates values on comma or blur: when the user types `"shower, tv"` and presses comma or blurs, the string splits into `["shower", "tv"]`, each rendered as a `<Badge variant="secondary">` with a remove `×` button (`aria-label="Hapus fasilitas: shower"`). The underlying form value is `string[]`. This is a small composed pattern from existing primitives (`Input` + `Badge` + `Button`) — no new shadcn component.

**Zod schema:**

```
name:        z.string().min(1, "Nama ruangan wajib diisi.").max(120, "Nama maksimal 120 karakter.")
branch_id:   z.string().uuid("Cabang tidak valid.").min(1, "Cabang wajib dipilih.")
room_type:   z.enum(["single","couple","group","vip"], { required_error: "Tipe ruangan wajib dipilih." })
capacity:    z.coerce.number({ invalid_type_error: "Kapasitas harus berupa angka." })
               .int("Kapasitas harus bilangan bulat.")
               .min(1, "Kapasitas minimal 1.").max(20, "Kapasitas maksimal 20.")
amenities:   z.array(z.string()).default([])
description: z.string().max(500, "Deskripsi maksimal 500 karakter.").optional()
is_active:   z.boolean()
```

**Button row:**

| Action | Label | Variant | Position |
|---|---|---|---|
| Cancel | Batal | ghost | Left |
| Submit | Simpan & Tambah Ruangan | primary | Right |

Submit: disabled while `isPending`; shows `<Loader2 className="animate-spin" size={14} />` in loading state.

"Batal" navigates to `/master/rooms` — does NOT call `window.history.back()`.

**Post-save redirect:** navigate to `/master/rooms/[newId]`. Toast: `"Ruangan berhasil ditambahkan."` (success).

**Duplicate name error (409):** `form.setError("name", { message: "Nama ruangan sudah digunakan di cabang ini." })` — field-level, not toast.

---

### RM-7. Edit page — `/master/rooms/[id]`

**Layout:**

```
[Back link]  "← Kembali ke Daftar Ruangan"
[Page header row]
  [h1 "Detail Ruangan"]  [subtitle: room.name]  [Status Badge — right slot]
[Photo picker section — Card]
[Form card — same fields as /new]
[Destructive actions — below form card]
```

**Status badge in header:** `<Badge variant={room.is_active ? "success" : "muted"}>` — visible at a glance before form content renders.

**Form fields:** identical to create page (RM-6), pre-populated from the fetched room DTO. `branch_id` is shown as a read-only `<Select>` (disabled) on the edit page — rooms cannot change branches after creation (data integrity: `sort_order` and `UNIQUE(branch_id, name)` constraint). A disabled `<Select>` renders identically to an enabled one but does not respond to interaction; it should have `aria-disabled="true"` and a tooltip or `<FormDescription>`: `"Cabang tidak dapat diubah setelah ruangan dibuat."`.

**Save button:** "Simpan Perubahan" (primary). On success: stay on page, toast `"Perubahan berhasil disimpan."`.

**Photo picker section (above form card):**

Sits in its own `<Card>` before the form Card, consistent with the therapist photo picker position at the top of the edit form. Section heading: `<p className="text-sm font-medium text-foreground mb-3">Foto Ruangan</p>`.

Photo picker layout (mirrors TEP-2 exactly, adapted for rooms):

```
[sm+ layout]
<div class="flex items-center gap-4">
  <RoomPhoto 80×80 rounded-md (not circular — rooms are not people)>
  <div class="flex flex-col gap-2">
    <Button "Unggah Foto" or "Ganti Foto">
    <Button "Hapus Foto" (destructive ghost, only when photo_url present)>
    <p class="text-xs text-muted-foreground">JPEG, PNG, WebP. Min. 800×600 px. Maks. 5 MB.</p>
  </div>
</div>

[mobile: flex-col, preview centered, buttons below]
```

**Photo shape:** `rounded-md` (not `rounded-full`) — rooms are physical spaces, not people. Size: `h-20 w-20` (80×80 px) on the edit page. No-photo fallback: `DoorOpen` icon centered in a `bg-muted rounded-md h-20 w-20` container.

**Upload:** same save-then-upload pattern as TEP-2. `POST /api/v1/tenant/rooms/:id/photo`. On file pick (after client validation), upload fires immediately. In-flight: spinner + "Foto sedang diunggah…", both buttons disabled. Post-upload amber note: `"Foto tersimpan. Simpan formulir untuk menerapkan perubahan lainnya."` (role="status", same as TEP-2). On success: preview updates to the returned `photo_url`.

**Remove photo:** "Hapus Foto" opens `AlertDialog`:
- Title: `"Hapus Foto Ruangan?"`
- Body: `"Foto ruangan \"[room.name]\" akan dihapus."`
- Confirm: `"Ya, Hapus"` (destructive)
- Cancel: `"Batal"`
- On confirm: call `DELETE /api/v1/tenant/rooms/:id/photo`. On success: preview reverts to `DoorOpen` fallback. On failure: toast `"Gagal menghapus foto. Coba lagi."`.

**Inactive record indicator on list row (RM-2):** `line-through text-muted-foreground` on Nama cell + `opacity-60` on thumbnail. No row background change.

**Destructive actions section (below form card):**

`<div className="flex justify-end gap-2 mt-4">`

1. **Nonaktifkan / Aktifkan** — context-sensitive. When `is_active = true`: "Nonaktifkan" (outline warning tone). When `is_active = false`: "Aktifkan" (outline success tone). No confirmation dialog. Optimistic flip. Toast on success: `"Ruangan diaktifkan."` or `"Ruangan dinonaktifkan."`.

2. **Hapus** — outline destructive. Opens `AlertDialog` before calling `DELETE /api/v1/tenant/rooms/:id`. On success: navigate to `/master/rooms`, toast `"Ruangan berhasil dihapus."`. On failure: toast `"Gagal menghapus ruangan. Coba lagi."`.

---

### RM-8. Photo treatment

**Shape:** `rounded-md` (4px or `md` radius token) — not circular. Rooms are physical spaces; circular framing is reserved for people (therapist avatars). This distinction is load-bearing for the product's visual language.

**Sizes by context:**

| Context | Size | Shape |
|---|---|---|
| List table thumbnail | 40×40 (`h-10 w-10`) | `rounded-md` |
| Edit page photo picker | 80×80 (`h-20 w-20`) | `rounded-md` |
| (Phase 5) Customer booking card | TBD — Phase 5 spec | — |

**No-photo fallback (all sizes):** `DoorOpen` icon centered inside a `bg-muted rounded-md` square matching the target size. The icon is `aria-hidden="true"`; the wrapping container carries `aria-label="Belum ada foto"` for screen readers.

**Recommended dimension hint shown to operator (edit page, below the photo buttons):**

```
<p className="text-xs text-muted-foreground">
  JPEG, PNG, WebP. Min. 800×600 px. Maks. 5 MB.
</p>
```

This is an informational hint — the backend's `ProcessUpload` does not reject images below 800×600 (the ADR 0011 pipeline does not enforce minimum dimensions). The hint sets expectations for photo quality; it is not a hard constraint enforced client-side.

**Server-side resize:** `imaging.Fit` to 1024×1024 (ADR 0012 §2.5, reusing ADR 0011 pipeline). Any aspect ratio accepted. Operators uploading portrait, landscape, or square images will see the result faithfully rendered because the `object-cover` CSS class crops the display area consistently regardless of the uploaded aspect ratio.

**`alt` text rule:** every `<img>` render of a room photo must have `alt="Foto ruangan [room.name]"`. When the fallback icon is shown, the container must have `aria-label="Belum ada foto"`.

---

### RM-9. Activate / deactivate + soft-delete

**Activate / deactivate:**
- Available from both the list page (row DropdownMenu) and the edit page (button below form).
- No confirmation dialog — toggle is reversible.
- Optimistic flip + `PATCH /api/v1/tenant/rooms/:id/status` with body `{"is_active": true|false}`.
- Rollback on failure with toast.

**Dropdown menu on list row (Aksi cell — same structure as ADO-8):**

```
<div class="flex items-center justify-end gap-1">
  <Button variant="ghost" size="icon" asChild aria-label="Edit ruangan: [room.name]">
    <Link href="/master/rooms/[id]"><Pencil size={14} /></Link>
  </Button>
  <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button variant="ghost" size="icon"
              aria-label="Tindakan lainnya untuk ruangan: [room.name]">
        <MoreHorizontal size={14} />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end">
      <DropdownMenuItem>
        {room.is_active
          ? <><EyeOff size={14} /> Nonaktifkan</>
          : <><Eye size={14} /> Aktifkan</>}
      </DropdownMenuItem>
      <DropdownMenuSeparator />
      <DropdownMenuItem className="text-destructive">
        <Trash2 size={14} /> Hapus
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</div>
```

**Soft-delete confirmation dialog (AlertDialog):**

- Title: `"Hapus Ruangan?"`
- Body: `"Ruangan \"[room.name]\" akan dihapus secara permanen dan tidak dapat dipulihkan."`
- Confirm: `"Ya, Hapus"` (destructive Button variant)
- Cancel: `"Batal"`
- Focus default on open: "Batal" (Cancel rendered before Hapus in DOM order — same Radix focus rule as ADO-8).

---

### RM-10. Empty state

**Unfiltered empty state** (no rooms exist for any accessible branch):

Layout inside `CardContent`, vertically centered, `py-16`:

```
[DoorOpen icon, size 40, text-muted-foreground/50, aria-hidden="true"]
[Headline]   "Belum ada ruangan"  — text-sm font-medium text-muted-foreground
[Supporting] "Tambahkan ruangan agar pelanggan dapat memilihnya saat booking."
             — text-xs text-muted-foreground/80 max-w-xs mx-auto text-center
[CTA]        "Tambah Ruangan"     — Button size="sm" asChild Link href="/master/rooms/new"
```

**Branch-filtered empty state** (branch selected, no rooms for that branch):

```
[DoorOpen icon, size 40, ...]
[Copy] "Belum ada ruangan untuk cabang ini."
[CTA]  "Tambah Ruangan"  — Button size="sm" asChild Link href="/master/rooms/new"
```

**Filter-active empty states (no CTA, informational only):**

| Filter state | Copy |
|---|---|
| `is_active=true` | `"Tidak ada ruangan aktif saat ini."` |
| `is_active=false` | `"Tidak ada ruangan nonaktif."` |
| `room_type=single` (or any type) | `"Tidak ada ruangan bertipe [Tipe] saat ini."` |

Type label in copy uses the display label (e.g., "Single", "Couple", "VIP") not the API enum value.

---

### RM-11. Validation messages (Indonesian)

| Field | Condition | Message |
|---|---|---|
| `name` | Empty | `"Nama ruangan wajib diisi."` |
| `name` | > 120 chars | `"Nama maksimal 120 karakter."` |
| `name` | Duplicate in branch (409) | `"Nama ruangan sudah digunakan di cabang ini."` (field-level via `form.setError`) |
| `branch_id` | Not selected | `"Cabang wajib dipilih."` |
| `room_type` | Not selected | `"Tipe ruangan wajib dipilih."` |
| `capacity` | Empty / not a number | `"Kapasitas wajib diisi."` |
| `capacity` | Non-integer | `"Kapasitas harus bilangan bulat."` |
| `capacity` | < 1 | `"Kapasitas minimal 1."` |
| `capacity` | > 20 | `"Kapasitas maksimal 20."` |
| `amenities` | Individual tag > 80 chars | `"Tag terlalu panjang (maks. 80 karakter)."` — shown below the tag input, clears on next change |
| `description` | > 500 chars | `"Deskripsi maksimal 500 karakter."` |
| Photo upload — client | Invalid MIME type | `"Format tidak didukung (JPEG, PNG, WebP)."` |
| Photo upload — client | File > 5 MB | `"Ukuran melebihi 5 MB."` |
| Photo upload — server failure | Any | `"Gagal mengunggah. Coba lagi."` |

All messages surfaced via `<FormMessage>` (inline, below the field) for form fields. Photo errors appear inline below the photo button group (same as TEP-2). No toast for form validation errors.

---

### RM-12. Accessibility

**Reorder buttons (RM-5):**
- `aria-label` per button: `"Pindah ke atas: [room.name]"` and `"Pindah ke bawah: [room.name]"` — unique per row, per direction.
- `aria-disabled="true"` set automatically via shadcn `Button` disabled prop.
- Tab order: ChevronUp before ChevronDown in DOM order, matching visual order.
- When the Urutan column is hidden (no single-branch filter active), reorder controls are absent from the DOM entirely — not just hidden — so they are not reachable via keyboard.

**Photo picker (RM-7):**
- Hidden `<input type="file">` associated with an `sr-only` label: `<label htmlFor="room-photo-upload" className="sr-only">Unggah foto ruangan</label>`.
- `aria-describedby="room-photo-upload-error room-photo-upload-hint"` on the file input. Error span `id="room-photo-upload-error"` rendered only when an error is present; hint span `id="room-photo-upload-hint"` always present.
- Post-upload amber note uses `role="status"` (polite live region) — announced by screen readers without interruption.
- `<img>` elements: `alt="Foto ruangan [room.name]"`. Fallback container: `aria-label="Belum ada foto"`.

**Row DropdownMenu (RM-9):**
- Trigger: `aria-label="Tindakan lainnya untuk ruangan: [room.name]"` — unique per row (WCAG 2.4.6).
- Edit button: `aria-label="Edit ruangan: [room.name]"` — unique per row.
- `Escape` closes the menu (Radix default).
- Focus returns to trigger on close (Radix default).

**AlertDialog (delete, soft-delete, remove photo):**
- Focus lands on "Batal" by default — Cancel is first focusable element in `AlertDialogFooter` (DOM order: Batal before Ya, Hapus; visual order reversed by flex-direction in footer).
- `Escape` closes without acting (Radix default).
- Focus returns to trigger on close.

**Tag input (amenities, RM-6):**
- Each tag `<Badge>` includes a remove button with `aria-label="Hapus fasilitas: [tag value]"`.
- The `<Input>` for typing new tags has `aria-label="Tambah fasilitas"` if no visible `<label>` is present (i.e., the label "Fasilitas" is above the tag area but the input itself is a separate child element — ensure `htmlFor` wires to the input's `id`).

**`branch_id` Select (disabled on edit):**
- Disabled `<Select>` gets `aria-disabled="true"`.
- `<FormDescription>` text: `"Cabang tidak dapat diubah setelah ruangan dibuat."` — linked via `aria-describedby`.

**Color contrast:**
- All Badge variants, button text, and muted foreground copy comply with the existing token set (WCAG 4.5:1 for text, 3:1 for UI). No new color tokens are introduced.
- `opacity-60` on inactive thumbnail: thumbnails are decorative; the `line-through text-muted-foreground` on the name cell is the primary inactive signal (non-color per WCAG 1.4.1).

**Reduced motion:**
- Optimistic row reorder: instant DOM swap, no transition. Satisfies `prefers-reduced-motion` trivially.
- Photo upload spinner: `animate-spin` is a CSS animation; Tailwind's base-layer `prefers-reduced-motion: reduce` rule suppresses it automatically.

---

### RM-13. Component reuse inventory

No new shadcn/ui primitives. All components are already vendored.

| Component | Where used in RM |
|---|---|
| `Card`, `CardContent` | Outer wrapper for list table, photo picker section, form |
| `Table`, `TableHeader`, `TableBody`, `TableRow`, `TableHead`, `TableCell` | Room list table |
| `Badge` | `room_type` (outline), Status (success/muted), page-header status on edit, tag items in amenities field |
| `Button` | CTA, reorder controls, row actions (pencil, more), photo upload/remove, form submit/cancel, AlertDialog actions |
| `Select`, `SelectTrigger`, `SelectContent`, `SelectItem` | `branch_id` and `room_type` fields |
| `Input` | `name`, `capacity`, amenities tag entry |
| `Textarea` (native `<textarea>` + class string, same pattern as addon/service forms) | `description` field |
| `Switch` | `is_active` field |
| `DropdownMenu`, `DropdownMenuTrigger`, `DropdownMenuContent`, `DropdownMenuItem`, `DropdownMenuSeparator` | Row actions |
| `AlertDialog`, `AlertDialogContent`, `AlertDialogHeader`, `AlertDialogTitle`, `AlertDialogDescription`, `AlertDialogFooter`, `AlertDialogAction`, `AlertDialogCancel` | Delete confirmation, remove photo confirmation |
| `Form`, `FormField`, `FormItem`, `FormLabel`, `FormControl`, `FormMessage`, `FormDescription` | Create / edit form fields |
| `Pagination` | List page pagination |
| `FilterBar` | List page filter container |
| `FilterSelect` | Status and Tipe filters |
| `BranchOverflowSelect` | Branch overflow (>3 branches) in filter bar |

**Lucide icons required:**

| Icon | Where |
|---|---|
| `DoorOpen` | Nav icon, list thumbnail fallback, edit page photo fallback, list empty state |
| `ChevronUp`, `ChevronDown` | Reorder controls |
| `Eye`, `EyeOff` | Activate/deactivate dropdown items |
| `Trash2` | Delete dropdown item |
| `ArrowLeft` | Back link |
| `Plus` | Page header CTA |
| `Pencil` | Row edit button |
| `MoreHorizontal` | Row three-dot trigger |
| `AlertCircle` | Fetch error inline alert |
| `Loader2` | Form submit and photo upload spinner |

**Tag input:** composed from existing `Input` + `Badge` + `Button` — no new library or component file. Implement as a small controlled sub-component within the room form file (e.g., `AmenitiesTagInput`).

**`RoomThumbnail`:** a small inline function (not a separate file) in the list `page.tsx`, analogous to `TherapistAvatar`. Renders `<img>` when `photo_url` is present, falls back to `DoorOpen` icon in a muted square.

---

### RM-14. Microcopy additions

**Page titles / subtitles:**

| Screen | `<h1>` | Subtitle |
|---|---|---|
| Room list | Ruangan | Kelola data ruangan di semua cabang Anda _(adaptive, see RM-2)_ |
| Room new | Tambah Ruangan | Isi detail ruangan baru |
| Room detail | Detail Ruangan | _(room.name as secondary label below h1)_ |

**CTA buttons:**

| Action | Label | Variant |
|---|---|---|
| Create room (list CTA) | Tambah Ruangan | primary |
| Submit create form | Simpan & Tambah Ruangan | primary |
| Save edit form | Simpan Perubahan | primary |
| Deactivate room | Nonaktifkan | outline (warning tone) |
| Activate room | Aktifkan | outline (success tone) |
| Delete room (edit page) | Hapus | outline destructive |
| Confirm delete | Ya, Hapus | AlertDialogAction (destructive) |
| Cancel | Batal | ghost |

**Toasts:**

| Event | Message | Variant |
|---|---|---|
| Room created | `"Ruangan berhasil ditambahkan."` | success |
| Room updated | `"Perubahan berhasil disimpan."` | success |
| Room deleted | `"Ruangan berhasil dihapus."` | success |
| Room activated | `"Ruangan diaktifkan."` | success |
| Room deactivated | `"Ruangan dinonaktifkan."` | success |
| Reorder failed | `"Gagal mengubah urutan. Coba lagi."` | destructive |
| Delete failed | `"Gagal menghapus ruangan. Coba lagi."` | destructive |
| Any mutation failure | `"Gagal menyimpan. Coba lagi."` | destructive |
| Load failure (list) | `"Gagal memuat data ruangan. Muat ulang halaman."` | (inline alert, not toast) |

**Confirmation dialogs:**

| Dialog | Title | Body | Confirm | Cancel |
|---|---|---|---|---|
| Delete room | `"Hapus Ruangan?"` | `"Ruangan \"[room.name]\" akan dihapus secara permanen dan tidak dapat dipulihkan."` | `"Ya, Hapus"` | `"Batal"` |
| Remove photo | `"Hapus Foto Ruangan?"` | `"Foto ruangan \"[room.name]\" akan dihapus."` | `"Ya, Hapus"` | `"Batal"` |

---

### RM-15. Open questions

1. **Photo DELETE endpoint:** ADR 0012 §2.3 specifies `DELETE /:id/photo` for photo removal. Confirm with `go-expert` that this endpoint is implemented (not a `PATCH` with `photo_key: null`). The UX flow is identical either way; only the HTTP call differs. **Blocking for `nextjs-expert` before implementing "Hapus Foto".**

2. **`branch_id` read-only on edit:** the spec locks `branch_id` to read-only after creation. If business requirements later allow reassigning a room to a different branch (unlikely given `sort_order` semantics), this will need an explicit migration path. Flag as accepted limitation.

3. **Amenities tag limit:** no maximum tag count is specified in ADR 0012. The spec here enforces a maximum individual tag length of 80 chars. Confirm with `go-expert` whether the API enforces any per-item length on the `TEXT[]` column, and whether an array length cap (e.g., max 20 tags) is appropriate. If a cap exists, add a validation message: `"Maksimal [N] fasilitas."`.

4. **`photo_url` in Room DTO:** confirm with `go-expert` that the Room DTO returns `photo_url` (resolved URL, nullable string) and not the raw `photo_key`. TypeScript type in `lib/types.ts` should have `photo_url: string | null`.

