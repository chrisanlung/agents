# Lustia — VM deployment runbook

The target of this runbook is a **single Linux VM** (Ubuntu 22.04+ / Debian 12+
/ similar) that you control. It reuses the same `docker-compose.yml` that runs
on your laptop — only the secret handling and process lifecycle change.

This is **Step 1** of [ADR 0006](../../../docs/DECISIONS/0006-secret-management-staging-prod.md):
plain `.env` on the VM, single operator, no secret manager yet. Migration
triggers to Step 2 (SOPS + age) and Step 3 (cloud KMS) are documented in the
ADR.

> ⚠ **Scope limit:** This is suitable for a dev / demo / evaluation VM, or a
> tightly-scoped internal staging the owner runs alone. The moment a second
> person needs deploy access, or real user data touches the VM, jump to
> Step 2 before anything else.

---

## 1. Provision the VM

Minimum spec for the Phase 1 + 2 stack (Postgres + migrator + auth +
Mailpit):

| Resource | Minimum | Comfortable |
| --- | --- | --- |
| CPU | 1 vCPU | 2 vCPU |
| RAM | 2 GB | 4 GB |
| Disk | 20 GB | 40 GB (leaves room for DB growth + logs) |
| Network | Public IPv4 + SSH port open | Same, plus a firewall restricting 8080/5433 to your IP |

Any provider works: DigitalOcean, Hetzner, Vultr, AWS EC2, Azure VM, GCP
Compute Engine, or your own Proxmox / VMware / bare-metal box. The runbook
assumes Ubuntu 22.04 LTS; translate apt commands for other distros.

**Firewall / security group rules you want:**

- `22/tcp` (SSH) — restrict to your IP if possible.
- `8080/tcp` (auth-service) — public if you want to hit it from anywhere, or
  restrict to your IP for a demo VM.
- `8025/tcp` (Mailpit UI) — **restrict** to your IP. Never leave the Mailpit
  UI open to the internet; it exposes every captured email with zero auth.
- `5433/tcp` (Postgres) — keep it closed externally. Connect via SSH
  tunnel when you need `psql` access.

---

## 2. Bootstrap the VM

Log in as a sudo-capable user and run:

```bash
# --- basic hygiene ---
sudo apt update && sudo apt upgrade -y
sudo apt install -y git openssl ufw curl

# --- create a dedicated deploy user (no sudo, no password login) ---
sudo adduser --disabled-password --gecos "" lustia
sudo usermod -aG docker lustia          # added after docker is installed below

# --- install docker engine + compose plugin ---
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker

# --- basic firewall (tweak as needed) ---
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 8080/tcp
# Mailpit UI — uncomment ONE line:
# sudo ufw allow 8025/tcp                         # if you accept public exposure (dev demo)
# sudo ufw allow from YOUR.IP.HERE to any port 8025  # safer
sudo ufw --force enable
```

Now add your SSH key to the `lustia` user so you can do the rest without
sudo:

```bash
sudo mkdir -p /home/lustia/.ssh
sudo cp ~/.ssh/authorized_keys /home/lustia/.ssh/authorized_keys
sudo chown -R lustia:lustia /home/lustia/.ssh
sudo chmod 700 /home/lustia/.ssh
sudo chmod 600 /home/lustia/.ssh/authorized_keys
```

From here on, `ssh lustia@<vm-ip>`.

---

## 3. Clone the repo and prepare layout

```bash
# as user `lustia`
sudo mkdir -p /srv/lustia
sudo chown lustia:lustia /srv/lustia
cd /srv/lustia

# clone the lusthing workspace (adjust repo URL)
git clone https://github.com/<your-org>/<your-repo>.git .

# sanity
ls lustia/deploy
#   docker-compose.yml  .env.example  init-db/  secrets/  README.md  vm/
```

---

## 4. Fill in the `.env`

```bash
cd /srv/lustia/lustia/deploy
cp .env.example .env

# Edit every CHANGE_ME_* and set production-grade passwords.
# Use a password manager to generate — do NOT reuse across environments.
$EDITOR .env

# Lock it down so only `lustia` can read it.
chmod 600 .env
```

**Recommended diff from `.env.example` for a VM:**

```diff
- POSTGRES_PASSWORD=CHANGE_ME_dev_only
+ POSTGRES_PASSWORD=<32-char random>

- LUSTIA_APP_PASSWORD=CHANGE_ME_dev_only
+ LUSTIA_APP_PASSWORD=<32-char random>

- SUPER_ADMIN_INITIAL_PASSWORD=CHANGE_ME_rotate_on_first_login
+ SUPER_ADMIN_INITIAL_PASSWORD=<20-char random; rotate on first login>

- SUPER_ADMIN_EMAIL=admin@lustia.local
+ SUPER_ADMIN_EMAIL=your.real.alias@your-domain.tld
```

If this VM will be accessed from a real domain, also update:

```diff
- PASSWORD_RESET_URL_BASE=http://localhost:3000/auth/reset
+ PASSWORD_RESET_URL_BASE=https://app.your-domain.tld/auth/reset
```

SMTP defaults point at Mailpit (inside the compose stack). If you want
**real** mail to reach your inbox, swap to a transactional provider:

```diff
- SMTP_HOST=mailpit
- SMTP_PORT=1025
- SMTP_USERNAME=
- SMTP_PASSWORD=
- SMTP_STARTTLS=false
+ SMTP_HOST=email-smtp.ap-southeast-3.amazonaws.com      # example: AWS SES Jakarta
+ SMTP_PORT=587
+ SMTP_USERNAME=<SES SMTP username>
+ SMTP_PASSWORD=<SES SMTP password>
+ SMTP_STARTTLS=true
+ SMTP_FROM=Lustia <no-reply@your-verified-domain.tld>
```

