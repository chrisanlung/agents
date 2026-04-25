# ADR 0014 — Phase 5: Booking Engine + Customer Mobile App

- **Status:** Accepted
- **Date:** 2026-04-25
- **Deciders:** owner (2026-04-25), orchestrator
- **Extends:** ADR 0002 (booking double-booking prevention), ADR 0009 (Phase 4 master data), ADR 0011 (storage abstraction), ADR 0012 (rooms)

---

## 1. Context

Phase 1–4 built every primitive needed to model spa/clinic operations: tenants, branches, rooms, therapists, services, add-ons, availability. Phase 5 turns those primitives into a **revenue-generating customer flow** — a mobile app where end-customers discover branches, book treatments, pay online, and receive a code redeemed at the cabang.

This is the largest phase by surface area: a new platform (Flutter), a new payment integration (Midtrans, dummy in Phase 5), the first customer-facing API surface (no auth context to lean on), and the booking-engine logic that ties together every Phase 4 entity. The double-booking prevention contract was set in ADR 0002 (DB-level GiST exclusion); this ADR consumes it.

## 2. Customer journey (binding)

```
Open app (no login, guest)
  ↓
Allow location → "Cabang terdekat dari kamu" (sorted by distance)
  ↓
Search by name/area, filter by service category + open-now, ⭐ favorite
  ↓
Pick branch → branch detail (foto, alamat, jam buka, layanan, terapis, ruangan)
  ↓
Book: pick service (mandatory) → add-ons (optional, multi-select)
        → room (auto-assigned by default, customer override optional)
        → therapist (auto-assigned by default, customer override optional)
        → time slot (mandatory; respects therapist availability + branch hours
                     + no-double-booking constraint)
  ↓
Customer info: nama + phone + email (no account creation)
  ↓
Pay (Midtrans Snap; in Phase 5 a DUMMY stub returns "paid" instantly —
    integrasi real Midtrans saat keys tersedia, no code change to flow)
  ↓
Booking confirmed: receive QR + 8-char alphanumeric code (dipisah hyphen,
    contoh "B7K3-M2QF") + email konfirmasi dengan kode
  ↓
At cabang: ops staff scan QR or input code → booking checked-in →
    service rendered → ops mark "completed"
```

## 3. Decisions

### 3.1 Marketplace model (single app)

One Flutter app "Lustia" listed in Play Store + App Store. Customer browses **all** active tenants' branches. White-label per tenant is deferred to Phase 7+ (no demand signal yet).

Branding: app published by "Lustia" entity. Privacy policy / Terms by Lustia. Tenant info shown per-branch (logo placeholder for Phase 5; tenant.logo_key in Phase 6+).

### 3.2 No customer authentication (guest-only)

- No login, no register, no JWT for customer side.
- Customer info (`name`, `phone`, `email`) collected per booking — stored on `booking` row directly, no `customer` table.
- Booking code is the auth artefact: the customer's possession of the code = right to redeem.
- Favorites stored locally on device (Flutter `shared_preferences`) — uninstall = lose favorites. Acceptable MVP.
- **Anti-spam:** rate limit per IP at the customer booking endpoint + (future) reCAPTCHA / hCaptcha. Phase 5 ships rate-limit only.

### 3.3 Payment: Midtrans Snap (dummy in Phase 5)

- **Phase 5 implementation:** stub `MidtransClient` interface with a dummy adapter that:
  - `CreateTransaction(booking)` returns a fake `snap_token` and `redirect_url`.
  - `HandleNotification(payload)` accepts any payload and returns `paid`.
  - Booking transitions from `pending_payment → paid` immediately on dummy "checkout".
- **Real Midtrans adapter:** stub interface, real implementation when keys arrive. Same interface, same flow, no code change at consumer site.
- **Snap UI vs Core API:** Snap (drop-in widget) when real keys arrive. Phase 5 dummy = single screen "Bayar (DUMMY)" button in Flutter.
- **Webhook:** stubbed endpoint `POST /api/v1/customer/payments/webhook` accepts dummy payload, marks booking paid. Real adapter later validates Midtrans signature.

### 3.4 Booking code: QR + 8-char alphanumeric

- Format: `[A-Z2-7]{4}-[A-Z2-7]{4}` (Crockford Base32 minus 0/O/1/I/L for legibility). Example: `B7K3-M2QF`.
- Random source: `crypto/rand` 5 bytes → 8 chars. ~10^12 keyspace per format → collision odds <10^-6 at 10k bookings.
- Stored on `booking.code` column, `UNIQUE` index.
- App also generates QR encoding `code` (NOT `booking.id`). Ops portal scans → backend lookups by code.
- Email sent on confirmation with code visible (fallback if app uninstalled).

