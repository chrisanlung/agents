# Booking Selection Screen — Design Spec
_Owned by `ui-ux-expert`. Implements flow BK-A6..A8 as a single progressive page._
_Status: Ready for implementation review — go-expert + flutter-expert._

---

## 1. Overview

The booking selection screen replaces three separate wizard steps (Therapist, Time, Room) with a single scrollable page that the customer works through top-to-bottom. The mental model follows the customer's natural question sequence: "Kapan saya mau datang? → Siapa yang akan menangani saya? → Jam berapa tersedia? → Ruangan mana?" The date anchors the entire availability graph, so it lands first. Therapist comes second because many customers have a strong preference — locking in the therapist before showing times means the time grid reflects only that therapist's real availability, eliminating confusion when a shown slot turns out to be taken when they actually try to book. Room is last: customers almost never care which room, so placing it after the high-stakes choices avoids friction at the start.

Progressive disclosure — each section is visually inert until the section above it is satisfied — keeps the screen from feeling overwhelming on first load. On completion of all four sections the sticky CTA bar activates and the customer taps through to payment.

This screen sits between the service/add-on selection steps (existing wizard) and the customer info step (also existing). It does not replace those steps; it replaces only the old Slot→Therapist→Room pages.

---

## 2. State Mockups

The mockups below use 390px mobile width as the reference column. On Flutter web the entire content column is capped at `maxWidth: 600` and centered. Section titles are 16dp `titleMedium`. Section bodies are 14dp `bodyMedium`. All numbers use `fontFeatures: [FontFeature.tabularFigures()]`.

### State A — Only date is showing (page first load)

```
┌─────────────────────────────────────────────┐
│ ← Pilih Jadwal & Terapis           [step x/n]│
│ ──────────────────── progress 3px ─────────── │
├─────────────────────────────────────────────┤
│  Layanan: Refleksi Kaki  •  60 mnt  Rp 85.000│  ← summary pill (read-only)
├─────────────────────────────────────────────┤
│                                             │
│  ① Tanggal                                  │
│  ┌──────────────────────────────────────┐   │
│  │  Hari ini, Kamis 1 Mei 2026  ▼       │   │  ← tappable date row
│  └──────────────────────────────────────┘   │
│                                             │
│  ② Pilih Terapis                            │
│  ┌──────────────────────────────────────┐   │
│  │  [icon sparkle]  Pilih terapis...    │   │  ← locked placeholder (56dp, low-opacity)
│  └──────────────────────────────────────┘   │
│     Pilih tanggal terlebih dahulu           │  ← lock caption (bodySmall)
│                                             │
│  ③ Pilih Waktu                              │
│  ┌──────────────────────────────────────┐   │
│  │  [icon clock]  Pilih waktu...        │   │  ← locked
│  └──────────────────────────────────────┘   │
│     Pilih terapis terlebih dahulu           │
│                                             │
│  ④ Pilih Ruangan                            │
│  ┌──────────────────────────────────────┐   │
│  │  [icon door]  Pilih ruangan...       │   │  ← locked
│  └──────────────────────────────────────┘   │
│     Pilih waktu terlebih dahulu             │
│                                             │
├─────────────────────────────────────────────┤
│  Rp 85.000          [Lanjut ke Pembayaran]  │  ← CTA bar, button disabled
└─────────────────────────────────────────────┘
```

> NOTE: On first load, "today" is auto-selected as the date. Section ① is immediately satisfied. Section ② unlocks on page load with the therapist list visible. Sections ③ and ④ remain locked until their predecessors are satisfied.

### State B — Date selected, therapist selected, sections ③ and ④ still locked

```
┌─────────────────────────────────────────────┐
│ ← Pilih Jadwal & Terapis                    │
│ ──────────────────── progress ─────────────  │
├─────────────────────────────────────────────┤
│  Refleksi Kaki  •  60 mnt  •  Rp 85.000    │
├─────────────────────────────────────────────┤
│  ① Tanggal                                  │
│  ┌──────────────────────────────────────┐   │
│  │  Kamis, 1 Mei 2026  ✓  Ubah tanggal │   │  ← selected row, check icon
│  └──────────────────────────────────────┘   │
│                                             │
│  ② Pilih Terapis                            │
│                                  ┌────────┐ │
│                                  │  2 / 4 │ │  ← counter badge (position/total)
│                                  └────────┘ │
│  ┌────────────────────────────────────┐ ░░  │  ← carousel card (next card bleeds ~16dp right)
│  │ [Full-width photo, 228dp tall,    ]│ ░░  │
│  │  cs.primaryContainer background  ]│ ░░  │
│  │  rounded top corners              │ ░░  │
│  ├────────────────────────────────────┤ ░░  │
│  │  Sari Dewi                    [✓] │ ░░  │  ← titleLarge w700 + check_circle
│  │  [Wanita] [Langsing] [163cm][58kg]│ ░░  │  ← chip row
│  │  Spesialis refleksi & aromaterapi │ ░░  │  ← bio 2 lines
│  │  ┌────────────────────────────┐   │ ░░  │
│  │  │     Pilih Sari Dewi        │   │ ░░  │  ← FilledButton.tonal
│  │  └────────────────────────────┘   │ ░░  │
│  └────────────────────────────────────┘ ░░  │
│         ●  ○  ○  ○                          │  ← indicator dots (active = filled)
│                                             │
│  ③ Pilih Waktu                              │
│  ┌──────────────────────────────────────┐   │  ← locked
│  │  [icon clock]  Pilih waktu...        │   │
│  └──────────────────────────────────────┘   │
│     Pilih terapis terlebih dahulu           │
│                                             │
│  ④ Pilih Ruangan    ...locked...            │
│                                             │
├─────────────────────────────────────────────┤
│  Rp 85.000          [Lanjut ke Pembayaran]  │  ← still disabled
└─────────────────────────────────────────────┘
```

