-- =============================================================================
-- Migration: 000014_seed_dev_master_data (DOWN)
-- Purpose  : Reversal of migration 000014 dev seed — removes all rows inserted
--            by the Phase 4 master data seed, in LIFO dependency order.
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
-- =============================================================================

-- 5. Availability windows
DELETE FROM therapist_availability
WHERE id IN (
    'f0000000-0000-0000-0004-000000000001',
    'f0000000-0000-0000-0004-000000000002',
    'f0000000-0000-0000-0004-000000000003',
    'f0000000-0000-0000-0004-000000000004',
    'f0000000-0000-0000-0004-000000000005',
    'f0000000-0000-0000-0004-000000000006'
);

-- 4. Therapist ↔ service mappings
DELETE FROM therapist_service
WHERE therapist_id IN (
    'f0000000-0000-0000-0003-000000000001',
    'f0000000-0000-0000-0003-000000000002'
);

-- 3. Therapists
DELETE FROM therapist
WHERE id IN (
    'f0000000-0000-0000-0003-000000000001',
    'f0000000-0000-0000-0003-000000000002'
);

-- 2. Services
DELETE FROM service
WHERE id IN (
    'f0000000-0000-0000-0002-000000000001',
    'f0000000-0000-0000-0002-000000000002',
    'f0000000-0000-0000-0002-000000000003'
);

-- 1. Branch
DELETE FROM branch
WHERE id = 'f0000000-0000-0000-0001-000000000001';
