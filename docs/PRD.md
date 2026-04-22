# Product Requirements — Lustia

> **App:** Lustia — Multi-Tenant Special Therapist Platform
> **Scope doc:** full roadmap in [phase_development_special_therapist.pdf](../phase_development_special_therapist.pdf)
> **Current scope (this PRD):** Phase 1 + Phase 2 only. Later phases tracked for context but not in scope here.

## 1. Summary

Lustia is a **multi-tenant B2B platform** for special therapist businesses (spa / wellness / therapy clinics). Each tenant (company) can have multiple branches (locations). Each branch has therapists, services, schedules, customers, bookings, invoices, and payments. The platform owner (super admin) approves tenants; tenants manage their own branches, staff, customers, and operations.

The full roadmap has 10 phases. **This deliverable covers only Phases 1 and 2** — the database foundation and the authentication/authorization service — establishing the secure, multi-tenant-isolated foundation that all later phases build on.

## 2. Users & roles (role matrix for Phase 2)

| Role | Scope | Typical abilities |
| --- | --- | --- |
| `super_admin` | Platform-wide | Approve tenants, manage tenant lifecycle, platform-wide reporting (later phases) |
| `tenant_admin` | One tenant | Manage own tenant's branches, staff, config; view all branches within tenant |
| `branch_admin` | One branch | Manage own branch's therapists, services, schedules, bookings |
| `finance` | One tenant | Read invoices, payments, financial reports across all branches of own tenant |
| `therapist` | Own profile + assigned bookings | Read own schedule, update own availability, see own assigned bookings |
| `customer` | Own records only | Register, browse services/therapists, book, pay, view own history (later phases) |

Role → permission matrix is part of Phase 2 deliverable.

## 3. User stories (Phase 1 + 2)

### Phase 1 — Foundation Database & Master Schema
- As an **architect**, I want a PostgreSQL schema with all core tables (tenants, branches, roles, permissions, users, customers, services, therapists, therapist_services, therapist_availabilities, bookings, invoices, payments) so that every later phase has a stable data contract.
- As an **architect**, I want every operational table to carry `tenant_id` (and `branch_id` where relevant) with foreign keys + indexes so isolation is enforced at the DB level.
- As an **architect**, I want Row-Level Security (RLS) policies on operational tables so that accidental cross-tenant reads are impossible even if the app forgets to filter.
- As an **engineer**, I want standardized audit fields (`created_at`, `updated_at`, `created_by`, `updated_by`) on every table, auto-populated by triggers where possible.
- As an **engineer**, I want seed data for roles, permissions, and reference statuses so the system has a usable baseline after a fresh migration.
- As an **engineer**, I want the migration folder to live **separately** from any service code (`/lustia/migrations/`) so migrations can be run independently by ops or by a dedicated migrator job.

### Phase 2 — Authentication, Authorization, Access Control
- As a **user**, I want to log in with email + password and receive a short-lived access token + long-lived refresh token, so the session is secure and revocable.
- As a **user**, I want to log out and have my refresh token revoked server-side.
- As a **user**, I want to change my password (old → new) with rate limiting to prevent abuse.
- As a **user**, I want to view and update my own profile (name, phone, avatar reference).
- As a **tenant_admin**, I want to create/activate/deactivate users within my own tenant, and assign them roles.
- As a **platform (middleware)**, every authenticated request must carry resolved `tenant_id` and (where applicable) `branch_id` so downstream code and RLS policies can enforce scope.
- As a **platform (middleware)**, every authorization check must reject requests lacking the required permission for the resource, with a consistent `FORBIDDEN` error.

## 4. Scope (in)

- PostgreSQL 16+ schema covering all Phase 1 tables (even those used by later phases — we want the shape locked in now).
- `golang-migrate`–compatible migration files in `/lustia/migrations/` (up + down, versioned).
- Seed migration for roles, permissions, reference statuses, and one bootstrap `super_admin` tenant + user (env-driven password).
- Auth microservice (`/lustia/services/auth/`) built in Go with **Gin + GORM + Clean Architecture + SOLID**, consuming `github.com/chrisanlung/common-configs` for config/logger/DB/transport.
- Auth endpoints: register (admin-initiated only in phase 2 — no public signup yet), login, refresh, logout, me, update profile, change password, admin user management (create/list/activate/deactivate/assign-roles).
- Middleware: JWT verify → tenant scoping → RBAC permission check.
- `docker-compose.yml` for local dev: Postgres + migrator + auth-service.
- Basic health/readiness endpoints on auth-service.
- Updated shared docs: `docs/DATA_MODEL.md`, `docs/API_CONTRACT.md` (auth section), `docs/SECURITY.md` (auth design), `docs/OPERATIONS.md` (compose + migrate runbook).

