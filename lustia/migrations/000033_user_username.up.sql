-- Migration 000033 — Add username to "user" table.
-- Owner confirmed 2026-04-30 (Opsi A): email + username coexist, both unique,
-- login accepts either. Email remains mandatory; username is optional.

-- 1. Add the column (nullable so existing rows are unaffected).
ALTER TABLE "user"
    ADD COLUMN username TEXT NULL;

-- 2. Partial unique index: case-insensitive, non-deleted rows only.
--    Using LOWER() so "alice" and "Alice" cannot coexist.
CREATE UNIQUE INDEX user_username_uidx
    ON "user" (LOWER(username))
    WHERE username IS NOT NULL AND deleted_at IS NULL;

-- 3. Check constraint: enforce format at DB level as a safety net.
--    The service layer validates first; the constraint is a backstop.
--      • 3–50 characters
--      • lowercase alphanumeric + dot/underscore only
--      • value must already be lowercase (username = LOWER(username) prevents
--        mixed-case writes bypassing the application layer)
ALTER TABLE "user"
    ADD CONSTRAINT user_username_format
    CHECK (
        username IS NULL
        OR (
            length(username) BETWEEN 3 AND 50
            AND username ~ '^[a-z0-9._]+$'
            AND username = LOWER(username)
        )
    );
