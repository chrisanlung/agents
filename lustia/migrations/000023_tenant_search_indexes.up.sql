-- =============================================================================
-- Migration: 000023_tenant_search_indexes (UP)
-- Purpose  : Trigram-based search on tenant name + slug for the platform-admin
--            tenant management list. Phase 4 polish — no schema change, just
--            the pg_trgm extension and two GIN indexes.
--
-- Search query pattern (in repository):
--   WHERE (lower(name) LIKE '%q%' OR lower(slug) LIKE '%q%')
-- Trigram GIN indexes make these ILIKE/LIKE substring scans O(log n) instead
-- of full-table scans.
--
-- See docs/DECISIONS/0013-offset-pagination.md for the broader pagination
-- migration; tenant search is the follow-up after offset pagination shipped.
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS tenant_name_trgm_idx
    ON tenant USING GIN (lower(name) gin_trgm_ops)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS tenant_slug_trgm_idx
    ON tenant USING GIN (lower(slug) gin_trgm_ops)
    WHERE deleted_at IS NULL;
