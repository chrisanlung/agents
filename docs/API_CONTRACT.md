# API Contract — Lustia Auth Service

_Owned by `go-expert`. Consumed by `nextjs-expert` and `flutter-expert`. Changes require frontend sign-off._

_Last updated: 2026-04-25 (ADR 0012 — Room (Ruangan) catalog)_

---

## 1. Conventions

| Item | Value |
|---|---|
| Base URL | `/api/v1` |
| Auth header | `Authorization: Bearer <access_token>` |
| Content-Type | `application/json` (request + response) |
| Timestamps | RFC 3339 UTC — `2026-04-18T12:00:00Z` |
| Request ID | `X-Request-ID` header — echoed back in response; generated if absent |
| Pagination | Offset-based (ADR 0013): `?page=<int>&limit=<int>` → `{"data": [...], "page": 1, "limit": 10, "total_count": 47, "total_pages": 5}`. Page is 1-indexed. Missing page defaults to 1. |
| Versioning | URL-based `/api/v1`; breaking changes bump to `/api/v2` |

### 1.1 Error Envelope

Every error response uses this shape:

```json
{
  "error": {
    "code": "VALIDATION",
    "message": "human-readable description",
    "details": {}
  }
}
```

`details` is omitted when empty.

---

## 2. JWT Format

Access tokens are RS256-signed JWTs. The `kid` header identifies the public key in the JWKS document.

### Header

```json
{ "alg": "RS256", "typ": "JWT", "kid": "primary" }
```

### Payload Claims

| Claim | Type | Description |
|---|---|---|
| `iss` | string | Issuer — always `lustia-auth` |
| `sub` | string | UserID (UUID) |
| `aud` | string[] | Always `["lustia"]` |
| `exp` | number | Unix timestamp — 15 minutes from issue |
| `iat` | number | Unix timestamp of issue |
| `jti` | string | Unique token ID (UUID v4) for audit |
| `tenant_id` | string | TenantID (UUID) or `"__platform__"` for super_admin |
| `roles` | string[] | Role names the user holds, e.g. `["tenant_admin"]` |
| `permissions` | string[] | Deduplicated permission codes, e.g. `["user.create","user.read"]` |
| `branches` | string[] | BranchID UUIDs the user is assigned to |
| `email` | string | User email (convenience; not authoritative) |
| `full_name` | string | User display name (convenience) |

### Consuming This Token in Other Services

1. Fetch JWKS from `GET /api/v1/auth/.well-known/jwks.json`. Cache the document with a TTL of 5 minutes.
2. On cache miss or unknown `kid`, re-fetch immediately.
3. Validate: `alg=RS256`, `iss=lustia-auth`, `aud` contains `lustia`, `exp > now`.
4. Extract `tenant_id`, `roles`, `permissions`, `branches` for authorization decisions.
5. Never call auth-service on every request — verify locally with the cached public key.

**Recommended middleware shape (Go):**

```go
// Middleware reads Authorization header, verifies RS256 signature with cached
// public key, and injects domain.AccessClaims into the request context.
func JWTVerify(jwksURL string) func(http.Handler) http.Handler { ... }
```

---

## 3. Multi-Tenant Login Resolution

The `POST /auth/login` endpoint accepts either an **email address** or a **username** in the `identifier` field. The service auto-detects which lookup to perform:

| `identifier` value | Lookup |
|---|---|
| Contains `@` | Treated as email → `FindByEmail` |
| No `@` | Treated as username → `FindByUsername` (case-insensitive) |

**Backward compatibility:** Old clients that send `"email"` instead of `"identifier"` continue to work. When `identifier` is absent, the server falls back to the `email` field. Both fields should not be sent together; if both are present `identifier` wins.

**Anti-enumeration:** Whether a lookup fails because the identifier does not exist or because the password is wrong, the server always returns `401 INVALID_CREDENTIALS`. The response never distinguishes between "email not found" and "username not found".

---

## 3a. Password-change gate

When the authenticated caller's JWT claim `must_change_password = true`, every protected endpoint except the three below returns **HTTP 403 `PASSWORD_CHANGE_REQUIRED`**:

- `GET /api/v1/auth/me` — view own profile
- `POST /api/v1/auth/me/password` — rotate password
- `POST /api/v1/auth/logout` — revoke current refresh token

The flag is set by:

- Migration `000006` on the bootstrap super admin row.
- `POST /api/v1/admin/users` on every admin-created user.
- Any future flow that elevates privileges and wants to force a rotation.

The flag is cleared atomically when `POST /api/v1/auth/me/password` succeeds (same UPDATE that writes the new `password_hash`). Callers still holding a stale access token must call `POST /api/v1/auth/refresh` to receive a fresh token with `must_change_password = false`; the refresh endpoint is public and therefore never blocked by this gate.

Downstream services verifying the JWT locally MUST honour the same rule: if they see `must_change_password = true` in the claim, they reject the request with `PASSWORD_CHANGE_REQUIRED` and point the client at the auth-service's rotation endpoint.

---

## 4. Endpoints

### 4.1 Public — no auth required

---

#### `POST /api/v1/auth/login`

Authenticates a user and returns a short-lived access token + long-lived refresh token.

**Rate limited:** 20 req/min per IP (in-memory; Redis in Phase 10).

**Request**

```json
{
  "identifier": "alice",
  "password": "s3cur3P@ssw0rd"
}
```

or (email form):

```json
{
  "identifier": "alice@example.com",
  "password": "s3cur3P@ssw0rd"
}
```

or (deprecated backward-compat form — old clients only):

```json
{
  "email": "alice@example.com",
  "password": "s3cur3P@ssw0rd"
}
```

| Field | Type | Validation |
|---|---|---|
| `identifier` | string | required (unless `email` present); min 3, max 320. Email or username. |
| `email` | string | **Deprecated** — use `identifier`. Accepted for backward compat; omitempty, valid email, max 320. |
| `password` | string | required, min 8, max 128 |

**Response `200 OK`**

```json
{
  "access_token": "<JWT>",
  "refresh_token": "<opaque>",
  "token_type": "Bearer",
  "expires_at": "2026-04-18T12:15:00Z",
  "user": {
    "id": "uuid",
    "email": "alice@example.com",
    "username": "alice",
    "full_name": "Alice Smith",
    "phone": "+62812...",
    "avatar_url": "https://...",
    "is_active": true,
    "is_super_admin": false,
    "must_change_password": false
  },
  "scope": "tenant",
  "memberships": [{ "membership_id": "...", "tenant_id": "...", "tenant_name": "Acme Spa", "tenant_slug": "acme-spa", "roles": ["tenant_admin"], "branches": ["branch-uuid-1"], "status": "active" }],
  "active_membership_id": "..."
}
```

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Malformed request body or both `identifier` and `email` absent |
| 401 | `INVALID_CREDENTIALS` | Email/username not found **or** wrong password (intentionally vague — anti-enumeration) |
| 401 | `ACCOUNT_LOCKED` | Too many failed attempts |
| 401 | `ACCOUNT_INACTIVE` | User deactivated or no active memberships |
| 429 | `RATE_LIMITED` | Too many login attempts from this IP |

---

#### `POST /api/v1/auth/refresh`

Rotates the refresh token: old token is revoked, new pair issued.

**Request**

```json
{ "refresh_token": "<opaque>" }
```

**Response `200 OK`** — same shape as login response without `user`.

**Errors**

| Status | Code | When |
|---|---|---|
| 401 | `TOKEN_INVALID` | Token not found |
| 401 | `REFRESH_REVOKED` | Token was revoked (logout or rotation) |
| 401 | `TOKEN_EXPIRED` | Token past its 14-day window |
| 401 | `ACCOUNT_INACTIVE` | User was deactivated since token issue |

---

#### `POST /api/v1/auth/password/forgot`

Initiates a password reset. Always returns `204` regardless of whether the email exists (anti-enumeration).

**Request**

```json
{
  "email": "alice@example.com",
  "tenant_slug": "acme-spa"
}
```

**Response `204 No Content`** — always.

_Email delivery (updated 2026-04-21): the reset email is now dispatched by the auth-service itself via SMTP. In the local compose stack, the SMTP target is the **Mailpit** container — open `http://localhost:8025` to view the captured email and verify the `?token=` link. Set `SMTP_ENABLED=false` to fall back to token-only mode. A dedicated notification-service still lands in Phase 3 for broadcast / batch / multi-channel flows (email + WhatsApp + SMS), at which point the SMTP path here becomes the fallback._

---

#### `POST /api/v1/auth/password/reset`

Consumes a reset token and sets a new password. Revokes all existing refresh tokens.

**Request**

```json
{
  "token": "<opaque reset token>",
  "new_password": "N3wP@ssw0rd!"
}
```

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 422 | `TOKEN_INVALID` | Token not found, expired, or already used (vague — anti-enumeration) |
| 400 | `VALIDATION` | Password too short/long |

---

#### `GET /api/v1/auth/.well-known/jwks.json`

Returns the JSON Web Key Set document for RS256 signature verification.

