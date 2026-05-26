# email-verification

Short-lived 6-digit email codes used to gate portal registration (and
future flows like password reset).

## Purpose & boundaries

Before a self-serve registration completes, the user must prove ownership
of the email they're registering with. This module owns:

- The code lifecycle: generate, send, verify, consume.
- The storage schema and rate-limit policy.
- The HTTP surface (`POST /api/user/auth/send-code`) and the
  `register` extension that consumes the code.
- The dev-mode fallback when SMTP is not configured.

Actual SMTP transport is delegated to `mailer`. User-row creation is
delegated to `user-accounts`.

## Storage

The baseline schema includes:

```sql
CREATE TABLE email_verification_codes (
    id              BIGSERIAL PRIMARY KEY,
    email           TEXT        NOT NULL,
    purpose         TEXT        NOT NULL,        -- 'register' (more later)
    code_hash       TEXT        NOT NULL,        -- sha256(code) hex
    expires_at      TIMESTAMPTZ NOT NULL,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at     TIMESTAMPTZ,
    attempts        INT         NOT NULL DEFAULT 0
);
CREATE INDEX email_verification_codes_active
  ON email_verification_codes (email, purpose, sent_at DESC);
CREATE INDEX email_verification_codes_unconsumed
  ON email_verification_codes (email, purpose, expires_at)
  WHERE consumed_at IS NULL;
```

Rows are kept after consumption for audit; the partial index ensures the
"find unconsumed unexpired" lookup is O(1) per email.

## Policy constants

Defined in `internal/service/verification/service.go`:

| Constant | Value | Why |
|---|---|---|
| `codeLength` | 6 decimal digits | Standard usability; brute-forceable space is 1e6 but `maxAttempts=5` per row + 10-min TTL keeps it negligible. |
| `codeTTL` | 10 minutes | Long enough to alt-tab to email + paste; short enough that a leaked code is short-lived. |
| `resendCooldown` | 60 seconds | Per (email, purpose). Prevents email-flood abuse. |
| `maxAttempts` | 5 | Per-row attempt cap. Burnt rows force a fresh `SendCode`. |

## Requirements

### Requirement: SendCode delivers a fresh single-use code

The system SHALL accept `POST /api/user/auth/send-code` with `{email}`,
generate a fresh 6-digit code, store its hash, and dispatch the email.

#### Scenario: First-time send for an email

- **WHEN** a client POSTs `{"email":"new@example.com"}` to `/api/user/auth/send-code`
- **THEN** the system SHALL generate a 6-digit code via `crypto/rand`
- **AND** insert a row with `sha256(code)` as `code_hash`, `expires_at = now()+10m`, and `purpose='register'`
- **AND** invoke `mailer.Send` with subject "【3xui Central】注册验证码" and a UTF-8 body containing the code + TTL
- **AND** respond `204 No Content`

#### Scenario: Second send within cooldown

- **GIVEN** a row exists for `(email='x@y', purpose='register')` with `sent_at` < 60s ago
- **WHEN** another send is requested for the same email + purpose
- **THEN** the system SHALL respond `429 Too Many Requests` with body `{"error":"请稍等再发，验证码 60 秒内只能发一次"}`
- **AND** SHALL NOT call the mailer
- **AND** SHALL NOT insert a new row

#### Scenario: Send after cooldown

- **GIVEN** the most recent row's `sent_at` is more than 60s old
- **WHEN** a new send is requested
- **THEN** the system SHALL insert a new row (the old row is left intact)
- **AND** subsequent Consume calls SHALL match against the NEWEST unconsumed unexpired row (`ORDER BY sent_at DESC LIMIT 1`)

### Requirement: Codes are hashed at rest

The system SHALL NOT store plaintext codes in the database.

#### Scenario: Code stored as SHA-256 hex

- **WHEN** SendCode persists a row
- **THEN** the `code_hash` column SHALL be `hex(sha256(code))`
- **AND** the plaintext code SHALL exist only in the email body and the in-process variable during request handling

Rationale for SHA-256 (not bcrypt): codes are 6 digits with a 10-min
TTL, so offline brute-forcing isn't a meaningful threat — by the time
an attacker computes the hash space (negligible with 6 digits anyway),
the code is already expired. bcrypt's slow-hash purpose is misapplied.

### Requirement: Consume validates and burns the code

