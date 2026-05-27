# migrations

The versioned SQL migration set + golang-migrate runner that applies
them at startup.

## Purpose & boundaries

Migrations are flat-file SQL (`migrations/NNNN_<slug>.up.sql` +
`.down.sql`) embedded into the Go binary via
`migrations/embed.go::go:embed`. The runner is golang-migrate with
the `iofs` driver. Pure SQL — no GORM AutoMigrate at startup.

Because the project has not been launched yet, pre-launch schema work
is collapsed into `0001_init`. Once a deployed database exists, future
schema changes should add a new numbered pair instead of editing
already-applied SQL.

## Current migrations

| Version | File | Purpose |
|---|---|---|
| 0001 | `0001_init.up.sql` | Baseline schema for the current app: users and OIDC identities, nodes, client ownerships, traffic samples, provisioning pools, plans, orders/payment columns, balance logs, webhooks/deliveries with persistent retry, email verification codes, notification log, WireGuard peers, admin actions, and settings. |

## Requirements

### Requirement: Migrations Are Pure SQL + Embedded

The system SHALL persist schema state as numbered SQL file pairs under
`backend/migrations/`, and SHALL embed them into the binary at compile
time via `migrations/embed.go`.

#### Scenario: New migration follows the naming convention

- **GIVEN** the project has a deployed database that cannot be recreated from scratch
- **WHEN** a new schema change ships
- **THEN** the change SHALL add two files: `NNNN_<descriptive-slug>.up.sql` and `NNNN_<descriptive-slug>.down.sql`
- **AND** `NNNN` SHALL be the next free integer (zero-padded to four digits)
- **AND** the `.up.sql` SHALL include a comment block explaining the schema's intent (the *why*, not the *what*)

#### Scenario: Pre-launch schema change updates the baseline

- **GIVEN** the project has not been formally launched
- **WHEN** a schema change is required
- **THEN** the change MAY update `0001_init.up.sql` and `0001_init.down.sql` directly
- **AND** it SHALL keep the baseline internally consistent for fresh databases

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
- **WHEN** the dashboard with versions 0001-0004 in the embed starts after launch
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
