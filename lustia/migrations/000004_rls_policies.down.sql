-- =============================================================================
-- Migration: 000004_rls_policies (DOWN)
-- Drops RLS policies and disables RLS. Reverses in LIFO order.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Revoke grants from lustia_app on platform tables
-- ---------------------------------------------------------------------------
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM lustia_app;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM lustia_migrator;
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM lustia_migrator;

-- ---------------------------------------------------------------------------
-- 13. payment
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON payment;
DROP POLICY IF EXISTS tenant_isolation_write  ON payment;
DROP POLICY IF EXISTS tenant_isolation_update ON payment;
ALTER TABLE payment DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 12. invoice
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON invoice;
DROP POLICY IF EXISTS tenant_isolation_write  ON invoice;
DROP POLICY IF EXISTS tenant_isolation_update ON invoice;
ALTER TABLE invoice DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 11. booking
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON booking;
DROP POLICY IF EXISTS tenant_isolation_write  ON booking;
DROP POLICY IF EXISTS tenant_isolation_update ON booking;
ALTER TABLE booking DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 10. therapist_availability
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON therapist_availability;
DROP POLICY IF EXISTS tenant_isolation_write  ON therapist_availability;
DROP POLICY IF EXISTS tenant_isolation_update ON therapist_availability;
ALTER TABLE therapist_availability DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 9. therapist_service
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON therapist_service;
DROP POLICY IF EXISTS tenant_isolation_write  ON therapist_service;
ALTER TABLE therapist_service DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 8. therapist
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON therapist;
DROP POLICY IF EXISTS tenant_isolation_write  ON therapist;
DROP POLICY IF EXISTS tenant_isolation_update ON therapist;
ALTER TABLE therapist DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 7. service
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON service;
DROP POLICY IF EXISTS tenant_isolation_write  ON service;
DROP POLICY IF EXISTS tenant_isolation_update ON service;
ALTER TABLE service DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 6. customer
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON customer;
DROP POLICY IF EXISTS tenant_isolation_write  ON customer;
DROP POLICY IF EXISTS tenant_isolation_update ON customer;
ALTER TABLE customer DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 5. refresh_token
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON refresh_token;
DROP POLICY IF EXISTS tenant_isolation_write  ON refresh_token;
DROP POLICY IF EXISTS tenant_isolation_update ON refresh_token;
ALTER TABLE refresh_token DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 4. user_branch
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON user_branch;
DROP POLICY IF EXISTS tenant_isolation_write  ON user_branch;
ALTER TABLE user_branch DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 3. user_role
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON user_role;
DROP POLICY IF EXISTS tenant_isolation_write  ON user_role;
ALTER TABLE user_role DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 2. "user"
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON "user";
DROP POLICY IF EXISTS tenant_isolation_write  ON "user";
DROP POLICY IF EXISTS tenant_isolation_update ON "user";
ALTER TABLE "user" DISABLE ROW LEVEL SECURITY;

-- ---------------------------------------------------------------------------
-- 1. branch
-- ---------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_isolation        ON branch;
DROP POLICY IF EXISTS tenant_isolation_write  ON branch;
DROP POLICY IF EXISTS tenant_isolation_update ON branch;
ALTER TABLE branch DISABLE ROW LEVEL SECURITY;
