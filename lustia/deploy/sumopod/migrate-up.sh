#!/usr/bin/env bash
# Run forward DB migrations as lustia_migrator (BYPASSRLS).
# Usage: PG_MIGRATOR_PASSWORD=xxx bash migrate-up.sh
set -euo pipefail

if [[ -z "${PG_MIGRATOR_PASSWORD:-}" ]]; then
  echo "ERROR: PG_MIGRATOR_PASSWORD must be set."
  exit 1
fi

DB_URL="postgres://lustia_migrator:${PG_MIGRATOR_PASSWORD}@localhost:5432/lustia?sslmode=disable"

migrate -path /opt/lustia/migrations -database "$DB_URL" up