**Response `200 OK`**

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "alg": "RS256",
      "kid": "primary",
      "n": "<base64url-encoded modulus>",
      "e": "<base64url-encoded exponent>"
    }
  ]
}
```

Cache with TTL 5 minutes. Re-fetch on unknown `kid`.

---

#### `GET /healthz`

Liveness probe. Returns 200 if the process is running.

**Response `200 OK`** — `{"status": "ok"}`

---

#### `GET /readyz`

Readiness probe. Returns 200 only if the database connection is reachable.

**Response `200 OK`** — `{"status": "ok"}`
**Response `503 Service Unavailable`** — `{"status": "db_unreachable", "error": "..."}`

---

### 4.2 Authenticated — self

All endpoints in this group require `Authorization: Bearer <access_token>`.

---

#### `POST /api/v1/auth/logout`

Revokes the provided refresh token. The access token is short-lived (15 min) and not blocklisted in Phase 2.

**Request** (body optional)

```json
{ "refresh_token": "<opaque>" }
```

**Response `204 No Content`**

---

#### `GET /api/v1/auth/me`

Returns the authenticated user's full profile, roles, branches, and tenant info.

**Response `200 OK`**

```json
{
  "user": {
    "id": "uuid",
    "tenant_id": "uuid",
    "email": "alice@example.com",
    "full_name": "Alice Smith",
    "phone": "+62...",
    "avatar_url": "https://...",
    "is_active": true,
    "roles": ["tenant_admin"],
    "branches": ["branch-uuid-1"]
  },
  "tenant": {
    "id": "uuid",
    "name": "Acme Spa",
    "slug": "acme-spa",
    "status": "active"
  }
}
```

`tenant` is omitted for super_admin users.

---

#### `PATCH /api/v1/auth/me`

Updates `full_name`, `phone`, or `avatar_url` on the authenticated user. Partial update — only provided fields are changed.

**Request**

```json
{
  "full_name": "Alice Brown",
  "phone": "+6281234567890",
  "avatar_url": "https://storage.example/avatar.jpg"
}
```

| Field | Validation |
|---|---|
| `full_name` | optional, min 1, max 200 |
| `phone` | optional, max 30 |
| `avatar_url` | optional, valid URL, max 2048 |

**Response `200 OK`** — updated `UserProfileResponse`

---

#### `POST /api/v1/auth/me/password`

Changes the authenticated user's password. Revokes all other refresh tokens (forces re-login on all other sessions).

**Request**

```json
{
  "old_password": "OldP@ss",
  "new_password": "NewP@ss123"
}
```

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 401 | `INVALID_CREDENTIALS` | Old password incorrect |

---

### 4.3 Authenticated — admin

All endpoints require `Authorization: Bearer <access_token>` plus the indicated permission in the token's `permissions` claim.

---

#### `POST /api/v1/admin/users`

Creates a new user within the authenticated admin's tenant. The initial password is generated server-side and returned **once** in the response — store it securely; it cannot be retrieved again.

**Required permission:** `user.create`

**Request**

```json
{
  "email": "bob@example.com",
  "username": "bob.jones",
  "full_name": "Bob Jones",
  "phone": "+62...",
  "role_ids": ["role-uuid-1"],
  "branch_ids": ["branch-uuid-1"]
}
```

| Field | Validation |
|---|---|
| `email` | required, valid email, max 320 |
| `username` | optional; 3–50 chars, lowercase `[a-z0-9._]` only; auto-lowercased |
| `full_name` | required, min 1, max 200 |
| `phone` | optional, max 30 |
| `role_ids` | optional, array of valid UUIDs |
| `branch_ids` | optional, array of valid UUIDs |

**Response `201 Created`**

```json
{
  "user": { "id": "...", "email": "...", "username": "bob.jones", ... },
  "initial_password": "generated-once-password",
  "created_user": true,
  "created_membership": true
}
```

**Errors**

| Status | Code | When |
|---|---|---|
| 409 | `DUPLICATE_EMAIL` | Email already exists |
| 409 | `USERNAME_ALREADY_TAKEN` | Username already taken by another user |
| 400 | `USERNAME_INVALID` | Username fails format rules |
| 404 | `NOT_FOUND` | A provided role_id or branch_id does not exist |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `user.create` |

---

#### `GET /api/v1/admin/users`

Lists users in the authenticated admin's tenant with optional filters and offset pagination.

**Required permission:** `user.read`

**Query parameters**

| Param | Type | Description |
|---|---|---|
| `role_id` | UUID | Filter by role assignment |
| `branch_id` | UUID | Filter by branch assignment |
| `is_active` | bool | Filter by active status |
| `page` | int | 1-indexed page number, default 1 |
| `limit` | int | 1–200, default 10 |

**Response `200 OK`**

```json
{
  "data": [ { ...UserProfileResponse }, ... ],
  "page": 1,
  "limit": 10,
  "total_count": 47,
  "total_pages": 5
}
```

---

#### `GET /api/v1/admin/users/:id`

Returns a single user within the authenticated admin's tenant.

**Required permission:** `user.read`

**Response `200 OK`** — `UserProfileResponse`

**Errors**

| Status | Code | When |
|---|---|---|
| 404 | `NOT_FOUND` | User not found or belongs to different tenant |

---

#### `PATCH /api/v1/admin/users/:id`

Updates a user's profile, active status, roles, and/or branch assignments. All changes are applied atomically. Use `null` in `role_ids` / `branch_ids` to leave assignments unchanged; use `[]` to clear all assignments.

**Required permission:** `user.update`

**Request**

```json
{
  "full_name": "Updated Name",
  "username": "alice.new",
  "phone": "+62...",
  "avatar_url": "https://...",
  "is_active": false,
  "role_ids": ["role-uuid-1"],
  "branch_ids": []
}
```

`username` uses three-valued semantics:
- **Key absent from JSON** — no change to existing username.
- **`"username": null`** — clears the username (sets to NULL).
- **`"username": "alice.new"`** — replaces the username (auto-lowercased, validated).

**Response `200 OK`** — updated `UserProfileResponse` (includes `username` field)

**Errors**

| Status | Code | When |
|---|---|---|
| 409 | `USERNAME_ALREADY_TAKEN` | New username already in use |
| 400 | `USERNAME_INVALID` | New username fails format rules |

---

#### `POST /api/v1/admin/users/:id/unlock`

Resets `failed_login_count` to 0 and clears `locked_until`.

**Required permission:** `user.update`

**Response `204 No Content`**

---

#### `GET /api/v1/admin/roles`

Returns all platform roles with their permission codes. Read-only in Phase 2.

**Required permission:** `role.read`

**Response `200 OK`**

```json
{
  "data": [
    {
      "id": "uuid",
      "name": "tenant_admin",
      "description": "Manages a single tenant",
      "permissions": [
        { "id": "uuid", "code": "user.create", "description": "Create users" },
        ...
      ]
    },
    ...
  ]
}
```

---

## 5. Error Code Catalog

| Code | HTTP Status | Meaning |
|---|---|---|
| `VALIDATION` | 400 | Request failed schema/binding validation |
| `UNAUTHENTICATED` | 401 | Missing or malformed Authorization header |
| `INVALID_CREDENTIALS` | 401 | Email or password incorrect (also used when user/tenant not found — anti-enumeration) |
| `TOKEN_EXPIRED` | 401 | Access or refresh token past its expiry |
| `TOKEN_INVALID` | 401 | Token is malformed, unknown, or has wrong signing key |
| `REFRESH_REVOKED` | 401 | Refresh token was explicitly revoked |
| `ACCOUNT_LOCKED` | 401 | Account temporarily locked due to failed attempts |
| `ACCOUNT_INACTIVE` | 401 | User `is_active = false` |
| `PASSWORD_CHANGE_REQUIRED` | 403 | Caller's JWT claim `must_change_password = true`; only `GET /auth/me`, `POST /auth/me/password`, and `POST /auth/logout` are permitted until the password is rotated. See §4.3. |
| `FORBIDDEN` | 403 | Authenticated but action not allowed (generic) |
| `INSUFFICIENT_PERMISSION` | 403 | Missing required permission code |
| `TENANT_INACTIVE` | 401 | Tenant status is not `active` |
| `TENANT_NOT_FOUND` | 401 | Tenant slug does not resolve (401 to prevent enumeration) |
| `NOT_FOUND` | 404 | Resource does not exist or is not visible to caller |
| `CONFLICT` | 409 | Generic unique-constraint or exclusion-constraint violation |
| `DUPLICATE_EMAIL` | 409 | Global `email` unique index violated |
| `USERNAME_ALREADY_TAKEN` | 409 | Global `LOWER(username)` unique partial index violated |
| `USERNAME_INVALID` | 400 | Username fails format rules (3–50 chars, `[a-z0-9._]` only) |
| `RATE_LIMITED` | 429 | Request throttled |
| `INTERNAL` | 500 | Unexpected server error |
| `DUPLICATE_PENDING_REGISTRATION` | 409 | Same email or slug already has a pending registration |
| `TENANT_SLUG_TAKEN` | 409 | Requested slug belongs to an already-approved tenant |
| `REGISTRATION_NOT_PENDING` | 409 | Approve/reject attempted on a non-pending registration |
| `INVALID_STATUS_TRANSITION` | 409 | Requested status change not permitted by the state machine |
| `BRANCH_LIMIT_REACHED` | 409 | Creating a branch would exceed the tenant's `max_branches` limit |

---

## 6. Deferred / Out of Scope (Phase 2)

| Item | Deferred to |
|---|---|
| Host-header tenant resolution | Phase 3 + ADR |
| Access token jti blocklist (logout revokes access token) | Phase 3 (Redis required) |
| Email delivery for password reset | ✅ Shipped 2026-04-21 via SMTP (Mailpit in dev). Multi-channel notification-service (WhatsApp/SMS/batch) still in Phase 3. |
| Public customer signup | Phase 5 |
| Tenant onboarding / approval API | Phase 3 |
| Role creation / permission editing endpoints | Post-Phase 2 |
| `must_change_password` enforcement | ✅ Shipped 2026-04-21 (migration 000006 + middleware + service + tests). No longer deferred. |
| MFA / OAuth / social login | Phase 7+ |

---

---

## 8. Public Registration (Phase 3)

### `POST /api/v1/register/company` — public, no auth

**Rate limited:** 3/hour per IP and per email (in-memory; Redis in Phase 10).

**Request**

```json
{
  "company_name": "Acme Wellness",
  "requested_slug": "acme-wellness",
  "package": "starter",
  "contact_name": "Alice Founder",
  "contact_email": "alice@acme-wellness.example",
  "contact_phone": "+628123456789"
}
```

| Field | Validation |
|---|---|
| `company_name` | required, 2–200 chars |
| `requested_slug` | required, 2–100 chars, must match `^[a-z0-9][a-z0-9-]*[a-z0-9]$` |
| `package` | optional, one of `starter \| growth \| enterprise`, defaults to `starter` |
| `contact_name` | required, 1–200 chars |
| `contact_email` | required, valid email, max 320 |
| `contact_phone` | optional, 5–30 chars |

**Response `201 Created`**

```json
{ "registration_id": "<uuid>", "status": "pending" }
```

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Shape/format failure |
| 409 | `DUPLICATE_PENDING_REGISTRATION` | Same email or slug already has a pending registration |
| 409 | `TENANT_SLUG_TAKEN` | Slug already belongs to an approved tenant |
| 429 | `RATE_LIMITED` | 3/hour per IP exceeded |

---

## 9. Admin: Tenant Approval (Phase 3)

All endpoints require `Authorization: Bearer <access_token>` with `scope=platform` (super admin).

### `GET /api/v1/admin/tenant-registrations`

**Required permission:** `tenant.approve`

**Query parameters:** `status=pending|approved|rejected|all` (default `pending`), `page` (default 1), `limit` (1–200, default 10).

**Response `200 OK`**

```json
{
  "data": [
    {
      "id": "<uuid>",
      "company_name": "Acme Wellness",
      "requested_slug": "acme-wellness",
      "package": "starter",
      "contact_name": "Alice Founder",
      "contact_email": "alice@acme-wellness.example",
      "contact_phone": "+628123456789",
      "status": "pending",
      "approved_at": null,
      "approved_by": null,
      "rejected_at": null,
      "rejection_reason": null,
      "created_at": "2026-04-22T10:00:00Z"
    }
  ],
  "page": 1,
  "limit": 10,
  "total_count": 12,
  "total_pages": 2
}
```

### `POST /api/v1/admin/tenant-registrations/:id/approve`

**Required permission:** `tenant.approve`

Side effects (single transaction): creates tenant, user, membership, assigns `tenant_admin` role, updates registration row, fires welcome email.

**Request** (all optional overrides):

```json
{ "package": "growth", "max_branches": 10 }
```

**Response `200 OK`**

```json
{
  "tenant": { "id": "...", "name": "...", "slug": "...", "status": "active", "package": "growth", "max_branches": 10, "contact_email": "...", "contact_name": "...", "approved_at": "...", "created_at": "..." },
  "tenant_admin": {
    "user_id": "<uuid>",
    "email": "alice@acme-wellness.example",
    "temporary_password": "<random 16 chars>"
  },
  "registration": { ...updated registration object... }
}
```

`temporary_password` is **always present** in this response and is also emailed to the contact. It will not appear in any subsequent API call.

**Errors:** `409 TENANT_SLUG_TAKEN`, `409 REGISTRATION_NOT_PENDING`, `404 NOT_FOUND`.

### `POST /api/v1/admin/tenant-registrations/:id/reject`

**Required permission:** `tenant.approve`

**Request:**

```json
{ "reason": "Duplicate application." }
```

**Response `200 OK`** — updated registration object (same shape as list item above).

### `GET /api/v1/admin/tenants`

**Required permission:** `tenant.read`

**Query parameters:** `status=pending_approval|active|suspended|deactivated|all`, `page` (default 1), `limit` (1–200, default 10).

**Response `200 OK`**

```json
{
  "data": [
    {
      "id": "...", "name": "...", "slug": "...", "status": "active",
      "package": "starter", "max_branches": 1,
      "contact_email": "...", "contact_name": "...",
      "approved_at": "...", "approved_by": "...",
      "rejected_at": null, "rejection_reason": null,
      "created_at": "...",
      "membership_count": 3,
      "branch_count": 1
    }
  ],
  "page": 1,
  "limit": 10,
  "total_count": 24,
  "total_pages": 3
}
```

### `GET /api/v1/admin/tenants/:id`

**Required permission:** `tenant.read`. Returns single tenant (same shape as list item, counts may be 0 in this path).

### `PATCH /api/v1/admin/tenants/:id/status`

**Required permission:** `tenant.update`

**Request:**

```json
{ "status": "suspended", "reason": "payment overdue" }
```

**Response `200 OK`** — updated tenant summary.

**Errors:** `409 INVALID_STATUS_TRANSITION`, `404 NOT_FOUND`.

**State machine:** `pending_approval → active | deactivated`, `active → suspended | deactivated`, `suspended → active | deactivated`. All other transitions return `409 INVALID_STATUS_TRANSITION`.

On transition to `deactivated`: all active memberships are suspended and all refresh tokens scoped to this tenant are revoked atomically.

---

## 10. Tenant: Branch Management (Phase 3)

All endpoints require `Authorization: Bearer <access_token>` with `scope=tenant` and the listed permission.

### `POST /api/v1/tenant/branches`

**Required permission:** `branch.create`

**Request:**

```json
{
  "name": "Main Branch",
  "code": "MAIN",
  "address_line1": "Jl. Sudirman No. 1",
  "city": "Jakarta",
  "province": "DKI Jakarta",
  "postal_code": "10270",
  "country": "ID",
  "timezone": "Asia/Jakarta",
  "contact_phone": "+62215551234",
  "contact_email": "main@acme-wellness.example"
}
```

**Response `201 Created`** — BranchResponse (see below).

**Errors:** `409 BRANCH_LIMIT_REACHED` when `COUNT(non-deleted branches) >= tenant.max_branches` (unless `max_branches = 999`).

### `GET /api/v1/tenant/branches`

**Required permission:** `branch.read`

**Query:** `status=active|inactive|all`, `page` (default 1), `limit` (1–200, default 10).

**Response `200 OK`** — `{ "data": [...BranchResponse], "page": 1, "limit": 10, "total_count": 3, "total_pages": 1 }`.

### `GET /api/v1/tenant/branches/:id`

**Required permission:** `branch.read`. Returns a single BranchResponse.

### `PATCH /api/v1/tenant/branches/:id`

**Required permission:** `branch.update`. Updates non-status fields (name, address, contact). All fields optional.

**Response `200 OK`** — updated BranchResponse.

### `PATCH /api/v1/tenant/branches/:id/status`

**Required permission:** `branch.update`

**Request:** `{ "status": "active" }` — one of `active | inactive`.

**Response `200 OK`** — updated BranchResponse.

**State machine:** `inactive → active`, `active → inactive`. Other transitions return `409 INVALID_STATUS_TRANSITION`.

### `DELETE /api/v1/tenant/branches/:id`

**Required permission:** `branch.delete`. Soft-deletes the branch (sets `deleted_at`). Only `inactive` branches may be deleted; deleting an `active` branch returns `409 INVALID_STATUS_TRANSITION`.

**Response `204 No Content`**.

### `GET /api/v1/tenant/onboarding-state`

**Required permission:** `branch.read`. Returns the tenant admin's post-login onboarding state.

**Response `200 OK`**

```json
{
  "has_branches": false,
  "active_branch_count": 0,
  "must_change_password": true
}
```

Used by the frontend dashboard redirect: if `has_branches = false` and caller is `tenant_admin` → redirect to `/onboarding/welcome`.

### BranchResponse shape

```json
{
  "id": "<uuid>",
  "tenant_id": "<uuid>",
  "name": "Main Branch",
  "code": "MAIN",
  "status": "inactive",
  "address_line1": "Jl. Sudirman No. 1",
  "address_line2": null,
  "city": "Jakarta",
  "province": "DKI Jakarta",
  "postal_code": "10270",
  "country": "ID",
  "timezone": "Asia/Jakarta",
  "contact_phone": "+62215551234",
  "contact_email": "main@acme-wellness.example",
  "activated_at": null,
  "created_at": "2026-04-22T12:00:00Z",
  "updated_at": "2026-04-22T12:00:00Z"
}
```

---

## 7. Open Questions

- ~~`must_change_password` column on `"user"` table — flagged to `db-designer`.~~ Resolved by migration 000006 and middleware enforcement.
- `password_reset` table has no RLS — flagged to `security-expert`.
- JWKS key rotation (multiple keys, `kid` selection) — flagged to `devops-expert`.
- In-memory rate limiter needs Redis migration — flagged to `devops-expert`.

---

## 11. Phase 4 — Master Operational Data

_Added 2026-04-22. Owned by `go-expert`. Implements ADR 0009 and migration 000013._

---

### 11.1 New Error Codes (Phase 4)

The following codes must be added to `constants/error_codes.go` during implementation. All other errors use existing codes from §5.

| Code | HTTP Status | Meaning |
|---|---|---|
| `THERAPIST_NOT_FOUND` | 404 | Therapist does not exist, is soft-deleted, or belongs to a different tenant |
| `SERVICE_NOT_FOUND` | 404 | Service does not exist, is soft-deleted, or belongs to a different tenant |
| `THERAPIST_HAS_ACTIVE_BOOKINGS` | 409 | Soft-delete attempted on a therapist with future non-terminal bookings (enforced in Phase 5; service layer must check once bookings table is populated) |
| `AVAILABILITY_OVERLAP` | 409 | A submitted availability window overlaps an existing window for the same therapist on the same day |
| `CROSS_BRANCH_FORBIDDEN` | 403 | `branch_admin` attempted to create or mutate a therapist or availability window outside their assigned branch |
| `UPLOAD_QUOTA_EXCEEDED` | 429 | Tenant has exceeded `UPLOAD_TENANT_HOURLY_LIMIT` uploads in the current hour (in-memory sliding window; resets on service restart) |
| `INVALID_IMAGE_FORMAT` | 400 | Uploaded file is not JPEG, PNG, or WebP |
| `IMAGE_TOO_LARGE` | 400 | File exceeds `UPLOAD_MAX_MB` after processing |
| `IMAGE_DIMENSIONS_TOO_LARGE` | 400 | Image width or height exceeds 4096 px |

---

### 11.2 Permission Matrix (Phase 4)

All Phase 4 endpoints require `scope=tenant` in the JWT. Permissions were seeded in migration 000005 and guard-inserted in migration 000013.

| Permission | Granted to |
|---|---|
| `therapist.read` | `super_admin`, `tenant_admin`, `branch_admin`, `customer` |
| `therapist.create` | `super_admin`, `tenant_admin`, `branch_admin` |
| `therapist.update` | `super_admin`, `tenant_admin`, `branch_admin` |
| `therapist.delete` | `super_admin`, `tenant_admin`, `branch_admin` |
| `service.read` | `super_admin`, `tenant_admin`, `branch_admin`, `therapist`, `customer` |
| `service.create` | `super_admin`, `tenant_admin`, `branch_admin` |
| `service.update` | `super_admin`, `tenant_admin`, `branch_admin` |
| `service.delete` | `super_admin`, `tenant_admin`, `branch_admin` |
| `availability.read` | `super_admin`, `tenant_admin`, `branch_admin`, `therapist` |
| `availability.create` | `super_admin`, `tenant_admin`, `branch_admin`, `therapist` |
| `availability.update` | `super_admin`, `tenant_admin`, `branch_admin`, `therapist` |
| `availability.delete` | `super_admin`, `tenant_admin`, `branch_admin`, `therapist` |

**Cross-branch authorization rule (cross-agent flag #1):** having `therapist.create`, `therapist.update`, `therapist.delete`, `availability.create`, or `availability.update` in the JWT is necessary but not sufficient for a `branch_admin`. The service layer MUST additionally verify that the target therapist's `branch_id` appears in the caller's `branches` JWT claim. If it does not, return `403 CROSS_BRANCH_FORBIDDEN`. A `tenant_admin` is not subject to this check (they manage all branches).

---

### 11.3 Cross-Agent Flag Resolutions

This section documents how each flag raised in the design phase is resolved in this contract.

**Flag #1 — Cross-branch authorization on availability write:** documented in §11.2. Service layer enforces `branches` claim check before any write on `therapist`, `therapist_service`, or `therapist_availability`. Returns `403 CROSS_BRANCH_FORBIDDEN`.

**Flag #2 — `PUT /therapists/:id/services` soft vs hard update:** the body contains the desired full list of `service_ids`. The backend computes the diff: rows in the new list that do not yet exist are inserted with `is_active = true`; existing rows whose `service_id` appears in the new list are set to `is_active = true` (re-activates previously deactivated mappings); existing rows whose `service_id` is absent from the new list are set to `is_active = false` (soft-deactivation, preserving the audit trail of `created_at`, `created_by`). No mapping rows are hard-deleted. This is documented explicitly in endpoint §11.6.2.

**Flag #3 — `GET /services?category=`:** the `category` query parameter is a case-sensitive string filter against `service.category`. No enum validation — free-form per the schema design. Documented in §11.5.1.

**Flag #4 — `availability.write` permission granularity:** the `PUT /availability` endpoint performs a full replace, which is both a create and an update operation (deletes old rows, inserts new ones). Since migration 000005 seeded `availability.create` and `availability.update` as separate codes (not a combined `availability.write`), the endpoint requires **both `availability.create` AND `availability.update`** in the caller's JWT claims. A caller with only one of the two is rejected with `403 INSUFFICIENT_PERMISSION`. Both codes are already granted together to `tenant_admin`, `branch_admin`, and `therapist`, so this has no practical UX impact.

**Flag #5 — Availability payload shape:** the flat shape `[{dow: 1, start: "09:00", end: "12:00"}, {dow: 1, start: "14:00", end: "17:00"}, ...]` is used. Rationale: it maps 1:1 to `therapist_availability` rows (one row per window), needs no client-side restructuring before a phase 5 booking query, and is simpler to validate with `go-playground/validator` slice tags. The per-day-grouped shape `[{dow, windows: [...]}]` is cleaner for the editor's internal model but requires an unwrap step on both server and client; the flat shape avoids that extra transformation at no readability cost for a contract of this size.

**Flag #6 — `therapist_service.is_active` in service detail response:** `GET /tenant/services/:id` returns only active mappings (`is_active = true`) in the `therapists` array. Inactive mappings are omitted from this read path. Rationale: the service detail view is a catalog/booking reference; showing deactivated mappings would require UI logic to filter or explain them. The therapist detail page (endpoint §11.4.3) returns all mappings with their `is_active` flag, which is where mapping management occurs.

**Flag #7 — Soft-delete vs deactivate semantics:**
- `PATCH /:id/status` with `{"is_active": false}` — **deactivate.** Sets `is_active = false`. Row remains visible in list queries when `is_active=all`. Can be reversed via `PATCH /:id/status` with `{"is_active": true}`.
- `DELETE /:id` — **soft-delete.** Sets `deleted_at = now()` AND `is_active = false` atomically. Row is excluded from all list queries regardless of `is_active` filter. Cannot be reactivated through the normal API. For therapist soft-delete, the service layer also sets `is_active = false` on all `therapist_service` rows for the same therapist in the same transaction.

---

### 11.4 Therapist Endpoints

All endpoints in this group require `Authorization: Bearer <access_token>` with `scope=tenant`.

**Shared response shape — TherapistResponse:**

```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "branch_id": "uuid",
  "user_id": "uuid",
  "full_name": "Siti Rahma",
  "gender": "female",
  "bio": "Berpengalaman 5 tahun dalam pijat relaksasi.",
  "photo_url": "http://localhost:8080/uploads/therapists/uuid/a1b2c3d4e5f6a7b8.jpg",
  "height_cm": 165,
  "weight_kg": 55,
  "build": "sedang",
  "specialties": [],
  "is_active": true,
  "joined_at": "2026-01-15T00:00:00Z",
  "created_at": "2026-04-22T10:00:00Z",
  "updated_at": "2026-04-22T10:00:00Z"
}
```

`user_id`, `gender`, `bio`, `photo_url` are `null` when not set. `photo_url` is the resolved public URL — the server stores an opaque storage key and resolves it at response time; clients must not attempt to construct this URL themselves. `height_cm`, `weight_kg`, `build` are always present (non-null). `specialties` is always an array (empty or populated). `joined_at` is the value stored in `therapist.joined_at`; it may differ from `created_at` if the admin back-fills an existing therapist's start date.

---

#### 11.4.1 `POST /api/v1/tenant/therapists`

Creates a new therapist profile scoped to a branch within the caller's tenant.

**Required permission:** `therapist.create`

**Cross-branch rule:** a `branch_admin` may only create therapists for branches in their JWT `branches` claim. A `tenant_admin` may specify any branch within the tenant.

**Request**

```json
{
  "branch_id": "uuid",
  "full_name": "Siti Rahma",
  "gender": "female",
  "phone": "+628123456789",
  "email": "siti@example.com",
  "bio": "Berpengalaman 5 tahun dalam pijat relaksasi.",
  "height_cm": 165,
  "weight_kg": 55,
  "build": "sedang",
  "joined_at": "2026-01-15",
  "user_id": null
}
```

| Field | Type | Validation |
|---|---|---|
| `branch_id` | string | required, uuid — must belong to caller's tenant |
| `full_name` | string | required, min=1, max=200 |
| `gender` | string | optional, oneof=`male female other` |
| `phone` | string | optional, max=30 |
| `email` | string | optional, valid email, max=320 |
| `bio` | string | optional, max=500 |
| `height_cm` | int | **required**, 100–250 (whole centimetres) |
| `weight_kg` | int | **required**, 30–250 (whole kilograms) |
| `build` | string | **required**, oneof=`langsing sedang atletis tegap` |
| `joined_at` | string | optional, RFC 3339 date (`YYYY-MM-DD`) |
| `user_id` | string | optional, uuid — must be an active member of the same tenant if provided |

**Note:** `photo_url` / `photo_key` are NOT accepted in the request body. Use `POST /therapists/:id/photo` to set a photo.

**Response `201 Created`** — TherapistResponse (see §11.4 shared shape)

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding/format failure |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.create` |
| 403 | `CROSS_BRANCH_FORBIDDEN` | `branch_admin` specified a `branch_id` not in their JWT `branches` claim |
| 404 | `NOT_FOUND` | `branch_id` does not exist in this tenant |
| 404 | `NOT_FOUND` | `user_id` provided but user is not an active member of this tenant |