You can keep Mailpit running as an internal catch-all while also wiring
real SMTP — they don't conflict. For a first VM, leaving SMTP pointed at
Mailpit is fine: you see the emails in the UI, users never get them, no
spoofing risk.

---

## 5. Generate the JWT key pair

```bash
cd /srv/lustia/lustia/deploy
openssl genrsa -out secrets/jwt_private.pem 4096
openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem
chmod 600 secrets/jwt_private.pem
```

⚠ **Never reuse dev keys on a VM that will hold real credentials.** Generate
fresh ones here.

---

## 6. First boot

```bash
cd /srv/lustia/lustia/deploy
docker compose up -d
docker compose logs -f migrator           # wait for clean exit (status 0)

# smoke test (from the VM)
curl -sf http://localhost:8080/readyz
```

From your laptop:

```bash
curl -sf http://<vm-ip>:8080/readyz
open http://<vm-ip>:8025                  # Mailpit UI — restrict firewall first
```

---

## 7. Bootstrap the super-admin password

Same as the laptop runbook in [`../README.md`](../README.md) step 6, but run
the `hashpw` utility from the auth container to avoid putting the plaintext
on your laptop's shell history:

```bash
# on the VM, as user `lustia`
docker compose exec auth /hashpw
# enter the password when prompted — it is not echoed nor stored in history

# the command prints an UPDATE statement. Run it:
docker compose exec -T postgres psql \
    -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$(cat <<'SQL'
UPDATE "user"
   SET password_hash = '<PASTE HASH HERE>'
 WHERE id = 'a0000000-0000-0000-0000-000000000001';
SQL
)"
```

Now log in:

```bash
curl -sf -X POST http://<vm-ip>:8080/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"email":"<super admin email>","password":"<the password>","tenant_slug":"__platform__"}'
```

The response carries `must_change_password` in the JWT claim — only
`GET /auth/me`, `POST /auth/me/password`, and `POST /auth/logout` are open
until you rotate via `POST /auth/me/password`. Same flow as laptop.

---

## 8. Keep the stack alive across reboots

Compose already has `restart: unless-stopped` on every long-running service.
That handles container crashes. To survive **VM reboots**, install the
systemd unit that ships next to this README:

```bash
sudo cp /srv/lustia/lustia/deploy/vm/lustia.service /etc/systemd/system/lustia.service
sudo systemctl daemon-reload
sudo systemctl enable --now lustia.service

# verify
sudo systemctl status lustia.service
```

What it does:
- On boot, `docker compose up -d` in `/srv/lustia/lustia/deploy/`.
- On shutdown, `docker compose down` (clean stop).
- Runs as user `lustia` — no root privileges inside the service.

---

## 9. Ongoing operations

### Deploy a new version
```bash
cd /srv/lustia
git pull
cd lustia/deploy
docker compose pull                # refresh pinned images (postgres, mailpit, migrator)
docker compose build auth          # rebuild local image on new code
docker compose up -d               # rolling restart where possible
docker compose logs -f migrator    # verify any new migration applied cleanly
```

### Rotate the super-admin password
Use the `hashpw` utility per step 7. The `must_change_password` flag is set
by migration 000006; any admin-created account will also force-rotate.

### Rotate the `lustia_app` DB password
1. Decide new password, update in Postgres:
   ```bash
   docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
     "ALTER ROLE lustia_app WITH PASSWORD '<new>';"
   ```
2. Update `.env` (`LUSTIA_APP_PASSWORD=<new>`).
3. `docker compose up -d auth` to pick up the new env.

### Rotate the JWT private key
Blueprint in [`../../../docs/SECURITY.md`](../../../docs/SECURITY.md) §9.2.
Requires multi-`kid` JWKS — tracked as an open item in OPERATIONS.md.

### Backups
Phase 1+2 treats this VM as replaceable. DB is ground truth:

```bash
# manual snapshot before any risky operation
docker compose exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom \
    > /srv/lustia/backups/lustia-$(date +%Y%m%d-%H%M%S).pgcustom

# list
ls -la /srv/lustia/backups/
```

Automate this with a cron on the VM if you stay on Step 1:

```cron
0 3 * * * cd /srv/lustia/lustia/deploy && docker compose exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom > /srv/lustia/backups/lustia-$(date +\%Y\%m\%d).pgcustom
# plus weekly: prune older than 28 days
```

For Step 2 / 3, move to managed snapshots (cloud-native or Restic to S3).

### Shutdown / teardown
```bash
sudo systemctl stop lustia.service      # graceful compose down
# to fully remove (keeps the DB volume unless -v):
cd /srv/lustia/lustia/deploy
docker compose down                     # stop + remove containers
docker compose down -v                  # also drop postgres_data volume — DB gone
```

---

## 10. When to graduate to Step 2 (SOPS + age)

Triggers from ADR 0006:

- ☐ A second person needs SSH / deploy access to this VM.
- ☐ The `.env` file is being copied / pasted in team chat.
- ☐ Compliance asks "how is secret access logged?"
- ☐ More than one VM.
- ☐ Real user data (not demo / synthetic) is stored.

When any of those becomes true, follow the SOPS + age migration runbook
(to be written in `vm/upgrade-step2.md` when first needed). The service
code and compose file **do not change** — only the mechanism that fills
`.env` changes.
