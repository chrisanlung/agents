-- Reverse 000037_payment_va_support.
DROP INDEX IF EXISTS pt_channel_status_idx;

ALTER TABLE payment_transaction
    DROP CONSTRAINT IF EXISTS chk_payment_transaction_channel,
    DROP CONSTRAINT IF EXISTS chk_payment_transaction_va_bank,
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS va_number,
    DROP COLUMN IF EXISTS va_bank;