**Side-effects:** none beyond the INSERT. Audit log event `therapist.created` is appended.

---

#### 11.4.2 `GET /api/v1/tenant/therapists`

Lists therapists within the caller's tenant. Offset-paginated (ADR 0013).

**Required permission:** `therapist.read`

**Query parameters**

| Param | Type | Description |
|---|---|---|
| `branch_id` | uuid | Filter to a single branch. A `branch_admin` always sees only their assigned branches regardless of this filter. |
| `is_active` | bool | `true` (default) / `false` / absent = active only. Pass `is_active=false` to list deactivated therapists. Soft-deleted rows (`deleted_at IS NOT NULL`) are never returned. |
| `page` | int | 1-indexed page number, default 1 |
| `limit` | int | 1–200, default 10 |

**Default sort:** `full_name ASC`, then `created_at ASC` as tiebreaker.

**Response `200 OK`**

```json
{
  "data": [ { ...TherapistResponse } ],
  "page": 1,
  "limit": 10,
  "total_count": 23,
  "total_pages": 3
}
```

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.read` |

---

#### 11.4.3 `GET /api/v1/tenant/therapists/:id`

Returns a single therapist with their current service mappings (all mappings, both active and inactive, so the Layanan tab can render the full picture with per-row `is_active` state).

**Required permission:** `therapist.read`

**Response `200 OK`**

```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "branch_id": "uuid",
  "user_id": null,
  "full_name": "Siti Rahma",
  "gender": "female",
  "bio": "...",
  "photo_url": null,
  "specialties": [],
  "is_active": true,
  "joined_at": "2026-01-15T00:00:00Z",
  "created_at": "2026-04-22T10:00:00Z",
  "updated_at": "2026-04-22T10:00:00Z",
  "services": [
    {
      "service_id": "uuid",
      "name": "Pijat Relaksasi 60 Menit",
      "category": "Pijat",
      "duration_minutes": 60,
      "price_idr": 150000,
      "is_active": true
    }
  ]
}
```

`services` is always an array (empty if no mappings exist). Each entry reflects the current `therapist_service.is_active` flag. Soft-deleted services are excluded from this array even if a mapping row exists.

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.read` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist does not exist, is soft-deleted, or belongs to a different tenant |

