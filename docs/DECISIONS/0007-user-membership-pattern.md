# ADR 0007 — User–Membership Pattern (One Login, Many Tenants)

- **Status:** Accepted
- **Date:** 2026-04-21
- **Deciders:** owner (2026-04-21 — "1 login bisa kelola multi tenant")
- **Supersedes:** the previous "user per tenant" model established by migrations 000002 and 000003
- **Related ADRs:** 0001 (multi-tenancy RLS), 0005 (super-admin sentinel)

---

## 1. Context

Current `"user"` schema keys identity to a tenant: `UNIQUE (tenant_id, email)`. Alice who works at Acme Spa AND at Beauty Clinic = two rows, two passwords. Login requires the user to know and type their tenant slug.

This ADR switches to the **workspace-membership pattern** used by Slack, Notion, Linear, Figma: one human = one `user` row (identity); relationships to tenants are separate `membership` rows. Login uses email+password only; after login the session is authenticated but has **no tenant context** until the user picks a workspace.

---

## 2. Decision — binding contract for all three agents

### 2.1 Schema (final state after migration 000009)

```
user            — identity, one row per human
  id                 UUID PK
  email              CITEXT NOT NULL           -- globally unique
  password_hash      TEXT NOT NULL
  full_name          TEXT NOT NULL
  phone              TEXT NULL
  avatar_url         TEXT NULL
  is_active          BOOLEAN NOT NULL DEFAULT true
  is_super_admin     BOOLEAN NOT NULL DEFAULT false   -- NEW (migration 9)
  last_login_at      TIMESTAMPTZ NULL
  failed_login_count INT NOT NULL DEFAULT 0
  locked_until       TIMESTAMPTZ NULL
  must_change_password BOOLEAN NOT NULL DEFAULT false
  metadata           JSONB NOT NULL DEFAULT '{}'
  deleted_at         TIMESTAMPTZ NULL
  created_at / updated_at / created_by / updated_by as before
  -- REMOVED: tenant_id column

  UNIQUE (email) WHERE deleted_at IS NULL        -- REPLACES UNIQUE (tenant_id, email)

membership      — NEW table: user ↔ tenant relationship
  id                 UUID PK DEFAULT gen_random_uuid()
  user_id            UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE
  tenant_id          UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE
  status             membership_status NOT NULL DEFAULT 'active'
  invited_at         TIMESTAMPTZ NULL
  joined_at          TIMESTAMPTZ NOT NULL DEFAULT now()
  left_at            TIMESTAMPTZ NULL
  metadata           JSONB NOT NULL DEFAULT '{}'
  created_at / updated_at / created_by / updated_by standard audit

  UNIQUE (user_id, tenant_id)
  INDEX  (tenant_id)
  INDEX  (user_id)

-- New enum for migration 000009:
CREATE TYPE membership_status AS ENUM ('active', 'suspended', 'invited', 'left');

user_role       — now keyed by MEMBERSHIP (was user_id)
  membership_id      UUID NOT NULL REFERENCES membership(id) ON DELETE CASCADE
  role_id            UUID NOT NULL REFERENCES role(id) ON DELETE RESTRICT
  assigned_at        TIMESTAMPTZ NOT NULL DEFAULT now()
  assigned_by        UUID NULL REFERENCES "user"(id)
  PRIMARY KEY (membership_id, role_id)

user_branch     — now keyed by MEMBERSHIP (was user_id)
  membership_id      UUID NOT NULL REFERENCES membership(id) ON DELETE CASCADE
  branch_id          UUID NOT NULL REFERENCES branch(id) ON DELETE CASCADE
  assigned_at        TIMESTAMPTZ NOT NULL DEFAULT now()
  assigned_by        UUID NULL REFERENCES "user"(id)
  PRIMARY KEY (membership_id, branch_id)
```

