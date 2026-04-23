# ADR 0008 — Tenant Onboarding & Branch Setup (Phase 3)

- **Status:** Accepted
- **Date:** 2026-04-22
- **Deciders:** owner (2026-04-22 — "lanjut Phase 3")
- **Related ADRs:** 0001 (RLS), 0003 (bootstrap super admin), 0005 (super admin sentinel), 0007 (user-membership)

---

## 1. Context

Phase 3 adds the **onboarding layer** on top of the auth foundation. Previously, tenants existed only if created manually via SQL. This ADR makes onboarding a real workflow:

- Anyone can submit a **company registration** (public endpoint, no auth).
- Platform admin reviews the queue and **approves or rejects**.
- On approval, the submitter's user + the tenant are created + a tenant_admin membership is issued + a welcome email is sent.
- Tenant admin then sets up **branches** (locations) for their tenant.
- Tenant/branch status transitions are enforced at the DB level (enum) and in the service layer.

---

## 2. Decision — binding contract for all three agents

### 2.1 Schema (new columns + 1 new table)

#### 2.1.1 `tenant` — add package + limits + rejection metadata

Migration 000011 adds:

```sql
ALTER TABLE tenant
    ADD COLUMN IF NOT EXISTS package TEXT NOT NULL DEFAULT 'starter'
        CHECK (package IN ('starter', 'growth', 'enterprise')),
    ADD COLUMN IF NOT EXISTS max_branches INT NOT NULL DEFAULT 1
        CHECK (max_branches >= 1),
    ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS approved_by UUID NULL
        REFERENCES "user"(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS rejected_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS rejected_by UUID NULL
        REFERENCES "user"(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS rejection_reason TEXT NULL
        CHECK (rejection_reason IS NULL OR char_length(rejection_reason) <= 1000);

-- Package → default max_branches matrix (applied on approval by application code):
--   starter    → 1
--   growth     → 5
--   enterprise → unlimited (stored as 999 sentinel)
```

Column `tenant.status` (existing enum `tenant_status`) now has a defined lifecycle:

```
pending_approval → active        (via admin approve)
pending_approval → deactivated   (via admin reject — the tenant row stays for audit)
active           → suspended     (via admin suspend; subscription lapse etc.)
suspended        → active        (via admin reinstate)
active|suspended → deactivated   (via admin permanently deactivate)
```

State transitions are enforced in the service layer, not via CHECK, because Postgres CHECK can't see the old value. `go-expert` implements a transition table.

#### 2.1.2 `branch` — add operational fields

Migration 000011 adds (the `branch` table already exists with `tenant_id`, `name`, `code`, `status`, `operational_hours`, `deleted_at`, audit):

```sql
ALTER TABLE branch
    ADD COLUMN IF NOT EXISTS address_line1 TEXT NULL,
    ADD COLUMN IF NOT EXISTS address_line2 TEXT NULL,
    ADD COLUMN IF NOT EXISTS city          TEXT NULL,
    ADD COLUMN IF NOT EXISTS province      TEXT NULL,
    ADD COLUMN IF NOT EXISTS postal_code   TEXT NULL,
    ADD COLUMN IF NOT EXISTS country       CHAR(2) NOT NULL DEFAULT 'ID',   -- ISO 3166-1 alpha-2
    ADD COLUMN IF NOT EXISTS timezone      TEXT NOT NULL DEFAULT 'Asia/Jakarta',
    ADD COLUMN IF NOT EXISTS contact_phone TEXT NULL,
    ADD COLUMN IF NOT EXISTS contact_email CITEXT NULL
        CHECK (contact_email IS NULL OR char_length(contact_email::text) BETWEEN 3 AND 320),
    ADD COLUMN IF NOT EXISTS activated_at  TIMESTAMPTZ NULL;
```

`branch.status` (existing `branch_status` enum: `active`, `inactive`) gains a defined lifecycle:

```
inactive → active       (via tenant_admin activate)
active   → inactive     (via tenant_admin deactivate)
inactive → soft-delete  (via tenant_admin delete — sets deleted_at; branch can still be read for audit)
```