---

#### 11.4.4 `PATCH /api/v1/tenant/therapists/:id`

Updates profile fields. Partial update — only provided fields are changed.

**Required permission:** `therapist.update`

**Cross-branch rule:** a `branch_admin` may only update therapists whose `branch_id` appears in their JWT `branches` claim.

**Request**

```json
{
  "full_name": "Siti Rahmawati",
  "gender": "female",
  "phone": "+628129999999",
  "email": "siti.new@example.com",
  "bio": "Updated bio.",
  "height_cm": 165,
  "weight_kg": 55,
  "build": "atletis",
  "joined_at": "2026-01-01",
  "user_id": "uuid"
}
```

| Field | Type | Validation |
|---|---|---|
| `full_name` | string | optional, min=1, max=200 |
| `gender` | string | optional, oneof=`male female other` |
| `phone` | string | optional, max=30 |
| `email` | string | optional, valid email, max=320 |
| `bio` | string | optional, max=500 |
| `height_cm` | int | optional, 100–250 |
| `weight_kg` | int | optional, 30–250 |
| `build` | string | optional, oneof=`langsing sedang atletis tegap` |
| `joined_at` | string | optional, RFC 3339 date (`YYYY-MM-DD`) |
| `user_id` | string | optional, uuid — must be an active member of the same tenant if non-null |

`branch_id` is not patchable — branch assignment is immutable after creation. `photo_url` / `photo_key` are not patchable here — use the photo upload endpoint.

**Response `200 OK`** — updated TherapistResponse (without `services` array; use `GET /:id` to reload services)

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding/format failure |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.update` |
| 403 | `CROSS_BRANCH_FORBIDDEN` | `branch_admin` targeting a therapist outside their branch |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |
| 404 | `NOT_FOUND` | `user_id` provided but user is not an active member of this tenant |

---

#### 11.4.5 `PATCH /api/v1/tenant/therapists/:id/status`

Activates or deactivates a therapist without soft-deleting the row. See §11.3 flag #7 for the semantic distinction.

**Required permission:** `therapist.update`

**Cross-branch rule:** same as §11.4.4.

**Request**

```json
{ "is_active": false }
```

| Field | Type | Validation |
|---|---|---|
| `is_active` | bool | required |

**Response `200 OK`** — updated TherapistResponse (without `services` array)

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Missing or non-boolean `is_active` |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.update` |
| 403 | `CROSS_BRANCH_FORBIDDEN` | Branch mismatch for `branch_admin` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |

**Side-effects:** none beyond the UPDATE. Audit log event `therapist.deactivated` or `therapist.activated`.

---

#### 11.4.6 `DELETE /api/v1/tenant/therapists/:id`

Soft-deletes a therapist. Sets `deleted_at = now()` and `is_active = false` atomically. Also sets `is_active = false` on all `therapist_service` rows for this therapist in the same transaction.

**Required permission:** `therapist.delete`

**Cross-branch rule:** same as §11.4.4.

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.delete` |
| 403 | `CROSS_BRANCH_FORBIDDEN` | Branch mismatch for `branch_admin` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |
| 409 | `THERAPIST_HAS_ACTIVE_BOOKINGS` | Therapist has future non-terminal bookings (enforced once bookings exist in Phase 5; returns 409 rather than silently allowing the delete) |

**Side-effects:** audit log event `therapist.deleted`. Cascade deactivation of `therapist_service` mappings is logged as part of the same event's metadata.

---

#### 11.4.7 `POST /api/v1/tenant/therapists/:id/photo`

Uploads a photo for a therapist. Replaces any previously uploaded photo. Returns the full updated TherapistResponse (with `photo_url` resolved).

**Required permission:** `therapist.update`

**Cross-branch rule:** same as §11.4.4.

**Content-Type:** `multipart/form-data`

**Form field:** `photo` (file) — required. Accepted formats: JPEG, PNG, WebP. Max size: `UPLOAD_MAX_MB` (default 5 MiB).

**Server-side pipeline (enforced in order):**
1. Body-size hard cap via `MaxBytesReader`.
2. Multipart parse (1 MiB in-memory spill).
3. MIME sniff — reject non-image content types.
4. Structural validation via `image.DecodeConfig` — kills polyglot files.
5. Dimension ceiling: max 4096×4096 px.
6. Re-encode via `imaging.Fit(1024, 1024)` — strips EXIF/metadata.
7. Per-tenant hourly quota check (`UPLOAD_TENANT_HOURLY_LIMIT`, default 30).
8. Storage write (key: `therapists/{id}/{16-hex}.{ext}`).
9. DB UPDATE in transaction.
10. Fire-and-forget delete of old key after transaction commits.

**Response `200 OK`** — full TherapistDetailResponse (same shape as `GET /:id`)

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Missing `photo` field or multipart parse failure |
| 400 | `INVALID_IMAGE_FORMAT` | File is not JPEG, PNG, or WebP |
| 400 | `IMAGE_TOO_LARGE` | File exceeds `UPLOAD_MAX_MB` after processing |
| 400 | `IMAGE_DIMENSIONS_TOO_LARGE` | Image exceeds 4096×4096 px |
| 400 | `VALIDATION` | Structural decode failure (polyglot / corrupted file) |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.update` |
| 403 | `CROSS_BRANCH_FORBIDDEN` | Branch mismatch for `branch_admin` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |
| 429 | `UPLOAD_QUOTA_EXCEEDED` | Tenant has exceeded the per-hour upload limit |

**Side-effects:** audit log event `therapist.photo_uploaded`.

---

#### 11.4.8 `DELETE /api/v1/tenant/therapists/:id/photo`

Removes the photo for a therapist. Sets `photo_key = NULL` on the row and schedules deletion of the storage object.

**Required permission:** `therapist.update`

**Cross-branch rule:** same as §11.4.4.

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.update` |
| 403 | `CROSS_BRANCH_FORBIDDEN` | Branch mismatch for `branch_admin` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |

**Side-effects:** audit log event `therapist.photo_removed`.

---

### 11.5 Service Endpoints

All endpoints in this group require `Authorization: Bearer <access_token>` with `scope=tenant`. Services are tenant-scoped — no `branch_id` filter applies.

**Shared response shape — ServiceResponse:**

```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "name": "Pijat Relaksasi 60 Menit",
  "description": "Pijat seluruh tubuh untuk relaksasi mendalam.",
  "category": "Pijat",
  "duration_minutes": 60,
  "price_idr": 150000,
  "currency": "IDR",
  "is_active": true,
  "created_at": "2026-04-22T10:00:00Z",
  "updated_at": "2026-04-22T10:00:00Z"
}
```

`description` and `category` are `null` when not set. `currency` is always present; Phase 4 only creates `IDR` services.

---

#### 11.5.1 `POST /api/v1/tenant/services`

Creates a new service in the caller's tenant.

**Required permission:** `service.create`

**Request**

```json
{
  "name": "Pijat Relaksasi 60 Menit",
  "description": "Pijat seluruh tubuh untuk relaksasi mendalam.",
  "category": "Pijat",
  "duration_minutes": 60,
  "price_idr": 150000
}
```

| Field | Type | Validation |
|---|---|---|
| `name` | string | required, min=1, max=200 |
| `description` | string | optional, max=1000 |
| `category` | string | optional, max=100 — free-form, no enum constraint |
| `duration_minutes` | int | required, min=1, max=1440 |
| `price_idr` | int | required, min=0 — stored in IDR, no decimal |

**Response `201 Created`** — ServiceResponse

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding/format failure |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `service.create` |

**Side-effects:** audit log event `service.created`.

---

#### 11.5.2 `GET /api/v1/tenant/services`

Lists services within the caller's tenant. Offset-paginated (ADR 0013).

**Required permission:** `service.read`

**Query parameters**

| Param | Type | Description |
|---|---|---|
| `is_active` | bool | `true` (default) / `false` / absent = active only. Soft-deleted rows never returned. |
| `category` | string | Case-sensitive string filter against `service.category`. No validation — free-form. |
| `page` | int | 1-indexed page number, default 1 |
| `limit` | int | 1–200, default 10 |

**Default sort:** `name ASC`, then `created_at ASC` as tiebreaker.

**Response `200 OK`**

```json
{
  "data": [ { ...ServiceResponse } ],
  "page": 1,
  "limit": 10,
  "total_count": 18,
  "total_pages": 2
}
```

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `service.read` |

---

#### 11.5.3 `GET /api/v1/tenant/services/:id`

Returns a single service with the list of therapists who have an **active** mapping to it (`therapist_service.is_active = true`). Inactive mappings are omitted — this endpoint is a catalog/booking reference view (cross-agent flag #6).

**Required permission:** `service.read`

**Response `200 OK`**

```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "name": "Pijat Relaksasi 60 Menit",
  "description": "...",
  "category": "Pijat",
  "duration_minutes": 60,
  "price_idr": 150000,
  "currency": "IDR",
  "is_active": true,
  "created_at": "2026-04-22T10:00:00Z",
  "updated_at": "2026-04-22T10:00:00Z",
  "therapists": [
    {
      "therapist_id": "uuid",
      "full_name": "Siti Rahma",
      "branch_id": "uuid",
      "branch_name": "Cabang Utama",
      "is_active": true
    }
  ]
}
```

`therapists` is always an array (empty if no active mappings). Soft-deleted therapists are excluded even if an active mapping row exists.

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `service.read` |
| 404 | `SERVICE_NOT_FOUND` | Service not found or soft-deleted |

---

#### 11.5.4 `PATCH /api/v1/tenant/services/:id`

Updates service fields. Partial update — only provided fields are changed.

**Required permission:** `service.update`

**Request**

```json
{
  "name": "Pijat Relaksasi Premium 60 Menit",
  "description": "Updated description.",
  "category": "Pijat Premium",
  "duration_minutes": 75,
  "price_idr": 200000
}
```

| Field | Type | Validation |
|---|---|---|
| `name` | string | optional, min=1, max=200 |
| `description` | string | optional, max=1000 |
| `category` | string | optional, max=100 |
| `duration_minutes` | int | optional, min=1, max=1440 |
| `price_idr` | int | optional, min=0 |

**Response `200 OK`** — updated ServiceResponse (without `therapists` array)

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding/format failure |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `service.update` |
| 404 | `SERVICE_NOT_FOUND` | Service not found |

---

#### 11.5.5 `PATCH /api/v1/tenant/services/:id/status`

Activates or deactivates a service. See §11.3 flag #7 for semantics.

**Required permission:** `service.update`

**Request**

```json
{ "is_active": false }
```

| Field | Type | Validation |
|---|---|---|
| `is_active` | bool | required |

**Response `200 OK`** — updated ServiceResponse (without `therapists` array)

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Missing or non-boolean `is_active` |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `service.update` |
| 404 | `SERVICE_NOT_FOUND` | Service not found |

---

#### 11.5.6 `DELETE /api/v1/tenant/services/:id`

Soft-deletes a service. Sets `deleted_at = now()` and `is_active = false` atomically. Does **not** automatically modify `therapist_service` rows — the Phase 5 booking engine filters `WHERE service.deleted_at IS NULL`, so orphaned active mapping rows are harmless. The service layer SHOULD set `therapist_service.is_active = false` in the same transaction for clarity, but this is an implementation detail, not a client-visible behavior.

**Required permission:** `service.delete`

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `service.delete` |
| 404 | `SERVICE_NOT_FOUND` | Service not found |

**Side-effects:** audit log event `service.deleted`.

---

### 11.6 Therapist ↔ Service Mapping Endpoints

---

#### 11.6.1 `GET /api/v1/tenant/therapists/:id/services`

Returns the current service mappings for a therapist, including both active and inactive. This endpoint is intentionally redundant with the `services` array on `GET /tenant/therapists/:id` (§11.4.3) — it exists as a standalone route for clients that only need the mapping list without the full therapist profile (e.g. the availability editor pre-check).

**Required permission:** `therapist.read`

**Response `200 OK`**

```json
{
  "therapist_id": "uuid",
  "services": [
    {
      "service_id": "uuid",
      "name": "Pijat Relaksasi 60 Menit",
      "category": "Pijat",
      "duration_minutes": 60,
      "price_idr": 150000,
      "is_active": true,
      "assigned_at": "2026-04-22T10:00:00Z"
    }
  ]
}
```

`services` is always an array. Soft-deleted services are excluded.

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.read` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |

