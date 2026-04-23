-- =============================================================================
-- Migration: 000012_seed_dev_registrations (DOWN)
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
--
-- Removes the sample pending registration inserted by the UP migration.
-- =============================================================================

DELETE FROM tenant_registration
 WHERE id = 'e0000000-0000-0000-0001-000000000001';