## 5. Scope (out / non-goals for Phase 1+2)

- Public customer signup (Phase 5).
- Tenant onboarding / approval UI flow (Phase 3).
- Branch setup CRUD (Phase 3).
- Therapist/service/availability CRUD endpoints (Phase 4) — **tables exist, endpoints later**.
- Booking, payment, notification, reporting (Phases 5–9).
- Frontend (web / mobile) — backend APIs only.
- OAuth / social login / MFA — email+password + JWT only for now; MFA deferred.
- Production deployment (k8s, cloud infra) — local Docker Compose only.

## 6. Constraints & decisions

- **Platforms:** Backend (Go microservice) only. No web or mobile clients built yet.
- **Database:** PostgreSQL 16+ (default per project db-designer agent).
- **ORM / HTTP:** GORM + Gin (default per project go-expert agent).
- **Architecture:** Clean Architecture + SOLID, **per microservice**. Each service has its own `domain` / `usecase` / `port` / `adapter` layers. No shared domain package across services.
- **Multi-tenancy:** shared schema + `tenant_id` column on every operational table + PostgreSQL **Row-Level Security** policies driven by `SET LOCAL app.current_tenant = '…'` per transaction. App middleware sets the setting; DB enforces isolation.
- **Auth:** email + Argon2id password hash; short-lived JWT access tokens (signed HS256 with rotated secret or RS256 — security-expert to pick); refresh tokens persisted as DB rows (revocable).
- **Shared library:** `github.com/chrisanlung/common-configs` for config, logger, DB manager, Redis, transport plumbing, OTel.
- **Migration tool:** `golang-migrate` (file-based, `{version}_{name}.up.sql` / `.down.sql`).
- **Code language:** Go 1.24+ (match common-configs).

## 7. Repo layout (target for Phase 1+2)

```
F:/Projects/lusthing/
├── CLAUDE.md
├── docs/                         # shared planning docs
└── lustia/                       # the app
    ├── README.md
    ├── .env.example
    ├── migrations/               # *** separate top-level migration folder ***
    │   ├── 000001_init_schema.up.sql
    │   ├── 000001_init_schema.down.sql
    │   ├── 000002_auth_tables.up.sql
    │   ├── 000002_auth_tables.down.sql
    │   ├── 000003_operational_tables.up.sql
    │   ├── 000003_operational_tables.down.sql
    │   ├── 000004_rls_policies.up.sql
    │   ├── 000004_rls_policies.down.sql
    │   ├── 000005_seed_reference.up.sql
    │   └── 000005_seed_reference.down.sql
    ├── services/
    │   └── auth/
    │       ├── cmd/auth/main.go
    │       ├── internal/
    │       │   ├── domain/
    │       │   ├── usecase/
    │       │   ├── port/
    │       │   └── adapter/{http,repository}/
    │       ├── config/app.yaml
    │       ├── Dockerfile
    │       ├── go.mod
    │       └── go.sum
    └── deploy/
        └── docker-compose.yml
```

## 8. Open questions

- JWT signing algorithm: HS256 (simple, shared secret) vs RS256 (public-key verify by other services). **Default pick for Phase 2: RS256**, private key in auth-service, public key distributed to other services via config or JWKS endpoint. Confirm with `security-expert`.
- Refresh token rotation: rotate on every refresh (recommended) vs long-lived re-use. **Default: rotate, single-use refresh tokens.** Security-expert to confirm.
- Super-admin seeding: first super admin created via env-driven bootstrap migration vs out-of-band `admin-cli`. **Default: bootstrap migration seeds a super admin whose initial password is taken from an env var (must be rotated on first login).**
- User-email uniqueness: globally unique or unique-per-tenant? **Default: unique per `(tenant_id, email)`** so the same email can exist in different tenants (common for B2B SaaS). Confirm.
- Branch admin / finance / therapist roles — how they get assigned to one specific branch vs across branches: via a `user_branch_assignments` table (many-to-many) or single `branch_id` on user. **Default: many-to-many** via `user_branches` assignment table, to support staff that rotate between branches.
