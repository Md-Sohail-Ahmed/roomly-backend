# Roomly backend

Roomly is a Go `net/http` API backed by MySQL. It provides employee registration and login, JWT authentication, room and amenity management, room availability, bookings, and admin user controls. This repository contains backend code only.

## Requirements

- Go 1.27 or newer
- MySQL with the Roomly tables created manually in MySQL Workbench
- A least-privilege MySQL account for the API

## Local setup

1. Create `roomly_db` and the `users`, `rooms`, `amenities`, `room_amenities`, and `bookings` tables in MySQL Workbench using your supplied schema or the additive fresh-database reference in [docs/mysql-schema.sql](docs/mysql-schema.sql). Keep the user password column named `password`; the repository maps it to an internal field that is never serialized.
2. Ensure the tables use InnoDB. The booking concurrency guarantee relies on transactional row locks. Keep the supplied foreign keys and unique constraints.
3. Copy `.env.example` to `.env`, set the database connection values, and replace `JWT_SECRET` with a randomly generated value of at least 32 bytes. `DB_TIMEZONE` must remain `UTC`; connections set both the Go parsing location and MySQL session timezone to UTC.
4. Start the API from this directory:

   ```powershell
   go run ./cmd/api
   ```

5. The health endpoint is `GET http://localhost:8080/health`.

`.env` is ignored by Git. Never commit real database credentials or JWT secrets. Production deployments should inject environment variables through the deployment secret manager and use a database account limited to the required tables and operations.

For a local least-privilege API account, create a dedicated MySQL account in Workbench and grant only the operations used by this API (replace the placeholder password yourself):

```sql
CREATE USER 'roomly_app'@'localhost' IDENTIFIED BY 'replace-this-password';
GRANT SELECT, INSERT, UPDATE, DELETE ON roomly_db.* TO 'roomly_app'@'localhost';
```

## First administrator

There is no public admin registration endpoint. First register a normal account with `POST /api/v1/auth/register`. Then run the one-time bootstrap command with that active employee's email:

```powershell
$env:ROOMLY_BOOTSTRAP_ADMIN_EMAIL = "admin@example.com"
go run ./cmd/admin-seed
Remove-Item Env:\ROOMLY_BOOTSTRAP_ADMIN_EMAIL
```

The command promotes the existing account only if no admin account exists. A MySQL named lock serializes concurrent bootstrap attempts. Later role changes and activation changes are available only to admins; the API prevents removal or demotion of the last active admin. The account password remains the password set at registration.

## API and testing

Route contracts, permissions, timestamps, and error formats are in [docs/API.md](docs/API.md). Import the routes into Postman or create requests from that reference. Register and log in first, then set `Authorization: Bearer <token>` for protected routes. Create an employee token to exercise employee routes; promote a separate account for admin routes.

Run checks from the module directory:

```powershell
gofmt -w cmd internal
go test ./...
go vet ./...
```

The current automated suite contains unit tests for service validation, JWT handling, role middleware, ownership, and pagination. Database integration tests require a separately provisioned MySQL database whose name ends in `_test` or `_testing`, with the Roomly schema installed. Set `ROOMLY_TEST_DSN` only for that isolated database to run the concurrent/back-to-back booking, cancellation, capacity, inactive-room, amenity-association, and initial-admin integration tests; the tests refuse other database names and are not run against your development database. For integration tests, create a separate `roomly_test` database and apply the reference schema after changing its `CREATE DATABASE` and `USE` names. Do not point `ROOMLY_TEST_DSN` at `roomly_db`.
