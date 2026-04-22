-- =============================================================================
-- Migration: 000009_user_membership
-- Purpose  : Implement ADR 0007 — User–Membership Pattern.
--            Switches from "one user row per tenant" to "one user row per
--            human identity + membership rows for tenant relationships".
--
-- Supersedes the previous model established by migrations 000002 and 000003.
-- Reference: docs/DECISIONS/0007-user-membership-pattern.md
--
-- OPERATOR NOTES:
--   • This migration revokes ALL active refresh tokens at the end (step 13).
--     Every user WILL be logged out and must re-authenticate. This is
--     intentional — in-flight JWTs reference the old schema shape.
--   • Migration is safe to re-run after a partial failure: most DDL steps use
--     IF NOT EXISTS / IF EXISTS / ON CONFLICT DO NOTHING guards.
--   • Destructive steps that cannot be guarded (DROP COLUMN, DROP CONSTRAINT,
--     DELETE) are noted with comments explaining the assumption.
--   • Run this migration as lustia_migrator (BYPASSRLS) as with all migrations.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- STEP 1: Add is_super_admin column to "user"
--
-- Replaces the "tenant_id IS NULL" sentinel for platform admins.
-- Defaults to false for all existing rows; step 2 backfills super admins.
-- ---------------------------------------------------------------------------
ALTER TABLE "user"
    ADD COLUMN IF NOT EXISTS is_super_admin BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN "user".is_super_admin IS
    'When true, this user is a platform super-admin. JWT scope=platform, '
    'tenant_id=''__platform__'', roles=[''super_admin''] are synthesised '
    'server-side — no membership or user_role rows are required. '
    'Added in migration 000009 (ADR 0007).';

-- ---------------------------------------------------------------------------
-- STEP 2: Backfill is_super_admin from the old tenant_id IS NULL sentinel
--
-- Assumption: the only user(s) with tenant_id IS NULL are platform super
-- admins (established by migration 000005 seed). If any non-admin user
-- somehow exists with tenant_id IS NULL they would also be promoted here —
-- that state was not valid in the previous schema.
-- ---------------------------------------------------------------------------
UPDATE "user"
   SET is_super_admin = true
 WHERE tenant_id IS NULL;

-- ---------------------------------------------------------------------------
-- STEP 3: Create membership_status enum
--
-- Exact values in exact order as specified in ADR §2.1.
-- IF NOT EXISTS guard: safe to re-run if the type was already created before
-- a prior failure.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'membership_status'
    ) THEN
        CREATE TYPE membership_status AS ENUM (
            'active',
            'suspended',
            'invited',
            'left'
        );
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- STEP 4: Create membership table
--
-- The join entity between a user (identity) and a tenant (workspace).
-- Each row = one human's relationship to one company.
-- Audit columns follow the project standard (created_at, updated_at,
-- created_by, updated_by). No soft-delete: leaving a tenant is expressed
-- via status='left' and left_at, preserving history.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS membership (
    id          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID            NOT NULL
                    CONSTRAINT fk_membership_user
                    REFERENCES "user"(id)
                    ON DELETE CASCADE,
    tenant_id   UUID            NOT NULL
                    CONSTRAINT fk_membership_tenant
                    REFERENCES tenant(id)
                    ON DELETE CASCADE,
    status      membership_status NOT NULL DEFAULT 'active',
    invited_at  TIMESTAMPTZ     NULL,
    joined_at   TIMESTAMPTZ     NOT NULL DEFAULT now(),
    left_at     TIMESTAMPTZ     NULL,
    metadata    JSONB           NOT NULL DEFAULT '{}'::jsonb,
    -- Audit
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    created_by  UUID            NULL,
    updated_by  UUID            NULL,

    CONSTRAINT uq_membership_user_tenant UNIQUE (user_id, tenant_id)
);

COMMENT ON TABLE membership IS
    'Workspace-membership pattern (ADR 0007): one row per user-tenant '
    'relationship. A user may belong to multiple tenants. Super admins '
    '(is_super_admin=true) have zero membership rows by default.';

-- Index on tenant_id: every FK column is indexed; tenant queries are common.
CREATE INDEX IF NOT EXISTS membership_tenant_id_idx ON membership (tenant_id);
-- Index on user_id: membership lookup by user is the hot path on login.
CREATE INDEX IF NOT EXISTS membership_user_id_idx   ON membership (user_id);
-- Partial index for active memberships — most queries care only about active.
CREATE INDEX IF NOT EXISTS membership_active_user_idx
    ON membership (user_id)
    WHERE status = 'active';

