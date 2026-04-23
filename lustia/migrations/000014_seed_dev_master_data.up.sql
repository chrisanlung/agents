-- =============================================================================
-- Migration: 000014_seed_dev_master_data (UP)
--
-- !! DEV-ONLY — DO NOT APPLY IN STAGING OR PRODUCTION !!
--
-- Inserts sample Phase 4 master data for the acme-spa tenant (seeded by
-- migration 000010). Provides a ready-to-use dataset for developing and
-- testing the Master Data UI (therapist list, service catalog, availability
-- editor) without hitting any real API endpoints.
--
-- In staging/prod, stop the migration runner at step 13:
--
--   migrate -database "$DATABASE_URL" -path ./migrations up 13
--
-- Or simply omit this file from the production image. The main chain (1–13)
-- produces a working system on any environment; only this seed is dev-only.
--
-- IDEMPOTENCY:
--   Every INSERT uses ON CONFLICT (id) DO NOTHING or ON CONFLICT DO NOTHING.
--   Re-running is safe and produces no error. Fixed UUIDs ensure the same
--   rows on every fresh DB.
--
-- FIXED UUIDs (prefix f0000000- = Phase 4 / "four"):
--
--   Branch "Cabang Utama"     : f0000000-0000-0000-0001-000000000001
--
--   Services (tenant-scoped, branch_id NULL):
--     Refleksi Kaki 60m       : f0000000-0000-0000-0002-000000000001
--     Aromaterapi 90m         : f0000000-0000-0000-0002-000000000002
--     Pijat Tradisional 120m  : f0000000-0000-0000-0002-000000000003
--
--   Therapists (branch-scoped):
--     Budi Santoso            : f0000000-0000-0000-0003-000000000001
--     Siti Rahayu             : f0000000-0000-0000-0003-000000000002
--
--   therapist_service mappings:
--     Budi → Refleksi Kaki    : (f..0003-001, f..0002-001)
--     Budi → Pijat Tradisional: (f..0003-001, f..0002-003)
--     Siti → Refleksi Kaki    : (f..0003-002, f..0002-001)
--     Siti → Aromaterapi      : (f..0003-002, f..0002-002)
--     Siti → Pijat Tradisional: (f..0003-002, f..0002-003)
--
--   therapist_availability (Budi):
--     Monday 09:00–17:00      : f0000000-0000-0000-0004-000000000001
--     Wednesday 09:00–17:00   : f0000000-0000-0000-0004-000000000002
--     Friday 09:00–17:00      : f0000000-0000-0000-0004-000000000003
--
--   therapist_availability (Siti):
--     Tuesday 08:00–16:00     : f0000000-0000-0000-0004-000000000004
--     Thursday 08:00–16:00    : f0000000-0000-0000-0004-000000000005
--     Saturday 09:00–15:00    : f0000000-0000-0000-0004-000000000006
--
-- DEPENDS ON:
--   Migration 000010 (acme-spa tenant, alice membership)
--   Migration 000013 (therapist.branch_id, service.category, therapist_service.is_active)
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. Branch: Cabang Utama (acme-spa's primary branch)
--
--    Migration 000010 created only the tenant + user + membership.
--    A branch is required before therapists can be assigned (branch_id NOT NULL
--    on therapist since migration 000013). We create one here.
-- ---------------------------------------------------------------------------
INSERT INTO branch (
    id,
    tenant_id,
    name,
    code,
    status,
    city,
    province,
    country_code,
    timezone,
    operational_hours,
    metadata,
    created_at,
    updated_at
)
VALUES (
    'f0000000-0000-0000-0001-000000000001',
    'd0000000-0000-0000-0001-000000000001',  -- acme-spa
    'Cabang Utama',
    'MAIN',
    'active',
    'Jakarta Selatan',
    'DKI Jakarta',
    'ID',
    'Asia/Jakarta',
    -- Monday–Saturday 09:00–21:00, closed Sunday
    '[
        {"day": 1, "open": "09:00", "close": "21:00"},
        {"day": 2, "open": "09:00", "close": "21:00"},
        {"day": 3, "open": "09:00", "close": "21:00"},
        {"day": 4, "open": "09:00", "close": "21:00"},
        {"day": 5, "open": "09:00", "close": "21:00"},
        {"day": 6, "open": "09:00", "close": "21:00"}
    ]'::jsonb,
    '{"source": "dev_seed"}'::jsonb,
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. Services (tenant-scoped — branch_id IS NULL)
--
--    Three realistic Indonesian-market wellness offerings. Prices in IDR.
--    duration_minutes drives scheduled_end computation in Phase 5.
-- ---------------------------------------------------------------------------
INSERT INTO service (
    id,
    tenant_id,
    branch_id,
    code,
    name,
    description,
    category,
    duration_minutes,
    price,
    currency,
    is_active,
    metadata,
    created_at,
    updated_at
)
VALUES
    (
        'f0000000-0000-0000-0002-000000000001',
        'd0000000-0000-0000-0001-000000000001',  -- acme-spa
        NULL,                                    -- tenant-wide
        'refleksi-kaki-60',
        'Refleksi Kaki 60 Menit',
        'Terapi refleksi telapak kaki selama 60 menit untuk melancarkan '
        'peredaran darah dan meredakan kelelahan.',
        'Refleksi',
        60,
        150000.00,
        'IDR',
        true,
        '{"source": "dev_seed"}'::jsonb,
        now(), now()
    ),
    (
        'f0000000-0000-0000-0002-000000000002',
        'd0000000-0000-0000-0001-000000000001',
        NULL,
        'aromaterapi-90',
        'Aromaterapi 90 Menit',
        'Pijat relaksasi menggunakan minyak esensial pilihan selama 90 menit. '
        'Membantu mengurangi stres dan meningkatkan kualitas tidur.',
        'Aromaterapi',
        90,
        250000.00,
        'IDR',
        true,
        '{"source": "dev_seed"}'::jsonb,
        now(), now()
    ),
    (
        'f0000000-0000-0000-0002-000000000003',
        'd0000000-0000-0000-0001-000000000001',
        NULL,
        'pijat-tradisional-120',
        'Pijat Tradisional 120 Menit',
        'Pijat seluruh tubuh dengan teknik tradisional Jawa selama 120 menit. '
        'Ideal untuk pemulihan total setelah aktivitas berat.',
        'Pijat',
        120,
        350000.00,
        'IDR',
        true,
        '{"source": "dev_seed"}'::jsonb,
        now(), now()
    )
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 3. Therapists (branch-scoped — branch_id references Cabang Utama)
-- ---------------------------------------------------------------------------
INSERT INTO therapist (
    id,
    tenant_id,
    branch_id,
    user_id,
    full_name,
    gender,
    bio,
    specialties,
    is_active,
    metadata,
    created_at,
    updated_at
)
VALUES
    (
        'f0000000-0000-0000-0003-000000000001',
        'd0000000-0000-0000-0001-000000000001',  -- acme-spa
        'f0000000-0000-0000-0001-000000000001',  -- Cabang Utama
        NULL,                                    -- no portal login
        'Budi Santoso',
        'male',
        'Terapis berpengalaman 5 tahun, spesialis refleksi dan pijat tradisional.',
        '["refleksi", "pijat_tradisional"]'::jsonb,
        true,
        '{"source": "dev_seed"}'::jsonb,
        now(), now()
    ),
    (
        'f0000000-0000-0000-0003-000000000002',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        NULL,
        'Siti Rahayu',
        'female',
        'Terapis senior 8 tahun, ahli aromaterapi dan semua layanan pijat.',
        '["aromaterapi", "refleksi", "pijat_tradisional"]'::jsonb,
        true,
        '{"source": "dev_seed"}'::jsonb,
        now(), now()
    )
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 4. Therapist ↔ Service mappings
--
--    Budi: Refleksi Kaki + Pijat Tradisional (not certified for Aromaterapi)
--    Siti: all three services
-- ---------------------------------------------------------------------------
INSERT INTO therapist_service (
    therapist_id,
    service_id,
    is_active,
    created_at
)
VALUES
    -- Budi
    (
        'f0000000-0000-0000-0003-000000000001',
        'f0000000-0000-0000-0002-000000000001',  -- Refleksi Kaki
        true, now()
    ),
    (
        'f0000000-0000-0000-0003-000000000001',
        'f0000000-0000-0000-0002-000000000003',  -- Pijat Tradisional
        true, now()
    ),
    -- Siti
    (
        'f0000000-0000-0000-0003-000000000002',
        'f0000000-0000-0000-0002-000000000001',  -- Refleksi Kaki
        true, now()
    ),
    (
        'f0000000-0000-0000-0003-000000000002',
        'f0000000-0000-0000-0002-000000000002',  -- Aromaterapi
        true, now()
    ),
    (
        'f0000000-0000-0000-0003-000000000002',
        'f0000000-0000-0000-0002-000000000003',  -- Pijat Tradisional
        true, now()
    )
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------
-- 5. Weekly availability
--
--    Budi: Mon / Wed / Fri  09:00–17:00
--    Siti: Tue / Thu        08:00–16:00; Sat 09:00–15:00
--
--    day_of_week follows ISO convention adopted in this project: 0=Sunday,
--    1=Monday, …, 6=Saturday.
--
--    effective_from = CURRENT_DATE (resolves to migration run date on a fresh
--    DB). effective_until NULL = indefinite recurring window.
-- ---------------------------------------------------------------------------
INSERT INTO therapist_availability (
    id,
    tenant_id,
    therapist_id,
    branch_id,
    day_of_week,
    start_time,
    end_time,
    effective_from,
    effective_until,
    created_at,
    updated_at
)
VALUES
    -- Budi — Monday
    (
        'f0000000-0000-0000-0004-000000000001',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0003-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        1, '09:00', '17:00', CURRENT_DATE, NULL, now(), now()
    ),
    -- Budi — Wednesday
    (
        'f0000000-0000-0000-0004-000000000002',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0003-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        3, '09:00', '17:00', CURRENT_DATE, NULL, now(), now()
    ),
    -- Budi — Friday
    (
        'f0000000-0000-0000-0004-000000000003',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0003-000000000001',
        'f0000000-0000-0000-0001-000000000001',
        5, '09:00', '17:00', CURRENT_DATE, NULL, now(), now()
    ),
    -- Siti — Tuesday
    (
        'f0000000-0000-0000-0004-000000000004',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0003-000000000002',
        'f0000000-0000-0000-0001-000000000001',
        2, '08:00', '16:00', CURRENT_DATE, NULL, now(), now()
    ),
    -- Siti — Thursday
    (
        'f0000000-0000-0000-0004-000000000005',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0003-000000000002',
        'f0000000-0000-0000-0001-000000000001',
        4, '08:00', '16:00', CURRENT_DATE, NULL, now(), now()
    ),
    -- Siti — Saturday
    (
        'f0000000-0000-0000-0004-000000000006',
        'd0000000-0000-0000-0001-000000000001',
        'f0000000-0000-0000-0003-000000000002',
        'f0000000-0000-0000-0001-000000000001',
        6, '09:00', '15:00', CURRENT_DATE, NULL, now(), now()
    )
ON CONFLICT (id) DO NOTHING;
