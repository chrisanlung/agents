# ADR 0010 — Tenant-wide add-on catalog

- **Status:** Accepted (rewritten 2026-04-24)
- **Date:** 2026-04-24 (revised)
- **Deciders:** owner (2026-04-24), orchestrator
- **Extends:** ADR 0009 (Phase 4 master operational data)
- **Supersedes internal v1:** An earlier draft of this ADR specified a per-service flat model (`service_addon`). That draft was rewritten in the same session before any agent downstream referenced the v1 design in customer-facing code. See `## Change log` at the bottom.

---

## 1. Context

Tenants need to offer paid optional extras — food, drinks, aromatherapy upgrade, hot towel, etc. — alongside their core services. The critical axis for this decision is **where add-ons are selected in the customer journey**.

Customer flow (Phase 5 booking):
1. Customer picks a service from the tenant's catalog (e.g. "Pijat 60 menit")
2. Customer optionally adds one or more extras (e.g. "Handuk panas", "Aromaterapi", "Jus segar")
3. Customer checks out

The owner's requirement (2026-04-24): **any add-on must be selectable with any service.** A customer picking reflexology should see the same "Jus segar" option as a customer picking massage. Tying add-ons to a specific service would force the operator to duplicate each add-on per service (repetitive entry for the operator) and more importantly would be inconsistent from the customer's point of view — why does the juice disappear when I change the service?

The question this ADR answers: **how is the add-on catalog modelled, scoped, and managed?**

## 2. Options considered

### Option A — Tenant-wide flat catalog (CHOSEN)
- `addon(id, tenant_id, name, description, price_idr, is_active, ...)` — single table, tenant-scoped.
- No mapping table. Any add-on is globally available across every service for the tenant.
- Booking engine (Phase 5) records selected add-ons on the booking row; nothing at the catalog level changes per booking.
- **Pros:** matches the customer journey exactly; operator edits one row when price changes; minimal tables; simplest CRUD UX (one list page).
- **Cons:** cannot express "this add-on only applies to service X" without a future mapping table — but the owner explicitly stated this is *not* a requirement.

### Option B — Catalog + per-service mapping
- `addon` + `service_addon(service_id, addon_id)` — add-on exists in tenant catalog, applied to N services.
- **Rejected:** required only if some add-ons are service-restricted. Owner's requirement is the opposite — everything is cross-service.

### Option C — Per-service flat (v1 of this ADR)
- `service_addon(service_id, name, price_idr, ...)` — each add-on lives under one service.
- **Rejected 2026-04-24** for the customer-journey reason above. Operator would duplicate the same "Aromaterapi" row under every massage service; customer would see the same extra disappear when changing service.

## 3. Decision

**Option A.** Tenant-wide flat catalog, no service mapping.

## 4. Binding contract for agents

### 4.1 Schema — owned by `db-designer`

New table `addon` (tenant-scoped):

| Column | Type | Constraints | Notes |
|---|---|---|---|
| `id` | `UUID` | PK | |
| `tenant_id` | `UUID` | NOT NULL, FK → `tenant(id)` RESTRICT | RLS anchor |
| `name` | `TEXT` | NOT NULL, length 1–120 | Display name, e.g. "Aromaterapi" |
| `description` | `TEXT` | NULL, length ≤ 500 | Optional detail shown in booking UI |
| `price_idr` | `BIGINT` | NOT NULL, CHECK ≥ 0 | IDR integer (consistent with `service.price` bigint after migration 000016) |
| `is_active` | `BOOLEAN` | NOT NULL, DEFAULT true | Temporarily remove from catalog without deleting |
| `sort_order` | `INT` | NOT NULL, DEFAULT 0, CHECK 0–9999 | Display order in the catalog list and customer picker |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT now() | |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT now() | |
| `created_by` | `UUID` | NULL, FK → `"user"(id)` SET NULL | |
| `updated_by` | `UUID` | NULL, FK → `"user"(id)` SET NULL | |
| `deleted_at` | `TIMESTAMPTZ` | NULL | Soft-delete. Hard-DELETE blocked at DB grant layer. |

**Constraints:**
- `UNIQUE(tenant_id, name) WHERE deleted_at IS NULL` — partial unique: duplicate name within the same tenant rejected.
- RLS policies mirroring `service`: `tenant_id::text = current_setting('app.current_tenant', true)` for SELECT / INSERT / UPDATE. No DELETE policy (no grant).
- `GRANT SELECT, INSERT, UPDATE ON addon TO lustia_app;` (no DELETE — soft-delete only).
- `updated_at` trigger using the existing `set_updated_at()` function.

