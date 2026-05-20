# Design — add-email-verification-and-oidc-hook

## Context

The v1 `bootstrap-central-panel` change shipped self-serve registration
with `email_verified=false` and an explicit comment that SMTP wasn't
yet wired. The frontend design-polish round added a math captcha as a
placeholder. We're now turning that placeholder into a real defense
(email verification code), wiring SMTP for real (`mailer`), and adding
the discovery surface so an OIDC button can appear when configured.

Two opportunistic cleanups ride along under the same "first-run" theme:
auto-generated admin password and login-form refresh.

Constraints:
- Stdlib only for SMTP — no `gomail` or vendor SDKs.
- Codes must be safe in a PostgreSQL leak (hashed).
- Dev workflow must not require a real SMTP server.
- OIDC flow itself stays a 501 stub — only the listing endpoint and UI
  scaffolding ship in this change.
- Frontend keeps the unified `/login` route, login/register tabs, and
  admin-first auto-fallback established by the design-polish round.

## Goals / Non-Goals

**Goals**
- Make registration require proof of email ownership when SMTP is set.
- Provide a discoverable, fail-soft hook so the OIDC button surfaces
  automatically the moment OIDC is configured.
- Remove the math-captcha placeholder; replace with the real flow.
- Let operators stand up the dashboard without inventing an admin
  password.

**Non-goals**
- Implementing the OIDC start/callback flow (separate change).
- Password reset / email-change verification (uses the same table, but
  a different `purpose` value; defer until a use-case lands).
- SMS / TOTP / WebAuthn.
- Vendor captcha (hCaptcha, Turnstile) — the proposal hook stays, but
  email verification + 60s rate limit is the v1 defense.
- Multi-provider OIDC (response shape supports it; env config doesn't
  yet).

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│  /login (SPA, unified)                                         │
│   ┌──────────────────────────────────────────────────────────┐ │
│   │ [登录 | 注册] tabs                                        │ │
│   │  email                                                    │ │
│   │  password                                                 │ │
│   │  (register only) 确认密码                                  │ │
│   │  (register only) 验证码  [发送验证码 / 60s]                 │ │
│   │  [继续 / 创建账户]                                         │ │
│   │  ─── 或使用其他方式登录 ───  (login only, if providers≠[]) │ │
│   │  [icon] 使用 <name> 登录                                   │ │
│   └──────────────────────────────────────────────────────────┘ │
└──────────────┬──────────────────────────────────────────┬─────┘
               │                                          │
               │ POST /auth/send-code                     │ GET /auth/oidc/providers
               │ POST /auth/register {code}               │
               ▼                                          ▼
┌─────────────────────────────────────────────────────────────────┐
│  user-auth handler                                              │
│   SendCode ───► verification.Service.SendCode                   │
│                   ├─ check cooldown (60s)                       │
│                   ├─ generate 6-digit (crypto/rand)             │
│                   ├─ INSERT email_verification_codes            │
│                   └─ mailer.Send(to, subject, body)             │
│                                                                 │
│   Register ───► if SMTP enabled:                                │
│                   verification.Service.Consume                  │
│                     ├─ SELECT latest unconsumed unexpired row   │
│                     ├─ compare sha256(code) to code_hash        │
│                     ├─ inc attempts on mismatch (cap 5)         │
│                     └─ UPDATE consumed_at = now() on match      │
│                 user.Service.Register (existing flow)           │
│                                                                 │
│   OIDCProviders ─► reflect cfg.OIDC.{DisplayName,IconURL,…}     │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
                       ┌──────────────────────┐
                       │  mailer.Send         │
                       │  ┌─ port 587 ► STARTTLS via smtp.SendMail
                       │  ├─ port 465 ► tls.Dial + smtp.NewClient
                       │  └─ disabled ► log INFO (dev fallback)
                       └──────────────────────┘
```

## Schema

New migration: `migrations/0004_email_verification_codes.up.sql`

```sql
CREATE TABLE email_verification_codes (
    id          BIGSERIAL PRIMARY KEY,
    email       TEXT        NOT NULL,
    purpose     TEXT        NOT NULL,
    code_hash   TEXT        NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at TIMESTAMPTZ,
    attempts    INT         NOT NULL DEFAULT 0
);
CREATE INDEX email_verification_codes_active
  ON email_verification_codes (email, purpose, sent_at DESC);
