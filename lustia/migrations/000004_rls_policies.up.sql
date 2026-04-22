-- =============================================================================
-- Migration: 000004_rls_policies
-- Purpose  : Enable Row-Level Security and create isolation policies on all
--            tenant-scoped tables. Grant DML to lustia_app.
--
-- RLS strategy:
--   - At the start of every request transaction the app calls:
--       SET LOCAL app.current_tenant = '<tenant_uuid>';
--   - USING clause filters rows: only rows where tenant_id matches the
--     setting are visible.
--   - WITH CHECK clause prevents INSERT/UPDATE of rows with a foreign tenant_id.
--   - current_setting('app.current_tenant', true) returns NULL (not an error)
--     when the setting is absent, causing all row checks to fail — effectively
--     denying all access until the setting is explicitly set.
--
-- Which tables get RLS:
--   tenant-scoped operational tables + auth tables that carry tenant_id.
--
-- Which tables do NOT get RLS:
--   tenant, role, permission, role_permission, audit_log.
--   These are platform-level tables managed by super_admin only.
--   The application layer enforces access; RLS would add no isolation benefit
--   here because they are intentionally cross-tenant (see DATA_MODEL.md §RLS).
--
-- Super-admin bypass:
--   lustia_migrator has BYPASSRLS (set in migration 000001).
--   Super-admin requests use the SET ROLE lustia_migrator approach, OR
--   a dedicated lustia_super_admin DB role with BYPASSRLS can be created.
--   See DATA_MODEL.md §RLS for the full decision.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Helper: grants for lustia_app on platform-level tables (no RLS needed)
-- ---------------------------------------------------------------------------
GRANT SELECT, INSERT, UPDATE, DELETE ON tenant           TO lustia_app;
GRANT SELECT                          ON role             TO lustia_app;
GRANT SELECT                          ON permission       TO lustia_app;
GRANT SELECT                          ON role_permission  TO lustia_app;
GRANT SELECT, INSERT                  ON audit_log        TO lustia_app;

-- lustia_migrator gets full access (BYPASSRLS already set on role)
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO lustia_migrator;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO lustia_migrator;

-- ---------------------------------------------------------------------------
-- 1. branch (has tenant_id)
-- ---------------------------------------------------------------------------
ALTER TABLE branch ENABLE ROW LEVEL SECURITY;
ALTER TABLE branch FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON branch
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON branch
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON branch
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON branch TO lustia_app;

-- ---------------------------------------------------------------------------
-- 2. "user" (has tenant_id; NULL for super_admin — policy handles NULL)
-- ---------------------------------------------------------------------------
ALTER TABLE "user" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "user" FORCE ROW LEVEL SECURITY;

-- For super_admin (tenant_id IS NULL): the setting 'app.current_tenant' will
-- be set to a special sentinel value '__platform__' by the application when
-- acting as super_admin, and we add an OR clause to allow NULL tenant rows
-- through for those connections that set the sentinel.
-- Normal tenant connections: tenant_id IS NOT NULL and must match.
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

GRANT SELECT, INSERT, UPDATE ON "user" TO lustia_app;
-- DELETE on "user" is intentionally withheld; soft-delete via deleted_at only.

-- ---------------------------------------------------------------------------
-- 3. user_role (tenant scoping via user FK — no direct tenant_id column)
--    RLS joins through the user table; simpler to scope at app layer.
--    We enable RLS here but delegate the join-based filter to app queries.
--    Policy uses a sub-select to check the user's tenant.
-- ---------------------------------------------------------------------------
ALTER TABLE user_role ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_role FORCE ROW LEVEL SECURITY;

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

GRANT SELECT, INSERT, DELETE ON user_role TO lustia_app;

-- ---------------------------------------------------------------------------
-- 4. user_branch (similar to user_role — tenant scoping via user FK)
-- ---------------------------------------------------------------------------
ALTER TABLE user_branch ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_branch FORCE ROW LEVEL SECURITY;

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

GRANT SELECT, INSERT, DELETE ON user_branch TO lustia_app;

-- ---------------------------------------------------------------------------
-- 5. refresh_token (tenant scoping via user FK)
-- ---------------------------------------------------------------------------
ALTER TABLE refresh_token ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_token FORCE ROW LEVEL SECURITY;

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

GRANT SELECT, INSERT, UPDATE ON refresh_token TO lustia_app;

-- ---------------------------------------------------------------------------
-- 6. customer
-- ---------------------------------------------------------------------------
ALTER TABLE customer ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON customer
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON customer
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON customer
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON customer TO lustia_app;

-- ---------------------------------------------------------------------------
-- 7. service
-- ---------------------------------------------------------------------------
ALTER TABLE service ENABLE ROW LEVEL SECURITY;
ALTER TABLE service FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON service
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON service
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON service
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON service TO lustia_app;

-- ---------------------------------------------------------------------------
-- 8. therapist
-- ---------------------------------------------------------------------------
ALTER TABLE therapist ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON therapist
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON therapist
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON therapist
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON therapist TO lustia_app;

-- ---------------------------------------------------------------------------
-- 9. therapist_service (tenant scoping via therapist FK)
-- ---------------------------------------------------------------------------
ALTER TABLE therapist_service ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_service FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON therapist_service
    AS PERMISSIVE FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM therapist t
            WHERE t.id = therapist_service.therapist_id
              AND t.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

CREATE POLICY tenant_isolation_write ON therapist_service
    AS PERMISSIVE FOR INSERT
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM therapist t
            WHERE t.id = therapist_service.therapist_id
              AND t.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

GRANT SELECT, INSERT, DELETE ON therapist_service TO lustia_app;

-- ---------------------------------------------------------------------------
-- 10. therapist_availability
-- ---------------------------------------------------------------------------
ALTER TABLE therapist_availability ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_availability FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON therapist_availability
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON therapist_availability
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON therapist_availability
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE, DELETE ON therapist_availability TO lustia_app;

-- ---------------------------------------------------------------------------
-- 11. booking
-- ---------------------------------------------------------------------------
ALTER TABLE booking ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON booking
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON booking
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON booking
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE ON booking TO lustia_app;

-- ---------------------------------------------------------------------------
-- 12. invoice
-- ---------------------------------------------------------------------------
ALTER TABLE invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON invoice
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON invoice
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON invoice
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE ON invoice TO lustia_app;

-- ---------------------------------------------------------------------------
-- 13. payment
-- ---------------------------------------------------------------------------
ALTER TABLE payment ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON payment
    AS PERMISSIVE FOR SELECT
    USING (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_write ON payment
    AS PERMISSIVE FOR INSERT
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

CREATE POLICY tenant_isolation_update ON payment
    AS PERMISSIVE FOR UPDATE
    USING  (tenant_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));

GRANT SELECT, INSERT, UPDATE ON payment TO lustia_app;
