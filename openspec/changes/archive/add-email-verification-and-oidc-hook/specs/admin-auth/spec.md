## MODIFIED Requirements

### Requirement: Environment-Supplied Admin Credential With Optional Password

The system SHALL continue to recognize exactly one administrator
identified by `ADMIN_USERNAME`. The administrator's password SHALL
remain sourced from the environment (`ADMIN_PASSWORD`), but the value
SHALL be OPTIONAL: when blank, the system generates a cryptographically
strong random password on startup and prints it to stderr so the
operator can capture it from first-boot logs and persist it to `.env`
for stability across restarts.

Existing behavior preserved: there is still no `admins` table, no admin
registration flow, no admin moderation endpoint. JWT issuance (audience
`admin`) is unchanged. Constant-time password comparison is unchanged.

#### Scenario: ADMIN_PASSWORD set in env

- **GIVEN** the operator has set `ADMIN_PASSWORD=secret` in `.env` or process env
- **WHEN** `config.Load()` runs
- **THEN** `cfg.Admin.Password` SHALL equal `"secret"` verbatim
- **AND** the system SHALL NOT print the bootstrap banner

#### Scenario: ADMIN_PASSWORD blank — auto-generate

- **GIVEN** `ADMIN_USERNAME` is set but `ADMIN_PASSWORD` is unset or empty
- **WHEN** `config.Load()` runs
- **THEN** the system SHALL generate a fresh password using 18 bytes from `crypto/rand` encoded with `base64.RawURLEncoding` (24 URL-safe chars)
- **AND** assign the generated value to `cfg.Admin.Password`
- **AND** write a banner to `os.Stderr` containing the literal text `ADMIN_PASSWORD=<value>` so the value is greppable from log scrapes
- **AND** the banner SHALL advise the operator to save the value into `.env` for stable credentials across restarts

#### Scenario: Generated password is cryptographically strong

- **WHEN** the auto-generation path is taken
- **THEN** the password SHALL be sourced from `crypto/rand` (NOT `math/rand`)
- **AND** SHALL contain at least 128 bits of entropy
- **AND** SHALL use only URL-safe base64 characters `[A-Za-z0-9_-]`

#### Scenario: Validation no longer requires ADMIN_PASSWORD

- **WHEN** `config.validate()` runs
- **THEN** the system SHALL NOT include `ADMIN_PASSWORD` in the missing-required-keys aggregation
- **AND** the existing required keys (`DATABASE_URL`, `JWT_SECRET`, `ADMIN_USERNAME`, OIDC quartet if any are set) SHALL still be enforced

#### Scenario: Restart without saving rotates the password

- **GIVEN** the operator ran the system once without `ADMIN_PASSWORD`, observed the printed value, but did NOT save it
- **WHEN** the process is restarted
- **THEN** a new password SHALL be generated and printed
- **AND** previously issued admin JWTs SHALL remain valid until their natural expiry (signature verification uses `JWT_SECRET`, not the current admin password)

#### Scenario: Login flow unchanged

- **GIVEN** the admin password (set or generated) is `P`
- **WHEN** a client POSTs `/api/admin/auth/login` with `{username: ADMIN_USERNAME, password: P}`
- **THEN** the system SHALL issue an admin JWT with audience `admin` (existing behavior)
- **AND** any wrong password SHALL return HTTP 401 with a generic error (existing behavior)
- **AND** the password comparison SHALL be constant-time (existing behavior)
