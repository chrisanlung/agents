-- =============================================================================
-- Migration: 000016_phase4_more_fixes (UP)
-- Purpose  : Second round of Phase 4 schema/type alignment fixes discovered by
--            the QA integration suite after migration 15 landed.
--
-- 1. therapist.joined_at — Go model has JoinedAt but original schema omitted
--    the column. Add it as nullable timestamptz.
--
-- 2. service.price — currently numeric(12,2); Go model uses int64 and the
--    GORM scan fails on fractional strings like "150000.00". IDR has no
--    fractional unit, so convert to BIGINT (whole Rupiah). Existing seeded
--    rows (migration 14) use whole numbers; the `::bigint` cast truncates
--    any accidental fraction safely.
-- =============================================================================

-- 1. therapist.joined_at
ALTER TABLE therapist
    ADD COLUMN IF NOT EXISTS joined_at TIMESTAMPTZ;

COMMENT ON COLUMN therapist.joined_at IS
    'Optional date the therapist joined the branch. API exposes as ISO-8601 (date-only YYYY-MM-DD is accepted on input).';

-- 2. service.price numeric(12,2) -> bigint
ALTER TABLE service
    ALTER COLUMN price TYPE BIGINT USING (price::bigint);

COMMENT ON COLUMN service.price IS
    'Price in whole Rupiah (IDR). Stored as BIGINT because IDR has no fractional unit.';
