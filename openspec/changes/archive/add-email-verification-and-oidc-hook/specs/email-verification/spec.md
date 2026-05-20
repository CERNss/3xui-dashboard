## ADDED Requirements

### Requirement: Email Verification Codes

The system SHALL issue and validate short-lived 6-digit codes to prove
email ownership for self-serve portal flows. Codes SHALL be scoped by
purpose so a code generated for one flow cannot be replayed against
another.

#### Scenario: SendCode issues a 6-digit code

- **WHEN** a client POSTs `/api/user/auth/send-code` with `{"email":"x@y.z"}`
- **AND** SMTP delivery is configured
- **THEN** the system SHALL generate a 6-digit decimal code via `crypto/rand`
- **AND** insert a row into `email_verification_codes` with `code_hash = sha256_hex(code)`, `purpose = 'register'`, `expires_at = now() + 10 minutes`
- **AND** dispatch an email to the address with the code in the body
- **AND** return HTTP 204 No Content

#### Scenario: Code is hashed at rest

- **WHEN** the system stores a verification row
- **THEN** the row's `code_hash` column SHALL contain the hex SHA-256 digest of the code
- **AND** the plaintext code SHALL NOT be persisted to any database column

#### Scenario: Resend within 60 seconds is rate-limited

- **GIVEN** a row for `(email, purpose)` with `sent_at` ≤ 60 seconds ago
- **WHEN** the same email requests a fresh code via SendCode
- **THEN** the system SHALL respond HTTP 429
- **AND** SHALL NOT insert a new row
- **AND** SHALL NOT invoke the mailer

#### Scenario: Resend after cooldown allowed

- **GIVEN** the most recent send for `(email, purpose)` is older than 60 seconds
- **WHEN** SendCode is invoked
- **THEN** the system SHALL insert a new row and dispatch the new code
- **AND** Consume SHALL match the newest unconsumed unexpired row when called subsequently

### Requirement: Code Consumption Is Single-Use

The system SHALL validate a presented code against the most recent
unconsumed, unexpired row for `(email, purpose)`, mark the row as
consumed on success, and reject repeat use.

#### Scenario: Successful consume

- **GIVEN** an unconsumed, unexpired row exists with `code_hash = sha256_hex(code)`
- **WHEN** Consume is invoked with the matching code
- **THEN** within a single DB transaction the system SHALL set `consumed_at = now()` on the row
- **AND** return success
- **AND** subsequent Consume calls with the same code SHALL fail (no eligible unconsumed row remains)

#### Scenario: Wrong code increments attempts

- **GIVEN** an unconsumed unexpired row exists
- **WHEN** Consume is invoked with a non-matching code
- **THEN** the system SHALL increment the row's `attempts` column
- **AND** return a mismatch error
- **AND** SHALL NOT mark the row consumed

#### Scenario: Burnt-out row after maxAttempts

- **GIVEN** the latest unconsumed unexpired row has `attempts >= 5`
- **WHEN** Consume is invoked (even with the correct code)
- **THEN** the system SHALL return a too-many-attempts error
- **AND** the user SHALL be expected to request a fresh code

#### Scenario: Expired code

- **GIVEN** the latest unconsumed row has `expires_at < now()`
- **WHEN** Consume runs
- **THEN** the system SHALL return an expired error

#### Scenario: No active code for email

- **WHEN** Consume runs and no unconsumed unexpired row exists for `(email, purpose)`
- **THEN** the system SHALL return a not-found error

### Requirement: Registration Requires Code When SMTP Is Enabled

When `cfg.SMTP.Enabled() == true`, the system SHALL enforce a valid
verification code on every self-serve registration, before any user
row is created.

#### Scenario: Register without code (SMTP enabled)

- **GIVEN** `cfg.SMTP.Enabled() == true`
- **WHEN** `POST /api/user/auth/register` arrives without a `code` field
- **THEN** the system SHALL respond HTTP 400 with body `{"error":"缺少邮箱验证码"}`
- **AND** SHALL NOT create a user row
- **AND** SHALL NOT invoke `usersvc.Register`

#### Scenario: Register with mismatched code

- **GIVEN** SMTP enabled and `req.Code` does not match the stored hash
- **WHEN** Register runs
- **THEN** the system SHALL respond HTTP 400 with `{"error":"验证码不正确"}`
- **AND** the row's `attempts` SHALL be incremented (so 5 wrong codes burn it)

#### Scenario: Register with valid code

- **GIVEN** SMTP enabled and `req.Code` matches the latest unconsumed unexpired row
- **WHEN** Register runs
- **THEN** the system SHALL Consume the code (flipping `consumed_at`)
- **THEN** invoke `usersvc.Register` to create the user
- **THEN** auto-issue a user JWT (existing behavior)

#### Scenario: Register without code (SMTP disabled, dev mode)

- **GIVEN** `cfg.SMTP.Enabled() == false`
- **WHEN** a client POSTs `/api/user/auth/register` with `code` omitted
- **THEN** the system SHALL skip Consume entirely
- **AND** create the user via `usersvc.Register`
- **AND** the mailer SHALL log any pending verification codes to stderr instead of mailing (for manual e2e testing without real SMTP)
