# ADR 0006 — Secret Management for Staging & Production

- **Status:** Accepted
- **Date:** 2026-04-21
- **Deciders:** owner (2026-04-21 — "aku ingin mencoba di VM dulu")

---

## Context

Phase 1 + 2 keep every secret in one of two local-dev-only locations:

- `lustia/deploy/.env` — DB passwords, super-admin bootstrap password, SMTP credentials.
- `lustia/deploy/secrets/jwt_private.pem` — JWT signing key, mounted read-only into the auth container.

Both are gitignored. Neither is acceptable for any environment that holds real user data. SECURITY.md §9.4 lists the secret-manager requirement as a blocking gate for staging deploy.

Secrets currently in scope:

| Secret | Consumer | Sensitivity | Rotation cadence (per SECURITY.md §9) |
| --- | --- | --- | --- |
| JWT RS256 private key | auth-service | Critical — full token forgery on compromise | Annual + on compromise |
| `lustia_app` DB password | auth-service, future services | High | Semi-annual |
| `lustia_migrator` DB password | migrator container | High | Semi-annual |
| Postgres superuser password | compose (dev only) | High (dev scope) | N/A in prod — no superuser access |
| Super-admin bootstrap password | operator, one-shot | High (one-time) | Rotated on first login (`must_change_password`) |
| SMTP credentials (provider API key) | auth-service | Medium | Per provider recommendation |

Future secrets (will land in later phases): payment gateway API keys, storage provider credentials, OpenTelemetry collector tokens.

---

## Options considered

| # | Option | Pros | Cons |
| --- | --- | --- | --- |
| A | **AWS Secrets Manager** + IAM role federation | Cloud-native; tight integration with ECS / EKS / Lambda; built-in rotation for RDS; audit via CloudTrail | AWS-only; costs per secret per month; opinionated API |
| B | **GCP Secret Manager** + service accounts | Cloud-native GCP; simple API; automatic replication | GCP-only |
| C | **Azure Key Vault** | Cloud-native Azure; HSM-backed option | Azure-only |
| D | **HashiCorp Vault** (self-hosted) | Multi-cloud; dynamic DB creds; PKI; transit encryption; industry standard | Operationally heavy — HA, unseal, backup all the team's problem |
| E | **HashiCorp Vault (HCP managed)** | Same feature set as D without self-host burden | Costs per hour; vendor lock-in on the feature set |
| F | **Doppler** (SaaS) | Fastest team onboarding; free tier; good CLI + CI integration; audit log | Third-party trust; pricing scales per seat + per secret; less fine-grained RBAC than Vault |
| G | **SOPS + cloud KMS** (e.g. `age` + AWS KMS) | Secrets live in git (encrypted); no extra infra; great for small teams | Rotation is manual; no dynamic creds; decryption happens at deploy time, not per-request |
| H | **Kubernetes Secrets + External Secrets Operator** | Works with A/B/C/D as backends; transparent to app (reads from env or file as usual) | Adds a layer; requires k8s |

---

## Decision

**Lustia ships on a VM first (single docker host).** Two-step path:

1. **Step 1 — "Mencoba" on VM (now)** — Same `.env` workflow as laptop, relocated to the VM. `.env` file lives at `/srv/lustia/deploy/.env` with mode `0600`, owned by the `lustia` deploy user. `.gitignored`, never leaves the VM. Deploy flow: `git pull` + `docker compose up -d`. Systemd unit keeps the stack running across reboots. This is the explicit-transitional posture — fine for a single operator on a dedicated server.
2. **Step 2 — SOPS + `age` (before second operator gets SSH access, or before "real" user data)** — Option **G**. Secrets encrypted at rest in the git repo using `age` recipients; decrypted at deploy time by the deploy user's age key which lives on the VM at `~/.config/sops/age/keys.txt` with mode `0600`. Enables multiple operators without sharing plaintext. Rotation is manual but auditable via git history.
3. **Step 3 — cloud KMS or managed secret store (when horizontally scaling, when audit regulations apply, or when moving to k8s / multi-VM)** — upgrade the `age` pattern to AWS KMS / Google Cloud KMS backing SOPS, or migrate to Option A/B/H entirely. **App code still does not change** — the invariant below is what makes this possible.

---

## Non-negotiable invariants regardless of step

1. **App-side shape does not change.** Auth-service continues to read from env vars and mounted files. No direct Vault/AWS SDK calls in the service code — that ties us to one backend.
2. **Least privilege per environment.** Staging workloads CANNOT read production secrets and vice versa. Enforce at the host/IAM/KMS layer — whichever is active at the current step.
3. **Audit log exists.** Step 1: git history of `.env.example` changes + server-side access log. Step 2: git history of encrypted files. Step 3: cloud audit log.
4. **Rotation runbooks exist BEFORE production.** See SECURITY.md §9.2 for JWT; equivalent needed for every secret row in the table above.

---

## Migration triggers (Step 1 → 2 → 3)

| Trigger | Action |
| --- | --- |
| A second person needs to deploy | Move to Step 2 (SOPS + age) |
| Compliance audit asks "how is access logged" | Move to Step 2 or 3 |
| More than one VM runs the stack | Move to Step 3 (cloud secret store) |
| Real user data hits staging | Confirm Step 2 minimum; re-audit before prod |
| Switch to k8s | Move to Step 3 (External Secrets Operator + cloud KMS) |

**Non-negotiable invariants regardless of backend:**

1. **App-side shape does not change.** Auth-service continues to read from env vars and mounted files. No direct Vault/AWS SDK calls in the service code — that ties us to one backend.
2. **Least privilege per environment.** Staging workloads CANNOT read production secrets and vice versa. Enforce at the secret manager's IAM layer.
3. **Audit log enabled.** Minimum requirement: know which principal fetched which secret at what time. Retain for 18 months (matches audit_log retention policy).
4. **Rotation runbooks exist BEFORE production.** See SECURITY.md §9.2 for JWT; equivalent needed for every row in the secrets table above.

---

## Consequences

### Positive
- Clear migration path from `.env` + mounted file (dev) → staging (G or H) → prod (H over A/B/C) without refactoring the service.
- Rotation becomes a backend-policy change, not a code change.
- Audit and access control centralize outside the repo and the CI system.

### Negative / trade-off
- Cost: every option except G adds a line item to the bill (pennies to dollars per month, depending on secret count).
- Operational complexity: especially Option D. The team must be ready to run Vault before picking D.
- Lock-in: Options A/B/C are cloud-native and hard to move. Option H (External Secrets Operator) with a provider behind it is the least-locked pattern.

### Follow-up work tracked here (not in scope for this ADR)
- **JWT key rotation runbook** — lives in SECURITY.md §9.2; implementation deferred until multi-`kid` JWKS is wired.
- **DB credential rotation automation** — Option D can issue dynamic per-connection creds; worth revisiting at the scale where that pays for itself.
- **Payment / storage secrets** — land when those services do. Same backend pattern applies.

---

## Decision log

- 2026-04-21 — ADR drafted. Waiting on user to confirm deployment target before choosing a primary option.
- 2026-04-21 — Owner confirmed: VM first. Status → Accepted. Step 1 (`.env` on VM) active; Step 2 (SOPS + age) documented as upgrade path. Runbook lives at [`lustia/deploy/vm/README.md`](../../lustia/deploy/vm/README.md).
