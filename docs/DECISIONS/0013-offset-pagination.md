# ADR 0013 — Offset-based pagination for all list endpoints

- **Status:** Accepted
- **Date:** 2026-04-25
- **Deciders:** owner (2026-04-25 — "ya opsi B saja"), orchestrator
- **Supersedes:** cursor pagination shape established in ADR 0008 (branch listing) and propagated to all subsequent list endpoints (tenants, registrations, therapists, services, addons, rooms).

---

## 1. Context

Owner reported on 2026-04-25 that cursor pagination ("Kembali ke awal" + "Halaman berikutnya") on the Tenant Registrations list was not user-friendly. With ~12 pending registrations the user cannot jump to page 5 directly, has no idea how many pages exist, and must traverse linearly. The same UX exists on every list page (tenants, branches, therapists, services, addons, rooms).

Three options were weighed:
- A: Keep cursor + add multi-step "back" via cursor stack.
- **B: Migrate to offset-based pagination with numbered page buttons.** ← chosen
- C: Cosmetic counter on cursor (rejected — misleading UX).

## 2. Decision

Migrate all list endpoints from cursor-based to offset-based pagination.

**Why offset is acceptable here:**
- All Lustia admin lists are bounded by tenant scale: starter ≤5 branches, growth ≤5 therapists/branch, typical tenant ≤100 services + addons. `OFFSET 1000` performance concern does not apply.
- Numbered page UX is universally familiar and matches the user's mental model.
- Page-drift on insert/delete during navigation is acceptable for admin lists with low write churn.

**Why not infinite scroll:** admin lists are scan-and-act flows (tenant approval, therapist edit). Numbered pages give better orientation than continuous scroll.

## 3. Binding contract

### 3.1 Query params (all list endpoints)

`?page=1&limit=10&[other-filters]`

- `page` is **1-indexed**. Missing page → page 1.
- `limit` capped via Gin binding tag `min=1,max=200`. Default 10.
- Other filters (status, branch_id, etc.) unchanged.

### 3.2 Response shape

```json
{
  "data": [...],
  "page": 1,
  "limit": 10,
  "total_count": 47,
  "total_pages": 5
}
```

- `total_count` is post-filter count.
- `total_pages = ceil(total_count / limit)`. Zero when `total_count = 0`.
- `next_cursor` field is **removed** from all list responses.

### 3.3 Endpoints affected (rewrite required)

- `GET /api/v1/admin/tenants`
- `GET /api/v1/admin/tenant-registrations`
- `GET /api/v1/admin/users` (if used)
- `GET /api/v1/tenant/branches`
- `GET /api/v1/tenant/therapists`
- `GET /api/v1/tenant/services`
- `GET /api/v1/tenant/addons`
- `GET /api/v1/tenant/rooms`

Each gets:
1. Query DTO updated: `Cursor string` → `Page int \`form:"page" binding:"omitempty,min=1"\``.
2. Repository: `OFFSET (page-1)*limit LIMIT limit`. Plus a separate `COUNT(*)` query under the same WHERE clause for `total_count`.
3. Response DTO: drop `next_cursor`, add `page`, `limit`, `total_count`, `total_pages`.

### 3.4 Frontend `<Pagination>` component (rewrite)

New API:
```ts
<Pagination
  pathname="/master/rooms"
  searchParams={{ status, branch_id, ... }}
  page={page}
  totalPages={totalPages}
  totalCount={totalCount}
  pageSize={10}
/>
```

Render: `« 1 ... 4 [5] 6 ... 12 »` with ellipsis for >7 pages, optional. Hint line: "Menampilkan 41-50 dari 47 data". Prev/Next disabled at boundaries.

All consumer pages (one per endpoint above) update:
- Read `page` from URL `searchParams` (default 1).
- Pass `page` + `totalPages` + `totalCount` to `<Pagination>`.
- Drop `cursor` / `nextCursor` / `hasPrev` props.

Three frontends in scope: `tenant-admin`, `platform-admin`, `ops`. Each has its own `<Pagination>` component file — keep them as twins (same shape, separate files per existing convention).

### 3.5 Performance

`COUNT(*)` on every list query is the new cost. Acceptable at Lustia scale (admin lists, max ~thousands of rows per tenant). If a future hot list exceeds 100k rows, revisit (e.g., approximate count via `pg_class.reltuples` for that specific endpoint).

### 3.6 Out of scope

- Page-size selector (10/25/50/100 dropdown) — Phase 5+ if requested.
- Customer-facing infinite scroll — different paradigm, evaluated separately.
- Server-side caching of `total_count` — premature.

## 4. Migration mechanics

**No DB migration required** — pagination is a query-shape change, not a schema change.

Rollout order (single PR / single session):
1. Update memory `feedback_list_pagination.md` (done).
2. Backend: all 8 endpoints — DTO + repository + service. Tests update.
3. Frontend: `<Pagination>` component (3 copies, twin) + every consumer list page (8 pages × 3 frontends = ~12 files actually affected).
4. API_CONTRACT.md: update each list endpoint's section.
5. Manual QA on each frontend.

## 5. Open questions

1. **Should `total_count` be returned even when filters yield 0?** Yes — `{ "data": [], "total_count": 0, "total_pages": 0 }`. Frontend hides pagination control when `total_pages ≤ 1`.
2. **Page > total_pages behavior.** Backend returns empty `data: []` and echoes the requested `page`. Frontend redirects to `total_pages` (or page 1 if zero) when it detects this — small UX nicety.
