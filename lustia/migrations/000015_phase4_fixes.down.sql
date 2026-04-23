-- =============================================================================
-- Migration: 000015_phase4_fixes (DOWN)
-- Reverse order of the UP migration.
-- =============================================================================

-- 3. Revoke UPDATE/DELETE on therapist_service + drop policies
REVOKE UPDATE, DELETE ON therapist_service FROM lustia_app;
DROP POLICY IF EXISTS therapist_service_tenant_delete ON therapist_service;
DROP POLICY IF EXISTS therapist_service_tenant_update ON therapist_service;

-- 2. service.code — restore NOT NULL (requires all existing rows to have a value)
UPDATE service SET code = 'svc-' || SUBSTRING(REPLACE(id::text, '-', ''), 1, 8)
 WHERE code IS NULL;
ALTER TABLE service
    ALTER COLUMN code SET NOT NULL;

-- 1. therapist contact columns — drop
ALTER TABLE therapist
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS phone;
