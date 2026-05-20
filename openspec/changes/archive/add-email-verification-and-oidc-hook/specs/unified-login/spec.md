## MODIFIED Requirements

### Requirement: Register Mode Uses Email Verification Code

The login page's 注册 tab SHALL drop the placeholder math captcha and
require a 6-digit email verification code instead. The user clicks
"发送验证码" to trigger `POST /api/user/auth/send-code`; the SPA mirrors
the server-side 60-second rate limit with a client-side countdown.

#### Scenario: Send code button enabled / disabled states

- **WHEN** the register form is empty or the email is malformed
- **THEN** the "发送验证码" button SHALL be enabled but clicking SHALL surface a client-side error "请先填写有效邮箱"
- **AND** SHALL NOT call the API

- **WHEN** the API call is in flight
- **THEN** the button SHALL show "发送中…" and be disabled

- **WHEN** the cooldown is active
- **THEN** the button SHALL show "Ns 后重试" (N counting down each second)
- **AND** SHALL be disabled
- **AND** the cooldown SHALL be 60 seconds, decremented client-side; the timer SHALL be cleared on `onUnmounted` to avoid leaks

#### Scenario: Code field shape

- **WHEN** the register form renders
- **THEN** the verification code input SHALL accept up to 6 digits (`maxlength="6"`, `inputmode="numeric"`, `pattern="\d{6}"`)
- **AND** declare `autocomplete="one-time-code"` so iOS / 1Password offer SMS-style autofill
- **AND** be visually centered with `tracking-[0.4em]` digit spacing for readability

#### Scenario: Submit register with code

- **GIVEN** the user has typed exactly 6 digits in the code field
- **WHEN** the submit button is clicked
- **THEN** the SPA SHALL call `portalAuthApi.register(email, password, code)`
- **AND** on success, store the portal token and navigate to `/portal`
- **AND** on backend rejection ("验证码不正确" etc.), display the server message verbatim

#### Scenario: Mode switch clears the code field

- **WHEN** the user switches from 注册 back to 登录 (or vice versa)
- **THEN** the code field's value SHALL be cleared
- **AND** the password confirm field SHALL be cleared
- **AND** any error message SHALL be cleared
- **AND** the cooldown timer SHALL NOT be reset (it represents a server-side state the user can re-trigger from the next mode load)

### Requirement: OIDC Button Row Beneath The Form (Login Mode Only)

The login page SHALL fetch the OIDC providers list on mount and render
one labeled button per provider beneath the email/password form. The
button row SHALL appear exclusively on the 登录 tab.

#### Scenario: Mount fetch

- **WHEN** the login view mounts
- **THEN** `onMounted` SHALL call `portalAuthApi.oidcProviders()`
- **AND** store the returned array in a reactive ref
- **AND** swallow any error (network, 404 on older backends, 5xx) by treating it as an empty list

#### Scenario: Empty providers — section hidden

- **GIVEN** the providers ref is `[]`
- **WHEN** the login view renders
- **THEN** neither the "或使用其他方式登录" divider nor any button SHALL appear

#### Scenario: Non-empty providers — divider + buttons rendered

- **GIVEN** the providers ref has 1+ elements
- **AND** the current mode is `login`
- **WHEN** the login view renders
- **THEN** a horizontal divider with the text "或使用其他方式登录" SHALL appear immediately below the form
- **AND** one button SHALL render per provider, labeled `使用 {{ p.name }} 登录`
- **AND** the button's leading visual SHALL be `<img src={{ p.icon }}>` when `p.icon` is set, otherwise a generic globe SVG

#### Scenario: Button click starts OIDC flow

- **WHEN** the user clicks a provider button
- **THEN** the SPA SHALL navigate the browser to `{p.login_url}?next={current ?next or /portal}` via `window.location.assign`
- **AND** SHALL preserve any existing query parameters on `login_url`

#### Scenario: OIDC buttons hidden in register mode

- **GIVEN** the providers ref is non-empty
- **WHEN** the user clicks the 注册 tab
- **THEN** the divider and buttons SHALL be hidden
- **AND** SHALL re-appear when they switch back to the 登录 tab

## REMOVED Requirements

### Requirement: Math Captcha On Register Form

**Removed by**: this change. Replaced by email verification code.

The earlier placeholder math captcha (`X + Y = ?`) is removed entirely.
Its scenarios, the `captchaA`/`captchaB`/`captchaInput`/`captchaValid`
refs, and the "换一道题" refresh button are gone. The bot-defense role
is now served by email verification + the 60-second send rate limit.
