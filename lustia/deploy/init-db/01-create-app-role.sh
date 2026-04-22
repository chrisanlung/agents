#!/bin/sh
# ---------------------------------------------------------------------------
# 01-create-app-role.sh
#
# Runs on first boot of the postgres container (triggered by
# /docker-entrypoint-initdb.d). Creates the `lustia_app` role that the
# auth-service (and every later service) uses for normal, RLS-enforced
# traffic. The `lustia_migrator` role and all schema objects are created by
# migration 000001.
#
# Environment (from docker-compose.yml):
#   POSTGRES_USER        → superuser owning the cluster (e.g. lustia_superuser)
#   POSTGRES_DB          → database name (e.g. lustia)
#   LUSTIA_APP_PASSWORD  → password for the new lustia_app role
#
# This file is re-run ONLY when the postgres_data volume is empty. To re-run
# against a fresh cluster: `docker compose down -v && docker compose up -d`.
# ---------------------------------------------------------------------------

set -eu

: "${POSTGRES_USER:?POSTGRES_USER is required}"
: "${POSTGRES_DB:?POSTGRES_DB is required}"
: "${LUSTIA_APP_PASSWORD:?LUSTIA_APP_PASSWORD is required}"

echo "[init-db] creating lustia_app role in database ${POSTGRES_DB}"

# Pass the password via a psql variable (:'app_password') rather than shell
# interpolation so special characters in the password cannot break out of
# the SQL literal. The DO block creates the role if absent, or resets its
# password if it already exists (idempotent across re-runs on a fresh volume).
psql --username "${POSTGRES_USER}" --dbname "${POSTGRES_DB}" \
     --no-psqlrc \
     --set ON_ERROR_STOP=1 \
     --set "db_name=${POSTGRES_DB}" \
     --set "app_password=${LUSTIA_APP_PASSWORD}" <<'SQL'
SET lustia.app_password = :'app_password';

DO
$do$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'lustia_app') THEN
    EXECUTE format(
      'CREATE ROLE lustia_app LOGIN PASSWORD %L',
      current_setting('lustia.app_password')
    );
  ELSE
    EXECUTE format(
      'ALTER ROLE lustia_app WITH LOGIN PASSWORD %L',
      current_setting('lustia.app_password')
    );
  END IF;
END
$do$;

GRANT CONNECT ON DATABASE :"db_name" TO lustia_app;
SQL

echo "[init-db] lustia_app role ready"
