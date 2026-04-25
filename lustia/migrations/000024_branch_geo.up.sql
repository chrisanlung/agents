-- =============================================================================
-- Migration: 000024_branch_geo (UP)
-- Purpose  : Phase 5 — Add geolocation coordinates to the branch table,
--            enabling the "nearest cabang" sort in the customer mobile app.
--            Implements ADR 0014 §3.9.
--
-- Extensions added:
--   cube          — prerequisite for earthdistance (installs earth() functions)
--   earthdistance — point-to-point distance queries using cube-based earth model
--
-- NOTE: btree_gist, pgcrypto, citext are already created in migration 000001.
-- Do NOT re-create them here.
--
-- CLEAN invariant: migrations 1 → 24 produce a working schema.
--
-- Re-run safety:
--   ALTER TABLE … ADD COLUMN uses IF NOT EXISTS.
--   CREATE EXTENSION uses IF NOT EXISTS.
--   CREATE INDEX uses IF NOT EXISTS.
--
-- Design notes:
--   NUMERIC(9,6) stores ±180.000000 with 6 decimal places (~0.1 m precision).
--   DOUBLE PRECISION was rejected: NUMERIC is exact (no float rounding), and
--   6dp precision (±0.111 m at equator) exceeds the map-picker input accuracy.
--   earthdistance (cube-based) is used instead of PostGIS because the only
--   operation needed is "sort by distance" — a full spatial engine is not
--   warranted at this scale. earthdistance is a standard Postgres contrib
--   module; PostGIS requires OS-level installation.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 0. Extensions required by this migration
--    cube must be created first — earthdistance depends on it.
-- ---------------------------------------------------------------------------

CREATE EXTENSION IF NOT EXISTS cube;
CREATE EXTENSION IF NOT EXISTS earthdistance;

-- ---------------------------------------------------------------------------
-- 1. Add latitude and longitude to branch
--
--    Both columns are NULL by default — existing branches have no coordinates
--    until a tenant admin enters them via the branch form (Phase 5) or the
--    planned geocoding job (Phase 6). NULL is semantically correct: "location
--    not yet specified" is different from "0° N, 0° E" (Gulf of Guinea).
-- ---------------------------------------------------------------------------

ALTER TABLE branch
    ADD COLUMN IF NOT EXISTS latitude  NUMERIC(9,6) NULL
        CONSTRAINT chk_branch_latitude  CHECK (latitude  IS NULL OR latitude  BETWEEN -90   AND 90),
    ADD COLUMN IF NOT EXISTS longitude NUMERIC(9,6) NULL
        CONSTRAINT chk_branch_longitude CHECK (longitude IS NULL OR longitude BETWEEN -180  AND 180);

COMMENT ON COLUMN branch.latitude IS
    'WGS84 latitude in decimal degrees. Range -90..90. NULL = coordinates not '
    'yet entered by tenant admin. Populated via the branch-form lat/lng fields '
    '(Phase 5) or automatic geocoding (Phase 6). Used by the mobile app to '
    'sort branches by distance using the earthdistance extension.';

COMMENT ON COLUMN branch.longitude IS
    'WGS84 longitude in decimal degrees. Range -180..180. NULL = coordinates '
    'not yet entered. Same lifecycle as latitude.';

-- ---------------------------------------------------------------------------
-- 2. Partial index for geo queries
--
--    Covers only rows where BOTH coordinates are set AND the branch is not
--    soft-deleted. A branch with only one coordinate set is not useful for
--    distance queries; the partial predicate excludes it.
--
--    The mobile app query is:
--      SELECT … FROM branch
--      WHERE deleted_at IS NULL AND status = 'active'
--      ORDER BY earth_distance(ll_to_earth(lat, lng), ll_to_earth($1, $2))
--
--    An ordinary B-tree index on (latitude, longitude) is used here instead
--    of a GiST index on an earth() value because NUMERIC columns cannot be
--    indexed with earthdistance's cube_gist operator class directly. For
--    Phase 5 volumes (hundreds of branches) a B-tree partial index is
--    sufficient; a full PostGIS GiST index can be added in Phase 6 if query
--    plans degrade.
-- ---------------------------------------------------------------------------

CREATE INDEX IF NOT EXISTS branch_geo_idx
    ON branch (latitude, longitude)
    WHERE latitude IS NOT NULL AND longitude IS NOT NULL AND deleted_at IS NULL;
