-- =============================================================================
-- Migration: 000027_seed_dev_bookings (UP)
-- Purpose  : Insert sample paid bookings for the acme-spa dev tenant.
--            Provides ready-to-use data for E2E testing the booking flow,
--            QR scan demo, check-in flow, and report views.
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
--
-- Depends on:
--   migration 000010  — acme-spa tenant  (d0000000-0000-0000-0001-000000000001)
--   migration 000014  — Cabang Utama     (f0000000-0000-0000-0001-000000000001)
--                     — Budi Santoso     (f0000000-0000-0000-0003-000000000001)
--                     — Siti Rahayu     (f0000000-0000-0000-0003-000000000002)
--                     — Refleksi Kaki 60m (f0000000-0000-0000-0002-000000000001)
--                     — Aromaterapi 90m  (f0000000-0000-0000-0002-000000000002)
--                     — Pijat 120m      (f0000000-0000-0000-0002-000000000003)
--   migration 000019  — add-ons (Aromaterapi Premium f..0005-001,
--                                Handuk Panas f..0005-002)
--   migration 000022  — rooms: VIP 1   (e0000000-0000-0000-0021-000000000001)
--                              Couple A (e0000000-0000-0000-0021-000000000002)
--                              Single 1 (e0000000-0000-0000-0021-000000000003)
--   migration 000025  — booking + booking_addon tables
--
-- In staging/prod stop the migration runner at step 26:
--
--   migrate -database "$DATABASE_URL" -path ./migrations up 26
--
-- Or omit this file from the production image.
--
-- IDEMPOTENCY:
--   Every INSERT uses ON CONFLICT (id) DO NOTHING.
--   Re-running is safe and produces no error.
--
-- SLOT DESIGN:
--   Slots are relative to NOW() so the dev seed never goes stale.
--   All times are in Asia/Jakarta (UTC+7); stored as TIMESTAMPTZ (UTC internally).
--   Five bookings across two days:
--
--   Booking 1: Budi · Refleksi Kaki 60m · VIP 1  · today 10:00–11:00 · paid
--   Booking 2: Siti · Aromaterapi 90m   · Couple A · today 14:00–15:30 · paid (+ Aromaterapi Premium add-on)
--   Booking 3: Budi · Pijat 120m        · Single 1 · today 16:00–18:00 · paid
--   Booking 4: Siti · Refleksi Kaki 60m · VIP 1    · tomorrow 09:00–10:00 · paid (+ Handuk Panas add-on)
--   Booking 5: Budi · Aromaterapi 90m   · Couple A · tomorrow 11:00–12:30 · checked_in
--
-- FIXED UUIDs (prefix b0000000-0000-0000-0027-* = bookings seed):
--   Booking 1 : b0000000-0000-0000-0027-000000000001
--   Booking 2 : b0000000-0000-0000-0027-000000000002
--   Booking 3 : b0000000-0000-0000-0027-000000000003
--   Booking 4 : b0000000-0000-0000-0027-000000000004
--   Booking 5 : b0000000-0000-0000-0027-000000000005
-- =============================================================================

-- Migration runs as lustia_migrator (BYPASSRLS). SET LOCAL is included as
-- best-practice guard consistent with migrations 000014, 000019, 000022.
SET LOCAL app.current_tenant = 'd0000000-0000-0000-0001-000000000001';

-- ---------------------------------------------------------------------------
-- 1. booking rows
-- ---------------------------------------------------------------------------

