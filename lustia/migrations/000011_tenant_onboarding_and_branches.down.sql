-- =============================================================================
-- Migration: 000011_tenant_onboarding_and_branches (DOWN)
-- Purpose  : Full reversal of migration 000011_tenant_onboarding_and_branches.
--
-- Reversal order (LIFO):
--   7. No-op: permission / role_permission guard-inserts are idempotent seeds;
--      reversing them here would delete rows that were also seeded by migration
--      000005. Down migration 000005 is responsible for cleaning those rows.
--      We therefore do NOT delete permission or role_permission rows here.
--   6. Revoke grants on tenant_registration.
--   5. Drop RLS policy on tenant_registration.
--   4. Drop tenant_registration table (cascades trigger + indexes).
--   3. Drop tenant_registration_status enum.
--   2. Drop added columns from branch.
--   1. Drop added columns from tenant.
--
-- NOTE: Only columns introduced by THIS migration are dropped. Columns that
-- existed before migration 000011 (i.e., from migrations 000001–000010) are
-- never touched. Specifically:
--   • branch.address_line1/2, city, province, postal_code, country_code,
--     contact_phone, contact_email were created in migration 000001 — they are
--     NOT dropped here. DROP COLUMN IF EXISTS guards protect against
--     double-application of the down migration.
--
-- Reference: docs/DECISIONS/0008-tenant-onboarding-and-branch-setup.md
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 6. Revoke grants on tenant_registration
-- ---------------------------------------------------------------------------

-- Revoke before dropping so the role's privilege cache is cleanly cleared.
REVOKE SELECT, INSERT, UPDATE ON tenant_registration FROM lustia_app;

-- ---------------------------------------------------------------------------
-- 5. Drop RLS policy on tenant_registration
--    (The table drop below would cascade, but explicit removal is cleaner and
--    matches the "every CREATE has a matching DROP in down" invariant.)
-- ---------------------------------------------------------------------------

DROP POLICY IF EXISTS tenant_registration_platform_only ON tenant_registration;

-- ---------------------------------------------------------------------------
-- 4. Drop tenant_registration table
--    (Indexes and trigger are dropped automatically by CASCADE.)
-- ---------------------------------------------------------------------------

DROP TABLE IF EXISTS tenant_registration;

-- ---------------------------------------------------------------------------
-- 3. Drop tenant_registration_status enum
-- ---------------------------------------------------------------------------

DROP TYPE IF EXISTS tenant_registration_status;

-- ---------------------------------------------------------------------------
-- 2. Drop columns added to branch by this migration
--    Only timezone and activated_at were newly added (address/contact columns
--    pre-existed from migration 000001 and must not be dropped here).
-- ---------------------------------------------------------------------------

ALTER TABLE branch
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS activated_at;

-- ---------------------------------------------------------------------------
-- 1. Drop columns added to tenant by this migration
-- ---------------------------------------------------------------------------

ALTER TABLE tenant
    DROP COLUMN IF EXISTS package,
    DROP COLUMN IF EXISTS max_branches,
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS rejected_at,
    DROP COLUMN IF EXISTS rejected_by,
    DROP COLUMN IF EXISTS rejection_reason;
