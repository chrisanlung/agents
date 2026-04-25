-- =============================================================================
-- Migration: 000028_public_rls_policies (DOWN)
-- =============================================================================

DROP POLICY IF EXISTS therapist_availability_public_select ON therapist_availability;
DROP POLICY IF EXISTS therapist_service_public_select ON therapist_service;
DROP POLICY IF EXISTS addon_public_select ON addon;
DROP POLICY IF EXISTS room_public_select ON room;
DROP POLICY IF EXISTS therapist_public_select ON therapist;
DROP POLICY IF EXISTS service_public_select ON service;
DROP POLICY IF EXISTS branch_public_select ON branch;
DROP POLICY IF EXISTS tenant_public_select ON tenant;
