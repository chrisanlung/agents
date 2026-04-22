# Security — Lustia Auth Service

_Owned by `security-expert`. Every diff that touches auth, authorization, crypto, DB schema, or secrets must be reviewed against this document before merge. Blocking authority: Critical and High findings block "done"._

_Last updated: 2026-04-18 — Phase 2 initial design (security-expert)_

---

## 1. STRIDE Threat Model (Phase 2 scope: auth-service + DB)

Scope boundary: the trust boundary is at the HTTPS ingress. Everything inside is considered semi-trusted; the DB layer is trusted only after middleware has set tenant context. Super-admin flows are treated as a separate, higher-risk path.

| # | Category | Concrete Threat | Where | Impact | Mitigation | Status |
|---|---|---|---|---|---|---|
| S-1 | Spoofing | Attacker replays a stolen access token to impersonate a legitimate user | `POST /auth/*` all protected endpoints | Account takeover | Short-lived JWT (15 min), `kid`-keyed RS256 signature verified from JWKS, no shared secret across services | Design |
| S-2 | Spoofing | Login request specifies a `tenant_slug` belonging to a different tenant, causing auth to run against the wrong user table slice | `POST /auth/login` | Cross-tenant credential stuffing — attacker may find a user whose email+password is reused in another tenant | Tenant resolved by `tenant_slug` in body; unknown tenant returns generic `INVALID_CREDENTIALS` — same response body, same timing as wrong password. Never confirm whether a tenant slug is valid to an unauthenticated caller | Design |
| S-3 | Spoofing | Forged `kid` in JWT header directing verifier to use attacker-controlled key | JWKS endpoint + JWT middleware | Token forgery → full account takeover | Verifier fetches JWKS only from the configured internal URL; it does not follow a `kid` that is a URL or path. `kid` is matched against the cached JWKS key set; unknown `kid` fails immediately | Design |
| T-1 | Tampering | JWT `claims` altered after issue (e.g. `tenant_id`, `permissions` array) | JWT middleware in all services | Privilege escalation, cross-tenant data access | RS256 asymmetric signature — no downstream service holds the private key; signature verification detects any claim mutation | Design |
| T-2 | Tampering | Refresh token row in DB modified to extend `expires_at` or clear `revoked_at` | `refresh_token` table | Persistent session after intended revocation | `lustia_app` DB role has no UPDATE privilege on `revoked_at` outside the application's controlled write path; all revocation goes through the repository layer | Design |
| T-3 | Tampering | `app.current_tenant` session variable overwritten mid-request to read another tenant's data | PostgreSQL session variable | Cross-tenant data read | `SET LOCAL` scopes the variable to the transaction only; it is reset on commit/rollback. The application must set it once at the start of each transaction and never accept it from user input | Design |
| R-1 | Repudiation | Admin claims they did not create a user or assign a role; no evidence trail | `POST /admin/users`, `PUT /admin/users/:id/roles` | Compliance gap — GDPR audit trail | Every state-changing action writes a row to `audit_log` with `actor_user_id`, `action`, `resource_type`, `resource_id`, `meta` (before/after diff), and `created_at`. Audit log is append-only, no RLS, `lustia_app` cannot DELETE rows | Design |
| R-2 | Repudiation | User denies logging in from an unusual location | `POST /auth/login` → `refresh_token` row | Insufficient evidence for incident response | `refresh_token` stores `ip` (INET) and `user_agent` on issue. Login success is written to `audit_log` with IP, user agent, and timestamp. These fields have a 90-day retention minimum (see Section 9) | Design |
| I-1 | Information Disclosure | `password_hash` leaks in API response, log line, or error message | `"user"` GORM model, any log call in usecase/handler | Offline Argon2id cracking of all user passwords | GORM struct tag `json:"-"` on `PasswordHash`; log redaction rule (see Section 7); unit test asserting serialized User struct contains no `password_hash` key; usecase never returns the full domain `User` entity — returns a `UserDTO` that has no hash field | Design |
| I-2 | Information Disclosure | Password reset response reveals whether an email exists in a tenant | `POST /auth/forgot-password` | User enumeration — attacker maps all valid emails per tenant | Response is always HTTP 204 regardless of whether the email exists. Processing time is equalized by always running Argon2id or a constant-time dummy operation before responding | Design |
| I-3 | Information Disclosure | `refresh_token.ip` and `refresh_token.user_agent` (PII) exposed in an API response or log | `refresh_token` table, any log/response path that includes token rows | GDPR violation — PII exfiltration | These fields are never returned in any API response. They are written on token issue and readable only by the super_admin or in a security incident. Retained for 90 days post-revocation (see Section 9), then purged | Design |
| I-4 | Information Disclosure | JWT private key exposed via misconfiguration (e.g. mounted at a web-accessible path, logged at startup) | `lustia/services/auth/config/`, startup logger | Platform-wide token forgery | Private key loaded from a mounted file (not env var); startup log confirms key was loaded but logs only the `kid` (SHA-256 of public key), never the key material | Design |
| D-1 | Denial of Service | Credential stuffing / brute-force login against a single account | `POST /auth/login` | Account lockout or server CPU exhaustion from Argon2id hashing | Per-IP rate limit: 10 req/min. Per-email rate limit: 5 req/min. After 10 consecutive failed attempts within 15 minutes: set `user.locked_until = now() + 30 minutes`. Argon2id hashing is gated behind a pre-check that short-circuits on locked accounts without hashing | Design |
| D-2 | Denial of Service | Mass password reset requests cause mail provider rate-limit or CPU spike | `POST /auth/forgot-password` | Email provider exhaustion; CPU from token hashing | Per-IP: 3 req/hour. Per-email: 3 req/hour. Rate limit checked before any DB read or token generation | Design |
| D-3 | Denial of Service | Oversized JSON body sent to any auth endpoint exhausts memory | All `auth-service` endpoints | Service crash / OOM | Request body size capped at 1 MB in Gin middleware (see Section 6). Log + reject oversized bodies with HTTP 413 | Design |
| E-1 | Elevation of Privilege | `tenant_admin` assigns `super_admin` role to themselves or another user within their tenant | `PUT /admin/users/:id/roles` | Platform-level takeover | `super_admin` role assignment is blocked for any caller whose JWT `tenant_id` is non-null. The RBAC middleware checks `user.assign_role` permission, but the usecase additionally enforces: if target role is `super_admin`, caller must have `tenant_id IS NULL` in claims. This is a hard rule in the usecase, not just middleware | Design |
| E-2 | Elevation of Privilege | `branch_admin` reads or modifies users outside their assigned branch | `GET /admin/users/:id`, `PUT /admin/users/:id` | Cross-branch data access within a tenant | After RBAC middleware passes, the usecase verifies `target_user.branch_ids ∩ caller.branch_ids != ∅`. This is an explicit object-level check, not a generic filter. `code-reviewer` checklist item: every `/admin/users/:id` handler must have this check | Design |
| E-3 | Elevation of Privilege | Expired or revoked refresh token reused after rotation | `POST /auth/refresh` | Persistent access after logout or password change | Single-use refresh tokens. On use: mark `revoked_at = now()`, insert new token. If a previously-revoked token is presented, treat it as a theft indicator: revoke the entire token family (all tokens sharing the same root `replaced_by` chain) and force re-login | Design |

