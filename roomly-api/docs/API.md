# Roomly API

Base path: `/api/v1`. Requests with JSON bodies must use `Content-Type: application/json`; unknown fields and trailing JSON values are rejected. Request bodies are limited to 1 MiB.

Protected requests use `Authorization: Bearer <JWT>`. A token identifies an active user; a missing, malformed, invalid, or expired token returns `401`. A verified identity without the needed role returns `403`. Errors use `{"error":"message"}`. Database details are logged server-side and are not returned to clients.

## Authentication

| Method and path | Access | Success |
|---|---|---|
| `POST /auth/register` | Public | `201`, employee user |
| `POST /auth/login` | Public | `200`, `{ "token": "...", "user": {...} }` |

Registration body: `{ "name": "Riya", "email": "riya@example.com", "password": "..." }`. Email is trimmed and lowercased. Password length is 8–72 bytes. Public registration cannot set roles. Duplicate email returns `409`; invalid values return `400`.

Login uses the same email/password fields. Wrong credentials and inactive users both return `401` with a generic message. JWTs use HS256, issuer `roomly-api`, user ID in `sub`, role, `iat`, and a 24-hour `exp`. Password data is not in the claims or JSON response.

## Rooms

| Method and path | Access | Success |
|---|---|---|
| `GET /rooms?page=1&limit=20&location=...&min_capacity=...` | Employee or admin | `200`, paginated `{data,total,page,limit}` |
| `GET /rooms/{id}` | Employee or admin | `200`, room with amenities |
| `POST /rooms` | Admin | `201` |
| `PUT /rooms/{id}` | Admin | `200` |
| `PATCH /rooms/{id}/status` | Admin | `200` |
| `GET /rooms/{id}/availability?start_time=...&end_time=...` | Employee or admin | `200`, `{room_id,start_time,end_time,available}` |

Room create/update body: `{ "name":"Cedar", "capacity":8, "location":"Floor 2" }`. Status body: `{ "is_active":false }`. Names and locations are required and bounded by the schema; capacity must be positive. List `limit` is 1–100. Location uses a parameterized contains match.

## Amenities

| Method and path | Access | Success |
|---|---|---|
| `GET /amenities?page=1&limit=20` | Employee or admin | `200`, paginated list |
| `GET /amenities/{id}` | Employee or admin | `200` |
| `POST /amenities` | Admin | `201` |
| `PUT /amenities/{id}` | Admin | `200` |
| `DELETE /amenities/{id}` | Admin | `204`; schema cascades room links |
| `GET /rooms/{id}/amenities` | Employee or admin | `200`, list |
| `POST /rooms/{id}/amenities/{amenityID}` | Admin | `204` |
| `DELETE /rooms/{id}/amenities/{amenityID}` | Admin | `204` |

Create/update body: `{ "name":"Projector", "description":"HDMI display" }`; description may be `null` or omitted. Duplicate amenity or duplicate association returns `409`; missing referenced IDs return `404`.

## Bookings

| Method and path | Access | Success |
|---|---|---|
| `POST /bookings` | Employee | `201` |
| `GET /bookings` | Admin | `200`, paginated; optional `status=active|cancelled` |
| `GET /bookings/mine` | Employee | `200`, paginated own bookings |
| `GET /bookings/{id}` | Owner or admin | `200` |
| `POST /bookings/{id}/cancel` | Owner or admin | `200`, cancelled booking |

Create body: `{ "room_id":1, "title":"Planning", "attendee_count":5, "start_time":"2030-05-01T09:00:00Z", "end_time":"2030-05-01T10:00:00Z" }`. Do not send `user_id`; ownership comes from the token. Times must be RFC3339 with a timezone (`Z` or an explicit offset), converted to UTC, and stored as UTC in MySQL `DATETIME`. Starts must be in the future; duration is 15 minutes–4 hours; attendees must fit the active room capacity. Inactive room, capacity, and overlap conflicts return `409`. Invalid times return `400`.

Overlap uses `existing.start_time < requested_end AND existing.end_time > requested_start`, with active bookings only. Create transactions lock the room row before checking overlap, so concurrent creates for the same room serialize. The availability endpoint reports a point-in-time result and does not reserve the interval.

Repeated cancellation is idempotent and returns the already-cancelled booking. Employees cannot view or cancel another user's booking; an ownership denial returns `403`. Admins can list, view, and cancel bookings for all users.

## Admin users

| Method and path | Access | Success |
|---|---|---|
| `GET /users?page=1&limit=20` | Admin | `200`, paginated list |
| `GET /users/{id}` | Admin | `200` |
| `PATCH /users/{id}/status` | Admin | `200` |
| `PATCH /users/{id}/role` | Admin | `200` |

Status body: `{ "is_active":true }`. Role body: `{ "role":"employee" }` or `{ "role":"admin" }`. The API blocks deactivation/demotion of the last active admin with `409`. There is no user deletion endpoint because bookings reference users. Bootstrap the first admin with `go run ./cmd/admin-seed` after setting `ROOMLY_BOOTSTRAP_ADMIN_EMAIL` to an active registered employee.

## Common status codes

- `200` successful read/update; `201` created; `204` no-content success.
- `400` invalid request, ID, timestamp, or validation; `401` unauthenticated; `403` insufficient role/ownership.
- `404` unknown resource; `409` duplicate, inactive-room, capacity, overlap, or protected-last-admin conflict; `415` wrong/missing JSON content type; `500` unexpected server error.
