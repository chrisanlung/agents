-- =============================================================================
-- Migration: 000009_user_membership (DOWN)
-- Purpose  : Full reversal of migration 000009. Restores the "user per tenant"
--            model: re-adds user.tenant_id, re-keys user_role/user_branch back
--            to user_id, drops membership table and enum, restores old RLS
--            policies.
--
-- Steps in LIFO order (reverse of the up migration steps 13 → 1):
--   13. Cannot undo refresh token revocations (tokens are time-limited;
--       users must re-login regardless after rolling back).
--   12. Restore old user_role/user_branch RLS policies.
--   11. Drop membership RLS policies + revoke grant.
--   10. Restore old "user" RLS policies.
--    9. Restore user.tenant_id + old unique indexes.
--    8. Restore user_role and user_branch back to user_id PKs.
--    7. (No direct restoration needed — rows deleted in step 7 are restored
--       indirectly: super admin's user_role entry is re-inserted by fixed
--       UUID in step 8.)
--    6. Drop membership_id columns from user_role/user_branch.
--    5. Drop the membership backfill rows.
--    4. Drop membership table.
--    3. Drop membership_status enum.
--    2. Reset is_super_admin = false for previously-NULL-tenant_id users.
--    1. Drop is_super_admin column.
--
-- OPERATOR NOTE: refresh token revocations (step 13) cannot be reversed.
-- Users will still need to re-login after rollback. This is acceptable.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- STEP 12 (reverse): Restore old RLS policies on user_role and user_branch
-- ---------------------------------------------------------------------------

-- ---- user_role ----
DROP POLICY IF EXISTS user_role_visible ON user_role;
DROP POLICY IF EXISTS user_role_write   ON user_role;

-- Recreate old sub-select-through-user.tenant_id policies.
-- NOTE: user.tenant_id does not exist yet at this point in the rollback —
-- these policies reference a column that will be restored in step 9 below.
-- PostgreSQL stores policies as text expressions and does not validate
-- column references at CREATE POLICY time, so this is safe. The policies
-- become valid once user.tenant_id is restored.
CREATE POLICY tenant_isolation ON user_role
    AS PERMISSIVE FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM "user" u
            WHERE u.id = user_role.user_id
              AND (
                  (u.tenant_id IS NOT NULL AND u.tenant_id::text = current_setting('app.current_tenant', true))
                  OR
                  (u.tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
              )
        )
    );

CREATE POLICY tenant_isolation_write ON user_role
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM "user" u
            WHERE u.id = user_role.user_id
              AND (
                  (u.tenant_id IS NOT NULL AND u.tenant_id::text = current_setting('app.current_tenant', true))
                  OR
                  (u.tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
              )
        )
    );

-- ---- user_branch ----
DROP POLICY IF EXISTS user_branch_visible ON user_branch;
DROP POLICY IF EXISTS user_branch_write   ON user_branch;

CREATE POLICY tenant_isolation ON user_branch
    AS PERMISSIVE FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM "user" u
            WHERE u.id = user_branch.user_id
              AND u.tenant_id IS NOT NULL
              AND u.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

CREATE POLICY tenant_isolation_write ON user_branch
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM "user" u
            WHERE u.id = user_branch.user_id
              AND u.tenant_id IS NOT NULL
              AND u.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

-- ---------------------------------------------------------------------------
-- STEP 11 (reverse): Disable membership RLS + revoke grant
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS membership_visible ON membership;
DROP POLICY IF EXISTS membership_write   ON membership;
DROP POLICY IF EXISTS membership_update  ON membership;

-- Note: DISABLE ROW LEVEL SECURITY automatically drops FORCE as well.
ALTER TABLE membership DISABLE ROW LEVEL SECURITY;

REVOKE SELECT, INSERT, UPDATE, DELETE ON membership FROM lustia_app;

-- ---------------------------------------------------------------------------
-- STEP 10 (reverse): Restore old "user" RLS policies
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS user_visible ON "user";
DROP POLICY IF EXISTS user_write   ON "user";
DROP POLICY IF EXISTS user_update  ON "user";

