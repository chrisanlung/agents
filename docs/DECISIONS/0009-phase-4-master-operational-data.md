# ADR 0009 — Phase 4: Master Operational Data

- **Status:** Accepted
- **Date:** 2026-04-23
- **Deciders:** owner (2026-04-23 — "go phase 4")
- **Supersedes:** — (extends ADR 0008)

---

## 1. Context

Phase 3 delivered tenant onboarding + branch setup. Before booking can ship (Phase 5), every tenant needs to populate its **operational master data**: therapists (staff who deliver services), services (offerings with duration + price), the mapping between them, and per-therapist availability windows. Branches already have `operational_hours` JSONB from Phase 3 — Phase 4 extends that into the per-therapist schedule domain.

The core question Phase 4 answers: **can the booking engine know "who is available to perform which service, at which branch, at time T"?** That is the single invariant every downstream feature depends on.

## 2. Decision — binding contract for all agents

### 2.1 Schema (new tables + extensions)

Tables introduced (or populated, since Phase 1 created empty shells):

- `therapist` — staff profile scoped to a branch (a therapist works at one branch; a tenant with N branches may have the same person twice if they work both). One `user_id` optional (only when the therapist logs into the ops portal). `tenant_id + branch_id` denormalised for RLS.
- `service` — service offering scoped to a tenant (shared across branches). Has `name`, `duration_minutes`, `price_idr`, `category`, `is_active`.
- `therapist_service` — many-to-many mapping; a therapist can perform a subset of the tenant's services. Carries `is_active` so a service can be "temporarily not offered by this therapist" without deleting the row.
- `therapist_availability` — per-therapist weekly recurring schedule ("Senin 09:00–17:00"). Simple weekly pattern; **no date overrides in Phase 4** — time-off / holiday exceptions land in Phase 5.
- `branch_operational_hours` — already exists as JSONB on `branch` (Phase 3). Phase 4 promotes read access to an API endpoint; no migration change.

All Phase 4 tables: `tenant_id UUID NOT NULL REFERENCES tenant(id)`, `branch_id UUID NOT NULL REFERENCES branch(id)` where branch-scoped, standard audit columns, RLS policies following the Phase 1–3 pattern.

**db-designer** owns the exact DDL and enumerations; flag schema choices in [docs/DATA_MODEL.md](../DATA_MODEL.md) with the reasoning.

### 2.2 API contract — new endpoints

All require `scope=tenant` JWT with matching permissions. Permissions new in Phase 4:

- `therapist.read`, `therapist.create`, `therapist.update`, `therapist.delete`
- `service.read`, `service.create`, `service.update`, `service.delete`
- `availability.read`, `availability.write` (per-therapist scoping enforced in service layer — a `branch_admin` can manage availability only for therapists at their branch)

Endpoint set (exact contract to be authored by **go-expert** in [docs/API_CONTRACT.md](../API_CONTRACT.md)):

#### Therapist
- `POST /api/v1/tenant/therapists` — create
- `GET /api/v1/tenant/therapists` — list (filter: `branch_id`, `is_active`, pagination)
- `GET /api/v1/tenant/therapists/:id` — detail including assigned services
- `PATCH /api/v1/tenant/therapists/:id` — update profile
- `PATCH /api/v1/tenant/therapists/:id/status` — activate/deactivate
- `DELETE /api/v1/tenant/therapists/:id` — soft delete

#### Service
- `POST /api/v1/tenant/services` — create
- `GET /api/v1/tenant/services` — list (filter: `is_active`, `category`)
- `GET /api/v1/tenant/services/:id` — detail including therapists offering it
- `PATCH /api/v1/tenant/services/:id` — update
- `PATCH /api/v1/tenant/services/:id/status` — activate/deactivate
- `DELETE /api/v1/tenant/services/:id` — soft delete

#### Therapist ↔ service mapping
- `PUT /api/v1/tenant/therapists/:id/services` — full replace of assigned services (body: array of service IDs)
- Alternate granular: `POST /:id/services/:serviceID`, `DELETE /:id/services/:serviceID` — implementation choice (go-expert picks the simpler one)

#### Availability
- `GET /api/v1/tenant/therapists/:id/availability` — returns weekly pattern
- `PUT /api/v1/tenant/therapists/:id/availability` — full replace of weekly pattern (body: `[{dow:1, start:"09:00", end:"17:00"}, ...]`)

#### Branch operational hours (read-only, already stored in Phase 3)
- `GET /api/v1/tenant/branches/:id/operational-hours` — extract the JSONB for the ops portal

