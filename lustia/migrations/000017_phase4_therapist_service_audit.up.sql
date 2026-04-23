-- =============================================================================
-- Migration: 000017_phase4_therapist_service_audit (UP)
-- Purpose  : Add updated_at + updated_by to therapist_service. The Go
--            reconcile flow (PUT /therapists/:id/services) tracks when a
--            mapping is soft-deactivated by stamping updated_at; the original
--            schema omitted these columns.
-- =============================================================================

ALTER TABLE therapist_service
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES "user"(id) ON DELETE SET NULL;

COMMENT ON COLUMN therapist_service.updated_at IS
    'Last time this mapping row was modified (is_active toggle). Phase 4 reconcile uses this for audit trail.';
