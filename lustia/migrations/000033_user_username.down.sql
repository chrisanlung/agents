-- Migration 000033 — Rollback: remove username from "user" table.

ALTER TABLE "user" DROP CONSTRAINT IF EXISTS user_username_format;
DROP INDEX IF EXISTS user_username_uidx;
ALTER TABLE "user" DROP COLUMN IF EXISTS username;
