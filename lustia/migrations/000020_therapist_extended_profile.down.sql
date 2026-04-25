-- =============================================================================
-- Migration: 000020_therapist_extended_profile (DOWN)
-- Purpose  : Reverse migration 000020.
--
-- Reversal order (strict reverse of UP steps):
--   7. Drop therapist_build_check, drop build column.
--   6. Drop therapist_weight_kg_check, drop weight_kg column.
--   5. Drop therapist_height_cm_check, drop height_cm column.
--   4. Drop therapist_photo_key_length constraint.
--   3. (No-op) — nulled photo_url values cannot be restored; data is lost.
--   2. Rename photo_key → photo_url.
--   1. Re-add the original inline CHECK from migration 000003 under its
--      original auto-generated name (therapist_photo_url_check) and with
--      the original 2048-char bound.
--
-- Warning: existing photo_url values that were present before migration
-- 000020 are permanently lost (they were nulled in step 3 of the UP). This
-- is accepted per ADR 0011 §2.3.1 — only dev-seed rows were affected.
-- =============================================================================

-- Step 7 reverse: build
ALTER TABLE therapist
    DROP CONSTRAINT IF EXISTS therapist_build_check;

ALTER TABLE therapist
    DROP COLUMN IF EXISTS build;

-- Step 6 reverse: weight_kg
ALTER TABLE therapist
    DROP CONSTRAINT IF EXISTS therapist_weight_kg_check;

ALTER TABLE therapist
    DROP COLUMN IF EXISTS weight_kg;

-- Step 5 reverse: height_cm
ALTER TABLE therapist
    DROP CONSTRAINT IF EXISTS therapist_height_cm_check;

ALTER TABLE therapist
    DROP COLUMN IF EXISTS height_cm;

-- Step 4 reverse: drop the new length constraint on photo_key
ALTER TABLE therapist
    DROP CONSTRAINT IF EXISTS therapist_photo_key_length;

-- Step 2 reverse: rename photo_key back to photo_url
ALTER TABLE therapist
    RENAME COLUMN photo_key TO photo_url;

-- Step 1 reverse: reinstate the original CHECK from migration 000003
-- (anonymous inline form — Postgres names it therapist_photo_url_check)
ALTER TABLE therapist
    ADD CONSTRAINT therapist_photo_url_check
        CHECK (photo_url IS NULL OR char_length(photo_url) <= 2048);
