-- =============================================================================
-- Migration: 000003_operational_tables (DOWN)
-- Reverses in strict LIFO order.
-- =============================================================================

-- 9. audit_log
DROP TABLE IF EXISTS audit_log;

-- 8. payment
DROP TABLE IF EXISTS payment;

-- 7. invoice
DROP TABLE IF EXISTS invoice;

-- 6. booking
DROP TABLE IF EXISTS booking;

-- 5. therapist_availability
DROP TABLE IF EXISTS therapist_availability;

-- 4. therapist_service
DROP TABLE IF EXISTS therapist_service;

-- 3. therapist
DROP TABLE IF EXISTS therapist;

-- 2. service
DROP TABLE IF EXISTS service;

-- 1. customer
DROP TABLE IF EXISTS customer;
