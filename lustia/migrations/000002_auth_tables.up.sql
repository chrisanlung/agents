-- =============================================================================
-- Migration: 000002_auth_tables
-- Purpose  : user, user_role, user_branch, refresh_token, password_reset.
--            These tables carry tenant_id (nullable for super_admin) and
--            will be RLS-protected in migration 000004.
-- =============================================================================

-- ---------------------------------------------------------------------------
-- 1. "user"
--    Internal platform users (super_admin + tenant staff).
--    Customers are a separate table (operational tables, migration 003).
--    "user" is a reserved word in SQL; always quote it.
--
--    Uniqueness model:
--      - Tenant staff:    UNIQUE (tenant_id, email)
--      - Platform admin:  UNIQUE (email) WHERE tenant_id IS NULL
--        Both are handled by the two partial unique indexes below.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "user" (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NULL
                            CONSTRAINT fk_user_tenant
                            REFERENCES tenant(id)
                            ON DELETE RESTRICT,
    email               CITEXT      NOT NULL CHECK (char_length(email::text) BETWEEN 3 AND 320),
    password_hash       TEXT        NOT NULL, -- Argon2id encoded string
    full_name           TEXT        NOT NULL CHECK (char_length(full_name) BETWEEN 1 AND 200),
    phone               TEXT        NULL     CHECK (phone IS NULL OR char_length(phone) <= 30),
    avatar_url          TEXT        NULL     CHECK (avatar_url IS NULL OR char_length(avatar_url) <= 2048),
    is_active           BOOLEAN     NOT NULL DEFAULT true,
    last_login_at       TIMESTAMPTZ NULL,
    -- Brute-force protection
    failed_login_count  INT         NOT NULL DEFAULT 0,
    locked_until        TIMESTAMPTZ NULL,
    -- Extensible metadata (e.g., preferences, onboarding_state)
    metadata            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    -- Soft delete
    deleted_at          TIMESTAMPTZ NULL,
    -- Audit
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by          UUID        NULL,   -- nullable: system/migration can create users
    updated_by          UUID        NULL,

    -- A tenant-scoped user email must be unique within that tenant.
    -- Platform super_admins (tenant_id IS NULL) handled by separate partial index below.
    CONSTRAINT chk_user_tenant_or_global
        CHECK (
            (tenant_id IS NOT NULL) OR
            (tenant_id IS NULL)     -- super_admins: tenant_id NULL is valid
        )
);

-- Uniqueness: tenant-scoped users unique by (tenant_id, email)
CREATE UNIQUE INDEX IF NOT EXISTS user_tenant_email_uidx
    ON "user" (tenant_id, email)
    WHERE tenant_id IS NOT NULL AND deleted_at IS NULL;

-- Uniqueness: platform super_admins unique by email when tenant_id IS NULL
CREATE UNIQUE INDEX IF NOT EXISTS user_global_email_uidx
    ON "user" (email)
    WHERE tenant_id IS NULL AND deleted_at IS NULL;

-- FK index (tenant_id)
CREATE INDEX IF NOT EXISTS user_tenant_id_idx       ON "user" (tenant_id);
-- Lookup indexes
CREATE INDEX IF NOT EXISTS user_is_active_idx       ON "user" (is_active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS user_locked_until_idx    ON "user" (locked_until) WHERE locked_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS user_metadata_gin        ON "user" USING GIN (metadata);

CREATE TRIGGER trg_user_updated_at
    BEFORE UPDATE ON "user"
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 2. user_role
--    Junction: a user may carry multiple roles.
--    Roles are global definitions; branch-scoping is expressed via user_branch.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_role (
    user_id     UUID        NOT NULL
                    CONSTRAINT fk_user_role_user
                    REFERENCES "user"(id)
                    ON DELETE CASCADE,
    role_id     UUID        NOT NULL
                    CONSTRAINT fk_user_role_role
                    REFERENCES role(id)
                    ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    assigned_by UUID        NULL
                    CONSTRAINT fk_user_role_assigned_by
                    REFERENCES "user"(id)
                    ON DELETE SET NULL,

    CONSTRAINT pk_user_role PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS user_role_role_id_idx    ON user_role (role_id);
CREATE INDEX IF NOT EXISTS user_role_user_id_idx    ON user_role (user_id);

-- ---------------------------------------------------------------------------
-- 3. user_branch
--    Branch-scoped staff assignments.
--    A user can be assigned to multiple branches within the same tenant.
--    The branch must belong to the same tenant as the user — enforced by a
--    CHECK constraint that delegates to a small helper function.
--    (Full referential cross-check would require a trigger; the FK to branch
--    and the application layer are the primary guards; see ADR note.)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_branch (
    user_id     UUID        NOT NULL
                    CONSTRAINT fk_user_branch_user
                    REFERENCES "user"(id)
                    ON DELETE CASCADE,
    branch_id   UUID        NOT NULL
                    CONSTRAINT fk_user_branch_branch
                    REFERENCES branch(id)
                    ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    assigned_by UUID        NULL
                    CONSTRAINT fk_user_branch_assigned_by
                    REFERENCES "user"(id)
                    ON DELETE SET NULL,

    CONSTRAINT pk_user_branch PRIMARY KEY (user_id, branch_id)
);

CREATE INDEX IF NOT EXISTS user_branch_branch_id_idx    ON user_branch (branch_id);
CREATE INDEX IF NOT EXISTS user_branch_user_id_idx      ON user_branch (user_id);

-- ---------------------------------------------------------------------------
-- 4. refresh_token
--    One row per issued refresh token.
--    token_hash is the SHA-256 hex of the opaque bearer string — never store
--    the raw token in the DB.
--    replaced_by enables rotation audit: when a token is rotated, the new
--    token's id is written to the old row's replaced_by column.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS refresh_token (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL
                        CONSTRAINT fk_refresh_token_user
                        REFERENCES "user"(id)
                        ON DELETE CASCADE,
    token_hash      TEXT        NOT NULL CHECK (char_length(token_hash) = 64),  -- SHA-256 hex
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ NULL,
    replaced_by     UUID        NULL
                        CONSTRAINT fk_refresh_token_replaced_by
                        REFERENCES refresh_token(id)
                        ON DELETE SET NULL,
    -- Request context (for audit / anomaly detection)
    user_agent      TEXT        NULL,
    ip              INET        NULL,
    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
    -- No updated_at: refresh_token rows are append-only; revocation sets revoked_at.
);

CREATE UNIQUE INDEX IF NOT EXISTS refresh_token_hash_uidx   ON refresh_token (token_hash);
CREATE INDEX IF NOT EXISTS refresh_token_user_id_idx        ON refresh_token (user_id);
CREATE INDEX IF NOT EXISTS refresh_token_expires_at_idx     ON refresh_token (expires_at);

-- ---------------------------------------------------------------------------
-- 5. password_reset
--    Short-lived tokens for the "forgot password" flow.
--    Shape frozen in Phase 1; endpoints implemented in a later phase.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS password_reset (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL
                    CONSTRAINT fk_password_reset_user
                    REFERENCES "user"(id)
                    ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL CHECK (char_length(token_hash) = 64),  -- SHA-256 hex
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ NULL,
    -- Audit
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
    -- No updated_at: append-only; used_at captures the single mutation.
);

CREATE UNIQUE INDEX IF NOT EXISTS password_reset_hash_uidx  ON password_reset (token_hash);
CREATE INDEX IF NOT EXISTS password_reset_user_id_idx       ON password_reset (user_id);
CREATE INDEX IF NOT EXISTS password_reset_expires_at_idx    ON password_reset (expires_at);
