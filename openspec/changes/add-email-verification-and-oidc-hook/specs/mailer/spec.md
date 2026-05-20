## ADDED Requirements

### Requirement: Stdlib-Only SMTP Wrapper

The system SHALL provide a `mailer.Mailer` type exposing `Send(to,
subject, body) error` that delivers UTF-8 plain-text email when SMTP is
configured, and falls back to a structured log otherwise. The
implementation SHALL use stdlib only — `net/smtp`, `crypto/tls`,
`encoding/base64`. No external mail SDK.

#### Scenario: Send via port 587 uses STARTTLS

- **GIVEN** `SMTP_PORT=587` and credentials are configured
- **WHEN** `Send(to, subject, body)` is invoked
- **THEN** the mailer SHALL call `smtp.SendMail(addr, smtp.PlainAuth(...), from, []string{to}, msg)`
- **AND** the stdlib SHALL negotiate STARTTLS automatically when the server advertises it

#### Scenario: Send via port 465 uses implicit TLS

- **GIVEN** `SMTP_PORT=465`
- **WHEN** `Send` is invoked
- **THEN** the mailer SHALL `tls.DialWithDialer(..., addr, &tls.Config{ServerName: host, MinVersion: TLS12})`
- **AND** wrap the TLS connection in `smtp.NewClient`
- **AND** authenticate with PLAIN when credentials are set
- **AND** issue MAIL FROM, RCPT TO, DATA in sequence, closing the writer and the client on success or error

#### Scenario: Send without credentials (open relay)

- **GIVEN** `SMTP_USERNAME` is empty
- **WHEN** Send is invoked
- **THEN** the mailer SHALL pass `nil` as the `smtp.Auth` argument
- **AND** transport SHALL proceed unauthenticated

#### Scenario: UTF-8 body framing

- **WHEN** building the RFC 5322 message
- **THEN** headers SHALL be CRLF-terminated
- **AND** include `Content-Type: text/plain; charset="utf-8"` and `Content-Transfer-Encoding: 8bit`
- **AND** the headers SHALL be separated from the body by a blank CRLF line

#### Scenario: ASCII subject is sent verbatim

- **WHEN** the subject contains only bytes < 0x80
- **THEN** the Subject header SHALL be `Subject: <subject>` verbatim (no encoding)

#### Scenario: Non-ASCII subject is RFC 2047 encoded

- **WHEN** the subject contains any byte ≥ 0x80 (e.g. Chinese characters)
- **THEN** the Subject header SHALL be `Subject: =?UTF-8?B?<base64>?=` where `<base64>` is `base64.StdEncoding.EncodeToString([]byte(subject))`
- **AND** standard mail clients SHALL render the original Unicode subject

### Requirement: Disabled-SMTP No-Op Fallback

When SMTP is not configured, the mailer SHALL NOT raise an error on
`Send` — instead it SHALL log the would-be message at INFO level so dev
workflows can observe the content without standing up an SMTP server.

#### Scenario: Send with SMTP disabled

- **GIVEN** `cfg.SMTP.Enabled() == false`
- **WHEN** `Send(to, subject, body)` is invoked
- **THEN** the mailer SHALL emit one structured slog INFO entry containing the `to`, `subject`, and `body` fields
- **AND** return `nil` (no transport error)
- **AND** callers SHALL be able to use the same `Send` API in dev and prod without branching

#### Scenario: Enabled() reflects configuration

- **WHEN** both `SMTP_HOST` and `SMTP_FROM` are non-empty
- **THEN** `Mailer.Enabled()` SHALL return `true`

- **WHEN** either env var is empty
- **THEN** `Mailer.Enabled()` SHALL return `false`
- **AND** `Send` SHALL take the log-fallback path described above
