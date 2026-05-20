# Tasks — add-email-verification-and-oidc-hook

Checkboxes reflect what's already shipped in code (all done by the time
this change is being documented). Future cleanup items are explicitly
marked `[ ]`.

## 1. Schema

- [x] 1.1 Migration `migrations/0004_email_verification_codes.up.sql` with the table + two indexes (active by `(email, purpose, sent_at DESC)`, partial unconsumed by `(email, purpose, expires_at) WHERE consumed_at IS NULL`)
- [x] 1.2 Migration `migrations/0004_email_verification_codes.down.sql` (`DROP TABLE IF EXISTS`)
- [x] 1.3 GORM type lives in `internal/service/verification/service.go::record` (not promoted to `internal/model` since it's service-internal); `TableName()` returns `email_verification_codes`

## 2. Mailer package (`internal/mailer`)

- [x] 2.1 `Mailer` struct + `New(cfg config.SMTP, logger *slog.Logger)` constructor
- [x] 2.2 `Send(to, subject, body) error` — branch on port (587 → STARTTLS via `smtp.SendMail`; 465 → `tls.DialWithDialer` + `smtp.NewClient`)
- [x] 2.3 No-op fallback when `cfg.Enabled() == false`: log INFO with `to`, `subject`, `body`; return `nil`
- [x] 2.4 RFC 5322 message builder: CRLF headers, blank-CRLF separator, `Content-Type: text/plain; charset="utf-8"`, `Content-Transfer-Encoding: 8bit`
- [x] 2.5 RFC 2047 subject encoder: `=?UTF-8?B?<base64>?=` triggered when any byte ≥ 0x80 (handles Chinese subjects)
- [x] 2.6 Sentinel error `ErrSMTPNotConfigured` exported for service callers that want to surface "really requires SMTP" (not used internally — Send always succeeds in disabled mode)

## 3. Verification service (`internal/service/verification`)

- [x] 3.1 `Purpose` typed string enum, currently single value `PurposeRegister = "register"`; codes scoped by purpose so a register code can't be replayed as a future reset code
- [x] 3.2 Constants: `codeLength=6`, `codeTTL=10m`, `resendCooldown=60s`, `maxAttempts=5`
- [x] 3.3 Sentinel errors: `ErrRateLimited`, `ErrCodeNotFound`, `ErrCodeExpired`, `ErrCodeMismatch`, `ErrTooManyAttempts`
- [x] 3.4 `Service` struct with `db`, `mailer`, `logger`; constructor `New(db, mailer, logger)`
- [x] 3.5 `SendCode(ctx, email, purpose)`:
  - Normalize email (lower + trim, matches `user.normalizeEmail`)
  - Query latest row in last `resendCooldown`; if found → `ErrRateLimited`
  - Generate code via `rand.Int(rand.Reader, 1_000_000)`; zero-pad to 6 digits
  - Insert row with `code_hash = sha256_hex(code)`, `expires_at = now+codeTTL`
  - Call `mailer.Send`; on failure, log warn + return wrapped error (row stays — operator can retry after cooldown OR read from logs in dev)
- [x] 3.6 `Consume(ctx, email, code, purpose)` in a single DB transaction:
  - Find latest `consumed_at IS NULL` row → `ErrCodeNotFound` on miss
  - Check `expires_at < now()` → `ErrCodeExpired`
  - Check `attempts >= maxAttempts` → `ErrTooManyAttempts`
  - Compare `sha256_hex(code)` vs `code_hash`:
    - mismatch → increment `attempts`, return `ErrCodeMismatch`
    - match → `UPDATE consumed_at = now()`, return `nil`

## 4. user service & handler updates

- [x] 4.1 `service/user/service.go::RegisterInput` gains `Code string` JSON field
- [x] 4.2 Handler `handler/user/auth.go::AuthHandler` constructor extended with `*verification.Service`, `config.OIDC`, `smtpOn bool` — declared in `NewAuthHandler(users, auth, verify, oidcCfg, smtpOn)`
- [x] 4.3 `Register` handler:
  - If `smtpOn && req.Code == ""` → 400 `"缺少邮箱验证码"`
  - If `smtpOn` → call `verify.Consume(ctx, email, code, PurposeRegister)`; map errors:
    - `ErrCodeMismatch`/`ErrCodeNotFound` → 400 `"验证码不正确"`
    - `ErrCodeExpired` → 400 `"验证码已过期，请重新发送"`
    - `ErrTooManyAttempts` → 429 `"验证次数过多，请重新发送验证码"`
  - Then proceed with existing `users.Register` call + JWT issuance
- [x] 4.4 `SendCode` handler: POST `/auth/send-code` → calls `verify.SendCode(ctx, email, PurposeRegister)`; map `ErrRateLimited` → 429 with friendly message; other errors → 500; success → 204 No Content
- [x] 4.5 `OIDCProviders` handler: GET `/auth/oidc/providers` → reflect `cfg.OIDC.{DisplayName,IconURL,Issuer}`; empty `OIDC.Enabled() == false` returns `[]` (status 200, not 404)
- [x] 4.6 Route mounting in `RegisterRoutes`: add `POST /auth/send-code` and `GET /auth/oidc/providers`; keep existing `OIDCStart`/`OIDCCallback` 501 stubs

## 5. config bootstrap

- [x] 5.1 Add `crypto/rand`, `encoding/base64`, `os` imports to `config/config.go`
- [x] 5.2 `generateAdminPassword()` — 18 bytes from `rand.Read`, encoded with `base64.RawURLEncoding` → 24 URL-safe chars
- [x] 5.3 After reading `ADMIN_PASSWORD`: if blank, generate one, assign to `cfg.Admin.Password`, write banner to `os.Stderr` with the literal `ADMIN_PASSWORD=<value>` line so it's greppable
- [x] 5.4 Remove `ADMIN_PASSWORD` from `validate()` required keys
- [x] 5.5 New env vars on `config.OIDC`: `DisplayName` (from `OIDC_DISPLAY_NAME`), `IconURL` (from `OIDC_ICON_URL`)

## 6. app wiring

- [x] 6.1 Add imports for `mailer` and `verification` packages in `internal/app/app.go`
- [x] 6.2 Construct `mailerSvc = mailer.New(cfg.SMTP, logger)` after `userService`
- [x] 6.3 Construct `verifyService = verification.New(db, mailerSvc, logger)`
- [x] 6.4 Pass new deps to handler: `userhandler.NewAuthHandler(userService, authSvc, verifyService, cfg.OIDC, cfg.SMTP.Enabled())`

## 7. Tests

- [x] 7.1 `config.TestLoad_FailsOnMissingRequired` updated to drop `ADMIN_PASSWORD` from the expected-missing list; also asserts that ADMIN_PASSWORD is NOT in the error message
- [x] 7.2 New `config.TestLoad_GeneratesAdminPasswordWhenBlank` asserts auto-gen path produces a ≥16-char value
- [x] 7.3 `internal/service/verification/service_test.go` — DB-gated on `INTEGRATION_DB_URL` (same pattern as e2e harness). Covers: SendCode happy path / hashed-at-rest assertion / 60s cooldown rejection, Consume happy path / replay returns NotFound / mismatch increments attempts / burnt row returns TooManyAttempts / force-expired row returns Expired. Plus pure-unit tests for `normalizeEmail` and `generateCode` (always run). DB tests skip cleanly when env not set.
- [x] 7.4 `internal/mailer/mailer_test.go` — pure unit (no DB). Captures slog records via a `capturingHandler`, asserts disabled-SMTP `Send` produces one INFO record with `to/subject/body` attrs. Plus `Enabled()` truth-table, RFC 5322 framing assertion, RFC 2047 subject encoding (ASCII verbatim vs Chinese base64). 7/7 pass.

## 8. Frontend

- [x] 8.1 `src/api/portal/auth.ts`:
  - `OIDCProvider` interface (`name`, `icon?`, `login_url`)
  - `register(email, password, code?)` (code optional in type)
  - `sendCode(email): POST /auth/send-code → void` (204)
  - `oidcProviders(): GET /auth/oidc/providers → OIDCProvider[]` (empty array on any error — fail-soft)
- [x] 8.2 `src/views/Login.vue` — drop the math-captcha block entirely (state + UI):
  - Remove refs: `captchaA`, `captchaB`, `captchaInput`, `captchaValid`, `refreshCaptcha`
  - Remove UI: "人机验证" row with arithmetic prompt
- [x] 8.3 `src/views/Login.vue` — add verification code state:
  - `code`, `codeSending`, `codeCooldown` refs + `cooldownTimer` interval id
  - `sendCode()` async — pre-check email format, call API, start 60s cooldown
  - `startCooldown(seconds)` decrements every 1s, clears interval on zero
  - `onUnmounted(clearInterval)` so navigating away doesn't leak the timer
- [x] 8.4 `src/views/Login.vue` — add verification code UI in register mode:
  - Input (inputmode=numeric, maxlength=6, `autocomplete="one-time-code"`, tracking-[0.4em] for visual digit gap)
  - Adjacent "发送验证码 / Ns 后重试 / 发送中…" button, disabled during sending/cooldown
  - Hint line `收到的验证码 10 分钟内有效`
- [x] 8.5 `src/views/Login.vue` — `doRegister` validates code length 6 client-side, passes `code` to `portalAuthApi.register`
- [x] 8.6 `src/views/Login.vue` — OIDC providers integration:
  - `oidcProviders` ref, `loadOIDC()` async (silent on error)
  - `startOIDC(provider)` navigates to `provider.login_url` with `?next=` preserved
  - `onMounted(loadOIDC)`
- [x] 8.7 `src/views/Login.vue` — OIDC button row in template:
  - Visible only when `mode === 'login' && oidcProviders.length > 0`
  - Horizontal divider with text `或使用其他方式登录`
  - One button per provider: icon (img if `provider.icon`, else generic globe SVG) + label `使用 {{name}} 登录`

## 9. Documentation

- [x] 9.1 New spec deltas in `openspec/changes/add-email-verification-and-oidc-hook/specs/`:
  - `email-verification/spec.md` — ADDED
  - `mailer/spec.md` — ADDED
  - `oidc-providers/spec.md` — ADDED
  - `user-accounts/spec.md` — MODIFIED (register accepts + verifies code)
  - `admin-auth/spec.md` — MODIFIED (ADMIN_PASSWORD optional with auto-gen)
  - `unified-login/spec.md` — MODIFIED (captcha → code; OIDC row)
- [x] 9.2 Promoted module specs (canonical, current-state) created under `openspec/specs/` for the new modules so consumers don't need to read the change folder to learn behavior:
  - `openspec/specs/email-verification/`, `mailer/`, `oidc-providers/`, `auth-bootstrap/`, `unified-login/`, `theme-system/`, `design-system/`, plus `README.md` index
- [x] 9.3 `deploy/.env.example` updated: ADMIN_PASSWORD comment rewritten to document the auto-generation banner + how to capture the value; new commented-out `OIDC_DISPLAY_NAME` / `OIDC_ICON_URL` block with fallback semantics explained; SMTP section comment rewritten to reflect the verification-code register flow (was about email-binding before).

## 10. Out of this change

These are explicitly NOT part of this change — opened as future tasks:

- OIDC start / callback / token-exchange flow (the 501 stubs stay).
- Password-reset flow (would add `PurposeReset` value + new endpoint;
  same table).
- Vendor captcha integration (the proposal hook in code comments
  references hCaptcha/Turnstile; left for future).
- Cleanup cron for `email_verification_codes` (re-evaluate after 30
  days of production traffic).
- Multi-provider OIDC config (response shape supports it; env config
  doesn't).