> NOTE: The ░░ at the right edge represents the ~16dp bleed of the next card, which acts as a swipe affordance hint. The counter badge shows "2 / 4" because the visible card (Sari Dewi) is at position 2 (after the "Otomatis" card at position 1). The "Otomatis" card is always position 1. The selected card (Sari Dewi) shows the `check_circle` trailing indicator — the user has already tapped "Pilih Sari Dewi" to confirm. Landing on a card by swipe alone does NOT select it.

### State C — Date + therapist + time selected, room still locked

```
┌─────────────────────────────────────────────┐
│ ← Pilih Jadwal & Terapis                    │
├─────────────────────────────────────────────┤
│  Refleksi Kaki  •  60 mnt  •  Rp 85.000    │
├─────────────────────────────────────────────┤
│  ① Tanggal  [Kamis, 1 Mei 2026]  [✓ Ubah]  │
│                                             │
│  ② Terapis  [Sari Dewi]  [✓ Ubah]          │  ← carousel collapses to summary row once ③ unlocks
│                                             │
│  ③ Pilih Waktu                              │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐  │  ← 3-column slot grid
│  │09.00│ │10.00│ │11.00│ │12.00│ │13.00│  │
│  └─────┘ └─────┘ └─────┘ └─────┘ └─────┘  │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐  │
│  │14.00│ │15.00│x│16.00│x│17.00│ │18.00│  │  x = disabled
│  └─────┘ └─────┘ └─────┘ └─────┘ └─────┘  │
│  Slot bertanda × sudah dibooking            │  ← legend
│                                             │
│  ④ Pilih Ruangan                            │
│  ┌──────────────────────────────────────┐   │  ← locked
│  │  [icon door]  Pilih ruangan...       │   │
│  └──────────────────────────────────────┘   │
│     Pilih waktu terlebih dahulu             │
│                                             │
├─────────────────────────────────────────────┤
│  Rp 85.000          [Lanjut ke Pembayaran]  │  ← disabled
└─────────────────────────────────────────────┘
```

### State D — All four sections filled, CTA enabled

```
┌─────────────────────────────────────────────┐
│ ← Pilih Jadwal & Terapis                    │
├─────────────────────────────────────────────┤
│  Refleksi Kaki  •  60 mnt  •  Rp 85.000    │
├─────────────────────────────────────────────┤
│  ① Tanggal  [Kamis, 1 Mei 2026]  [✓ Ubah]  │
│  ② Terapis  [Sari Dewi]          [✓ Ubah]  │
│  ③ Waktu    [10.00 – 11.00]      [✓ Ubah]  │
│                                             │
│  ④ Pilih Ruangan                            │
│  ┌──────────────────────────────────────┐   │  ← selected, highlighted
│  │ [photo]  Ruang Melati        [✓]    │   │
│  │  Single  •  Kapasitas 1             │   │
│  │  Aromaterapi  •  AC  •  Dim light   │   │
│  └──────────────────────────────────────┘   │
│  ┌──────────────────────────────────────┐   │  ← occupied, disabled
│  │ [photo]  Ruang Mawar         [×]    │   │
│  │  Couple  •  Kapasitas 2  •  ...     │   │
│  │  Sudah terisi di jam ini            │   │  ← inline reason
│  └──────────────────────────────────────┘   │
│                                             │
├─────────────────────────────────────────────┤
│  Rp 85.000          [Lanjut ke Pembayaran ▶]│  ← ENABLED, primary color
└─────────────────────────────────────────────┘
```

---

## 3. Section-by-Section Spec

---

### Section ① — Tanggal

#### Layout

A single tappable row (not a card, just an `InkWell`-wrapped `Container`) acting as a date display and trigger. Height: 56dp. Horizontal padding: 16dp. Corner radius: 12dp. Border: 1dp `colorScheme.outlineVariant`. Background: `colorScheme.surface`.

Left side: `Icons.calendar_today_outlined` (20dp, `cs.onSurfaceVariant`) + 12dp gap + formatted date string.

Right side: when a date is selected, a small teal "Ubah" text link (`labelMedium`, `cs.primary`) replaces the default right-arrow chevron.

Date format: "Kamis, 1 Mei 2026" — use Indonesian locale long format, capitalised day name.

Tapping anywhere on the row opens `showDatePicker()` with `initialDate: today`, `firstDate: today`, `lastDate: today + 60 days`. The date picker uses the app's `ThemeData` so it inherits the teal primary automatically.

#### Selected state

- Border: 2dp `cs.primary`
- Background: `cs.primaryContainer` at 30% opacity (use `cs.primaryContainer.withAlpha(77)`)
- Leading icon color: `cs.primary`
- Trailing: "Ubah" link in `cs.primary`
- No elevation change (this is a row, not a card)

#### Disabled state

N/A — date is always active. Today is auto-selected so the section is never in a "pick something" blocked state.

#### Loading state

N/A — no network call needed for the date row itself.

#### Empty state

N/A.

#### Error state

N/A.

#### Micro-interactions

- On tap: standard Material ripple (`InkWell`).
- On date picker close with a new date: the row content cross-fades (150ms `AnimatedSwitcher`) from the old date to the new. Sections ②, ③, ④ reset and re-animate to their locked states if downstream selections become invalid.
- Cascade reset rule: changing the date clears selected slot and selected room (a different day may have different availability) but does NOT clear the therapist selection (therapist preference is date-independent).

