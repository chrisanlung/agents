-- =============================================================================
-- Migration 000036 — DOWN.
-- =============================================================================

DROP POLICY IF EXISTS therapist_availability_delete ON therapist_availability;
REVOKE DELETE ON therapist_availability FROM lustia_app;
