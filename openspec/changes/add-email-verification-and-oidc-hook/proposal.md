# add-email-verification-and-oidc-hook

## Why

Self-serve portal registration in the v1 (`bootstrap-central-panel`) build
had no defense against fake or hostile signups: anyone with a reachable
endpoint could spam `POST /api/user/auth/register` and create accounts
without ever proving control of the email. The earlier proposal noted
"SMTP delivery / verification is not implemented" and shipped users with
`email_verified=false` — a deliberate v1 punt.

At the same time, the login chrome now needs a discoverable hook for the
operator's OIDC identity provider (集换社, an in-house SSO, etc.) so the
"使用 X 登录" button can appear next to the email/password form. The OIDC
flow itself isn't ready, but the **presentation surface** has to be in
place so the rest of the page renders correctly today and the button can
light up the moment an operator sets `OIDC_*` env vars.

Two additional cleanups ride along because they belong to the same
"first-run experience" theme:

- **ADMIN_PASSWORD** is currently hard-required at startup, forcing
  operators to invent a password before they have a working system. We
  want the binary to bootstrap a random one on its own when the env var
  is blank, and log it once so the operator can paste it into `.env`.
- The math-captcha placeholder added during the design polish round
  needs to come out — email verification is the real bot defense, the
  captcha was always a stand-in.

## What Changes

### New capabilities

- **`email-verification`** — short-lived 6-digit codes scoped by purpose
  (currently `register`), 10-minute TTL, 60-second send cooldown, codes
  hashed at rest, single-use semantics. Storage in a new
  `email_verification_codes` table.

- **`mailer`** — stdlib-only SMTP wrapper. Handles STARTTLS (port 587)
  and implicit-TLS (port 465) branches. UTF-8 plain-text body, RFC 2047
  subject encoding for non-ASCII (e.g. Chinese). When SMTP is not
  configured, falls back to a structured INFO log of the message so dev
  flows can copy the verification code from stderr.

- **`oidc-providers`** — new public endpoint
  `GET /api/user/auth/oidc/providers` that returns an array describing
  the configured OIDC IdP (display name, optional icon URL, login URL).
  Empty array when not configured. Two new env vars
  (`OIDC_DISPLAY_NAME`, `OIDC_ICON_URL`) feed the human-facing button
  label; the existing `OIDC_*` flow stubs remain 501 until the federation
  work lands separately.

### Modified capabilities

- **`user-accounts`** — `RegisterInput` gains a `Code` field. When SMTP
  is enabled, register MUST call the verification service's `Consume`
  before creating the user row; when SMTP is disabled (dev), the check
  is skipped so testing isn't blocked by missing mail infra. Behavior
  for OIDC, public-registration toggle, and domain allowlist is unchanged.

- **`admin-auth`** — `ADMIN_PASSWORD` is no longer required at startup.
  When blank, the system generates a 24-char URL-safe random password via
  `crypto/rand` and prints a banner to stderr advising the operator to
  copy it into `.env` for stability across restarts. `ADMIN_USERNAME`
  remains required. JWT issuance and credential check are unchanged.

- **`unified-login`** — login page drops the math-captcha placeholder.
  Register mode gains a 6-digit verification code field with an inline
  "发送验证码" button + 60-second client-side countdown that mirrors the
  server-side rate limit. Login mode renders the OIDC button row beneath
  the form when providers are returned by the API.

### Behavior NOT changing (called out so reviewers don't ask)

- Admin / portal sessions stay separate (two Pinia stores, two token
  storage keys).
- `users.email_verified` semantics — still set per the existing logic;
  successful Consume on register flips it via the existing code path.
- Webhook events emitted on register — unchanged (`user.registered`).
- Existing OIDC start / callback handlers — remain 501 stubs.

## Capabilities

### New Capabilities

- `email-verification`: 6-digit code lifecycle (send, store, consume),
  rate limiting (60s per email/purpose), single-use semantics, hashed
  storage, dev-mode fallback when SMTP is not configured.
- `mailer`: stdlib SMTP wrapper exposing `Send(to, subject, body)`,
  STARTTLS vs implicit-TLS routing by port, UTF-8 body with RFC 2047
  subject encoding, no-op logging fallback when not configured.
- `oidc-providers`: public providers listing endpoint, two new UI-hint
  env vars, frontend integration that renders one button per provider
  beneath the login form.

### Modified Capabilities

- `user-accounts`: register accepts and verifies a 6-digit code when
  SMTP is enabled.
- `admin-auth`: `ADMIN_PASSWORD` becomes optional with crypto/rand
  auto-generation on blank.
- `unified-login` (frontend chrome): captcha → code field; OIDC button
  row appears when providers are returned.
