# ADR 0012 — Ruangan (rooms) catalog

- **Status:** Accepted
- **Date:** 2026-04-25
- **Deciders:** owner (2026-04-25 — "kita butuh ruangan"), orchestrator
- **Extends:** ADR 0009 (Phase 4 master operational data), ADR 0011 (storage abstraction)

---

## 1. Context

The booking engine (Phase 5) needs to allocate a physical room to each booking. A "couple massage" needs a couple room, "VIP service" needs a VIP room. Without rooms, two bookings could collide on the same physical resource.

Phase 4 ships the **room catalog** (CRUD + photo). Phase 5 wires the booking-time uniqueness constraint `(room_id, time_window)`.

Owner requirements (2026-04-25):
- Term: **Ruangan** (Indonesian, neutral across spa/clinic/salon).
- Branch-scoped: each room belongs to exactly one branch.
- Customer-visible: customer picks a room when booking (Phase 5).
- No double-booking on (room, time slot) — enforced in Phase 5 booking engine.
- Manageable by `tenant_admin` (full, all branches) and `branch_admin` (only rooms at branches they belong to).
- UI: Operasional → Ruangan, list paginated 10, reorder up/down, photo support.

## 2. Decisions

### 2.1 Schema (migration 000021)

New table `room`:

| Column | Type | Constraints | Notes |
|---|---|---|---|
| `id` | `UUID` | PK | |
| `tenant_id` | `UUID` | NOT NULL, FK → `tenant(id)` RESTRICT | RLS anchor |
| `branch_id` | `UUID` | NOT NULL, FK → `branch(id)` RESTRICT | Room belongs to one branch. RESTRICT prevents silent deletion when a branch is removed — operator must reassign or delete rooms first. |
| `name` | `TEXT` | NOT NULL, length 1–120 | Display name e.g. "VIP 1", "Couple Room A" |
| `description` | `TEXT` | NULL, length ≤ 500 | Optional detail shown on customer booking screen |
| `room_type` | `TEXT` | NOT NULL, CHECK `IN ('single','couple','group','vip')` | Customer-facing category |
| `capacity` | `SMALLINT` | NOT NULL, CHECK 1–20, DEFAULT 1 | Max simultaneous occupants |
| `amenities` | `TEXT[]` | NOT NULL, DEFAULT `'{}'` | Free-text tags ("shower","aromaterapi","tv"). Service layer accepts/returns; no FK lookup |
| `photo_key` | `TEXT` | NULL, length ≤ 512 | Storage key (ADR 0011 §2.1). Set only via upload endpoint. |
| `is_active` | `BOOLEAN` | NOT NULL, DEFAULT true | Temporarily hide without deleting |
| `sort_order` | `INT` | NOT NULL, DEFAULT 0, CHECK 0–9999 | Display order in list + customer picker |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT now() | |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT now() | |
| `created_by` | `UUID` | NULL, FK → `"user"(id)` SET NULL | |
| `updated_by` | `UUID` | NULL, FK → `"user"(id)` SET NULL | |
| `deleted_at` | `TIMESTAMPTZ` | NULL | Soft-delete; no DELETE grant |

**Constraints:**
- `UNIQUE(branch_id, name) WHERE deleted_at IS NULL` — no duplicate name within a branch (different branches may share names).
- RLS: standard `tenant_id::text = current_setting('app.current_tenant', true)` for SELECT/INSERT/UPDATE.
- Grants: `SELECT, INSERT, UPDATE` to `lustia_app` (no DELETE — soft-delete only).
- `updated_at` trigger via existing `set_updated_at()`.

**Indexes:**
- `room_tenant_branch_idx`: `(tenant_id, branch_id, sort_order)` WHERE `is_active = true AND deleted_at IS NULL` — primary list/booking-engine scan.
- `room_tenant_id_idx`: `(tenant_id)` — RLS predicate.

**Why no booking-resource table yet:** `room.id` is sufficient as the booking-engine resource handle. Phase 5 will add `booking(room_id, starts_at, ends_at)` + a `tstzrange` exclusion constraint to guarantee no double-booking. Adding a separate `resource` abstraction now is YAGNI.

### 2.2 Permissions (UUID namespace `c0000000-0000-0000-0021-*`)

Lesson from migration 000018 collision: pick a fresh namespace that does not overlap with any existing permission UUID. The `0021` suffix matches the migration number for traceability.

