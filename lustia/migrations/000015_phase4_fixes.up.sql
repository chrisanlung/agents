-- =============================================================================
-- Migration: 000015_phase4_fixes (UP)
-- Purpose  : Phase 4 post-delivery fixes — security H-1 + bug P4 schema gaps.
--
-- 1. Add phone + email columns to therapist — the Go model and API contract
--    (§11.4) carry these fields but the original schema omitted them.
--    BUG-P4 discovered during QA integration run.
--
-- 2. Make service.code nullable — the Go service layer now generates a code
--    from the service name at create time, but the original column was
--    NOT NULL with no default, causing every POST /tenant/services to fail.
--    We keep the column (future features may care) and relax the NOT NULL.
--
-- 3. Add UPDATE + DELETE RLS policies on therapist_service plus the matching
--    GRANT UPDATE, DELETE — SECURITY.md Phase 4 H-1 fix. Without this, the
--    PUT /therapists/:id/services reconcile path cannot toggle is_active on
--    existing mapping rows (lustia_app had SELECT/INSERT only).
--
-- CLEAN: Migration chain 1 → 15 still produces a working system on any
-- environment. Migration 14 (dev seed) runs between 13 and 15 but is
-- idempotent and doesn't touch the columns we alter here.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. therapist contact columns
-- ---------------------------------------------------------------------------
ALTER TABLE therapist
    ADD COLUMN IF NOT EXISTS phone TEXT,
    ADD COLUMN IF NOT EXISTS email TEXT;

COMMENT ON COLUMN therapist.phone IS
    'Optional contact phone for the therapist. Added 2026-04-23 to match ADR 0009 §2.1 therapist profile fields.';
COMMENT ON COLUMN therapist.email IS
    'Optional contact email for the therapist. Distinct from the linked user.email (therapist.user_id → user).';

-- ---------------------------------------------------------------------------
-- 2. service.code — relax to NULL so the service layer can omit it or
--    generate a slug without blocking inserts.
-- ---------------------------------------------------------------------------
ALTER TABLE service
    ALTER COLUMN code DROP NOT NULL;

-- ---------------------------------------------------------------------------
-- 3. therapist_service RLS — UPDATE + DELETE policies and grants
--    (SECURITY.md Phase 4 H-1)
-- ---------------------------------------------------------------------------
-- The SELECT policy from migration 4 checks that the therapist row exists in
-- the caller's tenant. Apply the same idea to UPDATE + DELETE so lustia_app
-- can only mutate mappings for therapists in its current tenant context.

DROP POLICY IF EXISTS therapist_service_tenant_update ON therapist_service;
CREATE POLICY therapist_service_tenant_update ON therapist_service
    AS PERMISSIVE FOR UPDATE
    USING (
        EXISTS (
            SELECT 1 FROM therapist t
             WHERE t.id = therapist_service.therapist_id
               AND t.tenant_id::text = current_setting('app.current_tenant', true)
        )
    )
    WITH CHECK (
        EXISTS (
            SELECT 1 FROM therapist t
             WHERE t.id = therapist_service.therapist_id
               AND t.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

DROP POLICY IF EXISTS therapist_service_tenant_delete ON therapist_service;
CREATE POLICY therapist_service_tenant_delete ON therapist_service
    AS PERMISSIVE FOR DELETE
    USING (
        EXISTS (
            SELECT 1 FROM therapist t
             WHERE t.id = therapist_service.therapist_id
               AND t.tenant_id::text = current_setting('app.current_tenant', true)
        )
    );

GRANT UPDATE, DELETE ON therapist_service TO lustia_app;
