-- =============================================================================
-- Migration: 000002_auth_tables (DOWN)
-- Reverses in strict LIFO order.
-- =============================================================================

-- 5. password_reset
DROP TABLE IF EXISTS password_reset;

-- 4. refresh_token
DROP TABLE IF EXISTS refresh_token;

-- 3. user_branch
DROP TABLE IF EXISTS user_branch;

-- 2. user_role
DROP TABLE IF EXISTS user_role;

-- 1. "user"
DROP TABLE IF EXISTS "user";