### 3.5 No cancel / no refund

- Customer cannot cancel booking once paid. App makes this explicit in T&C + before payment confirmation.
- Tenant (ops/branch_admin) **can** force-cancel from ops portal with reason (returns slot to availability, marks booking `cancelled`, no refund). Phase 5 has no refund path.
- This simplifies payment flow: no Midtrans refund API, no partial refund calc, no race between cancel + check-in.

### 3.6 Auto-assign with optional override (therapist + room)

- Customer required to pick: service, slot start time.
- Customer optional: therapist (default = "Pilih saja"), room (default = "Pilih saja").
- Auto-assign algorithm (server-side):
  - Filter therapists who: (a) work at this branch, (b) can perform this service (via `therapist_service`), (c) have availability for the slot, (d) no double-booking.
  - Pick the one with the lowest sort_order then lowest UUID (deterministic).
  - Filter rooms: (a) at this branch, (b) capacity ≥ 1 (Phase 5 has no group bookings), (c) no double-booking. Pick lowest sort_order.
  - If no candidate exists → return `409 Conflict` "Tidak ada terapis/ruangan tersedia di slot ini."
- Manual override: customer-supplied IDs replace auto-assign; backend still validates the same constraints.

### 3.7 Manual no-show flag

- Slot does NOT auto-release when customer doesn't arrive. Slot stays blocked until end-of-slot.
- Ops staff manually flags `status = no_show` after the slot ends.
- Exclusion constraint partial predicate already excludes `cancelled` and `no_show` from blocking — see ADR 0002.

### 3.8 Search + filter (mobile MVP)

- **Geolocation primary:** request location permission, sort branches by Haversine distance ascending.
- **Search by:** branch name (substring) + city/area (substring). Backend: `pg_trgm` indexes on `branch.name` + `branch.city` (mirror tenant search pattern from Phase 4 polish).
- **Filter:** service category (`Pijat`/`Facial`/`Nail`/etc — derived from `service.category` distinct values per branch), open-now (`now()` falls within branch operational_hours JSON).
- **Defer:** price range, rating, advanced filter, map view.

### 3.9 Geo schema additions

Migration 24 adds:
- `branch.latitude` `NUMERIC(9,6)` NULL — value range -90..90
- `branch.longitude` `NUMERIC(9,6)` NULL — value range -180..180
- CHECK constraints for valid range
- Index: `branch_geo_idx` on `(latitude, longitude)` partial WHERE both not null AND deleted_at IS NULL
- Use `earthdistance` extension (cheaper than full PostGIS for "nearest" queries; adequate at our scale): `CREATE EXTENSION IF NOT EXISTS cube; CREATE EXTENSION IF NOT EXISTS earthdistance;`
- Branch form in tenant-admin gets a map picker (or fallback: lat/lng input fields) — Phase 5 ships fallback fields; map picker = Phase 6 polish.

### 3.10 Notifications

- **Email confirmation only** for Phase 5: subject "Booking dikonfirmasi" with code + branch + slot + service. Already-existing email infra (Mailpit dev / SMTP prod) — no new dep.
- **Defer:** push notification (FCM), SMS, WhatsApp.
- **Reminder H-1:** deferred to Phase 6 (cron infra not in repo yet).

### 3.11 Concierge mode (ops books on behalf)

- Ops portal extends with a "Buat booking baru" page: same flow as customer app, but actor = ops staff JWT.
- Customer info entered manually by ops; payment flagged `paid_at_venue` (skip Midtrans for concierge — ops collects cash/QRIS at front desk separately).
- Phase 5 includes this — useful for elderly customer / phone-in. Backend endpoint distinguishes by JWT scope: `customer` (no JWT, public) vs `tenant` (ops staff JWT) — different validation rules.

### 3.12 Operator-side changes (web portals)

**Ops portal (port 3003):**
- New "Booking" tab: today's list, status filter, time-slot calendar view (optional Phase 5 polish).
- "Check-in" flow: scan QR camera (browser `MediaDevices` API) OR type code → backend lookup → confirm → status `checked_in`.
- "Tandai selesai" / "Tandai no-show" buttons per booking row.
- New: "Buat booking baru atas nama customer" (concierge mode).