**Super admin:** `user.is_super_admin = true`, `user.deleted_at IS NULL`, has ZERO membership rows. Roles for super admin are inherent to the flag (no `user_role` entry needed). The `super_admin` role in the `role` table remains for documentation/reporting but is unused at the membership layer.

### 2.2 Migration 000009 — exact shape (CLEAN — no manual SQL outside migrations)

The migration must be **self-contained**: running migrations 1→9 on a fresh DB must produce the final state, using only the seed data from migration 5. No operator hand-edits.

Up steps (in order, single file `000009_user_membership.up.sql`):

1. `ALTER TABLE "user" ADD COLUMN is_super_admin BOOLEAN NOT NULL DEFAULT false;`
2. `UPDATE "user" SET is_super_admin = true WHERE tenant_id IS NULL;`
3. `CREATE TYPE membership_status AS ENUM (...)`.
4. `CREATE TABLE membership (...)` with all constraints + indexes.
5. **Backfill from user.tenant_id:**
   ```sql
   INSERT INTO membership (id, user_id, tenant_id, status, joined_at, created_at, updated_at)
   SELECT gen_random_uuid(), u.id, u.tenant_id, 'active', u.created_at, now(), now()
   FROM "user" u
   WHERE u.tenant_id IS NOT NULL AND u.deleted_at IS NULL;
   ```
6. **Add membership_id to user_role and user_branch:**
   ```sql
   ALTER TABLE user_role  ADD COLUMN membership_id UUID;
   ALTER TABLE user_branch ADD COLUMN membership_id UUID;

   UPDATE user_role ur
      SET membership_id = m.id
     FROM membership m
    WHERE m.user_id = ur.user_id;

   UPDATE user_branch ub
      SET membership_id = m.id
     FROM membership m
    WHERE m.user_id = ub.user_id;
   ```