CREATE INDEX email_verification_codes_unconsumed
  ON email_verification_codes (email, purpose, expires_at)
  WHERE consumed_at IS NULL;
```

Two indexes:
- `..._active` — cooldown lookup (find latest row in last 60s).
- `..._unconsumed` (partial) — Consume's "latest unconsumed unexpired"
  lookup; partial so it stays small as consumed rows accumulate.

Rows are kept after consumption for audit. There is currently no GC job;
the table is expected to remain small (one signup ≈ one row, plus a few
retry rows). If volume grows, a daily `DELETE WHERE consumed_at <
now() - 30d OR (consumed_at IS NULL AND expires_at < now() - 7d)` can
be added — out of scope here.

## Code lifecycle

1. **SendCode**
   - `email = lower(trim(email))` (matches user service normalization)
   - Cooldown: `SELECT … WHERE email=$1 AND purpose=$2 AND sent_at > now()-60s LIMIT 1` → if found, return `ErrRateLimited`.
   - Generate: `rand.Int(rand.Reader, 1_000_000)` → zero-padded 6 digits.
   - Hash: `hex(sha256(code))`. Rationale below.
   - Insert row with `expires_at = now() + 10m`.
   - Call `mailer.Send`. Mailer failure does NOT roll back the row — operator can re-send after cooldown or read the code from stderr.

2. **Consume** (single DB transaction)
   - `SELECT … WHERE email=$1 AND purpose=$2 AND consumed_at IS NULL ORDER BY sent_at DESC LIMIT 1` → `ErrCodeNotFound` on miss.
   - If `expires_at < now()` → `ErrCodeExpired`.
   - If `attempts >= 5` → `ErrTooManyAttempts`.
   - If `sha256(code) != row.code_hash`:
     - `UPDATE … SET attempts = attempts + 1 WHERE id = $1`.
     - Return `ErrCodeMismatch`.
   - Else:
     - `UPDATE … SET consumed_at = now() WHERE id = $1`.
     - Return `nil`.

### Why SHA-256 (not bcrypt) for code hashing

Codes are 6 decimal digits with a 10-minute TTL. The reason we use slow
hashes for passwords is to make offline brute-force expensive against a
DB dump. For a 6-digit code that expires in 10 minutes:
- Brute space is 10⁶, which is "instant" regardless of hash function.
- `attempts >= 5` cap burns the row after 5 wrong guesses; you can't
  brute-force online either.
- A DB dump containing 6-digit hashes is essentially equivalent to a
  dump containing plaintext, but the codes are stale within an hour.

bcrypt would buy nothing here while making cold-storage rotation /
backup hygiene marginally more annoying. SHA-256 hex keeps the row
compact and indexable. This is the same reasoning used by Auth0, AWS
Cognito and similar.

## OIDC providers endpoint

`GET /api/user/auth/oidc/providers` (public) returns:

```json
[
  { "name": "集换社",
    "icon": "https://cdn.example.com/jhs.svg",
    "login_url": "/api/user/auth/oidc/start" }
]
```

- Empty array when `cfg.OIDC.Enabled() == false`. **Empty array, not
  404** — the empty list is the truthful answer; 404 would force the
  frontend to special-case "endpoint missing" vs "no providers".
- `name` falls back to `url.Parse(Issuer).Host` then literal `"OIDC"`
  if `OIDC_DISPLAY_NAME` is unset.
- `icon` omitted (omitempty) when unset; frontend renders a generic
  globe SVG.
- Response array is iterable (`v-for`) so future multi-provider config
  doesn't require an API shape change.

The frontend's `loadOIDC()` swallows any error and falls back to `[]`
so older backends (without this route) cleanly hide the section instead
of breaking the page.

## ADMIN_PASSWORD bootstrap

`config.Load()` after reading `ADMIN_PASSWORD`:

```go
if cfg.Admin.Password == "" {
    pw, err := generateAdminPassword()      // 18 bytes from crypto/rand
    if err != nil { return nil, … }         //   → 24 chars base64.RawURLEncoding
    cfg.Admin.Password = pw
    fmt.Fprintf(os.Stderr, banner, pw)      // 5-line greppable block
}
```

- `validate()` no longer lists `ADMIN_PASSWORD` as required.
- We use `fmt.Fprintf(os.Stderr, …)` not slog because slog isn't
  configured yet at this point in Load().
- A unit test asserts:
  - Missing `ADMIN_PASSWORD` no longer fails Load.
  - Generated value is ≥ 16 chars (defending against bytes-vs-encoded
    mix-ups regressing the size).
- The value is **not** persisted to disk. Operator's responsibility to
  copy it to `.env` for stable credentials across restarts. JWTs
  outlive a password change because verification uses `JWT_SECRET` only.

## Frontend impl

Single touched file: `frontend/src/views/Login.vue`.

- Removes math captcha state (`captchaA`, `captchaB`, `captchaInput`,
  `captchaValid`, `refreshCaptcha`).
- Adds `code`, `codeSending`, `codeCooldown`, `cooldownTimer`,
  `sendCode()`, `startCooldown()`.
- Adds `oidcProviders`, `loadOIDC()`, `startOIDC(provider)`.
- `onMounted(loadOIDC)`; `onUnmounted(clearInterval(cooldownTimer))`.
- Send button disabled while sending OR `codeCooldown > 0`.
- OIDC button row hidden in register mode (only the login tab shows
  third-party logins; registration is portal-only).

API surface (`frontend/src/api/portal/auth.ts`):
- `sendCode(email): POST /auth/send-code` → 204.
- `oidcProviders(): GET /auth/oidc/providers` → `OIDCProvider[]`.
- `register(email, password, code?)` — code is optional in the type so
  dev workflows without SMTP can still call register without the field.

## Risks

| Risk | Mitigation |
|---|---|
| Operator forgets to capture the auto-generated admin password and gets locked out on restart. | Banner is loud (5-line `=` rule) and explicitly says "save into .env to keep credentials stable". Restarting WILL generate a new password — but the operator can always restart again to see it. |
| SMTP misconfigured silently logs codes to stderr instead of mailing. | `cfg.SMTP.Enabled()` is computed up front; `app.go` passes that bool to the auth handler so register *enforces* the code only when SMTP is enabled. If SMTP is intended but misconfigured (host correct, From missing), `Enabled()` returns false and the operator effectively runs in dev mode — surfaceable by checking startup logs. Acceptable trade-off; alternative is to fail-fast which would prevent dev workflow. |
| Verification table grows unbounded. | Out of scope; size envelope is small (one row per signup attempt). Add cleanup cron if it becomes an issue. |
| OIDC button shows even when start/callback aren't wired (501). | Acceptable for now — clicking goes to a 501 page, operator can see what's missing. Once OIDC ships, button works without UI changes. |
| 60s client-side cooldown drifts from server-side rate limit. | They mirror exactly. If the server returns 429, the SPA still resets the cooldown to 60 — duplicates are idempotent. |

## Open questions

None blocking. Listed for future work:
- Should the verification table get a daily cleanup job? Re-evaluate
  after 30 days of production traffic.
- Should the OIDC providers endpoint cache the response? Currently it
  returns config-derived data with O(1) cost; caching would buy ms but
  complicate config reload. Defer.

## Migration plan

- Existing portal users: untouched. `email_verified` flag continues to
  reflect whatever was set on registration.
- Existing admin operators with `ADMIN_PASSWORD` set in `.env`: behavior
  unchanged (Load uses the provided value).
- Existing admin operators on first deploy after this change: if they
  somehow had `ADMIN_PASSWORD` unset before, validate() used to fail;
  now Load generates one and prints it. No actual users are in this
  bucket since validate would have prevented startup, so no migration
  needed beyond restart.

## Test plan

Unit:
- `config.TestLoad_GeneratesAdminPasswordWhenBlank` — auto-gen path.
- `config.TestLoad_FailsOnMissingRequired` (updated) — ADMIN_PASSWORD
  no longer in required-keys assertion.
- `mailer` no-op fallback log assertion (logger captures).
- `verification.SendCode` rate-limit boundary (last row sent_at
  exactly 60s ago — should pass).
- `verification.Consume` matrix: success, mismatch increments,
  expired, exhausted, already-consumed.

Integration (against real Postgres):
- Register → SendCode → wrong code 5 times → 6th attempt with the
  correct code returns `ErrTooManyAttempts` (row burnt).
- Register without code when SMTP enabled returns 400 with
  "缺少邮箱验证码".

Manual e2e (browser):
- Dev mode (SMTP off): register completes without code; SendCode
  emits a structured log containing the code.
- Prod mode (SMTP on): full happy path with real inbox.
- OIDC config absent: button row hidden.
- OIDC config present: button label reads "使用 <DisplayName> 登录"
  and falls back to issuer hostname when DisplayName empty.