---

## 2. Authentication Design

### 2.1 Password Hashing

Algorithm: **Argon2id** via `github.com/alexedwards/argon2id`.

Parameters:

| Parameter | Value | Rationale |
|---|---|---|
| Time (`t`) | 3 iterations | Minimum recommended for interactive logins per RFC 9106 |
| Memory (`m`) | 65536 KiB (64 MiB) | Resists GPU/ASIC attacks; acceptable latency on 1-core |
| Parallelism (`p`) | 2 threads | Matches minimum recommended; scales to 2 cores |
| Salt length | 16 bytes | From `crypto/rand`; embedded in the encoded string |
| Key length | 32 bytes | 256-bit output |

The `argon2id.DefaultParams` from the library does not match these values. Explicitly construct:

```go
params := &argon2id.Params{
    Memory:      64 * 1024,
    Iterations:  3,
    Parallelism: 2,
    SaltLength:  16,
    KeyLength:   32,
}
```

**Rehash-on-login policy:** On every successful login, compare the stored hash's parameters against the current configured parameters. If they differ (parameters were tightened during a security upgrade), re-hash the provided plaintext password with the new parameters and update `user.password_hash` atomically within the same transaction. This upgrades hashes transparently without forcing password resets.

### 2.2 Account Lockout

- **Threshold:** 10 consecutive failed login attempts within a 15-minute sliding window.
- **Action:** Set `user.locked_until = now() + 30 minutes`. Do not reset `failed_login_count` on lockout — only on a successful login.
- **Behavior during lockout:** Return the generic `INVALID_CREDENTIALS` error (same body and timing as a wrong password — do not reveal the account is locked to an unauthenticated caller). Internally log the lockout event to `audit_log`.
- **Unlock:** `locked_until` expires automatically (the auth service checks `locked_until > now()` at login). Manual early unlock is an admin-only operation: `DELETE`/clear `locked_until` via `PUT /admin/users/:id/unlock`, requiring `user.update` permission and tenant scope verification.
- **Short-circuit behavior:** If `locked_until > now()`, the auth service returns `INVALID_CREDENTIALS` immediately without computing Argon2id. This prevents the lockout path from being used as a CPU DoS oracle.
- **Implementation note for `go-expert`:** The failed_login_count increment and lockout check must be atomic (single UPDATE with conditional logic or a SELECT FOR UPDATE on the user row) to prevent race conditions under concurrent login attempts.

### 2.3 Password Policy

| Rule | Value |
|---|---|
| Minimum length | 10 characters |
| Maximum length | 128 characters (prevent DoS via Argon2id on gigabyte inputs) |
| Complexity | At least 1 uppercase, 1 lowercase, 1 digit, 1 special character |
| Banned list | Top-1000 common passwords checked at registration and password change (static list embedded; Pwned Passwords API integration deferred to Phase 10) |
| Rotation | No mandatory periodic rotation (NIST SP 800-63B guidance: forced rotation increases weak password reuse) |
| History | No repeated-password check in Phase 2 (deferred; requires storing previous hashes) |

Validation is enforced in the HTTP handler DTO binding layer (`go-expert`: use `go-playground/validator/v10` custom validators for complexity and banned list).

### 2.4 JWT Access Tokens

**Algorithm:** RS256 (RSA + SHA-256). The auth-service holds the RSA private key exclusively. All other services verify signatures using the public key fetched from the JWKS endpoint. No downstream service ever needs — or should ever receive — the private key.

