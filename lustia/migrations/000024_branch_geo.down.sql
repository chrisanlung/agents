-- =============================================================================
-- Migration: 000024_branch_geo (DOWN)
-- Purpose  : Revert geolocation additions to branch.
--
-- Safe to re-run: DROP INDEX / DROP COLUMN use IF EXISTS.
--
-- NOTE: The cube and earthdistance extensions are NOT dropped here because
-- other tables or future migrations may depend on them. Extensions are
-- cluster-level objects; dropping them could break concurrent code paths.
-- If a full rollback of the extensions is required, do it manually via:
--   DROP EXTENSION IF EXISTS earthdistance;
--   DROP EXTENSION IF EXISTS cube;
-- =============================================================================

DROP INDEX IF EXISTS branch_geo_idx;

ALTER TABLE branch
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude;
