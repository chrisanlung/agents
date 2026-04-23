-- =============================================================================
-- Migration: 000017_phase4_therapist_service_audit (DOWN)
-- =============================================================================

ALTER TABLE therapist_service
    DROP COLUMN IF EXISTS updated_by,
    DROP COLUMN IF EXISTS updated_at;