---

#### 11.6.2 `PUT /api/v1/tenant/therapists/:id/services`

Full-replace of the therapist's service assignments. The body declares the desired active set. The backend reconciles as follows (cross-agent flag #2):

1. For each `service_id` in the request body that has no existing `therapist_service` row: INSERT a new row with `is_active = true`.
2. For each `service_id` in the request body that has an existing row with `is_active = false`: UPDATE `is_active = true` (re-activates a previously suspended mapping).
3. For each `service_id` in the request body that has an existing row with `is_active = true`: no change.
4. For each existing row whose `service_id` is NOT in the request body: UPDATE `is_active = false` (soft-deactivation; row and its `created_at`/`created_by` are preserved).

Sending an empty array (`"service_ids": []`) soft-deactivates all current mappings.

**Required permission:** `therapist.update` (mapping management is considered a therapist profile update)

**Cross-branch rule:** same as §11.4.4.

**Request**

```json
{ "service_ids": ["uuid-1", "uuid-2"] }
```

| Field | Type | Validation |
|---|---|---|
| `service_ids` | []string | required (empty array is valid), each element: uuid, must belong to caller's tenant, service must not be soft-deleted |

**Response `200 OK`**

```json
{
  "therapist_id": "uuid",
  "services": [
    {
      "service_id": "uuid",
      "name": "Pijat Relaksasi 60 Menit",
      "category": "Pijat",
      "duration_minutes": 60,
      "price_idr": 150000,
      "is_active": true,
      "assigned_at": "2026-04-22T10:00:00Z"
    }
  ]
}
```

The response reflects the full mapping state after reconciliation (all rows for this therapist, including rows just set to `is_active = false`), matching the shape of §11.6.1. This allows the client to re-render the Layanan tab without a separate GET.

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding failure or a `service_id` is not a valid UUID |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `therapist.update` |
| 403 | `CROSS_BRANCH_FORBIDDEN` | Branch mismatch for `branch_admin` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |
| 404 | `SERVICE_NOT_FOUND` | One or more `service_id` values do not exist in this tenant or are soft-deleted |

**Side-effects:** audit log event `therapist_service.updated` with metadata listing added and removed service IDs.

---

### 11.7 Availability Endpoints

---

#### 11.7.1 `GET /api/v1/tenant/therapists/:id/availability`

Returns the therapist's current weekly availability pattern as a flat list of windows sorted by `day_of_week ASC`, then `start_time ASC`.

**Required permission:** `availability.read`

**Response `200 OK`**

```json
{
  "therapist_id": "uuid",
  "windows": [
    { "id": "uuid", "dow": 1, "start": "09:00", "end": "12:00" },
    { "id": "uuid", "dow": 1, "start": "14:00", "end": "17:00" },
    { "id": "uuid", "dow": 3, "start": "09:00", "end": "17:00" }
  ]
}
```

`windows` is always an array (empty if no availability is set). `dow` follows the DB convention: 0 = Sunday, 1 = Monday, …, 6 = Saturday. `start` and `end` are `"HH:MM"` strings. `id` is the `therapist_availability.id` UUID — exposed so the client can reference individual rows if needed (Phase 5).

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `availability.read` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |

---

#### 11.7.2 `PUT /api/v1/tenant/therapists/:id/availability`

Full-replace of the therapist's weekly availability pattern. All existing `therapist_availability` rows for this therapist are deleted and replaced with the submitted windows in a single transaction. This is a destructive replace — the caller submits the complete desired state.

Sending an empty array (`"windows": []`) clears all availability.