CREATE TRIGGER trg_membership_updated_at
    BEFORE UPDATE ON membership
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- STEP 5: Backfill membership rows from user.tenant_id
--
-- Every non-deleted tenant-scoped user gets one 'active' membership.
-- Super admins (tenant_id IS NULL) are skipped — they need no membership row.
-- joined_at is set to user.created_at as the best available approximation.
--
-- ON CONFLICT DO NOTHING: safe to re-run if some rows already exist.
-- ---------------------------------------------------------------------------
INSERT INTO membership (id, user_id, tenant_id, status, joined_at, created_at, updated_at)
SELECT
    gen_random_uuid(),
    u.id,
    u.tenant_id,
    'active'::membership_status,
    u.created_at,
    now(),
    now()
FROM "user" u
WHERE u.tenant_id IS NOT NULL
  AND u.deleted_at IS NULL
ON CONFLICT (user_id, tenant_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- STEP 6: Add membership_id column to user_role and user_branch
--
-- Initially nullable so we can backfill before enforcing NOT NULL.
-- ---------------------------------------------------------------------------
ALTER TABLE user_role
    ADD COLUMN IF NOT EXISTS membership_id UUID NULL;

ALTER TABLE user_branch
    ADD COLUMN IF NOT EXISTS membership_id UUID NULL;

-- Backfill user_role.membership_id: join through the newly created membership
-- rows. For super admins (who have no membership), this leaves NULL — step 7
-- cleans those up.
UPDATE user_role ur
   SET membership_id = m.id
  FROM membership m
 WHERE m.user_id = ur.user_id
   AND ur.membership_id IS NULL;

-- Backfill user_branch.membership_id similarly.
UPDATE user_branch ub
   SET membership_id = m.id
  FROM membership m
 WHERE m.user_id = ub.user_id
   AND ub.membership_id IS NULL;

-- ---------------------------------------------------------------------------
-- STEP 7: Delete orphan rows that did not map to a membership
--
-- These are role/branch assignments belonging to super admins. Super admins
-- use the is_super_admin flag; their user_role entries are now obsolete.
-- The delete is non-reversible in this step; the down migration restores the
-- super admin's role assignment from fixed UUIDs.
--
-- Assumption: no non-super-admin user has membership_id IS NULL at this point.
-- If the backfill in step 6 missed rows (e.g., deleted users with role
-- assignments), they are also cleaned up here — they were already orphaned.
-- ---------------------------------------------------------------------------
DELETE FROM user_role
 WHERE membership_id IS NULL;

DELETE FROM user_branch
 WHERE membership_id IS NULL;

-- ---------------------------------------------------------------------------
-- STEP 8: Swap primary keys and FKs on user_role
-- ---------------------------------------------------------------------------

-- Drop the old PK constraint (was (user_id, role_id)).
-- Cannot use IF NOT EXISTS on DROP CONSTRAINT; if migration failed mid-step
-- on a re-run, the PK may already be gone. Guard with DO block.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_role'
          AND constraint_name = 'pk_user_role'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_role DROP CONSTRAINT pk_user_role;
    END IF;
END
$$;

-- Make membership_id NOT NULL now that orphans are gone.
ALTER TABLE user_role ALTER COLUMN membership_id SET NOT NULL;

-- New PK is (membership_id, role_id) — consistent with ADR §2.1.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_role'
          AND constraint_name = 'user_role_pkey'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_role ADD CONSTRAINT user_role_pkey
            PRIMARY KEY (membership_id, role_id);
    END IF;
END
$$;

-- Add FK from user_role.membership_id → membership(id).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_role'
          AND constraint_name = 'fk_user_role_membership'
    ) THEN
        ALTER TABLE user_role
            ADD CONSTRAINT fk_user_role_membership
            FOREIGN KEY (membership_id)
            REFERENCES membership(id)
            ON DELETE CASCADE;
    END IF;
END
$$;

-- Drop the old user_id column, its FK, supporting index, AND any RLS policy
-- that references user_id (migration 4 created tenant_isolation policies that
-- subquery user_role.user_id). Policies must come off first, then index, then
-- FK, then the column itself — Postgres refuses DROP COLUMN while any
-- dependent object still exists.
-- Step 12 recreates the policies fresh against membership_id.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'user_role' AND column_name = 'user_id'
    ) THEN
        DROP POLICY IF EXISTS tenant_isolation        ON user_role;
        DROP POLICY IF EXISTS tenant_isolation_write  ON user_role;
        DROP POLICY IF EXISTS tenant_isolation_update ON user_role;
        DROP INDEX  IF EXISTS user_role_user_id_idx;
        ALTER TABLE user_role DROP CONSTRAINT IF EXISTS fk_user_role_user;
        ALTER TABLE user_role DROP COLUMN user_id;
    END IF;
