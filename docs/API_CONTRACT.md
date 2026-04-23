# API Contract — Lustia Auth Service

_Owned by `go-expert`. Consumed by `nextjs-expert` and `flutter-expert`. Changes require frontend sign-off._

_Last updated: 2026-04-22 (Phase 3 — Tenant Onboarding & Branch Setup)_

---

## 1. Conventions

| Item | Value |
|---|---|
| Base URL | `/api/v1` |
| Auth header | `Authorization: Bearer <access_token>` |
| Content-Type | `application/json` (request + response) |
| Timestamps | RFC 3339 UTC — `2026-04-18T12:00:00Z` |
| Request ID | `X-Request-ID` header — echoed back in response; generated if absent |
| Pagination | Cursor-based: `?cursor=<opaque>&limit=<int>` → `{"data": [...], "next_cursor": "..."}`. Absent `next_cursor` means last page. |
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

The `POST /auth/login` endpoint resolves tenant context via the `tenant_slug` field:

| `tenant_slug` value | Scope resolved |
|---|---|
| Absent / empty | **Rejected** — tenant_slug is required in Phase 2 |
| `"__platform__"` | Super-admin login — user must have `tenant_id IS NULL` |
| Any other slug | Tenant staff — resolves `tenant_id` via `tenant.slug` |

Host-header resolution (`tenant-slug.lustia.example`) is documented but **not implemented in Phase 2**. A future ADR will cover it.

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
  "email": "alice@example.com",
  "password": "s3cur3P@ssw0rd",
  "tenant_slug": "acme-spa"
}
```

| Field | Type | Validation |
|---|---|---|
| `email` | string | required, valid email, max 320 |
| `password` | string | required, min 8, max 128 |
| `tenant_slug` | string | required, max 100. Use `"__platform__"` for super_admin. |

**Response `200 OK`**

```json
{
  "access_token": "<JWT>",
  "refresh_token": "<opaque>",
  "token_type": "Bearer",
  "expires_at": "2026-04-18T12:15:00Z",
  "user": {
    "id": "uuid",
    "tenant_id": "uuid",
    "email": "alice@example.com",
    "full_name": "Alice Smith",
    "phone": "+62812...",
    "avatar_url": "https://...",
    "is_active": true,
    "roles": ["tenant_admin"],
    "branches": ["branch-uuid-1"]
  }
}
```

**Errors**

| Status | Code | When |
|---|---|---|
| 400 | `VALIDATION` | Malformed request body |
| 401 | `INVALID_CREDENTIALS` | Wrong email or password (intentionally vague) |
| 401 | `ACCOUNT_LOCKED` | Too many failed attempts |
| 401 | `ACCOUNT_INACTIVE` | User deactivated |
| 401 | `TENANT_NOT_FOUND` | Slug resolves to nothing (mapped to 401 to prevent enumeration) |
| 401 | `TENANT_INACTIVE` | Tenant status is not `active` |
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
  "full_name": "Bob Jones",
  "phone": "+62...",
  "role_ids": ["role-uuid-1"],
  "branch_ids": ["branch-uuid-1"]
}
```

| Field | Validation |
|---|---|
| `email` | required, valid email, max 320 |
| `full_name` | required, min 1, max 200 |
| `phone` | optional, max 30 |
| `role_ids` | optional, array of valid UUIDs |
| `branch_ids` | optional, array of valid UUIDs |

**Response `201 Created`**

```json
{
  "user": { ... },
  "initial_password": "generated-once-password"
}
```

**Errors**

| Status | Code | When |
|---|---|---|
| 409 | `DUPLICATE_EMAIL` | Email already exists in this tenant |
| 404 | `NOT_FOUND` | A provided role_id or branch_id does not exist |
| 403 | `INSUFFICIENT_PERMISSION` | Missing `user.create` |

---

#### `GET /api/v1/admin/users`

Lists users in the authenticated admin's tenant with optional filters and cursor pagination.

**Required permission:** `user.read`

**Query parameters**

| Param | Type | Description |
|---|---|---|
| `role_id` | UUID | Filter by role assignment |
| `branch_id` | UUID | Filter by branch assignment |
| `is_active` | bool | Filter by active status |
| `cursor` | string | Opaque pagination cursor from previous response |
| `limit` | int | 1–200, default 50 |

**Response `200 OK`**

```json
{
  "data": [ { ...UserProfileResponse }, ... ],
  "next_cursor": "opaque-cursor-string"
}
```

`next_cursor` is absent when there are no more results.

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
  "phone": "+62...",
  "avatar_url": "https://...",
  "is_active": false,
  "role_ids": ["role-uuid-1"],
  "branch_ids": []
}
```

**Response `200 OK`** — updated `UserProfileResponse`

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
| `DUPLICATE_EMAIL` | 409 | `(tenant_id, email)` unique index violated |
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

**Query parameters:** `status=pending|approved|rejected|all` (default `pending`), `cursor`, `limit` (1–200, default 50).

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
  "next_cursor": null
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

**Query parameters:** `status=pending_approval|active|suspended|deactivated|all`, `cursor`, `limit`.

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
  "next_cursor": null
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

**Query:** `status=active|inactive|all`, `cursor`, `limit`.

**Response `200 OK`** — `{ "data": [...BranchResponse], "next_cursor": "..." }`.

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