---

### Section ② — Pilih Terapis

#### Layout

A full-width, one-card-at-a-time **PageView carousel** replacing the former vertical list. The carousel sits inside the parent `SingleChildScrollView` as a fixed-height block — it does not scroll independently; the parent page scrolls, but within the therapist section the user swipes horizontally.

**Carousel container:** full column width minus 16dp horizontal screen padding on each side. Height: 380dp (BS-T11 `carousel-card-height`). A 16dp peek of the next card bleeds past the right edge (BS-T11 `carousel-peek-width`), achieved by:
- Setting `PageView` `viewportFraction: 1.0 - (16 / screenWidth)` so the next card bleeds into view.
- Wrapping the carousel in a negative right-margin or `OverflowBox` to allow the bleed to escape the section padding on the right side only.

**Card anatomy (top to bottom, within each carousel card):**

```
┌──────────────────────────────────────────────┐
│                                              │   ↑
│   [Full-width photo, rounded top corners]    │  ~228dp  (≥60% of card height)
│                                              │   ↓
├──────────────────────────────────────────────┤
│  Full Name                          [✓/○]   │   ← titleLarge, w700; trailing
│  [Wanita] [Langsing] [163 cm] [58 kg]       │   ← chip row
│  Bio snippet, max 2 lines, ellipsis…        │   ← bodySmall, cs.onSurfaceVariant
│  ┌─────────────────────────────────────┐    │
│  │         Pilih [Nama]               │    │   ← explicit selection button
│  └─────────────────────────────────────┘    │
└──────────────────────────────────────────────┘
```

**Photo area:** fills the top 228dp of the card (60% of 380dp). `BorderRadius` on top corners only: 12dp. `BoxFit.cover`. For `photo_url` null or load failure: do NOT show a plain grey box. Instead, show a tinted background (`cs.secondaryContainer`) with the therapist's initials centered in `headlineLarge`, `fontWeight.w700`, `cs.onSecondaryContainer`. The initial is derived from `full_name` — first letter of each word, max 2 characters (e.g. "Sari Dewi" → "SD").

**Below the photo:**
- Name: `titleLarge`, `fontWeight.w700`, single line, `TextOverflow.ellipsis`.
- Trailing selection indicator (top-right of the name row): `Icons.check_circle` (24dp, `cs.primary`) when selected; `Icons.radio_button_unchecked` (24dp, `cs.outlineVariant`) when not selected.
- Chip row: a horizontal `Wrap` of compact pills. Order: gender ("Wanita"/"Pria"), build value, height ("163 cm"), weight ("58 kg"). If height or weight is null, omit that chip. Use same pill spec as before: horizontal padding 8dp, vertical 3dp, radius 999, background `cs.secondaryContainer`, text `cs.onSecondaryContainer`, `labelSmall`, `fontWeight.w500`.
- Bio snippet: `bodySmall`, `cs.onSurfaceVariant`, max 2 lines, `TextOverflow.ellipsis`. Omit entirely if `bio` is null.
- "Pilih [Nama]" button: `FilledButton.tonal` spanning the full inner card width (minus 16dp padding on each side within the card). Label: "Pilih Sari Dewi". On the "Otomatis" card the label reads "Pilih Otomatis". This is the explicit selection trigger — do NOT rely solely on tapping the card body.

**Position counter badge:** overlaid top-right of the carousel block (outside the card, above it in the Z axis). Content: "1 / 8" format, `labelSmall`, `fontWeight.w600`, `cs.onPrimary` on a `cs.primary` background, `borderRadius: 999`, padding `horizontal: 8, vertical: 4`. Positioned using a `Stack` around the carousel widget. Updates on every page change via `PageController.addListener`.

**Indicator dots** below the carousel, centered horizontally:
- When therapist count (including "Otomatis") is 8 or fewer: individual filled dots. Active dot: 8dp diameter, `cs.primary`. Inactive dot: 6dp diameter, `cs.outlineVariant`. Gap between dots: 6dp. Dots animate width and color over 150ms on page change.
- When count exceeds 8: replace dots with a compact scrolling bar indicator (a thin 4dp-tall `LinearProgressIndicator`-style strip, `cs.outlineVariant` track, `cs.primary` fill, width proportional to position / total).

**Left/right arrow buttons** (one on each side of the indicator row, not overlaid on the card):
- Rendered only when `MediaQuery.of(context).navigationMode == NavigationMode.directional` OR on flutter web when hover capability is detected via `kIsWeb && !Platform.isAndroid && !Platform.isIOS` heuristic (see BS-T11).
- Style: `IconButton.filled` using `cs.surface` background and `cs.onSurface` icon, size 40dp, `Icons.chevron_left` / `Icons.chevron_right`. Left arrow disabled (`onPressed: null`) on the first card; right arrow disabled on the last card.
- These are the primary selection navigation affordance for pointer/keyboard users.

**"Otomatis" card (always first, position 0):** same overall 380dp card. Photo area replaced by a full-width 228dp `cs.primaryContainer` background with `Icons.auto_awesome` (64dp, `cs.primary`) centered and a subtitle "Biar sistem yang memilih" in `bodyMedium`, `cs.onPrimaryContainer`, centered below the icon. Name row: "Otomatis" in `titleLarge`, `fontWeight.w700`. No chip row. No bio. "Pilih Otomatis" button.

#### Discoverability mitigations