INSERT INTO booking (
    id,
    tenant_id,
    branch_id,
    service_id,
    room_id,
    therapist_id,
    customer_name,
    customer_phone,
    customer_email,
    code,
    scheduled_start,
    scheduled_end,
    status,
    total_price_idr,
    payment_method,
    payment_reference,
    paid_at,
    created_at,
    updated_at
)
VALUES
    -- Booking 1: Budi · Refleksi Kaki 60m · VIP 1 · today 10:00–11:00 · paid
    (
        'b0000000-0000-0000-0027-000000000001',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0002-000000000001',  -- Refleksi Kaki 60m
        'e0000000-0000-0000-0021-000000000001',  -- VIP 1
        'f0000000-0000-0000-0003-000000000001',  -- Budi Santoso
        'Rina Kusuma',
        '+62811234001',
        'rina.kusuma@example.com',
        'B7K3-M2QF',
        (CURRENT_DATE + INTERVAL '10 hours' - INTERVAL '7 hours'),  -- 10:00 WIB → UTC
        (CURRENT_DATE + INTERVAL '11 hours' - INTERVAL '7 hours'),  -- 11:00 WIB → UTC
        'paid',
        120000,
        'midtrans',
        'dev-snap-token-0001',
        now() - INTERVAL '2 hours',
        now() - INTERVAL '2 hours',
        now() - INTERVAL '2 hours'
    ),

    -- Booking 2: Siti · Aromaterapi 90m · Couple A · today 14:00–15:30 · paid
    (
        'b0000000-0000-0000-0027-000000000002',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0002-000000000002',  -- Aromaterapi 90m
        'e0000000-0000-0000-0021-000000000002',  -- Couple A
        'f0000000-0000-0000-0003-000000000002',  -- Siti Rahayu
        'Dodi Pratama',
        '+62812345002',
        'dodi.pratama@example.com',
        'C4F2-H6TN',
        (CURRENT_DATE + INTERVAL '14 hours' - INTERVAL '7 hours'),
        (CURRENT_DATE + INTERVAL '15 hours 30 minutes' - INTERVAL '7 hours'),
        'paid',
        225000,  -- 190000 (Aromaterapi) + 35000 (add-on Aromaterapi Premium)
        'midtrans',
        'dev-snap-token-0002',
        now() - INTERVAL '1 hour',
        now() - INTERVAL '1 hour',
        now() - INTERVAL '1 hour'
    ),

    -- Booking 3: Budi · Pijat Tradisional 120m · Single 1 · today 16:00–18:00 · paid
    (
        'b0000000-0000-0000-0027-000000000003',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0002-000000000003',  -- Pijat Tradisional 120m
        'e0000000-0000-0000-0021-000000000003',  -- Single 1
        'f0000000-0000-0000-0003-000000000001',  -- Budi Santoso
        'Astrid Wahyuni',
        '+62813456003',
        'astrid.wahyuni@example.com',
        'D2R7-P5XK',
        (CURRENT_DATE + INTERVAL '16 hours' - INTERVAL '7 hours'),
        (CURRENT_DATE + INTERVAL '18 hours' - INTERVAL '7 hours'),
        'paid',
        250000,
        'paid_at_venue',
        NULL,
        now() - INTERVAL '30 minutes',
        now() - INTERVAL '30 minutes',
        now() - INTERVAL '30 minutes'
    ),

    -- Booking 4: Siti · Refleksi Kaki 60m · VIP 1 · tomorrow 09:00–10:00 · paid
    (
        'b0000000-0000-0000-0027-000000000004',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0002-000000000001',  -- Refleksi Kaki 60m
        'e0000000-0000-0000-0021-000000000001',  -- VIP 1
        'f0000000-0000-0000-0003-000000000002',  -- Siti Rahayu
        'Fajar Nugroho',
        '+62814567004',
        'fajar.nugroho@example.com',
        'E2W4-Q3LM',
        (CURRENT_DATE + INTERVAL '1 day 9 hours' - INTERVAL '7 hours'),
        (CURRENT_DATE + INTERVAL '1 day 10 hours' - INTERVAL '7 hours'),
        'paid',
        145000,  -- 120000 (Refleksi Kaki) + 25000 (add-on Handuk Panas)
        'midtrans',
        'dev-snap-token-0004',
        now() - INTERVAL '15 minutes',
        now() - INTERVAL '15 minutes',
        now() - INTERVAL '15 minutes'
    ),

    -- Booking 5: Budi · Aromaterapi 90m · Couple A · tomorrow 11:00–12:30 · checked_in
    (
        'b0000000-0000-0000-0027-000000000005',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0002-000000000002',  -- Aromaterapi 90m
        'e0000000-0000-0000-0021-000000000002',  -- Couple A
        'f0000000-0000-0000-0003-000000000001',  -- Budi Santoso
        'Hesti Lestari',
        '+62815678005',
        'hesti.lestari@example.com',
        'F6N2-S3VB',
        (CURRENT_DATE + INTERVAL '1 day 11 hours' - INTERVAL '7 hours'),
        (CURRENT_DATE + INTERVAL '1 day 12 hours 30 minutes' - INTERVAL '7 hours'),
        'checked_in',
        190000,
        'midtrans',
        'dev-snap-token-0005',
        now() - INTERVAL '5 minutes',
        now() - INTERVAL '5 minutes',
        now() - INTERVAL '5 minutes'
    )
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. booking_addon rows
--
-- Booking 2 + Aromaterapi Premium add-on (35,000 IDR)
-- Booking 4 + Handuk Panas add-on       (25,000 IDR)
-- ---------------------------------------------------------------------------

INSERT INTO booking_addon (booking_id, addon_id, price_idr)
VALUES
    (
        'b0000000-0000-0000-0027-000000000002',
        'f0000000-0000-0000-0005-000000000001',  -- Aromaterapi Premium
        35000
    ),
    (
        'b0000000-0000-0000-0027-000000000004',
        'f0000000-0000-0000-0005-000000000002',  -- Handuk Panas
        25000
    )
ON CONFLICT DO NOTHING;
