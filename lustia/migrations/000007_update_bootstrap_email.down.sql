-- =============================================================================
-- Reverse 000007_update_bootstrap_email.
-- =============================================================================

UPDATE "user"
   SET email = 'superadmin@lustia.internal'
 WHERE id = 'a0000000-0000-0000-0000-000000000001'
   AND email = 'admin@lustia.local';
