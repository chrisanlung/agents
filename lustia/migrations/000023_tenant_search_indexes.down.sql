-- =============================================================================
-- Migration: 000023_tenant_search_indexes (DOWN)
-- =============================================================================

DROP INDEX IF EXISTS tenant_slug_trgm_idx;
DROP INDEX IF EXISTS tenant_name_trgm_idx;

-- pg_trgm extension is left in place — other future indexes may depend on it.