**Indexes:**
- `addon_tenant_active_idx`: `(tenant_id, sort_order)` WHERE `is_active = true AND deleted_at IS NULL` — primary list scan for both admin UI and customer picker.
- `addon_tenant_id_idx`: `(tenant_id)` — RLS predicate.

Migration `000018_phase4_addons.up.sql` + down. Dev seed `000019_seed_dev_addons.up.sql` inserts ~4 sample add-ons at tenant level for `acme-spa`.

### 4.2 API contract — owned by `go-expert`

All require `scope=tenant` JWT. Reuse permissions `addon.read` / `addon.write` — new permission rows (see §4.2.1), NOT reusing `service.*` because the add-on catalog is a distinct resource and a future `tenant_admin_readonly` role may have different access.

#### 4.2.1 New permissions
- `addon.read` — list and detail
- `addon.create`, `addon.update`, `addon.delete` — mutations

Grant to the existing `tenant_admin` role in the seed. `branch_admin` does NOT get add-on write permissions (add-ons are tenant-scoped; a branch-admin should not mutate the tenant catalog). `branch_admin` may get `addon.read` only so they can view (useful for ops context Phase 5).

#### 4.2.2 Endpoints

- `GET    /api/v1/tenant/addons` — list (filter: `is_active`, cursor pagination, page size 10 default per Lustia list convention)
- `GET    /api/v1/tenant/addons/:id` — detail
- `POST   /api/v1/tenant/addons` — create
- `PATCH  /api/v1/tenant/addons/:id` — update (name/description/price/sort_order)
- `PATCH  /api/v1/tenant/addons/:id/status` — activate/deactivate (body: `{"is_active": true|false}`)
- `DELETE /api/v1/tenant/addons/:id` — soft delete
- `PUT    /api/v1/tenant/addons/reorder` — bulk reorder (body: `{"items": [{"id","sort_order"}]}`, max 200, atomic tx)

No change to `GET /services/:id`. Services and add-ons are independent resources at the API level. Phase 5 booking engine will join them per booking.

### 4.3 Frontend — owned by `nextjs-expert` (design by `ui-ux-expert`)

**tenant-admin portal only.** Platform-admin and ops portals: out of scope for Phase 4.

New top-level master-data page: `/master/addons`
- **List page** (`/master/addons`) — cursor-paginated table with default page size 10 (per Lustia list convention). Columns: name, price (Badge outline), status (Badge active/inactive), sort, actions. Filter bar with `is_active` filter. Header CTA "Tambah Add-on".
- **Create form** (`/master/addons/new`) — form with name / description / price / is_active. Same layout grammar as service form.
- **Edit form** (`/master/addons/[id]`) — populated form + soft-delete action.
- **Reorder UX** — up/down arrows in list page (optimistic + bulk PUT, same pattern we built in the superseded design; keeps the UX familiar).
- **Inactive row styling** — `line-through text-muted-foreground` on name (non-color indicator, WCAG 1.4.1).

Sidebar nav: add "Tambahan" (or "Add-on") item under the existing "Master Data" section, alongside Terapis and Layanan.

No embedded editor on service detail page. The third-tab approach from the superseded design is removed.

### 4.4 Non-goals (deferred to Phase 5+)

- Per-service add-on restrictions (Option B) — only add when a tenant requests it.
- Customer-facing selection UI — that's Phase 5.
- Add-on capacity limits per booking — defaulted to 0..N unlimited in Phase 4; booking engine may cap in Phase 5 if needed.
- Add-on images, tags, categories.
- Add-on stock/inventory.

### 4.5 Review gates

- `security-expert`: RLS coverage on `addon`; permission wiring (branch_admin correctly blocked from write); cross-tenant isolation; soft-delete correctness.
- `qa-expert`: coverage for list/create/update/status/delete/reorder + cross-tenant isolation + duplicate-name rejection + pagination cursor.

## 5. Open questions

1. **Permission name `addon.*` vs `catalog.addon.*`** — flat names align with the existing `service.*`, `therapist.*` pattern. Go with flat.
2. **Max add-ons per booking** — not enforced in Phase 4. Deferred to Phase 5 booking engine if operators complain.
3. **Recovery UI for soft-deleted add-ons** — deferred, same as services/therapists.

## 6. Change log

- **2026-04-24 (initial draft):** per-service flat model (`service_addon` table, embedded editor in service detail page). Implemented end-to-end including migrations, backend, frontend, review gates. Build green, tests 21/21.
- **2026-04-24 (pivot, same session, before commit):** owner revised requirement — add-ons must apply globally across bookings, not per service. Rationale: customer journey consistency. ADR rewritten in place, all implementation from the first draft is being replaced. No code from the first draft was merged or deployed. This change log remains to preserve the design trail.
