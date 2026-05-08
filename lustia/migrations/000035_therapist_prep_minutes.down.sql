-- =============================================================================
-- Migration: 000035_therapist_prep_minutes (DOWN)
-- Purpose  : Reverse the prep_minutes column addition.
--            Drop the CHECK constraint before dropping the column (Postgres
--            requires the constraint to be removed independently when using
--            an explicit constraint name; DROP COLUMN would cascade it, but
--            being explicit keeps the rollback intention clear).
-- =============================================================================

ALTER TABLE therapist
    DROP CONSTRAINT IF EXISTS therapist_prep_minutes_check;

ALTER TABLE therapist
    DROP COLUMN IF EXISTS prep_minutes;