7. Clean rows that did not map (super admin's role assignments — super admin uses `is_super_admin` flag instead):
   ```sql
   DELETE FROM user_role   WHERE membership_id IS NULL;
   DELETE FROM user_branch WHERE membership_id IS NULL;
   ```
8. Swap primary keys:
   ```sql
   ALTER TABLE user_role   DROP CONSTRAINT user_role_pkey;
   ALTER TABLE user_role   ALTER COLUMN membership_id SET NOT NULL;
   ALTER TABLE user_role   ADD CONSTRAINT user_role_pkey PRIMARY KEY (membership_id, role_id);
   ALTER TABLE user_role   ADD CONSTRAINT fk_user_role_membership FOREIGN KEY (membership_id) REFERENCES membership(id) ON DELETE CASCADE;
   ALTER TABLE user_role   DROP COLUMN user_id;

   -- same for user_branch
   ```
9. **Drop user.tenant_id + unique constraint:**
   ```sql
   ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_tenant_email_uidx;
   DROP INDEX IF EXISTS user_tenant_email_uidx;
   DROP INDEX IF EXISTS user_global_email_uidx;
   ALTER TABLE "user" DROP COLUMN tenant_id;
   CREATE UNIQUE INDEX user_email_uidx ON "user" (email) WHERE deleted_at IS NULL;
   ```
10. **Update RLS policies on `"user"`:**
    ```sql
    DROP POLICY IF EXISTS tenant_isolation        ON "user";
    DROP POLICY IF EXISTS tenant_isolation_write  ON "user";
    DROP POLICY IF EXISTS tenant_isolation_update ON "user";

    -- New policy: a user row is visible if the caller has a membership in any
    -- of the user's tenants OR the current_tenant is '__platform__' (super admin).
    CREATE POLICY user_visible ON "user"
        AS PERMISSIVE FOR SELECT
        USING (
            current_setting('app.current_tenant', true) = '__platform__'
            OR EXISTS (
                SELECT 1 FROM membership m
                 WHERE m.user_id = "user".id
                   AND m.tenant_id::text = current_setting('app.current_tenant', true)
                   AND m.status = 'active'
            )
            -- Callers can always see themselves
            OR id::text = current_setting('app.current_user', true)
        );

    CREATE POLICY user_write ON "user"
        AS PERMISSIVE FOR INSERT
        WITH CHECK (
            current_setting('app.current_tenant', true) = '__platform__'
            -- New users are created at the platform layer; a tenant admin
            -- invites/creates a new user, which writes both "user" + membership.
        );

    CREATE POLICY user_update ON "user"
        AS PERMISSIVE FOR UPDATE
        USING (
            current_setting('app.current_tenant', true) = '__platform__'
            OR id::text = current_setting('app.current_user', true)     -- self-update
            OR EXISTS (
                SELECT 1 FROM membership m
                 WHERE m.user_id = "user".id
                   AND m.tenant_id::text = current_setting('app.current_tenant', true)
                   AND m.status = 'active'
            )
        );
    ```
11. **Enable RLS on `membership`:**
    ```sql
    ALTER TABLE membership ENABLE ROW LEVEL SECURITY;
    ALTER TABLE membership FORCE ROW LEVEL SECURITY;

    CREATE POLICY membership_visible ON membership
        AS PERMISSIVE FOR SELECT
        USING (
            current_setting('app.current_tenant', true) = '__platform__'
            OR tenant_id::text = current_setting('app.current_tenant', true)
            OR user_id::text  = current_setting('app.current_user', true)   -- a user can always see their own memberships
        );

    CREATE POLICY membership_write ON membership
        AS PERMISSIVE FOR INSERT WITH CHECK (
            current_setting('app.current_tenant', true) = '__platform__'
            OR tenant_id::text = current_setting('app.current_tenant', true)
        );

    CREATE POLICY membership_update ON membership
        AS PERMISSIVE FOR UPDATE
        USING (
            current_setting('app.current_tenant', true) = '__platform__'
            OR tenant_id::text = current_setting('app.current_tenant', true)
        );

    GRANT SELECT, INSERT, UPDATE, DELETE ON membership TO lustia_app;
    ```
12. **Update refresh_token policies** — they stay as-is (migration 8 already keyed them on refresh_token.tenant_id), but the tenant_id on new refresh_token rows is now the **active membership's tenant_id**, not user.tenant_id. Schema-wise nothing changes; semantics-wise the Go layer fills the right value.

Down (`000009_user_membership.down.sql`) reverses steps 11 → 1 strictly (recreate tenant_id column, backfill user.tenant_id from the row's first membership, drop membership, etc.). A full reverse is required by the CLEAN invariant.

### 2.3 API contract (auth-service)

**Changed endpoints:**

`POST /api/v1/auth/login`

Request:
```json
{
  "email": "alice@example.com",
  "password": "s3cr3t"
}
```
(No `tenant_slug`. Field removed.)

Response — **three shapes depending on outcome:**

A. User is `is_super_admin=true` (no memberships expected):
```json
{
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "Bearer",
  "expires_at": "...",
  "scope": "platform",
  "user": { id, email, full_name, phone, avatar_url, is_active, is_super_admin, must_change_password },
  "memberships": []
}
```
JWT claim: `tenant_id = "__platform__"`, `roles = ["super_admin"]`.

B. User has exactly one active membership:
```json
{
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "Bearer",
  "expires_at": "...",
  "scope": "tenant",
  "user": { ... },
  "memberships": [{ tenant_id, tenant_name, tenant_slug, roles, branches, status }],
  "active_membership_id": "<id>"
}
```
JWT claim already includes `tenant_id`, `membership_id`, `roles`, `branches`.

C. User has multiple active memberships:
```json
{
  "access_token": "...",            // SCOPED TO USER ONLY, NO TENANT
  "refresh_token": "...",
  "token_type": "Bearer",
  "expires_at": "...",
  "scope": "user",                  // signals "must select tenant"
  "user": { ... },
  "memberships": [ ...multiple... ],
  "active_membership_id": null
}
```
JWT claim: `tenant_id = null`, `membership_id = null`, `roles = []`, `branches = []`.
Any protected endpoint except `/auth/select-tenant`, `/auth/me`, `/auth/logout` returns **`403 TENANT_NOT_SELECTED`** until the user calls select-tenant.

**New endpoint:**

`POST /api/v1/auth/select-tenant`  (auth required; any scope)

Request:
```json
{ "tenant_id": "uuid-of-a-membership-the-caller-owns" }
```

Response:
```json
{
  "access_token": "...",          // NEW, scope=tenant, tenant_id filled
  "refresh_token": "...",         // NEW, rotated
  "token_type": "Bearer",
  "expires_at": "...",
  "scope": "tenant",
  "active_membership_id": "<id>",
  "membership": { tenant_id, tenant_name, tenant_slug, roles, branches, status }
}
```

Errors:
- `403 TENANT_NOT_SELECTED` — caller had no tenant context and called a protected non-select-tenant endpoint. **New error code.**
- `403 FORBIDDEN` — tenant_id does not match any active membership of the caller.
- `403 TENANT_INACTIVE` — the tenant itself is not active.

`GET /api/v1/auth/me`

Response now includes `memberships` + `active_membership_id`:
```json
{
  "user": { ... },
  "active_membership_id": "<id> or null",
  "tenant": null | { id, name, slug, status },     // only when scope=tenant
  "memberships": [{ tenant_id, tenant_name, tenant_slug, roles, branches, status }]
}
```

**JWT claim shape (final):**

```
{
  iss, sub, aud, exp, iat, jti,
  scope: "platform" | "tenant" | "user",
  tenant_id:     "uuid" | "__platform__" | null,
  membership_id: "uuid" | null,
  roles:         string[],     // empty for scope=user
  permissions:   string[],     // empty for scope=user
  branches:      string[],
  email, full_name,
  must_change_password
}
```

### 2.4 Frontend flow

Per app:

- `/login` — email + password only (remove tenant slug field in tenant-admin + ops; platform-admin unchanged).
- **Login server action decides next hop based on `scope`:**
  - `scope=platform` → set cookies, redirect `/dashboard` (platform-admin: allow; others: show role-mismatch toast, no cookie).
  - `scope=tenant` → role check → set cookies → redirect `/dashboard` (role check per-portal unchanged).
  - `scope=user` → set cookies, redirect **`/select-tenant`**.
- **New route `/select-tenant`** (tenant-admin + ops only — platform-admin skips; its users never hit `scope=user`):
  - Server Component fetches `/auth/me`, lists memberships.
  - Client picks one → Server Action `selectTenantAction(tenant_id)` calls `/auth/select-tenant`, replaces cookies with the new scoped tokens, runs the per-portal role check (reject with role-mismatch toast if caller doesn't have the expected role in that specific membership), redirects `/dashboard`.
- **Dashboard header** gets a **workspace switcher** (tenant-admin + ops): dropdown shows `memberships`, click another one → calls `selectTenantAction`, full-page refresh. Platform-admin has no switcher.
- **Middleware** on `/dashboard`: if cookie present but JWT `scope=user` → redirect `/select-tenant`.

### 2.5 Admin create-user flow

`POST /api/v1/admin/users` (tenant_admin in a specific tenant) now:

1. Look up `user` by email globally.
2. If found AND active membership in caller's tenant exists → `409 CONFLICT`.
3. If found AND no membership in caller's tenant → create `membership` only (invite existing identity).
4. If not found → create `user` + `membership` + `user_role` + `user_branch` atomically.

Request body unchanged from current shape. Response adds `created_user: bool` + `created_membership: bool` for clarity.

### 2.6 Super admin semantics

- `is_super_admin = true` → JWT always has `tenant_id = "__platform__"`, `scope=platform`, `roles = ["super_admin"]` (synthesized server-side from the flag, NOT from `role` table lookup).
- Super admins can optionally ALSO have memberships (e.g., for QA testing). Login for super admin goes down path A even if memberships exist — they always land in platform scope. Switching to a specific tenant uses `/auth/select-tenant` same as tenant staff; result is `scope=tenant` with roles derived from memberships. They can switch back to platform scope via a new variant: `POST /auth/select-tenant { "platform": true }`. **Out of scope for Phase 2 MVP** — leave commented hook.

---

## 3. Backfill & idempotency requirements (CLEAN)

**Non-negotiable:** a fresh Postgres instance, running migrations 1→9 without operator intervention, must arrive at a working state identical to what we have now except in the new shape:

- Super admin `admin@lustia.local` exists with `is_super_admin = true`, no memberships. Login works.
- Tenant `acme-spa` exists (from seed? see below). Alice has a membership to acme-spa with `tenant_admin` role. Login works.

**Important:** the existing seed migration 5 already inserts the super admin user. The `acme-spa` tenant + `alice` user were created **via Postman / ad-hoc SQL** on the developer's machine, NOT via a migration. Those are NOT reproducible on a fresh server.

**Action required:** add an OPTIONAL seed migration `000010_seed_dev_tenant.up.sql` (or extend `000005`) that creates the dev tenant + sample tenant_admin, but ONLY when a guard env / explicit flag is set. Recommended: a separate migration file suffixed `_dev.up.sql` that the operator runs in dev only. For this ADR: keep the dev seed OUT of the main chain; ship it as `000010_seed_dev_data.up.sql` that can be skipped in prod.

`db-designer` decides the exact split and documents it in DATA_MODEL.md.

---

## 4. RLS invariants preserved

- `lustia_app` role continues to enforce all reads/writes.
- `app.current_tenant` / `app.current_user` GUC variables remain the enforcement axis.
- New policy on `"user"` and `membership` covered in §2.2.
- All operational tables (`customer`, `service`, `therapist`, `booking`, `invoice`, `payment`, `audit_log`, `branch`) untouched — they still filter by their own `tenant_id`.
- `refresh_token.tenant_id` semantics: populated from the **scoped** session's tenant (whichever membership was active when the refresh token was issued). Refresh rotation within the same session keeps the same tenant_id; calling `/auth/select-tenant` revokes old refresh and issues a new one with the new tenant.

---

## 5. Breaking changes to existing code/data

- **Postman collection**: login body loses `tenant_slug`; response grows `scope`, `memberships`, `active_membership_id`; new `/auth/select-tenant` request. Delete the old "Login (tenant-admin)" logic that assumed tenant_slug in body. **`qa-expert` or orchestrator updates** `postman_collection.json` and `test.http` alongside the backend work.
- **Frontend**: 3 forms modified. New route + component per applicable portal.
- **Existing JWTs**: after migration runs, in-flight tokens become semi-valid — `tenant_id` claim still matches, but repository queries now expect `membership_id`. Mitigation: the migration invalidates all active refresh tokens on completion (adds `UPDATE refresh_token SET revoked_at = now() WHERE revoked_at IS NULL` at the end of migration 9). All users must re-login. Acceptable since this is dev-only.

---

## 6. Open questions (defer, not blockers)

1. **Cross-tenant roles/permissions** (e.g., a user that's `tenant_admin` in acme-spa but `therapist` in beauty-clinic) — handled by the pattern: each membership has its own role set.
2. **Invited-but-not-joined flow** (email invite link) — schema supports via `status='invited'`. Controller work deferred.
3. **Leave tenant endpoint** — deferred. Status transition to `'left'`.
4. **Default tenant on login** — if >1 membership, we prompt every time. Adding a `last_used_tenant_id` on user is deferred.

---

## 7. Decision log

- 2026-04-21 — Owner requested one-login multi-tenant. Status: Accepted. Three agents to implement in parallel: `db-designer` (migration 9 + DATA_MODEL), `go-expert` (backend + API_CONTRACT), `nextjs-expert` (3 frontends). This ADR is the binding contract.