**Key size:** Minimum 3072-bit RSA key (recommended: 4096-bit for new deployments). Generate with:

```bash
openssl genrsa -out auth-private.pem 4096
openssl rsa -in auth-private.pem -pubout -out auth-public.pem
```

The `kid` (Key ID) for JWKS is the SHA-256 thumbprint of the public key in JWK format (per RFC 7638). This ensures `kid` is deterministic and rotatable.

**Claim set:**

| Claim | Type | Value |
|---|---|---|
| `iss` | string | `https://auth.lustia.internal` (configurable) |
| `sub` | string | `user.id` (UUID) |
| `aud` | string | `lustia` (platform audience; configurable) |
| `exp` | NumericDate | `iat + 900` (15 minutes) |
| `iat` | NumericDate | Unix timestamp at issue |
| `jti` | string | `crypto/rand` UUID per token (for future revocation blacklist) |
| `kid` | header | Key ID for JWKS lookup |
| `tenant_id` | string | `user.tenant_id` UUID, or `null` for super_admin |
| `roles` | []string | User's assigned role names |
| `permissions` | []string | Resolved permission codes for this user |
| `branches` | []string | Branch UUIDs assigned to this user (empty for tenant-wide roles) |

**Do not embed:** `password_hash`, `email`, `full_name`, or any PII beyond the minimum needed for routing.

**JWKS endpoint:** `GET /auth/.well-known/jwks.json` — public, unauthenticated, cacheable. Returns the current public key in JWK format.

**Downstream verification protocol:**
- Services fetch JWKS on startup and cache with a 5-minute TTL.
- On TTL expiry or unknown `kid`, re-fetch JWKS before rejecting the token (allows key rotation without downtime).
- Clock-skew allowance: 30 seconds (`golang-jwt/jwt/v5` `WithLeeway(30 * time.Second)`).
- Reject tokens with `alg: none` or any algorithm other than `RS256`. Use `jwt.WithValidMethods([]string{"RS256"})`.

### 2.5 Refresh Tokens

