# API Contract

_Owned by `go-expert`. Consumed by `nextjs-expert` and `flutter-expert`. Changes require frontend sign-off._

## Conventions
- Base URL: `/api/v1`
- Auth: _TBD (e.g., `Authorization: Bearer <access_token>`)_
- Content-Type: `application/json`
- Error envelope:
  ```json
  { "error": { "code": "string", "message": "string", "details": {} } }
  ```
- Pagination: cursor-based — `?limit=50&cursor=...` → response `{ "data": [...], "next_cursor": "..." }`
- Timestamps: RFC 3339 UTC.

## Endpoints

### `POST /api/v1/example`
_Short description._

**Request**
```json
{ "field": "string" }
```

**Response `201 Created`**
```json
{ "id": "uuid", "field": "string", "created_at": "2026-04-18T10:00:00Z" }
```

**Errors**
| Status | Code | When |
| --- | --- | --- |
| 400 | `VALIDATION` | Invalid payload |

## Error code catalog
| Code | HTTP | Meaning |
| --- | --- | --- |
| `VALIDATION` | 400 | Request failed schema validation |
| `UNAUTHENTICATED` | 401 | Missing or invalid credentials |
| `FORBIDDEN` | 403 | Authenticated but not authorized |
| `NOT_FOUND` | 404 | Resource does not exist or not visible |
| `CONFLICT` | 409 | State conflict (e.g., duplicate) |
| `RATE_LIMITED` | 429 | Throttled |
| `INTERNAL` | 500 | Unexpected server error |

## Open questions
-