-- NOTE: These policies reference user.tenant_id which is restored in step 9.
-- As above, PostgreSQL defers column reference validation; safe to create now.
CREATE POLICY tenant_isolation ON "user"
    AS PERMISSIVE FOR SELECT
    USING (
        (tenant_id IS NOT NULL AND tenant_id::text = current_setting('app.current_tenant', true))
        OR
        (tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
    );

CREATE POLICY tenant_isolation_write ON "user"
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        (tenant_id IS NOT NULL AND tenant_id::text = current_setting('app.current_tenant', true))
        OR
        (tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
    );

CREATE POLICY tenant_isolation_update ON "user"
    AS PERMISSIVE FOR UPDATE
    USING (
        (tenant_id IS NOT NULL AND tenant_id::text = current_setting('app.current_tenant', true))
        OR
        (tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
    )
    WITH CHECK (
        (tenant_id IS NOT NULL AND tenant_id::text = current_setting('app.current_tenant', true))
        OR
        (tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
    );

-- ---------------------------------------------------------------------------
-- STEP 9 (reverse): Restore user.tenant_id and old unique constraint
--
-- Backfill strategy: give each user their first active membership's tenant_id.
-- If a user somehow has multiple memberships (shouldn't occur in dev but
-- logically possible), the JOIN picks one arbitrarily (DISTINCT ON user_id
-- with an order by joined_at). Super admins (is_super_admin=true) map to NULL.
-- ---------------------------------------------------------------------------

-- Drop the new global unique index first.
DROP INDEX IF EXISTS user_email_uidx;

-- Re-add tenant_id column.
ALTER TABLE "user"
    ADD COLUMN IF NOT EXISTS tenant_id UUID NULL
        CONSTRAINT fk_user_tenant
        REFERENCES tenant(id)
        ON DELETE RESTRICT;

-- Backfill tenant_id from the earliest active membership per user.
UPDATE "user" u
   SET tenant_id = m.tenant_id
  FROM (
      SELECT DISTINCT ON (user_id)
          user_id,
          tenant_id
        FROM membership
       WHERE status = 'active'
       ORDER BY user_id, joined_at ASC
  ) m
 WHERE u.id = m.user_id;

-- Super admins retain tenant_id = NULL (they had no membership; no row in
-- the sub-query above will match them).

-- Restore old partial unique indexes.
CREATE UNIQUE INDEX IF NOT EXISTS user_tenant_email_uidx
    ON "user" (tenant_id, email)
    WHERE tenant_id IS NOT NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS user_global_email_uidx
    ON "user" (email)
    WHERE tenant_id IS NULL AND deleted_at IS NULL;

-- Restore FK supporting index.
CREATE INDEX IF NOT EXISTS user_tenant_id_idx ON "user" (tenant_id);

-- ---------------------------------------------------------------------------
-- STEP 8 (reverse): Restore user_role and user_branch back to user_id PKs
--
-- The reversal sequence for each table:
--   a. Add user_id column (nullable for backfill).
--   b. Backfill user_id from membership.user_id via membership_id.
--   c. Restore super admin's user_role entry (was deleted in step 7 of up).
--   d. Drop new PK + FK.
--   e. Make user_id NOT NULL, add old PK and old FK.
--   f. Drop membership_id column.
-- ---------------------------------------------------------------------------

-- ---- user_role ----

-- a. Add user_id column.
ALTER TABLE user_role
    ADD COLUMN IF NOT EXISTS user_id UUID NULL;

-- b. Backfill from membership.
UPDATE user_role ur
   SET user_id = m.user_id
  FROM membership m
 WHERE m.id = ur.membership_id
   AND ur.user_id IS NULL;

-- c. Restore the super admin role assignment deleted by step 7.
--    The super admin's membership_id is NULL in the new shape, but we need
--    to re-insert into the old shape where user_id is the key.
--    Use fixed UUIDs from migration 000005: user a0000000…0001, role b0000000…0001.
--    We INSERT directly with user_id / role_id and no membership_id,
--    but at this stage user_role still has membership_id as PK.
--    We must first drop the new PK before we can insert without a membership_id.

-- d. Drop new PK and FK on user_role.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_role'
          AND constraint_name = 'user_role_pkey'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_role DROP CONSTRAINT user_role_pkey;
    END IF;
END
$$;

ALTER TABLE user_role DROP CONSTRAINT IF EXISTS fk_user_role_membership;

-- Now we can insert the super admin row (membership_id will be NULL; we
-- drop membership_id column in sub-step f anyway).
INSERT INTO user_role (user_id, role_id, assigned_at)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'b0000000-0000-0000-0000-000000000001',
    now()
)
ON CONFLICT DO NOTHING;

-- e. Make user_id NOT NULL, restore old PK and FK.
ALTER TABLE user_role ALTER COLUMN user_id SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_role'
          AND constraint_name = 'pk_user_role'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_role ADD CONSTRAINT pk_user_role
            PRIMARY KEY (user_id, role_id);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_role'
          AND constraint_name = 'fk_user_role_user'
    ) THEN
        ALTER TABLE user_role
            ADD CONSTRAINT fk_user_role_user
            FOREIGN KEY (user_id)
            REFERENCES "user"(id)
            ON DELETE CASCADE;
    END IF;
END
$$;

-- f. Drop membership_id column.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'user_role' AND column_name = 'membership_id'
    ) THEN
        ALTER TABLE user_role DROP COLUMN membership_id;
    END IF;