**Tenant admin (port 3002):**
- New "Booking" page: cross-branch list, filter (date range, branch, status, service), CSV export deferred.
- Cancel-with-reason action.
- New: "Laporan" page — total bookings, revenue (sum of paid bookings), no-show rate per branch — basic metrics only.

**Platform admin (port 3001):**
- No changes Phase 5. Booking is tenant-scoped.

### 3.13 Booking lifecycle (state machine)

```
pending_payment ──pay──▶ paid ──checkin──▶ checked_in ──complete──▶ completed
        │                  │                                              
        └─expire(15m)─▶ expired           
                           │                                              
                           ├─ops cancel──▶ cancelled                      
                           │                                              
                           └─slot ends, ops flags──▶ no_show              
```

- `pending_payment` (15min TTL — slot reserved tentatively, soft-locked, NOT a confirmed booking yet)
- `paid` (after payment confirms — slot hard-locked, exclusion constraint applies)
- `checked_in` (ops scanned code on the day)
- `completed` (ops marked done after service)
- `expired` (15min passed without payment — slot released)
- `cancelled` (ops force-cancel)
- `no_show` (slot ended, customer did not arrive)

Exclusion constraint partial predicate excludes `pending_payment`, `expired`, `cancelled`, `no_show` from blocking — only `paid`, `checked_in`, `completed` count as "booked".

**Soft-lock during pending_payment:** to prevent double-attempts during the 15min payment window, treat `pending_payment` as blocking too — but with a separate timer that releases on expiry. Implementation: **include `pending_payment` in the exclusion predicate** + a background job (or lazy: on every list-availability call) that sweeps expired pending bookings to `expired` status. Trade-off: simpler than a parallel "soft-reservation" table. Default: lazy sweep.

### 3.14 Non-goals (deferred)

- Push notifications (FCM)
- SMS / WhatsApp notifications
- Customer accounts / login / history
- Refund / cancellation by customer
- Real Midtrans integration (keys not yet acquired)
- Reschedule (Phase 6 — cancel + new booking workaround)
- Rating & review
- Loyalty / membership / points
- Tenant-config policies (refund tiers, custom slot duration, etc.)
- Map picker for branch lat/lng in tenant-admin (use input fields)
- Multi-photo gallery per branch / service / room
- Branch operating hours per-day exceptions (holidays)
- Therapist time-off override (Phase 4 weekly pattern is the only primitive)
- Group bookings / multiple customers on one slot
- Add-on photos
- White-label per-tenant app
- Tenant logo in branch detail
- Offline mode for the Flutter app
- iOS-specific Apple Pay / Sign-in-with-Apple (App Store may require for any app with login — N/A for guest-only Phase 5)

### 3.15 Schema additions (binding for db-designer)

**Migration 24 — branch geo:**
- Add `latitude`, `longitude` to `branch`
- Add `cube` + `earthdistance` extensions
- Index `branch_geo_idx`
- Update DATA_MODEL.md `branch` section

**Migration 25 — booking core:**
- Tables: `booking`, `booking_addon` (M2M between booking and selected add-ons)
- `booking` columns: `id`, `tenant_id`, `branch_id`, `service_id`, `room_id`, `therapist_id`, `customer_name`, `customer_phone`, `customer_email`, `code` UNIQUE, `scheduled_start` TIMESTAMPTZ, `scheduled_end` TIMESTAMPTZ, `status` TEXT CHECK enum, `total_price_idr` BIGINT, `payment_method` TEXT (`midtrans` / `paid_at_venue` / null), `payment_reference` TEXT (snap_token or txn_id), `paid_at` TIMESTAMPTZ NULL, `cancelled_at` TIMESTAMPTZ NULL, `cancelled_by` UUID NULL, `cancel_reason` TEXT NULL, `created_at`, `updated_at`, audit columns
- GiST exclusion constraints (per ADR 0002):
  - `(therapist_id WITH =, tstzrange(scheduled_start, scheduled_end) WITH &&)` WHERE therapist_id IS NOT NULL AND status IN (`pending_payment`,`paid`,`checked_in`,`completed`)
  - `(room_id WITH =, tstzrange(scheduled_start, scheduled_end) WITH &&)` WHERE room_id IS NOT NULL AND status IN (...)
- Required extensions: `btree_gist`
- Indexes: `(tenant_id, branch_id, scheduled_start)`, `(code)` UNIQUE, `(status, scheduled_start)` for sweep job
- RLS: `tenant_id::text = current_setting('app.current_tenant', true)` + special read access for customer endpoints (see §3.17)
- Grants: SELECT, INSERT, UPDATE for `lustia_app`. No DELETE — soft-delete via `cancelled` status, no need for `deleted_at` column on booking.
- `booking_addon` columns: `booking_id` UUID FK CASCADE, `addon_id` UUID FK RESTRICT, `price_idr` BIGINT (snapshot at booking time — addon price could change later), PK `(booking_id, addon_id)`.

