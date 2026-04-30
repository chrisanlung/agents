-- =============================================================================
-- Migration: 000031_seed_dev_payment_lifecycle (UP)
-- Purpose  : Dev-only seed — populate payment/settlement/disbursement rows
--            for the acme-spa tenant so the Keuangan UI has finance data
--            on a fresh dev environment.
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
--
-- Depends on:
--   migration 000010  — acme-spa tenant  (d0000000-0000-0000-0001-000000000001)
--   migration 000027  — dev booking seed (UUIDs b0000000-0000-0000-0027-0000000000{1..5})
--   migration 000029  — payment_transaction, settlement_batch, tenant_disbursement
--
-- In staging/prod stop the migration runner at step 28:
--   migrate -database "$DATABASE_URL" -path ./migrations up 28
-- Or omit this file from the production image.
--
-- Seed design:
--   settlement_batch  : one row  (yesterday's iPaymu settlement)
--   tenant_disbursement: one row (last week period, status=transferred)
--   payment_transaction: four rows (one per paid booking from migration 27)
--     Booking 1 (b..0001) : status=settled, linked to settlement_batch
--     Booking 2 (b..0002) : status=settled, linked to settlement_batch
--     Booking 3 (b..0003) : status=disbursed, linked to settlement_batch + disbursement
--     Booking 4 (b..0004) : status=paid     (settled not yet run)
--     (Booking 5 is checked_in, not paid — no payment_transaction row)
--
-- Fixed UUIDs (prefix e0000000-0000-0000-0031-* = finance seed):
--   settlement_batch  : e0000000-0000-0000-0031-000000000001
--   tenant_disbursement: e0000000-0000-0000-0031-000000000002
--   payment_transaction 1: e0000000-0000-0000-0031-000000000011
--   payment_transaction 2: e0000000-0000-0000-0031-000000000012
--   payment_transaction 3: e0000000-0000-0000-0031-000000000013
--   payment_transaction 4: e0000000-0000-0000-0031-000000000014
--
-- Platform fee: floor(received * 0.05) as per ADR 0015 §2.4.
--   Booking 1: received=120000, fee=6000,  net=114000
--   Booking 2: received=225000, fee=11250, net=213750
--   Booking 3: received=250000, fee=12500, net=237500  (disbursed)
--
-- IDEMPOTENCY:
--   Every INSERT uses ON CONFLICT (id) DO NOTHING.
-- =============================================================================

-- Migration runs as lustia_migrator (BYPASSRLS). SET LOCAL is consistent with
-- migrations 000014, 000019, 000022, 000027 pattern.
SET LOCAL app.current_tenant = '__platform__';

-- ---------------------------------------------------------------------------
-- 1. settlement_batch (platform-level; needs __platform__ sentinel)
-- ---------------------------------------------------------------------------

INSERT INTO settlement_batch (
    id,
    provider,
    settled_at,
    total_amount_idr,
    transaction_count,
    raw_payload,
    created_at,
    created_by
)
VALUES (
    'e0000000-0000-0000-0031-000000000001',
    'dummy',
    (now() - INTERVAL '1 day'),   -- yesterday's settlement batch
    595000,                        -- 120000 + 225000 + 250000
    3,
    '{"source": "dev-seed", "items": [{"ref": "dev-ref-0001"}, {"ref": "dev-ref-0002"}, {"ref": "dev-ref-0003"}]}'::jsonb,
    now() - INTERVAL '1 day',
    NULL                           -- no platform admin in dev seed
)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. tenant_disbursement (tenant-scoped; switch sentinel)
-- ---------------------------------------------------------------------------

SET LOCAL app.current_tenant = 'd0000000-0000-0000-0001-000000000001';

INSERT INTO tenant_disbursement (
    id,
    tenant_id,
    period_start,
    period_end,
    gross_amount_idr,
    platform_fee_idr,
    net_amount_idr,
    transaction_count,
    status,
    bank_reference,
    notes,
    transferred_at,
    transferred_by,
    created_at,
    updated_at
)
VALUES (
    'e0000000-0000-0000-0031-000000000002',
    'd0000000-0000-0000-0001-000000000001',
    (CURRENT_DATE - INTERVAL '7 days')::date,
    (CURRENT_DATE - INTERVAL '1 day')::date,
    250000,          -- booking 3 only (other bookings settled but not disbursed)
    12500,           -- floor(250000 * 0.05)
    237500,          -- 250000 - 12500
    1,
    'transferred',
    'DEV-BANK-TRF-001',
    'Dev seed disbursement for Booking 3 (Pijat 120m)',
    now() - INTERVAL '12 hours',
    NULL,            -- no platform admin user in dev seed
    now() - INTERVAL '1 day',
    now() - INTERVAL '12 hours'
)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 3. payment_transaction rows (tenant-scoped)
-- ---------------------------------------------------------------------------

INSERT INTO payment_transaction (
    id,
    tenant_id,
    booking_id,
    provider,
    provider_reference,
    qr_string,
    qr_image_url,
    qr_expires_at,
    expected_amount_idr,
    received_amount_idr,
    platform_fee_idr,
    tenant_net_idr,
    status,
    paid_at,
    settled_at,
    disbursed_at,
    settlement_batch_id,
    disbursement_id,
    raw_webhook,
    created_at,
    updated_at
)
VALUES
    -- Payment 1: Booking 1 (Refleksi Kaki 60m, 120000) → settled
    (
        'e0000000-0000-0000-0031-000000000011',
        'd0000000-0000-0000-0001-000000000001',
        'b0000000-0000-0000-0027-000000000001',
        'dummy',
        'dev-ref-0001',
        NULL,                   -- QR display data not needed post-payment
        NULL,
        now() - INTERVAL '2 hours 15 minutes',  -- already expired (past 15-min window)
        120000,
        120000,
        6000,                   -- floor(120000 * 0.05)
        114000,                 -- 120000 - 6000
        'settled',
        now() - INTERVAL '2 hours',
        now() - INTERVAL '1 day',
        NULL,
        'e0000000-0000-0000-0031-000000000001',
        NULL,
        '{"event": "payment_success", "trx_id": "dev-ref-0001", "amount": 120000}'::jsonb,
        now() - INTERVAL '2 hours 15 minutes',
        now() - INTERVAL '1 day'
    ),

    -- Payment 2: Booking 2 (Aromaterapi 90m + add-on, 225000) → settled
    (
        'e0000000-0000-0000-0031-000000000012',
        'd0000000-0000-0000-0001-000000000001',
        'b0000000-0000-0000-0027-000000000002',
        'dummy',
        'dev-ref-0002',
        NULL,
        NULL,
        now() - INTERVAL '1 hour 15 minutes',
        225000,
        225000,
        11250,                  -- floor(225000 * 0.05)
        213750,                 -- 225000 - 11250
        'settled',
        now() - INTERVAL '1 hour',
        now() - INTERVAL '1 day',
        NULL,
        'e0000000-0000-0000-0031-000000000001',
        NULL,
        '{"event": "payment_success", "trx_id": "dev-ref-0002", "amount": 225000}'::jsonb,
        now() - INTERVAL '1 hour 15 minutes',
        now() - INTERVAL '1 day'
    ),

    -- Payment 3: Booking 3 (Pijat 120m, 250000) → disbursed (linked to disbursement)
    (
        'e0000000-0000-0000-0031-000000000013',
        'd0000000-0000-0000-0001-000000000001',
        'b0000000-0000-0000-0027-000000000003',
        'dummy',
        'dev-ref-0003',
        NULL,
        NULL,
        now() - INTERVAL '45 minutes',
        250000,
        250000,
        12500,                  -- floor(250000 * 0.05)
        237500,                 -- 250000 - 12500
        'disbursed',
        now() - INTERVAL '30 minutes',
        now() - INTERVAL '1 day',
        now() - INTERVAL '12 hours',
        'e0000000-0000-0000-0031-000000000001',
        'e0000000-0000-0000-0031-000000000002',
        '{"event": "payment_success", "trx_id": "dev-ref-0003", "amount": 250000}'::jsonb,
        now() - INTERVAL '45 minutes',
        now() - INTERVAL '12 hours'
    ),

    -- Payment 4: Booking 4 (Refleksi Kaki 60m + add-on, 145000) → paid (not yet settled)
    (
        'e0000000-0000-0000-0031-000000000014',
        'd0000000-0000-0000-0001-000000000001',
        'b0000000-0000-0000-0027-000000000004',
        'dummy',
        'dev-ref-0004',
        NULL,
        NULL,
        now() - INTERVAL '15 minutes',
        145000,
        145000,
        NULL,                   -- not yet settled; platform_fee computed at settlement
        NULL,
        'paid',
        now() - INTERVAL '15 minutes',
        NULL,
        NULL,
        NULL,
        NULL,
        '{"event": "payment_success", "trx_id": "dev-ref-0004", "amount": 145000}'::jsonb,
        now() - INTERVAL '16 minutes',
        now() - INTERVAL '15 minutes'
    )
ON CONFLICT (id) DO NOTHING;
