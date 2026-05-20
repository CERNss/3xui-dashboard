# auth-bootstrap

Covers how the single administrator credential is loaded at startup,
including the auto-generation fallback when `ADMIN_PASSWORD` is blank.

## Purpose & boundaries

The dashboard recognizes exactly ONE administrator. There is no admin
registration flow, no `admins` table, no role escalation. The credential
lives in environment configuration (`.env` or process env).

This spec covers the **loading and bootstrapping** of that credential.
Issuing JWTs against it is the concern of `admin-auth`; the SPA login
form is the concern of `unified-login`.

## Configuration surface

Env vars consumed by `internal/config/config.go::Load()`:

| Var | Type | Required | Notes |
|---|---|---|---|
| `ADMIN_USERNAME` | string | Yes | Treated as an email-shaped account string. No format validation on load. |
| `ADMIN_PASSWORD` | string | No | When blank, Load() auto-generates a 24-char URL-safe random value and writes it to stderr (see scenarios). |

`ADMIN_USERNAME` continues to be hard-required (`validate()` aggregates the
missing-key error message). `ADMIN_PASSWORD` is OPTIONAL — its absence
triggers the bootstrap path described below.

## Requirements

### Requirement: Single admin credential from env

The system SHALL recognize exactly one administrator identified by
`ADMIN_USERNAME`, with no database row representing the admin account.

#### Scenario: Admin username loaded from env at startup

- **WHEN** the process starts with `ADMIN_USERNAME` set
- **THEN** `cfg.Admin.Username` SHALL equal that value verbatim
- **AND** no `admins` table or admin row SHALL be created

#### Scenario: Missing admin username fails fast

- **WHEN** the process starts without `ADMIN_USERNAME` set
- **THEN** `config.Load()` SHALL return an aggregated error that names
  `ADMIN_USERNAME` (alongside any other missing required keys)
- **AND** the process SHALL exit before HTTP listeners bind

### Requirement: Admin password is optional with auto-generation

The system SHALL accept a blank `ADMIN_PASSWORD` and bootstrap one,
publishing it to stderr so the operator can capture it in first-boot
logs and persist it into `.env`.

#### Scenario: Operator boots without ADMIN_PASSWORD

- **GIVEN** `ADMIN_USERNAME` is set but `ADMIN_PASSWORD` is unset or empty
- **WHEN** `config.Load()` runs
- **THEN** the system SHALL generate a fresh password using
  `crypto/rand` (18 random bytes → 24 chars of `base64.RawURLEncoding`)
- **AND** assign it to `cfg.Admin.Password` for the rest of the process lifetime
- **AND** write a banner to `os.Stderr` that includes the literal text
  `ADMIN_PASSWORD=<value>` so it is greppable from the log
- **AND** the banner SHALL advise the operator to copy the value into `.env`
  to keep credentials stable across restarts

#### Scenario: ADMIN_PASSWORD present takes precedence

- **GIVEN** `ADMIN_PASSWORD` is set to a non-empty value
- **WHEN** `config.Load()` runs
- **THEN** the system SHALL use the supplied value verbatim
- **AND** SHALL NOT print the bootstrap banner

#### Scenario: Generated password is suitable as a bootstrap secret

- **GIVEN** the auto-generation path was taken
- **THEN** the generated password SHALL be drawn from `crypto/rand`
  (not `math/rand`)
- **AND** SHALL have at least 128 bits of entropy (18 bytes ≥ 144 bits)
- **AND** SHALL use only URL-safe characters (`[A-Za-z0-9_-]`)

### Requirement: Restart without persistence rotates the password

Auto-generation is NOT persisted to disk — the next process restart
generates a new password unless the operator has saved it into `.env`.

#### Scenario: Restart without saving the value

- **GIVEN** the operator started without `ADMIN_PASSWORD`, observed the
  printed value, but did NOT copy it into `.env`
- **WHEN** the process is restarted
- **THEN** a different password SHALL be generated and printed
- **AND** existing admin JWTs that were issued under the previous password
  SHALL remain valid until their natural expiry (`auth.AccessTokenTTL`),
  because token verification depends only on `JWT_SECRET`, not on the
  current admin password

## Implementation notes

- `internal/config/config.go::generateAdminPassword()` — entropy source +
  encoding. Returns `(string, error)`; the only error path is `rand.Read`
  failure, which is treated as fatal startup error.
- The banner is intentionally noisy (multiple `=` lines) so operators
  scanning logs don't miss it. We do not log via slog because the slog
  handler is not yet configured at `config.Load()` time.
- `validate()` no longer includes `ADMIN_PASSWORD` in its required-key
  list (per `internal/config/config_test.go::TestLoad_FailsOnMissingRequired`).
- A second test
  (`TestLoad_GeneratesAdminPasswordWhenBlank`) covers the auto-gen path.

## Out of scope

- Password rotation tooling (no admin endpoint changes the password — it
  is sourced from env exclusively).
- Multiple admins or admin RBAC.
- Storing the auto-generated value back to disk on the operator's behalf
  (would surprise infra-as-code workflows where `.env` is generated).