| Permission | super_admin | tenant_admin | branch_admin | UUID |
|---|---|---|---|---|
| `room.read` | ✓ | ✓ | ✓ | `c0000000-0000-0000-0021-000000000001` |
| `room.create` | ✓ | ✓ | ✓ (own branch only — service-layer enforced) | `c0000000-0000-0000-0021-000000000002` |
| `room.update` | ✓ | ✓ | ✓ (own branch only) | `c0000000-0000-0000-0021-000000000003` |
| `room.delete` | ✓ | ✓ | ✓ (own branch only) | `c0000000-0000-0000-0021-000000000004` |

**Branch-scope enforcement:** RLS only enforces tenant isolation, not branch-level. The service layer (`RoomService`) must reject when caller is `branch_admin` and `room.branch_id ∉ caller.branches`. Same pattern as `TherapistService` (Phase 4).

### 2.3 API contract (new in API_CONTRACT.md §13)

All under `/api/v1/tenant/rooms`. Auth: `scope=tenant` + listed permission.

- `GET    /` — list (filter: `branch_id`, `is_active`, `room_type`; cursor pagination, default 10, max 200)
- `GET    /:id` — detail
- `POST   /` — create
- `PATCH  /:id` — update name/description/type/capacity/amenities/sort_order
- `PATCH  /:id/status` — activate/deactivate
- `DELETE /:id` — soft delete
- `PUT    /reorder` — bulk reorder, atomic (max 200 items, scoped to one branch in the request body)
- `POST   /:id/photo` — multipart upload, reuses ADR 0011 storage pipeline + per-tenant quota
- `DELETE /:id/photo` — remove photo

Response shape (no `tenant_id`, no `photo_key`):
```json
{ "id":"…","branch_id":"…","name":"VIP 1","description":null,
  "room_type":"vip","capacity":2,"amenities":["shower","tv"],
  "photo_url":"https://…/uploads/rooms/…/abc.jpg",
  "is_active":true,"sort_order":0,
  "created_at":"…","updated_at":"…" }
```

### 2.4 Frontend

Sidebar: Operasional → Ruangan (sejajar Terapis / Layanan / Tambahan).

Routes (tenant-admin only):
- `/master/rooms` — list, paginated 10, filter (branch + status + type), reorder up/down (page-scoped, same pattern as addons), photo thumbnail in row
- `/master/rooms/new` — create form (no photo on create — upload available after first save, like therapist)
- `/master/rooms/[id]` — edit form + photo picker (reuses pattern from therapist photo) + soft-delete

Form fields: name, branch (Select — only branches user can manage), room_type (Select 4 values), capacity (number), amenities (tag input — split on comma, no autocomplete in Phase 4), description, is_active.

Customer-facing assets (photo, room_type, capacity, amenities) prominent; admin-only (sort_order, is_active) less prominent.

### 2.5 Photo upload — reuse ADR 0011 infrastructure

Photo key format: `rooms/{room_id}/{hex16}.{ext}`.

Pipeline identical to therapist photo (ADR 0011 §2.4):
1. `MaxBytesReader` first
2. `ParseMultipartForm(1<<20)`
3. `ProcessUpload` (MIME sniff + DecodeConfig + imaging re-encode + EXIF strip)
4. Per-tenant quota check (shared counter with therapist uploads)
5. Storage write
6. DB update in tx
7. Async old-key delete

No new storage code — reuses `Storage` interface + `ProcessUpload` + `TenantQuota`.

### 2.6 Non-goals (deferred)

- Room ↔ service mapping (which services can run in which room) — Phase 5+ if needed.
- Room ↔ therapist mapping — same.
- Room booking conflict resolution — Phase 5 booking engine.
- Room pricing surcharge — Phase 5+.
- Customer-visible room picker — Phase 5.
- Multi-photo per room (gallery) — single photo only.

### 2.7 Review gates

- security-expert: RLS coverage, branch-scope service-layer enforcement (IDOR cross-branch), photo upload reuses validated ADR 0011 pipeline, permission UUID collision check.
- qa-expert: CRUD + reorder + branch-scope rejection (branch_admin trying to mutate room in foreign branch) + duplicate name within branch + cross-tenant isolation + cursor pagination.

## 3. Open questions

1. **`amenities` schema.** Free-text array vs lookup table. Decision: free-text. Operator-defined; consistent with `service.category` (ADR 0009 reasoning). If a tenant wants standardized amenities later, a lookup table can be added without breaking the array column.
2. **Capacity vs `room_type`.** Both expose similar info to customers. Decision: keep both. `room_type` is the UX label ("Couple Room") for filtering/picking; `capacity` is the hard number for booking-engine validation.
3. **Reorder cross-branch.** Reorder request body must specify a single `branch_id`; sort_order is per-branch. Service layer rejects if items span branches.
