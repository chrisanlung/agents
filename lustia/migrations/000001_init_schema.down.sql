-- =============================================================================
-- Migration: 000001_init_schema (DOWN)
-- Reverses in strict LIFO order.
-- =============================================================================

-- 8. role_permission
DROP TABLE IF EXISTS role_permission;

-- 7. permission
DROP TABLE IF EXISTS permission;

-- 6. role
DROP TABLE IF EXISTS role;

-- 5. branch
DROP TABLE IF EXISTS branch;

-- 4. tenant
DROP TABLE IF EXISTS tenant;

-- 3. Shared trigger function
DROP FUNCTION IF EXISTS set_updated_at();

-- 2. Enum types (drop in reverse order)
DROP TYPE IF EXISTS payment_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS invoice_status;
DROP TYPE IF EXISTS booking_source;
DROP TYPE IF EXISTS booking_status;
DROP TYPE IF EXISTS branch_status;
DROP TYPE IF EXISTS tenant_status;

-- 1. Extensions
-- NOTE: Only drop extensions if no other objects depend on them.
-- In a shared DB, dropping citext/pgcrypto could break other users.
-- Intentionally omitted from auto-reverse; DBA should confirm before running.
-- DROP EXTENSION IF EXISTS btree_gist;
-- DROP EXTENSION IF EXISTS citext;
-- DROP EXTENSION IF EXISTS pgcrypto;

-- 0. DB roles
-- NOTE: Dropping roles will fail if they own objects or have active connections.
-- DBA must revoke all grants first. Commented out for safety.
-- DROP ROLE IF EXISTS lustia_migrator;
-- DROP ROLE IF EXISTS lustia_app;
