-- =============================================================================
-- Migration 000036 — Add DELETE policy on therapist_availability.
--
-- Bug fix: therapist_availability has SELECT / INSERT / UPDATE policies but no
-- DELETE policy. With FORCE ROW LEVEL SECURITY enabled, DELETE silently
-- affects zero rows. ReplaceAllForTherapist (the "save jadwal" flow) does
-- DELETE then INSERT in a transaction; DELETE silently no-ops, INSERT then
-- collides with the existing rows on the GiST exclusion constraint
-- (therapist_id, branch_id, day_of_week + time range), surfacing as a 409
-- "resource conflict" in the tenant-admin therapist availability editor.
--
-- Same class as migration 000034 (user_role / user_branch).
-- =============================================================================

DROP POLICY IF EXISTS therapist_availability_delete ON therapist_availability;
CREATE POLICY therapist_availability_delete ON therapist_availability
    AS PERMISSIVE FOR DELETE
    USING (tenant_id::text = current_setting('app.current_tenant', true));

GRANT DELETE ON therapist_availability TO lustia_app;
