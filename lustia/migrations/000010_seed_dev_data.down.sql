-- =============================================================================
-- Migration: 000010_seed_dev_data (DOWN)
-- Purpose  : Remove the dev-only acme-spa tenant and alice user.
--
-- ⚠  DEV-ONLY — This file should not exist in staging/prod images ⚠
--
-- Deletion order respects FK constraints:
--   user_role → membership → user → tenant
-- Cascade is not relied upon here; explicit deletes document intent and
-- are robust even if ON DELETE CASCADE were changed in the future.
-- =============================================================================

-- 1. Remove alice's tenant_admin role assignment.
DELETE FROM user_role
 WHERE membership_id = 'd0000000-0000-0000-0003-000000000001'
   AND role_id       = 'b0000000-0000-0000-0000-000000000002';

-- 2. Remove alice's membership to acme-spa.
DELETE FROM membership
 WHERE id = 'd0000000-0000-0000-0003-000000000001';

-- 3. Remove alice's user record.
DELETE FROM "user"
 WHERE id = 'd0000000-0000-0000-0002-000000000001';

-- 4. Remove acme-spa tenant.
DELETE FROM tenant
 WHERE id = 'd0000000-0000-0000-0001-000000000001';