- **Format:** 32 bytes from `crypto/rand`, base64url-encoded (43 characters). Opaque to the client.
- **Storage:** SHA-256 hash of the raw token stored in `refresh_token.token_hash` (hex, exactly 64 chars). The raw token is returned to the client once on issue and never stored in plaintext.
- **Lifetime:** 14-day sliding window (each successful use resets the 14-day window). Absolute cap: 30 days from first issue (tracked via the root token's `issued_at` in the rotation chain). After 30 days, the user must re-authenticate regardless of activity.
- **Single-use rotation:** On `POST /auth/refresh`:
  1. Look up `token_hash` using `subtle.ConstantTimeCompare` (see Section 4).
  2. Verify `expires_at > now()` and `revoked_at IS NULL`.
  3. Atomically: set `revoked_at = now()` on the old row, insert a new row with `replaced_by` pointing from old → new.
  4. Return new raw token to client; client must discard the old one.
- **Theft detection:** If a token that has already been marked `revoked_at IS NOT NULL` is presented, this indicates token replay (possible theft). Action: revoke all tokens in the same chain (walk the `replaced_by` graph) and write a `security.refresh_token_replay` event to `audit_log`. The user must re-authenticate.
- **Revocation triggers:**
  - User-initiated logout (`POST /auth/logout`): revokes the presented token only.
  - Password change (`PUT /auth/me/password`): revokes all refresh tokens for the user.
  - Admin deactivation (`PUT /admin/users/:id` `is_active=false`): revokes all refresh tokens for the user.
  - Account lockout: does not auto-revoke existing tokens (tokens remain valid until their next use, which will fail the `is_active` check). This is acceptable for Phase 2; force-revocation on lockout can be added later.
- **Cleanup job:** A periodic job (flag for `go-expert` and `devops-expert`) should `DELETE FROM refresh_token WHERE expires_at < now() - interval '30 days'` to prevent unbounded table growth.

### 2.6 Password Reset

- **Token format:** 32 bytes from `crypto/rand`, base64url-encoded. SHA-256 hash stored in `password_reset.token_hash`.
- **Expiry:** 60 minutes from issue (`expires_at = now() + interval '1 hour'`).
- **Single-use:** On successful consumption, set `used_at = now()`. Any subsequent use of the same token is rejected (check `used_at IS NULL AND expires_at > now()`).
- **Constant-time lookup:** Do not query by `token_hash` as a literal WHERE clause if the DB can return the hash value in an error path. Use `WHERE token_hash = $1` with a parameterized query; compare the returned hash against the request value with `subtle.ConstantTimeCompare` in application code.
- **Rate limits:** Per-IP: 3 requests/hour. Per-email address: 3 requests/hour. Rate limit is checked before any DB read or token generation.
- **Response:** Always HTTP 204. Never confirm or deny that the email exists in the tenant. Processing time is equalized: if the email does not exist, perform a constant-time dummy comparison before responding (to prevent timing oracle on email existence).
- **No RLS on `password_reset` table — confirmed acceptable:** The lookup is by opaque token hash before tenant context is established. The hash is cryptographically random (32 bytes of entropy); enumeration is computationally infeasible. The additional controls (single-use, 60-minute expiry, per-IP and per-email rate limiting, constant-time comparison) are sufficient. No schema change needed. This is the accepted design.

### 2.7 MFA

MFA is **deferred to Phase 10** (before production customer-facing launch). The schema currently has no MFA columns on `"user"`. When Phase 10 is scoped, add:

- `mfa_enabled BOOLEAN NOT NULL DEFAULT false`
- `mfa_secret TEXT NULL` (encrypted TOTP seed or WebAuthn credential reference)
- `mfa_backup_codes TEXT[] NULL` (hashed backup codes)

Flag for `db-designer` when Phase 10 begins. For Phase 2, the absence of MFA is a documented and accepted risk. Super admin accounts should be given a strong password and the bootstrap email must be a monitored address (see Section 11, production-readiness checklist).

---

## 3. Authorization Design

### 3.1 Model

**RBAC with tenant + branch scoping.** Permissions are fine-grained string codes (`booking.create`, `user.assign_role`, etc.) seeded in the `permission` table. Roles aggregate permissions via `role_permission`. Users hold roles via `user_role`. Branch assignments via `user_branch` further scope branch-admin, finance, and therapist roles.

This is not full ABAC. For Phase 2, the combination of permission string + tenant scoping + branch scoping covers all required access control decisions. Migrate to ABAC or ReBAC only if dynamic attribute-based rules emerge that cannot be expressed as permission codes.

### 3.2 JWT Claims for Authorization

The JWT `permissions` claim is a pre-resolved, flat array of permission code strings. On login, the auth-service:

1. Loads the user's roles from `user_role`.
2. Resolves all permissions for those roles from `role_permission`.
3. Embeds the deduplicated permission list in the JWT `permissions` claim.
4. Embeds `tenant_id` (null for super_admin), `roles`, and `branches` (array of branch UUIDs).

This avoids a DB lookup on every protected request. The trade-off: permissions are stale until the token expires (15 minutes). For Phase 2 this is acceptable. If a permission must be revoked immediately (e.g. role removed from a compromised account), the remediation is to revoke all refresh tokens for that user (forcing re-login and claim re-issue) and wait up to 15 minutes for existing access tokens to expire.

### 3.3 Enforcement Layers (Three Layers — All Must Pass)

**Layer 1 — JWT Middleware** (`lustia/services/auth/internal/adapter/http/middleware/jwt.go`):
- Validates token signature, `exp`, `iss`, `aud`, algorithm.
- Extracts and stores claims in Gin context.
- On failure: HTTP 401, log event (never log the token itself).

**Layer 2 — RBAC Permission Middleware** (`middleware/rbac.go`):
- `RequirePermission("booking.create")` applied per route group.
- Checks `claims.Permissions` contains the required code.
- On failure: HTTP 403, write `authz.denied` event to `audit_log` with endpoint, user ID, required permission.

**Layer 3 — Object-Level Check in Usecase** (every usecase that touches a resource by ID):
- `GET /admin/users/:id` — usecase must verify `target_user.tenant_id == caller.tenant_id` (and, for branch-scoped roles, `target_user.branch_ids ∩ caller.branch_ids != ∅`).
- This check happens in the usecase layer, not the handler and not the middleware. The middleware cannot verify object ownership because it does not load the target resource.
- `code-reviewer` checklist item (mandatory at every PR): every endpoint with a resource ID parameter must have a corresponding object-level tenant/branch check in the usecase. "Passes RBAC middleware" is not sufficient.
- RLS at the DB layer is the final backstop: even if both Layer 2 and Layer 3 have bugs, an incorrectly scoped DB query returns an empty result set rather than leaking data.

### 3.4 Super Admin Authorization

**Decision: `lustia_super_admin` DB role with `BYPASSRLS` is NOT recommended for application traffic. The `__platform__` sentinel value is the accepted approach.**

Reasoning (for `go-expert` — this gates implementation):

The `lustia_super_admin` DB role with `BYPASSRLS` requires the connection pool to either (a) maintain a separate pool of connections authenticated as `lustia_super_admin`, or (b) use `SET ROLE lustia_super_admin` per-request. Option (a) doubles connection pool complexity. Option (b) requires careful cleanup on connection return to the pool — a missed `RESET ROLE` would allow a subsequent regular request to run as the super_admin DB role, which is a critical privilege escalation bug. The blast radius of getting that wrong is the entire DB with no RLS protection.

The `__platform__` sentinel approach uses a single `lustia_app` connection pool. The sentinel is validated in application code (a super_admin JWT has `tenant_id: null`; the middleware sets `app.current_tenant = '__platform__'`). The RLS policies for the relevant tables include the explicit sentinel branch. All connections are the same DB role. If the app forgets to set the variable, the safe-fail behavior (empty result) applies regardless of whether the caller is a super_admin or a regular user.

**The `lustia_migrator` role with `BYPASSRLS` is retained for DDL migrations only** and must never be used for application request handling.

See also: `docs/DECISIONS/0005-super-admin-rls-bypass.md`.

### 3.5 Multi-Tenant Login

**Tenant resolution on login:**

1. Client sends `POST /auth/login` with body: `{ "email": "...", "password": "...", "tenant_slug": "..." }`.
2. Auth-service looks up tenant by `slug` WHERE `status = 'active' AND deleted_at IS NULL`.
3. If tenant not found: return `INVALID_CREDENTIALS` (HTTP 401) — do not reveal the tenant slug was invalid.
4. Looks up user by `(tenant_id, email)` using the tenant found in step 2.
5. If user not found: return `INVALID_CREDENTIALS` — same body, same timing as wrong password (run a dummy Argon2id verify or `subtle.ConstantTimeCompare` against a fixed dummy hash to equalize timing).
6. Verifies password, checks `is_active`, checks `locked_until`.
7. On success: issue tokens, reset `failed_login_count = 0`, update `last_login_at`.

**Super admin login:** Uses a fixed `tenant_slug` value of `__platform__` (or omits `tenant_slug`; the auth-service checks `tenant_id IS NULL` when the slug is absent or `__platform__`). The super admin user row has `tenant_id IS NULL`.

**Timing equalization is mandatory:** steps 3, 4, and 5 must each take the same wall-clock time regardless of which branch was taken. Use `subtle.ConstantTimeCompare` with a dummy value when no real comparison is needed.

### 3.6 IDOR Prevention Checklist (for `code-reviewer`)

Every endpoint with a path parameter that is a resource ID must answer "yes" to all three questions:

1. Does the usecase load the resource and verify `resource.tenant_id == claims.TenantID`?
2. If the endpoint is branch-scoped: does it verify the resource's branch is in `claims.Branches`?
3. If the super_admin is the caller (`claims.TenantID == nil`): is cross-tenant access explicitly intended and logged?

Failure on any question is a High severity finding that blocks merge.

---

## 4. Cryptography Choices

| Concern | Algorithm / Library | Notes |
|---|---|---|
| Password hashing | Argon2id (`github.com/alexedwards/argon2id`) | Parameters: t=3, m=64MiB, p=2, salt=16B, key=32B |
| JWT signing | RS256 — RSA-PKCS1v15 + SHA-256 (`github.com/golang-jwt/jwt/v5`) | 4096-bit key recommended; 3072-bit minimum |
| JWT key ID | SHA-256 JWK thumbprint (RFC 7638) | Deterministic, rotation-safe `kid` |
| Refresh token raw value | 32 bytes from `crypto/rand`, base64url-encoded | Never stored; only the SHA-256 hash is persisted |
| Password reset token | 32 bytes from `crypto/rand`, base64url-encoded | Same as above |
| Token hash storage | `crypto/sha256` → hex encoding | Standard library only |
| Token comparison | `crypto/subtle.ConstantTimeCompare` | Mandatory for all hash/token equality checks |
| TLS | TLS 1.2 minimum, TLS 1.3 preferred | Enforced at ingress (nginx/caddy/load balancer); flag for `devops-expert` |
| HSTS | `max-age=31536000; includeSubDomains` | Add `preload` once stable in production |
| Randomness | `crypto/rand` exclusively | `math/rand` must not be used for any security-relevant value |
| Symmetric encryption (future) | AES-256-GCM or ChaCha20-Poly1305 | Not in Phase 2 scope; state this for future reference |

**`jti` claim:** Each access token gets a unique `jti` generated via `crypto/rand` UUID. Not actively checked against a blacklist in Phase 2 (stateless verification is the goal). When a revocation blacklist is needed (e.g. immediate token invalidation without waiting 15 minutes), the `jti` is the key to blacklist. Flag for `go-expert`: store the blacklist in Redis when that need arises.

**RSA private key generation for local dev:**

```bash
openssl genrsa -out lustia/services/auth/config/jwt-private.pem 4096
# Never commit this file. It is listed in .gitignore.
```

Key is loaded from a file path specified in config (`jwt.private_key_path`). It is never pasted into an environment variable (to avoid shell history and process list exposure).

---

## 5. Secret Storage

### 5.1 Local Development

- `.env` file at `lustia/.env` (gitignored; template at `lustia/.env.example` with placeholder values only).
- JWT private key: mounted as a PEM file at a path specified in `.env` (`JWT_PRIVATE_KEY_PATH`). Not pasted into the env var itself.
- DB credentials: `lustia_app` user password in `.env`. `lustia_migrator` password in `.env` (used only by the migrator container).
- `.env.example` must never contain real secrets — only commented-out variable names and format hints.

### 5.2 Staging and Production

Secret management is deferred. Flag for `devops-expert` with the following recommendation:

- Use a managed secret store: HashiCorp Vault (self-hosted) or AWS Secrets Manager / GCP Secret Manager / Azure Key Vault (cloud-native). Doppler is a viable middle ground for early-stage.
- JWT private key should be stored as a secret and mounted into the container as a file (not an env var) at deploy time.
- DB passwords should be rotated semi-annually or on any suspected compromise.
- Kubernetes deployments: use `ExternalSecrets` operator or `sealed-secrets`. Do not use `kubectl create secret` with passwords in shell history.
- CI/CD: use OIDC federation from GitHub Actions to the cloud provider. No long-lived access keys stored as GitHub secrets.

### 5.3 Never Log the Following

(Hard rules — enforced by code review and the redaction test described in Section 7)

- `password` (plaintext)
- `password_hash`
- JWT access token (full value)
- Refresh token (full value)
- Password reset token (full value)
- `Authorization` header contents
- DB password
- JWT private key (any portion)
- `client_secret` (future OAuth flows)

---

## 6. Web Platform Hardening

### 6.1 Security Headers

Applied to every HTTP response from the auth-service via Gin middleware (flag for `go-expert` to implement as a global middleware):

| Header | Value |
|---|---|
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` (add `preload` once stable) |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=(), interest-cohort=()` |
| `X-Frame-Options` | `DENY` |
| `Cache-Control` | `no-store` on all auth endpoints (`/auth/*`) |

CSP is deferred — this is a pure API service returning JSON; there is no HTML surface in Phase 2. Add a strict CSP if any HTML responses are ever introduced.

### 6.2 CORS

- Explicit allowlist from configuration (`cors.allowed_origins: ["https://app.lustia.com", "https://admin.lustia.com"]`).
- Never `Access-Control-Allow-Origin: *` with credentials.
- For Phase 2 (API-only, no browser client yet): CORS may be `*` without credentials on public endpoints only (`/auth/.well-known/jwks.json`). All other endpoints require explicit origin allowlist.
- Flag for `go-expert`: implement CORS as a Gin middleware that reads the `allowed_origins` list from config and reflects only trusted origins. Use `github.com/gin-contrib/cors`.

### 6.3 CSRF

Bearer-token authentication (`Authorization: Bearer <token>`) is not vulnerable to classical CSRF because browsers do not automatically attach Bearer tokens to cross-origin requests. No CSRF token is needed for Phase 2.

**Exception:** If a cookie-based refresh token flow is introduced in a later phase (e.g. `HttpOnly; Secure; SameSite=Strict` cookie for refresh token), CSRF protection (`SameSite=Strict` + double-submit cookie or signed CSRF token) must be added at that time. Flag this decision point for the phase that introduces cookie-based flows.

### 6.4 Rate Limiting

All limits are per the identifying dimension shown. Limits are enforced in application middleware using a sliding-window counter (Redis-backed for multi-instance deployments; in-memory with `golang.org/x/time/rate` for single-instance Phase 2 local dev). Flag for `go-expert`.

| Endpoint | Per-IP limit | Per-account/email limit | Notes |
|---|---|---|---|
| `POST /auth/login` | 10 req/min | 5 req/min by email | Account lockout at 10 consecutive failures (separate from rate limit) |
| `POST /auth/refresh` | 30 req/min | 20 req/min by user ID | Token theft detection handles abuse beyond rate limiting |
| `POST /auth/logout` | 20 req/min | — | |
| `POST /auth/forgot-password` | 3 req/hour | 3 req/hour by email | Checked before any DB read |
| `POST /auth/reset-password` | 3 req/hour | 3 req/hour by email | |
| `POST /admin/users` | 30 req/min | — | Admin endpoints: baseline app-wide limit |
| All other endpoints | 60 req/min per IP | — | Default baseline |

### 6.5 Request Body Size

Maximum JSON body size: **1 MB** for all auth endpoints. Enforced as the first Gin middleware before any handler is called:

```go
router.Use(func(c *gin.Context) {
    c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
    c.Next()
})
```

Return HTTP 413 on oversized bodies.

---

## 7. Logging Rules

### 7.1 Must Log

Write structured log entries (JSON, via `common-configs` logger) for all of the following. Each entry must include: `timestamp`, `trace_id`, `request_id`, `event_type`, and the fields noted.

| Event | Fields to include |
|---|---|
| Login success | `user_id`, `tenant_id`, IP hash (SHA-256 first 8 chars, not raw IP), `user_agent` hash |
| Login failure | `tenant_id`, email hash (SHA-256 first 8 chars, not raw email), failure reason code, IP hash |
| Account lockout | `user_id`, `tenant_id`, IP hash |
| Logout | `user_id`, `tenant_id` |
| Password change | `user_id`, `tenant_id` |
| Token refresh | `user_id` |
| Token replay detected | `user_id`, `tenant_id`, full event detail to `audit_log` |
| Password reset requested | email hash, IP hash |
| Password reset consumed | `user_id`, `tenant_id` |
| Authz denied | `user_id`, `tenant_id`, endpoint, required permission |
| Admin user create/update/deactivate | `actor_user_id`, `target_user_id`, `tenant_id`, action |
| Admin role assign/revoke | `actor_user_id`, `target_user_id`, role name |
| JWT key rotation (new `kid` active) | `kid`, timestamp |

**Note on IP and email in logs:** Log a short prefix of the SHA-256 hash (not the raw value) to allow correlation within an incident investigation without storing raw PII in structured logs. The raw IP and user agent are stored in `refresh_token` table (see Section 1, I-3 for retention rules).

### 7.2 Must Never Log

The following values must never appear in any log line, error message, HTTP response body, stack trace, or metric label:

- `password` (any plaintext password)
- `password_hash` (Argon2id encoded string)
- JWT access token (full value or payload)
- Refresh token (raw value)
- Password reset token (raw value)
- `Authorization` header value
- DB credentials (`lustia_app` or `lustia_migrator` passwords)
- JWT private key (PEM content or any derivative)
- Full email address in logs (use hashed prefix)
- Full IP address in application logs (raw IP is in `refresh_token` table only)

### 7.3 Redaction Test (for `qa-expert`)

The following test must exist in `lustia/services/auth/internal/` and pass in CI:

**Test name:** `TestNoSensitiveFieldsInLogs`

**Method:** In a login flow integration test, capture all log output (redirect logger to a buffer). Attempt login with a known password. Assert that the log buffer does not contain any of the following substrings: the plaintext password, the word `password_hash`, any substring matching the pattern of an Argon2id encoded string (`$argon2id$`), any substring matching a JWT structure (three base64 segments separated by `.` with 100+ characters).

This test is a regression gate. If a future refactor accidentally starts logging the hash (e.g. via GORM debug mode or a struct marshaled to log), the test fails the CI build.

**Additional test:** `TestUserDTODoesNotIncludePasswordHash` — marshal a `User` domain entity (with a non-empty `PasswordHash` field) to JSON and assert the result contains no `password_hash` key. This validates the `json:"-"` struct tag on the GORM model.

---

## 8. Supply Chain

| Control | Tool | Where | Status |
|---|---|---|---|
| Go version pinned | `go 1.24` directive in `go.mod` | `lustia/services/auth/go.mod` | Required |
| Dependency integrity | `go mod verify` | CI pipeline | Required |
| Vulnerability scan | `govulncheck ./...` | CI — runs on every push to main and on PRs | Flag for `devops-expert` |
| Container image scan | `trivy image` | CI — on Docker build | Flag for `devops-expert` |
| Dependency updates | Dependabot or Renovate | `.github/dependabot.yml` | Flag for `devops-expert` |
| GitHub Actions pins | All `uses:` lines must pin to full commit SHA | `.github/workflows/` | Flag for `devops-expert` |
| Workflow permissions | `permissions: contents: read` as default; escalate per-job only | `.github/workflows/` | Flag for `devops-expert` |
| SBOM | Syft / CycloneDX on release builds | CI release workflow | Deferred to Phase 3 |

`go mod verify` must pass in CI before any build artifact is produced. A failed `go mod verify` (checksum mismatch) is a build-blocking error, not a warning.

---

## 9. Operational and Runtime Security

### 9.1 Secret Rotation Cadence

| Secret | Scheduled rotation | On-compromise rotation |
|---|---|---|
| JWT RS256 private key | Annually | Immediately — see Section 9.2 |
| DB password (`lustia_app`) | Semi-annually | Immediately |
| DB password (`lustia_migrator`) | Semi-annually | Immediately |
| Bootstrap super admin password | On first login (enforced by `must_change_password` — see flag below) | Immediately |

✅ **Resolved 2026-04-21** — migration `000006_user_must_change_password` adds the column (NOT NULL, DEFAULT false) and flips the bootstrap super admin to `true`. The auth-service `middleware.PasswordChangeRequired` blocks every protected endpoint except `GET /auth/me`, `POST /auth/me/password`, and `POST /auth/logout`, returning HTTP 403 `PASSWORD_CHANGE_REQUIRED`. `UserRepository.UpdatePassword` clears the flag atomically with the new hash. Admin-created users (`service.UserService.CreateUser`) are seeded with `must_change_password = true`. Regression tests live in `internal/service/security_test.go`.

### 9.2 JWT Key Rotation Incident Response

If the JWT private key is suspected compromised:

1. Generate a new RSA key pair. Give it a new `kid`.
2. Deploy the new auth-service with both keys in the JWKS — the old public key and the new public key. The service begins signing with the new private key immediately.
3. Old access tokens (signed with the old `kid`) remain verifiable from JWKS for up to 15 minutes (their maximum remaining lifetime). After 15 minutes all old tokens have expired.
4. Remove the old public key from JWKS after 20 minutes (15-min token lifetime + 30-sec clock skew allowance + operational margin).
5. Refresh tokens are opaque and not JWT-signed. They are stored as hashes in the DB. A key compromise does not directly affect refresh tokens, but if the attacker used a forged JWT to issue refresh tokens, those must be identified and revoked. This requires reviewing `audit_log` for `auth.login_success` events issued during the compromise window and revoking all refresh tokens for those users.
6. Write a `security.jwt_key_rotated` event to `audit_log` with the old and new `kid`.

### 9.3 Audit Log Retention

| Phase | Retention | Storage |
|---|---|---|
| Hot (queryable) | 18 months | Primary PostgreSQL DB |
| Archive | 18 months–5 years | Cold storage (S3/GCS/Blob with lifecycle policy) |
| Deletion | After 5 years | Per applicable data retention law (adjust for GDPR/PDPA if Indonesian law requires shorter) |

Flag for `devops-expert`: implement `audit_log` table partitioning by `created_at` (monthly range partitions) before production launch. Without partitioning, purging old rows requires a full table scan. See `DATA_MODEL.md` open question 5.

### 9.4 Production Readiness Checklist (pre-staging-deploy)

The following items must be completed before any staging or production deployment. They are not Phase 2 code tasks — they are operational gates:

- [ ] Bootstrap super admin email (currently `admin@lustia.local` per migration 000007 — local-dev placeholder, works with Mailpit) replaced with a real, monitored mailbox address via `UPDATE "user" SET email = '...' WHERE id = 'a0000000-0000-0000-0000-000000000001'` before any staging deploy. *(Migration 000007 replaced the earlier unroutable `superadmin@lustia.internal` placeholder on 2026-04-21.)*
- [x] `must_change_password` column added to `"user"` (migration 000006); bootstrap user seeded with `must_change_password = true`; auth-service enforces password change on first login via `middleware.PasswordChangeRequired`. ✅ 2026-04-21.
- [ ] JWT private key generated with at least 3072-bit RSA, stored in a secret manager (not `.env`).
- [ ] TLS 1.2+ enforced at ingress; HSTS header enabled.
- [ ] Rate limiting backed by Redis (not in-memory) for multi-instance deployments.
- [ ] `govulncheck` and `trivy` passing with no Critical/High findings.
- [ ] `audit_log` table partitioned by month (or archival policy defined).
- [ ] Super admin bootstrap email changed and password set via the post-migration runbook (see ADR 0003).
- [ ] CORS `allowed_origins` list populated with real production origins (not `*`).
- [ ] Log pipeline configured to redact (or never receive) the fields listed in Section 7.2.
- [ ] Refresh token cleanup job scheduled.

---

## 10. Review Log

_Each security review of a diff or PR appends an entry here. Blocking findings (Critical, High) must be resolved before the orchestrator marks the work complete._

| Date | Change reviewed | Findings | Status |
| --- | --- | --- | --- |
| 2026-04-21 | `must_change_password` enforcement shipped | **[Critical — Resolved]** Migration 000006, `middleware.PasswordChangeRequired`, service wiring (Login / Refresh / CreateUser), atomic clear in `UpdatePassword`, regression tests in `security_test.go`. | resolved |
| 2026-04-18 | Phase 1 schema + Phase 2 auth-service design (initial security design review) | **[Critical — Resolved in follow-up 2026-04-21]** `must_change_password` enforcement was missing; see row above for the fix. **[High]** No object-level tenant/branch check specification existed prior to this document: `/admin/users/:id` and similar resource-by-ID endpoints had no explicit requirement to verify the target resource belongs to the caller's tenant. Without this check, any `tenant_admin` with `user.read` permission could read any user on the platform by guessing UUIDs (IDOR / CWE-639). Fix: Section 3.3 Layer 3 and Section 3.6 are now the explicit requirement. `code-reviewer` must treat absence of this check as a blocking finding. **[High]** `refresh_token.ip` and `user_agent` PII retention was undefined: no retention or access rule existed. These fields are accessible to anyone with DB read access. Fix: Section 1 (I-3) and Section 9.3 now mandate 90-day post-revocation retention, no API exposure, and access restricted to security incident review. **[Medium]** Bootstrap super admin email is non-routable (`superadmin@lustia.internal`): in production this address receives no mail, so password reset emails and security alerts go nowhere. Fix: production readiness checklist item added (Section 9.4). **[Medium]** JWT lifetime and revocation gap: a token issued to a user whose account is subsequently deactivated or whose role is removed remains valid for up to 15 minutes. This is an accepted design trade-off (stateless JWT) but must be documented. Fix: documented in Section 3.2; immediate remediation path (revoke all refresh tokens) documented in Section 2.5. **[Low]** No explicit request body size cap was specified in prior docs: without a cap, an attacker can send a multi-gigabyte body to the Argon2id password verification endpoint and exhaust memory. Fix: 1 MB cap specified in Section 6.5. **[Info]** MFA absence is a documented and accepted risk for Phase 2; super admin accounts are the highest-value target. Noted in Section 2.7. | in-review |

---

## 11. Open Questions for the Team

The following questions require a decision from the user or the relevant agent before the listed work can be completed. They are not blockers for Phase 2 code unless noted.

1. **MFA timing** — Phase 2 (optional enrollment now, while the auth-service is being built) or Phase 10 (before production launch)? Adding MFA to the auth-service is significantly cheaper now than retrofitting it in Phase 10. The DB schema has no MFA columns yet; adding them now is a one-migration change. Recommendation: add the columns as nullable in a Phase 2 migration, implement TOTP enrollment as optional, gate on a feature flag. Blocks: `db-designer` (schema), `go-expert` (TOTP flow). Needs decision from user.

2. **Password reset token distribution channel** — email only, or SMS/WhatsApp as well? If multi-channel, what is the sending service (SendGrid, Mailgun, Twilio, WhatsApp Business API)? This gates the `forgot-password` endpoint implementation. Flag for `go-expert`. Needs decision from user.

3. **JWKS rotation schedule and ownership** — who is responsible for generating a new RSA key pair and deploying it? Platform team (a single shared key for all tenants and all services) or per-tenant keys? Recommendation: single platform key, owned by the platform team, rotated annually with the incident procedure defined in Section 9.2. Per-tenant keys add significant operational complexity without a commensurate security benefit for Phase 2. Needs confirmation from user.

4. **Customer-facing login (Phase 5) — shared JWT format or separate issuer?** If customers (Phase 5) use the same auth-service and JWT format as staff, the `roles` and `permissions` claims need to express customer-scope permissions. If a separate issuer is used, downstream services must verify two distinct JWKS endpoints and `iss` values. Recommendation: same auth-service, same JWT format, same JWKS endpoint. Add a `user_type: staff | customer` claim to distinguish at the application layer. Needs decision from user before Phase 5 design begins.

5. **`audit_log` partitioning schedule** — monthly range partitions are recommended (see Section 9.3 and `DATA_MODEL.md` open question 5). Should this be included in Phase 2 migrations or deferred to a pre-production migration? Recommendation: add partitioning in a Phase 2 migration now while the table is empty. The cost of retrofitting range partitioning on a populated table is high. Needs decision from `db-designer` and user.

6. **Refresh token cleanup job ownership** — scheduled DELETE of expired/revoked tokens. Should this be a cron job in the auth-service itself (using a goroutine on a ticker), a separate Kubernetes CronJob, or a pg_cron job in the DB? Flag for `go-expert` and `devops-expert`. Needs decision before production.

7. **Rate limiting backend for multi-instance deployments** — in-memory rate limiting (acceptable for Phase 2 single-instance Docker Compose) does not work correctly when multiple auth-service instances are running. A Redis-backed rate limiter (e.g. `go-redis` + sliding-window Lua script) is required for production. Flag for `devops-expert`. Needs confirmation before any horizontal scaling.

---

_End of SECURITY.md — Phase 2 baseline._