END
$$;

-- New index on membership_id is the PK — no separate index needed.
-- Retain role_id index for reverse lookups (which memberships have a role).
-- user_role_role_id_idx already exists from migration 000002; keep it.

-- ---------------------------------------------------------------------------
-- STEP 8b: Swap primary keys and FKs on user_branch (mirrors user_role)
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_branch'
          AND constraint_name = 'pk_user_branch'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_branch DROP CONSTRAINT pk_user_branch;
    END IF;
END
$$;

ALTER TABLE user_branch ALTER COLUMN membership_id SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_branch'
          AND constraint_name = 'user_branch_pkey'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_branch ADD CONSTRAINT user_branch_pkey
            PRIMARY KEY (membership_id, branch_id);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_branch'
          AND constraint_name = 'fk_user_branch_membership'
    ) THEN
        ALTER TABLE user_branch
            ADD CONSTRAINT fk_user_branch_membership
            FOREIGN KEY (membership_id)
            REFERENCES membership(id)
            ON DELETE CASCADE;
    END IF;
END
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'user_branch' AND column_name = 'user_id'
    ) THEN
        DROP POLICY IF EXISTS tenant_isolation        ON user_branch;
        DROP POLICY IF EXISTS tenant_isolation_write  ON user_branch;
        DROP POLICY IF EXISTS tenant_isolation_update ON user_branch;
        DROP INDEX  IF EXISTS user_branch_user_id_idx;
        ALTER TABLE user_branch DROP CONSTRAINT IF EXISTS fk_user_branch_user;
        ALTER TABLE user_branch DROP COLUMN user_id;
    END IF;
END
$$;

-- New index on membership_id (PK covers the leading column; add explicit
-- index for reverse lookup by branch_id to resolve "which memberships at
-- this branch" — branch_id is already the second PK column; the separate
-- index below covers queries that start from branch_id alone).
CREATE INDEX IF NOT EXISTS user_branch_branch_id_idx
    ON user_branch (branch_id);

-- ---------------------------------------------------------------------------
-- STEP 9: Drop user.tenant_id and replace the unique constraint
--
-- The old dual-partial-index uniqueness model (per-tenant + global-null)
-- is replaced by a single global partial unique index on email.
-- ---------------------------------------------------------------------------

-- Drop old partial unique indexes. Use DROP INDEX IF EXISTS — idempotent.
DROP INDEX IF EXISTS user_tenant_email_uidx;
DROP INDEX IF EXISTS user_global_email_uidx;

-- Drop old dependents on user.tenant_id in the right order:
--   RLS policies → FK → index → column. Step 10 recreates policies against
--   the new membership-based model.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'user' AND column_name = 'tenant_id'
    ) THEN
        DROP POLICY IF EXISTS tenant_isolation        ON "user";
        DROP POLICY IF EXISTS tenant_isolation_write  ON "user";
        DROP POLICY IF EXISTS tenant_isolation_update ON "user";
        ALTER TABLE "user" DROP CONSTRAINT IF EXISTS fk_user_tenant;
        DROP INDEX IF EXISTS user_tenant_id_idx;
        ALTER TABLE "user" DROP COLUMN tenant_id;
    END IF;
END
$$;

-- New global unique index: email must be unique across all active (non-deleted) users.
-- This enforces the "one login per human" invariant from ADR §2.1.
CREATE UNIQUE INDEX IF NOT EXISTS user_email_uidx
    ON "user" (email)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- STEP 10: Replace RLS policies on "user"
--
-- Old policies used tenant_id column (now dropped). New policies use
-- membership cross-reference or is_super_admin flag.
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON "user";
DROP POLICY IF EXISTS tenant_isolation_write  ON "user";
DROP POLICY IF EXISTS tenant_isolation_update ON "user";

-- Also drop the new-name policies in case of re-run after partial failure.
DROP POLICY IF EXISTS user_visible ON "user";
DROP POLICY IF EXISTS user_write   ON "user";
DROP POLICY IF EXISTS user_update  ON "user";

-- SELECT: visible if (a) platform scope, (b) caller shares a tenant with the
-- user via an active membership, or (c) the row is the caller themselves.
CREATE POLICY user_visible ON "user"
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
        OR EXISTS (
            SELECT 1
              FROM membership m
             WHERE m.user_id  = "user".id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
               AND m.status = 'active'
        )
        -- A user can always see themselves (e.g. during /auth/me with scope=user).
        OR id::text = current_setting('app.current_user', true)
    );

-- INSERT: new users can only be created by the platform layer. Tenant admins
-- calling POST /admin/users go through the platform layer which sets
-- app.current_tenant = '__platform__' for the user-creation step, then
-- creates the membership separately under the tenant scope.
CREATE POLICY user_write ON "user"
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        current_setting('app.current_tenant', true) = '__platform__'
    );

