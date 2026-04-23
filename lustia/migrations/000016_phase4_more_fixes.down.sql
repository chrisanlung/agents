-- =============================================================================
-- Migration: 000016_phase4_more_fixes (DOWN)
-- =============================================================================

-- 2. Revert price back to numeric(12,2)
ALTER TABLE service
    ALTER COLUMN price TYPE NUMERIC(12, 2) USING (price::numeric(12, 2));

-- 1. Drop therapist.joined_at
ALTER TABLE therapist
    DROP COLUMN IF EXISTS joined_at;
