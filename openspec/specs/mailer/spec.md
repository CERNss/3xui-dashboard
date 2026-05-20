# mailer

Stdlib-only SMTP wrapper. Sends UTF-8 plain-text email when SMTP is
configured; falls back to a logging no-op when not.

## Purpose & boundaries

Other services (currently only `verification`; future: password reset,
plan-expiry notice) need to send transactional email. This module
provides a single `Send(to, subject, body) error` surface and isolates
all SMTP transport quirks (STARTTLS vs implicit TLS, header encoding,
auth methods) here.

No queue, no retries inside the mailer — callers decide retry policy.
The verification service treats a send failure as fatal-for-the-request
because the verification record is already persisted; the operator can
re-send after the cooldown.

## Configuration

Env vars consumed in `internal/config/config.go::Load()`, mapped to
`config.SMTP`:

| Var | Default | Notes |
|---|---|---|
| `SMTP_HOST` | — | Empty disables real delivery. |
| `SMTP_PORT` | 587 | 587 → STARTTLS path; 465 → implicit-TLS path. |
| `SMTP_USERNAME` | — | When empty, auth is skipped entirely (open relay). |
| `SMTP_PASSWORD` | — | Used with username for PLAIN auth. |
| `SMTP_FROM` | — | Required for `Enabled()` to return true. |
| `SMTP_USE_TLS` | true | Reserved — current branch logic uses port, not this flag. |

`config.SMTP.Enabled() == (Host != "" && From != "")`.

## Requirements

### Requirement: Enabled() reports real-delivery availability

The mailer SHALL expose an `Enabled()` accessor that callers can use
to branch on whether SMTP is actually configured.

#### Scenario: Both host and from set

- **WHEN** `SMTP_HOST` and `SMTP_FROM` are both non-empty
- **THEN** `Mailer.Enabled()` SHALL return `true`

#### Scenario: Host or from missing

- **WHEN** either of those env vars is empty
- **THEN** `Mailer.Enabled()` SHALL return `false`
- **AND** `Send` SHALL log instead of attempting transport

### Requirement: Send logs in lieu of delivery when disabled

When SMTP is not configured, `Send` SHALL log the would-be message at
INFO level and return `nil` — operators in dev can copy verification
codes out of stderr without standing up a mail server.

#### Scenario: Send with SMTP disabled

- **GIVEN** `Mailer.Enabled() == false`
- **WHEN** `Send(to, subject, body)` is called
- **THEN** the mailer SHALL emit one structured log entry at INFO level
  containing `to`, `subject`, and `body`
- **AND** SHALL return `nil` (no transport error)

### Requirement: STARTTLS path for port 587 / default ports

The mailer SHALL use `net/smtp.SendMail` for the STARTTLS upgrade path
on standard submission ports.

#### Scenario: Send via port 587

- **GIVEN** `SMTP_PORT=587` and credentials configured
- **WHEN** `Send` is called
- **THEN** the mailer SHALL hand off to `smtp.SendMail(addr, PlainAuth, from, []string{to}, msg)`
- **AND** the stdlib SHALL negotiate STARTTLS automatically when the server advertises it
- **AND** any error from `SendMail` SHALL be wrapped with `fmt.Errorf("smtp send: %w", err)` and returned to the caller

#### Scenario: No credentials configured (open relay)

- **GIVEN** `SMTP_USERNAME` is empty
- **WHEN** `Send` is called
- **THEN** the mailer SHALL pass `nil` as the `smtp.Auth` argument
- **AND** delivery SHALL proceed unauthenticated

### Requirement: Implicit TLS path for port 465

The mailer SHALL connect via `crypto/tls.Dial` for the implicit-TLS path
(port 465), bypassing stdlib's STARTTLS code path which doesn't apply.

#### Scenario: Send via port 465

- **GIVEN** `SMTP_PORT=465`
- **WHEN** `Send` is called
- **THEN** the mailer SHALL `tls.DialWithDialer` with `ServerName=Host` and `MinVersion=TLS12`
- **AND** wrap the TLS conn in `smtp.NewClient`
- **AND** authenticate with PLAIN auth if credentials are set
- **AND** issue MAIL FROM, RCPT TO, DATA in sequence
- **AND** close the writer + quit the client even on error paths

### Requirement: UTF-8 body and RFC 2047 subject encoding

The mailer SHALL emit `Content-Type: text/plain; charset="utf-8"` and
RFC 2047-encode the Subject header when it contains non-ASCII bytes.

#### Scenario: ASCII-only subject

- **WHEN** the subject contains only bytes < 0x80
- **THEN** the header SHALL be `Subject: <subject>` verbatim

#### Scenario: Chinese / non-ASCII subject

- **WHEN** the subject contains any byte ≥ 0x80
- **THEN** the header SHALL be `Subject: =?UTF-8?B?<base64>?=`
  where `<base64>` is `base64.StdEncoding(subject)`
- **AND** clients displaying the message SHALL render the original
  Unicode subject (verified against Gmail / 网易 / 腾讯 mail in
  manual e2e)

#### Scenario: Body framing

- **WHEN** building the RFC 5322 message
- **THEN** headers SHALL be CRLF-terminated
- **AND** body SHALL be separated from headers by a blank CRLF line
- **AND** Content-Transfer-Encoding SHALL be `8bit` (no quoted-printable / base64 re-encoding of the body)

## Out of scope

- HTML/multipart messages (no HTML template needs yet).
- DKIM signing (operator's MTA handles it upstream).
- Bounce handling, suppression lists.
- Async send queue with retries (verification's send is synchronous in
  the request path; if reliability becomes an issue, wrap with the
  webhook-style persistent retry pattern).