Unique constraint: `UNIQUE (tenant_id, code) WHERE deleted_at IS NULL`.

#### 2.1.3 `tenant_registration` — NEW table for the public queue

```sql
CREATE TYPE tenant_registration_status AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE tenant_registration (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_name      TEXT NOT NULL CHECK (char_length(company_name) BETWEEN 2 AND 200),
    requested_slug    TEXT NOT NULL CHECK (char_length(requested_slug) BETWEEN 2 AND 100 AND requested_slug ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$'),
    package           TEXT NOT NULL DEFAULT 'starter'
        CHECK (package IN ('starter', 'growth', 'enterprise')),

    contact_name      TEXT NOT NULL CHECK (char_length(contact_name) BETWEEN 1 AND 200),
    contact_email     CITEXT NOT NULL
        CHECK (char_length(contact_email::text) BETWEEN 3 AND 320),
    contact_phone     TEXT NULL CHECK (contact_phone IS NULL OR char_length(contact_phone) BETWEEN 5 AND 30),

    status            tenant_registration_status NOT NULL DEFAULT 'pending',

    -- Set on approval:
    approved_tenant_id     UUID NULL REFERENCES tenant(id) ON DELETE SET NULL,
    approved_user_id       UUID NULL REFERENCES "user"(id) ON DELETE SET NULL,
    approved_at            TIMESTAMPTZ NULL,
    approved_by            UUID NULL REFERENCES "user"(id) ON DELETE SET NULL,

    -- Set on rejection:
    rejected_at            TIMESTAMPTZ NULL,
    rejected_by            UUID NULL REFERENCES "user"(id) ON DELETE SET NULL,
    rejection_reason       TEXT NULL CHECK (rejection_reason IS NULL OR char_length(rejection_reason) <= 1000),

    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tenant_registration_status_idx ON tenant_registration (status, created_at);
CREATE UNIQUE INDEX tenant_registration_pending_email_uidx
    ON tenant_registration (contact_email)
    WHERE status = 'pending';  -- prevent duplicate pending submissions from same email
CREATE UNIQUE INDEX tenant_registration_pending_slug_uidx
    ON tenant_registration (requested_slug)
    WHERE status = 'pending';

GRANT SELECT, INSERT ON tenant_registration TO lustia_app;
GRANT UPDATE ON tenant_registration TO lustia_app;  -- for status transitions by super admin
```

**RLS on `tenant_registration`:** platform-only.

```sql
ALTER TABLE tenant_registration ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_registration FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_registration_platform_only ON tenant_registration
    AS PERMISSIVE FOR ALL
    USING (current_setting('app.current_tenant', true) = '__platform__')
    WITH CHECK (current_setting('app.current_tenant', true) = '__platform__');
```

