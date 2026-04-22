#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# run-local.sh — launches auth-service against the host's Postgres 17.
#
# Prereqs:
#   - Postgres 17 running on localhost:5432, database "lustia" exists with all
#     migrations applied (including 000006_user_must_change_password and
#     000007_update_bootstrap_email).
#   - Role lustia_app has the password below (ALTER ROLE lustia_app WITH
#     PASSWORD 'lustia_app_local';).
#   - JWT key pair at ../dev-secrets/jwt_private.pem (4096-bit RSA).
#   - Run from `lustia/services/auth/`.
#
# Usage:
#   source run-local.sh && go run ./cmd/auth
#       or
#   ./run-local.sh run
#
# Customize any var here; none of these values are secrets (dev scope only).
# -----------------------------------------------------------------------------
set -euo pipefail

# --- App ---
export GIN_MODE=debug
export HTTP_PORT=8080
export LOG_LEVEL=info
export CONFIG_PATH="$(pwd)/config/app.yaml"
export APP_DISPLAY_NAME="Lustia"

# --- Database (local Postgres 17) ---
#
# Connects as `lustia_app` with full RLS enforced. The tenant middleware
# opens a per-request transaction, sets app.current_tenant/app.current_user
# via set_config(..., is_local=true), and commits/rolls back at end. For
# public endpoints the default tenant is the '__platform__' sentinel (ADR
# 0005); tenant-staff flows switch via TxManager.SetTenantContext after
# resolving the tenant slug.
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=lustia
export DB_USER=lustia_app
export DB_PASSWORD=lustia_app_local

# --- JWT (dev key pair, 4096-bit RSA) ---
export JWT_PRIVATE_KEY_PATH="$(pwd)/../../dev-secrets/jwt_private.pem"
export JWT_KID=primary

# --- SMTP — off by default. Flip to `true` + start Mailpit to test mail. ---
export SMTP_ENABLED=${SMTP_ENABLED:-false}
export SMTP_HOST=${SMTP_HOST:-localhost}
export SMTP_PORT=${SMTP_PORT:-1025}
export SMTP_USERNAME=${SMTP_USERNAME:-}
export SMTP_PASSWORD=${SMTP_PASSWORD:-}
export SMTP_FROM="${SMTP_FROM:-Lustia <no-reply@lustia.local>}"
export SMTP_STARTTLS=${SMTP_STARTTLS:-false}
export SMTP_TIMEOUT_MS=${SMTP_TIMEOUT_MS:-5000}
export PASSWORD_RESET_URL_BASE=${PASSWORD_RESET_URL_BASE:-http://localhost:3002/auth/reset}

# --- CORS — dev web frontends ---
# Comma-separated exact origins. Leave unset to disable CORS entirely.
export CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS:-http://localhost:3001,http://localhost:3002,http://localhost:3003}

# If invoked with `./run-local.sh run`, actually start the service.
if [ "${1:-}" = "run" ]; then
    exec go run ./cmd/auth
fi
