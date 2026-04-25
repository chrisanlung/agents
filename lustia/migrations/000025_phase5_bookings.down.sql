-- =============================================================================
-- Migration: 000025_phase5_bookings (DOWN)
-- Purpose  : Drop booking engine tables added in the UP migration.
--
-- Order matters: booking_addon FK references booking, so drop addon table first.
-- =============================================================================

DROP TABLE IF EXISTS booking_addon;
DROP TABLE IF EXISTS booking;