Public registration endpoint runs under the `__platform__` sentinel (the tenant middleware installs this by default for public endpoints; migration 10's policy covers the write path).

#### 2.1.4 Triggers

A `set_updated_at` BEFORE UPDATE trigger on `branch` and `tenant_registration` (the function already exists from migration 1).

---

### 2.2 API contract — new + modified endpoints

All endpoints under `/api/v1` (unchanged convention).

#### 2.2.1 `POST /register/company` — **NEW**, public (no auth)

Request:
```json
{
  "company_name": "Acme Wellness",
  "requested_slug": "acme-wellness",
  "package": "starter",
  "contact_name": "Alice Founder",
  "contact_email": "alice@acme-wellness.example",
  "contact_phone": "+628123456789"
}
```

Response `201 Created`:
```json
{
  "registration_id": "<uuid>",
  "status": "pending"
}
```

Error codes:
- `400 VALIDATION` — slug/email/phone/package shape
- `409 DUPLICATE_PENDING_REGISTRATION` (new) — same email or slug already has a pending registration
- `409 TENANT_SLUG_TAKEN` (new) — slug already belongs to an approved tenant
- `429 RATE_LIMITED` — 3/hour per IP

**Rate limiting:** per-IP + per-email, 3/hour each. Reuse the in-memory limiter (to be Redis in Phase 10).

#### 2.2.2 `GET /admin/tenants` — super admin only

Filter by status (`?status=pending_approval|active|suspended|deactivated|all`), cursor pagination (`?cursor=...&limit=50`, default limit 50).

Response:
```json
{
  "data": [
    {
      "id": "<uuid>",
      "name": "Acme Wellness",
      "slug": "acme-wellness",
      "status": "pending_approval",
      "package": "starter",
      "max_branches": 1,
      "contact_email": "alice@acme-wellness.example",
      "contact_name": "Alice Founder",
      "approved_at": null,
      "approved_by": null,
      "rejected_at": null,
      "rejection_reason": null,
      "created_at": "...",
      "membership_count": 0,
      "branch_count": 0
    }
  ],
  "next_cursor": null
}
```

Permissions: requires `tenant.read`.

#### 2.2.3 `GET /admin/tenant-registrations` — super admin only

Same shape as `GET /admin/tenants`, returns the `tenant_registration` queue. Filter by status (`pending|approved|rejected|all`, default `pending`).

#### 2.2.4 `POST /admin/tenant-registrations/:id/approve` — super admin only

Side effects (all within one transaction):
1. Find the registration. Must have `status = 'pending'`.
2. Verify `requested_slug` is still not taken by any approved tenant.
3. Create `tenant` row with `status = 'active'`, `approved_at = now()`, `approved_by = caller`, `package`/`max_branches` from the registration (+ the matrix in §2.1.1).
4. Create `user` row (global) for the contact — `is_super_admin = false`, `must_change_password = true`, `password_hash = <Argon2id of a temporary password>`. Capture the plaintext temporary password for the email.
5. Create `membership` linking the new user to the new tenant, `status = 'active'`.
6. Create `user_role` assignment with `tenant_admin` role.
7. Update the `tenant_registration` row: `status = 'approved'`, `approved_tenant_id`, `approved_user_id`, `approved_at`, `approved_by`.
8. Fire-and-forget email via Mailpit/SMTP to the contact: welcome, login URL, temporary password.
9. Audit log: `tenant.approved`.

Request body (optional override):
```json
{
  "package": "growth",   // optional — override the requested package
  "max_branches": 10     // optional — override the default for the package
}
```

Response `200`:
```json
{
  "tenant": { ...full tenant object... },
  "tenant_admin": {
    "user_id": "<uuid>",
    "email": "alice@acme-wellness.example",
    "temporary_password": "<random 16 chars>"   // returned ONCE; also emailed
  },
  "registration": { ...updated registration... }
}
```

Errors: `409 TENANT_SLUG_TAKEN`, `409 REGISTRATION_NOT_PENDING`, `404 NOT_FOUND`.

#### 2.2.5 `POST /admin/tenant-registrations/:id/reject` — super admin only

Request:
```json
{ "reason": "Duplicate application; existing tenant found under different slug." }
```

Side effects: set `status='rejected'`, `rejected_at`, `rejected_by`, `rejection_reason`. Optional email notify (fire-and-forget).

Response `200`: the updated registration row.

#### 2.2.6 `PATCH /admin/tenants/:id/status` — super admin only

Request: `{ "status": "suspended", "reason": "payment overdue" }`.

Response `200`: updated tenant. Enforces the state-machine in §2.1.1 — invalid transitions return `409 INVALID_STATUS_TRANSITION` (new error code).

#### 2.2.7 Branch CRUD — tenant admin only (permissions: `branch.*`)

- `POST   /api/v1/tenant/branches`           — create
- `GET    /api/v1/tenant/branches`           — list (cursor pagination, filter by status)
- `GET    /api/v1/tenant/branches/:id`       — one
- `PATCH  /api/v1/tenant/branches/:id`       — update non-status fields
- `PATCH  /api/v1/tenant/branches/:id/status`— change status (enforced transitions)
- `DELETE /api/v1/tenant/branches/:id`       — soft-delete

All are tenant-scoped via RLS (existing policies on `branch`). The `POST` endpoint enforces `COUNT(branch WHERE tenant_id = caller_tenant AND deleted_at IS NULL) < tenant.max_branches`; returns `409 BRANCH_LIMIT_REACHED` (new error code) if exceeded. The `max_branches = 999` sentinel for enterprise is treated as unlimited in the check.

---

### 2.3 Frontend flows

#### 2.3.1 Public registration — currently NOT a web app target

Phase 3 does not ship a public landing site. The public registration endpoint exists and is tested via Postman; a public landing can land in Phase 5+ (customer app) or as a dedicated marketing site.

**Decision:** no new Next.js app for Phase 3 public registration. `nextjs-expert` does not build it.

#### 2.3.2 Platform admin — tenant approval queue

**New route** `/tenants` in `platform-admin`:
- Table with 4 tabs: `Pending | Active | Suspended | Rejected/Deactivated`
- Per-row: company name, slug, requested package, contact, submitted date, buttons "Review".
- Clicking "Review" opens a right-side drawer:
  - Registration details (read-only)
  - Override dropdown: package (starter/growth/enterprise), max_branches (number)
  - Two buttons: **Approve** (primary), **Reject** (danger)
  - Reject opens a textarea for the reason.
  - On approve: success toast showing the temporary password for the tenant admin — "Share securely with the tenant; they'll be required to change it on first login." Also mentions the email was sent via SMTP.

**Approval status tab**: shows active/suspended tenants with status-change actions (suspend/reinstate/deactivate) from a `...` menu per row.

Uses new shadcn primitives: `Table`, `Tabs`, `Sheet` (drawer), `Alert Dialog` (for destructive actions), `Badge` (for status). Vendor source files into `components/ui/` as before.

#### 2.3.3 Tenant admin — branch management

**New route** `/branches` in `tenant-admin`:
- Table listing all branches (active + inactive filter).
- "New Branch" button → opens a dialog/sheet with form: name, code, address, timezone, contact. Submit → POST.
- Per-row: edit (opens same form populated), activate/deactivate toggle, delete (confirmation dialog).
- Branch count vs `tenant.max_branches` indicator (e.g. "3 / 5 branches used").

**New route** `/onboarding/welcome` (optional — only for first-login tenant_admin who just approved):
- Post-login landing: "Welcome to Lustia! Set up your first branch."
- If branches.length === 0 → stays on this screen; "Add first branch" CTA.
- Once at least one branch exists → redirect `/dashboard`.

Redirect logic: `/dashboard` Server Component checks branch count via a new `GET /api/v1/tenant/onboarding-state` endpoint (returns `{ has_branches: bool, must_change_password: bool, ... }`). If no branches yet and user is tenant_admin → `redirect('/onboarding/welcome')`.

#### 2.3.4 Ops portal — no new screens for Phase 3

Phase 3 doesn't touch ops staff workflows (those come in Phase 4+ when therapist/service/availability land). The ops app stays as-is.

---

### 2.4 Backend architecture

- **New service**: `service/tenant_service.go` (tenant CRUD + status transitions), `service/registration_service.go` (public registration + approval flow), `service/branch_service.go` (branch CRUD + limit check).
- **New repository**: `repository/branch_repository.go`, `repository/registration_repository.go`. Existing `tenant_repository.go` gets new methods (List, UpdateStatus, CountByFilter).
- **New model**: `model/tenant_registration.go`, `model/branch.go` enhanced.
- **New controller**: `controller/tenant_controller.go` (admin/tenants/*), `controller/registration_controller.go` (public register + admin queue), `controller/branch_controller.go` (tenant/branches/*).
- **Middleware**: no new middleware. Reuse `ScopeGate` + `RBAC`.
- **Router**: `route/route.go` wires new routes under `/api/v1/register/*` (public group), `/api/v1/admin/tenant-registrations/*` + `/api/v1/admin/tenants/*` (super admin only), `/api/v1/tenant/branches/*` (tenant scope).
- **Permission seeds**: migration 11 adds any missing permission codes. The existing seed has `tenant.read/create/update/delete/approve`, `branch.read/create/update/delete`. If any are missing, migration 11 inserts them ON CONFLICT DO NOTHING. Role-permission mapping: `super_admin` auto-gets all (already inherits `*.*`); `tenant_admin` gets `branch.*` (already assigned, verify).

---

### 2.5 Error codes (add to `constants/error_codes.go`)

```
CodeDuplicatePendingRegistration = "DUPLICATE_PENDING_REGISTRATION"
CodeTenantSlugTaken              = "TENANT_SLUG_TAKEN"
CodeRegistrationNotPending       = "REGISTRATION_NOT_PENDING"
CodeInvalidStatusTransition      = "INVALID_STATUS_TRANSITION"
CodeBranchLimitReached           = "BRANCH_LIMIT_REACHED"
```

Update `docs/API_CONTRACT.md` Error Code Catalog.

---

### 2.6 Email templates

Two new transactional emails (delivered via existing Mailpit/SMTP path):

1. **Tenant approved** — to registration contact:
   - Subject: "Selamat datang di Lustia — akun Anda siap"
   - Text body: login URL (e.g. `http://localhost:3002/login`), temporary password, tenant slug, note about required password change on first login.
2. **Tenant rejected** — optional, phase-3-scope but can ship without:
   - Subject: "Status registrasi perusahaan Anda"
   - Text body: registration_id, rejection reason, support contact.

Reuse `helper/email.go` — no new helper needed.

---

## 3. CLEAN invariant — migration 11 must be self-contained

As with ADR 0007, the migration chain `1 → 11` on a fresh DB must produce a working system. Migration 11:

- Adds `package` / `max_branches` / `approved_*` / `rejected_*` columns to `tenant` (IF NOT EXISTS).
- Adds address/timezone/contact columns to `branch` (IF NOT EXISTS).
- Creates `tenant_registration_status` enum (IF NOT EXISTS equivalent via DO block).
- Creates `tenant_registration` table + indexes + RLS + grants.
- Inserts missing permission codes via `ON CONFLICT DO NOTHING`.
- Down migration reverses every `CREATE`/`ALTER`.

Dev-seed migration `000012_seed_dev_registrations.up.sql` (optional, dev-only) adds 1–2 sample pending registrations for UI testing. Same pattern as migration 10 — separate file, skip in staging/prod by stopping `migrate up` at 11.

---

## 4. Cross-agent interface summary

- `db-designer` produces: migration 11 (+ optional 12), updates `docs/DATA_MODEL.md`.
- `go-expert` produces: all new services/repositories/controllers/routes, updates `docs/API_CONTRACT.md`. Depends on the column names in §2.1 and the endpoint paths in §2.2.
- `nextjs-expert` produces: platform-admin `/tenants` screens, tenant-admin `/branches` + `/onboarding/welcome` + onboarding redirect. Depends on the response shapes in §2.2 and the error codes in §2.5.

All three trust the ADR as the source of truth if docs haven't landed yet when they start.

---

## 5. Open questions (deferred)

1. Does `POST /register/company` need a CAPTCHA before staging deploy? → Yes (security-expert flags), add after Phase 3 basics ship.
2. Should the temporary password be emailed OR shown on the admin approval screen ONLY? → Both, for Phase 3 convenience; tightening to email-only is a Phase-10 hardening item.
3. Do we allow a user to submit multiple pending registrations for different companies simultaneously? → **No** for Phase 3: unique partial index on `contact_email` WHERE `status='pending'` prevents it. Revisit if real users need multi-company onboarding.
4. Does deactivation cascade to members (revoke all active memberships)? → **Yes** — the tenant status transition to `deactivated` must also transition all memberships to `'suspended'`. `go-expert` implements this cascade in the service layer.

---

## 6. Decision log

- 2026-04-22 — Owner approved Phase 3 start. ADR drafted, 3-agent parallel delegation authorized.
