-- =============================================================================
-- Migration 000034 — Add DELETE policies on user_role and user_branch.
--
-- Bug fix: migrations 000004 and 000009 created RLS policies for SELECT and
-- INSERT only on user_role and user_branch. Combined with `FORCE ROW LEVEL
-- SECURITY`, this makes DELETE silently affect zero rows. The user-update
-- handler does `DELETE WHERE membership_id = ?` then re-INSERTs the new role
-- set; with the silent DELETE, re-INSERT collides with the existing row on
-- the (membership_id, role_id) primary key and surfaces as a 409 conflict
-- ("resource conflict") in the tenant-admin user form.
--
-- Fix: add tenant-scoped DELETE policies that mirror the existing INSERT
-- policies. UPDATE is also added defensively for future fields.
-- =============================================================================

-- ---- user_role ----
DROP POLICY IF EXISTS user_role_delete ON user_role;
CREATE POLICY user_role_delete ON user_role
    AS PERMISSIVE FOR DELETE
    USING (
        EXISTS (
             SELECT 1 FROM membership m
             WHERE m.id            = user_role.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

DROP POLICY IF EXISTS user_role_update ON user_role;
CREATE POLICY user_role_update ON user_role
    AS PERMISSIVE FOR UPDATE
    USING (
        EXISTS (
             SELECT 1 FROM membership m
             WHERE m.id            = user_role.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    )
    WITH CHECK (
        EXISTS (
             SELECT 1 FROM membership m
             WHERE m.id            = user_role.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

GRANT UPDATE ON user_role TO lustia_app;

-- ---- user_branch ----
DROP POLICY IF EXISTS user_branch_delete ON user_branch;
CREATE POLICY user_branch_delete ON user_branch
    AS PERMISSIVE FOR DELETE
    USING (
        EXISTS (
             SELECT 1 FROM membership m
             WHERE m.id            = user_branch.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

DROP POLICY IF EXISTS user_branch_update ON user_branch;
CREATE POLICY user_branch_update ON user_branch
    AS PERMISSIVE FOR UPDATE
    USING (
        EXISTS (
             SELECT 1 FROM membership m
             WHERE m.id            = user_branch.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    )
    WITH CHECK (
        EXISTS (
             SELECT 1 FROM membership m
             WHERE m.id            = user_branch.membership_id
               AND m.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

GRANT UPDATE ON user_branch TO lustia_app;
