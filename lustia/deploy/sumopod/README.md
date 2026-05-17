# Sumopod UAT deploy — auth-service on Tencent VPS Ubuntu 24.04

Single-server UAT deploy for `auth-service`. PostgreSQL 17 self-installed on
the same VPS (no managed DB). HTTP-only on port 8080 (no domain / SSL yet).
Frontend (3 portals) deployed separately to Vercel later.

## Server

- Public IP: `43.129.55.236` (Sumopod Tencent)
- OS: Ubuntu 24.04 LTS
- Spec: 2 vCPU / 4 GB RAM / 60 GB disk

## Files in this directory

- `server-setup.sh` — runs **on the server**: installs PG 17 + golang-migrate,
  creates `lustia` system user + app dirs, configures UFW, sets up Postgres
  roles + database. **Idempotent.**
- `lustia-auth.service` — systemd unit, deployed to `/etc/systemd/system/`.
- `env.uat.template` — environment file template, becomes `/opt/lustia/config/.env`.
- `migrate-up.sh` — wrapper around `migrate` CLI (golang-migrate v4.17.1).

## Deploy flow

> Run from your local machine (Windows / Git Bash). All commands target
> `ubuntu@43.129.55.236`. Replace `<PASS>` with the SSH password and the
> `<APP_PG_PW>` / `<MIGRATOR_PG_PW>` placeholders with strong random values
> before running.

### 1. Push deploy files to the server

```bash
plink -ssh -pw <PASS> ubuntu@43.129.55.236 "mkdir -p ~/deploy"
pscp -pw <PASS> server-setup.sh migrate-up.sh lustia-auth.service \
     ubuntu@43.129.55.236:~/deploy/
```

### 2. Run server provisioning

```bash
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "cd ~/deploy && PG_APP_PASSWORD=<APP_PG_PW> PG_MIGRATOR_PASSWORD=<MIGRATOR_PG_PW> bash server-setup.sh"
```

This installs PG 17, creates the `lustia` DB + roles, opens UFW 22+8080.

### 3. Build the binary locally (cross-compile to Linux amd64)

```bash
cd lustia/services/auth
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/auth-linux ./cmd/auth
```

### 4. Push binary, migrations, and JWT key to the server

```bash
# Binary
pscp -pw <PASS> bin/auth-linux ubuntu@43.129.55.236:/tmp/auth
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "sudo install -o lustia -g lustia -m 0755 /tmp/auth /opt/lustia/bin/auth && rm /tmp/auth"

# Migrations
pscp -pw <PASS> -r ../../migrations ubuntu@43.129.55.236:/tmp/migrations
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "sudo rsync -a --delete --chown=lustia:lustia /tmp/migrations/ /opt/lustia/migrations/ && rm -rf /tmp/migrations"

# JWT private key (generate fresh for UAT — DO NOT reuse dev key)
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out /tmp/jwt_uat.pem
pscp -pw <PASS> /tmp/jwt_uat.pem ubuntu@43.129.55.236:/tmp/jwt_private.pem
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "sudo install -o lustia -g lustia -m 0600 /tmp/jwt_private.pem /opt/lustia/config/jwt_private.pem && rm /tmp/jwt_private.pem"
rm /tmp/jwt_uat.pem
```

### 5. Build and push `.env`

Copy `env.uat.template` to `env.uat.local`, fill in placeholders
(IPAYMU credentials, PG passwords), then push:

```bash
pscp -pw <PASS> env.uat.local ubuntu@43.129.55.236:/tmp/.env
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "sudo install -o lustia -g lustia -m 0600 /tmp/.env /opt/lustia/config/.env && rm /tmp/.env"
```

### 6. Run migrations

```bash
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "PG_MIGRATOR_PASSWORD=<MIGRATOR_PG_PW> bash ~/deploy/migrate-up.sh"
```

### 7. Install systemd unit + start the service

```bash
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "sudo install -o root -g root -m 0644 ~/deploy/lustia-auth.service /etc/systemd/system/ \
   && sudo systemctl daemon-reload \
   && sudo systemctl enable --now lustia-auth"
```

### 8. Verify

```bash
# Liveness from outside
curl -sS http://43.129.55.236:8080/healthz

# Readiness (also pings DB)
curl -sS http://43.129.55.236:8080/readyz

# Logs
plink -ssh -pw <PASS> ubuntu@43.129.55.236 "sudo journalctl -u lustia-auth -n 50 --no-pager"
```

## Re-deploys (after the first one)

When code changes:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/auth-linux ./cmd/auth
pscp -pw <PASS> bin/auth-linux ubuntu@43.129.55.236:/tmp/auth
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "sudo install -o lustia -g lustia -m 0755 /tmp/auth /opt/lustia/bin/auth \
   && rm /tmp/auth \
   && sudo systemctl restart lustia-auth"
```

When migrations change:

```bash
pscp -pw <PASS> -r ../../migrations ubuntu@43.129.55.236:/tmp/migrations
plink -ssh -pw <PASS> ubuntu@43.129.55.236 \
  "sudo rsync -a --delete --chown=lustia:lustia /tmp/migrations/ /opt/lustia/migrations/ \
   && rm -rf /tmp/migrations \
   && PG_MIGRATOR_PASSWORD=<MIGRATOR_PG_PW> bash ~/deploy/migrate-up.sh"
```

## Hardening checklist (do before customer testing starts)

- [ ] Disable password SSH; switch to key-only auth.
- [ ] Change the initial Ubuntu user password (the one shared in chat).
- [ ] Rotate the iPaymu sandbox API key if it was shared in chat.
- [ ] Set up a domain + Caddy + Let's Encrypt; switch CORS_ALLOWED_ORIGINS to
      Vercel HTTPS origins.
- [ ] Configure SMTP for password-reset emails.
- [ ] Schedule daily `pg_dump` to off-server storage (rsync to S3-compatible).
- [ ] fail2ban: enable `[sshd]` jail.
