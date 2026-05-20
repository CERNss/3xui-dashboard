# migrations

The versioned SQL migration set + golang-migrate runner that applies
them at startup.

## Purpose & boundaries

Migrations are flat-file SQL (`migrations/NNNN_<slug>.up.sql` +
`.down.sql`) embedded into the Go binary via
`migrations/embed.go::go:embed`. The runner is golang-migrate with
the `iofs` driver. Pure SQL — no GORM AutoMigrate at startup.

Adjacent: every module that touches the schema cites its migration
filename (e.g. `email-verification` references
`0004_email_verification_codes.up.sql`).

## Current migrations

| Version | File | Purpose |
|---|---|---|
| 0001 | `0001_init.up.sql` | Initial tables: `users`, `nodes`, `client_ownerships`, `traffic_samples`, `plans`, `orders`, `balance_logs`, `webhooks`, `webhook_deliveries`, `settings`. Includes partial unique indexes (`users(LOWER(email)) WHERE email IS NOT NULL`, `users(oidc_subject) WHERE oidc_subject IS NOT NULL`, `users(sub_id)`, `client_ownerships(node_id, inbound_tag, client_email)`, `orders(idempotency_key)`) and traffic-query indexes (`traffic_samples(node_id, taken_at)`, partial client/inbound). |
| 0002 | `0002_node_scheme.up.sql` | Adds `nodes.scheme` (`http` / `https`) with default `https`. |
| 0003 | `0003_webhook_retry.up.sql` | Adds `webhook_deliveries.next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now()` + partial index `webhook_deliveries_due ON (status, next_attempt_at) WHERE status='pending'` so the retry job's lookup is O(due rows). |
| 0004 | `0004_email_verification_codes.up.sql` | Adds `email_verification_codes` table for the `email-verification` flow. Two indexes: `..._active (email, purpose, sent_at DESC)` for the 60s cooldown lookup, and partial `..._unconsumed (email, purpose, expires_at) WHERE consumed_at IS NULL` for Consume. |

## Requirements

### Requirement: Migrations Are Pure SQL + Embedded

The system SHALL persist every schema change as a numbered SQL file
pair under `backend/migrations/`, and SHALL embed them into the
binary at compile time via `migrations/embed.go`.

#### Scenario: New migration follows the naming convention

- **WHEN** a new schema change ships
- **THEN** the change SHALL add two files: `NNNN_<descriptive-slug>.up.sql` and `NNNN_<descriptive-slug>.down.sql`
- **AND** `NNNN` SHALL be the next free integer (zero-padded to four digits)
- **AND** the `.up.sql` SHALL include a comment block explaining the schema's intent (the *why*, not the *what*)

#### Scenario: Migrations embedded for distribution

- **WHEN** the binary is built
- **THEN** every `.sql` file under `migrations/` SHALL be embedded via the `go:embed` directive in `migrations/embed.go`
- **AND** the running binary SHALL NOT require the source `.sql` files to be present on disk at deploy time

### Requirement: Startup Migration Runner

The system SHALL apply pending migrations on startup when
`DB_MIGRATE_ON_BOOT=true`, gated by a DB-connection retry loop for
docker-compose race tolerance.

#### Scenario: Fresh database

- **GIVEN** an empty Postgres database
- **WHEN** the dashboard starts with `DB_MIGRATE_ON_BOOT=true`
- **THEN** `repository.MigrateUp` SHALL apply every migration in order
- **AND** record applied versions in the `schema_migrations` table managed by golang-migrate

#### Scenario: Partial database

- **GIVEN** a database already at version 2
- **WHEN** the dashboard with versions 0001-0004 in the embed starts
- **THEN** `MigrateUp` SHALL apply 0003 and 0004 only
- **AND** SHALL NOT re-run 0001 / 0002

#### Scenario: Up-to-date database is a no-op

- **GIVEN** the database is already at the latest version
- **WHEN** `MigrateUp` runs
- **THEN** it SHALL return `migrate.ErrNoChange` internally and log "migrations: up-to-date" at INFO level
- **AND** SHALL NOT error out

#### Scenario: DB_MIGRATE_ON_BOOT=false skips the runner

- **WHEN** the env var is false
- **THEN** the dashboard SHALL NOT run migrations on startup
- **AND** SHALL assume migrations are applied by a separate job (e.g. a CI step)
- **AND** SHALL still start serving requests if the schema is current; if not, runtime queries SHALL fail with a clear "table not found" error

### Requirement: Down Migration Symmetry

The system SHALL ship a `.down.sql` for every `.up.sql`. Down
migrations SHALL be lossless reverses where possible (`DROP TABLE`,
`ALTER TABLE DROP COLUMN`); pure index removals are also acceptable.

#### Scenario: Each up has a matching down

- **WHEN** a developer reviews the migrations directory
- **THEN** every `NNNN_*.up.sql` SHALL have a sibling `NNNN_*.down.sql`
- **AND** the down SHALL undo the schema effect of the up (data loss is acceptable on down — we only run `down` in development)

### Requirement: Connection Retry For Docker-Compose Race

The system SHALL retry the initial DB connection with backoff so a
slow-starting Postgres container doesn't crash the dashboard process.

#### Scenario: Postgres not ready yet

- **WHEN** `repository.Open` is called and the DB refuses connections (still booting)
- **THEN** the open SHALL retry every ~500ms, capped at 20 attempts (~10s)
- **AND** if Postgres becomes ready within that window, the open SHALL succeed
- **AND** if not, the dashboard process SHALL exit with a clear "could not connect to DB" error

## Out of scope

- Online schema migrations (every change here is offline-safe).
- Per-tenant schemas / multi-tenancy.
- Automatic rollback on a failed migration (golang-migrate's behavior
  on partial failure is the operator's responsibility to recover from
  — the schema_migrations row marks "dirty" and they must investigate).