-- UPDATE: platform scope, or a tenant admin in the same tenant, or self.
CREATE POLICY user_update ON "user"
    AS PERMISSIVE FOR UPDATE
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
        OR id::text = current_setting('app.current_user', true)
        OR EXISTS (
            SELECT 1
              FROM membership m
             WHERE m.user_id  = "user".id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
               AND m.status = 'active'
        )
    );

-- ---------------------------------------------------------------------------
-- STEP 11: Enable RLS on membership + create policies + grant
-- ---------------------------------------------------------------------------
ALTER TABLE membership ENABLE ROW LEVEL SECURITY;
ALTER TABLE membership FORCE ROW LEVEL SECURITY;

-- Drop in case of re-run.
DROP POLICY IF EXISTS membership_visible ON membership;
DROP POLICY IF EXISTS membership_write   ON membership;
DROP POLICY IF EXISTS membership_update  ON membership;

-- SELECT: platform scope, or caller's tenant, or the user themselves
-- (so that /auth/me can enumerate memberships regardless of active scope).
CREATE POLICY membership_visible ON membership
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
        OR tenant_id::text = current_setting('app.current_tenant', true)
        OR user_id::text   = current_setting('app.current_user', true)
    );

-- INSERT: platform scope, or a request already scoped to that tenant.
CREATE POLICY membership_write ON membership
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        current_setting('app.current_tenant', true) = '__platform__'
        OR tenant_id::text = current_setting('app.current_tenant', true)
    );

-- UPDATE: platform scope, or a request scoped to that tenant (tenant admin
-- can suspend/reactivate members within their own tenant).
CREATE POLICY membership_update ON membership
    AS PERMISSIVE FOR UPDATE
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
        OR tenant_id::text = current_setting('app.current_tenant', true)
    );

-- Grant: mirrors the pattern from migration 000004 for similar tables.
GRANT SELECT, INSERT, UPDATE, DELETE ON membership TO lustia_app;

-- ---------------------------------------------------------------------------
-- STEP 12: Update RLS policies on user_role and user_branch
--
-- Old policies sub-selected through user.tenant_id (now dropped).
-- New policies sub-select through membership.tenant_id.
-- ---------------------------------------------------------------------------

-- ---- user_role ----
DROP POLICY IF EXISTS tenant_isolation       ON user_role;
DROP POLICY IF EXISTS tenant_isolation_write ON user_role;

-- Also drop in case of re-run.
DROP POLICY IF EXISTS user_role_visible ON user_role;
DROP POLICY IF EXISTS user_role_write   ON user_role;

CREATE POLICY user_role_visible ON user_role
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
        OR EXISTS (
            SELECT 1
              FROM membership m
             WHERE m.id         = user_role.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

CREATE POLICY user_role_write ON user_role
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        current_setting('app.current_tenant', true) = '__platform__'
        OR EXISTS (
            SELECT 1
              FROM membership m
             WHERE m.id         = user_role.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

-- ---- user_branch ----
DROP POLICY IF EXISTS tenant_isolation       ON user_branch;
DROP POLICY IF EXISTS tenant_isolation_write ON user_branch;

-- Also drop in case of re-run.
DROP POLICY IF EXISTS user_branch_visible ON user_branch;
DROP POLICY IF EXISTS user_branch_write   ON user_branch;

CREATE POLICY user_branch_visible ON user_branch
    AS PERMISSIVE FOR SELECT
    USING (
        current_setting('app.current_tenant', true) = '__platform__'
        OR EXISTS (
            SELECT 1
              FROM membership m
             WHERE m.id         = user_branch.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

CREATE POLICY user_branch_write ON user_branch
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        current_setting('app.current_tenant', true) = '__platform__'
        OR EXISTS (
            SELECT 1
              FROM membership m
             WHERE m.id         = user_branch.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

-- ---------------------------------------------------------------------------
-- STEP 13: Revoke all active refresh tokens
--
-- Required by ADR §5: in-flight tokens carry the old JWT shape (no
-- membership_id claim; tenant_id derived from user.tenant_id which is now
-- gone). All sessions must re-authenticate so they receive tokens in the new
-- shape. Acceptable: this is a dev-only migration run.
--
-- This step is intentionally non-guarded. Re-running the migration after a
-- prior full success will find revoked_at IS NOT NULL on all rows and the
-- UPDATE will touch zero rows — safe.
-- ---------------------------------------------------------------------------
UPDATE refresh_token
   SET revoked_at = now()
 WHERE revoked_at IS NULL;
