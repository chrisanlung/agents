-- =============================================================================
-- Migration: 000006_user_must_change_password
-- Purpose  : Add the must_change_password flag on "user" to force a password
--            rotation after admin-created accounts or the bootstrap super-admin
--            seed. Addresses the Critical finding from security-expert raised
--            during the Phase-2 review (SECURITY.md review log).
--
-- Behavior : Default false for all existing and future users. The bootstrap
--            super-admin row (tenant_id IS NULL,
--            email = 'superadmin@lustia.internal') is flipped to true so any
--            operator who ran the seed MUST rotate before logging in.
-- =============================================================================

ALTER TABLE "user"
    ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN "user".must_change_password IS
    'When true, the user must change their password before the next protected request. Set on admin-created users and on seeded accounts.';

-- Force the bootstrap super admin to rotate on first login.
UPDATE "user"
   SET must_change_password = true
 WHERE tenant_id IS NULL
   AND email = 'superadmin@lustia.internal';
