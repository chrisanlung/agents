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

# --- Phase 6: Payment adapter (ADR 0015, SECURITY.md C-2) ---
# APP_ENV must be "dev" or "local" when PAYMENT_PROVIDER=dummy.
# The service will log.Fatal at startup if PAYMENT_PROVIDER=dummy and
# APP_ENV is anything else (production safety gate).
export APP_ENV=dev
# export PAYMENT_PROVIDER=dummy

# --- iPaymu sandbox (Phase 6) ---
# Switch to the iPaymu sandbox by commenting out PAYMENT_PROVIDER=dummy above
# and uncommenting the five lines below. Steps to bring up the tunnel first:
#   1. ngrok http 8080  (already authed; run in a separate shell, leave open)
#   2. Confirm the public URL: curl -s http://127.0.0.1:4040/api/tunnels
#   3. If the ngrok URL changes (free plan rotates), update IPAYMU_NOTIFY_URL
#      below and re-source this file + restart the auth service.
#   4. Test path: create a booking via mobile -> grab trx_id from auth logs ->
#      visit https://sandbox.ipaymu.com/send-notify and push a synthetic
#      notification to the IPAYMU_NOTIFY_URL value below.
#
export PAYMENT_PROVIDER=ipaymu
export IPAYMU_BASE_URL=https://sandbox.ipaymu.com
export IPAYMU_VA=0000005714983489
export IPAYMU_API_KEY=SANDBOX429CAC48-22DE-43FE-9153-26DD0BB5671D
export IPAYMU_NOTIFY_URL=https://darkish-trifle-kept.ngrok-free.dev/api/v1/public/payments/webhook
# Sandbox-only: simulator emits a static placeholder X-Signature, not a real
# HMAC. Skip verification so end-to-end testing works. NEVER set in prod.
export IPAYMU_SKIP_SIGNATURE_VERIFY=true

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
export CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS:-http://localhost:3001,http://localhost:3002,http://localhost:3003,http://localhost:5000,http://localhost:8081}

# --- Storage (ADR 0011) ---
# Local driver writes to a sub-directory of the repo checkout (gitignored).
# Switch to r2/supabase by overriding STORAGE_DRIVER and supplying credentials
# before sourcing this file. Complete OPERATIONS.md §10.4 before doing so.
export STORAGE_DRIVER=local
export STORAGE_LOCAL_PATH="$(pwd)/storage-data/uploads"
export STORAGE_PUBLIC_BASE_URL=http://localhost:8080/uploads
export UPLOAD_MAX_MB=5
export UPLOAD_TENANT_HOURLY_LIMIT=30

# Ensure the local upload directory exists before the service starts.
# The service itself fails-fast if the path is missing or non-writable (ADR 0011 §2.2).
mkdir -p "$STORAGE_LOCAL_PATH"

# If invoked with `./run-local.sh run`, actually start the service.
if [ "${1:-}" = "run" ]; then
    exec go run ./cmd/auth
fi
