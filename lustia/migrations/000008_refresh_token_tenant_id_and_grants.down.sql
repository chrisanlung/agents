-- =============================================================================
-- Reverse 000008.
-- =============================================================================

DROP INDEX IF EXISTS refresh_token_tenant_user_idx;

-- Restore the original tenant-isolation policies (user-join subqueries).
DROP POLICY IF EXISTS refresh_token_select ON refresh_token;
DROP POLICY IF EXISTS refresh_token_write  ON refresh_token;
DROP POLICY IF EXISTS refresh_token_update ON refresh_token;

CREATE POLICY tenant_isolation ON refresh_token
    AS PERMISSIVE FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM "user" u
            WHERE u.id = refresh_token.user_id
              AND (
                  (u.tenant_id IS NOT NULL AND u.tenant_id::text = current_setting('app.current_tenant', true))
                  OR
                  (u.tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
              )
        )
    );

CREATE POLICY tenant_isolation_write ON refresh_token
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM "user" u
            WHERE u.id = refresh_token.user_id
              AND (
                  (u.tenant_id IS NOT NULL AND u.tenant_id::text = current_setting('app.current_tenant', true))
                  OR
                  (u.tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
              )
        )
    );

CREATE POLICY tenant_isolation_update ON refresh_token
    AS PERMISSIVE FOR UPDATE
    USING (
        EXISTS (
            SELECT 1 FROM "user" u
            WHERE u.id = refresh_token.user_id
              AND (
                  (u.tenant_id IS NOT NULL AND u.tenant_id::text = current_setting('app.current_tenant', true))
                  OR
                  (u.tenant_id IS NULL AND current_setting('app.current_tenant', true) = '__platform__')
              )
        )
    );

ALTER TABLE refresh_token DROP COLUMN IF EXISTS tenant_id;

REVOKE DELETE ON refresh_token FROM lustia_app;
REVOKE SELECT, INSERT, UPDATE ON password_reset FROM lustia_app;
