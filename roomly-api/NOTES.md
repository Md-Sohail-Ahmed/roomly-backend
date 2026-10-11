# Implementation notes and assessment deviations

- Database setup remains manual through MySQL Workbench. This project intentionally has no migration framework and does not create, drop, or recreate tables at startup.
- Queries target the supplied schema, including `users.password`, `rooms`, `amenities`, `room_amenities`, and `bookings`. Public registration always inserts the `employee` role.
- Booking timestamps use RFC3339 strings with an explicit timezone offset at the API boundary and are normalized to UTC. The MySQL `DATETIME` columns store UTC wall time; configure `DB_TIMEZONE=UTC`.
- Booking creates start a `READ COMMITTED` transaction and lock the room row with `SELECT ... FOR UPDATE` before checking active overlaps and inserting. Every booking creation path must use this lock. This serializes competing creates for one room even when there is no existing booking row. The supplied `(room_id, start_time, end_time)` index supports the overlap lookup. Cancelled bookings are ignored.
- Cancellation is idempotent: repeating cancellation returns the booking in its `cancelled` state. Admins may view and cancel any booking; employees may view or cancel only their own. Employees create their own bookings; request bodies do not accept `user_id`.
- Room availability is `GET /api/v1/rooms/{id}/availability?start_time=...&end_time=...`. The overlap rule is `existing.start_time < requested_end AND existing.end_time > requested_start` for `status='active'`.
- Amenity deletion follows the schema's `ON DELETE CASCADE` behavior for `room_amenities`. Foreign keys reject invalid associations.
- Admin bootstrap promotes an already registered active employee only when no admin exists. This is an operational one-time command, not a public route.
- MySQL integration tests cover simultaneous and back-to-back booking requests, cancellation availability, capacity/inactive rooms, amenity associations, and initial-admin/last-admin behavior. They are skipped unless `ROOMLY_TEST_DSN` points to an isolated database ending in `_test` or `_testing`. No isolated DSN is configured in this environment, so the SQL-level tests were not executed here. Unit tests do not prove the MySQL locking behavior.
- Manual schema review: the supplied schema already contains the booking lookup and user lookup indexes used by the API. No additive SQL is currently required.
