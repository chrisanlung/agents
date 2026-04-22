-- =============================================================================
-- Migration: 000008_refresh_token_tenant_id_and_grants
-- Purpose  : (1) Fill in GRANTs missed by migration 000004 for password_reset
--            (no RLS → not auto-granted) and for DELETE on refresh_token
--            (needed by the cleanup goroutine).
--            (2) Denormalise tenant_id onto refresh_token so the refresh
--            flow can resolve tenant before loading the user — this breaks
--            the chicken-and-egg problem caused by the user-table RLS
--            policy requiring app.current_tenant set in advance.
--            (3) Relax the SELECT policy on refresh_token to USING(true) —
--            a 72-char random hash is itself the capability; RLS adds no
--            security beyond the hash lookup and blocks the refresh flow
--            when current_tenant does not already match the token's tenant.
--            Write paths (INSERT, UPDATE) remain tenant-scoped via the new
--            direct tenant_id column.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Missing grants
-- ---------------------------------------------------------------------------

-- password_reset: no RLS on this table (lookup is by hashed-token capability,
-- set before tenant context is known). GRANT was omitted in 000004.
GRANT SELECT, INSERT, UPDATE ON password_reset TO lustia_app;

-- refresh_token: the background cleanup goroutine issues DELETE; migration 4
-- only granted SELECT/INSERT/UPDATE.
GRANT DELETE ON refresh_token TO lustia_app;

-- ---------------------------------------------------------------------------
-- 2. refresh_token.tenant_id (denormalised from "user".tenant_id)
-- ---------------------------------------------------------------------------

ALTER TABLE refresh_token
    ADD COLUMN IF NOT EXISTS tenant_id UUID NULL
        CONSTRAINT fk_refresh_token_tenant
        REFERENCES tenant(id)
        ON DELETE CASCADE;

COMMENT ON COLUMN refresh_token.tenant_id IS
    'Denormalised from "user".tenant_id so the refresh flow can resolve the caller''s tenant by hash-lookup before loading the user row. NULL for platform super-admins (user.tenant_id IS NULL).';

-- Backfill existing rows by joining through "user".
-- (Temporarily elevate to BYPASSRLS role is not needed: this runs as
-- lustia_migrator which already has BYPASSRLS.)
UPDATE refresh_token rt
   SET tenant_id = u.tenant_id
  FROM "user" u
 WHERE u.id = rt.user_id
   AND rt.tenant_id IS DISTINCT FROM u.tenant_id;

-- ---------------------------------------------------------------------------
-- 3. Replace the refresh_token RLS policies with direct-tenant_id checks.
-- ---------------------------------------------------------------------------
--
-- Rationale:
--   - SELECT becomes permissive. The caller ALREADY holds the opaque
--     refresh token (72-char URL-safe random), which is the capability.
--     Requiring a matching current_tenant on SELECT means we cannot look
--     the row up before we know the tenant — exactly the problem we are
--     solving. The hash comparison + single-use rotation + revocation
--     provide the actual security.
--   - INSERT + UPDATE stay tenant-scoped via the new direct column;
--     callers cannot mint or revoke tokens for another tenant.

DROP POLICY IF EXISTS tenant_isolation        ON refresh_token;
DROP POLICY IF EXISTS tenant_isolation_write  ON refresh_token;
DROP POLICY IF EXISTS tenant_isolation_update ON refresh_token;

-- Permissive SELECT — the hash itself is the auth.
CREATE POLICY refresh_token_select ON refresh_token
    AS PERMISSIVE FOR SELECT
    USING (true);

-- INSERT must target the caller's tenant (or platform scope for super admin).
CREATE POLICY refresh_token_write ON refresh_token
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        (tenant_id IS NOT NULL AND tenant_id::text = current_setting('app.current_tenant', true))
        OR
        (tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
    );

-- UPDATE (e.g. revoke) only on rows the caller owns.
CREATE POLICY refresh_token_update ON refresh_token
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
-- 4. Supporting index
-- ---------------------------------------------------------------------------

CREATE INDEX IF NOT EXISTS refresh_token_tenant_user_idx
    ON refresh_token (tenant_id, user_id)
    WHERE revoked_at IS NULL;
