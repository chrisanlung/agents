-- =============================================================================
-- Migration 000034 — DOWN.
-- Reverts the DELETE / UPDATE policies added in the up migration.
-- =============================================================================

DROP POLICY IF EXISTS user_role_delete   ON user_role;
DROP POLICY IF EXISTS user_role_update   ON user_role;
DROP POLICY IF EXISTS user_branch_delete ON user_branch;
DROP POLICY IF EXISTS user_branch_update ON user_branch;

REVOKE UPDATE ON user_role   FROM lustia_app;
REVOKE UPDATE ON user_branch FROM lustia_app;
