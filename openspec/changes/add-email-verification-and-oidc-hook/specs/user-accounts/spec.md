## MODIFIED Requirements

### Requirement: Self-Serve Registration Requires Email Verification When SMTP Is Enabled

The system SHALL extend the existing portal registration flow to require
a 6-digit email verification code when SMTP delivery is configured. The
code SHALL be obtained via `POST /api/user/auth/send-code` and consumed
in the same request as registration. When SMTP is disabled (dev mode),
the code field SHALL be optional so developer workflows are not blocked
by missing mail infrastructure.

Existing behavior preserved: registration is still gated by the
`public_registration_enabled` setting and the email-domain allowlist;
`users.registered` events are still emitted on success.

#### Scenario: Register with valid code (SMTP enabled)

- **GIVEN** `cfg.SMTP.Enabled() == true`
- **AND** the user has previously called `POST /auth/send-code` and received an email containing a 6-digit code
- **WHEN** the SPA POSTs `/api/user/auth/register` with `{email, password, code}`
- **AND** the code matches the latest unconsumed unexpired row
- **THEN** the system SHALL consume the code (`consumed_at = now()`)
- **AND** create the user via `usersvc.Register` with the existing validation chain
- **AND** issue a portal JWT in the response (`tokenResponse` with `token`, `expires_at`, `user_id`, `email`)

#### Scenario: Register without code (SMTP enabled)

- **GIVEN** `cfg.SMTP.Enabled() == true`
- **WHEN** the request omits the `code` field or sends an empty `code`
- **THEN** the system SHALL respond HTTP 400 with `{"error":"缺少邮箱验证码"}`
- **AND** SHALL NOT create a user row
- **AND** SHALL NOT call `usersvc.Register`

#### Scenario: Register with wrong code (SMTP enabled)

- **WHEN** `req.Code` does not match any active row
- **THEN** the system SHALL respond HTTP 400 with `{"error":"验证码不正确"}`
- **AND** the verification row's `attempts` counter SHALL be incremented

#### Scenario: Register with expired code (SMTP enabled)

- **WHEN** the latest unconsumed row for the email has `expires_at < now()`
- **THEN** the system SHALL respond HTTP 400 with `{"error":"验证码已过期，请重新发送"}`

#### Scenario: Too many wrong attempts on a code

- **GIVEN** the latest unconsumed row's `attempts >= 5`
- **WHEN** Register is invoked (regardless of code correctness)
- **THEN** the system SHALL respond HTTP 429 with `{"error":"验证次数过多，请重新发送验证码"}`

#### Scenario: Register in dev mode (SMTP disabled)

- **GIVEN** `cfg.SMTP.Enabled() == false`
- **WHEN** a client POSTs `/api/user/auth/register` with `code` omitted or any string value
- **THEN** the system SHALL skip Consume entirely
- **AND** create the user via the existing `usersvc.Register` validation chain
- **AND** issue a portal JWT in the response

#### Scenario: `RegisterInput` shape

- **WHEN** the request body deserializes into `service/user.RegisterInput`
- **THEN** the struct SHALL contain `Email string`, `Password string`, and `Code string` JSON fields
- **AND** existing callers (e.g. handler-level wiring) SHALL pass the value through unchanged
