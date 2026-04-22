-- =============================================================================
-- Reverse 000006_user_must_change_password.
-- =============================================================================

ALTER TABLE "user"
    DROP COLUMN IF EXISTS must_change_password;
