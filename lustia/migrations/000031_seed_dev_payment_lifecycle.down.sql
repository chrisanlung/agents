-- =============================================================================
-- Migration: 000031_seed_dev_payment_lifecycle (DOWN)
-- Reverses migration 000031: remove dev-seed payment/settlement/disbursement rows.
--
-- Order:
--   1. Delete payment_transaction rows (holds FKs to batch + disbursement).
--   2. Delete tenant_disbursement row.
--   3. Delete settlement_batch row.
-- =============================================================================

DELETE FROM payment_transaction
WHERE id IN (
    'e0000000-0000-0000-0031-000000000011',
    'e0000000-0000-0000-0031-000000000012',
    'e0000000-0000-0000-0031-000000000013',
    'e0000000-0000-0000-0031-000000000014'
);

DELETE FROM tenant_disbursement
WHERE id = 'e0000000-0000-0000-0031-000000000002';

DELETE FROM settlement_batch
WHERE id = 'e0000000-0000-0000-0031-000000000001';
