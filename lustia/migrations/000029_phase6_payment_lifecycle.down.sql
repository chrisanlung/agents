-- =============================================================================
-- Migration: 000029_phase6_payment_lifecycle (DOWN)
-- Reverses migration 000029 in strict dependency order.
--
-- Order:
--   1. Revoke grants on all three tables.
--   2. Drop triggers.
--   3. Drop RLS policies (preceded by DISABLE RLS).
--   4. Drop indexes.
--   5. Drop tables in FK-safe order:
--        payment_transaction first (holds FKs to settlement_batch + tenant_disbursement)
--        then settlement_batch and tenant_disbursement (no mutual FK).
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Revoke grants
-- ---------------------------------------------------------------------------

REVOKE SELECT, INSERT, UPDATE ON payment_transaction FROM lustia_app;
REVOKE SELECT, INSERT         ON settlement_batch     FROM lustia_app;
REVOKE SELECT, INSERT, UPDATE ON tenant_disbursement  FROM lustia_app;

-- ---------------------------------------------------------------------------
-- 2. Drop triggers
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_pt_updated_at ON payment_transaction;
DROP TRIGGER IF EXISTS trg_td_updated_at ON tenant_disbursement;

-- ---------------------------------------------------------------------------
-- 3. Disable RLS + drop policies
-- ---------------------------------------------------------------------------

ALTER TABLE IF EXISTS payment_transaction DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS pt_tenant_select  ON payment_transaction;
DROP POLICY IF EXISTS pt_public_select  ON payment_transaction;
DROP POLICY IF EXISTS pt_tenant_insert  ON payment_transaction;
DROP POLICY IF EXISTS pt_tenant_update  ON payment_transaction;

ALTER TABLE IF EXISTS settlement_batch DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS sb_platform_only ON settlement_batch;

ALTER TABLE IF EXISTS tenant_disbursement DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS td_tenant_select ON tenant_disbursement;
DROP POLICY IF EXISTS td_tenant_insert ON tenant_disbursement;
DROP POLICY IF EXISTS td_tenant_update ON tenant_disbursement;

-- ---------------------------------------------------------------------------
-- 4. Drop tables (payment_transaction first to release FKs to the other two)
-- ---------------------------------------------------------------------------

DROP TABLE IF EXISTS payment_transaction;
DROP TABLE IF EXISTS settlement_batch;
DROP TABLE IF EXISTS tenant_disbursement;