END
$$;

-- Restore old user_id index.
CREATE INDEX IF NOT EXISTS user_role_user_id_idx ON user_role (user_id);

-- ---- user_branch ----

-- a. Add user_id column.
ALTER TABLE user_branch
    ADD COLUMN IF NOT EXISTS user_id UUID NULL;

-- b. Backfill from membership.
UPDATE user_branch ub
   SET user_id = m.user_id
  FROM membership m
 WHERE m.id = ub.membership_id
   AND ub.user_id IS NULL;

-- d. Drop new PK and FK.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_branch'
          AND constraint_name = 'user_branch_pkey'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_branch DROP CONSTRAINT user_branch_pkey;
    END IF;
END
$$;

ALTER TABLE user_branch DROP CONSTRAINT IF EXISTS fk_user_branch_membership;

-- e. Make user_id NOT NULL, restore old PK and FK.
ALTER TABLE user_branch ALTER COLUMN user_id SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_branch'
          AND constraint_name = 'pk_user_branch'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE user_branch ADD CONSTRAINT pk_user_branch
            PRIMARY KEY (user_id, branch_id);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'user_branch'
          AND constraint_name = 'fk_user_branch_user'
    ) THEN
        ALTER TABLE user_branch
            ADD CONSTRAINT fk_user_branch_user
            FOREIGN KEY (user_id)
            REFERENCES "user"(id)
            ON DELETE CASCADE;
    END IF;
END
$$;

-- f. Drop membership_id column.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'user_branch' AND column_name = 'membership_id'
    ) THEN
        ALTER TABLE user_branch DROP COLUMN membership_id;
    END IF;
END
$$;

-- Restore old user_id index.
CREATE INDEX IF NOT EXISTS user_branch_user_id_idx ON user_branch (user_id);

-- ---------------------------------------------------------------------------
-- STEP 5+4 (reverse): Drop membership table
--
-- CASCADE is intentional here: the migration trigger is owned by the table.
-- All data is destroyed. The up migration recreates it from user.tenant_id.
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS membership CASCADE;

-- ---------------------------------------------------------------------------
-- STEP 3 (reverse): Drop membership_status enum
-- ---------------------------------------------------------------------------
DROP TYPE IF EXISTS membership_status;

-- ---------------------------------------------------------------------------
-- STEP 2 (reverse): Reset is_super_admin to false for former NULL-tenant users
--
-- In a rollback these rows now have tenant_id restored (step 9 above left
-- them with tenant_id = NULL for true super admins). Reset the flag since
-- tenant_id IS NULL is back to being the sentinel.
-- ---------------------------------------------------------------------------
UPDATE "user"
   SET is_super_admin = false
 WHERE is_super_admin = true;

-- ---------------------------------------------------------------------------
-- STEP 1 (reverse): Drop is_super_admin column
-- ---------------------------------------------------------------------------
ALTER TABLE "user" DROP COLUMN IF EXISTS is_super_admin;