**Migration 26 — permissions:**
- New permission rows: `booking.read`, `booking.create`, `booking.cancel`, `booking.checkin`, `booking.complete`, `booking.no_show`
- UUID namespace `c0000000-0000-0000-0026-*` (verify clean before write — lesson from migration 18)
- Wire to roles:
  - super_admin: all
  - tenant_admin: all
  - branch_admin: all (branch-scoped via service-layer check)
  - therapist: read only (Phase 5 ops portal — therapist sees own schedule)
  - customer role: not applicable (no customer JWT)

### 3.16 API contract (binding for go-expert)

**Customer-facing public endpoints (no JWT):**

- `GET /api/v1/public/branches` — list branches with filters: `q` (name/city), `lat`, `lng` (sort by distance), `category`, `open_now`, `?limit=&page=`. Response includes branch detail, service summary, distance_meters if lat/lng given. **Tenant filter:** only branches of `tenant.status = 'active'` and `branch.status = 'active'` are returned.
- `GET /api/v1/public/branches/:id` — full branch detail incl. services, therapists list (active only), rooms list (active only).
- `GET /api/v1/public/branches/:id/availability?service_id=&date=` — compute available slots for a service on a given date. Returns array of `{start, end, therapists_available_count, rooms_available_count}`.
- `POST /api/v1/public/bookings` — create booking. Body: `{branch_id, service_id, addon_ids[], room_id?, therapist_id?, scheduled_start, customer_name, customer_phone, customer_email}`. Returns `{booking_id, code, snap_token, redirect_url, total_price_idr}`. Status starts `pending_payment`.
- `POST /api/v1/public/payments/webhook` — Midtrans webhook (dummy in Phase 5). Updates booking status. Verifies signature in real impl.
- `GET /api/v1/public/bookings/:code` — lookup by code. Returns booking detail (no customer email/phone — only what the holder needs to verify). For app's "my booking" view (since no account, app uses local storage of recent codes).

**Operator endpoints (tenant JWT):**

- `GET /api/v1/tenant/bookings` — list with filters (date, branch, status, service). Page/limit per ADR 0013.
- `GET /api/v1/tenant/bookings/:id` — detail.
- `POST /api/v1/tenant/bookings` — concierge create (ops books on behalf). Same shape as public create but with `payment_method: paid_at_venue` allowed.
- `POST /api/v1/tenant/bookings/:id/checkin` — ops scan code → checkin. Body `{code}` for verification.
- `POST /api/v1/tenant/bookings/:id/complete` — ops mark done.
- `POST /api/v1/tenant/bookings/:id/no-show` — ops mark no-show.
- `POST /api/v1/tenant/bookings/:id/cancel` — ops force-cancel. Body `{reason}`.
- `GET /api/v1/tenant/bookings/by-code/:code` — alternative scan lookup endpoint.
- `GET /api/v1/tenant/reports/bookings/summary?from=&to=&branch_id=` — aggregate metrics for tenant-admin reports page.

**Permission enforcement matrix:**

| Endpoint | Permission |
|---|---|
| `GET /tenant/bookings` | `booking.read` |
| `POST /tenant/bookings` (concierge) | `booking.create` |
| `POST /tenant/bookings/:id/checkin` | `booking.checkin` |
| `POST /tenant/bookings/:id/complete` | `booking.complete` |
| `POST /tenant/bookings/:id/no-show` | `booking.no_show` |
| `POST /tenant/bookings/:id/cancel` | `booking.cancel` |
| `GET /tenant/reports/bookings/*` | `booking.read` |

All operator endpoints with `:id` apply branch-scope check at service layer (mirror `RoomService` / `TherapistService` pattern).

### 3.17 Public endpoints + RLS

Customer endpoints have NO `app.current_tenant` context (no JWT). Two strategies:
- **(a)** Public endpoints set `app.current_tenant = '__public__'` and add an additive RLS policy `FOR SELECT TO lustia_app USING (current_setting('app.current_tenant', true) = '__public__' AND tenant.status = 'active' AND branch.status = 'active')` for the relevant tables.
- **(b)** Public endpoints use a separate connection role (`lustia_public`) with read-only grants and looser RLS.