The system SHALL expose `Consume(email, code, purpose)` to service callers,
which validates a presented code and flips `consumed_at` atomically.

#### Scenario: Successful consume

- **GIVEN** an unconsumed unexpired row exists for `(email, purpose)`
- **AND** `sha256(code) == row.code_hash`
- **WHEN** Consume is called
- **THEN** within a single DB transaction the system SHALL set `consumed_at = now()` on that row
- **AND** return `nil`

#### Scenario: No matching row

- **WHEN** Consume is called with `(email, purpose)` having no unconsumed unexpired row
- **THEN** the system SHALL return `ErrCodeNotFound`

#### Scenario: Code mismatch increments attempts

- **GIVEN** an unconsumed unexpired row exists but `sha256(code) != row.code_hash`
- **WHEN** Consume runs
- **THEN** the system SHALL increment `attempts` on that row
- **AND** return `ErrCodeMismatch`

#### Scenario: Burnt-out row

- **GIVEN** `attempts >= 5` on the latest unconsumed unexpired row
- **WHEN** Consume runs (regardless of whether the code is correct)
- **THEN** the system SHALL return `ErrTooManyAttempts`
- **AND** the caller (handler) SHALL surface `429` with a hint to request a fresh code

#### Scenario: Expired row

- **GIVEN** the latest unconsumed row's `expires_at < now()`
- **WHEN** Consume runs
- **THEN** the system SHALL return `ErrCodeExpired`

#### Scenario: Replay of consumed code

- **GIVEN** a row's `consumed_at IS NOT NULL`
- **WHEN** Consume runs with that row's code
- **THEN** the system SHALL find no eligible row (`WHERE consumed_at IS NULL`) and return `ErrCodeNotFound`

### Requirement: Register endpoint enforces a valid code in production

When SMTP is configured, the system SHALL require `code` on register
calls and reject any registration that fails Consume.

#### Scenario: Register without code (SMTP enabled)

- **GIVEN** `cfg.SMTP.Enabled() == true`
- **WHEN** a client POSTs to `/api/user/auth/register` with no `code` field
- **THEN** the system SHALL respond `400 Bad Request` with `{"error":"缺少邮箱验证码"}`
- **AND** SHALL NOT create a user row

#### Scenario: Register with wrong code

- **WHEN** the code in the request does not match
- **THEN** the system SHALL respond `400` with `{"error":"验证码不正确"}`
- **AND** SHALL NOT create a user row
- **AND** the attempt counter on that code row SHALL be incremented

#### Scenario: Register with valid code

- **WHEN** the code matches and is unexpired/unconsumed
- **THEN** the system SHALL Consume the code first
- **AND** THEN create the user via `usersvc.Service.Register`
- **AND** auto-issue a user JWT (existing behavior)

### Requirement: Dev mode permits registration without SMTP

When SMTP is not configured, the system SHALL skip code verification on
register so dev workflows are not blocked by missing mail infrastructure.

#### Scenario: Register without code (SMTP disabled)

- **GIVEN** `cfg.SMTP.Enabled() == false`
- **WHEN** a client POSTs to `/api/user/auth/register` with `code` omitted
- **THEN** the system SHALL skip Consume entirely
- **AND** create the user as if registration were unprotected
- **AND** the SendCode endpoint, if called, SHALL log the code to stderr via the mailer's no-op fallback (so manual e2e testing can still complete the flow)

#### Scenario: SendCode in dev mode

- **GIVEN** `cfg.SMTP.Enabled() == false`
- **WHEN** SendCode is called
- **THEN** the system SHALL still generate, store, and "send" the code
- **AND** the mailer SHALL log at INFO level: subject, to, body (containing the code)
- **AND** the endpoint SHALL respond `204` so the SPA's UX is unchanged

## Frontend behavior

`frontend/src/views/Login.vue` in register mode:

- Calls `portalAuthApi.sendCode(email)` when user clicks "发送验证码"
- Starts a 60-second countdown after a successful send; the send button
  is disabled until the countdown expires.
- Validates `code.length === 6` client-side before calling register.
- On register failure (e.g. wrong code), surfaces the backend error verbatim.

## Out of scope

- SMS / TOTP verification.
- Password-reset flow (would use `PurposeReset` value in the same table).
- Vendor captcha integration (deferred — registration's defense in v1 is
  email verification + the 60s send rate limit).