**Required permission:** `availability.create` AND `availability.update` (both required — see §11.3 flag #4)

**Cross-branch rule:** same as §11.4.4.

**Payload shape** (cross-agent flag #5 — flat, one object per window):

```json
{
  "windows": [
    { "dow": 1, "start": "09:00", "end": "12:00" },
    { "dow": 1, "start": "14:00", "end": "17:00" },
    { "dow": 3, "start": "09:00", "end": "17:00" }
  ]
}
```

| Field | Type | Validation |
|---|---|---|
| `windows` | []object | required (empty array is valid) |
| `windows[].dow` | int | required, min=0, max=6 (0=Sunday) |
| `windows[].start` | string | required, format `HH:MM`, must be a valid 24h time |
| `windows[].end` | string | required, format `HH:MM`, must be a valid 24h time, must be after `start` |

**Service-layer validations (applied before DB write, not expressible as binding tags alone):**
1. `end` must be strictly after `start` on each window.
2. No two windows for the same `dow` may overlap: for any pair on the same day, `window_a.end <= window_b.start` (after sorting by `start`). Returns `409 AVAILABILITY_OVERLAP` if violated.
3. Maximum 3 windows per day (matches the UI constraint from DESIGN_SYSTEM.md §3A). Returns `400 VALIDATION` if exceeded.
4. `start` and `end` must be on 5-minute boundaries (i.e. minutes must be divisible by 5). Returns `400 VALIDATION` if violated. The DB stores whatever TIME value the service sends; the 5-minute rule is enforced here at the service layer.

**Response `200 OK`** — same shape as `GET /availability` (§11.7.1) reflecting the new state after replace.

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding failure, invalid time format, end ≤ start, >3 windows/day, or non-5-minute boundary |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `availability.create` or `availability.update` (either missing blocks the request) |
| 403 | `CROSS_BRANCH_FORBIDDEN` | Branch mismatch for `branch_admin` |
| 404 | `THERAPIST_NOT_FOUND` | Therapist not found |
| 409 | `AVAILABILITY_OVERLAP` | Two windows on the same day overlap |

**Side-effects:** audit log event `therapist_availability.replaced` with `window_count` in metadata.

---

### 11.8 Branch Operational Hours

---

#### 11.8.1 `GET /api/v1/tenant/branches/:id/operational-hours`

Extracts the `operational_hours` JSONB field from the branch row. No new DB query — reads the existing `branch` row and projects the `operational_hours` field. This endpoint is read-only; to update operational hours use `PATCH /api/v1/tenant/branches/:id` (Phase 3, §10).

**Required permission:** `branch.read`

**Response `200 OK`**

```json
{
  "branch_id": "uuid",
  "timezone": "Asia/Jakarta",
  "operational_hours": [
    { "day": "monday",    "open": "09:00", "close": "21:00" },
    { "day": "tuesday",   "open": "09:00", "close": "21:00" },
    { "day": "wednesday", "open": "09:00", "close": "21:00" },
    { "day": "thursday",  "open": "09:00", "close": "21:00" },
    { "day": "friday",    "open": "09:00", "close": "21:00" },
    { "day": "saturday",  "open": "10:00", "close": "20:00" },
    { "day": "sunday",    "open": "10:00", "close": "18:00" }
  ]
}
```

The `operational_hours` array shape mirrors the JSONB stored in `branch.operational_hours` (Phase 3 schema). An empty array `[]` is returned if the branch has not yet had operational hours configured. `timezone` is always present — the frontend availability editor uses it to annotate the display. The JSONB is returned as-is from the DB; no re-projection.

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `branch.read` |
| 404 | `NOT_FOUND` | Branch not found or belongs to a different tenant |

---

## 12. Add-on Catalog (ADR 0010)

_Rewritten 2026-04-24. Owned by `go-expert`. Implements ADR 0010 — tenant-wide add-on catalog (not per-service). The original per-service draft from the same session is superseded in full._

Add-ons are **tenant-wide** optional paid extras. Any add-on in the tenant's catalog is available alongside any service during a booking. No mapping table. See `docs/DECISIONS/0010-per-service-addons.md` for full rationale.

---

### 12.1 New Error Codes (ADR 0010)

| Code | HTTP Status | Meaning |
|---|---|---|
| `ADDON_NOT_FOUND` | 404 | Add-on does not exist, is soft-deleted, or belongs to a different tenant |
| `DUPLICATE_ADDON_NAME` | 409 | An active add-on with the same name already exists for this tenant (partial unique index on `(tenant_id, name) WHERE deleted_at IS NULL`) |

---

### 12.2 Permission Matrix (ADR 0010)

Add-ons are a standalone tenant resource. New `addon.*` permission namespace — distinct from `service.*` so future roles can diverge.

| Operation | Required permission | Roles granted |
|---|---|---|
| List / get add-on | `addon.read` | `super_admin`, `tenant_admin`, `branch_admin` |
| Create add-on | `addon.create` | `super_admin`, `tenant_admin` |
| Update add-on | `addon.update` | `super_admin`, `tenant_admin` |
| Soft-delete add-on | `addon.delete` | `super_admin`, `tenant_admin` |
| Reorder add-ons | `addon.update` | `super_admin`, `tenant_admin` |
| Change add-on status | `addon.update` | `super_admin`, `tenant_admin` |

All endpoints require `scope=tenant` in the JWT. `branch_admin` is **read-only** on add-ons — they cannot mutate the tenant catalog.

---

### 12.3 Shared Response Shape — AddonResponse

```json
{
  "id": "uuid",
  "name": "Aromaterapi Premium",
  "description": "Minyak esensial lavender pilihan.",
  "price_idr": 35000,
  "is_active": true,
  "sort_order": 0,
  "created_at": "2026-04-24T10:00:00Z",
  "updated_at": "2026-04-24T10:00:00Z"
}
```

`description` is `null` when not set. `tenant_id` is intentionally omitted — it is implicit from the JWT and enforced by RLS.

---

### 12.4 Add-on Endpoints

All endpoints require `Authorization: Bearer <access_token>` with `scope=tenant`.

---

#### 12.4.1 `GET /api/v1/tenant/addons`

Lists all non-soft-deleted add-ons for the caller's tenant. Offset-paginated (ADR 0013), default page size 10, max 200 per page. Both active and inactive rows returned by default (admin view).

**Required permission:** `addon.read`

**Query parameters**

| Param | Type | Description |
|---|---|---|
| `is_active` | bool | Optional. `true` = active only; `false` = inactive only; absent = all non-deleted |
| `page` | int | 1-indexed page number, default 1 |
| `limit` | int | Page size 1–200, default 10 |

**Default sort:** `sort_order ASC`, then `created_at ASC`, then `id ASC` as tiebreaker.

**Response `200 OK`**

```json
{
  "data": [ { ...AddonResponse } ],
  "page": 1,
  "limit": 10,
  "total_count": 8,
  "total_pages": 1
}
```

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `addon.read` |

---

#### 12.4.2 `GET /api/v1/tenant/addons/:id`

Returns a single add-on by ID. Cross-tenant IDs return `404` (IDOR guard).

**Required permission:** `addon.read`

**Response `200 OK`** — AddonResponse

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `addon.read` |
| 404 | `ADDON_NOT_FOUND` | Add-on not found, soft-deleted, or belongs to a different tenant |

---

#### 12.4.3 `POST /api/v1/tenant/addons`

Creates a new tenant-wide add-on. `tenant_id` is taken from the JWT — never from the request body.

**Required permission:** `addon.create`

**Request**

```json
{
  "name": "Aromaterapi Premium",
  "description": "Minyak esensial lavender pilihan.",
  "price_idr": 35000,
  "sort_order": 0
}
```

| Field | Type | Validation |
|---|---|---|
| `name` | string | required, min=1, max=120 (unicode rune count) |
| `description` | string | optional, max=500 |
| `price_idr` | int64 | required, min=0 — whole Rupiah, no decimal |
| `sort_order` | int | optional, default=0, min=0, max=9999 |

**Response `201 Created`** — AddonResponse (`is_active` defaults to `true`)

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding failure or service-layer length/range check |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `addon.create` |
| 409 | `DUPLICATE_ADDON_NAME` | Active add-on with same name already exists for this tenant |

---

#### 12.4.4 `PATCH /api/v1/tenant/addons/:id`

Updates add-on fields. Partial update — only provided fields are changed.

**Required permission:** `addon.update`

**Request**

```json
{
  "name": "Aromaterapi",
  "description": "Updated description.",
  "price_idr": 30000,
  "sort_order": 1
}
```

| Field | Type | Validation |
|---|---|---|
| `name` | string | optional, min=1, max=120 |
| `description` | string | optional, max=500 |
| `price_idr` | int64 | optional, min=0 |
| `sort_order` | int | optional, min=0, max=9999 |

**Response `200 OK`** — updated AddonResponse

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding failure or constraint violation |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `addon.update` |
| 404 | `ADDON_NOT_FOUND` | Add-on not found, soft-deleted, or belongs to a different tenant |
| 409 | `DUPLICATE_ADDON_NAME` | Name change conflicts with existing active add-on |

---

#### 12.4.5 `PATCH /api/v1/tenant/addons/:id/status`

Activates or deactivates an add-on without soft-deleting it.

**Required permission:** `addon.update`

**Request**

```json
{ "is_active": false }
```

| Field | Type | Validation |
|---|---|---|
| `is_active` | bool | required |

**Response `200 OK`** — updated AddonResponse

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Missing or non-boolean `is_active` |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `addon.update` |
| 404 | `ADDON_NOT_FOUND` | Add-on not found |

---

#### 12.4.6 `DELETE /api/v1/tenant/addons/:id`

Soft-deletes an add-on. Sets `deleted_at = now()` and `is_active = false`. Excluded from all subsequent list queries. No hard-DELETE is issued (DB grant does not permit it).

**Required permission:** `addon.delete`

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 403 | `INSUFFICIENT_PERMISSION` | Missing `addon.delete` |
| 404 | `ADDON_NOT_FOUND` | Add-on not found |

**Side-effects:** audit log event `addon.deleted`.

---

#### 12.4.7 `PUT /api/v1/tenant/addons/reorder`

Bulk-updates `sort_order` for multiple add-ons atomically (all-or-nothing in one transaction). All provided IDs must belong to the caller's tenant; any foreign or nonexistent ID aborts the entire request (IDOR guard).

**Required permission:** `addon.update`

**Request**

```json
{
  "items": [
    { "id": "uuid-b", "sort_order": 0 },
    { "id": "uuid-a", "sort_order": 1 },
    { "id": "uuid-c", "sort_order": 2 }
  ]
}
```

| Field | Type | Validation |
|---|---|---|
| `items` | []object | required, min=1, max=200 |
| `items[].id` | string | required, uuid |
| `items[].sort_order` | int | required, min=0, max=9999 |

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding failure or empty items list |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `addon.update` |
| 404 | `ADDON_NOT_FOUND` | One or more IDs not found, soft-deleted, or belong to a different tenant |

**Atomicity guarantee:** the entire bulk update succeeds or fails together. Partial updates are not possible.

---

## 13. Rooms (Ruangan) Catalog (ADR 0012)

**Base path:** `/api/v1/tenant/rooms`
**Auth:** `scope=tenant` + listed permission on every route.
**Branch-scope rule:** callers with role `branch_admin` may only create, read, update, delete, and reorder rooms that belong to a branch in their JWT `branches` claim. Attempting to act on a room in a foreign branch returns `403 CROSS_BRANCH_FORBIDDEN`.

### Room object

```json
{
  "id": "uuid",
  "branch_id": "uuid",
  "name": "VIP 1",
  "description": "Ruangan premium dengan shower dan TV",
  "room_type": "vip",
  "capacity": 2,
  "amenities": ["shower", "tv", "aromaterapi"],
  "photo_url": "https://…/uploads/rooms/…/abc.jpg",
  "is_active": true,
  "sort_order": 0,
  "created_at": "2026-04-25T10:00:00Z",
  "updated_at": "2026-04-25T10:00:00Z"
}
```

`tenant_id` and `photo_key` are **never** included in any response. `photo_url` is `null` when no photo has been uploaded. `amenities` is always a JSON array (never `null`); empty array `[]` means no amenities.

### 13.1 List rooms

`GET /api/v1/tenant/rooms`  **Permission:** `room.read`

**Query parameters**

| Parameter | Type | Description |
|---|---|---|
| `branch_id` | uuid (optional) | Filter by branch |
| `is_active` | bool (optional) | Filter by active status; absent returns all non-deleted |
| `room_type` | string (optional) | One of `single`, `couple`, `group`, `vip` |
| `page` | int (optional) | 1-indexed page number, default 1 |
| `limit` | int (optional) | 1–200, default 10 |

**Response `200 OK`**

```json
{ "data": [ { "…room object…" } ], "page": 1, "limit": 10, "total_count": 6, "total_pages": 1 }
```

**Errors:** `400 VALIDATION`, `403 INSUFFICIENT_PERMISSION`.

### 13.2 Get room

`GET /api/v1/tenant/rooms/:id`  **Permission:** `room.read`

**Response `200 OK`** — room object.

**Errors:** `404 ROOM_NOT_FOUND` when not found, soft-deleted, or cross-tenant.

### 13.3 Create room

`POST /api/v1/tenant/rooms`  **Permission:** `room.create`

`photo_key` is NOT settable via this endpoint — upload photo via `POST /:id/photo` after first save.

**Request body**

| Field | Type | Validation |
|---|---|---|
| `branch_id` | string | required, uuid |
| `name` | string | required, 1–120 chars |
| `description` | string | optional, max 500 chars |
| `room_type` | string | required, one of `single`, `couple`, `group`, `vip` |
| `capacity` | int | required, 1–20 |
| `amenities` | []string | optional, max 20 elements, each element max 80 chars; service normalises (lowercase, dedup, trim) |
| `sort_order` | int | optional, 0–9999, default 0 |

**Response `201 Created`** — room object.

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding failure or service-layer validation |
| 403 | `CROSS_BRANCH_FORBIDDEN` | `branch_admin` targeting a branch not in their JWT |
| 404 | `NOT_FOUND` | `branch_id` does not exist or does not belong to caller's tenant |
| 409 | `DUPLICATE_ROOM_NAME` | Name already used in the same branch |

### 13.4 Update room

`PATCH /api/v1/tenant/rooms/:id`  **Permission:** `room.update`

All fields optional. `branch_id` is **immutable**: supplying a different value returns `400 ROOM_BRANCH_IMMUTABLE`. Supplying the same value is allowed.

**Request body fields:** `name`, `description`, `room_type`, `capacity`, `amenities`, `sort_order` (all optional).

**Response `200 OK`** — updated room object.

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `ROOM_BRANCH_IMMUTABLE` | `branch_id` in body differs from stored value |
| 403 | `CROSS_BRANCH_FORBIDDEN` | `branch_admin` — room belongs to foreign branch |
| 404 | `ROOM_NOT_FOUND` | Not found or soft-deleted |
| 409 | `DUPLICATE_ROOM_NAME` | Updated name already used in same branch |

### 13.5 Change room status

`PATCH /api/v1/tenant/rooms/:id/status`  **Permission:** `room.update`

**Request:** `{ "is_active": false }`

**Response `200 OK`** — updated room object.

**Errors:** `403 CROSS_BRANCH_FORBIDDEN`, `404 ROOM_NOT_FOUND`.

### 13.6 Soft-delete room

`DELETE /api/v1/tenant/rooms/:id`  **Permission:** `room.delete`

Sets `deleted_at = now()` and `is_active = false`. No hard DELETE.

**Response `204 No Content`**

**Errors:** `403 CROSS_BRANCH_FORBIDDEN`, `404 ROOM_NOT_FOUND`.

### 13.7 Reorder rooms (bulk)

`PUT /api/v1/tenant/rooms/reorder`  **Permission:** `room.update`

Atomically updates `sort_order` for a batch of rooms. All items must belong to the specified `branch_id`. Max 200 items.

**Request body**

| Field | Type | Validation |
|---|---|---|
| `branch_id` | string | required, uuid — all items must belong to this branch |
| `items` | []object | required, min=1, max=200 |
| `items[].id` | string | required, uuid |
| `items[].sort_order` | int | required, 0–9999 |

**Response `204 No Content`**

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Binding failure, any item belongs to a different branch, or sort_order out of range |
| 403 | `CROSS_BRANCH_FORBIDDEN` | `branch_admin` — `branch_id` not in their JWT |
| 404 | `ROOM_NOT_FOUND` | One or more IDs not found, soft-deleted, or cross-tenant |

**Atomicity guarantee:** entire bulk update succeeds or fails together.

### 13.8 Upload room photo

`POST /api/v1/tenant/rooms/:id/photo`  **Permission:** `room.update`

`Content-Type: multipart/form-data`, field name `photo`. Accepted: JPEG, PNG, WebP. Max size: `UPLOAD_MAX_MB` (default 5 MiB). Max dimensions: 4096×4096 px. EXIF stripped on upload. Per-tenant hourly quota **shared** with therapist uploads.

Storage key format: `rooms/{room_id}/{16-hex}.{ext}`.

**Response `200 OK`** — updated room object (with `photo_url` resolved).

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `INVALID_IMAGE_FORMAT` | Not JPEG, PNG, or WebP |
| 400 | `IMAGE_TOO_LARGE` | Exceeds `UPLOAD_MAX_MB` |
| 400 | `IMAGE_DIMENSIONS_TOO_LARGE` | Width or height exceeds 4096 px |
| 403 | `CROSS_BRANCH_FORBIDDEN` | `branch_admin` on foreign branch |
| 404 | `ROOM_NOT_FOUND` | Not found or soft-deleted |
| 429 | `UPLOAD_QUOTA_EXCEEDED` | Per-tenant hourly limit reached |

### 13.9 Remove room photo

`DELETE /api/v1/tenant/rooms/:id/photo`  **Permission:** `room.update`

Clears `photo_key` on the room row; schedules async deletion of the old storage object.

**Response `204 No Content`**

**Errors:** `403 CROSS_BRANCH_FORBIDDEN`, `404 ROOM_NOT_FOUND`.

---

## §14 — Booking Engine (ADR 0014)

_Last updated: 2026-04-25_

### Security notes (inline)

- **C-1:** `total_price_idr` is NEVER accepted from the request body on any booking-create endpoint. It is computed server-side from DB-fetched `service.price_idr` + sum of `addon.price_idr`. Any client-supplied value is ignored by design (field is absent from the DTO).
- **H-2:** All `/public/*` booking endpoints are rate-limited per IP. Limits: `POST /public/bookings` 5 req/min; `GET /public/bookings/:code` 20 req/min; `GET /public/branches/:id/availability` 30 req/min; `GET /public/branches` 60 req/min. Webhook is not rate-limited.
- **H-7:** `GET /public/bookings/:code` always scopes the DB query to `WHERE code = $1`. No unbounded public booking scan is possible.

---

### 14.1 Public endpoints (no JWT)

#### 14.1.1 List branches (public)

`GET /api/v1/public/branches`

**Query params:**

| Param | Type | Description |
|---|---|---|
| `q` | string | Substring search on branch name or city |
| `lat` | float | Caller latitude — enables distance sorting |
| `lng` | float | Caller longitude — enables distance sorting |
| `category` | string | Filter by service category |
| `open_now` | bool | Filter to branches open right now |
| `page` | int | Page number (default 1) |
| `limit` | int | Page size (default 10, max 50) |

**Response `200 OK`:**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Lustia Utama",
      "city": "Jakarta",
      "province": "DKI Jakarta",
      "address_line1": "Jl. Sudirman No. 1",
      "contact_phone": "021-1234567",
      "contact_email": "info@lustia.local",
      "latitude": -6.2088,
      "longitude": 106.8456,
      "distance_meters": 1234.5,
      "categories": ["Pijat", "Facial"]
    }
  ],
  "page": 1,
  "limit": 10,
  "total_count": 42
}
```

Only returns branches with `branch.status = 'active'` AND `tenant.status = 'active'`. `distance_meters` is `null` when `lat`/`lng` are not provided.

**Errors:** `400 VALIDATION`.

---

#### 14.1.2 Branch availability slots

`GET /api/v1/public/branches/:id/availability`

**Query params:**

| Param | Type | Required | Description |
|---|---|---|---|
| `service_id` | UUID | yes | Service to book |
| `date` | string | yes | Date in `YYYY-MM-DD` |
| `therapist_id` | UUID | no | When supplied, each slot gains a boolean `therapist_available` indicating whether that specific therapist is free at the slot (no overlapping booking AND their weekly schedule covers the slot's day-of-week and time range). Aggregate counts are unaffected. |

**Response `200 OK` — without `therapist_id`:**
```json
{
  "branch_id": "uuid",
  "service_id": "uuid",
  "date": "2026-04-26",
  "slots": [
    {
      "start": "2026-04-26T09:00:00Z",
      "end": "2026-04-26T10:00:00Z",
      "therapists_available_count": 3,
      "rooms_available_count": 2,
      "available_room_ids": ["uuid-room-1", "uuid-room-2"]
    }
  ]
}
```

**Response `200 OK` — with `?therapist_id=<UUID>`:**
```json
{
  "branch_id": "uuid",
  "service_id": "uuid",
  "date": "2026-04-26",
  "slots": [
    {
      "start": "2026-04-26T09:00:00Z",
      "end": "2026-04-26T10:00:00Z",
      "therapists_available_count": 3,
      "rooms_available_count": 1,
      "available_room_ids": ["uuid-room-2"],
      "therapist_available": true
    },
    {
      "start": "2026-04-26T09:30:00Z",
      "end": "2026-04-26T10:30:00Z",
      "therapists_available_count": 3,
      "rooms_available_count": 2,
      "available_room_ids": ["uuid-room-1", "uuid-room-2"],
      "therapist_available": false
    }
  ]
}
```

**Field semantics:**

| Field | Always present | Description |
|---|---|---|
| `therapists_available_count` | yes | Count of ALL therapists at the branch who can perform the service and are not booked at this slot. Unaffected by `therapist_id` filter. |
| `rooms_available_count` | yes | Count of rooms at the branch NOT booked at this slot. Equal to `len(available_room_ids)`. |
| `available_room_ids` | yes | UUIDs of rooms not booked at this slot. Empty array when no rooms are configured. Use this to grey-out fully-booked rooms after the customer picks a slot. |
| `therapist_available` | only when `?therapist_id` given | `true` = therapist is free (no conflicting booking AND their weekly schedule covers the slot). `false` = therapist is booked OR their schedule does not cover this slot's day/time. **Field is absent** when `therapist_id` was not supplied — old clients receive the same JSON as before. |

**UI guidance for Flutter:** When `therapist_available=false` AND `therapists_available_count > 0`, the slot is still bookable with a different therapist. The recommended UX label is *"Terapis sudah dibooking di jam ini"*. When `therapist_available=false` AND `therapists_available_count = 0`, the slot itself is blocked (no therapist available at all) — show it as fully disabled without a therapist-specific reason.

**Unknown `therapist_id`:** A UUID that is syntactically valid but not in the database returns `200` with all slots having `therapist_available=false` rather than a `404`. This avoids hard errors when the customer is rapidly switching between therapist options during the selection flow.

Calls the lazy expiry sweep before computing slots. Slots with `therapists_available_count = 0` are omitted. Slot windows are generated at 30-minute intervals between 09:00 and 21:00 UTC on the requested date (operational hours parsing is a planned enhancement).

**Errors:** `400 VALIDATION` (invalid `service_id`, `date`, or malformed `therapist_id`), `404 NOT_FOUND` (branch or service not found).

---

#### 14.1.3 Create booking (customer)

`POST /api/v1/public/bookings`

**Rate limit:** 5 req/min per IP.

**Request body:**
```json
{
  "branch_id": "uuid",
  "service_id": "uuid",
  "addon_ids": ["uuid"],
  "room_id": "uuid",
  "therapist_id": "uuid",
  "scheduled_start": "2026-04-26T10:00:00Z",
  "customer_name": "Budi Santoso",
  "customer_phone": "081234567890",
  "customer_email": "budi@example.com"
}
```

`room_id` and `therapist_id` are optional — server auto-assigns if omitted. `addon_ids` may be empty. `total_price_idr` is intentionally absent (C-1).

**Response `201 Created`:**
```json
{
  "id": "uuid",
  "code": "B7K3-M2QF",
  "status": "pending_payment",
  "total_price_idr": 180000,
  "snap_token": "dummy-snap-token-B7K3-M2QF",
  "redirect_url": "http://localhost:8080/dummy-payment?order=B7K3-M2QF",
  "scheduled_start": "2026-04-26T10:00:00Z",
  "scheduled_end": "2026-04-26T11:00:00Z",
  "addons": [
    { "addon_id": "uuid", "name": "Aromaterapi", "price_idr": 30000 }
  ]
}
```

Calls lazy expiry sweep before INSERT. On DB GiST exclusion constraint violation → `409 BOOKING_SLOT_CONFLICT`.

**Errors:**

| Code | HTTP | Meaning |
|---|---|---|
| `VALIDATION` | 400 | Missing/invalid fields |
| `NOT_FOUND` | 404 | branch_id, service_id, addon_id, room_id, or therapist_id not found or cross-tenant |
| `BOOKING_SLOT_CONFLICT` | 409 | Therapist or room already booked for this slot (DB exclusion constraint) |
| `NO_THERAPIST_AVAILABLE` | 409 | Auto-assign: no therapist available for slot |
| `THERAPIST_NOT_FOR_SERVICE` | 409 | Specified therapist not mapped to service |
| `RATE_LIMITED` | 429 | IP rate limit exceeded |

---

#### 14.1.4 Get booking by code (public)

`GET /api/v1/public/bookings/:code`

**Rate limit:** 20 req/min per IP.

**Response `200 OK`:**
```json
{
  "code": "B7K3-M2QF",
  "branch_name": "Lustia Utama",
  "service_name": "Pijat Relaksasi",
  "scheduled_start": "2026-04-26T10:00:00Z",
  "scheduled_end": "2026-04-26T11:00:00Z",
  "status": "paid",
  "total_price_idr": 180000,
  "customer_name": "Budi Santoso",
  "customer_phone": "****7890",
  "customer_email": "bu**@example.com",
  "addons": []
}
```

`customer_phone` masked to last 4 digits; `customer_email` masked to first 2 chars + domain (M-2). Query is always `WHERE code = $1` — never unbounded (H-7).

**Errors:** `400 BOOKING_CODE_INVALID`, `404 BOOKING_NOT_FOUND`, `429 RATE_LIMITED`.

---

#### 14.1.5 Payment webhook

`POST /api/v1/public/payments/webhook`

Not rate-limited (Midtrans retries on non-200). Always returns `200 OK` even on internal errors to suppress retry storms.

**Request body (Midtrans notification shape):**
```json
{
  "order_id": "B7K3-M2QF",
  "transaction_status": "settlement",
  "status_code": "200",
  "gross_amount": "180000.00",
  "signature_key": "<sha512>",
  "payment_type": "credit_card"
}
```

**Behaviour:**
- Real adapter: verifies SHA-512 signature (`subtle.ConstantTimeCompare`) before any DB read (H-3).
- Validates `gross_amount >= booking.total_price_idr`; underpayment → audit log `payment.amount_mismatch`, return 200, do NOT mark paid (H-4).
- Conditional UPDATE `WHERE status = 'pending_payment'`; `rowsAffected = 0` distinguishes already-paid (idempotent) vs expired (H-5 alert to ops).

**Response `200 OK`:** `{"status": "ok"}`

---

### 14.2 Operator endpoints (tenant JWT required)

All operator endpoints require `Authorization: Bearer <token>` with `scope=tenant`.

#### 14.2.1 List bookings

`GET /api/v1/tenant/bookings`  **Permission:** `booking.read`

**Query params:** `branch_id`, `status`, `service_id`, `from` (YYYY-MM-DD), `to` (YYYY-MM-DD), `page`, `limit`.

**Response `200 OK`:**
```json
{
  "data": [ /* BookingResponse array */ ],
  "page": 1, "limit": 10, "total_count": 47, "total_pages": 5
}
```

branch_admin callers are automatically restricted to their JWT `branches` claim.

---

#### 14.2.2 Get booking detail

`GET /api/v1/tenant/bookings/:id`  **Permission:** `booking.read`

Returns full `BookingResponse` including unmasked `customer_phone` and `customer_email`.

**Errors:** `404 BOOKING_NOT_FOUND`, `403 CROSS_BRANCH_FORBIDDEN`.

---

#### 14.2.3 Get booking by code (operator)

`GET /api/v1/tenant/bookings/by-code/:code`  **Permission:** `booking.read`

Returns full `BookingResponse`. Used by ops scan flow as an alternative to `:id`.

---

#### 14.2.4 Create concierge booking

`POST /api/v1/tenant/bookings`  **Permission:** `booking.create`

Same request shape as `POST /public/bookings`. Payment method is always `paid_at_venue`; booking starts as `paid` immediately (no Midtrans transaction). H-6: all FK resources validated against caller's branch scope.

**Response `201 Created`:** `CreateBookingResponse` (snap_token and redirect_url are empty).

---

#### 14.2.5 Check in

`POST /api/v1/tenant/bookings/:id/checkin`  **Permission:** `booking.checkin`

**Request body (optional):**
```json
{ "code": "B7K3-M2QF" }
```

Transitions `paid → checked_in`. If `code` is provided, it must match `booking.code`. Returns updated `BookingResponse`.

**Errors:** `404 BOOKING_NOT_FOUND`, `409 BOOKING_INVALID_STATUS_TRANSITION`, `400 BOOKING_CODE_INVALID`.

---

#### 14.2.6 Complete

`POST /api/v1/tenant/bookings/:id/complete`  **Permission:** `booking.complete`

Transitions `checked_in → completed`. Returns updated `BookingResponse`.

**Errors:** `404 BOOKING_NOT_FOUND`, `409 BOOKING_INVALID_STATUS_TRANSITION`.

---

#### 14.2.7 No-show

`POST /api/v1/tenant/bookings/:id/no-show`  **Permission:** `booking.no_show`

Transitions `paid` or `checked_in → no_show`. Returns updated `BookingResponse`.

**Errors:** `404 BOOKING_NOT_FOUND`, `409 BOOKING_INVALID_STATUS_TRANSITION`.

---

#### 14.2.8 Cancel (ops force-cancel)

`POST /api/v1/tenant/bookings/:id/cancel`  **Permission:** `booking.cancel`

**Request body:**
```json
{ "reason": "Customer tidak bisa hadir" }
```

Transitions `paid`, `checked_in`, **or `pending_payment`** → `cancelled`. `reason` is required (max 1000 chars). No refund path in Phase 5.

When cancelled from `pending_payment`, the associated awaiting `payment_transaction` row is atomically marked `voided` so the customer's QR code becomes inert. Audit action is `booking.cancelled_pending` (vs. `booking.cancelled` for paid/checked_in). Both variants record `previous_status` in audit `Meta`.

Returns updated `BookingResponse`.

**Errors:** `400 VALIDATION`, `404 BOOKING_NOT_FOUND`, `409 BOOKING_INVALID_STATUS_TRANSITION`.

---

#### 14.2.10 Sync payment status (manual provider poll)

`POST /api/v1/tenant/bookings/:id/sync-payment`  **Permission:** `booking.read`

No request body required.

Polls the payment provider (iPaymu / dummy) for the current transaction state and applies any outstanding transition:

| Provider status | Action |
|---|---|
| `pending` | No-op — returns current booking unchanged |
| `paid` | `payment_transaction → paid`, `booking → paid` (H-5 idempotent) |
| `expired` | `payment_transaction → expired`, `booking → expired` |
| `failed` | `payment_transaction → failed`, `booking → expired` |

Idempotent — safe to call repeatedly. Emits audit log entry `payment.synced_manually` with `provider_status` and `action_taken` in Meta.

If the booking is not `pending_payment`, returns the current `BookingResponse` as-is (no-op, no provider call made).

**Response `200 OK`:** full `BookingResponse` shape (same as `GET /tenant/bookings/:id`).

**Errors:** `403 CROSS_BRANCH_FORBIDDEN`, `404 BOOKING_NOT_FOUND`.

---

#### 14.2.11 Report summary  <!-- was 14.2.9 before sync-payment was added -->

`GET /api/v1/tenant/reports/bookings/summary`  **Permission:** `booking.read`

**Query params:** `branch_id` (optional), `from` (YYYY-MM-DD, required), `to` (YYYY-MM-DD, required).

**Response `200 OK`:**
```json
{
  "total_bookings": 142,
  "total_paid_idr": 21300000,
  "completed_count": 98,
  "cancelled_count": 12,
  "no_show_count": 8,
  "expired_count": 24,
  "no_show_rate": 0.0755
}
```

`no_show_rate = no_show_count / (completed_count + no_show_count)`, 0 when denominator is 0.

---

### 14.3 Booking status lifecycle

```
pending_payment ──pay (webhook / sync)──▶ paid ──checkin──▶ checked_in ──complete──▶ completed
       │                                    │                    │
       ├──expire (15 min sweep / sync)──▶ expired                └──ops cancel──▶ cancelled
       │
       └──ops cancel (booking.cancel)──▶ cancelled   [payment_transaction → voided]
                                    │
                                    └──slot ends, ops flags──▶ no_show
