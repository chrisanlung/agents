#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# Lustia UAT — one-shot server provisioning for Sumopod VPS Tencent (Ubuntu 24.04).
# Idempotent: re-runs are safe.
#
# What this does (as root via sudo):
#   1. Timezone Asia/Jakarta + apt update.
#   2. Install: PostgreSQL 17, build-essential, curl, git, ufw, fail2ban.
#   3. Install golang-migrate CLI (for running DB migrations).
#   4. Create system user `lustia` (no password, no shell login) — auth-service
#      runs as this user (NOT root, NOT ubuntu).
#   5. Create app dirs: /opt/lustia/{bin,migrations,config,storage} owned by lustia.
#   6. UFW firewall: allow 22 (SSH) + 8080 (HTTP backend, no SSL yet — UAT only).
#   7. Postgres: enable btree_gist, create roles `lustia_app` + `lustia_migrator`,
#      create database `lustia` (idempotent — skip if exists).
#
# What it DOES NOT do:
#   - Install the auth-service binary or run migrations (that's done via SCP +
#     migrate.sh from the local machine after this script completes).
#   - Set up SSL / Caddy (no domain yet — UAT runs HTTP-only over IP).
#   - Configure the .env file (copied separately).
# ---------------------------------------------------------------------------
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "Re-running with sudo..."
  exec sudo -E bash "$0" "$@"
fi

PG_APP_PASSWORD="${PG_APP_PASSWORD:-}"
PG_MIGRATOR_PASSWORD="${PG_MIGRATOR_PASSWORD:-}"

if [[ -z "$PG_APP_PASSWORD" || -z "$PG_MIGRATOR_PASSWORD" ]]; then
  echo "ERROR: PG_APP_PASSWORD and PG_MIGRATOR_PASSWORD must be set in env."
  echo "Example: PG_APP_PASSWORD=xxx PG_MIGRATOR_PASSWORD=yyy bash server-setup.sh"
  exit 1
fi

echo "==> [1/7] Timezone + apt update"
timedatectl set-timezone Asia/Jakarta
apt-get update -y
apt-get upgrade -y

echo "==> [2/7] Install base packages"
apt-get install -y --no-install-recommends \
  build-essential curl git ufw fail2ban ca-certificates gnupg lsb-release \
  postgresql-common

# Install PostgreSQL 17 from official PGDG repo (Ubuntu 24.04 default ships PG 16).
echo "==> [3/7] Install PostgreSQL 17"
if ! command -v psql >/dev/null 2>&1 || ! psql --version | grep -q "17\."; then
  /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
  apt-get update -y
  apt-get install -y postgresql-17 postgresql-contrib-17
fi
systemctl enable --now postgresql

echo "==> [4/7] Install golang-migrate CLI v4.17.1"
if ! command -v migrate >/dev/null 2>&1; then
  curl -fsSL https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz \
    | tar xz -C /tmp
  install -m 0755 /tmp/migrate /usr/local/bin/migrate
  rm -f /tmp/migrate /tmp/LICENSE /tmp/README.md
fi

echo "==> [5/7] Create lustia system user + app dirs"
if ! id lustia >/dev/null 2>&1; then
  useradd --system --home /opt/lustia --shell /usr/sbin/nologin --create-home lustia
fi
install -d -o lustia -g lustia -m 0755 \
  /opt/lustia/bin \
  /opt/lustia/migrations \
  /opt/lustia/config \
  /opt/lustia/storage \
  /opt/lustia/storage/uploads
chmod 700 /opt/lustia/config

echo "==> [6/7] UFW firewall (22 + 8080)"
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 8080/tcp
ufw --force enable

echo "==> [7/7] PostgreSQL: btree_gist + roles + database"
sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
-- btree_gist needs to be a contrib package; available in postgresql-contrib-17.
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lustia_migrator') THEN
    CREATE ROLE lustia_migrator WITH LOGIN BYPASSRLS PASSWORD '$PG_MIGRATOR_PASSWORD';
  ELSE
    ALTER ROLE lustia_migrator WITH LOGIN BYPASSRLS PASSWORD '$PG_MIGRATOR_PASSWORD';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lustia_app') THEN
    CREATE ROLE lustia_app WITH LOGIN PASSWORD '$PG_APP_PASSWORD';
  ELSE
    ALTER ROLE lustia_app WITH LOGIN PASSWORD '$PG_APP_PASSWORD';
  END IF;
END
\$\$;

SELECT 'CREATE DATABASE lustia OWNER lustia_migrator'
  WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lustia') \gexec
SQL

# Enable extension inside the lustia DB (must be in DB context, not server).
sudo -u postgres psql -v ON_ERROR_STOP=1 -d lustia <<'SQL'
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
GRANT CONNECT ON DATABASE lustia TO lustia_app;
GRANT USAGE ON SCHEMA public TO lustia_app;
SQL

echo
echo "==========================="
echo "Server setup complete."
echo "==========================="
echo "Postgres: 5432 (localhost only — no external port opened)"
echo "App user: lustia (system, no shell)"
echo "App dir : /opt/lustia"
echo "Firewall: 22 + 8080 only"
echo
echo "Next steps (from your local machine):"
echo "  1. scp the auth binary to /opt/lustia/bin/auth"
echo "  2. scp the migrations folder to /opt/lustia/migrations/"
echo "  3. scp the JWT private key to /opt/lustia/config/jwt_private.pem"
echo "  4. scp the .env file to /opt/lustia/config/.env (chmod 600)"
echo "  5. ssh to server and run: /opt/lustia/bin/migrate-up.sh"
echo "  6. install systemd unit + start"