### 2.3 Frontend flows

#### tenant-admin portal (port 3002)
New nav section **"Master Data"** (or "Operasional") with sub-pages:

- `/master/therapists` — list + CRUD + activate/deactivate
- `/master/therapists/:id` — edit profile + availability editor (weekly grid) + assigned services multi-select
- `/master/services` — list + CRUD + activate/deactivate
- `/master/services/:id` — edit detail + therapists offering it (read-only — managed from therapist side)

**ui-ux-expert** owns layout, interaction patterns (especially the weekly availability editor and multi-select UX for therapist ↔ service mapping), empty states, and the position of "Master Data" within the existing app navigation. Recommendation: put it behind a disclosure/secondary-nav so the dashboard stays focused on daily operations.

#### ops portal (port 3003) — out of scope for Phase 4
Ops staff (supervisor, therapist) will consume availability read endpoints in Phase 5 when booking lands. Phase 4 only builds the admin-side CRUD.

#### platform-admin portal (port 3001) — no changes
Platform admin doesn't touch tenant-owned master data.

### 2.4 Backend architecture

Same modular monolith pattern. New packages under `internal/service/`:
- `therapist_service.go`
- `service_service.go` (named `catalog_service.go` if the stutter bothers you)
- `availability_service.go`

New controllers, repositories, model files mirror the pattern from Phase 3. No new middleware.

### 2.5 Non-goals (deferred to Phase 5+)

- **Date-specific availability overrides** — holidays, time-off, training days. Weekly pattern is the only primitive in Phase 4.
- **Service pricing tiers / discounts** — single price per service.
- **Therapist skill levels / certifications** — not modelled in Phase 4.
- **Service packages / bundles** — single-service offerings only.
- **Cross-branch therapist roster** — a therapist row is bound to exactly one branch in Phase 4; a human who works at two branches has two therapist rows (linked optionally by the same `user_id`).
- **Customer-facing service catalog** — public customer endpoints are Phase 5.
- **Booking conflict resolution** — availability is *declared* in Phase 4; *consumed* in Phase 5 (booking service checks availability before inserting).

### 2.6 CLEAN invariant — Phase 4 migrations

New migrations `000013_phase4_master_data.up.sql` (+ down) and optionally `000014_seed_dev_master_data.up.sql` (dev-only sample therapists/services for acme-spa). Migrations 1 → 13 must produce a working system on any environment; migration 14 stays dev-only (same separation pattern as 10 and 12).

### 2.7 Review gates

Same as Phase 3:
- **security-expert** reviews once API contract + service layer code lands — focus on RLS coverage for new tables, authorization checks on cross-branch access (a branch_admin at branch A must not mutate therapists at branch B).
- **qa-expert** (pinch-hit go-expert) writes integration tests for the 4 critical flows: create therapist + assign services, edit availability, activate/deactivate cascade, cross-branch isolation regression.

## 3. Open questions

1. **Availability granularity** ✅ Resolved 2026-04-22 — `TIME` columns with minute precision. UI rounds to desired step (15/30 min) at the service layer; DB stores whatever valid TIME the service sends. See DATA_MODEL.md §Phase 4 Q1.
2. **Service duration** ✅ Resolved 2026-04-22 — fixed `duration_minutes INT NOT NULL`. Variable ranges deferred to Phase 5+. See DATA_MODEL.md §Phase 4 Q2.
3. **`therapist.user_id`** ✅ Resolved 2026-04-22 — service-layer enforcement only. DB trigger deferred (same as cross-tenant check on `therapist_service`). See DATA_MODEL.md §Phase 4 Q3.
4. **Soft-delete semantics** ✅ Resolved 2026-04-22 — soft-delete only via `deleted_at`; hard DELETE is blocked at DB level (no DELETE grant on `therapist` for `lustia_app`). Service layer must reject deletion if future non-terminal bookings exist (Phase 5 enforcement). `therapist_service` and `therapist_availability` rows are not auto-modified on soft-delete; service layer should set `therapist_service.is_active = false` as part of the deactivation transaction. See DATA_MODEL.md §Phase 4 Q4.

Each open question is resolved by the agent that owns the affected doc (db-designer for #1–#3, security-expert for #4).

## 4. Decision log

- 2026-04-23 — Owner approved Phase 4 start. ADR drafted. Parallel design phase (db-designer + ui-ux-expert) spawning next.
- 2026-04-22 — db-designer resolved open questions Q1–Q4. Schema extensions designed. Migrations 000013 (structural) and 000014 (dev seed) written. DATA_MODEL.md §Phase 4 appended.