**Decision: (a)** — single connection pool, single role, additive policy. Simpler to operate. Document in DATA_MODEL.md the `__public__` sentinel pattern (mirrors `__platform__` from ADR 0005).

For booking INSERT (customer side): backend validates `tenant_id` and `branch_id` from request body, sets `app.current_tenant = <resolved tenant_id>` for the INSERT transaction, then resets to `__public__` for the response read.

### 3.18 Mobile app (binding for flutter-expert)

**Scaffold:**
- `lustia/mobile/` Flutter project, Dart 3+, Flutter stable channel
- iOS + Android both supported from day one
- Android: minSdkVersion 21 (Android 5.0), targetSdkVersion latest
- iOS: deployment target iOS 13+
- State management: **Riverpod 2** (per `flutter-expert` agent default)
- Navigation: **go_router**
- HTTP: **dio** with interceptor for shared headers + retry
- Secure storage: not needed Phase 5 (no auth tokens)
- Local storage (favorites): `shared_preferences`
- QR generation: `qr_flutter`
- Geolocation: `geolocator` + `permission_handler`
- Maps: NOT in Phase 5 (defer; show distance only)
- Image: `cached_network_image`

**Screens:**
1. Splash + onboarding (welcome, location permission ask)
2. Branch list (sorted by distance + search bar + favorite tab toggle)
3. Branch detail (foto, info, daftar layanan, tombol "Booking")
4. Booking flow (multi-step wizard):
   - Pick service
   - Pick add-ons (optional)
   - Pick date + slot time
   - Pick therapist (optional, default auto)
   - Pick room (optional, default auto)
   - Customer info form
   - Confirmation summary + price total + "Bayar" CTA
5. Payment screen (Phase 5 = dummy "Bayar (DUMMY)" button → instant success)
6. Booking confirmation screen (QR + code + "Tunjukkan ini di cabang")
7. "Booking saya" (recent codes from local storage — list, tap to re-show QR)
8. Settings (terms, privacy, app version)

**Branding:**
- Lustia color palette (match web design tokens)
- Logo placeholder for Phase 5
- Splash screen + app icon — generated via `flutter_launcher_icons` from a single source asset

**Build flavors:**
- `dev` — points to `http://10.0.2.2:8080` (Android emulator) / `http://localhost:8080` (iOS simulator)
- `prod` — placeholder for production API URL

### 3.19 Review gates

- **security-expert:** customer endpoints are the first public attack surface. Threat model: rate limiting per IP, anti-enumeration on `branch_id` and `code` lookups, SSRF on payment webhook, prevention of negative-amount manipulation, JWT scope leakage between operator and public paths, RLS coverage of `__public__` policy.
- **qa-expert:** booking lifecycle state machine, exclusion constraint regression (concurrent insert test), 15min expiry sweep correctness, concierge vs public flow parity, code uniqueness probability test.
- **code-reviewer:** standard quality review on diffs.

### 3.20 CLEAN invariant

Migrations 1 → 25 (or 26 with permissions) produce a working schema. Migration 24 (geo) + 25 (booking) + 26 (permissions). No dev-only seeds for Phase 5 in the CLEAN chain — bookings are user-generated.

Optional dev seed migration 27: 5 sample paid bookings spanning today + tomorrow at acme-spa for E2E demo.

## 4. Open questions

1. **Booking expiry sweep — lazy vs cron?** Decision: lazy (sweep on every list-availability + booking-create call). Cron infra deferred. Trade-off: a forever-pending booking could block a slot if no one ever queries again. Acceptable since 15min expiry is short and the constraint only blocks legitimate parallel attempts (which already trigger sweep).

2. **Email confirmation for concierge bookings (paid_at_venue)?** Yes — same template, includes the code. Customer emails in concierge flow are entered by ops; assume valid.

3. **What happens if Midtrans dummy + real disagree?** Dummy ALWAYS returns paid; real may return failed. The interface returns `(status PaymentStatus, err error)` — adapter is responsible for mapping. Booking row only marks `paid` when status is explicitly `Paid`. Other statuses (`Pending`, `Failed`, `Expired`) keep booking in `pending_payment` until next webhook or manual intervention.

4. **Geocoding for branch address → lat/lng — automatic?** No, Phase 5 expects tenant admin to enter lat/lng manually (input fields). Auto-geocoding via Google Maps API / Nominatim deferred to Phase 6.

5. **Booking code regenerate on cancel?** No — `cancel` keeps the code (audit trail). New booking gets new code. UNIQUE constraint OK because cancelled rows still hold their code.