The photo-first one-card-at-a-time layout is optimised for decision quality (customers see each therapist's face clearly) at the cost of at-a-glance scanning (all options are never simultaneously visible). The following four affordances are required to compensate:

1. **Position counter badge ("1 / 8"):** top-right of the carousel block, always visible, tells the user exactly how many therapists exist and where they are in the sequence. Updates immediately on every swipe.

2. **Page indicator dots (or bar indicator):** below the carousel. Provides spatial awareness and a secondary tap target for navigating by position. Dot layout for <= 8 therapists; compact bar for > 8. Always rendered — never hidden.

3. **Next-card bleed (~16dp peek):** the right edge of the next card is always visible past the current card. This is the primary gestural affordance signalling "you can swipe right." Implemented via `viewportFraction` less than 1.0 (see BS-T11). Ensures discoverability even for users who have never encountered a PageView pattern before.

4. **Left/right arrow buttons:** rendered alongside the indicator for desktop browser and accessibility navigation contexts. Hidden via MediaQuery on touch-only devices. Ensures full keyboard and mouse operability on Flutter web, satisfying WCAG 2.1 SC 2.1.1 (Keyboard).

#### Selection model

**Landing on a card by swiping does NOT auto-select it.** The customer is browsing; they may swipe through all therapists before deciding. Selection requires an explicit tap on the "Pilih [Nama]" button at the bottom of the visible card.

When the button is tapped:
- Apply BS-T8 selected-card recipe: 2dp `cs.primary` border, `cs.primaryContainer` background, scale-pop micro-animation (`scaleAnimation`: 1.0 → 1.03 → 1.0, 200ms, `Curves.easeOut`).
- Update the position counter badge styling: when the currently visible card is selected, the badge changes to `cs.primaryContainer` background + `cs.onPrimaryContainer` text (subtle confirmation without blocking content).
- Tapping "Pilih" on an already-selected card does nothing — selection is locked until the user swipes to a different card and taps "Pilih" there.

The trailing indicator icon (`check_circle` vs `radio_button_unchecked`) on each card reflects the global selection state regardless of which card is currently visible. This provides feedback when the user swipes away from a card they already selected.

#### Selected state (BS-T8 applied to carousel card)

- Card border: 2dp `cs.primary`
- Card background: `cs.primaryContainer` (full, not transparent)
- Card name text: `cs.onPrimaryContainer`
- Trailing icon: `Icons.check_circle` (24dp, `cs.primary`)
- Chips: background switches to `cs.primary` at 20% alpha, text to `cs.primary`
- Elevation: 4 (BS-T3)
- Scale-pop on tap: 1.0 → 1.03 → 1.0, 200ms, `Curves.easeOut` (wrapped in `AnimatedScale` or a short `Tween<double>` driven by an `AnimationController`)
- Badge: `cs.primaryContainer` background + `cs.onPrimaryContainer` text when the current page is the selected therapist

#### Loading state

Replace the carousel slot with a single shimmer card skeleton of the same 380dp height and full-width dimensions. The skeleton shows:
- A top rectangle (228dp, `cs.surfaceContainerHighest`, top-radius 12dp) representing the photo area.
- Two shimmer lines below (~16dp tall, `cs.surfaceContainerHighest`, varying widths) representing the name and chip row.
- A shimmer rectangle (~44dp tall) at the bottom representing the button.

Shimmer uses the same 1400ms opacity oscillation (`anim-shimmer-cycle`, BS-T6). No dots, no counter badge, no arrows during loading — their container space is reserved with empty `SizedBox` equivalents to prevent layout shift on load completion. `prefers-reduced-motion`: static placeholder color, no animation.

```
┌──────────────────────────────────────────────┐
│                                              │
│   [████████████████████ shimmer ████████]    │  ← 228dp photo area
│                                              │
├──────────────────────────────────────────────┤
│  [██████████████████]                        │  ← name shimmer
│  [███] [██████] [██████]                     │  ← chip row shimmer
│  [████████████████████████████████]          │  ← button shimmer
└──────────────────────────────────────────────┘
```

#### Empty state

When the branch has zero active therapists (only "Otomatis" would be shown), the carousel renders a single non-swipeable placeholder card:

```
┌──────────────────────────────────────────────┐
│                                              │
│   [Icons.person_search_outlined, 64dp,       │
│    cs.onSurfaceVariant]                      │
│                                              │
│   Cabang ini belum punya                     │
│   therapist tersedia                         │
│   (bodyMedium, cs.onSurfaceVariant,          │
│    centered, max 2 lines)                    │
│                                              │
└──────────────────────────────────────────────┘
```

No "Pilih" button. No counter badge ("—"). Indicator dot area shows a single inactive dot. Swipe disabled (`physics: NeverScrollableScrollPhysics()`). The "Otomatis" card is NOT shown in this state — if there are no therapists, the system also cannot auto-assign anyone.

#### Error state

When therapist list fetch fails, replace the carousel with a single card-shaped error block of the same 380dp height:

```
┌──────────────────────────────────────────────┐
│                                              │
│   [Icons.cloud_off_outlined, 48dp,           │
│    cs.error]                                 │
│                                              │
│   Gagal memuat                               │
│   (bodyMedium, cs.onSurfaceVariant)          │
│                                              │
│   [  Tap untuk coba lagi  ]                  │
│   (OutlinedButton, full width)               │
│                                              │
└──────────────────────────────────────────────┘
```

The retry button triggers re-fetch. No counter, no dots, no arrows. Swipe disabled.

#### Micro-interactions

- **Swipe / page change:** page snap with `PageView` default snap physics. Position counter badge content updates with `AnimatedSwitcher` (100ms crossfade). Indicator dot/bar position transitions at 150ms (`anim-card-select` duration). No haptic on swipe.
- **"Pilih" button tap on an unselected card:** scale-pop (1.0 → 1.03 → 1.0, 200ms, `Curves.easeOut`). Card border and background transition via `AnimatedContainer` (200ms). Haptic: `HapticFeedback.lightImpact()`. Page auto-scrolls so Section ③ top is visible using `Scrollable.ensureVisible` (200ms, `Curves.easeOut`).
- **"Pilih" button tap on an already-selected card:** no animation, no haptic, no state change.
- **Section unlock animation:** same as before — carousel container slides in from opacity 0 + `Offset(0, 0.06)` over 225ms, `Curves.easeOut`. Under `prefers-reduced-motion`: instant show.
- **Arrow button tap (desktop/web):** `PageController.animateToPage` with 250ms `Curves.easeInOut`. Mirrors the swipe experience at a slightly longer duration to feel deliberate on mouse/keyboard.

---

### Section ③ — Pilih Waktu

#### Layout

3-column grid of time slot chips. Each chip: minimum 100dp wide, 44dp tall (meeting touch target), `borderRadius: 8`. The grid uses `GridView` with `crossAxisCount: 3`, `crossAxisSpacing: 8`, `mainAxisSpacing: 8`, `childAspectRatio: 2.2`.

Chip label: time string "09.00" in `labelLarge`, `fontWeight.w600`, tabular figures. No end-time displayed inside the chip — the service duration context (shown in the summary pill at top) makes the end time derivable. Optionally show "–10.00" as a `labelSmall` line below if the designer judges it clearer; flag as open question.

Above the grid: a small subtitle row — `bodySmall`, `cs.onSurfaceVariant` — "Slot tersedia untuk [Sari Dewi] pada [Kamis, 1 Mei]". When "Otomatis" is selected: "Slot dengan minimal 1 terapis tersedia."

#### Selected state

- Background: `cs.primary`
- Text: `cs.onPrimary`
- Border: none (filled chip)
- Elevation: no extra shadow (chips don't use elevation)
- Leading check icon: `Icons.check` (14dp, `cs.onPrimary`) inline left of label

#### Disabled state (slot unavailable)

A slot is unavailable when `therapist_available == false` (per-therapist filter, post backend extension) OR `therapistsAvailableCount == 0 && roomsAvailableCount == 0` (aggregate, current model).

- Background: `cs.surfaceContainerHighest`
- Text: `cs.onSurface` at 38% opacity
- Border: 1dp `cs.outlineVariant` at 38% opacity
- Pointer: `IgnorePointer(ignoring: true)`
- Tooltip / Semantics label: "Penuh — tidak dapat dipilih" (screen reader; no visible tooltip on mobile)
- No ripple

Disabled chip MUST NOT display an inline text reason within the chip (too small). The reason is surfaced only via a Semantics label and a legend line below the grid: "Slot dengan tampilan redup sudah penuh atau tidak tersedia untuk terapis pilihan."

#### Loading state

Replace the grid with 9 skeleton chips (3×3 matrix of `cs.surfaceContainerHighest` rounded boxes, same 44dp height). Shimmer same approach as Section ②. Show only when an async fetch is in progress (i.e., after therapist or date changes, before new availability data arrives).

#### Empty state

When the API returns zero slots for the selected date + therapist combination:
```
[Icon: Icons.event_busy_outlined, 48dp, cs.onSurfaceVariant]
Tidak ada slot tersedia untuk terapis ini pada tanggal tersebut.
[TextButton: Coba tanggal lain]  ← taps Section ① row to reopen date picker
```
Centered within the section. No full-screen takeover.

When "Otomatis" is selected and zero slots exist (branch fully booked):
```
[Icon: Icons.event_busy_outlined]
Tidak ada slot tersedia pada tanggal ini.
[TextButton: Coba tanggal lain]
```

#### Error state

Small inline strip inside the section boundaries:
```
[Icons.wifi_off_outlined, 18dp, cs.error]  Gagal memuat slot.  [Coba Lagi]
```
`cs.errorContainer` background, `borderRadius: 8`, 12dp padding. "Coba Lagi" is a `TextButton` that invalidates the provider and re-fetches.

#### Micro-interactions

- Section unlock: same slide-fade animation as Section ② (225ms, easeOut). Scroll to Section ③ top automatically.
- Slot tap: ripple + 150ms background color transition (FilterChip's built-in selected state animation is appropriate here).
- Haptic: `HapticFeedback.lightImpact()`.
- When slot changes after a room was already selected: clear the room selection and show a transient `SnackBar` — "Waktu diubah. Pilih ruangan lagi." (2s duration).

---

### Section ④ — Pilih Ruangan

#### Layout

Vertical list of room cards, identical structural pattern to therapist cards but with different content. Card height: minimum 96dp (auto-expands for long amenity chips). Outer padding: 16dp.

Card anatomy:
- Photo tile: 80×80dp, `borderRadius: 8`, `BoxFit.cover`. Placeholder: `cs.surfaceContainerHighest` + `Icons.meeting_room_outlined` (36dp, `cs.onSurfaceVariant`).
- 12dp gap
- Text column (expanded):
  - Row 1: room name — `titleMedium`, `fontWeight.w600`
  - Row 2: room type chip + "Kapasitas: N" in `bodySmall`, `cs.onSurfaceVariant`
  - Row 3: amenity chips — `Wrap` with `spacing: 6, runSpacing: 4`. Each chip: same pill style as therapist body chips (secondary container). Cap at 3 chips shown + "+N lagi" overflow chip if there are more. Amenity chip height: 24dp (use `VisualDensity.compact` + `MaterialTapTargetSize.shrinkWrap`).
- Trailing: selection indicator

Room type label mapping:
- `single` → "Single"
- `couple` → "Couple"
- `vip` → "VIP"
- Any other string: capitalise first letter and display as-is.

First card is "Otomatis (Pilihkan Saja)". Visual: same layout, photo tile replaced by a 80×80dp `cs.primaryContainer` tile with `Icons.hotel` or `Icons.king_bed_outlined` (32dp, `cs.primary`) centered. Name: "Otomatis". Subtitle: "Kami pilihkan ruangan yang tersedia untukmu."

#### Selected state

Identical to therapist card selected state: `cs.primaryContainer` background, 2dp `cs.primary` border, `Icons.check_circle` trailing, elevation 4.

#### Disabled state (room occupied at chosen slot)

A room is disabled when its `id` is NOT in the `available_room_ids` array returned by the extended availability API for the chosen slot. Until the API extension lands (flag as open question), the frontend must disable rooms conservatively by checking `roomsAvailableCount == 0` for the slot (which hides all rooms, not per-room).

- `Opacity`: 0.38 on the entire card
- `IgnorePointer(ignoring: true)`
- Trailing: `Icons.block_outlined` (20dp, `cs.onSurfaceVariant`)
- Inline reason below the card (outside card bounds, inside card margin): `bodySmall`, `cs.error` — "Sudah terisi di jam ini"
- No ripple, focus excluded

Design principle: the disabled-reason copy must appear inline, not in a tooltip, because mobile users do not have hover.

#### Loading state

2 skeleton cards (auto + 1 room placeholder). Same shimmer as other sections.

#### Empty state

Zero rooms available at the chosen slot. Show the "Otomatis" card only (system will assign an available room at booking time). Below it:
```
[Icons.meeting_room_outlined, 32dp, cs.onSurfaceVariant]
Semua ruangan sudah terisi di jam ini.
Pilih ruangan lain atau ubah waktu, atau pilih Otomatis dan kami yang atur.
```
The user can still proceed by selecting "Otomatis" (system will attempt to find a room at booking time; if none available, the booking API returns a conflict error which the payment screen handles).

Zero rooms registered at branch (empty branch config):
```
[Icons.meeting_room_outlined, 48dp, cs.onSurfaceVariant]
Belum ada ruangan terdaftar di cabang ini.
Hubungi cabang untuk informasi lebih lanjut.
```
In this case, do NOT show the "Otomatis" card either — there is genuinely no room. CTA remains disabled.

#### Error state

Same inline strip pattern as Section ③:
```
[Icons.wifi_off_outlined]  Gagal memuat ruangan.  [Coba Lagi]
```

#### Micro-interactions

- Section unlock: same slide-fade 225ms. Scroll into view.
- Room tap: ripple + elevation/color transition 150ms.
- Haptic: `HapticFeedback.lightImpact()`.

---

## 4. Sticky CTA Bar Spec

The CTA bar is a fixed bottom bar using Flutter's `bottomNavigationBar` slot (elevation 8). It respects `SafeArea` for home-bar insets. It is always visible regardless of scroll position — it does not disappear when scrolling up.

Layout: `Material > SafeArea(top: false) > Padding(fromLTRB(16, 12, 16, 12)) > Row`.

### Sub-state: Nothing or only date selected

```
[  Rp 85.000                    ]  [  Lanjut ke Pembayaran  ]
  bodyMedium, cs.onSurfaceVariant    disabled: cs.primary at alpha 80
```
Price is always visible (from the service selected earlier). Button is disabled (`onPressed: null`). No label explaining why — progressive disclosure in the page content itself communicates the requirement. Do not add a tooltip or banner.

Button style: `ElevatedButton` with explicit colors (matching the existing `_BottomNavBar` pattern):
- Disabled: `backgroundColor: cs.primary.withAlpha(80)`, `foregroundColor: cs.onPrimary.withAlpha(180)`
- This ensures the button shape and text remain visible but clearly inactive, which matches WCAG SC 1.4.3 exception for disabled controls.

### Sub-state: Partial selection (1–3 of 4 complete)

Exactly the same as above. No intermediate state copy changes.

### Sub-state: All four sections complete

```
[  Rp 85.000                    ]  [▶ Lanjut ke Pembayaran  ]
  titleMedium bold, cs.primary       FilledButton, full color, active
```

Left side upgrades to `titleMedium` + `fontWeight.w700` + `cs.primary` color when all fields are filled, providing a satisfying visual reward. The price uses `font-variant-numeric: tabular-nums`.

Button label: "Lanjut ke Pembayaran". No icon inside the button label — the label is clear enough. Icon can be added as `FilledButton.icon` with `Icons.arrow_forward` if the product owner prefers.

Button `minimumSize: const Size(double.infinity * 0.55, 56)` — it takes roughly 55% of the bar width, leaving space for the price on the left.

CTA activation animation: when the last field is filled, the button transitions from disabled to enabled via a 200ms color animation. The price label also animates from `cs.onSurfaceVariant` to `cs.primary` over 200ms using `AnimatedDefaultTextStyle`.

### Add-on total

If the user chose add-ons in the previous step, the price displayed is the running total (service + add-ons). The breakdown is not expanded here — that is the job of the payment screen. A simple `bodySmall` line below the price: "Termasuk tambahan" if `selectedAddonIds.isNotEmpty`.

---

## 5. Accessibility

### Tap targets

- All interactive cards: minimum height 96dp for room cards (easily above 48dp target). Therapist carousel cards are 380dp tall — far above target.
- Therapist carousel: the primary tap target is the "Pilih [Nama]" `FilledButton.tonal` at the bottom of each card. It spans the full card inner width and must be at least 44dp tall (`minimumSize: Size(double.infinity, 44)`). The card body is NOT independently tappable as a selection trigger — only the explicit button is. (The card is swipeable, not tappable-to-select, so this avoids accidental selection on swipe.)
- Room cards: the entire card surface is the tap target via `InkWell` wrapping the `Card`'s child.
- Date row: 56dp tall, full width.
- Slot chips: 44dp tall minimum — enforced via `childAspectRatio: 2.2` in the 3-column grid. On very narrow screens (< 360px), drop to `crossAxisCount: 2` so chips remain >= 44dp wide.
- CTA button: 56dp tall, explicit in `minimumSize`.
- "Ubah" / "Coba Lagi" text links: minimum 44dp touch target via `Padding` or `minimumSize` on `TextButton`.

### Color contrast

- Card name text (`titleMedium`) on `cs.surface`: inherits Material 3 contrast guarantee (≥ 4.5:1 for body text on surface).
- Selected card: `cs.onPrimaryContainer` on `cs.primaryContainer` — Material 3 color system guarantees ≥ 4.5:1.
- Disabled text (`cs.onSurface` at opacity 38%): this intentionally falls below 4.5:1 per WCAG SC 1.4.3 exception for disabled inactive controls. Must NOT be the only indicator of the disabled state — combine with `IgnorePointer`, `lock_outline` icon, and programmatic `focusable: false`.
- Slot chip disabled: same exception applies. The legend text below the grid provides a text explanation for users who cannot perceive the color difference.
- Inline disabled-reason copy (`cs.error` on white): `cs.error` in Material 3 Teal scheme is `#B3261E` on white → contrast ratio ~5.1:1, passes AA.
- CTA bar background: `cs.surface` white; price text `cs.primary` teal → Material 3 guarantees ≥ 3:1 for large text (the price is rendered at `titleMedium` 16sp bold, qualifying as large text under WCAG).

### Keyboard / focus order (Flutter web)

Tab order flows: date row → left-arrow button (if visible) → "Pilih" button of current carousel card → right-arrow button (if visible) → slot chips (row-major order, left to right, top to bottom) → room cards → CTA button. The carousel itself is not in the tab order — navigation is via the arrow buttons. The "Pilih" button always reflects the currently-visible card; as the user navigates via arrow buttons, the focused "Pilih" button updates to the new card's label. Locked sections must be fully excluded from the tab order (`ExcludeSemantics` + `FocusTraversalPolicy` to skip). When a section unlocks, the first arrow button (or the "Pilih" button if arrows are hidden) receives focus automatically (`FocusScope.of(context).requestFocus()`).

Escape key: no modal-close behavior needed on this screen (no overlays except the date picker which handles its own escape).

### Screen reader semantics

Therapist carousel region:
```
Semantics(
  label: 'Pilih terapis. ${currentIndex + 1} dari ${total} ditampilkan.',
  container: true,
)
```

Each carousel card's "Pilih" button:
```
Semantics(
  label: isSelected
      ? 'Terapis Sari Dewi, wanita, langsing. ${bio snippet}. Dipilih.'
      : 'Terapis Sari Dewi, wanita, langsing. ${bio snippet}. Belum dipilih. Pilih Sari Dewi.',
  button: true,
  onTap: isSelected ? null : selectTherapist,
)
```

Arrow buttons:
```
Semantics(label: 'Terapis sebelumnya', button: true)   // left arrow
Semantics(label: 'Terapis berikutnya', button: true)   // right arrow
```

Position counter badge:
```
Semantics(label: '${currentIndex + 1} dari ${total} terapis', excludeSemantics: true)
```

Each slot chip:
```
Semantics(
  label: isAvailable ? 'Slot 09.00, tersedia' : 'Slot 09.00, tidak tersedia',
  button: isAvailable,
  excludeSemantics: true,  // suppress child text duplication
)
```

Each room card:
```
Semantics(
  label: '${room.name}, ${roomType}, kapasitas ${capacity}. ${amenities}. ${isOccupied ? "Sudah terisi di jam ini." : "Tersedia."}',
  button: !isOccupied,
)
```

Locked section placeholders:
```
Semantics(
  label: 'Pilih ${sectionName}. Tersedia setelah ${previousSection} dipilih.',
  excludeSemantics: true,
)
```

### Reduced motion

All `AnimatedContainer`, `AnimatedOpacity`, `AnimatedSwitcher`, and explicit `animate...Page` calls must check `MediaQuery.of(context).disableAnimations` and either skip the animation entirely or reduce it to a crossfade (no translate). Implement via a helper:

```
bool _animate(BuildContext context) =>
    !MediaQuery.of(context).disableAnimations;
```

When `_animate == false`: section unlock shows immediately (no slide). Date crossfade becomes instant. Shimmer becomes static placeholder color.

---

## 6. Open Questions

0. **Design tradeoff — photo-first carousel vs vertical list (resolved, mitigations documented).** The switch from a vertical list to a one-card-at-a-time carousel maximises decision quality: customers can see each therapist's face at full card width rather than a 72dp thumbnail, which directly addresses the customer feedback ("ingin melihat foto terapisnya"). The cost is that all options are never simultaneously visible, which risks the customer not discovering all available therapists. This tradeoff is accepted because: (a) the four discoverability mitigations (counter badge, indicator dots, peek bleed, arrow buttons) together provide strong affordance that more cards exist; (b) a therapist selection is a high-involvement decision — quality of evaluation per card outweighs the cost of one additional swipe per therapist; (c) the original vertical list's 72dp photo was too small to meaningfully drive physical preference decisions. If subsequent usability testing shows customers are completing the carousel at fewer than 2 full passes (i.e., not swiping past the first card), revisit whether a 2-column photo grid is a better middle ground. Flag for product review post-launch.

1. **Large therapist lists in the carousel.** If a branch has 20+ therapists, the carousel becomes tedious to swipe through. The current carousel design handles up to ~15 therapists comfortably (dots switch to a bar indicator above 8). For branches with consistently large rosters, consider a secondary "Lihat semua" fallback that opens the original vertical list in a bottom sheet, allowing fast scanning. This is out of scope for the current iteration. The data model already bundles therapists in `BranchDetail`; if list size grows, `go-expert` will need a separate `GET /branches/:id/therapists?page=` endpoint. Flag for orchestrator.

2. **Per-room availability in the current API.** `AvailabilitySlot` currently carries only `rooms_available_count` (a count, not IDs). To correctly disable specific rooms in Section ④, the API needs to return `available_room_ids: []` per slot, or a separate endpoint `GET /branches/:id/rooms/availability?slot_start=&therapist_id=`. Until this exists, the frontend can only disable ALL rooms when `rooms_available_count == 0`, not individual ones. `go-expert` needs to extend the availability endpoint. Flag for orchestrator.

3. **Per-therapist slot availability.** Similarly, `AvailabilitySlot` does not carry a per-therapist availability flag. When the user picks therapist X and then views slots, which slots are greyed out? The current aggregate `therapistsAvailableCount` is a count across all therapists, not specific to one. The backend needs `GET /branches/:id/availability?service_id=&date=&therapist_id=` to return therapist-specific slot availability. Flag for `go-expert` via orchestrator.

4. **End time in slot chips.** Should the slot chip display just "09.00" or "09.00 – 10.00"? The service duration is known (shown in the summary pill), so an expert user can derive the end time. For first-time users, showing the range reduces ambiguity. This is a product decision — either is implementable. Currently specced as start-time only; add a `labelSmall` end time below if product approves.

5. **Bio truncation behavior.** Bio is shown as max 2 lines with ellipsis in the card. Should there be a "Lihat selengkapnya" expansion, or a tap-to-bottom-sheet detail? Currently out of scope for this flow; bio is read-only informational.

6. **Cascade reset on date change.** Specced as: date change clears slot + room but NOT therapist. Is this the right decision? A therapist's availability can differ day to day, so technically their slots will change. However, keeping the therapist selection on date change gives better UX continuity. The slot grid re-fetches automatically, showing which slots are valid for that therapist on the new date.

7. **"Pilih Saja" room card vs required room selection.** In the existing wizard, room is optional (null = auto). This spec maintains that behavior — the "Otomatis" card counts as a valid selection, enabling the CTA. Is there a business rule where certain services require a specific room type? If so, go-expert needs a `room_type_required` field on the service model.

8. **Scroll-to-newly-unlocked-section behavior.** The spec calls for auto-scroll to the top of the next section on each selection. On very short devices (< 600px height) this could be jarring if the therapist list is long. Consider constraining auto-scroll to only trigger when the next section is out of the viewport (not visible at all), rather than always scrolling.

---

## 7. Asset List

### Icons (Material / Flutter built-in — no custom assets needed)

| Usage | Icon name |
|---|---|
| Date row leading | `Icons.calendar_today_outlined` |
| Date picker trigger | `Icons.edit_calendar_outlined` |
| Auto-therapist tile | `Icons.auto_awesome` (64dp in carousel card) |
| Person photo placeholder (carousel, initials fallback) | Initials text — no icon used |
| Person photo placeholder (error/null fallback icon) | `Icons.person_outlined` (used only in small contexts, not carousel) |
| Carousel left arrow | `Icons.chevron_left` |
| Carousel right arrow | `Icons.chevron_right` |
| Carousel unselected trailing indicator | `Icons.radio_button_unchecked` |
| Empty state — no therapists | `Icons.person_search_outlined` |
| Error state — carousel | `Icons.cloud_off_outlined` |
| Room photo placeholder | `Icons.meeting_room_outlined` |
| Auto-room tile | `Icons.king_bed_outlined` |
| Slot chip selected check | `Icons.check` (inline, 14dp) |
| Card selected trailing | `Icons.check_circle` |
| Card disabled trailing | `Icons.block_outlined` |
| Section locked trailing | `Icons.lock_outline` |
| Empty state — slot | `Icons.event_busy_outlined` |
| Empty state — room | `Icons.meeting_room_outlined` |
| Error state | `Icons.wifi_off_outlined` |
| CTA arrow (optional) | `Icons.arrow_forward` |
| Section unlock indicator | `Icons.expand_more` → hidden after unlock |

### Fonts

No new fonts. The existing app typeface (system default via Material 3) is sufficient. Tabular figures are enabled via `fontFeatures: [FontFeature.tabularFigures()]` on price and time strings.

### Images

- Therapist photos: loaded via `CachedNetworkImage` from `photo_url` (nullable). Placeholder and error widget already specced above.
- Room photos: same. Use `CachedNetworkImage`.
- No decorative illustrations needed — the empty states use Material icons at large scale (48–64dp) which is consistent with the payment screen's `Icons.timer_off_outlined` empty state pattern already in production.

---
