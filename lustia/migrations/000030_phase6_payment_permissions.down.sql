-- =============================================================================
-- Migration: 000030_phase6_payment_permissions (DOWN)
-- Reverses migration 000030: remove finance/disbursement/settlement permissions
-- and their role_permission wires.
--
-- Order:
--   1. Delete role_permission rows that reference the five permissions.
--   2. Delete the five permission rows.
-- =============================================================================

-- 1. Remove role wires
DELETE FROM role_permission
WHERE permission_id IN (
    SELECT id FROM permission
    WHERE code IN (
        'finance.read',
        'finance.read_all',
        'disbursement.create',
        'disbursement.transfer',
        'settlement.reconcile'
    )
);

-- 2. Remove permission rows
DELETE FROM permission
WHERE code IN (
    'finance.read',
    'finance.read_all',
    'disbursement.create',
    'disbursement.transfer',
    'settlement.reconcile'
);
