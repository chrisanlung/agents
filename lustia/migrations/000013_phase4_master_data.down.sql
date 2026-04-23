-- =============================================================================
-- Migration: 000013_phase4_master_data (DOWN)
-- Purpose  : Full reversal of migration 000013_phase4_master_data.
--
-- Reversal order (LIFO):
--   5. No-op: permission / role_permission guard-inserts are idempotent seeds
--      that were first inserted by migration 000005. Down migration 000005 owns
--      their removal. We do NOT delete them here.
--   4. No-op: therapist_availability had no structural change in this migration.
--   3. therapist_service — drop is_active column (and its index).
--   2. service — drop category column (and its index).
--   1. therapist — drop branch_id column, constraint, and indexes.
--
-- NOTE: Only columns introduced by THIS migration are dropped. Pre-existing
-- columns from migration 000003 are never touched.
--
-- Reference: docs/DECISIONS/0009-phase-4-master-operational-data.md
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 3. therapist_service — drop is_active
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS therapist_service_active_idx;

ALTER TABLE therapist_service
    DROP COLUMN IF EXISTS is_active;

-- ---------------------------------------------------------------------------
-- 2. service — drop category
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS service_tenant_category_active_idx;

ALTER TABLE service
    DROP COLUMN IF EXISTS category;

-- ---------------------------------------------------------------------------
-- 1. therapist — drop branch_id, constraint, and indexes
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS therapist_tenant_branch_active_idx;
DROP INDEX IF EXISTS therapist_branch_id_idx;

ALTER TABLE therapist
    DROP COLUMN IF EXISTS branch_id;