```

The lazy expiry sweep runs on `POST /public/bookings` and `GET /public/branches/:id/availability` before the main operation.

---

### 14.4 Booking error codes

| Code | HTTP | Meaning |
|---|---|---|
| `BOOKING_NOT_FOUND` | 404 | Booking ID or code does not resolve |
| `BOOKING_CODE_INVALID` | 400 | Code format is invalid |
| `BOOKING_SLOT_CONFLICT` | 409 | DB GiST exclusion constraint fired (double-booking) |
| `BOOKING_EXPIRED` | 409 | Booking has expired (15-min TTL elapsed) |
| `BOOKING_INVALID_STATUS_TRANSITION` | 409 | Status transition not valid for current booking state |
| `NO_THERAPIST_AVAILABLE` | 409 | Auto-assign: no therapist available for slot |
| `NO_ROOM_AVAILABLE` | 409 | Auto-assign: no room available for slot |
| `THERAPIST_NOT_FOR_SERVICE` | 409 | Therapist not mapped to requested service |

---

### 14.5 `BookingResponse` shape (operator)

```json
{
  "id": "uuid",
  "branch_id": "uuid",
  "branch_name": "Lustia Utama",
  "service_id": "uuid",
  "service_name": "Pijat Relaksasi",
  "room_id": "uuid",
  "room_name": "Ruangan A",
  "therapist_id": "uuid",
  "therapist_name": "Sari Dewi",
  "customer_name": "Budi Santoso",
  "customer_phone": "081234567890",
  "customer_email": "budi@example.com",
  "code": "B7K3-M2QF",
  "scheduled_start": "2026-04-26T10:00:00Z",
  "scheduled_end": "2026-04-26T11:00:00Z",
  "total_price_idr": 180000,
  "payment_method": "midtrans",
  "payment_reference": "snap-token-xyz",
  "paid_at": "2026-04-26T09:55:00Z",
  "status": "paid",
  "cancelled_at": null,
  "cancelled_by": null,
  "cancel_reason": null,
  "checked_in_at": null,
  "checked_in_by": null,
  "completed_at": null,
  "completed_by": null,
  "addons": [
    { "addon_id": "uuid", "name": "Aromaterapi", "price_idr": 30000 }
  ],
  "created_at": "2026-04-26T09:50:00Z",
  "updated_at": "2026-04-26T09:55:00Z"
}
```


---

## §15 Payment + Finance + Payout (ADR 0015)

_Last updated: 2026-05-01 (Phase 6 — ADR 0015)_

### 15.1 Conventions for this section

| Item | Value |
|---|---|
| Auth | Public endpoints: no JWT. Tenant endpoints: `Bearer` JWT. Admin endpoints: `Bearer` JWT with `super_admin` role. |
| Rate limits | Webhook: none. Polling: 12/min per IP. Booking create: 5/min per IP. |
| Language | Error messages on `/tenant/*` and `/admin/*` endpoints are in Indonesian. `/public/*` errors are in English. |

---

### 15.2 Public — Payment

#### POST `/api/v1/public/payments/webhook`

Receives iPaymu payment notification. Always returns HTTP 200 (provider must not retry on non-200). Raw body bytes are passed to the adapter for HMAC verification before JSON decode.

**Request:** raw JSON body (provider-specific; no fixed schema)

**Response 200:**
```json
{ "status": "ok" }
```

---

#### GET `/api/v1/public/bookings/:code/payment-status`

Polling endpoint for Flutter app. Rate-limited 12/min per IP.

**Response 200:**
```json
{
  "status": "awaiting | paid | expired | failed",
  "paid_at": "2026-04-30T10:15:00Z",
  "qr_expires_at": "2026-04-30T10:30:00Z"
}
```

**Errors:** `404 BOOKING_NOT_FOUND`, `400 BOOKING_CODE_INVALID`

---

#### POST `/api/v1/public/payments/dummy-trigger` _(dev/local only)_

Simulates a payment confirmation for testing. Build-tag gated — not present in production binary.

**Request:**
```json
{ "code": "AB12-CD34", "amount_idr": 150000 }
```

**Response 200:**
```json
{ "status": "ok", "triggered_for": "AB12-CD34" }
```

---

#### POST `/api/v1/public/bookings` — Phase 6 response shape change

Response now includes QRIS fields instead of Midtrans snap token:

```json
{
  "id": "...", "code": "AB12-CD34", "total_price_idr": 150000,
  "qr_string": "00020101...",
  "qr_image_url": "https://api.qrserver.com/...",
  "qr_expires_at": "2026-04-30T10:30:00+07:00",
  "payment_reference": "ipaymu-trx-uuid",
  "status": "pending_payment"
}
```

Fields `snap_token` and `redirect_url` are empty strings (deprecated).

**Flags for frontends:** `flutter-expert` must update `PaymentScreen` to render `qr_string` via `qr_flutter`. `nextjs-expert` no change (concierge flow unaffected).

---

### 15.3 Tenant — Finance (requires `finance.read`)

#### GET `/api/v1/tenant/finance/balance`

Three-tier balance summary card.

**Response 200:**
```json
{
  "in_process_idr": 500000,
  "ready_to_disburse_idr": 1200000,
  "disbursed_idr": 3500000
}
```

---

#### GET `/api/v1/tenant/finance/transactions`

**Query:** `?status=&from_date=&to_date=&page=1&limit=10`

**Response 200:**
```json
{
  "data": [{
    "id": "...", "booking_id": "...", "provider_reference": "...", "provider": "ipaymu",
    "status": "paid", "expected_amount_idr": 150000, "received_amount_idr": 150000,
    "platform_fee_idr": 7500, "tenant_net_idr": 142500,
    "paid_at": "2026-04-30T10:15:00Z", "settled_at": null, "disbursed_at": null,
    "created_at": "2026-04-30T10:00:00Z"
  }],
  "total": 1, "page": 1, "total_pages": 1
}
```

---

#### GET `/api/v1/tenant/finance/disbursements`

**Query:** `?status=&page=1&limit=10`

**Response 200:** paginated list of `tenant_disbursement` rows.

---

#### GET `/api/v1/tenant/finance/disbursements/:id`

**Response 200:** disbursement detail + contributing transactions array.

---

### 15.4 Platform Admin — Settlement (requires `settlement.reconcile` / `finance.read_all`)

#### POST `/api/v1/admin/settlement/reconcile`

Triggers iPaymu daily settlement report fetch and bulk-marks transactions settled.

**Request:**
```json
{ "date": "2026-04-30" }
```

**Response 200:**
```json
{
  "batch_id": "...", "settled_at": "2026-04-30",
  "transaction_count": 12, "total_amount_idr": 1800000, "mismatch_count": 0
}
```

**Errors:** `400 INVALID_DATE`

---

#### GET `/api/v1/admin/settlement-batches/summary`

Returns aggregate KPIs for the platform-admin "Volume Disetel Minggu Ini" dashboard card.
Cross-reference: platform-admin dashboard design (`docs/DESIGN_FLOWS/platform-admin-dashboard.md` §5.2).

**Auth:** `finance.read_all` permission, platform scope only (same as `ListSettlementBatches`).

**Query parameters:**

| Parameter | Type | Required | Description |
|---|---|---|---|
| `from` | `YYYY-MM-DD` | Yes | Earliest `settled_at` date to include (inclusive). Interpreted as midnight Asia/Jakarta → UTC. |
| `to` | `YYYY-MM-DD` | Yes | Latest `settled_at` date to include (inclusive). Interpreted as end-of-day Asia/Jakarta (23:59:59.999999999) → UTC. |

**Constraints:**
- `to` must not be before `from` (400).
- Date range must not exceed 90 days (400 — dashboard summary, not an export).

**Response `200 OK`:**

```json
{
  "from": "2026-04-26",
  "to": "2026-05-02",
  "batch_count": 12,
  "total_volume_idr": 18450000,
  "total_platform_fee_idr": 922500,
  "total_payout_idr": 17527500
}
```

| Field | Description |
|---|---|
| `batch_count` | Number of distinct `settlement_batch_id` values on `payment_transaction` rows whose `settled_at` falls within `[from, to]` and `status IN ('settled', 'disbursed')`. |
| `total_volume_idr` | `SUM(received_amount_idr)` — gross customer volume. |
| `total_platform_fee_idr` | `SUM(platform_fee_idr)` — total Lustia platform fee (5% flat, ADR 0015 §2.4). |
| `total_payout_idr` | `SUM(tenant_net_idr)` — total amount destined for tenant payout. |

When no transactions match the window all sums are `0` and `batch_count` is `0` — never `null`.

**Errors:**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | `from` or `to` is absent |
| 400 | `INVALID_DATE` | Either date is not valid `YYYY-MM-DD` |
| 400 | `VALIDATION` | `to` is before `from`, or range exceeds 90 days |
| 401 | `UNAUTHORIZED` | Missing or invalid JWT |
| 403 | `INSUFFICIENT_PERMISSION` | Caller lacks `finance.read_all` |

> **Route ordering note:** this route is registered before `GET /admin/settlement-batches/:id` so Gin does not match the literal string `summary` as an `:id` path parameter.

---

#### GET `/api/v1/admin/settlement-batches`

**Query:** `?provider=&page=1&limit=10`

---

#### GET `/api/v1/admin/settlement-batches/:id`

Returns batch detail + transactions + mismatches array.

---

### 15.5 Platform Admin — Payout (requires `disbursement.create` / `disbursement.transfer`)

#### GET `/api/v1/admin/payout/tenant-summary`

**Query:** `?period_start=YYYY-MM-DD&period_end=YYYY-MM-DD`

Phase 6: returns placeholder message; per-tenant balance listing is Phase 7.

---

#### POST `/api/v1/admin/disbursements`

Calculates payout and creates a pending disbursement row.

**Request:**
```json
{ "tenant_id": "...", "period_start": "2026-04-21", "period_end": "2026-04-27" }
```

**Response 201:** full `DisbursementDetail` (same shape as GET /:id).

---

#### GET `/api/v1/admin/disbursements`

**Query:** `?status=&page=1&limit=10`

---

#### GET `/api/v1/admin/disbursements/:id`

---

#### POST `/api/v1/admin/disbursements/:id/processing`

Transitions `pending → processing`. No body required.

**Response 200:** `{ "status": "processing" }`

---

#### POST `/api/v1/admin/disbursements/:id/transferred`

Transitions `processing → transferred`. Also bulk-marks linked payment_transactions as `disbursed` in the same DB transaction (flag #4).

**Request (optional):**
```json
{ "bank_reference": "BCA-TRF-20260430", "notes": "Transfer via BCA m-banking" }
```

**Response 200:** `{ "status": "transferred" }`

---

#### POST `/api/v1/admin/disbursements/:id/failed`

Transitions `processing → failed`.

**Request (optional):**
```json
{ "reason": "Bank rejected: invalid account number" }
```

---

#### POST `/api/v1/admin/disbursements/:id/cancel`

Transitions `pending → cancelled` only.

**Errors:** `409 DISBURSEMENT_NOT_CANCELLABLE` if not pending.

---

### 15.6 Permission matrix (Phase 6)

| Permission | super_admin | tenant_admin | branch_admin |
|---|---|---|---|
| `finance.read` | ✓ | ✓ | ✓ |
| `finance.read_all` | ✓ | — | — |
| `disbursement.create` | ✓ | — | — |
| `disbursement.transfer` | ✓ | — | — |
| `settlement.reconcile` | ✓ | — | — |

---

### 15.7 Error codes (Phase 6)

| Code | HTTP | Meaning |
|---|---|---|
| `PAYMENT_TRANSACTION_NOT_FOUND` | 404 | No payment_transaction for the given booking/reference |
| `PAYMENT_RETRY_NOT_ALLOWED` | 422 | RetryQR attempted on non-pending_payment booking |
| `DISBURSEMENT_NOT_FOUND` | 404 | Disbursement ID not found or access denied |
| `DISBURSEMENT_INVALID_TRANSITION` | 422 | State machine violation |
| `DISBURSEMENT_NOT_CANCELLABLE` | 422 | Cancel attempted on non-pending disbursement |
| `SETTLEMENT_BATCH_NOT_FOUND` | 404 | Settlement batch ID not found |
| `INVALID_DATE` | 400 | date field not in YYYY-MM-DD format |
